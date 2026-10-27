// 本文件负责按源类型创建 Fetcher，并处理 URL 模板、认证与 HTTP 客户端选项。
package fetcher

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/rocuae/importsshkey/internal/config"
)

// Options HTTP 拉取选项
type Options struct {
	// Timeout 单次请求超时
	Timeout time.Duration
	// InsecureSkipVerify 跳过 TLS 证书校验（仅限自签名内网）
	InsecureSkipVerify bool
	// MaxRetries 可重试错误的最大重试次数
	MaxRetries int
}

// Factory 根据源配置创建对应的 Fetcher。
// sourceName 是解析后的规范源名（github/launchpad/配置中的名称）。
func Factory(source *config.SourceConfig, sourceName string, creds *config.Credential, user string, vars map[string]string, opts Options) (Fetcher, error) {
	client, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}

	switch sourceName {
	case "github":
		token := ""
		if creds != nil {
			token = creds.ResolvedValue
		}
		return NewGitHubFetcher(client, token, opts.MaxRetries), nil

	case "launchpad":
		return NewLaunchpadFetcher(client, opts.MaxRetries), nil
	}

	params := make(map[string]string, len(source.DefaultVars)+len(vars)+2)
	for k, v := range source.DefaultVars {
		params[k] = v
	}
	for k, v := range vars {
		params[k] = v
	}
	params["User"] = user
	params["Source"] = sourceName

	rawURL, err := resolveURL(source, params)
	if err != nil {
		return nil, err
	}

	format := source.Format
	if format == "" {
		format = "plaintext"
	}

	fetcher := NewHTTPFetcher(client, rawURL, format).WithRetries(opts.MaxRetries)

	if creds != nil {
		switch creds.Type {
		case "bearer":
			fetcher.WithToken(creds.ResolvedValue)
		case "basic":
			fetcher.WithBasicAuth(creds.ResolvedUsername, creds.ResolvedPassword)
		}
	}

	return fetcher, nil
}

// newHTTPClient 构建带超时和 TLS 选项的 HTTP 客户端。
func newHTTPClient(opts Options) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.InsecureSkipVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// resolveURL 解析模板 URL，合并默认变量和命令行变量。
func resolveURL(source *config.SourceConfig, params map[string]string) (string, error) {
	if source.URL == "" {
		return "", fmt.Errorf("source URL is empty")
	}
	if !strings.Contains(source.URL, "{{") {
		if err := validateURL(source.URL); err != nil {
			return "", err
		}
		return source.URL, nil
	}

	tmpl, err := template.New("url").Funcs(template.FuncMap{
		"urlquery":   url.QueryEscape,
		"pathEscape": url.PathEscape,
	}).Parse(source.URL)
	if err != nil {
		return "", fmt.Errorf("invalid URL template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}
	rendered := buf.String()
	if err := validateURL(rendered); err != nil {
		return "", err
	}
	return rendered, nil
}

// validateURL 限制模板渲染后的地址必须是带 host 的 http/https URL。
func validateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid URL %q: scheme must be http or https", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid URL %q: host is required", raw)
	}
	return nil
}

// ResolveAlias 根据别名查找源配置。
// 内置源返回规范名 github/launchpad，避免调用方依赖 gh/lp 别名。
func ResolveAlias(cfg *config.Config, alias string) (string, *config.SourceConfig, error) {
	name, src, ok := cfg.GetSource(alias)
	if !ok {
		if builtin, exists := GetBuiltinSource(alias); exists {
			return builtin.Alias, builtin, nil
		}
		return "", nil, fmt.Errorf("source not found: %s", alias)
	}
	return name, src, nil
}

// GetBuiltinSource 获取内置数据源配置。内置源无需在配置文件中定义。
func GetBuiltinSource(alias string) (*config.SourceConfig, bool) {
	switch alias {
	case "gh", "github":
		return &config.SourceConfig{
			Alias:  "github",
			URL:    "https://api.github.com/users/{{ .User }}/keys",
			Format: "github_json",
		}, true
	case "lp", "launchpad":
		return &config.SourceConfig{
			Alias:  "launchpad",
			URL:    "https://launchpad.net/~{{ .User }}/+sshkeys",
			Format: "plaintext",
		}, true
	}
	return nil, false
}

// IsBuiltinSource 检查是否为内置源
func IsBuiltinSource(alias string) bool {
	_, ok := GetBuiltinSource(alias)
	return ok
}
