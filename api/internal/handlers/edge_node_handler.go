package handlers

import (
	"log"
	"strconv"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EdgeNodeHandler Edge Node 管理处理器
type EdgeNodeHandler struct {
	edgeNodeRepo   *database.EdgeNodeRepository
	printerRepo    *database.PrinterRepository
	wsManager      *websocket.ConnectionManager
	tokenUsageRepo *database.TokenUsageRepository
}

// NewEdgeNodeHandler 创建 Edge Node 管理处理器
func NewEdgeNodeHandler(edgeNodeRepo *database.EdgeNodeRepository, printerRepo *database.PrinterRepository, wsManager *websocket.ConnectionManager, tokenUsageRepo *database.TokenUsageRepository) *EdgeNodeHandler {
	return &EdgeNodeHandler{
		edgeNodeRepo:   edgeNodeRepo,
		printerRepo:    printerRepo,
		wsManager:      wsManager,
		tokenUsageRepo: tokenUsageRepo,
	}
}

// RegisterEdgeNodeRequest Edge Node 注册请求
// node_id 由服务端生成，客户端只需提供节点名称
type RegisterEdgeNodeRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

// UpdateEdgeNodeRequest Edge Node 更新请求
type UpdateEdgeNodeRequest struct {
	Name              string   `json:"name" binding:"required,min=1,max=100"`
	Status            string   `json:"status" binding:"omitempty,oneof=online offline maintenance"`
	Enabled           *bool    `json:"enabled"`  // 使用指针类型以区分未设置和false
	Version           string   `json:"version"`
	Location          string   `json:"location"`
	Latitude          *float64 `json:"latitude"`
	Longitude         *float64 `json:"longitude"`
	IPAddress         *string  `json:"ip_address"`
	MACAddress        string   `json:"mac_address"`
	NetworkInterface  string   `json:"network_interface"`
	OSVersion         string   `json:"os_version"`
	CPUInfo           string   `json:"cpu_info"`
	MemoryInfo        string   `json:"memory_info"`
	DiskInfo          string   `json:"disk_info"`
	ConnectionQuality string   `json:"connection_quality"`
	Latency           int      `json:"latency"`
}


// EdgeNodeInfo Edge Node 信息响应
type EdgeNodeInfo struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Status            string    `json:"status"`
	Enabled           bool      `json:"enabled"`
	Version           string    `json:"version"`
	LastHeartbeat     time.Time `json:"last_heartbeat"`
	Location          string    `json:"location"`
	Latitude          *float64  `json:"latitude"`
	Longitude         *float64  `json:"longitude"`
	IPAddress         *string   `json:"ip_address,omitempty"`
	MACAddress        string    `json:"mac_address"`
	NetworkInterface  string    `json:"network_interface"`
	OSVersion         string    `json:"os_version"`
	CPUInfo           string    `json:"cpu_info"`
	MemoryInfo        string    `json:"memory_info"`
	DiskInfo          string    `json:"disk_info"`
	ConnectionQuality string    `json:"connection_quality"`
	Latency           int       `json:"latency"`
	PrinterCount      int       `json:"printer_count"`    // 管理的打印机数量
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RegisterEdgeNode 注册 Edge Node
// 服务端生成 node_id (UUID)，状态默认为 offline
// Edge 端需保存返回的 node_id，避免重复注册
func (h *EdgeNodeHandler) RegisterEdgeNode(c *gin.Context) {
	var req RegisterEdgeNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}

	// 创建 Edge Node，服务端生成 UUID 作为 node_id
	node := &models.EdgeNode{
		ID:            uuid.New().String(), // 服务端生成 UUID
		Name:          req.Name,
		Status:        "offline", // 默认为 offline，通过 WebSocket 心跳变为 online
		Enabled:       true,       // 注册完成后默认启用节点
		LastHeartbeat: time.Now(),
	}

	if err := h.edgeNodeRepo.CreateEdgeNode(node); err != nil {
		log.Printf("Failed to register edge node: %v", err)
		InternalErrorResponse(c, "注册 Edge Node 失败")
		return
	}

	// 返回节点信息
	nodeInfo := EdgeNodeInfo{
		ID:                node.ID,
		Name:              node.Name,
		Status:            node.Status,
		Enabled:           node.Enabled,
		Version:           node.Version,
		LastHeartbeat:     node.LastHeartbeat,
		Location:          node.Location,
		Latitude:          node.Latitude,
		Longitude:         node.Longitude,
		IPAddress:         node.IPAddress,
		MACAddress:        node.MACAddress,
		NetworkInterface:  node.NetworkInterface,
		OSVersion:         node.OSVersion,
		CPUInfo:           node.CPUInfo,
		MemoryInfo:        node.MemoryInfo,
		DiskInfo:          node.DiskInfo,
		ConnectionQuality: node.ConnectionQuality,
		Latency:           node.Latency,
		CreatedAt:         node.CreatedAt,
		UpdatedAt:         node.UpdatedAt,
	}

	log.Printf("Edge Node %s registered successfully", node.Name)
	CreatedResponse(c, nodeInfo)
}

