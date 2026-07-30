package config

import "errors"

var (
	// ErrConfigNotFound 配置文件不存在
	ErrConfigNotFound = errors.New("config file not found")
	// ErrInvalidConfig 配置格式错误
	ErrInvalidConfig = errors.New("invalid config format")
	// ErrMissingCredential 凭证环境变量缺失
	ErrMissingCredential = errors.New("credential environment variable not set")
)
