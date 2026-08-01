// validator 包提供 SSH 公钥校验工具
package validator

import (
	"strings"

	"golang.org/x/crypto/ssh"
	"github.com/rocuae/importsshkey/go-server/internal/model"
)

// ValidatePublicKey 校验 SSH 公钥格式是否合法
// 参数：
//   - key: SSH 公钥内容
// 返回：
//   - error: 校验失败错误
func ValidatePublicKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrEmptyKey
	}

	// 使用 golang.org/x/crypto/ssh 进行严格校验
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		return ErrInvalidKeyFormat
	}

	return nil
}

// ValidateUsername 校验用户名格式是否合法
// 仅允许字母、数字、下划线、连字符
// 参数：
//   - username: 用户名
// 返回：
//   - error: 校验失败错误
func ValidateUsername(username string) error {
	if username == "" {
		return ErrEmptyUsername
	}

	for _, c := range username {
		if !isAllowedChar(c) {
			return ErrInvalidUsername
		}
	}

	return nil
}

// isAllowedChar 检查字符是否允许出现在用户名中
func isAllowedChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '-'
}

// 定义校验错误
var (
	ErrEmptyKey        = &ValidationError{Message: "public key is empty"}
	ErrInvalidKeyFormat = &ValidationError{Message: "invalid SSH public key format"}
	ErrEmptyUsername   = &ValidationError{Message: "username is empty"}
	ErrInvalidUsername = &ValidationError{Message: "invalid username format (only alphanumeric, underscore, hyphen allowed)"}
)

// ValidationError 校验错误
type ValidationError struct {
	Message string
}

// Error 返回错误信息
func (e *ValidationError) Error() string {
	return e.Message
}

// ToUser 将 User 模型转换为用户信息（用于响应）
func ToUser(user *model.User) map[string]interface{} {
	return map[string]interface{}{
		"username":    user.Username,
		"public_key":  user.PublicKey,
		"source":      user.Source,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	}
}