// ListEdgeNodes 获取 Edge Node 列表
// 支持分页、状态过滤、排序和搜索
func (h *EdgeNodeHandler) ListEdgeNodes(c *gin.Context) {
	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	
	// 获取排序参数
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	
	// 获取搜索参数
	search := c.Query("search")

	// 验证排序字段
	validSortFields := map[string]bool{
		"last_heartbeat": true,
		"printer_count":  true,
		"created_at":     true,
		"name":           true,
	}
	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}
	
	// 验证排序方向
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize

	// 可选：检查并更新超时的节点状态（3分钟超时）
	if updatedCount, err := h.edgeNodeRepo.CheckAndUpdateOfflineNodes(3); err != nil {
		log.Printf("⚠️ [DEBUG] Failed to check offline nodes: %v", err)
	} else if updatedCount > 0 {
		log.Printf("📱 [DEBUG] Updated %d nodes to offline status", updatedCount)
	}

	// 查询 Edge Node 列表
	log.Printf("🔍 [DEBUG] 查询Edge Nodes: offset=%d, pageSize=%d, status='%s', sortBy='%s', sortOrder='%s', search='%s'", 
		offset, pageSize, status, sortBy, sortOrder, search)
	nodes, total, err := h.edgeNodeRepo.ListEdgeNodes(offset, pageSize, status, sortBy, sortOrder, search)
	if err != nil {
		log.Printf("❌ [DEBUG] Failed to list edge nodes: %v", err)
		InternalErrorResponse(c, "获取 Edge Node 列表失败")
		return
	}
	log.Printf("📊 [DEBUG] 查询结果: 找到 %d 个节点，总数 %d", len(nodes), total)

	// 转换为响应格式
	nodeInfos := make([]EdgeNodeInfo, len(nodes))
	for i, node := range nodes {
		// 获取该边缘节点管理的打印机数量
		printerCount, err := h.printerRepo.CountPrintersByEdgeNode(node.ID)
		if err != nil {
			log.Printf("⚠️ [DEBUG] Failed to get printer count for edge node %s: %v", node.ID, err)
			printerCount = 0 // 如果查询失败，设置为0
		}
		
		nodeInfos[i] = EdgeNodeInfo{
			ID:                node.ID,
			Name:              node.Name,
			Status:            node.Status,
			Enabled:           node.Enabled,
			Version:           node.Version,
			LastHeartbeat:     node.LastHeartbeat,
			Location:          node.Location,
			Latitude:          node.Latitude,
			Longitude:         node.Longitude,
			IPAddress:         node.IPAddress,
			MACAddress:        node.MACAddress,
			NetworkInterface:  node.NetworkInterface,
			OSVersion:         node.OSVersion,
			CPUInfo:           node.CPUInfo,
			MemoryInfo:        node.MemoryInfo,
			DiskInfo:          node.DiskInfo,
			ConnectionQuality: node.ConnectionQuality,
			Latency:           node.Latency,
			PrinterCount:      printerCount,
			CreatedAt:         node.CreatedAt,
			UpdatedAt:         node.UpdatedAt,
		}
	}

	PaginatedSuccessResponse(c, nodeInfos, total, page, pageSize)
}

// GetEdgeNode 获取 Edge Node 详情
func (h *EdgeNodeHandler) GetEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		BadRequestResponse(c, "Edge Node ID 不能为空")
		return
	}

	node, err := h.edgeNodeRepo.GetEdgeNodeByID(nodeID)
	if err != nil {
		NotFoundResponse(c, "Edge Node 不存在")
		return
	}

	// 获取该边缘节点管理的打印机数量
	printerCount, err := h.printerRepo.CountPrintersByEdgeNode(node.ID)
	if err != nil {
		log.Printf("⚠️ [DEBUG] Failed to get printer count for edge node %s: %v", node.ID, err)
		printerCount = 0 // 如果查询失败，设置为0
	}

	nodeInfo := EdgeNodeInfo{
		ID:                node.ID,
		Name:              node.Name,
		Status:            node.Status,
		Enabled:           node.Enabled,
		Version:           node.Version,
		LastHeartbeat:     node.LastHeartbeat,
		Location:          node.Location,
		Latitude:          node.Latitude,
		Longitude:         node.Longitude,
		IPAddress:         node.IPAddress,
		MACAddress:        node.MACAddress,
		NetworkInterface:  node.NetworkInterface,
		OSVersion:         node.OSVersion,
		CPUInfo:           node.CPUInfo,
		MemoryInfo:        node.MemoryInfo,
		DiskInfo:          node.DiskInfo,
		ConnectionQuality: node.ConnectionQuality,
		Latency:           node.Latency,
		PrinterCount:      printerCount,
		CreatedAt:         node.CreatedAt,
		UpdatedAt:         node.UpdatedAt,
	}

	SuccessResponse(c, nodeInfo)
}

