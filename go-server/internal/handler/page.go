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
	userRepo repository.UserRepository
}

// NewPageHandler 创建页面处理器
// 参数：
//   - userRepo: 用户仓储
// 返回：
//   - *PageHandler: 处理器实例
func NewPageHandler(userRepo repository.UserRepository) *PageHandler {
	return &PageHandler{userRepo: userRepo}
}

// Stats 统计数据响应
type Stats struct {
	TotalUsers     int            `json:"total_users"`
	Version        string         `json:"version"`
	RecentActivity []ActivityItem `json:"recent_activity"`
}

// ActivityItem 活动记录
type ActivityItem struct {
	User    string `json:"user"`
	Action  string `json:"action"`
	Time    string `json:"time"`
	Source  string `json:"source"`
}

// Page 主页面
// @Summary 主页面
// @Description 返回 Web 管理界面
// @Tags page
// @Produce html
// @Router / [get]
func (h *PageHandler) Page(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, htmlContent)
}

// StatsAPI 统计数据接口
// @Summary 获取统计数据
// @Description 返回用户数量和最近活动
// @Tags stats
// @Produce json
// @Success 200 {object} Stats
// @Router /stats [get]
func (h *PageHandler) StatsAPI(c *gin.Context) {
	users, err := h.userRepo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	// 获取最近活动（取最后5个用户）
	recentActivity := make([]ActivityItem, 0)
	limit := 5
	if len(users) < limit {
		limit = len(users)
	}

	// 按更新时间倒序
	for i := len(users) - 1; i >= len(users)-limit && i >= 0; i-- {
		user := users[i]
		recentActivity = append(recentActivity, ActivityItem{
			User:   user.Username,
			Action: "update",
			Time:   user.UpdatedAt.Format(time.RFC3339),
			Source: user.Source,
		})
	}

	c.JSON(http.StatusOK, Stats{
		TotalUsers:     len(users),
		Version:        "v1.0",
		RecentActivity: recentActivity,
	})
}


