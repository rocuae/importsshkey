// 本文件实现 list 服务：读取托管条目并按源筛选。
package service

import (
	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/domain"
	"github.com/rocuae/importsshkey/internal/fetcher"
	"github.com/rocuae/importsshkey/internal/manager"
)

// ListResult 列表操作的结果
type ListResult struct {
	// Entries 公钥条目列表
	Entries []*domain.KeyEntry
	// Total 条目总数
	Total int
}

// ListService 列出公钥服务
type ListService struct {
	// cfg 全局配置（用于别名解析）
	cfg *config.Config
	// manager authorized_keys 文件管理器
	manager *manager.Manager
}

// NewListService 创建列表服务
// 参数：
//   - cfg: 全局配置
//   - mgr: authorized_keys 管理器
//
// 返回：
//   - *ListService: 服务实例
func NewListService(cfg *config.Config, mgr *manager.Manager) *ListService {
	return &ListService{cfg: cfg, manager: mgr}
}

// Run 执行列表查询。
// showFingerprint 仅为保持 CLI 接口兼容，实际展示方式由调用方决定。
// 参数：
//   - sourceFilter: 源筛选（空表示全部）
//
// 返回：
//   - *ListResult: 查询结果
//   - error: 执行错误
func (s *ListService) Run(sourceFilter string, showFingerprint bool) (*ListResult, error) {
	if sourceFilter != "" {
		if name, _, err := fetcher.ResolveAlias(s.cfg, sourceFilter); err == nil {
			sourceFilter = name
		} else if domain.ValidateSource(sourceFilter) != nil {
			return nil, err
		}
	}
	entries, err := s.manager.List(sourceFilter)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Entries: entries,
		Total:   len(entries),
	}, nil
}
