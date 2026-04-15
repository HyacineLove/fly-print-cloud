package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/logger"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// 写入等待时间
	writeWait = 10 * time.Second

	// Pong 等待时间
	pongWait = 60 * time.Second

	// Ping 发送间隔
	pingPeriod = (pongWait * 9) / 10

	// 最大消息大小
	maxMessageSize = 8192

	// ACK 等待超时时间
	ackTimeout = 5 * time.Second
)

// Connection 表示单个 WebSocket 连接
type Connection struct {
	NodeID       string
	Conn         *websocket.Conn
	Send         chan []byte
	Manager      *ConnectionManager
	PrinterRepo  *database.PrinterRepository
	EdgeNodeRepo *database.EdgeNodeRepository
	PrintJobRepo *database.PrintJobRepository
	FileRepo     *database.FileRepository
	TokenManager *security.TokenManager

	// ACK 机制相关
	pendingAcks map[string]chan struct{}
	ackMutex    sync.Mutex
}

// NewConnection 创建新连接
func NewConnection(nodeID string, conn *websocket.Conn, manager *ConnectionManager, printerRepo *database.PrinterRepository, edgeNodeRepo *database.EdgeNodeRepository, printJobRepo *database.PrintJobRepository, fileRepo *database.FileRepository, tokenManager *security.TokenManager) *Connection {
	return &Connection{
		NodeID:       nodeID,
		Conn:         conn,
		Send:         make(chan []byte, 256),
		Manager:      manager,
		PrinterRepo:  printerRepo,
		EdgeNodeRepo: edgeNodeRepo,
		PrintJobRepo: printJobRepo,
		FileRepo:     fileRepo,
		TokenManager: tokenManager,
		pendingAcks:  make(map[string]chan struct{}),
	}
}

// ReadPump 处理从客户端读取消息
func (c *Connection) ReadPump() {
	defer func() {
		c.Manager.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			logger.Error("WebSocket read error", zap.String("node_id", c.NodeID), zap.Error(err))
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket unexpected close error", zap.String("node_id", c.NodeID), zap.Error(err))
			}
			break
		}

		logger.Debug("WebSocket received raw message", zap.String("node_id", c.NodeID), zap.String("message", string(messageBytes)))

		// 先解析消息类型
		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			logger.Error("Failed to parse message from node", zap.String("node_id", c.NodeID), zap.Error(err))
			continue
		}

		logger.Debug("WebSocket parsed message from node", zap.String("node_id", c.NodeID), zap.String("type", msg.Type))

		// ACK 消息特殊处理：msg_id 在顶层而非 data 中
		if msg.Type == MsgTypeAck {
			var ack CommandAck
			if err := json.Unmarshal(messageBytes, &ack); err != nil {
				logger.Error("Failed to parse ACK message from node", zap.String("node_id", c.NodeID), zap.Error(err))
				continue
			}
			c.handleAckDirect(&ack)
		} else {
			// 其他消息正常处理
			c.handleMessage(&msg)
		}
	}
}

// WritePump 处理向客户端发送消息
func (c *Connection) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送队列中的其他消息
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理接收到的消息
func (c *Connection) handleMessage(msg *Message) {
	logger.Debug("Received message from node", zap.String("node_id", c.NodeID), zap.String("type", msg.Type))

	// 标准验证顺序（所有消息统一处理）：
	// 1. 节点存在性检查（所有消息都需要，包括 heartbeat 和 job_update）
	// 2. 节点启用检查（heartbeat 和 job_update 放行，允许禁用节点维持连接和完成任务收尾）

	// 步骤1: 检查节点是否存在（所有消息都需要）
	node, err := c.EdgeNodeRepo.GetEdgeNodeByID(c.NodeID)
	if err != nil || node == nil {
		logger.Warn("Message rejected: node not found", zap.String("node_id", c.NodeID), zap.String("message_type", msg.Type))
		c.sendError("node_not_found", "Edge node not found", "")
		return
	}

	// 步骤2: 检查节点是否启用（heartbeat 和 job_update 放行）
	if msg.Type != MsgTypeHeartbeat && msg.Type != MsgTypeJobUpdate {
		if !node.Enabled {
			logger.Warn("Message rejected: node is disabled", zap.String("node_id", c.NodeID), zap.String("message_type", msg.Type))
			c.sendError("node_disabled", "Edge node has been disabled by administrator", "")
			return
		}
	}

	switch msg.Type {
	case MsgTypeHeartbeat:
		c.handleHeartbeat(msg)
	case MsgTypeJobUpdate:
		c.handleJobUpdate(msg)
	case MsgTypeSubmitPrintParams:
		c.handleSubmitPrintParams(msg)
	case MsgTypeRequestUploadToken:
		c.handleRequestUploadToken(msg)
	case MsgTypeAck:
		c.handleAck(msg)
	default:
		logger.Warn("Unknown message type from node", zap.String("type", msg.Type), zap.String("node_id", c.NodeID))
	}
}

