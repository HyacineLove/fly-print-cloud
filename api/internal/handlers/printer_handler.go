package handlers

import (
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/websocket"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PrinterHandler struct {
	printerRepo   *database.PrinterRepository
	edgeNodeRepo  *database.EdgeNodeRepository
	wsManager     *websocket.ConnectionManager
	tokenUsageRepo *database.TokenUsageRepository
}

func NewPrinterHandler(printerRepo *database.PrinterRepository, edgeNodeRepo *database.EdgeNodeRepository, wsManager *websocket.ConnectionManager, tokenUsageRepo *database.TokenUsageRepository) *PrinterHandler {
	return &PrinterHandler{
		printerRepo:   printerRepo,
		edgeNodeRepo:  edgeNodeRepo,
		wsManager:     wsManager,
		tokenUsageRepo: tokenUsageRepo,
	}
}



// UpdatePrinterRequest 更新打印机请求（Edge Node使用）
type UpdatePrinterRequest struct {
	Name            string                        `json:"name" binding:"required,min=1,max=100"`
	Model           string                        `json:"model"`
	SerialNumber    string                        `json:"serial_number"`
	Status          string                        `json:"status" binding:"required,oneof=ready printing error offline"`
	FirmwareVersion string                        `json:"firmware_version"`
	PortInfo        string                        `json:"port_info"`
	IPAddress       *string                       `json:"ip_address"`
	MACAddress      string                        `json:"mac_address"`
	NetworkConfig   string                        `json:"network_config"`
	Latitude        *float64                      `json:"latitude"`
	Longitude       *float64                      `json:"longitude"`
	Location        string                        `json:"location"`
	Capabilities    models.PrinterCapabilities    `json:"capabilities"`
	QueueLength     int                           `json:"queue_length"`
}

// AdminUpdatePrinterRequest 管理界面更新打印机请求
type AdminUpdatePrinterRequest struct {
	DisplayName string `json:"display_name" binding:"omitempty,max=100"`
	Enabled     *bool  `json:"enabled"`  // 使用指针类型以区分未设置和false
}

// PrinterWithStatus 包含实际状态的打印机信息
type PrinterWithStatus struct {
	*models.Printer
	EdgeNodeEnabled bool   `json:"edge_node_enabled"`
	ActuallyEnabled bool   `json:"actually_enabled"`
	DisabledReason  string `json:"disabled_reason,omitempty"`
}

// NewPrinterWithStatus 创建包含实际状态的打印机信息
func NewPrinterWithStatus(printer *models.Printer, edgeNodeEnabled bool) *PrinterWithStatus {
	actuallyEnabled := printer.Enabled && edgeNodeEnabled
	
	var disabledReason string
	if !actuallyEnabled {
		if !printer.Enabled {
			disabledReason = "打印机被禁用"
		} else if !edgeNodeEnabled {
			disabledReason = "Edge Node被禁用"
		}
	}
	
	return &PrinterWithStatus{
		Printer:         printer,
		EdgeNodeEnabled: edgeNodeEnabled,
		ActuallyEnabled: actuallyEnabled,
		DisabledReason:  disabledReason,
	}
}

// Edge 注册打印机请求（简化版）
type EdgeRegisterPrinterRequest struct {
	Name            string                        `json:"name" binding:"required,min=1,max=100"`
	Model           string                        `json:"model"`
	SerialNumber    string                        `json:"serial_number"`
	FirmwareVersion string                        `json:"firmware_version"`
	PortInfo        string                        `json:"port_info"`
	IPAddress       *string                       `json:"ip_address"`
	MACAddress      string                        `json:"mac_address"`
	Capabilities    models.PrinterCapabilities    `json:"capabilities"`
}

// 管理员 API

