// handler 包定义 HTTP 请求处理器
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rocuae/importsshkey/go-server/internal/model"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
	"github.com/rocuae/importsshkey/go-server/pkg/validator"
)

// KeyHandler 公钥处理器
type KeyHandler struct {
	// repo 多公钥仓储
	repo repository.UserKeyRepository
	// auditLog 审计日志仓储；写入失败只忽略，不影响主流程
	auditLog repository.AuditLogRepository
}

// NewKeyHandler 创建公钥处理器
func NewKeyHandler(repo repository.UserKeyRepository, auditLog repository.AuditLogRepository) *KeyHandler {
	return &KeyHandler{repo: repo, auditLog: auditLog}
}

// GetKey 获取用户全部公钥（纯文本，每行一把）
// @Summary 获取指定用户的 SSH 公钥
// @Tags keys
// @Produce plain
// @Param username path string true "用户名"
// @Success 200 {string} string "公钥内容"
// @Failure 404 {object} ErrorResponse "用户不存在"
// @Router /keys/{username} [get]
func (h *KeyHandler) GetKey(c *gin.Context) {
	username := c.Param("username")
	if err := validator.ValidateUsername(username); err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	keys, err := h.repo.ListByUsername(username)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Database error")
		return
	}
	if len(keys) == 0 {
		writeError(c, http.StatusNotFound, "User not found")
		return
	}

	lines := make([]string, len(keys))
	for i, key := range keys {
		lines[i] = key.PublicKey
	}
	c.String(http.StatusOK, strings.Join(lines, "\n")+"\n")
}

// GetKeyMetadata 获取用户全部公钥的元数据
// @Summary 获取用户公钥的元数据
// @Tags keys
// @Produce json
// @Param username path string true "用户名"
// @Router /keys/{username}/metadata [get]
func (h *KeyHandler) GetKeyMetadata(c *gin.Context) {
	username := c.Param("username")
	if err := validator.ValidateUsername(username); err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	keys, err := h.repo.ListByUsername(username)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Database error")
		return
	}
	if len(keys) == 0 {
		writeError(c, http.StatusNotFound, "User not found")
		return
	}

	items := make([]gin.H, len(keys))
	for i, key := range keys {
		items[i] = gin.H{
			"fingerprint": key.Fingerprint,
			"source":      key.Source,
			"created_at":  key.CreatedAt.Format(time.RFC3339),
			"updated_at":  key.UpdatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"username":          username,
		"public_key_exists": true,
		"count":             len(keys),
		"keys":              items,
	})
}

// PutKey 全量替换用户公钥
// @Summary 全量替换用户的 SSH 公钥
// @Description 请求中的 keys 会成为该用户的全部公钥；支持旧的单 public_key 字段
// @Tags keys
// @Accept json
// @Produce json
// @Param username path string true "用户名"
// @Param request body PutKeyRequest true "公钥列表"
// @Router /keys/{username} [put]
func (h *KeyHandler) PutKey(c *gin.Context) {
	h.writeKeys(c, true)
}

// PostKey 追加用户公钥
// @Summary 追加用户的 SSH 公钥
// @Tags keys
// @Accept json
// @Produce json
// @Param username path string true "用户名"
// @Param request body PutKeyRequest true "公钥列表"
// @Router /keys/{username} [post]
func (h *KeyHandler) PostKey(c *gin.Context) {
	h.writeKeys(c, false)
}