// handleAck 处理确认消息（已废弃，使用 handleAckDirect）
func (c *Connection) handleAck(msg *Message) {
	logger.Warn("handleAck called via handleMessage (deprecated path)", zap.String("node_id", c.NodeID))
	// 这个路径已被 ReadPump 中的直接解析替代，不应该到达这里
}

// handleAckDirect 直接处理已解析的 ACK 消息
func (c *Connection) handleAckDirect(ack *CommandAck) {
	if ack.MsgID == "" {
		logger.Warn("Received ACK without MsgID from node", zap.String("node_id", c.NodeID))
		return
	}

	c.ackMutex.Lock()
	defer c.ackMutex.Unlock()

	if ch, exists := c.pendingAcks[ack.MsgID]; exists {
		close(ch) // 关闭 channel 通知等待者
		delete(c.pendingAcks, ack.MsgID)
		logger.Debug("Received ACK for message from node", zap.String("msg_id", ack.MsgID), zap.String("node_id", c.NodeID), zap.String("command_id", ack.CommandID), zap.String("status", ack.Status))
	} else {
		logger.Warn("Received ACK for unknown or timed-out message", zap.String("msg_id", ack.MsgID), zap.String("node_id", c.NodeID))
	}
}

// SendCommandWithAck 发送指令并等待确认
func (c *Connection) SendCommandWithAck(cmd *Command, timeout time.Duration) error {
	if cmd.MsgID == "" {
		cmd.MsgID = uuid.New().String()
	}

	ackCh := make(chan struct{})

	c.ackMutex.Lock()
	c.pendingAcks[cmd.MsgID] = ackCh
	c.ackMutex.Unlock()

	// 无论成功与否，都要确保从 map 中移除（避免内存泄漏）
	// 注意：如果成功收到 ACK，handleAck 会删除 map entry。
	// 这里主要是处理超时或发送失败的情况。
	// 但如果在 handleAck 删除前 delete，会导致 handleAck 找不到。
	// 所以最好的方式是：如果 handleAck 还没处理，这里超时后删除。
	defer func() {
		c.ackMutex.Lock()
		delete(c.pendingAcks, cmd.MsgID)
		c.ackMutex.Unlock()
	}()

	logger.Debug("Sending command to node with ACK expectation", zap.String("command_type", cmd.Type), zap.String("msg_id", cmd.MsgID), zap.String("node_id", c.NodeID))
	if err := c.SendCommand(cmd); err != nil {
		return err
	}

	select {
	case <-ackCh:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for ACK for message %s", cmd.MsgID)
	}
}

