package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"
	"github.com/gorilla/websocket"
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
)

// Connection 表示单个 WebSocket 连接
type Connection struct {
	NodeID         string
	Conn           *websocket.Conn
	Send           chan []byte
	Manager        *ConnectionManager
	PrinterRepo    *database.PrinterRepository
	EdgeNodeRepo   *database.EdgeNodeRepository
	PrintJobRepo   *database.PrintJobRepository
	FileRepo       *database.FileRepository
	TokenManager   *security.TokenManager
}

// NewConnection 创建新连接
func NewConnection(nodeID string, conn *websocket.Conn, manager *ConnectionManager, printerRepo *database.PrinterRepository, edgeNodeRepo *database.EdgeNodeRepository, printJobRepo *database.PrintJobRepository, fileRepo *database.FileRepository, tokenManager *security.TokenManager) *Connection {
	return &Connection{
		NodeID:         nodeID,
		Conn:           conn,
		Send:           make(chan []byte, 256),
		Manager:        manager,
		PrinterRepo:    printerRepo,
		EdgeNodeRepo:   edgeNodeRepo,
		PrintJobRepo:   printJobRepo,
		FileRepo:       fileRepo,
		TokenManager:   tokenManager,
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
			log.Printf("WebSocket read error for node %s: %v", c.NodeID, err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket unexpected close error for node %s: %v", c.NodeID, err)
			}
			break
		}

		log.Printf("WebSocket received raw message from node %s: %s", c.NodeID, string(messageBytes))

		// 解析消息
		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("Failed to parse message from node %s: %v", c.NodeID, err)
			continue
		}

		log.Printf("WebSocket parsed message from node %s: type=%s", c.NodeID, msg.Type)

		// 处理消息
		c.handleMessage(&msg)
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
	log.Printf("Received message from node %s: type=%s", c.NodeID, msg.Type)

	// 标准验证顺序（所有消息统一处理）：
	// 1. 节点存在性检查（所有消息都需要，包括 heartbeat 和 job_update）
	// 2. 节点启用检查（heartbeat 和 job_update 放行，允许禁用节点维持连接和完成任务收尾）
	
	// 步骤1: 检查节点是否存在（所有消息都需要）
	node, err := c.EdgeNodeRepo.GetEdgeNodeByID(c.NodeID)
	if err != nil || node == nil {
		log.Printf("Message rejected: node %s not found, message type: %s", c.NodeID, msg.Type)
		c.sendError("node_not_found", "Edge node not found", "")
		return
	}

	// 步骤2: 检查节点是否启用（heartbeat 和 job_update 放行）
	if msg.Type != MsgTypeHeartbeat && msg.Type != MsgTypeJobUpdate {
		if !node.Enabled {
			log.Printf("Message rejected: node %s is disabled, message type: %s", c.NodeID, msg.Type)
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
	default:
		log.Printf("Unknown message type: %s from node %s", msg.Type, c.NodeID)
	}
}

