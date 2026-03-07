package handlers

import (
	"net/http"
	"time"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/websocket"

	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查处理器
type HealthHandler struct {
	db        *database.DB
	wsManager *websocket.ConnectionManager
	startTime time.Time
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler(db *database.DB, wsManager *websocket.ConnectionManager) *HealthHandler {
	return &HealthHandler{
		db:        db,
		wsManager: wsManager,
		startTime: time.Now(),
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string          `json:"status"`
	Service   string          `json:"service"`
	Version   string          `json:"version"`
	Uptime    string          `json:"uptime"`
	Timestamp string          `json:"timestamp"`
	Database  DatabaseHealth  `json:"database"`
	WebSocket WebSocketHealth `json:"websocket"`
	System    SystemHealth    `json:"system"`
}

// DatabaseHealth 数据库健康状态
type DatabaseHealth struct {
	Status       string `json:"status"`
	Connected    bool   `json:"connected"`
	OpenConns    int    `json:"open_connections"`
	MaxOpenConns int    `json:"max_open_connections"`
	IdleConns    int    `json:"idle_connections"`
}

// WebSocketHealth WebSocket健康状态
type WebSocketHealth struct {
	Status            string `json:"status"`
	ActiveConnections int    `json:"active_connections"`
}

// SystemHealth 系统健康状态
type SystemHealth struct {
	Status string `json:"status"`
}

// BasicHealth 基础健康检查（快速响应）
// @Summary 基础健康检查
// @Description 快速检查服务是否运行
// @Tags 健康检查
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "服务正常"
// @Router /health [get]
func (h *HealthHandler) BasicHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"message": "success",
		"data": gin.H{
			"status":  "ok",
			"service": "fly-print-cloud-api",
		},
	})
}

// DetailedHealth 详细健康检查
// @Summary 详细健康检查
// @Description 检查服务各组件的健康状态
// @Tags 健康检查
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse "详细健康状态"
// @Router /api/v1/health [get]
func (h *HealthHandler) DetailedHealth(c *gin.Context) {
	now := time.Now()
	uptime := now.Sub(h.startTime)

	// 检查数据库连接
	dbHealth := h.checkDatabase()

	// 检查 WebSocket
	wsHealth := h.checkWebSocket()

	// 检查系统
	sysHealth := h.checkSystem()

	// 计算整体状态
	overallStatus := "healthy"
	if dbHealth.Status == "unhealthy" || wsHealth.Status == "unhealthy" || sysHealth.Status == "unhealthy" {
		overallStatus = "unhealthy"
	} else if dbHealth.Status == "degraded" || wsHealth.Status == "degraded" || sysHealth.Status == "degraded" {
		overallStatus = "degraded"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Service:   "fly-print-cloud-api",
		Version:   "1.0.0",
		Uptime:    formatUptime(uptime),
		Timestamp: now.Format(time.RFC3339),
		Database:  dbHealth,
		WebSocket: wsHealth,
		System:    sysHealth,
	}

	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"code":    statusCode,
		"message": overallStatus,
		"data":    response,
	})
}

// checkDatabase 检查数据库连接状态
func (h *HealthHandler) checkDatabase() DatabaseHealth {
	health := DatabaseHealth{
		Status:    "healthy",
		Connected: false,
	}

	// 检查数据库连接
	if h.db == nil {
		health.Status = "unhealthy"
		return health
	}

	// 执行简单的 ping 查询
	err := h.db.Ping()
	if err != nil {
		health.Status = "unhealthy"
		return health
	}

	health.Connected = true

	// 获取连接池统计
	stats := h.db.Stats()
	health.OpenConns = stats.OpenConnections
	health.MaxOpenConns = stats.MaxOpenConnections
	health.IdleConns = stats.Idle

	// 检查连接池是否接近饱和
	if health.MaxOpenConns > 0 {
		usage := float64(health.OpenConns) / float64(health.MaxOpenConns)
		if usage > 0.9 {
			health.Status = "degraded"
		}
	}

	return health
}

// checkWebSocket 检查 WebSocket 状态
func (h *HealthHandler) checkWebSocket() WebSocketHealth {
	health := WebSocketHealth{
		Status: "healthy",
	}

	if h.wsManager == nil {
		health.Status = "unhealthy"
		return health
	}

	// 获取活跃连接数
	health.ActiveConnections = h.wsManager.GetActiveConnectionCount()

	// 如果连接数过多，标记为降级
	if health.ActiveConnections > 1000 {
		health.Status = "degraded"
	}

	return health
}

// checkSystem 检查系统状态
func (h *HealthHandler) checkSystem() SystemHealth {
	return SystemHealth{
		Status: "healthy",
	}
}

// formatUptime 格式化运行时间
func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return formatDuration(days, "d", hours, "h", minutes, "m", seconds, "s")
	} else if hours > 0 {
		return formatDuration(hours, "h", minutes, "m", seconds, "s")
	} else if minutes > 0 {
		return formatDuration(minutes, "m", seconds, "s")
	}
	return formatDuration(seconds, "s")
}

func formatDuration(values ...interface{}) string {
	result := ""
	for i := 0; i < len(values); i += 2 {
		if i+1 < len(values) {
			result += formatInt(values[i]) + values[i+1].(string)
		}
	}
	return result
}

func formatInt(v interface{}) string {
	switch val := v.(type) {
	case int:
		return formatIntValue(val)
	default:
		return "0"
	}
}

func formatIntValue(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return intToString(n)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	var result string
	for n > 0 {
		result = string(rune('0'+(n%10))) + result
		n /= 10
	}
	return result
}