// handleSubmitPrintParams 处理提交打印参数消息
func (c *Connection) handleSubmitPrintParams(msg *Message) {
	logger.Debug("Processing print params from node", zap.String("node_id", c.NodeID))

	var payload SubmitPrintParamsPayload

	// Check if msg.Data is already a map or needs unmarshalling
	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		// Convert map to struct manually or re-marshal/unmarshal
		dataBytes, err := json.Marshal(dataMap)
		if err != nil {
			logger.Error("Failed to marshal submit print params map from node", zap.String("node_id", c.NodeID), zap.Error(err))
			return
		}
		if err := json.Unmarshal(dataBytes, &payload); err != nil {
			logger.Error("Failed to unmarshal submit print params map from node", zap.String("node_id", c.NodeID), zap.Error(err))
			return
		}
	} else {
		// Try to marshal whatever it is (e.g. string) and unmarshal
		dataBytes, err := json.Marshal(msg.Data)
		if err != nil {
			logger.Error("Failed to marshal submit print params data from node", zap.String("node_id", c.NodeID), zap.Error(err))
			return
		}
		if err := json.Unmarshal(dataBytes, &payload); err != nil {
			logger.Error("Failed to parse submit print params data from node", zap.String("node_id", c.NodeID), zap.Error(err))
			return
		}
	}

	// 验证必要字段
	if payload.FileID == "" || payload.PrinterID == "" {
		logger.Warn("Missing required fields in submit print params from node", zap.String("node_id", c.NodeID))
		return
	}

	// 获取文件信息
	file, err := c.FileRepo.GetByID(payload.FileID)
	if err != nil {
		logger.Error("Failed to get file", zap.String("file_id", payload.FileID), zap.Error(err))
		return
	}
	if file == nil {
		logger.Warn("File not found", zap.String("file_id", payload.FileID))
		return
	}

	// 重新生成文件URL（因为DB不存储URL）
	file.URL = "/api/v1/files/" + file.ID

	// 标准验证顺序：节点存在性 → 打印机存在性 → 节点启用 → 打印机启用
	// 注意：节点存在性和启用已由 handleMessage 通用拦截器处理

	// 步骤1: 检查打印机是否存在
	printer, err := c.PrinterRepo.GetPrinterByID(payload.PrinterID)
	if err != nil {
		logger.Error("Failed to get printer", zap.String("printer_id", payload.PrinterID), zap.Error(err))
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}
	if printer == nil {
		logger.Warn("Printer not found", zap.String("printer_id", payload.PrinterID))
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}

	// 步骤2: 验证打印机是否属于该节点
	if printer.EdgeNodeID != c.NodeID {
		logger.Warn("Printer does not belong to node", zap.String("printer_id", payload.PrinterID), zap.String("node_id", c.NodeID))
		c.sendError("printer_not_belong_to_node", "Printer does not belong to this node", payload.PrinterID)
		return
	}

	// 步骤3: 检查打印机是否被禁用
	if !printer.Enabled {
		logger.Warn("Printer disabled, rejecting print job submission", zap.String("printer_id", payload.PrinterID), zap.String("node_id", c.NodeID))
		c.sendError("printer_disabled", "Printer has been disabled by administrator", payload.PrinterID)
		return
	}

	// 创建打印任务
	job := &models.PrintJob{
		Name:       file.OriginalName,
		Status:     "pending",
		PrinterID:  payload.PrinterID,
		UserID:     file.UploaderID,
		UserName:   getUserDisplayName(file.UploaderID), // 从用户ID获取显示名称
		FilePath:   file.FilePath,
		FileURL:    file.URL,
		FileSize:   file.Size,
		Copies:     1,
		MaxRetries: 3,
	}

	// 设置 Options
	if val, ok := payload.Options["copies"]; ok {
		if v, ok := val.(float64); ok {
			job.Copies = int(v)
		}
	}
	if val, ok := payload.Options["paper_size"]; ok {
		if v, ok := val.(string); ok {
			job.PaperSize = v
		}
	}
	if val, ok := payload.Options["color_mode"]; ok {
		if v, ok := val.(string); ok {
			job.ColorMode = v
		}
	}
	if val, ok := payload.Options["duplex_mode"]; ok {
		if v, ok := val.(string); ok {
			job.DuplexMode = v
		}
	}
	if val, ok := payload.Options["page_count"]; ok {
		if v, ok := val.(float64); ok {
			job.PageCount = int(v)
		}
	}

	// 保存任务
	if err := c.PrintJobRepo.CreatePrintJob(job); err != nil {
		logger.Error("Failed to create print job", zap.Error(err))
		return
	}

	logger.Info("Print job created for file", zap.String("job_id", job.ID), zap.String("file_id", file.ID))

	// 分发任务到打印机所属的 Edge Node
	if err := c.Manager.DispatchPrintJob(printer.EdgeNodeID, job, printer.Name); err != nil {
		logger.Error("Failed to dispatch print job to node", zap.String("job_id", job.ID), zap.String("node_id", printer.EdgeNodeID), zap.Error(err))
	} else {
		logger.Info("Print job dispatched to node", zap.String("job_id", job.ID), zap.String("node_id", printer.EdgeNodeID))
		job.Status = "dispatched"
		if updateErr := c.PrintJobRepo.UpdatePrintJob(job); updateErr != nil {
			logger.Error("Failed to update job status to dispatched", zap.Error(updateErr))
		}
	}
}

