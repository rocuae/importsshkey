// 本文件负责配置文件的原子写入与稳定排序的 YAML 序列化。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SaveToFile 原子保存配置到文件，并保留 .bak 备份。
func (c *Config) SaveToFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if old, err := os.ReadFile(path); err == nil {
		if writeErr := os.WriteFile(path+".bak", old, 0600); writeErr != nil {
			return fmt.Errorf("backup config: %w", writeErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config for backup: %w", err)
	}

	data := marshalConfig(c)
	tmp, err := os.CreateTemp(dir, ".sources-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0600); err != nil {
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	success = true
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("set config permissions: %w", err)
	}
	return nil
}

// marshalConfig 将配置序列化为带注释、顺序稳定的 YAML。
func marshalConfig(cfg *Config) []byte {
	var b strings.Builder
	b.WriteString("# iskey 配置文件\n")
	b.WriteString("# 默认路径: ~/.config/iskey/sources.yaml\n")
	b.WriteString("# 使用 iskey config init 生成此文件\n\n")

	hasDefaults := cfg.Defaults.Output != "" || cfg.Defaults.Timeout != 0 ||
		cfg.Defaults.MaxRetries != 0 || cfg.Defaults.InsecureSkipVerify
	if hasDefaults {
		b.WriteString("# 全局默认配置\n")
		b.WriteString("defaults:\n")
		if cfg.Defaults.Output != "" {
			fmt.Fprintf(&b, "  output: %s\n", yamlString(cfg.Defaults.Output))
		}
		if cfg.Defaults.Timeout != 0 {
			fmt.Fprintf(&b, "  timeout: %d\n", cfg.Defaults.Timeout)
		}
		if cfg.Defaults.MaxRetries != 0 {
			fmt.Fprintf(&b, "  max_retries: %d\n", cfg.Defaults.MaxRetries)
		}
		if cfg.Defaults.InsecureSkipVerify {
			b.WriteString("  insecure_skip_verify: true\n")
		}
	} else {
		b.WriteString("# 全局默认配置（选填，系统已有默认值）\n")
		b.WriteString("# defaults:\n")
		b.WriteString("#   output: \"~/.ssh/authorized_keys\"\n")
		b.WriteString("#   timeout: 10\n")
		b.WriteString("#   max_retries: 3\n")
	}
	b.WriteString("\n")

	if len(cfg.Credentials) > 0 {
		b.WriteString("# 认证凭据定义\n")
		b.WriteString("# 重要: Token 值必须从环境变量读取，YAML 中严禁硬编码\n")
		b.WriteString("credentials:\n")
		for _, name := range sortedKeys(cfg.Credentials) {
			cred := cfg.Credentials[name]
			fmt.Fprintf(&b, "  %s:\n", yamlKey(name))
			fmt.Fprintf(&b, "    type: %s\n", yamlString(cred.Type))
			if cred.ValueFromEnv != "" {
				fmt.Fprintf(&b, "    value_from_env: %s\n", yamlString(cred.ValueFromEnv))
			}
			if cred.UsernameFromEnv != "" {
				fmt.Fprintf(&b, "    username_from_env: %s\n", yamlString(cred.UsernameFromEnv))
			}
			if cred.PasswordFromEnv != "" {
				fmt.Fprintf(&b, "    password_from_env: %s\n", yamlString(cred.PasswordFromEnv))
			}
		}
	} else {
		b.WriteString("# 认证凭据定义（选填，用于私有源）\n")
		b.WriteString("# credentials:\n")
		b.WriteString("#   my-token:\n")
		b.WriteString("#     type: bearer\n")
		b.WriteString("#     value_from_env: GITLAB_PRIVATE_TOKEN\n")
	}
	b.WriteString("\n")

	if len(cfg.Sources) > 0 {
		b.WriteString("# 数据源定义\n")
		b.WriteString("# 内置源 gh/lp 始终可用，无需配置\n")
		b.WriteString("# 使用方式: iskey add <alias>:<username>\n")
		b.WriteString("sources:\n")
		for _, name := range sortedKeys(cfg.Sources) {
			src := cfg.Sources[name]
			fmt.Fprintf(&b, "  %s:\n", yamlKey(name))
			fmt.Fprintf(&b, "    alias: %s\n", yamlString(src.Alias))
			fmt.Fprintf(&b, "    url: %s\n", yamlString(src.URL))
			fmt.Fprintf(&b, "    format: %s\n", yamlString(src.Format))
			if src.AuthRef != "" {
				fmt.Fprintf(&b, "    auth_ref: %s\n", yamlString(src.AuthRef))
			}
			if len(src.DefaultVars) > 0 {
				b.WriteString("    default_vars:\n")
				for _, key := range sortedKeys(src.DefaultVars) {
					fmt.Fprintf(&b, "      %s: %s\n", yamlKey(key), yamlString(src.DefaultVars[key]))
				}
			}
		}
	} else {
		b.WriteString("# 数据源定义（选填）\n")
		b.WriteString("# 内置源 gh/lp 始终可用，无需配置\n")
		b.WriteString("sources: {}\n")
	}

	return []byte(b.String())
}

// sortedKeys 返回 map 的有序键，保证生成的 YAML 可重复。
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// yamlKey 在安全时输出裸键，否则退化为带引号的字符串。
func yamlKey(s string) string {
	if s != "" && isPlainYAML(s) {
		return s
	}
	return yamlString(s)
}

// yamlString 将字符串转义为双引号 YAML 标量。
func yamlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// isPlainYAML 判断字符串能否作为 YAML 裸键使用。
func isPlainYAML(s string) bool {
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			continue
		case i > 0 && (r == '+' || r == '@'):
			continue
		default:
			return false
		}
	}
	return true
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Output:     "~/.ssh/authorized_keys",
			Timeout:    10,
			MaxRetries: 3,
		},
		Credentials: make(map[string]Credential),
		Sources:     make(map[string]SourceConfig),
	}
}
