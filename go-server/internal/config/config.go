// config 包负责加载和解析配置
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	// ServerHost 服务器监听地址
	ServerHost string
	// ServerPort 服务器监听端口
	ServerPort int
	// DBDriver 数据库驱动: sqlite / postgres
	DBDriver string
	// DBPath SQLite 数据库文件路径（仅 SQLite）
	DBPath string
	// DBHost PostgreSQL 主机地址
	DBHost string
	// DBPort PostgreSQL 端口
	DBPort int
	// DBUser PostgreSQL 用户名
	DBUser string
	// DBPassword PostgreSQL 密码
	DBPassword string
	// DBName PostgreSQL 数据库名
	DBName string
	// DBSSLMode PostgreSQL SSL 模式
	DBSSLMode string
	// AdminToken 管理员 Token（为空时禁止写操作）
	AdminToken string
	// AllowedOrigins 允许的 CORS 来源
	AllowedOrigins string
	// LogLevel 日志级别
	LogLevel string
}

// YAMLConfig 对应 config.yaml 的嵌套结构；零值字段不会覆盖内置默认值。
type YAMLConfig struct {
	Server struct {
		// Host 监听地址
		Host string `yaml:"host"`
		// Port 监听端口
		Port int `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		// Driver 数据库类型：sqlite/postgres
		Driver string `yaml:"driver"`
		// Path SQLite 文件路径
		Path string `yaml:"path"`
		// Host PostgreSQL 主机
		Host string `yaml:"host"`
		// Port PostgreSQL 端口
		Port int `yaml:"port"`
		// User PostgreSQL 用户
		User string `yaml:"user"`
		// Password PostgreSQL 密码
		Password string `yaml:"password"`
		// DBName PostgreSQL 数据库名
		DBName string `yaml:"dbname"`
		// SSLMode PostgreSQL SSL 模式
		SSLMode string `yaml:"sslmode"`
	} `yaml:"database"`
	Security struct {
		// AdminToken 写接口 Bearer Token
		AdminToken string `yaml:"admin_token"`
		// AllowedOrigins CORS 白名单，逗号分隔或 *
		AllowedOrigins string `yaml:"allowed_origins"`
	} `yaml:"security"`
	Log struct {
		// Level 日志级别
		Level string `yaml:"level"`
	} `yaml:"log"`
}

// Load 从 config.yaml 和环境变量加载配置
// 参数：
//   - configFile: 配置文件路径，为空时使用默认的 config.yaml
//
// 返回：
//   - *Config: 配置对象
//   - error: 配置文件读取或解析错误
func Load(configFile string) (*Config, error) {
	// 默认值
	cfg := &Config{
		ServerHost:     "0.0.0.0",
		ServerPort:     8080,
		DBDriver:       "sqlite",
		DBPath:         "./iskey.db",
		DBHost:         "localhost",
		DBPort:         5432,
		DBUser:         "postgres",
		DBPassword:     "",
		DBName:         "iskey",
		DBSSLMode:      "disable",
		AdminToken:     "",
		AllowedOrigins: "*",
		LogLevel:       "info",
	}

	// 默认配置文件路径
	if configFile == "" {
		configFile = "config.yaml"
	}

	// 尝试从配置文件加载
	if data, err := os.ReadFile(configFile); err == nil {
		var yamlCfg YAMLConfig
		if unmarshalErr := yaml.Unmarshal(data, &yamlCfg); unmarshalErr != nil {
			return nil, fmt.Errorf("parse config file %s: %w", configFile, unmarshalErr)
		}
		{
			if yamlCfg.Server.Host != "" {
				cfg.ServerHost = yamlCfg.Server.Host
			}
			if yamlCfg.Server.Port != 0 {
				cfg.ServerPort = yamlCfg.Server.Port
			}
			if yamlCfg.Database.Driver != "" {
				cfg.DBDriver = yamlCfg.Database.Driver
			}
			if yamlCfg.Database.Path != "" {
				cfg.DBPath = yamlCfg.Database.Path
			}
			if yamlCfg.Database.Host != "" {
				cfg.DBHost = yamlCfg.Database.Host
			}
			if yamlCfg.Database.Port != 0 {
				cfg.DBPort = yamlCfg.Database.Port
			}
			if yamlCfg.Database.User != "" {
				cfg.DBUser = yamlCfg.Database.User
			}
			if yamlCfg.Database.Password != "" {
				cfg.DBPassword = yamlCfg.Database.Password
			}
			if yamlCfg.Database.DBName != "" {
				cfg.DBName = yamlCfg.Database.DBName
			}
			if yamlCfg.Database.SSLMode != "" {
				cfg.DBSSLMode = yamlCfg.Database.SSLMode
			}
			if yamlCfg.Security.AdminToken != "" {
				cfg.AdminToken = yamlCfg.Security.AdminToken
			}
			if yamlCfg.Security.AllowedOrigins != "" {
				cfg.AllowedOrigins = yamlCfg.Security.AllowedOrigins
			}
			if yamlCfg.Log.Level != "" {
				cfg.LogLevel = yamlCfg.Log.Level
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config file %s: %w", configFile, err)
	}

	// 环境变量覆盖
	if value, exists := os.LookupEnv("SERVER_HOST"); exists && value != "" {
		cfg.ServerHost = value
	}
	if value, exists := os.LookupEnv("SERVER_PORT"); exists && value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			cfg.ServerPort = port
		}
	}
	if value, exists := os.LookupEnv("DB_DRIVER"); exists && value != "" {
		cfg.DBDriver = value
	}
	if value, exists := os.LookupEnv("DB_PATH"); exists && value != "" {
		cfg.DBPath = value
	}
	if value, exists := os.LookupEnv("DB_HOST"); exists && value != "" {
		cfg.DBHost = value
	}
	if value, exists := os.LookupEnv("DB_PORT"); exists && value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			cfg.DBPort = port
		}
	}
	if value, exists := os.LookupEnv("DB_USER"); exists && value != "" {
		cfg.DBUser = value
	}
	if value, exists := os.LookupEnv("DB_PASSWORD"); exists && value != "" {
		cfg.DBPassword = value
	}
	if value, exists := os.LookupEnv("DB_NAME"); exists && value != "" {
		cfg.DBName = value
	}
	if value, exists := os.LookupEnv("DB_SSLMODE"); exists && value != "" {
		cfg.DBSSLMode = value
	}
	if value, exists := os.LookupEnv("ADMIN_TOKEN"); exists && value != "" {
		cfg.AdminToken = value
	}
	if value, exists := os.LookupEnv("ALLOWED_ORIGINS"); exists && value != "" {
		cfg.AllowedOrigins = value
	}
	if value, exists := os.LookupEnv("LOG_LEVEL"); exists && value != "" {
		cfg.LogLevel = value
	}

	if cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
		return nil, fmt.Errorf("invalid server port: %d", cfg.ServerPort)
	}
	switch cfg.DBDriver {
	case "sqlite", "postgres", "postgresql":
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.DBDriver)
	}
	return cfg, nil
}

// DSN 根据驱动类型构建数据库连接字符串
// 返回：
//   - string: 数据库连接字符串
func (c *Config) DSN() string {
	switch c.DBDriver {
	case "postgres", "postgresql":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
	default: // sqlite
		return c.DBPath
	}
}
