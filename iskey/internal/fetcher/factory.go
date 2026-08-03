package fetcher

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/rocuae/importsshkey/internal/config"
)

// Factory 根据源配置创建对应的 Fetcher
// 参数：
//   - source: 数据源配置
//   - creds: 凭据配置（可为 nil）
//   - user: 目标用户名
//
// 返回：
//   - Fetcher: 拉取器实例
//   - error: 配置错误
func Factory(source *config.SourceConfig, creds *config.Credential, user string) (Fetcher, error) {
	client := &http.Client{}

	// 处理内置源别名
	alias := source.Alias

	switch alias {
	case "gh", "github":
		token := ""
		if creds != nil && creds.ResolvedValue != "" {
			token = creds.ResolvedValue
		}
		return NewGitHubFetcher(client, token), nil

	case "lp", "launchpad":
		return NewLaunchpadFetcher(client), nil

	default:
		// 自定义 URL
		url, err := resolveURL(source, user)
		if err != nil {
			return nil, err
		}

		format := source.Format
		if format == "" {
			format = "plaintext"
		}

		fetcher := NewHTTPFetcher(client, url, format)

		// 注入认证信息
		if creds != nil {
			switch creds.Type {
			case "bearer":
				fetcher.WithToken(creds.ResolvedValue)
			}
		}

		return fetcher, nil
	}
}

// resolveURL 解析模板 URL
// 参数：
//   - source: 数据源配置
//   - user: 用户名
//
// 返回：
//   - string: 解析后的 URL
//   - error: 模板解析错误
func resolveURL(source *config.SourceConfig, user string) (string, error) {
	if source.URL == "" {
		return "", fmt.Errorf("source URL is empty")
	}

	// 检测是否包含模板变量
	if !strings.Contains(source.URL, "{{") {
		return source.URL, nil
	}

	tmpl, err := template.New("url").Parse(source.URL)
	if err != nil {
		return "", fmt.Errorf("invalid URL template: %w", err)
	}

	params := map[string]string{"User": user}
	for k, v := range source.DefaultVars {
		params[k] = v
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

// ResolveAlias 根据别名查找源配置
// 参数：
//   - cfg: 全局配置
//   - alias: 源别名（如 "gh", "work"）
//
// 返回：
//   - string: 源名称
//   - *SourceConfig: 源配置
//   - error: 未找到
func ResolveAlias(cfg *config.Config, alias string) (string, *config.SourceConfig, error) {
	name, src, ok := cfg.GetSource(alias)
	if !ok {
		// 检查内置源
		if builtin, exists := GetBuiltinSource(alias); exists {
			return alias, builtin, nil
		}
		return "", nil, fmt.Errorf("source not found: %s", alias)
	}
	return name, src, nil
}

// GetBuiltinSource 获取内置数据源配置
// 内置源无需在配置文件中定义，始终可用
// 参数：
//   - alias: 源别名
//
// 返回：
//   - *SourceConfig: 源配置
//   - bool: 是否为内置源
func GetBuiltinSource(alias string) (*config.SourceConfig, bool) {
	switch alias {
	case "gh", "github":
		return &config.SourceConfig{
			Alias:  "gh",
			URL:    "https://api.github.com/users/{{ .User }}/keys",
			Format: "github_json",
		}, true
	case "lp", "launchpad":
		return &config.SourceConfig{
			Alias:  "lp",
			URL:    "https://launchpad.net/~{{ .User }}/+sshkeys",
			Format: "plaintext",
		}, true
	}
	return nil, false
}

// IsBuiltinSource 检查是否为内置源
// 参数：
//   - alias: 源别名
//
// 返回：
//   - bool: 是否为内置源
func IsBuiltinSource(alias string) bool {
	_, ok := GetBuiltinSource(alias)
	return ok
}