// handleSubmitPrintParams 处理提交打印参数消息
func (c *Connection) handleSubmitPrintParams(msg *Message) {
	log.Printf("Processing print params from node %s", c.NodeID)

	var payload SubmitPrintParamsPayload
	
	// Check if msg.Data is already a map or needs unmarshalling
	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		// Convert map to struct manually or re-marshal/unmarshal
		dataBytes, err := json.Marshal(dataMap)
		if err != nil {
			log.Printf("Failed to marshal submit print params map from node %s: %v", c.NodeID, err)
			return
		}
		if err := json.Unmarshal(dataBytes, &payload); err != nil {
			log.Printf("Failed to unmarshal submit print params map from node %s: %v", c.NodeID, err)
			return
		}
	} else {
		// Try to marshal whatever it is (e.g. string) and unmarshal
		dataBytes, err := json.Marshal(msg.Data)
		if err != nil {
			log.Printf("Failed to marshal submit print params data from node %s: %v", c.NodeID, err)
			return
		}
		if err := json.Unmarshal(dataBytes, &payload); err != nil {
			log.Printf("Failed to parse submit print params data from node %s: %v", c.NodeID, err)
			return
		}
	}

	// 验证必要字段
	if payload.FileID == "" || payload.PrinterID == "" {
		log.Printf("Missing required fields (file_id or printer_id) in submit print params from node %s", c.NodeID)
		return
	}

	// 获取文件信息
	file, err := c.FileRepo.GetByID(payload.FileID)
	if err != nil {
		log.Printf("Failed to get file %s: %v", payload.FileID, err)
		return
	}
	if file == nil {
		log.Printf("File %s not found", payload.FileID)
		return
	}

	// 重新生成文件URL（因为DB不存储URL）
	file.URL = "/api/v1/files/" + file.ID

	// 标准验证顺序：节点存在性 → 打印机存在性 → 节点启用 → 打印机启用
	// 注意：节点存在性和启用已由 handleMessage 通用拦截器处理

	// 步骤1: 检查打印机是否存在
	printer, err := c.PrinterRepo.GetPrinterByID(payload.PrinterID)
	if err != nil {
		log.Printf("Failed to get printer %s: %v", payload.PrinterID, err)
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}
	if printer == nil {
		log.Printf("Printer %s not found", payload.PrinterID)
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}
	
	// 步骤2: 验证打印机是否属于该节点
	if printer.EdgeNodeID != c.NodeID {
		log.Printf("Printer %s does not belong to node %s", payload.PrinterID, c.NodeID)
		c.sendError("printer_not_belong_to_node", "Printer does not belong to this node", payload.PrinterID)
		return
	}
	
	// 步骤3: 检查打印机是否被禁用
	if !printer.Enabled {
		log.Printf("Printer %s is disabled, rejecting print job submission from node %s", payload.PrinterID, c.NodeID)
		c.sendError("printer_disabled", "Printer has been disabled by administrator", payload.PrinterID)
		return
	}

	// 创建打印任务
	job := &models.PrintJob{
		Name:         file.OriginalName,
		Status:       "pending",
		PrinterID:    payload.PrinterID,
		UserID:       file.UploaderID,
		UserName:     "Edge User", // TODO: 如果有用户信息，应该从某处获取
		FilePath:     file.FilePath,
		FileURL:      file.URL,
		FileSize:     file.Size,
		Copies:       1,
		MaxRetries:   3,
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
		log.Printf("Failed to create print job: %v", err)
		return
	}
	
	log.Printf("Print job %s created for file %s", job.ID, file.ID)

	// 分发任务到打印机所属的 Edge Node
	if err := c.Manager.DispatchPrintJob(printer.EdgeNodeID, job, printer.Name); err != nil {
		log.Printf("Failed to dispatch print job %s to node %s: %v", job.ID, printer.EdgeNodeID, err)
	} else {
		log.Printf("Print job %s dispatched to node %s", job.ID, printer.EdgeNodeID)
		job.Status = "dispatched"
		if updateErr := c.PrintJobRepo.UpdatePrintJob(job); updateErr != nil {
			log.Printf("Failed to update job status to dispatched: %v", updateErr)
		}
	}
}

// handleHeartbeat 处理心跳消息
func (c *Connection) handleHeartbeat(msg *Message) {
	log.Printf("Processing heartbeat from node %s", c.NodeID)
	
	// 更新 Edge Node 的最后心跳时间和状态
	if err := c.EdgeNodeRepo.UpdateHeartbeat(c.NodeID); err != nil {
		log.Printf("Failed to update heartbeat for node %s: %v", c.NodeID, err)
		return
	}
	if err := c.EdgeNodeRepo.UpdateStatus(c.NodeID, "online"); err != nil {
		log.Printf("Failed to update status for node %s: %v", c.NodeID, err)
		return
	}
	
	// 解析心跳数据（可选）
	if msg.Data != nil {
		var heartbeatData HeartbeatData
		dataBytes, err := json.Marshal(msg.Data)
		if err == nil {
			if err := json.Unmarshal(dataBytes, &heartbeatData); err == nil {
				log.Printf("Heartbeat data from node %s: CPU=%.2f%%, Memory=%.2f%%, Disk=%.2f%%", 
					c.NodeID, heartbeatData.SystemInfo.CPUUsage, 
					heartbeatData.SystemInfo.MemoryUsage, heartbeatData.SystemInfo.DiskUsage)
			}
		}
	}
	
	log.Printf("Successfully processed heartbeat from node %s", c.NodeID)
}

