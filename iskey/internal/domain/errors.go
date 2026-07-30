package domain

import "errors"

var (
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidKeyFormat 无效的公钥格式
	ErrInvalidKeyFormat = errors.New("invalid SSH public key format")
	// ErrEmptyKey 公钥内容为空
	ErrEmptyKey = errors.New("public key is empty")
	// ErrConfigNotFound 配置文件不存在
	ErrConfigNotFound = errors.New("config file not found")
	// ErrSourceNotFound 数据源不存在
	ErrSourceNotFound = errors.New("source not found")
	// ErrAuthFailed 认证失败
	ErrAuthFailed = errors.New("authentication failed")
	// ErrNetworkTimeout 网络超时
	ErrNetworkTimeout = errors.New("network timeout")
	// ErrWriteFailed 文件写入失败
	ErrWriteFailed = errors.New("file write failed")
)
