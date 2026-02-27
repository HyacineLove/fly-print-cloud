package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"
	"fly-print-cloud/api/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		fmt.Printf("Failed to create upload directory: %v\n", err)
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
		// 使用凭证验证
		payload, err := h.tokenManager.ValidateUploadToken(token)
		if err != nil {
			log.Printf("Upload token validation failed: %v", err)
			errorCode := security.GetTokenErrorCode(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   errorCode,
				"message": err.Error(),
			})
			return
		}
		uploaderID = payload.NodeID // 使用节点ID作为上传者标识
		nodeID = payload.NodeID
		log.Printf("File upload authorized via token for node %s, printer %s", payload.NodeID, payload.PrinterID)
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

	// Validate size
	if fileHeader.Size > h.config.MaxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File too large. Max size: %d bytes", h.config.MaxSize)})
		return
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	allowed := map[string]bool{
		// 图片格式
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".bmp":  true,
		".gif":  true,
		".tiff": true,
		".webp": true,
		// 文档格式
		".pdf":  true,
		".doc":  true,
		".docx": true,
	}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"error":   "file_type_not_allowed",
			"message": fmt.Sprintf("不支持的文件类型 '%s'。仅支持：PNG, JPG, JPEG, BMP, GIF, TIFF, WEBP, PDF, DOC, DOCX", ext),
		})
		return
	}

	// Generate safe filename
	fileUUID := uuid.New().String()
	fileName := fileUUID + ext
	filePath := filepath.Join(h.config.UploadDir, fileName)

	// Save file
	if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
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
		"code": 200,
		"message": "success",
		"data": file,
	})
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
			log.Printf("Download token validation failed for file %s: %v", id, err)
			errorCode := security.GetTokenErrorCode(err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"error":   errorCode,
				"message": err.Error(),
			})
			return
		}
		log.Printf("File download authorized via token for file %s, job %s, node %s", payload.FileID, payload.JobID, payload.NodeID)
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
		log.Printf("Upload token verification failed: %v", err)
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
