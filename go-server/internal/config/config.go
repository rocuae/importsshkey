// config 包负责加载和解析配置
package config

import (
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	// ServerHost 服务器监听地址
	ServerHost string
	// ServerPort 服务器监听端口
	ServerPort int
	// DBDriver 数据库驱动: sqlite / postgres
	DBDriver string
	// DSN 数据库连接字符串
	DSN string
	// AdminToken 管理员 Token（为空时禁止写操作）
	AdminToken string
	// AllowedOrigins 允许的 CORS 来源
	AllowedOrigins string
	// LogLevel 日志级别
	LogLevel string
}

// Load 从环境变量加载配置
// 返回：
//   - *Config: 配置对象
func Load() *Config {
	port, err := strconv.Atoi(getEnv("SERVER_PORT", "8080"))
	if err != nil {
		port = 8080
	}

	return &Config{
		ServerHost:     getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:     port,
		DBDriver:       getEnv("DB_DRIVER", "sqlite"),
		DSN:            getEnv("DB_DSN", "./iskey.db"),
		AdminToken:     getEnv("ADMIN_TOKEN", ""),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
	}
}

// getEnv 获取环境变量，不存在时返回默认值
// 参数：
//   - key: 环境变量名
//   - defaultValue: 默认值
// 返回：
//   - string: 环境变量值或默认值
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