// ListPrinters 获取所有打印机列表（管理员）
// 支持分页、按Edge Node筛选、按状态筛选、按名称搜索
func (h *PrinterHandler) ListPrinters(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	edgeNodeID := c.Query("edge_node_id") // 支持按Edge Node筛选
	status := c.Query("status")           // 支持按状态筛选
	search := c.Query("search")           // 支持按名称/别名/ID搜索

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	var printers []*models.Printer
	var total int
	var err error

	if edgeNodeID != "" {
		// 按Edge Node筛选
		printers, err = h.printerRepo.ListPrintersByEdgeNode(edgeNodeID)
		if err != nil {
			log.Printf("Failed to list printers by edge node: %v", err)
			InternalErrorResponse(c, "获取打印机列表失败")
			return
		}
		total = len(printers)
	} else {
		// 获取所有打印机
		printers, total, err = h.printerRepo.ListPrinters(page, pageSize)
		if err != nil {
			log.Printf("Failed to list printers: %v", err)
			InternalErrorResponse(c, "获取打印机列表失败")
			return
		}
	}

	// 应用状态筛选（后端筛选）
	if status != "" {
		filtered := make([]*models.Printer, 0)
		for _, printer := range printers {
			if printer.Status == status {
				filtered = append(filtered, printer)
			}
		}
		printers = filtered
		total = len(printers)
	}

	// 应用搜索筛选（按名称、别名、ID搜索，后端筛选）
	if search != "" {
		searchLower := strings.ToLower(search)
		filtered := make([]*models.Printer, 0)
		for _, printer := range printers {
			// 搜索名称
			if strings.Contains(strings.ToLower(printer.Name), searchLower) {
				filtered = append(filtered, printer)
				continue
			}
			// 搜索别名
			if printer.DisplayName != "" && strings.Contains(strings.ToLower(printer.DisplayName), searchLower) {
				filtered = append(filtered, printer)
				continue
			}
			// 搜索ID
			if strings.Contains(strings.ToLower(printer.ID), searchLower) {
				filtered = append(filtered, printer)
				continue
			}
		}
		printers = filtered
		total = len(printers)
	}

	// 获取所有相关的Edge Node状态信息
	edgeNodeStatusMap := make(map[string]bool)
	for _, printer := range printers {
		if _, exists := edgeNodeStatusMap[printer.EdgeNodeID]; !exists {
			edgeNode, err := h.edgeNodeRepo.GetEdgeNodeByID(printer.EdgeNodeID)
			if err != nil {
				log.Printf("Failed to get edge node %s: %v", printer.EdgeNodeID, err)
				edgeNodeStatusMap[printer.EdgeNodeID] = false // 默认为禁用
			} else {
				edgeNodeStatusMap[printer.EdgeNodeID] = edgeNode.Enabled
			}
		}
	}

	// 转换为包含实际状态的打印机信息
	printersWithStatus := make([]*PrinterWithStatus, len(printers))
	for i, printer := range printers {
		edgeNodeEnabled := edgeNodeStatusMap[printer.EdgeNodeID]
		printersWithStatus[i] = NewPrinterWithStatus(printer, edgeNodeEnabled)
	}

	totalPages := (total + pageSize - 1) / pageSize
	response := gin.H{
		"items":       printersWithStatus,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	}

	SuccessResponse(c, response)
}

// GetPrinter 获取打印机详情
func (h *PrinterHandler) GetPrinter(c *gin.Context) {
	printerID := c.Param("id")
	if printerID == "" {
		BadRequestResponse(c, "打印机 ID 不能为空")
		return
	}

	printer, err := h.printerRepo.GetPrinterByID(printerID)
	if err != nil {
		NotFoundResponse(c, "打印机不存在")
		return
	}

	// 获取Edge Node状态
	edgeNode, err := h.edgeNodeRepo.GetEdgeNodeByID(printer.EdgeNodeID)
	if err != nil {
		log.Printf("Failed to get edge node %s: %v", printer.EdgeNodeID, err)
		// 如果无法获取Edge Node状态，假设为禁用
		printerWithStatus := NewPrinterWithStatus(printer, false)
		SuccessResponse(c, printerWithStatus)
		return
	}

	printerWithStatus := NewPrinterWithStatus(printer, edgeNode.Enabled)
	SuccessResponse(c, printerWithStatus)
}

