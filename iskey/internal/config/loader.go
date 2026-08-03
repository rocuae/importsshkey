package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader 配置加载器
type Loader struct {
	// configFile 配置文件路径
	configFile string
}

// NewLoader 创建配置加载器
// 参数：
//   - configFile: 配置文件路径（空字符串使用默认路径）
//
// 返回：
//   - *Loader: 加载器实例
func NewLoader(configFile string) *Loader {
	if configFile == "" {
		configFile = DefaultConfigPath()
	}
	return &Loader{configFile: configFile}
}

// DefaultConfigPath 获取默认配置文件路径
// 返回：
//   - string: ~/.config/iskey/sources.yaml
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/iskey/sources.yaml"
	}
	return filepath.Join(home, ".config", "iskey", "sources.yaml")
}

// Load 加载并解析配置文件
// 返回：
//   - *Config: 解析后的配置对象
//   - error: 文件读取或解析错误
func (l *Loader) Load() (*Config, error) {
	// 读取文件
	data, err := os.ReadFile(l.configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, l.configFile)
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 环境变量替换
	expanded := expandEnvVars(string(data))

	// 解析 YAML
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// 设置默认值
	setDefaults(&cfg)

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// 解析凭证环境变量
	if err := cfg.ResolveCredentials(); err != nil {
		return nil, fmt.Errorf("resolve credentials: %w", err)
	}

	return &cfg, nil
}

// LoadOrDefault 加载配置文件，如果不存在则返回默认配置
// 返回：
//   - *Config: 配置对象
//   - error: 解析错误（文件不存在不报错）
func (l *Loader) LoadOrDefault() (*Config, error) {
	cfg, err := l.Load()
	if err != nil {
		// 如果是配置文件不存在，返回默认配置
		if strings.Contains(err.Error(), "config file not found") {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	return cfg, nil
}

// ConfigExists 检查配置文件是否存在
// 返回：
//   - bool: 是否存在
func (l *Loader) ConfigExists() bool {
	_, err := os.Stat(l.configFile)
	return err == nil
}

// expandEnvVars 替换字符串中的环境变量引用
// 支持 ${VAR} 和 $VAR 两种格式
// 参数：
//   - s: 包含环境变量引用的字符串
//
// 返回：
//   - string: 替换后的字符串
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		// 支持 ${VAR:-default} 语法
		if idx := strings.Index(key, ":-"); idx >= 0 {
			varName := key[:idx]
			defaultValue := key[idx+2:]
			if value := os.Getenv(varName); value != "" {
				return value
			}
			return defaultValue
		}
		return os.Getenv(key)
	})
}

// setDefaults 设置配置默认值
// 参数：
//   - cfg: 配置对象
func setDefaults(cfg *Config) {
	if cfg.Defaults.Output == "" {
		cfg.Defaults.Output = "~/.ssh/authorized_keys"
	}
	if cfg.Defaults.Timeout == 0 {
		cfg.Defaults.Timeout = 10
	}
	if cfg.Defaults.MaxRetries == 0 {
		cfg.Defaults.MaxRetries = 3
	}

	// 设置源的默认值
	for name, src := range cfg.Sources {
		if src.Format == "" {
			src.Format = "plaintext"
		}
		// 如果没有别名，使用名称作为别名
		if src.Alias == "" {
			src.Alias = name
		}
		cfg.Sources[name] = src
	}
}
