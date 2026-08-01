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
	data, err := marshalConfig(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// marshalConfig 将配置序列化为 YAML
// 参数：
//   - cfg: 配置对象
//
// 返回：
//   - []byte: YAML 数据
//   - error: 序列化错误
func marshalConfig(cfg *Config) ([]byte, error) {
	// 如果是空配置，返回模板内容
	if cfg.Defaults == (Defaults{}) && len(cfg.Credentials) == 0 && len(cfg.Sources) == 0 {
		return []byte(defaultConfigTemplate()), nil
	}

	// 否则使用标准 YAML 序列化
	return yaml.Marshal(cfg)
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

// defaultConfigTemplate 返回默认配置模板
func defaultConfigTemplate() string {
	return `# iskey 配置文件
# 默认路径: ~/.config/iskey/sources.yaml
# 使用 iskey config init 生成此文件

# 全局默认配置（选填，系统已有默认值）
# defaults:
#   output: "~/.ssh/authorized_keys"  # 默认值
#   timeout: 10                        # 默认值
#   max_retries: 3                     # 默认值

# 认证凭据定义（选填，用于私有源）
# 重要: Token 值必须从环境变量读取，YAML 中严禁硬编码
# credentials:
#   my-gitlab-token:
#     type: bearer
#     value_from_env: GITLAB_PRIVATE_TOKEN

# 数据源定义（选填）
# 内置源 gh/lp 始终可用，无需配置
# 使用方式: iskey add <alias>:<username>
sources: {}
`
}
