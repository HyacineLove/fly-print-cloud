package websocket

import (
	"log"
	"net/http"
	"strings"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/middleware"
	"fly-print-cloud/api/internal/security"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler WebSocket 处理器
type WebSocketHandler struct {
	manager        *ConnectionManager
	printerRepo    *database.PrinterRepository
	edgeNodeRepo   *database.EdgeNodeRepository
	printJobRepo   *database.PrintJobRepository
	fileRepo       *database.FileRepository
	tokenManager   *security.TokenManager
	allowedOrigins []string // 允许的Origin列表
}

// NewWebSocketHandler 创建 WebSocket 处理器
func NewWebSocketHandler(manager *ConnectionManager, printerRepo *database.PrinterRepository, edgeNodeRepo *database.EdgeNodeRepository, printJobRepo *database.PrintJobRepository, fileRepo *database.FileRepository, tokenManager *security.TokenManager, allowedOrigins []string) *WebSocketHandler {
	return &WebSocketHandler{
		manager:        manager,
		printerRepo:    printerRepo,
		edgeNodeRepo:   edgeNodeRepo,
		printJobRepo:   printJobRepo,
		fileRepo:       fileRepo,
		tokenManager:   tokenManager,
		allowedOrigins: allowedOrigins,
	}
}

// HandleConnection 处理 WebSocket 连接升级
func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	// 检查连接数限制
	if h.manager.GetConnectionCount() >= 1000 {
		log.Printf("WebSocket connection limit reached (1000), rejecting new connection")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "too many connections"})
		return
	}

	// 验证 OAuth2 token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		log.Printf("WebSocket connection missing Authorization header: node_id=%s", c.Query("node_id"))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		log.Printf("WebSocket connection invalid Authorization format: node_id=%s", c.Query("node_id"))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
		return
	}

	// 使用导出的 OAuth2 中间件验证 token
	tokenInfo, err := middleware.ValidateOAuth2Token(token)
	if err != nil {
		log.Printf("WebSocket OAuth2 token validation failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// 检查是否有 edge:* scope
	if !middleware.HasRequiredScope(tokenInfo, "edge:heartbeat") {
		log.Printf("WebSocket token missing required scope: edge:heartbeat")
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient scope"})
		return
	}

	// 优先从 query parameter 获取 node_id
	nodeID := c.Query("node_id")
	if nodeID == "" {
		// 如果 URL 参数中没有 node_id，尝试从 token 获取
		nodeID = h.extractNodeIDFromTokenInfo(tokenInfo)
		if nodeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing node_id"})
			return
		}
	}

	log.Printf("WebSocket connection request from node: %s (user: %s)", nodeID, tokenInfo.Sub)

	// 检查节点是否存在（放行禁用节点的连接，允许心跳和状态上报）
	node, err := h.edgeNodeRepo.GetEdgeNodeByID(nodeID)
	if err != nil {
		log.Printf("WebSocket connection rejected: node %s not found: %v", nodeID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "node_not_found", "message": "Edge node not found"})
		return
	}
	// 不再检查节点禁用状态，允许禁用节点建立WebSocket连接以维持监控能力
	if !node.Enabled {
		log.Printf("WebSocket connection accepted for disabled node %s (monitoring only)", nodeID)
	}

	// 配置WebSocket升级器
	upgrader := websocket.Upgrader{
		ReadBufferSize:  10 * 1024 * 1024, // 10MB - 限制单次读取消息大小
		WriteBufferSize: 10 * 1024 * 1024, // 10MB - 限制单次写入消息大小
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")

			// 非浏览器客户端（如边缘节点）可能没有 Origin
			if origin == "" {
				return true
			}

			// 检查Origin是否在允许列表中
			for _, allowed := range h.allowedOrigins {
				if origin == allowed {
					return true
				}
			}

			log.Printf("WebSocket connection rejected: origin not allowed: %s", origin)
			return false
		},
	}

	// 升级 HTTP 连接到 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection for node %s: %v", nodeID, err)
		return
	}

	// 创建连接对象
	connection := NewConnection(nodeID, conn, h.manager, h.printerRepo, h.edgeNodeRepo, h.printJobRepo, h.fileRepo, h.tokenManager)

	// 注册连接
	h.manager.register <- connection

	// 启动读写协程
	go connection.WritePump()
	go connection.ReadPump()

	log.Printf("WebSocket connection established for Edge Node: %s", nodeID)
}

// extractNodeIDFromTokenInfo 从 token 信息中提取 node_id
func (h *WebSocketHandler) extractNodeIDFromTokenInfo(tokenInfo *middleware.OAuth2TokenInfo) string {
	// 对于 Client Credentials Flow，subject 通常是 client_id
	// 可以根据实际需求调整提取逻辑
	if tokenInfo.Sub != "" {
		return tokenInfo.Sub
	}

	// 如果有自定义 claims，也可以从中提取
	// 这里可以根据实际的 token 结构调整
	return ""
}
