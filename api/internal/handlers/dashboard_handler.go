package handlers

import (
	"strconv"
	"time"

	"fly-print-cloud/api/internal/database"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	printJobRepo *database.PrintJobRepository
	alertRepo    *database.OperationalAlertRepository
}

func NewDashboardHandler(printJobRepo *database.PrintJobRepository, alertRepo *database.OperationalAlertRepository) *DashboardHandler {
	return &DashboardHandler{
		printJobRepo: printJobRepo,
		alertRepo:    alertRepo,
	}
}

func (h *DashboardHandler) GetMaintenance(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 100 {
		size = 20
	}
	alerts, total, err := h.alertRepo.List("open", c.Query("resource_type"), c.Query("node_id"), c.Query("printer_id"), nil, nil, (page-1)*size, size)
	if err != nil {
		InternalErrorResponse(c, "查询当前告警失败")
		return
	}
	summary, err := h.alertRepo.DeviceOverview()
	if err != nil {
		InternalErrorResponse(c, "查询告警统计失败")
		return
	}
	SuccessResponse(c, gin.H{"items": alerts, "total": total, "page": page, "page_size": size,
		"summary": summary, "refreshed_at": time.Now()})
}

func (h *DashboardHandler) GetAlertHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if size < 1 || size > 100 {
		size = 20
	}
	var from, to *time.Time
	if v := c.Query("from"); v != "" {
		if t, e := time.Parse(time.RFC3339, v); e == nil {
			from = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, e := time.Parse(time.RFC3339, v); e == nil {
			to = &t
		}
	}
	alerts, total, err := h.alertRepo.List(c.DefaultQuery("status", "resolved"), c.Query("resource_type"), c.Query("node_id"), c.Query("printer_id"), from, to, (page-1)*size, size)
	if err != nil {
		InternalErrorResponse(c, "查询告警历史失败")
		return
	}
	SuccessResponse(c, gin.H{"items": alerts, "total": total, "page": page, "page_size": size})
}

// GetTrends 获取打印任务趋势数据
func (h *DashboardHandler) GetTrends(c *gin.Context) {
	period := c.DefaultQuery("period", "day")
	if period != "day" && period != "month" && period != "year" {
		BadRequestResponse(c, "period 仅支持 day、month 或 year")
		return
	}
	buckets, err := h.printJobRepo.TrendBuckets(period, time.Now())
	if err != nil {
		InternalErrorResponse(c, "查询打印趋势失败")
		return
	}
	SuccessResponse(c, gin.H{"period": period, "buckets": buckets})
}
