package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"fly-print-cloud/api/internal/logger"
	"fly-print-cloud/api/internal/models"
	"fly-print-cloud/api/internal/security"

	"go.uber.org/zap"
)

// ConnectionManager 管理所有 WebSocket 连接
type ConnectionManager struct {
	connections  map[string]*Connection // node_id -> connection
	broadcast    chan []byte            // 广播消息通道
	register     chan *Connection       // 新连接注册
	unregister   chan *Connection       // 连接断开
	mutex        sync.RWMutex           // 并发安全
	TokenManager *security.TokenManager // 凭证管理器
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(tokenManager *security.TokenManager) *ConnectionManager {
	return &ConnectionManager{
		connections:  make(map[string]*Connection),
		broadcast:    make(chan []byte),
		register:     make(chan *Connection),
		unregister:   make(chan *Connection),
		TokenManager: tokenManager,
	}
}

// Run 启动连接管理器
func (m *ConnectionManager) Run() {
	for {
		select {
		case conn := <-m.register:
			m.registerConnection(conn)

		case conn := <-m.unregister:
			m.unregisterConnection(conn)

		case message := <-m.broadcast:
			m.broadcastMessage(message)
		}
	}
}

// registerConnection 注册新连接
func (m *ConnectionManager) registerConnection(conn *Connection) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 如果已有连接，先关闭旧连接
	if existingConn, exists := m.connections[conn.NodeID]; exists {
		logger.Info("Replacing existing connection for node", zap.String("node_id", conn.NodeID))
		close(existingConn.Send)
	}

	m.connections[conn.NodeID] = conn
	logger.Info("Edge Node connected", zap.String("node_id", conn.NodeID), zap.Int("total_connections", len(m.connections)))

	// 启动离线任务补偿（Reconciliation）
	go func() {
		// 等待连接稳定
		time.Sleep(500 * time.Millisecond)

		logger.Debug("Checking pending jobs for re-connected node", zap.String("node_id", conn.NodeID))
		jobs, err := conn.PrintJobRepo.GetPendingOrDispatchedJobsByEdgeNodeID(conn.NodeID)
		if err != nil {
			logger.Error("Failed to fetch pending jobs for node", zap.String("node_id", conn.NodeID), zap.Error(err))
			return
		}

		if len(jobs) > 0 {
			logger.Info("Found pending/dispatched jobs for node, re-dispatching", zap.Int("count", len(jobs)), zap.String("node_id", conn.NodeID))
			for _, job := range jobs {
				// 重新分发任务
				// 注意：job.PrinterName 已由查询填充
				if err := m.DispatchPrintJob(conn.NodeID, job, job.PrinterName); err != nil {
					logger.Error("Failed to re-dispatch job", zap.String("job_id", job.ID), zap.Error(err))
				} else {
					logger.Info("Successfully re-dispatched job", zap.String("job_id", job.ID))
				}
				// 避免瞬间流量突发
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// unregisterConnection 注销连接
func (m *ConnectionManager) unregisterConnection(conn *Connection) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if currentConn, exists := m.connections[conn.NodeID]; exists {
		// 只有当要注销的连接是当前映射中的连接时才删除
		// 避免新连接注册后，旧连接注销导致新连接被误删
		if currentConn == conn {
			delete(m.connections, conn.NodeID)
			logger.Info("Edge Node disconnected", zap.String("node_id", conn.NodeID), zap.Int("total_connections", len(m.connections)))
		} else {
			logger.Debug("Ignored unregister request for replaced connection of node", zap.String("node_id", conn.NodeID))
		}

		// 安全关闭channel，避免重复关闭
		select {
		case <-conn.Send:
			// channel已经关闭
		default:
			close(conn.Send)
		}
	}
}

// broadcastMessage 广播消息到所有连接
func (m *ConnectionManager) broadcastMessage(message []byte) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for nodeID, conn := range m.connections {
		select {
		case conn.Send <- message:
		default:
			logger.Warn("Failed to send broadcast message to node, closing connection", zap.String("node_id", nodeID))
			close(conn.Send)
			delete(m.connections, nodeID)
		}
	}
}

// SendToNode 发送消息到指定节点
func (m *ConnectionManager) SendToNode(nodeID string, message []byte) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	conn, exists := m.connections[nodeID]
	if !exists {
		return ErrNodeNotConnected
	}

	select {
	case conn.Send <- message:
		return nil
	default:
		return ErrConnectionClosed
	}
}

// GetConnectedNodes 获取已连接的节点列表
func (m *ConnectionManager) GetConnectedNodes() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	nodes := make([]string, 0, len(m.connections))
	for nodeID := range m.connections {
		nodes = append(nodes, nodeID)
	}
	return nodes
}

