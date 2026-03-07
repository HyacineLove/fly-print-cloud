package handlers

import (
	"net/http"

	"fly-print-cloud/api/internal/auth"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"

	"github.com/gin-gonic/gin"
)

// OAuth2ClientHandler OAuth2 客户端管理处理器
type OAuth2ClientHandler struct {
	clientRepo *database.OAuth2ClientRepository
}

// NewOAuth2ClientHandler 创建 OAuth2 客户端管理处理器
func NewOAuth2ClientHandler(clientRepo *database.OAuth2ClientRepository) *OAuth2ClientHandler {
	return &OAuth2ClientHandler{clientRepo: clientRepo}
}

// createOAuth2ClientRequest 创建客户端请求
type createOAuth2ClientRequest struct {
	ClientID      string `json:"client_id" binding:"required"`
	ClientType    string `json:"client_type" binding:"required"`
	AllowedScopes string `json:"allowed_scopes" binding:"required"`
	Description   string `json:"description"`
}

// updateOAuth2ClientRequest 更新客户端请求
type updateOAuth2ClientRequest struct {
	AllowedScopes string `json:"allowed_scopes"`
	Description   string `json:"description"`
	Enabled       *bool  `json:"enabled"`
}

// List 获取客户端列表
func (h *OAuth2ClientHandler) List(c *gin.Context) {
	page, pageSize, offset := ParsePaginationParams(c)
	// OAuth2客户端列表默认使用20条每页
	if c.Query("page_size") == "" {
		pageSize = 20
		offset = (page - 1) * pageSize
	}

	clients, total, err := h.clientRepo.List(offset, pageSize)
	if err != nil {
		InternalErrorResponse(c, "获取客户端列表失败")
		return
	}

	if clients == nil {
		clients = []*models.OAuth2Client{}
	}

	PaginatedSuccessResponse(c, clients, total, page, pageSize)
}

// Create 创建客户端
func (h *OAuth2ClientHandler) Create(c *gin.Context) {
	var req createOAuth2ClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}

	// 检查 client_id 是否已存在
	exists, err := h.clientRepo.ClientIDExists(req.ClientID)
	if err != nil {
		InternalErrorResponse(c, "检查客户端ID失败")
		return
	}
	if exists {
		BadRequestResponse(c, "client_id 已存在")
		return
	}

	// 验证 client_type
	if req.ClientType != "edge_node" && req.ClientType != "third_party" {
		BadRequestResponse(c, "client_type 必须是 edge_node 或 third_party")
		return
	}

	// 生成随机密钥
	rawSecret, err := auth.GenerateClientSecret()
	if err != nil {
		InternalErrorResponse(c, "生成客户端密钥失败")
		return
	}

	// 哈希密钥
	secretHash, err := auth.HashClientSecret(rawSecret)
	if err != nil {
		InternalErrorResponse(c, "哈希客户端密钥失败")
		return
	}

	client := &models.OAuth2Client{
		ClientID:         req.ClientID,
		ClientSecretHash: secretHash,
		ClientType:       req.ClientType,
		AllowedScopes:    req.AllowedScopes,
		Description:      req.Description,
		Enabled:          true,
	}

	if err := h.clientRepo.Create(client); err != nil {
		InternalErrorResponse(c, "创建客户端失败")
		return
	}

	// 返回时包含明文密钥（仅此一次）
	CreatedResponse(c, gin.H{
		"id":             client.ID,
		"client_id":      client.ClientID,
		"client_secret":  rawSecret,
		"client_type":    client.ClientType,
		"allowed_scopes": client.AllowedScopes,
		"description":    client.Description,
		"enabled":        client.Enabled,
		"created_at":     client.CreatedAt,
		"message":        "请保存 client_secret，此密钥仅显示一次",
	})
}

// Get 获取客户端详情
func (h *OAuth2ClientHandler) Get(c *gin.Context) {
	id := c.Param("id")
	client, err := h.clientRepo.GetByID(id)
	if err != nil {
		NotFoundResponse(c, "客户端不存在")
		return
	}

	SuccessResponse(c, client)
}

// Update 更新客户端信息
func (h *OAuth2ClientHandler) Update(c *gin.Context) {
	id := c.Param("id")
	client, err := h.clientRepo.GetByID(id)
	if err != nil {
		NotFoundResponse(c, "客户端不存在")
		return
	}

	var req updateOAuth2ClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}

	if req.AllowedScopes != "" {
		client.AllowedScopes = req.AllowedScopes
	}
	if req.Description != "" {
		client.Description = req.Description
	}
	if req.Enabled != nil {
		client.Enabled = *req.Enabled
	}

	if err := h.clientRepo.Update(client); err != nil {
		InternalErrorResponse(c, "更新客户端失败")
		return
	}

	SuccessResponse(c, client)
}

// ResetSecret 重置客户端密钥
func (h *OAuth2ClientHandler) ResetSecret(c *gin.Context) {
	id := c.Param("id")

	// 确认客户端存在
	_, err := h.clientRepo.GetByID(id)
	if err != nil {
		NotFoundResponse(c, "客户端不存在")
		return
	}

	// 生成新密钥
	rawSecret, err := auth.GenerateClientSecret()
	if err != nil {
		InternalErrorResponse(c, "生成客户端密钥失败")
		return
	}

	secretHash, err := auth.HashClientSecret(rawSecret)
	if err != nil {
		InternalErrorResponse(c, "哈希客户端密钥失败")
		return
	}

	if err := h.clientRepo.UpdateSecret(id, secretHash); err != nil {
		InternalErrorResponse(c, "更新客户端密钥失败")
		return
	}

	SuccessResponse(c, gin.H{
		"client_secret": rawSecret,
		"message":       "密钥已重置，请保存新密钥，此密钥仅显示一次",
	})
}

// Delete 删除客户端
func (h *OAuth2ClientHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.clientRepo.Delete(id); err != nil {
		NotFoundResponse(c, "客户端不存在")
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "删除成功",
	})
}
