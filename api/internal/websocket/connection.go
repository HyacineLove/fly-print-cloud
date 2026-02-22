package websocket

import (
	"encoding/json"
	"log"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/models"
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
}

// NewConnection 创建新连接
func NewConnection(nodeID string, conn *websocket.Conn, manager *ConnectionManager, printerRepo *database.PrinterRepository, edgeNodeRepo *database.EdgeNodeRepository, printJobRepo *database.PrintJobRepository, fileRepo *database.FileRepository) *Connection {
	return &Connection{
		NodeID:         nodeID,
		Conn:           conn,
		Send:           make(chan []byte, 256),
		Manager:        manager,
		PrinterRepo:    printerRepo,
		EdgeNodeRepo:   edgeNodeRepo,
		PrintJobRepo:   printJobRepo,
		FileRepo:       fileRepo,
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

	switch msg.Type {
	case MsgTypeHeartbeat:
		c.handleHeartbeat(msg)
	case MsgTypePrinterStatus:
		c.handlePrinterStatus(msg)
	case MsgTypeJobUpdate:
		c.handleJobUpdate(msg)
	case MsgTypeSubmitPrintParams:
		c.handleSubmitPrintParams(msg)
	default:
		log.Printf("Unknown message type: %s from node %s", msg.Type, c.NodeID)
	}
}

// handleSubmitPrintParams 处理提交打印参数消息
func (c *Connection) handleSubmitPrintParams(msg *Message) {
	log.Printf("Processing print params from node %s", c.NodeID)

	var payload SubmitPrintParamsPayload
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("Failed to marshal submit print params data from node %s: %v", c.NodeID, err)
		return
	}

	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		log.Printf("Failed to parse submit print params data from node %s: %v", c.NodeID, err)
		return
	}

	// 验证必要字段
	if payload.FileID == "" || payload.PrinterID == "" {
		log.Printf("Missing required fields (file_id or printer_id) in submit print params from node %s", c.NodeID)
		return
	}
	
	// 验证 Task Token
	if !c.Manager.ValidateTaskToken(payload.TaskToken, payload.FileID) {
		log.Printf("Invalid or expired task token for file %s from node %s", payload.FileID, c.NodeID)
		// TODO: 可以发送错误消息回 Edge
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

	// 获取打印机信息
	printer, err := c.PrinterRepo.GetPrinterByID(payload.PrinterID)
	if err != nil {
		log.Printf("Failed to get printer %s: %v", payload.PrinterID, err)
		return
	}
	if printer == nil {
		log.Printf("Printer %s not found", payload.PrinterID)
		return
	}
	
	// 验证打印机是否属于该 Node
	if printer.EdgeNodeID != c.NodeID {
		log.Printf("Printer %s does not belong to node %s", payload.PrinterID, c.NodeID)
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

// handlePrinterStatus 处理打印机状态消息
func (c *Connection) handlePrinterStatus(msg *Message) {
	log.Printf("Processing printer status update from node %s", c.NodeID)
	
	// 解析打印机状态数据
	var statusData PrinterStatusData
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		log.Printf("Failed to marshal printer status data from node %s: %v", c.NodeID, err)
		return
	}
	
	if err := json.Unmarshal(dataBytes, &statusData); err != nil {
		log.Printf("Failed to parse printer status data from node %s: %v", c.NodeID, err)
		return
	}
	
	log.Printf("Printer status data: printer_id=%s, status=%s, queue_length=%d", 
		statusData.PrinterID, statusData.Status, statusData.QueueLength)
	
	// 使用消息中的node_id而不是连接时的NodeID，因为可能不匹配
	messageNodeID := msg.NodeID
	if messageNodeID == "" {
		messageNodeID = c.NodeID // 如果消息中没有node_id，使用连接时的ID
	}
	
	// 通过名称和边缘节点ID查找打印机
	printer, err := c.PrinterRepo.GetPrinterByNameAndEdgeNode(statusData.PrinterID, messageNodeID)
	if err != nil {
		log.Printf("Printer %s not found for node %s (connection: %s): %v", statusData.PrinterID, messageNodeID, c.NodeID, err)
		return
	}
	
	// 直接使用客户端状态（统一标准）
	printer.Status = statusData.Status
	printer.QueueLength = statusData.QueueLength
	
	if err := c.PrinterRepo.UpdatePrinter(printer); err != nil {
		log.Printf("Failed to update printer %s status: %v", statusData.PrinterID, err)
		return
	}
	
	log.Printf("Successfully updated printer %s status to %s (queue: %d)", 
		statusData.PrinterID, statusData.Status, statusData.QueueLength)
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
	
	log.Printf("Job update data: job_id=%s, status=%s, progress=%d", 
		jobData.JobID, jobData.Status, jobData.Progress)
	
	// 更新数据库中的任务状态
	if err := c.PrintJobRepo.UpdateJobStatus(jobData.JobID, jobData.Status, jobData.Progress); err != nil {
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