// UpdatePrinter 更新打印机（管理员）
func (h *PrinterHandler) UpdatePrinter(c *gin.Context) {
	printerID := c.Param("id")
	if printerID == "" {
		BadRequestResponse(c, "打印机 ID 不能为空")
		return
	}

	// 检查打印机是否存在
	printer, err := h.printerRepo.GetPrinterByID(printerID)
	if err != nil {
		NotFoundResponse(c, "打印机不存在")
		return
	}

	// 尝试解析为管理界面的简单更新请求
	var adminReq AdminUpdatePrinterRequest
	if err := c.ShouldBindJSON(&adminReq); err == nil {
		// 管理界面更新（仅更新display_name和enabled）
		oldEnabled := printer.Enabled
		if adminReq.DisplayName != "" {
			printer.DisplayName = adminReq.DisplayName
		}
		if adminReq.Enabled != nil {
			printer.Enabled = *adminReq.Enabled
		}

		// 当打印机从启用变为禁用时，撤销该节点上此打印机的所有上传Token
		if oldEnabled && !printer.Enabled && h.tokenUsageRepo != nil {
			if revoked, err := h.tokenUsageRepo.RevokeTokensByNodeAndResource("upload", printer.EdgeNodeID, printer.ID); err != nil {
				log.Printf("Warning: failed to revoke upload tokens for disabled printer %s on node %s: %v", printer.ID, printer.EdgeNodeID, err)
			} else if revoked > 0 {
				log.Printf("Revoked %d upload tokens for disabled printer %s on node %s", revoked, printer.ID, printer.EdgeNodeID)
			}
		}
	} else {
		// 尝试解析为Edge Node的完整更新请求
		var req UpdatePrinterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			BadRequestResponse(c, "请求参数无效")
			return
		}

		// 更新所有打印机信息
		printer.Name = req.Name
		printer.Model = req.Model
		printer.SerialNumber = req.SerialNumber
		printer.Status = req.Status
		printer.FirmwareVersion = req.FirmwareVersion
		printer.PortInfo = req.PortInfo
		printer.IPAddress = req.IPAddress
		printer.MACAddress = req.MACAddress
		printer.NetworkConfig = req.NetworkConfig
		printer.Latitude = req.Latitude
		printer.Longitude = req.Longitude
		printer.Location = req.Location
		printer.Capabilities = req.Capabilities
		printer.QueueLength = req.QueueLength
	}

	if err := h.printerRepo.UpdatePrinter(printer); err != nil {
		log.Printf("Failed to update printer %s: %v", printerID, err)
		InternalErrorResponse(c, "更新打印机失败")
		return
	}

	// 功能 3.2.3: 移除 printer_state WebSocket 消息
	// 打印机启用/禁用状态变更不再通过 WebSocket 通知 Edge 端
	// Edge 端请求时会通过 API 错误码感知打印机状态

	log.Printf("Printer %s updated successfully", printer.Name)
	SuccessResponse(c, printer)
}

// DeletePrinter 删除打印机
func (h *PrinterHandler) DeletePrinter(c *gin.Context) {
	printerID := c.Param("id")
	if printerID == "" {
		BadRequestResponse(c, "打印机 ID 不能为空")
		return
	}

	// 检查打印机是否存在
	_, err := h.printerRepo.GetPrinterByID(printerID)
	if err != nil {
		NotFoundResponse(c, "打印机不存在")
		return
	}

	// 删除打印机
	if err := h.printerRepo.DeletePrinter(printerID); err != nil {
		log.Printf("Failed to delete printer %s: %v", printerID, err)
		InternalErrorResponse(c, "删除打印机失败")
		return
	}

	// 功能 3.2.4: 移除 printer_deleted WebSocket 消息
	// 打印机删除不再通过 WebSocket 通知 Edge 端
	// Edge 端请求时会收到 printer_not_found 错误，然后触发重新注册

	log.Printf("Printer %s deleted successfully", printerID)
	SuccessResponse(c, gin.H{"message": "打印机删除成功"})
}

// Edge Node API

// EdgeRegisterPrinter Edge Node 注册打印机
// 优化：云端生成唯一ID，如果打印机已存在则返回原有ID
func (h *PrinterHandler) EdgeRegisterPrinter(c *gin.Context) {
	edgeNodeID := c.Param("node_id")
	if edgeNodeID == "" {
		BadRequestResponse(c, "Edge Node ID 不能为空")
		return
	}

	var req EdgeRegisterPrinterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestResponse(c, "请求参数无效")
		return
	}

	// 验证 Edge Node 是否存在
	_, err := h.edgeNodeRepo.GetEdgeNodeByID(edgeNodeID)
	if err != nil {
		BadRequestResponse(c, "Edge Node 不存在")
		return
	}

	// 检查是否已存在相同名称的打印机
	existingPrinter, err := h.printerRepo.GetPrinterByNameAndEdgeNode(req.Name, edgeNodeID)
	isNew := false
	
	var printer *models.Printer
	if err == nil && existingPrinter != nil {
		// 打印机已存在，更新信息但保持原ID
		existingPrinter.Model = req.Model
		existingPrinter.SerialNumber = req.SerialNumber
		existingPrinter.FirmwareVersion = req.FirmwareVersion
		existingPrinter.PortInfo = req.PortInfo
		existingPrinter.IPAddress = req.IPAddress
		existingPrinter.MACAddress = req.MACAddress
		existingPrinter.Capabilities = req.Capabilities
		
		if err := h.printerRepo.UpdatePrinter(existingPrinter); err != nil {
			log.Printf("Failed to update existing printer %s: %v", existingPrinter.ID, err)
			InternalErrorResponse(c, "更新打印机失败")
			return
		}
		printer = existingPrinter
		log.Printf("Printer %s updated by edge node %s (ID: %s)", printer.Name, edgeNodeID, printer.ID)
	} else {
		// 打印机不存在，创建新记录
		isNew = true
		printer = &models.Printer{
			ID:              uuid.New().String(),
			Name:            req.Name,
			Model:           req.Model,
			SerialNumber:    req.SerialNumber,
			Status:          "offline", // 默认状态
			Enabled:         true,      // 默认启用
			FirmwareVersion: req.FirmwareVersion,
			PortInfo:        req.PortInfo,
			IPAddress:       req.IPAddress,
			MACAddress:      req.MACAddress,
			NetworkConfig:   "",
			Capabilities:    req.Capabilities,
			EdgeNodeID:      edgeNodeID,
			QueueLength:     0,
		}

		if err := h.printerRepo.CreatePrinter(printer); err != nil {
			log.Printf("Failed to create printer by edge node %s: %v", edgeNodeID, err)
			InternalErrorResponse(c, "注册打印机失败")
			return
		}
		log.Printf("Printer %s registered by edge node %s (ID: %s)", printer.Name, edgeNodeID, printer.ID)
	}

	// 返回响应，包含打印机ID和是否为新注册
	CreatedResponse(c, gin.H{
		"id":       printer.ID,
		"name":     printer.Name,
		"is_new":   isNew,
		"printer":  printer,
	})
}

