package service

import (
	"github.com/importsshkey/importsshkey/internal/config"
	"github.com/importsshkey/importsshkey/internal/domain"
	"github.com/importsshkey/importsshkey/internal/manager"
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
	cfg     *config.Config
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

// Run 执行列表查询
// 参数：
//   - sourceFilter: 源筛选（空表示全部）
//   - showFingerprint: 是否显示指纹
//
// 返回：
//   - *ListResult: 查询结果
//   - error: 执行错误
func (s *ListService) Run(sourceFilter string, showFingerprint bool) (*ListResult, error) {
	entries, err := s.manager.List(sourceFilter)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Entries: entries,
		Total:   len(entries),
	}, nil
}