// writeKeys 是 PUT/POST 共用实现：replace 为 true 时全量替换，否则追加。
func (h *KeyHandler) writeKeys(c *gin.Context, replace bool) {
	username := c.Param("username")
	if err := validator.ValidateUsername(username); err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	var req PutKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(c, http.StatusRequestEntityTooLarge, "Request body too large")
			return
		}
		writeError(c, http.StatusBadRequest, "invalid JSON body")
		return
	}

	inputs, err := collectKeyInputs(req)
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	keys := make([]*model.UserKey, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		if validateErr := validator.ValidatePublicKey(input.PublicKey); validateErr != nil {
			writeError(c, http.StatusBadRequest, validateErr.Error())
			return
		}
		fingerprint, fingerprintErr := validator.Fingerprint(input.PublicKey)
		if fingerprintErr != nil {
			writeError(c, http.StatusBadRequest, fingerprintErr.Error())
			return
		}
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		keys = append(keys, &model.UserKey{
			Username:    username,
			PublicKey:   strings.TrimSpace(input.PublicKey),
			Fingerprint: fingerprint,
			Source:      input.Source,
		})
	}

	action := "add"
	actionLabel := "added"
	if replace {
		err = h.repo.Replace(username, keys)
		action = "replace"
		actionLabel = "replaced"
	} else {
		err = h.repo.AddMany(keys)
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Failed to save key")
		return
	}

	stored, err := h.repo.ListByUsername(username)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Database error")
		return
	}
	count := len(stored)

	_ = h.auditLog.Create(&model.AuditLog{
		Username:  username,
		Action:    action,
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"username":   username,
		"action":     actionLabel,
		"count":      count,
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

// DeleteKey 删除用户公钥；可选 fingerprint 仅删除一把
// @Summary 删除用户的 SSH 公钥
// @Tags keys
// @Produce json
// @Param username path string true "用户名"
// @Param fingerprint query string false "仅删除指定指纹"
// @Router /keys/{username} [delete]
func (h *KeyHandler) DeleteKey(c *gin.Context) {
	username := c.Param("username")
	if err := validator.ValidateUsername(username); err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	fingerprint := strings.TrimSpace(c.Query("fingerprint"))
	var (
		removed int64
		err     error
	)
	if fingerprint != "" {
		removed, err = h.repo.DeleteByFingerprint(username, fingerprint)
	} else {
		removed, err = h.repo.DeleteByUsername(username)
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Failed to delete key")
		return
	}
	if removed == 0 {
		writeError(c, http.StatusNotFound, "User not found")
		return
	}

	_ = h.auditLog.Create(&model.AuditLog{
		Username:  username,
		Action:    "delete",
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"username": username,
		"action":   "deleted",
		"removed":  removed,
	})
}

// ListKeys 分页列出用户
// @Summary 分页列出用户
// @Tags keys
// @Produce json
// @Param q query string false "用户名模糊搜索"
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页数量，默认 50，最大 200"
// @Router /keys [get]
func (h *KeyHandler) ListKeys(c *gin.Context) {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 50)
	if pageSize > 200 {
		pageSize = 200
	}
	query := strings.TrimSpace(c.Query("q"))

	users, total, err := h.repo.ListUsers(query, (page-1)*pageSize, pageSize)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "Database error")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"users":     users,
	})
}

// parsePositiveInt 解析正整数查询参数，非法时使用回退值。
func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

// writeError 输出统一错误结构。
func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{Error: message})
}

// collectKeyInputs 兼容 keys 数组与旧版 public_key 字段。
func collectKeyInputs(req PutKeyRequest) ([]KeyInput, error) {
	if len(req.Keys) > 0 {
		return req.Keys, nil
	}
	if strings.TrimSpace(req.PublicKey) != "" {
		return []KeyInput{{PublicKey: req.PublicKey, Source: req.Source}}, nil
	}
	return nil, validator.ErrEmptyKey
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	// Error 面向调用方的错误描述
	Error string `json:"error"`
}

// KeyInput 单把公钥输入
type KeyInput struct {
	// PublicKey SSH 公钥原文
	PublicKey string `json:"public_key"`
	// Source 可选来源标识
	Source string `json:"source"`
}

// PutKeyRequest 添加/替换公钥请求；Keys 优先，PublicKey 用于兼容旧客户端
type PutKeyRequest struct {
	// Keys 多公钥输入，优先于 PublicKey
	Keys []KeyInput `json:"keys"`
	// PublicKey 旧版单公钥字段，仅为兼容保留
	PublicKey string `json:"public_key"`
	// Source 旧版单公钥的来源字段
	Source string `json:"source"`
}

// DeleteKeyResponse 删除公钥响应
type DeleteKeyResponse struct {
	// Success 固定为 true
	Success bool `json:"success"`
	// Username 被操作的用户名
	Username string `json:"username"`
	// Action 固定为 deleted
	Action string `json:"action"`
	// Removed 实际删除的公钥数量
	Removed int64 `json:"removed"`
}

// ListKeysResponse 列出用户响应
type ListKeysResponse struct {
	// Total 匹配的用户总数
	Total int64 `json:"total"`
	// Page 当前页码（从 1 开始）
	Page int `json:"page"`
	// PageSize 每页数量（最大 200）
	PageSize int `json:"page_size"`
	// Users 当前页用户名
	Users []string `json:"users"`
}
