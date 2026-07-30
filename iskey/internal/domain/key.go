// domain 包定义核心领域模型，与外部实现无关
package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// KeyEntry 公钥实体，表示一条 SSH 公钥记录
// 包含公钥内容、来源标识、用户名和 SHA256 指纹
type KeyEntry struct {
	// PublicKey 公钥内容，如 "ssh-ed25519 AAAAC3..."
	PublicKey string
	// Source 来源标识，如 "github", "work"
	Source string
	// User 用户名，如 "zhangsan"
	User string
	// Fingerprint SHA256 指纹，Base64 编码
	Fingerprint string
	// Comment authorized_keys 行尾注释，格式: iskey:<source>:<user>:<fingerprint>
	Comment string
}

// NewKeyEntry 创建公钥实体，自动校验格式并计算指纹
// 参数：
//   - pubKey: SSH 公钥内容
//   - source: 来源标识
//   - user: 用户名
//
// 返回：
//   - *KeyEntry: 校验通过的公钥实体
//   - error: 格式错误（ErrEmptyKey 或 ErrInvalidKeyFormat）
func NewKeyEntry(pubKey, source, user string) (*KeyEntry, error) {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return nil, ErrEmptyKey
	}

	// 校验公钥格式
	if err := ValidatePublicKey(pubKey); err != nil {
		return nil, err
	}

	// 计算指纹
	fp := Fingerprint(pubKey)
	if fp == "" {
		return nil, fmt.Errorf("%w: failed to compute fingerprint", ErrInvalidKeyFormat)
	}

	return &KeyEntry{
		PublicKey:   pubKey,
		Source:      source,
		User:        user,
		Fingerprint: fp,
		Comment:     FormatComment(source, user, fp),
	}, nil
}

// ValidatePublicKey 校验 SSH 公钥格式是否合法
// 参数：
//   - pubKey: SSH 公钥内容
//
// 返回：
//   - error: ErrEmptyKey 或 ErrInvalidKeyFormat
func ValidatePublicKey(pubKey string) error {
	pubKey = strings.TrimSpace(pubKey)
	if pubKey == "" {
		return ErrEmptyKey
	}

	// 使用 golang.org/x/crypto/ssh 进行严格校验
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidKeyFormat, err)
	}

	return nil
}

// Fingerprint 计算公钥的 SHA256 指纹
// 参数：
//   - pubKey: SSH 公钥内容
//
// 返回：
//   - string: Base64 编码的 SHA256 指纹
func Fingerprint(pubKey string) string {
	// 使用 golang.org/x/crypto/ssh 解析公钥
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return ""
	}

	// 计算 SHA256 指纹
	hash := sha256.Sum256(parsed.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
}

// FormatComment 格式化 authorized_keys 行尾注释
// 参数：
//   - source: 来源标识
//   - user: 用户名
//   - fingerprint: 指纹
//
// 返回：
//   - string: 格式化的注释，如 "iskey:github:zhangsan:SHA256:xxx"
func FormatComment(source, user, fingerprint string) string {
	return fmt.Sprintf("iskey:%s:%s:%s", source, user, fingerprint)
}

// ParseComment 解析 authorized_keys 行尾注释
// 参数：
//   - comment: 注释内容
//
// 返回：
//   - source: 来源标识
//   - user: 用户名
//   - fingerprint: 指纹
//   - ok: 是否解析成功
func ParseComment(comment string) (source, user, fingerprint string, ok bool) {
	// 移除 "# " 前缀（如有）
	comment = strings.TrimPrefix(comment, "# ")
	comment = strings.TrimSpace(comment)

	// 解析 iskey:<source>:<user>:<fingerprint>
	parts := strings.SplitN(comment, ":", 4)
	if len(parts) != 4 || parts[0] != "iskey" {
		return "", "", "", false
	}

	return parts[1], parts[2], parts[3], true
}