// IsNodeConnected 检查节点是否在线
func (m *ConnectionManager) IsNodeConnected(nodeID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, exists := m.connections[nodeID]
	return exists
}

// DisconnectNode 主动断开指定节点的 WebSocket 连接
// 用于节点删除时清理资源
func (m *ConnectionManager) DisconnectNode(nodeID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	conn, exists := m.connections[nodeID]
	if !exists {
		return ErrNodeNotConnected
	}

	// 发送关闭通知（可选）
	closeMsg := map[string]interface{}{
		"type": "server_close",
		"data": map[string]string{
			"reason":  "node_deleted",
			"message": "Edge node has been deleted by administrator",
		},
	}
	if msgBytes, err := json.Marshal(closeMsg); err == nil {
		select {
		case conn.Send <- msgBytes:
			// 消息发送成功，等待一小段时间让 Edge 端接收
			time.Sleep(100 * time.Millisecond)
		default:
			// 发送失败，直接关闭
		}
	}

	// 关闭连接
	delete(m.connections, nodeID)
	close(conn.Send)
	conn.Conn.Close()

	logger.Info("Forcefully disconnected Edge Node (node deleted)", zap.String("node_id", nodeID), zap.Int("total_connections", len(m.connections)))
	return nil
}

// GetConnectionCount 获取连接数量
func (m *ConnectionManager) GetConnectionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.connections)
}

// DispatchPreviewFile 发送预览文件命令
func (m *ConnectionManager) DispatchPreviewFile(nodeID string, fileID, fileURL, fileName string, fileSize int64, fileType string) error {
	logger.Debug("Preparing to dispatch preview file to node", zap.String("node_id", nodeID), zap.String("file_name", fileName), zap.String("file_id", fileID))

	payload := PreviewFilePayload{
		FileID:   fileID,
		FileURL:  fileURL,
		FileName: fileName,
		FileSize: fileSize,
		FileType: fileType,
	}

	// 生成文件访问凭证（用于预览）
	if fileURL != "" && m.TokenManager != nil {
		// 从 FileURL 提取 fileID（格式: /api/v1/files/{fileID}）
		extractedFileID := ""
		if len(fileURL) > 14 { // len("/api/v1/files/") = 14
			extractedFileID = fileURL[14:]
		}
		if extractedFileID != "" {
			// 生成预览专用 token，使用 "preview" 作为 jobID
			token, expiresAt, err := m.TokenManager.GenerateDownloadToken(extractedFileID, "preview", nodeID)
			if err != nil {
				logger.Error("Failed to generate download token for preview", zap.Error(err))
			} else {
				payload.FileAccessToken = token
				payload.FileAccessTokenExpiresAt = &expiresAt
				logger.Debug("Generated preview download token", zap.Time("expires_at", expiresAt))
			}
		}
	}

	msg := &Message{
		Type:      CmdTypePreviewFile,
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      payload,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal preview message", zap.Error(err))
		return err
	}

	logger.Debug("Sending preview message to node", zap.String("node_id", nodeID), zap.Int("payload_size", len(msgBytes)))
	return m.SendToNode(nodeID, msgBytes)
}

