// 本文件实现 GitHub 用户公钥拉取，支持可选 Token 与重试。
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rocuae/importsshkey/internal/domain"
)

// GitHubFetcher GitHub 公钥拉取器
type GitHubFetcher struct {
	// client 复用连接池的 HTTP 客户端
	client *http.Client
	// token 可选 GitHub API Token，用于提升限额
	token string
	// maxRetries 429/5xx 与网络错误的最大重试次数
	maxRetries int
}

// NewGitHubFetcher 创建 GitHub 拉取器
func NewGitHubFetcher(client *http.Client, token string, maxRetries int) *GitHubFetcher {
	return &GitHubFetcher{client: client, token: token, maxRetries: maxRetries}
}

// Fetch 从 GitHub 拉取用户的 SSH 公钥
func (g *GitHubFetcher) Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error) {
	user := params["User"]
	if user == "" {
		return nil, fmt.Errorf("user parameter is required")
	}

	apiURL := fmt.Sprintf("https://api.github.com/users/%s/keys?per_page=100", url.PathEscape(user))
	headers := map[string]string{"Accept": "application/vnd.github.v3+json"}
	if g.token != "" {
		headers["Authorization"] = "Bearer " + g.token
	}

	resp, err := getWithRetry(ctx, g.client, apiURL, headers, g.maxRetries)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	body, err := readLimited(resp)
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
		entry, err := domain.NewKeyEntry(k.Key, "github", user)
		if err != nil {
			// 跳过无效公钥，但保留其余可用的 key
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
