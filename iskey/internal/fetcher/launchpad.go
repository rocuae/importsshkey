// 本文件实现 Launchpad 用户公钥拉取，并解析纯文本响应。
package fetcher

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rocuae/importsshkey/internal/domain"
)

// LaunchpadFetcher Launchpad 公钥拉取器
type LaunchpadFetcher struct {
	// client 复用连接池的 HTTP 客户端
	client *http.Client
	// maxRetries 429/5xx 与网络错误的最大重试次数
	maxRetries int
}

// NewLaunchpadFetcher 创建 Launchpad 拉取器
func NewLaunchpadFetcher(client *http.Client, maxRetries int) *LaunchpadFetcher {
	return &LaunchpadFetcher{client: client, maxRetries: maxRetries}
}

// Fetch 从 Launchpad 拉取用户的 SSH 公钥
func (l *LaunchpadFetcher) Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error) {
	user := params["User"]
	if user == "" {
		return nil, fmt.Errorf("user parameter is required")
	}

	rawURL := fmt.Sprintf("https://launchpad.net/~%s/+sshkeys", url.PathEscape(user))
	resp, err := getWithRetry(ctx, l.client, rawURL, nil, l.maxRetries)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("launchpad returned %d", resp.StatusCode)
	}
	body, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	return parsePlaintextKeys(bytes.NewReader(body), "launchpad", user)
}

// parsePlaintextKeys 解析纯文本格式的公钥列表
func parsePlaintextKeys(r io.Reader, source, user string) ([]*domain.KeyEntry, error) {
	var entries []*domain.KeyEntry
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := domain.NewKeyEntry(line, source, user)
		if err != nil {
			// 跳过无效公钥，但保留其余可用的 key
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
