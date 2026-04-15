package handlers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/logger"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"
	"fly-print-cloud/api/internal/utils"
	"fly-print-cloud/api/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	uploadRuleMaxSizeBytes = 10 * 1024 * 1024
	uploadRuleMaxPages     = 5
)

type FileHandler struct {
	repo         *database.FileRepository
	config       *config.StorageConfig
	wsManager    *websocket.ConnectionManager
	tokenManager *security.TokenManager
}

func NewFileHandler(repo *database.FileRepository, cfg *config.StorageConfig, wsManager *websocket.ConnectionManager, tokenManager *security.TokenManager) *FileHandler {
	// Ensure upload directory exists
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		logger.Error("Failed to create upload directory", zap.Error(err))
	}
	return &FileHandler{repo: repo, config: cfg, wsManager: wsManager, tokenManager: tokenManager}
}

// Upload 上传文件
func (h *FileHandler) Upload(c *gin.Context) {
	// 检查是否使用上传凭证
	token := c.Query("token")
	var uploaderID string
	var nodeID string

	if token != "" {
		// 第一阶段：使用轻量验证（不消耗Token），提前检查Token有效性
		payload, err := h.tokenManager.VerifyUploadTokenLightweight(token)
		if err != nil {
			logger.Warn("Upload token lightweight verification failed", zap.Error(err))
			errorCode := security.GetTokenErrorCode(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   errorCode,
				"message": err.Error(),
			})
			return
		}
		// 暂存信息，稍后验证通过再使用
		nodeID = payload.NodeID
		uploaderID = payload.NodeID // 使用节点ID作为上传者标识
		logger.Debug("File upload pre-authorized for node", zap.String("node_id", payload.NodeID), zap.String("printer_id", payload.PrinterID))
	} else {
		// 使用 OAuth2 验证（可选认证模式下由中间件处理）
		if val, exists := c.Get("external_id"); exists {
			uploaderID = val.(string)
		} else if val, exists := c.Get("sub"); exists {
			uploaderID = val.(string)
		} else {
			// 没有凭证也没有 OAuth2 认证
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   "unauthorized",
				"message": "Upload token or OAuth2 authentication required",
			})
			return
		}
		// 从查询参数获取 node_id（可选）
		nodeID = c.Query("node_id")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Open the file to check magic bytes
	srcFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer srcFile.Close()

	if err := h.validateUploadRules(fileHeader, srcFile); err != nil {
		if err == errUploadInvalidType {
			BadRequestWithCode(c, ErrCodeFileInvalidType)
			return
		}
		if err == errUploadTooLarge {
			BadRequestWithCode(c, ErrCodeFileTooLarge)
			return
		}
		if err == errUploadTooManyPages {
			BadRequestWithCode(c, ErrCodeFileTooManyPages)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate file"})
		return
	}

	// Sanitize filename
	safeFilename := security.SanitizeFilename(fileHeader.Filename)

	// Generate safe filename with extension
	ext := filepath.Ext(safeFilename)
	fileUUID := uuid.New().String()
	fileName := fileUUID + ext
	filePath := filepath.Join(h.config.UploadDir, fileName)

	// Save file
	if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// 第二阶段：所有验证通过后，真正验证并消耗Token
	if token != "" {
		payload, err := h.tokenManager.ValidateUploadToken(token)
		if err != nil {
			// Token验证失败，删除已保存的文件
			os.Remove(filePath)
			logger.Warn("Upload token validation failed", zap.Error(err))
			errorCode := security.GetTokenErrorCode(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   errorCode,
				"message": err.Error(),
			})
			return
		}
		logger.Debug("File upload fully authorized via token", zap.String("node_id", payload.NodeID), zap.String("printer_id", payload.PrinterID))
	}

	// Create DB record
	file := &models.File{
		OriginalName: fileHeader.Filename,
		FileName:     fileName,
		FilePath:     filePath,
		MimeType:     fileHeader.Header.Get("Content-Type"),
		Size:         fileHeader.Size,
		UploaderID:   uploaderID,
	}

	if err := h.repo.Create(file); err != nil {
		// Clean up file if DB fails
		os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	// Generate URL (relative path for API)
	file.URL = fmt.Sprintf("/api/v1/files/%s", file.ID)

	// Check if node_id is provided for preview
	if nodeID != "" {
		// Dispatch preview command
		if err := h.wsManager.DispatchPreviewFile(nodeID, file.ID, file.URL, file.OriginalName, file.Size, file.MimeType); err != nil {
			// Log error but continue
			fmt.Printf("Failed to dispatch preview to node %s: %v\n", nodeID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    file,
	})
}

var (
	errUploadInvalidType  = fmt.Errorf("invalid file type")
	errUploadTooLarge     = fmt.Errorf("file too large")
	errUploadTooManyPages = fmt.Errorf("too many pages")
)

func (h *FileHandler) validateUploadRules(fileHeader *multipart.FileHeader, srcFile multipart.File) error {
	if fileHeader.Size > uploadRuleMaxSizeBytes {
		return errUploadTooLarge
	}
	buffer := make([]byte, 512)
	bytesRead, err := srcFile.Read(buffer)
	if err != nil && err != io.EOF {
		return err
	}
	contentType := http.DetectContentType(buffer[:bytesRead])
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !security.IsAllowedFileType(contentType, security.AllowedPrintFileTypes) {
		allowedExtensions := []string{".docx", ".doc", ".pdf"}
		isAllowedExt := false
		for _, allowedExt := range allowedExtensions {
			if ext == allowedExt {
				isAllowedExt = true
				break
			}
		}
		if !isAllowedExt {
			return errUploadInvalidType
		}
	}
	if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if ext != ".pdf" && ext != ".doc" && ext != ".docx" {
		return nil
	}
	pageCount, err := utils.ValidateDocumentPageCountFromReader(srcFile, srcFile, fileHeader.Size, ext, uploadRuleMaxPages)
	if err != nil {
		logger.Debug("document page validation failed", zap.Error(err), zap.String("file", fileHeader.Filename))
		return errUploadTooManyPages
	}
	logger.Debug("document page validation passed", zap.Int("pages", pageCount), zap.String("file", fileHeader.Filename))
	return nil
}

// Download 下载文件
func (h *FileHandler) Download(c *gin.Context) {
	id := c.Param("id")

	// 检查是否使用下载凭证
	token := c.Query("token")

	if token != "" {
		// 使用凭证验证
		payload, err := h.tokenManager.ValidateDownloadToken(token, id, "")
		if err != nil {
			logger.Warn("Download token validation failed for file", zap.String("file_id", id), zap.Error(err))
			errorCode := security.GetTokenErrorCode(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   errorCode,
				"message": err.Error(),
			})
			return
		}
		logger.Debug("File download authorized via token", zap.String("file_id", payload.FileID), zap.String("job_id", payload.JobID), zap.String("node_id", payload.NodeID))
	} else {
		// 使用 OAuth2 验证
		// 检查是否有认证信息
		_, hasAuth := c.Get("external_id")
		if !hasAuth {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   "unauthorized",
				"message": "Download token or OAuth2 authentication required",
			})
			return
		}

		// 获取文件信息进行权限检查
		file, err := h.repo.GetByID(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if file == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}

		// Permission check for OAuth2 users
		roles, _ := c.Get("roles")
		rolesSlice, _ := roles.([]string)

		hasAdmin := false
		hasReadScope := false
		for _, r := range rolesSlice {
			if r == "admin" || r == "fly-print-admin" {
				hasAdmin = true
			}
			if r == "file:read" {
				hasReadScope = true
			}
		}

		currentUser, _ := c.Get("external_id")
		isOwner := currentUser == file.UploaderID

		if !hasAdmin && !hasReadScope && !isOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		// Serve file
		c.FileAttachment(file.FilePath, file.OriginalName)
		return
	}

	// 使用凭证验证时，直接获取并返回文件
	file, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if file == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Serve file
	c.FileAttachment(file.FilePath, file.OriginalName)
}

