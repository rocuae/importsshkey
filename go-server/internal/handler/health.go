// handler 包定义 HTTP 请求处理器
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealthHandler 健康检查处理器
type HealthHandler struct {
	// db 用于执行 Ping 检查数据库可用性，可为 nil
	db *gorm.DB
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Health 健康检查
// @Summary 健康检查接口
// @Description 返回服务状态
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "服务正常"
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	status := "ok"
	code := http.StatusOK
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			status = "degraded"
			code = http.StatusServiceUnavailable
		} else {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := sqlDB.PingContext(ctx); err != nil {
				status = "degraded"
				code = http.StatusServiceUnavailable
			}
		}
	}
	c.JSON(code, gin.H{
		"status":    status,
		"service":   "iskey-server",
		"version":   "v1.1",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	// Status ok 或 degraded
	Status string `json:"status"`
	// Service 服务名称
	Service string `json:"service"`
	// Version 服务版本
	Version string `json:"version"`
	// Timestamp 响应时间（RFC3339）
	Timestamp string `json:"timestamp"`
}