func (h *PrinterHandler) EdgeDeletePrinter(c *gin.Context) {
	edgeNodeID := c.Param("node_id")
	printerID := c.Param("printer_id")
	if edgeNodeID == "" || printerID == "" {
		BadRequestResponse(c, "参数不能为空")
		return
	}
	_, err := h.edgeNodeRepo.GetEdgeNodeByID(edgeNodeID)
	if err != nil {
		BadRequestResponse(c, "Edge Node 不存在")
		return
	}
	if err := h.printerRepo.DeletePrinterByEdgeNode(printerID, edgeNodeID); err != nil {
		NotFoundResponse(c, "打印机不存在")
		return
	}
	SuccessResponse(c, gin.H{"message": "打印机删除成功"})
}

// ========== 功能 3.2.2: 批量状态上报 API ==========

// PrinterStatusItem 单个打印机状态项
type PrinterStatusItem struct {
	PrinterID   string `json:"printer_id" binding:"required"`
	Status      string `json:"status" binding:"required,oneof=ready printing error offline"`
	QueueLength int    `json:"queue_length"`
}

// EdgeBatchStatusRequest 批量状态上报请求
type EdgeBatchStatusRequest struct {
	Printers []PrinterStatusItem `json:"printers" binding:"required,min=1,max=100"`
}

// PrinterStatusError 单个打印机状态更新错误
type PrinterStatusError struct {
	PrinterID string `json:"printer_id"`
	Error     string `json:"error"`
}

// EdgeBatchUpdatePrinterStatus Edge Node 批量上报打印机状态
func (h *PrinterHandler) EdgeBatchUpdatePrinterStatus(c *gin.Context) {
	edgeNodeID := c.Param("node_id")
	if edgeNodeID == "" {
		BadRequestResponse(c, "Edge Node ID 不能为空")
		return
	}

	var req EdgeBatchStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequestResponse(c, "请求参数无效")
		return
	}

	// 验证 Edge Node 是否存在
	_, err := h.edgeNodeRepo.GetEdgeNodeByID(edgeNodeID)
	if err != nil {
		BadRequestResponse(c, "Edge Node 不存在")
		return
	}

	var updated int
	var failed int
	var errors []PrinterStatusError

	for _, item := range req.Printers {
		// 获取打印机信息
		printer, err := h.printerRepo.GetPrinterByID(item.PrinterID)
		if err != nil {
			failed++
			errors = append(errors, PrinterStatusError{
				PrinterID: item.PrinterID,
				Error:     "printer_not_found",
			})
			continue
		}

		// 验证打印机属于该 Edge Node
		if printer.EdgeNodeID != edgeNodeID {
			failed++
			errors = append(errors, PrinterStatusError{
				PrinterID: item.PrinterID,
				Error:     "printer_not_belong_to_node",
			})
			continue
		}

		// 功能 3.2.3: 放行禁用打印机的状态上报请求
		// 禁用的打印机仍然可以上报状态，以便监控其实际运行状态
		// 节点禁用检查由中间件 EdgeNodeEnabledCheck 处理

		// 更新打印机状态
		printer.Status = item.Status
		printer.QueueLength = item.QueueLength

		if err := h.printerRepo.UpdatePrinter(printer); err != nil {
			log.Printf("Failed to update printer %s status: %v", item.PrinterID, err)
			failed++
			errors = append(errors, PrinterStatusError{
				PrinterID: item.PrinterID,
				Error:     "update_failed",
			})
			continue
		}

		updated++
	}

	log.Printf("Batch status update from edge node %s: %d updated, %d failed", edgeNodeID, updated, failed)
	SuccessResponse(c, gin.H{
		"updated": updated,
		"failed":  failed,
		"errors":  errors,
	})
}