// DispatchPrintJob 发送打印任务指令
func (m *ConnectionManager) DispatchPrintJob(nodeID string, job *models.PrintJob, printerName string) error {
	// 构造打印任务数据
	printJobData := PrintJobData{
		JobID:       job.ID,
		Name:        job.Name,
		PrinterID:   job.PrinterID,
		PrinterName: printerName,
		FilePath:    job.FilePath,
		FileURL:     job.FileURL,
		FileSize:    job.FileSize,
		PageCount:   job.PageCount,
		Copies:      job.Copies,
		PaperSize:   job.PaperSize,
		ColorMode:   job.ColorMode,
		DuplexMode:  job.DuplexMode,
		MaxRetries:  job.MaxRetries,
	}

	// 如果有文件URL，生成一次性下载凭证
	if job.FileURL != "" && m.TokenManager != nil {
		// 从 FileURL 提取 fileID（格式: /api/v1/files/{fileID}）
		fileID := ""
		if len(job.FileURL) > 14 { // len("/api/v1/files/") = 14
			fileID = job.FileURL[14:]
		}
		if fileID != "" {
			token, expiresAt, err := m.TokenManager.GenerateDownloadToken(fileID, job.ID, nodeID)
			if err != nil {
				logger.Error("Failed to generate download token for job", zap.String("job_id", job.ID), zap.Error(err))
			} else {
				printJobData.FileAccessToken = token
				printJobData.FileAccessTokenExpiresAt = &expiresAt
				logger.Debug("Generated download token for job", zap.String("job_id", job.ID), zap.Time("expires_at", expiresAt))
			}
		}
	}

	// 构造指令消息
	command := Command{
		Type:      CmdTypePrintJob,
		CommandID: job.ID, // 使用job ID作为command ID
		Timestamp: time.Now(),
		Target:    nodeID,
		Data:      printJobData,
	}

	// 获取连接并使用 ACK 机制发送
	m.mutex.RLock()
	conn, exists := m.connections[nodeID]
	m.mutex.RUnlock()

	if !exists {
		return ErrNodeNotConnected
	}

	// 使用 10秒超时等待 ACK
	// 如果超时，意味着 Edge 端虽然在线但未能确认接收任务
	err := conn.SendCommandWithAck(&command, 10*time.Second)
	if err != nil {
		// ACK超时或失败，将任务状态回滚到pending，以便重试机制重新分发
		logger.Warn("Failed to receive ACK for print job from node, rolling back status to pending", zap.String("job_id", job.ID), zap.String("node_id", nodeID), zap.Error(err))

		// 回滚任务状态
		job.Status = "pending"
		job.ErrorMessage = fmt.Sprintf("Failed to receive ACK from edge node: %v", err)

		// 更新任务到数据库（使用Connection中的PrintJobRepo）
		if updateErr := conn.PrintJobRepo.UpdatePrintJob(job); updateErr != nil {
			logger.Error("Failed to rollback job status to pending", zap.String("job_id", job.ID), zap.Error(updateErr))
		}
	}

	return err
}

func (m *ConnectionManager) DispatchNodeEnabledChange(nodeID string, enabled bool) error {
	payload := NodeEnabledPayload{
		Enabled: enabled,
	}

	msg := &Message{
		Type:      CmdTypeNodeState,
		NodeID:    nodeID,
		Timestamp: time.Now(),
		Data:      payload,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return m.SendToNode(nodeID, msgBytes)
}

// DispatchCancelJob 发送取消打印任务通知
func (m *ConnectionManager) DispatchCancelJob(nodeID string, jobID string) error {
	logger.Info("Sending cancel job command to node", zap.String("node_id", nodeID), zap.String("job_id", jobID))

	cmd := Command{
		Type:      "cancel_job",
		CommandID: jobID,
		Timestamp: time.Now(),
		Target:    nodeID,
		Data: map[string]string{
			"job_id": jobID,
			"reason": "cancelled_by_user",
		},
	}

	msgBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	return m.SendToNode(nodeID, msgBytes)
}

// GetActiveConnectionCount 获取当前活跃连接数
func (m *ConnectionManager) GetActiveConnectionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.connections)
}

// GetConnectedNodeIDs 获取所有已连接的节点ID
func (m *ConnectionManager) GetConnectedNodeIDs() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	nodeIDs := make([]string, 0, len(m.connections))
	for nodeID := range m.connections {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}
