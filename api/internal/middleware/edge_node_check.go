package middleware

import (
	"net/http"

	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EdgeNodeEnabledCheck 检查 Edge 节点是否被禁用
// 如果节点被禁用，返回 403 Forbidden 错误
// 用于保护 Edge 相关的 API 端点
func EdgeNodeEnabledCheck(edgeNodeRepo *database.EdgeNodeRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.Param("node_id")
		if nodeID == "" {
			// 如果没有 node_id 参数，跳过检查
			c.Next()
			return
		}

		// 查询节点状态
		node, err := edgeNodeRepo.GetEdgeNodeByID(nodeID)
		if err != nil {
			logger.Warn("EdgeNodeEnabledCheck: node not found", zap.String("node_id", nodeID), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"error":   "node_not_found",
				"message": "Edge node not found",
			})
			c.Abort()
			return
		}

		// 检查节点是否被禁用
		if !node.Enabled {
			logger.Warn("EdgeNodeEnabledCheck: node is disabled, rejecting request", zap.String("node_id", nodeID))
			c.JSON(http.StatusForbidden, gin.H{
				"code":    http.StatusForbidden,
				"error":   "node_disabled",
				"message": "Edge node has been disabled by administrator",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
