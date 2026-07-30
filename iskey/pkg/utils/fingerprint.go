// utils 包提供公共工具函数
package utils

import (
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/ssh"
)

// Fingerprint 计算 SSH 公钥的 SHA256 指纹
// 参数：
//   - pubKey: SSH 公钥内容（如 "ssh-ed25519 AAAAC3..."）
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
