// 本文件提供内嵌管理页面与公开统计接口。
// handler 包定义 HTTP 请求处理器
package handler

import (
	_ "embed"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rocuae/importsshkey/go-server/internal/repository"
)

//go:embed web/index.html
var htmlContent string

// PageHandler 页面处理器
type PageHandler struct {
	// userRepo 用于统计用户与公钥数量
	userRepo repository.UserKeyRepository
	// auditRepo 用于读取最近活动
	auditRepo repository.AuditLogRepository
}

// NewPageHandler 创建页面处理器
func NewPageHandler(userRepo repository.UserKeyRepository, auditRepo repository.AuditLogRepository) *PageHandler {
	return &PageHandler{userRepo: userRepo, auditRepo: auditRepo}
}

// Stats 统计数据响应，不含用户名等隐私信息
type Stats struct {
	// TotalUsers 不同用户名数量
	TotalUsers int64 `json:"total_users"`
	// TotalKeys 公钥总条数
	TotalKeys int64 `json:"total_keys"`
	// Version 接口版本号
	Version string `json:"version"`
	// RecentActivity 最近 5 条审计动作（不含用户名）
	RecentActivity []ActivityItem `json:"recent_activity"`
}

// ActivityItem 活动记录（不暴露用户名）
type ActivityItem struct {
	// Action 审计动作：add/replace/delete
	Action string `json:"action"`
	// Time RFC3339 格式的操作时间
	Time string `json:"time"`
}

// Page 主页面
func (h *PageHandler) Page(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, htmlContent)
}

// StatsAPI 统计数据接口
func (h *PageHandler) StatsAPI(c *gin.Context) {
	totalUsers, err := h.userRepo.CountUsers()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Failed to get stats")
		return
	}
	totalKeys, err := h.userRepo.CountKeys()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	logs, err := h.auditRepo.ListRecent(5)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Failed to get stats")
		return
	}
	recent := make([]ActivityItem, 0, len(logs))
	for _, log := range logs {
		recent = append(recent, ActivityItem{
			Action: log.Action,
			Time:   log.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, Stats{
		TotalUsers:     totalUsers,
		TotalKeys:      totalKeys,
		Version:        "v1.1",
		RecentActivity: recent,
	})
}
