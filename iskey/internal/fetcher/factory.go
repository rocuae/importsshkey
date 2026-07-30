package fetcher

import (
	"bytes"
	"fmt"
	"net/http"
	"text/template"

	"github.com/importsshkey/importsshkey/internal/config"
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
	// 优先使用静态 URL
	if source.URL != "" {
		return source.URL, nil
	}

	// 使用模板 URL
	if source.URLTemplate == "" {
		return "", fmt.Errorf("source URL is empty")
	}

	tmpl, err := template.New("url").Parse(source.URLTemplate)
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
		return "", nil, fmt.Errorf("source not found: %s", alias)
	}
	return name, src, nil
}
