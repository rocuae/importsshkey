package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/importsshkey/importsshkey/internal/domain"
)

// GitHubFetcher GitHub 公钥拉取器
// 通过 GitHub API 获取用户的 SSH 公钥
type GitHubFetcher struct {
	// client HTTP 客户端
	client *http.Client
	// token GitHub API Token（可选，提升限额）
	token string
}

// NewGitHubFetcher 创建 GitHub 拉取器
// 参数：
//   - client: HTTP 客户端
//   - token: GitHub Token（可选）
//
// 返回：
//   - *GitHubFetcher: 拉取器实例
func NewGitHubFetcher(client *http.Client, token string) *GitHubFetcher {
	return &GitHubFetcher{client: client, token: token}
}

// Fetch 从 GitHub 拉取用户的 SSH 公钥
// 参数：
//   - ctx: 上下文
//   - params: 必须包含 "User" 键
//
// 返回：
//   - []*domain.KeyEntry: 公钥列表
//   - error: 网络错误或解析错误
func (g *GitHubFetcher) Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error) {
	user := params["User"]
	if user == "" {
		return nil, fmt.Errorf("User parameter is required")
	}

	url := fmt.Sprintf("https://api.github.com/users/%s/keys", user)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
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
			// 跳过无效公钥
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
