package fetcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/importsshkey/importsshkey/internal/domain"
)

// LaunchpadFetcher Launchpad 公钥拉取器
// 通过 Launchpad API 获取用户的 SSH 公钥
type LaunchpadFetcher struct {
	client *http.Client
}

// NewLaunchpadFetcher 创建 Launchpad 拉取器
// 参数：
//   - client: HTTP 客户端
//
// 返回：
//   - *LaunchpadFetcher: 拉取器实例
func NewLaunchpadFetcher(client *http.Client) *LaunchpadFetcher {
	return &LaunchpadFetcher{client: client}
}

// Fetch 从 Launchpad 拉取用户的 SSH 公钥
// 参数：
//   - ctx: 上下文
//   - params: 必须包含 "User" 键
//
// 返回：
//   - []*domain.KeyEntry: 公钥列表
//   - error: 网络错误或解析错误
func (l *LaunchpadFetcher) Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error) {
	user := params["User"]
	if user == "" {
		return nil, fmt.Errorf("User parameter is required")
	}

	url := fmt.Sprintf("https://launchpad.net/~%s/+sshkeys", user)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Launchpad returned %d", resp.StatusCode)
	}

	return parsePlaintextKeys(resp.Body, "launchpad", user)
}

// parsePlaintextKeys 解析纯文本格式的公钥列表
// 参数：
//   - r: 读取器
//   - source: 来源标识
//   - user: 用户名
//
// 返回：
//   - []*domain.KeyEntry: 公钥列表
//   - error: 解析错误
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
			// 跳过无效公钥
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
