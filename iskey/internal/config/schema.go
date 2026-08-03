// config 包负责加载和解析配置文件
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config 全局配置
type Config struct {
	// Defaults 全局默认配置
	Defaults Defaults `yaml:"defaults"`
	// Credentials 认证凭据定义
	Credentials map[string]Credential `yaml:"credentials"`
	// Sources 数据源定义
	Sources map[string]SourceConfig `yaml:"sources"`
}

// Defaults 全局默认配置
type Defaults struct {
	// Output 目标 authorized_keys 文件路径，默认: ~/.ssh/authorized_keys
	Output string `yaml:"output"`
	// Timeout HTTP 请求超时时间（秒），默认: 10
	Timeout int `yaml:"timeout"`
	// InsecureSkipVerify 是否跳过 TLS 证书校验（仅限内网自签名），默认: false
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
	// MaxRetries 拉取失败时的最大重试次数，默认: 3
	MaxRetries int `yaml:"max_retries"`
}

// Credential 认证凭据
type Credential struct {
	// Type 认证类型: bearer / basic / none
	Type string `yaml:"type"`
	// ValueFromEnv 环境变量名（Bearer 类型）
	ValueFromEnv string `yaml:"value_from_env"`
	// UsernameFromEnv 环境变量名（Basic 类型）
	UsernameFromEnv string `yaml:"username_from_env"`
	// PasswordFromEnv 环境变量名（Basic 类型）
	PasswordFromEnv string `yaml:"password_from_env"`
	// ResolvedValue 解析后的值（Bearer 类型）
	ResolvedValue string `yaml:"-"`
	// ResolvedUsername 解析后的用户名（Basic 类型）
	ResolvedUsername string `yaml:"-"`
	// ResolvedPassword 解析后的密码（Basic 类型）
	ResolvedPassword string `yaml:"-"`
}

// SourceConfig 数据源配置
type SourceConfig struct {
	// Alias 短别名，用于命令行快速引用
	Alias string `yaml:"alias"`
	// URL 静态 URL 或模板 URL，支持 {{ .VarName }} 语法
	URL string `yaml:"url"`
	// Format 返回内容解析格式: plaintext 或 github_json，默认: plaintext
	Format string `yaml:"format"`
	// AuthRef 引用的凭证名称
	AuthRef string `yaml:"auth_ref"`
	// DefaultVars 默认模板变量值
	DefaultVars map[string]string `yaml:"default_vars"`
}

// Validate 验证配置是否合法
// 返回：
//   - error: 验证错误
func (c *Config) Validate() error {
	// 验证数据源
	for name, src := range c.Sources {
		if src.URL == "" {
			return fmt.Errorf("source %q: url is required", name)
		}
		if src.Format != "" && src.Format != "plaintext" && src.Format != "github_json" {
			return fmt.Errorf("source %q: invalid format %q (must be plaintext or github_json)", name, src.Format)
		}
	}

	return nil
}

// ResolveCredentials 解析所有凭证的环境变量值
// 返回：
//   - error: 环境变量缺失错误
func (c *Config) ResolveCredentials() error {
	for name, cred := range c.Credentials {
		switch cred.Type {
		case "bearer":
			if cred.ValueFromEnv == "" {
				return fmt.Errorf("credential %q: value_from_env is required for bearer type", name)
			}
			value := os.Getenv(cred.ValueFromEnv)
			if value == "" {
				return fmt.Errorf("credential %q: environment variable %s is not set", name, cred.ValueFromEnv)
			}
			cred.ResolvedValue = value
			c.Credentials[name] = cred

		case "basic":
			if cred.UsernameFromEnv == "" || cred.PasswordFromEnv == "" {
				return fmt.Errorf("credential %q: username_from_env and password_from_env are required for basic type", name)
			}
			username := os.Getenv(cred.UsernameFromEnv)
			if username == "" {
				return fmt.Errorf("credential %q: environment variable %s is not set", name, cred.UsernameFromEnv)
			}
			password := os.Getenv(cred.PasswordFromEnv)
			if password == "" {
				return fmt.Errorf("credential %q: environment variable %s is not set", name, cred.PasswordFromEnv)
			}
			cred.ResolvedUsername = username
			cred.ResolvedPassword = password
			c.Credentials[name] = cred

		case "none", "":
			// 无需认证

		default:
			return fmt.Errorf("credential %q: unsupported type %q (must be bearer, basic, or none)", name, cred.Type)
		}
	}

	return nil
}

// GetCredential 获取指定名称的凭证
// 参数：
//   - name: 凭证名称
//
// 返回：
//   - *Credential: 凭证对象（可能为 nil）
//   - bool: 是否找到
func (c *Config) GetCredential(name string) (*Credential, bool) {
	if name == "" {
		return nil, false
	}
	cred, ok := c.Credentials[name]
	if !ok {
		return nil, false
	}
	return &cred, true
}

// GetSource 获取指定别名的数据源
// 参数：
//   - alias: 源别名或名称
//
// 返回：
//   - string: 源名称
//   - *SourceConfig: 源配置
//   - bool: 是否找到
func (c *Config) GetSource(alias string) (string, *SourceConfig, bool) {
	// 处理内置别名
	switch alias {
	case "gh":
		alias = "github"
	case "lp":
		alias = "launchpad"
	}

	// 先按名称查找
	if src, ok := c.Sources[alias]; ok {
		return alias, &src, true
	}

	// 再按别名查找
	for name, src := range c.Sources {
		if src.Alias == alias {
			return name, &src, true
		}
		// 去掉 "my-" 前缀匹配
		cleaned := strings.TrimPrefix(name, "my-")
		if cleaned == alias {
			return name, &src, true
		}
	}

	return "", nil, false
}

// GetAllSources 获取所有数据源
// 返回：
//   - map[string]*SourceConfig: 源名称 -> 源配置
func (c *Config) GetAllSources() map[string]*SourceConfig {
	result := make(map[string]*SourceConfig)
	for name, src := range c.Sources {
		srcCopy := src
		result[name] = &srcCopy
	}
	return result
}

// AddSource 添加数据源
// 参数：
//   - name: 源名称
//   - src: 源配置
func (c *Config) AddSource(name string, src SourceConfig) {
	if c.Sources == nil {
		c.Sources = make(map[string]SourceConfig)
	}
	c.Sources[name] = src
}

// RemoveSource 移除数据源
// 参数：
//   - name: 源名称或别名
//
// 返回：
//   - bool: 是否找到并移除
func (c *Config) RemoveSource(name string) bool {
	// 先按名称查找
	if _, ok := c.Sources[name]; ok {
		delete(c.Sources, name)
		return true
	}

	// 再按别名查找
	for key, src := range c.Sources {
		if src.Alias == name {
			delete(c.Sources, key)
			return true
		}
	}

	return false
}

// HasSource 检查数据源是否存在
// 参数：
//   - name: 源名称或别名
//
// 返回：
//   - bool: 是否存在
func (c *Config) HasSource(name string) bool {
	_, _, ok := c.GetSource(name)
	return ok
}