// VerifyUploadToken 轻量验证上传Token（不消耗一次性Token）
// GET /api/v1/files/verify-upload-token?token=xxx
func (h *FileHandler) VerifyUploadToken(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"error":   "missing_token",
			"message": "Upload token is required",
		})
		return
	}

	// 使用轻量验证方法（不标记为已使用）
	payload, err := h.tokenManager.VerifyUploadTokenLightweight(token)
	if err != nil {
		logger.Warn("Upload token verification failed", zap.Error(err))
		errorCode := security.GetTokenErrorCode(err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"error":   errorCode,
			"message": err.Error(),
			"valid":   false,
		})
		return
	}

	// 返回验证结果和Token信息
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Token is valid",
		"valid":   true,
		"data": gin.H{
			"node_id":    payload.NodeID,
			"printer_id": payload.PrinterID,
			"expires_at": payload.ExpiresAt,
		},
	})
}

func (h *FileHandler) PreflightUpload(c *gin.Context) {
	token := c.Query("token")
	if token != "" {
		_, err := h.tokenManager.VerifyUploadTokenLightweight(token)
		if err != nil {
			logger.Warn("Upload token preflight verification failed", zap.Error(err))
			errorCode := security.GetTokenErrorCode(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   errorCode,
				"message": err.Error(),
				"valid":   false,
			})
			return
		}
	} else {
		if _, exists := c.Get("external_id"); !exists {
			if _, exists := c.Get("sub"); !exists {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"error":   "unauthorized",
					"message": "Upload token or OAuth2 authentication required",
					"valid":   false,
				})
				return
			}
		}
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeBadRequest,
			"message": "No file uploaded",
			"valid":   false,
		})
		return
	}
	srcFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    ErrCodeInternalServerError,
			"message": "Failed to open uploaded file",
			"valid":   false,
		})
		return
	}
	defer srcFile.Close()
	if err := h.validateUploadRules(fileHeader, srcFile); err != nil {
		if err == errUploadInvalidType {
			BadRequestWithCode(c, ErrCodeFileInvalidType)
			return
		}
		if err == errUploadTooLarge {
			BadRequestWithCode(c, ErrCodeFileTooLarge)
			return
		}
		if err == errUploadTooManyPages {
			BadRequestWithCode(c, ErrCodeFileTooManyPages)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    ErrCodeInternalServerError,
			"message": "Failed to validate file",
			"valid":   false,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Preflight validation passed",
		"valid":   true,
	})
}
