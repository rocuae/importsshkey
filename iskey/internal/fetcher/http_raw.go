package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/importsshkey/importsshkey/internal/domain"
)

// HTTPFetcher 通用 HTTP 公钥拉取器
// 支持 plaintext 和 github_json 两种格式
type HTTPFetcher struct {
	client  *http.Client
	url     string
	format  string
	token   string
	headers map[string]string
}

// NewHTTPFetcher 创建通用 HTTP 拉取器
// 参数：
//   - client: HTTP 客户端
//   - url: 请求 URL（已解析模板变量）
//   - format: 解析格式 (plaintext / github_json)
//
// 返回：
//   - *HTTPFetcher: 拉取器实例
func NewHTTPFetcher(client *http.Client, url, format string) *HTTPFetcher {
	return &HTTPFetcher{client: client, url: url, format: format}
}

// WithToken 设置认证 Token
// 参数：
//   - token: Bearer Token
//
// 返回：
//   - *HTTPFetcher: 自身（支持链式调用）
func (h *HTTPFetcher) WithToken(token string) *HTTPFetcher {
	h.token = token
	return h
}

// WithHeaders 设置自定义请求头
// 参数：
//   - headers: 请求头键值对
//
// 返回：
//   - *HTTPFetcher: 自身（支持链式调用）
func (h *HTTPFetcher) WithHeaders(headers map[string]string) *HTTPFetcher {
	h.headers = headers
	return h
}

// Fetch 从 HTTP 端点拉取公钥
// 参数：
//   - ctx: 上下文
//   - params: 模板变量（URL 已预处理）
//
// 返回：
//   - []*domain.KeyEntry: 公钥列表
//   - error: 网络错误或解析错误
func (h *HTTPFetcher) Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", h.url, nil)
	if err != nil {
		return nil, err
	}

	// 设置认证头
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	// 设置自定义请求头
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	user := params["User"]
	source := params["Source"]
	if source == "" {
		source = "custom"
	}

	switch h.format {
	case "github_json":
		return parseGitHubJSON(resp.Body, source, user)
	default:
		return parsePlaintextKeys(resp.Body, source, user)
	}
}

// parseGitHubJSON 解析 GitHub API 格式的公钥
// 参数：
//   - r: 读取器
//   - source: 来源标识
//   - user: 用户名
//
// 返回：
//   - []*domain.KeyEntry: 公钥列表
//   - error: 解析错误
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
			// 跳过无效公钥
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
