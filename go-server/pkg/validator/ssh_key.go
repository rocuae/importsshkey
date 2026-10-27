// validator 包提供 SSH 公钥校验工具
package validator

import (
	"strings"

	"github.com/rocuae/importsshkey/go-server/internal/model"
	"golang.org/x/crypto/ssh"
)

// ValidatePublicKey 校验 SSH 公钥格式是否合法
// 参数：
//   - key: SSH 公钥内容
//
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

// Fingerprint 计算公钥的 SHA256 指纹（格式 SHA256:base64）
func Fingerprint(key string) (string, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(key)))
	if err != nil {
		return "", ErrInvalidKeyFormat
	}
	return ssh.FingerprintSHA256(publicKey), nil
}

// ValidateUsername 校验用户名格式是否合法
// 仅允许字母、数字、下划线、连字符
// 参数：
//   - username: 用户名
//
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
	ErrEmptyKey         = &ValidationError{Message: "public key is empty"}
	ErrInvalidKeyFormat = &ValidationError{Message: "invalid SSH public key format"}
	ErrEmptyUsername    = &ValidationError{Message: "username is empty"}
	ErrInvalidUsername  = &ValidationError{Message: "invalid username format (only alphanumeric, underscore, hyphen allowed)"}
)

// ValidationError 校验错误
type ValidationError struct {
	// Message 面向调用方的错误描述
	Message string
}

// Error 返回错误信息
func (e *ValidationError) Error() string {
	return e.Message
}

// ToUser 将旧版单公钥模型转换为响应 map，仅用于兼容旧调用方。
func ToUser(user *model.User) map[string]interface{} {
	return map[string]interface{}{
		"username":   user.Username,
		"public_key": user.PublicKey,
		"source":     user.Source,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}
}