// UpdateEdgeNode 更新 Edge Node
func (h *EdgeNodeHandler) UpdateEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		BadRequestResponse(c, "Edge Node ID 不能为空")
		return
	}

	var req UpdateEdgeNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}

	// 检查节点是否存在
	node, err := h.edgeNodeRepo.GetEdgeNodeByID(nodeID)
	if err != nil {
		NotFoundResponse(c, "Edge Node 不存在")
		return
	}

	// 更新节点信息
	node.Name = req.Name
	
	// 只有当Status字段不为空时才更新
	if req.Status != "" {
		node.Status = req.Status
	}
	
	// 处理Enabled字段更新（逻辑级联，不修改printer的enable状态）
	oldEnabled := node.Enabled
	if req.Enabled != nil {
		node.Enabled = *req.Enabled
	}
	
	node.Version = req.Version
	node.Location = req.Location
	node.Latitude = req.Latitude
	node.Longitude = req.Longitude
	node.IPAddress = req.IPAddress
	node.MACAddress = req.MACAddress
	node.NetworkInterface = req.NetworkInterface
	node.OSVersion = req.OSVersion
	node.CPUInfo = req.CPUInfo
	node.MemoryInfo = req.MemoryInfo
	node.DiskInfo = req.DiskInfo
	node.ConnectionQuality = req.ConnectionQuality
	node.Latency = req.Latency

	if err := h.edgeNodeRepo.UpdateEdgeNode(node); err != nil {
		log.Printf("Failed to update edge node %s: %v", nodeID, err)
		InternalErrorResponse(c, "更新 Edge Node 失败")
		return
	}

	// 当节点从启用变为禁用时，撤销该节点的所有上传Token
	if oldEnabled && !node.Enabled && h.tokenUsageRepo != nil {
		if revoked, err := h.tokenUsageRepo.RevokeTokensByNodeAndType("upload", node.ID); err != nil {
			log.Printf("Warning: failed to revoke upload tokens for disabled node %s: %v", node.ID, err)
		} else if revoked > 0 {
			log.Printf("Revoked %d upload tokens for disabled node %s", revoked, node.ID)
		}
	}

	// 不再向 Edge 端发送 node_state 消息
	// 禁用节点的请求将在云端通过中间件拦截

	nodeInfo := EdgeNodeInfo{
		ID:                node.ID,
		Name:              node.Name,
		Status:            node.Status,
		Enabled:           node.Enabled,
		Version:           node.Version,
		LastHeartbeat:     node.LastHeartbeat,
		Location:          node.Location,
		Latitude:          node.Latitude,
		Longitude:         node.Longitude,
		IPAddress:         node.IPAddress,
		MACAddress:        node.MACAddress,
		NetworkInterface:  node.NetworkInterface,
		OSVersion:         node.OSVersion,
		CPUInfo:           node.CPUInfo,
		MemoryInfo:        node.MemoryInfo,
		DiskInfo:          node.DiskInfo,
		ConnectionQuality: node.ConnectionQuality,
		Latency:           node.Latency,
		CreatedAt:         node.CreatedAt,
		UpdatedAt:         node.UpdatedAt,
	}

	log.Printf("Edge Node %s updated successfully", node.Name)
	SuccessResponse(c, nodeInfo)
}

// DeleteEdgeNode 删除 Edge Node
func (h *EdgeNodeHandler) DeleteEdgeNode(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		BadRequestResponse(c, "Edge Node ID 不能为空")
		return
	}

	// 检查节点是否存在
	_, err := h.edgeNodeRepo.GetEdgeNodeByID(nodeID)
	if err != nil {
		NotFoundResponse(c, "Edge Node 不存在")
		return
	}

	// 删除节点（软删除）
	if err := h.edgeNodeRepo.DeleteEdgeNode(nodeID); err != nil {
		log.Printf("Failed to delete edge node %s: %v", nodeID, err)
		InternalErrorResponse(c, "删除 Edge Node 失败")
		return
	}

	// 级联删除该节点下的所有打印机（硬删除）
	// 由于节点是软删除，数据库的 ON DELETE CASCADE 不会触发，需要手动删除打印机
	if h.printerRepo != nil {
		if err := h.printerRepo.DeletePrintersByEdgeNode(nodeID); err != nil {
			log.Printf("Warning: failed to delete printers for node %s: %v", nodeID, err)
			// 不中断流程，继续关闭 WebSocket 连接
		} else {
			log.Printf("Successfully deleted all printers for node %s", nodeID)
		}
	}

	// 关闭该节点的 WebSocket 连接（如果存在）
	if h.wsManager != nil {
		if err := h.wsManager.DisconnectNode(nodeID); err != nil {
			if err.Error() != "edge node not connected" {
				log.Printf("Warning: failed to disconnect WebSocket for deleted node %s: %v", nodeID, err)
			}
		} else {
			log.Printf("WebSocket connection closed for deleted node %s", nodeID)
		}
	}

	log.Printf("Edge Node %s deleted successfully", nodeID)
	SuccessResponse(c, gin.H{"message": "Edge Node 删除成功"})
}

