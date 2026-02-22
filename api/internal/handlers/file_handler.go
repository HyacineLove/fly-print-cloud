package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fly-print-cloud/api/internal/config"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FileHandler struct {
	repo      *database.FileRepository
	config    *config.StorageConfig
	wsManager *websocket.ConnectionManager
}

func NewFileHandler(repo *database.FileRepository, cfg *config.StorageConfig, wsManager *websocket.ConnectionManager) *FileHandler {
	// Ensure upload directory exists
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		fmt.Printf("Failed to create upload directory: %v\n", err)
	}
	return &FileHandler{repo: repo, config: cfg, wsManager: wsManager}
}

// Upload 上传文件
func (h *FileHandler) Upload(c *gin.Context) {
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
		".pdf": true, 
		".jpg": true, 
		".jpeg": true, 
		".png": true, 
		".docx": true,
	}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed. Supported: PDF, JPG, PNG, DOCX"})
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

	// Get User ID from context
	var userIDStr string
	if val, exists := c.Get("external_id"); exists {
		userIDStr = val.(string)
	} else if val, exists := c.Get("sub"); exists {
		userIDStr = val.(string)
	} else {
		// Should not happen if authenticated, but fallback
		userIDStr = "anonymous"
	}

	// Create DB record
	file := &models.File{
		OriginalName: fileHeader.Filename,
		FileName:     fileName,
		FilePath:     filePath,
		MimeType:     fileHeader.Header.Get("Content-Type"),
		Size:         fileHeader.Size,
		UploaderID:   userIDStr,
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
	nodeID := c.Query("node_id")
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
	file, err := h.repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if file == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Permission check
	// If user has admin role or file:read scope, allow
	// Otherwise check if uploader_id matches current user
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
}
