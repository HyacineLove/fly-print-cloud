package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParsePaginationParams 解析分页参数
// 返回 page（页码，从1开始）, pageSize（每页数量）, offset（偏移量）
func ParsePaginationParams(c *gin.Context) (page, pageSize, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "10"))

	// 参数校验
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 计算偏移量
	offset = (page - 1) * pageSize
	return
}