// handleRequestUploadToken 处理请求上传凭证消息
func (c *Connection) handleRequestUploadToken(msg *Message) {
	log.Printf("Processing upload token request from node %s", c.NodeID)

	// 标准验证顺序：节点存在性 → 节点启用 → 打印机存在性 → 打印机归属 → 打印机启用
	// 注意：节点存在性和启用已由 handleMessage 通用拦截器处理

	// 解析请求数据
	var payload RequestUploadTokenPayload
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("Failed to marshal upload token request data from node %s: %v", c.NodeID, err)
		c.sendError("invalid_request", "Failed to parse request data", "")
		return
	}

	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		log.Printf("Failed to parse upload token request data from node %s: %v", c.NodeID, err)
		c.sendError("invalid_request", "Failed to parse request data", "")
		return
	}

	// 验证 printer_id 必填
	if payload.PrinterID == "" {
		log.Printf("Missing printer_id in upload token request from node %s", c.NodeID)
		c.sendError("invalid_request", "printer_id is required", "")
		return
	}

	// 步骤3: 检查打印机是否存在
	printer, err := c.PrinterRepo.GetPrinterByID(payload.PrinterID)
	if err != nil {
		log.Printf("Failed to get printer %s: %v", payload.PrinterID, err)
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}
	if printer == nil {
		log.Printf("Printer %s not found", payload.PrinterID)
		c.sendError("printer_not_found", "Printer not found", payload.PrinterID)
		return
	}

	// 步骤4: 验证打印机是否属于该节点
	if printer.EdgeNodeID != c.NodeID {
		log.Printf("Printer %s does not belong to node %s", payload.PrinterID, c.NodeID)
		c.sendError("printer_not_belong_to_node", "Printer does not belong to this node", payload.PrinterID)
		return
	}

	// 步骤5: 检查打印机是否被禁用
	if !printer.Enabled {
		log.Printf("Printer %s is disabled, rejecting upload token request from node %s", payload.PrinterID, c.NodeID)
		c.sendError("printer_disabled", "Printer has been disabled by administrator", payload.PrinterID)
		return
	}

	// 生成上传凭证
	token, expiresAt, err := c.TokenManager.GenerateUploadToken(c.NodeID, payload.PrinterID)
	if err != nil {
		log.Printf("Failed to generate upload token for node %s: %v", c.NodeID, err)
		c.sendError("token_generation_failed", "Failed to generate upload token", "")
		return
	}

	// 构造两个URL
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
			UploadURL: apiUploadURL,  // API上传端点
			WebURL:    webUploadURL,  // Web上传页面
			NodeID:    c.NodeID,
			PrinterID: payload.PrinterID,
		},
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal upload token response: %v", err)
		return
	}

	c.Send <- responseBytes
	log.Printf("Upload token generated for node %s, printer %s - API: %s, Web: %s", c.NodeID, payload.PrinterID, apiUploadURL, webUploadURL)
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
	log.Printf("Processing job update from node %s", c.NodeID)
	
	// 解析任务状态数据
	var jobData JobUpdateData
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("Failed to marshal job update data from node %s: %v", c.NodeID, err)
		return
	}
	
	if err := json.Unmarshal(dataBytes, &jobData); err != nil {
		log.Printf("Failed to parse job update data from node %s: %v", c.NodeID, err)
		return
	}
	
	var errMsg string
	if jobData.ErrorMessage != nil {
		errMsg = *jobData.ErrorMessage
	}

	log.Printf("Job update data: job_id=%s, status=%s, progress=%d, error=%s", 
		jobData.JobID, jobData.Status, jobData.Progress, errMsg)

	if err := c.PrintJobRepo.UpdateJobStatus(jobData.JobID, jobData.Status, jobData.Progress, errMsg); err != nil {
		log.Printf("Failed to update job %s status: %v", jobData.JobID, err)
		return
	}

	log.Printf("Successfully updated job %s status to %s (progress: %d%%)", 
		jobData.JobID, jobData.Status, jobData.Progress)
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
