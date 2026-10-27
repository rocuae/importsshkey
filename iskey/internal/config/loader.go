package config

import (
	"errors"
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

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// 只对 YAML 字符串节点的值做环境变量替换。直接对整份文本做替换会让
	// 环境变量里的换行/冒号破坏 YAML 结构。
	expandEnvNode(&root)

	var cfg Config
	if len(root.Content) > 0 {
		if err := root.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
	}

	// 设置默认值
	setDefaults(&cfg)

	// 验证配置（不读取凭据环境变量，未使用的凭据不应导致命令失败）
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
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
		if errors.Is(err, ErrConfigNotFound) {
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

// expandEnvNode 递归展开 YAML 字符串节点中的环境变量。
// 只修改标量字符串的值，不会改变文档结构。
func expandEnvNode(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode && (node.Tag == "!!str" || node.Tag == "") {
		node.Value = expandEnvVars(node.Value)
		return
	}
	for _, child := range node.Content {
		expandEnvNode(child)
	}
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
