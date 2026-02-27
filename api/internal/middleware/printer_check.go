package middleware

import (
	"net/http"

	"fly-print-cloud/api/internal/database"
	"github.com/gin-gonic/gin"
)

// PrinterEnabledCheck 检查打印机是否被禁用
// 如果打印机被禁用或不存在，返回相应错误
// 用于保护打印机相关的 API 端点
func PrinterEnabledCheck(printerRepo *database.PrinterRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		printerID := c.Param("printer_id")
		if printerID == "" {
			// 如果没有 printer_id 参数，跳过检查
			c.Next()
			return
		}

		// 查询打印机状态
		printer, err := printerRepo.GetPrinterByID(printerID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"error":   "printer_not_found",
				"message": "Printer not found or has been deleted",
			})
			c.Abort()
			return
		}

		// 检查打印机是否被禁用
		if !printer.Enabled {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    http.StatusForbidden,
				"error":   "printer_disabled",
				"message": "Printer has been disabled by administrator",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// PrinterExistsCheck 检查打印机是否存在（不检查启用状态）
// 用于需要知道打印机存在但不需要检查启用状态的端点
func PrinterExistsCheck(printerRepo *database.PrinterRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		printerID := c.Param("printer_id")
		if printerID == "" {
			c.Next()
			return
		}

		// 查询打印机是否存在
		_, err := printerRepo.GetPrinterByID(printerID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"error":   "printer_not_found",
				"message": "Printer not found or has been deleted",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
