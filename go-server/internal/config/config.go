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

// YAMLConfig YAML 配置文件结构
type YAMLConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Driver   string `yaml:"driver"`
		Path     string `yaml:"path"`
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
	Security struct {
		AdminToken     string `yaml:"admin_token"`
		AllowedOrigins string `yaml:"allowed_origins"`
	} `yaml:"security"`
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
}

// Load 从 config.yaml 和环境变量加载配置
// 参数：
//   - configFile: 配置文件路径，为空时使用默认的 config.yaml
// 返回：
//   - *Config: 配置对象
func Load(configFile string) *Config {
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
		if err := yaml.Unmarshal(data, &yamlCfg); err == nil {
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

	return cfg
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


