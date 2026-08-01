// handler 包定义 HTTP 请求处理器
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rocuae/importsshkey/go-server/internal/model"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
	"github.com/rocuae/importsshkey/go-server/pkg/validator"
)

// KeyHandler 公钥处理器
type KeyHandler struct {
	repo     repository.UserRepository
	auditLog repository.AuditLogRepository
}

// NewKeyHandler 创建公钥处理器
// 参数：
//   - repo: 用户仓储
//   - auditLog: 审计日志仓储
// 返回：
//   - *KeyHandler: 处理器实例
func NewKeyHandler(repo repository.UserRepository, auditLog repository.AuditLogRepository) *KeyHandler {
	return &KeyHandler{repo: repo, auditLog: auditLog}
}

// GetKey 获取用户公钥
// @Summary 获取指定用户的 SSH 公钥
// @Description 返回纯文本格式的公钥内容
// @Tags keys
// @Accept plain
// @Produce plain
// @Param username path string true "用户名"
// @Success 200 {string} string "公钥内容"
// @Failure 400 {object} ErrorResponse "用户名无效"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /keys/{username} [get]
func (h *KeyHandler) GetKey(c *gin.Context) {
	username := c.Param("username")
	if err := validator.ValidateUsername(username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.repo.GetByUsername(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.String(http.StatusOK, user.PublicKey)
}

// GetKeyMetadata 获取用户公钥元数据
// @Summary 获取用户公钥的元数据
// @Description 返回公钥的更新时间和来源信息
// @Tags keys
// @Accept json
// @Produce json
// @Param username path string true "用户名"
// @Success 200 {object} map[string]interface{} "元数据"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /keys/{username}/metadata [get]
func (h *KeyHandler) GetKeyMetadata(c *gin.Context) {
	username := c.Param("username")
	user, err := h.repo.GetByUsername(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":          user.Username,
		"public_key_exists": true,
		"metadata": gin.H{
			"updated_at": user.UpdatedAt.Format(time.RFC3339),
			"source":     user.Source,
		},
	})
}

// PutKey 添加或更新用户公钥
// @Summary 添加或更新用户的 SSH 公钥
// @Description 需要 Bearer Token 认证
// @Tags keys
// @Accept json
// @Produce json
// @Param username path string true "用户名"
// @Param request body PutKeyRequest true "公钥信息"
// @Success 200 {object} PutKeyResponse "操作成功"
// @Failure 400 {object} ErrorResponse "请求无效"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "写操作禁用"
// @Router /keys/{username} [put]
func (h *KeyHandler) PutKey(c *gin.Context) {
	username := c.Param("username")
	if err := validator.ValidateUsername(username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		PublicKey string `json:"public_key" binding:"required"`
		Source    string `json:"source"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "public_key is required"})
		return
	}

	if err := validator.ValidatePublicKey(req.PublicKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := &model.User{
		Username:  username,
		PublicKey: req.PublicKey,
		Source:    req.Source,
	}

	if err := h.repo.CreateOrUpdate(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save key"})
		return
	}

	// 记录审计日志
	h.auditLog.Create(&model.AuditLog{
		Username:  username,
		Action:    "update",
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"username":   username,
		"action":     "updated",
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

// DeleteKey 删除用户公钥
// @Summary 删除用户的 SSH 公钥
// @Description 需要 Bearer Token 认证
// @Tags keys
// @Accept json
// @Produce json
// @Param username path string true "用户名"
// @Success 200 {object} DeleteKeyResponse "操作成功"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "写操作禁用"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /keys/{username} [delete]
func (h *KeyHandler) DeleteKey(c *gin.Context) {
	username := c.Param("username")

	exists, err := h.repo.Exists(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := h.repo.DeleteByUsername(username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete key"})
		return
	}

	// 记录审计日志
	h.auditLog.Create(&model.AuditLog{
		Username:  username,
		Action:    "delete",
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"username": username,
		"action":   "deleted",
	})
}

// ListKeys 列出所有用户
// @Summary 列出所有用户
// @Description 需要 Bearer Token 认证
// @Tags keys
// @Accept json
// @Produce json
// @Success 200 {object} ListKeysResponse "用户列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "写操作禁用"
// @Router /keys [get]
func (h *KeyHandler) ListKeys(c *gin.Context) {
	users, err := h.repo.ListAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	usernames := make([]string, len(users))
	for i, u := range users {
		usernames[i] = u.Username
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(users),
		"users": usernames,
	})
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// PutKeyRequest 添加/更新公钥请求
type PutKeyRequest struct {
	PublicKey string `json:"public_key"`
	Source    string `json:"source"`
}

// PutKeyResponse 添加/更新公钥响应
type PutKeyResponse struct {
	Success   bool   `json:"success"`
	Username  string `json:"username"`
	Action    string `json:"action"`
	UpdatedAt string `json:"updated_at"`
}

// DeleteKeyResponse 删除公钥响应
type DeleteKeyResponse struct {
	Success  bool   `json:"success"`
	Username string `json:"username"`
	Action   string `json:"action"`
}

// ListKeysResponse 列出用户响应
type ListKeysResponse struct {
	Total int      `json:"total"`
	Users []string `json:"users"`
}