// handleHeartbeat 处理心跳消息
func (c *Connection) handleHeartbeat(msg *Message) {
	logger.Debug("Processing heartbeat from node", zap.String("node_id", c.NodeID))

	// 更新 Edge Node 的最后心跳时间和状态
	if err := c.EdgeNodeRepo.UpdateHeartbeat(c.NodeID); err != nil {
		logger.Error("Failed to update heartbeat for node", zap.String("node_id", c.NodeID), zap.Error(err))
		return
	}
	if err := c.EdgeNodeRepo.UpdateStatus(c.NodeID, "online"); err != nil {
		logger.Error("Failed to update status for node", zap.String("node_id", c.NodeID), zap.Error(err))
		return
	}

	// 解析心跳数据（可选）
	if msg.Data != nil {
		var heartbeatData HeartbeatData
		dataBytes, err := json.Marshal(msg.Data)
		if err == nil {
			if err := json.Unmarshal(dataBytes, &heartbeatData); err == nil {
				logger.Debug("Heartbeat data from node",
					zap.String("node_id", c.NodeID),
					zap.Float64("cpu_usage", heartbeatData.SystemInfo.CPUUsage),
					zap.Float64("memory_usage", heartbeatData.SystemInfo.MemoryUsage),
					zap.Float64("disk_usage", heartbeatData.SystemInfo.DiskUsage))
			}
		}
	}

	logger.Debug("Successfully processed heartbeat from node", zap.String("node_id", c.NodeID))
}

// handleRequestUploadToken 处理请求上传凭证消息
func (c *Connection) handleRequestUploadToken(msg *Message) {
	logger.Debug("Processing upload token request from node", zap.String("node_id", c.NodeID))

	// 标准验证顺序：节点存在性 → 节点启用 → 打印机存在性 → 打印机归属 → 打印机启用
	// 注意：节点存在性和启用已由 handleMessage 通用拦截器处理

	// 解析请求数据
	var payload RequestUploadTokenPayload
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		logger.Error("Failed to marshal upload token request data from node", zap.String("node_id", c.NodeID), zap.Error(err))
		c.sendError("invalid_request", "Failed to parse request data", "")
		return
	}

	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		logger.Error("Failed to parse upload token request data from node", zap.String("node_id", c.NodeID), zap.Error(err))
		c.sendError("invalid_request", "Failed to parse request data", "")
		return
	}

	// 验证 printer_id 必填
	if payload.PrinterID == "" {
		logger.Warn("Missing printer_id in upload token request from node", zap.String("node_id", c.NodeID))
		c.sendError("invalid_request", "printer_id is required", "")
		return
	}

	// 步骤3: 检查打印机是否存在
	printer, err := c.PrinterRepo.GetPrinterByID(payload.PrinterID)
	if err != nil {
		logger.Error("Failed to get printer", zap.String("printer_id", payload.PrinterID), zap.Error(err))
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}
	if printer == nil {
		logger.Warn("Printer not found", zap.String("printer_id", payload.PrinterID))
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}

	// 步骤4: 验证打印机是否属于该节点
	if printer.EdgeNodeID != c.NodeID {
		logger.Warn("Printer does not belong to node", zap.String("printer_id", payload.PrinterID), zap.String("node_id", c.NodeID))
		c.sendError("printer_not_belong_to_node", "Printer does not belong to this node", payload.PrinterID)
		return
	}

	// 步骤5: 检查打印机是否被禁用
	if !printer.Enabled {
		logger.Warn("Printer disabled, rejecting upload token request", zap.String("printer_id", payload.PrinterID), zap.String("node_id", c.NodeID))
		c.sendError("printer_disabled", "Printer has been disabled by administrator", payload.PrinterID)
		return
	}

	// 生成上传凭证
	token, expiresAt, err := c.TokenManager.GenerateUploadToken(c.NodeID, payload.PrinterID)
	if err != nil {
		logger.Error("Failed to generate upload token for node", zap.String("node_id", c.NodeID), zap.Error(err))
		c.sendError("token_generation_failed", "Failed to generate upload token", "")
		return
	}

	// 构造两个URL（相对于 API 根路径，不包含网关前缀）
	// 1. API上传URL：用于Edge端程序化上传（POST请求）
	apiUploadURL := fmt.Sprintf("/api/v1/files?token=%s", token)

	// 2. Web上传页面URL：用于生成二维码/链接给用户（GET请求）
	webUploadURL := fmt.Sprintf("/upload?token=%s&node_id=%s&printer_id=%s", token, c.NodeID, payload.PrinterID)

	// 发送上传凭证响应
	response := map[string]interface{}{
		"type": CmdTypeUploadToken,
		"data": UploadTokenResponsePayload{
			Token:     token,
			ExpiresAt: expiresAt,
			UploadURL: apiUploadURL, // API上传端点
			WebURL:    webUploadURL, // Web上传页面
			NodeID:    c.NodeID,
			PrinterID: payload.PrinterID,
		},
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		logger.Error("Failed to marshal upload token response", zap.Error(err))
		return
	}

	c.Send <- responseBytes
	logger.Info("Upload token generated", zap.String("node_id", c.NodeID), zap.String("printer_id", payload.PrinterID), zap.String("api_url", apiUploadURL), zap.String("web_url", webUploadURL))
}

