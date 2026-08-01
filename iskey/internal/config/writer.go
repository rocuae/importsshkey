package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
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
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// DefaultConfig 返回默认配置
// 返回：
//   - *Config: 默认配置对象
func DefaultConfig() *Config {
	enabled := true
	return &Config{
		Defaults: Defaults{
			Output:     "~/.ssh/authorized_keys",
			Timeout:    10,
			MaxRetries: 3,
		},
		Credentials: make(map[string]Credential),
		Sources: map[string]SourceConfig{
			"github": {
				Alias:       "gh",
				URLTemplate: "https://api.github.com/users/{{ .User }}/keys",
				Format:      "github_json",
				Enabled:     &enabled,
			},
			"launchpad": {
				Alias:       "lp",
				URLTemplate: "https://launchpad.net/~{{ .User }}/+sshkeys",
				Format:      "plaintext",
				Enabled:     &enabled,
			},
		},
	}
}
