// 本文件实现通用 HTTP 公钥拉取，支持 plaintext、github_json 与自动识别。
package fetcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rocuae/importsshkey/internal/domain"
)

// HTTPFetcher 通用 HTTP 公钥拉取器，支持 plaintext、github_json 和 auto。
type HTTPFetcher struct {
	// client 复用连接池的 HTTP 客户端
	client *http.Client
	// url 已渲染模板变量的最终请求地址
	url string
	// format 响应解析格式：plaintext/github_json/auto
	format string
	// token Bearer 认证令牌
	token string
	// basicUser Basic Auth 用户名
	basicUser string
	// basicPass Basic Auth 密码
	basicPass string
	// headers 调用方追加的自定义请求头
	headers map[string]string
	// maxRetries 可重试错误的最大重试次数
	maxRetries int
}

// NewHTTPFetcher 创建通用 HTTP 拉取器
func NewHTTPFetcher(client *http.Client, rawURL, format string) *HTTPFetcher {
	return &HTTPFetcher{client: client, url: rawURL, format: format}
}

// WithToken 设置 Bearer Token
func (h *HTTPFetcher) WithToken(token string) *HTTPFetcher {
	h.token = token
	return h
}

// WithBasicAuth 设置 Basic Auth
func (h *HTTPFetcher) WithBasicAuth(username, password string) *HTTPFetcher {
	h.basicUser = username
	h.basicPass = password
	return h
}

// WithHeaders 设置自定义请求头
func (h *HTTPFetcher) WithHeaders(headers map[string]string) *HTTPFetcher {
	h.headers = headers
	return h
}

// WithRetries 设置可重试错误的最大重试次数
func (h *HTTPFetcher) WithRetries(maxRetries int) *HTTPFetcher {
	h.maxRetries = maxRetries
	return h
}

// Fetch 从 HTTP 端点拉取公钥
func (h *HTTPFetcher) Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error) {
	headers := make(map[string]string, len(h.headers)+1)
	switch {
	case h.token != "":
		headers["Authorization"] = "Bearer " + h.token
	case h.basicUser != "" || h.basicPass != "":
		encoded := base64.StdEncoding.EncodeToString([]byte(h.basicUser + ":" + h.basicPass))
		headers["Authorization"] = "Basic " + encoded
	}
	for key, value := range h.headers {
		headers[key] = value
	}

	resp, err := getWithRetry(ctx, h.client, h.url, headers, h.maxRetries)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := readLimited(resp)
	if err != nil {
		return nil, err
	}

	user := params["User"]
	source := params["Source"]
	if source == "" {
		source = "custom"
	}

	switch h.format {
	case "github_json":
		return parseGitHubJSON(bytes.NewReader(body), source, user)
	case "auto":
		trimmed := bytes.TrimSpace(body)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			return parseGitHubJSON(bytes.NewReader(trimmed), source, user)
		}
		return parsePlaintextKeys(bytes.NewReader(trimmed), source, user)
	default:
		return parsePlaintextKeys(bytes.NewReader(body), source, user)
	}
}

// parseGitHubJSON 解析 GitHub API 格式的公钥
func parseGitHubJSON(r io.Reader, source, user string) ([]*domain.KeyEntry, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var keys []struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &keys); err != nil {
		return nil, err
	}

	entries := make([]*domain.KeyEntry, 0, len(keys))
	for _, k := range keys {
		entry, err := domain.NewKeyEntry(k.Key, source, user)
		if err != nil {
			// 跳过无效公钥，但保留其余可用的 key
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
