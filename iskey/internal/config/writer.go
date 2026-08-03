package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveToFile 保存配置到文件
// 参数：
//   - path: 文件路径
//
// 返回：
//   - error: 写入错误
func (c *Config) SaveToFile(path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// 序列化为 YAML
	data := marshalConfig(c)

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// marshalConfig 将配置序列化为带注释的 YAML
// 参数：
//   - cfg: 配置对象
//
// 返回：
//   - []byte: YAML 数据
func marshalConfig(cfg *Config) []byte {
	var b strings.Builder

	// 头部注释
	b.WriteString("# iskey 配置文件\n")
	b.WriteString("# 默认路径: ~/.config/iskey/sources.yaml\n")
	b.WriteString("# 使用 iskey config init 生成此文件\n")
	b.WriteString("\n")

	// defaults 部分
	hasDefaults := cfg.Defaults != (Defaults{})
	if hasDefaults {
		b.WriteString("# 全局默认配置\n")
		b.WriteString("defaults:\n")
		if cfg.Defaults.Output != "" {
			b.WriteString(fmt.Sprintf("  output: \"%s\"\n", cfg.Defaults.Output))
		}
		if cfg.Defaults.Timeout != 0 {
			b.WriteString(fmt.Sprintf("  timeout: %d\n", cfg.Defaults.Timeout))
		}
		if cfg.Defaults.MaxRetries != 0 {
			b.WriteString(fmt.Sprintf("  max_retries: %d\n", cfg.Defaults.MaxRetries))
		}
	} else {
		b.WriteString("# 全局默认配置（选填，系统已有默认值）\n")
		b.WriteString("# defaults:\n")
		b.WriteString("#   output: \"~/.ssh/authorized_keys\"  # 默认值\n")
		b.WriteString("#   timeout: 10                        # 默认值\n")
		b.WriteString("#   max_retries: 3                     # 默认值\n")
	}
	b.WriteString("\n")

	// credentials 部分
	if len(cfg.Credentials) > 0 {
		b.WriteString("# 认证凭据定义\n")
		b.WriteString("# 重要: Token 值必须从环境变量读取，YAML 中严禁硬编码\n")
		b.WriteString("credentials:\n")
		for name, cred := range cfg.Credentials {
			b.WriteString(fmt.Sprintf("  %s:\n", name))
			b.WriteString(fmt.Sprintf("    type: %s\n", cred.Type))
			if cred.ValueFromEnv != "" {
				b.WriteString(fmt.Sprintf("    value_from_env: %s\n", cred.ValueFromEnv))
			}
			if cred.UsernameFromEnv != "" {
				b.WriteString(fmt.Sprintf("    username_from_env: %s\n", cred.UsernameFromEnv))
			}
			if cred.PasswordFromEnv != "" {
				b.WriteString(fmt.Sprintf("    password_from_env: %s\n", cred.PasswordFromEnv))
			}
		}
	} else {
		b.WriteString("# 认证凭据定义（选填，用于私有源）\n")
		b.WriteString("# 重要: Token 值必须从环境变量读取，YAML 中严禁硬编码\n")
		b.WriteString("# credentials:\n")
		b.WriteString("#   my-gitlab-token:\n")
		b.WriteString("#     type: bearer\n")
		b.WriteString("#     value_from_env: GITLAB_PRIVATE_TOKEN\n")
	}
	b.WriteString("\n")

	// sources 部分
	if len(cfg.Sources) > 0 {
		b.WriteString("# 数据源定义\n")
		b.WriteString("# 内置源 gh/lp 始终可用，无需配置\n")
		b.WriteString("# 使用方式: iskey add <alias>:<username>\n")
		b.WriteString("sources:\n")
		for name, src := range cfg.Sources {
			b.WriteString(fmt.Sprintf("  %s:\n", name))
			b.WriteString(fmt.Sprintf("    alias: %s\n", src.Alias))
			b.WriteString(fmt.Sprintf("    url: \"%s\"\n", src.URL))
			b.WriteString(fmt.Sprintf("    format: %s\n", src.Format))
			if src.AuthRef != "" {
				b.WriteString(fmt.Sprintf("    auth_ref: %s\n", src.AuthRef))
			}
			if len(src.DefaultVars) > 0 {
				b.WriteString("    default_vars:\n")
				for k, v := range src.DefaultVars {
					b.WriteString(fmt.Sprintf("      %s: %s\n", k, v))
				}
			}
		}
	} else {
		b.WriteString("# 数据源定义（选填）\n")
		b.WriteString("# 内置源 gh/lp 始终可用，无需配置\n")
		b.WriteString("# 使用方式: iskey add <alias>:<username>\n")
		b.WriteString("sources: {}\n")
	}

	return []byte(b.String())
}

// DefaultConfig 返回默认配置
// 返回：
//   - *Config: 默认配置对象
func DefaultConfig() *Config {
	return &Config{
		Defaults:    Defaults{},
		Credentials: make(map[string]Credential),
		Sources:     make(map[string]SourceConfig),
	}
}
