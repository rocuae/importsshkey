// fetcher 包定义数据拉取接口和实现
package fetcher

import (
	"context"

	"github.com/rocuae/importsshkey/internal/domain"
)

// Fetcher 定义拉取公钥列表的能力
type Fetcher interface {
	// Fetch 拉取指定用户的公钥列表
	// 参数：
	//   - ctx: 上下文，用于超时控制和取消
	//   - params: 模板变量，如 {"User": "zhangsan", "Team": "infra"}
	// 返回：
	//   - []*domain.KeyEntry: 公钥列表
	//   - error: 网络错误、解析错误或认证失败
	Fetch(ctx context.Context, params map[string]string) ([]*domain.KeyEntry, error)
}
