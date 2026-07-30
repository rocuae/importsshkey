// handler 包定义 HTTP 请求处理器
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查处理器
type HealthHandler struct{}

// NewHealthHandler 创建健康检查处理器
// 返回：
//   - *HealthHandler: 处理器实例
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health 健康检查
// @Summary 健康检查接口
// @Description 返回服务状态
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "服务正常"
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "iskey-server",
		"version":   "v1.0",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}