// sendError 发送错误消息到 Edge 节点
func (c *Connection) sendError(code, message, printerID string) {
	errorData := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	if printerID != "" {
		errorData["printer_id"] = printerID
	}

	errorMsg := map[string]interface{}{
		"type": CmdTypeError,
		"data": errorData,
	}

	errorBytes, _ := json.Marshal(errorMsg)
	c.Send <- errorBytes
}

// handleJobUpdate 处理任务状态更新
func (c *Connection) handleJobUpdate(msg *Message) {
	logger.Debug("Processing job update from node", zap.String("node_id", c.NodeID))

	// 解析任务状态数据
	var jobData JobUpdateData
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		logger.Error("Failed to marshal job update data from node", zap.String("node_id", c.NodeID), zap.Error(err))
		return
	}

	if err := json.Unmarshal(dataBytes, &jobData); err != nil {
		logger.Error("Failed to parse job update data from node", zap.String("node_id", c.NodeID), zap.Error(err))
		return
	}

	var errMsg string
	if jobData.ErrorMessage != nil {
		errMsg = *jobData.ErrorMessage
	}

	logger.Debug("Job update data", zap.String("job_id", jobData.JobID), zap.String("status", jobData.Status), zap.Int("progress", jobData.Progress), zap.String("error", errMsg))

	// 终态保护：任务已完成或已失败时，不再接受后续状态覆盖。
	// 这可避免Edge迟到上报覆盖云端人工取消（failed）结果。
	existingJob, err := c.PrintJobRepo.GetPrintJobByID(jobData.JobID)
	if err != nil {
		logger.Error("Failed to get current job before update", zap.String("job_id", jobData.JobID), zap.Error(err))
		return
	}
	if existingJob == nil {
		logger.Warn("Job not found when handling status update", zap.String("job_id", jobData.JobID))
		return
	}
	if existingJob.Status == "completed" || existingJob.Status == "failed" {
		logger.Debug("Ignoring late status update for terminal job", zap.String("job_id", jobData.JobID), zap.String("current_status", existingJob.Status), zap.String("incoming_status", jobData.Status))
		return
	}

	if err := c.PrintJobRepo.UpdateJobStatus(jobData.JobID, jobData.Status, jobData.Progress, errMsg); err != nil {
		logger.Error("Failed to update job status", zap.String("job_id", jobData.JobID), zap.Error(err))
		return
	}

	logger.Info("Successfully updated job status", zap.String("job_id", jobData.JobID), zap.String("status", jobData.Status), zap.Int("progress", jobData.Progress))
}

// SendCommand 发送指令到 Edge Node
func (c *Connection) SendCommand(cmd *Command) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	select {
	case c.Send <- data:
		return nil
	default:
		close(c.Send)
		return err
	}
}

// getUserDisplayName 从用户ID获取显示名称
// 如果无法获取用户名，返回默认值
func getUserDisplayName(userID string) string {
	if userID == "" {
		return "Unknown User"
	}

	// 注意：这里需要注入 UserRepository 来查询用户名
	// 为了避免循环依赖，暂时使用简单的显示方式
	// 在实际使用中，应该在 ConnectionManager 中注入 UserRepository
	// 并通过 Connection 访问

	// 简单处理：如果是 UUID 格式，显示前8位
	if len(userID) > 8 {
		return "User-" + userID[:8]
	}

	return "User-" + userID
}
