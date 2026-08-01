package service

import (
	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/manager"
)

// RemoveResult 移除操作的结果
type RemoveResult struct {
	// Status 操作状态
	Status string
	// Removed 删除的条目数
	Removed int
	// Error 错误信息（如有）
	Error error
}

// RemoveService 移除公钥服务
type RemoveService struct {
	cfg     *config.Config
	manager *manager.Manager
}

// NewRemoveService 创建移除服务
// 参数：
//   - cfg: 全局配置
//   - mgr: authorized_keys 管理器
//
// 返回：
//   - *RemoveService: 服务实例
func NewRemoveService(cfg *config.Config, mgr *manager.Manager) *RemoveService {
	return &RemoveService{cfg: cfg, manager: mgr}
}

// Run 执行移除操作
// 参数：
//   - source: 来源标识
//   - user: 用户名
//   - fingerprint: 指纹（优先使用）
//
// 返回：
//   - *RemoveResult: 操作结果
//   - error: 执行错误
func (s *RemoveService) Run(source, user, fingerprint string) (*RemoveResult, error) {
	removed, err := s.manager.RemoveByTarget(source, user, fingerprint)
	if err != nil {
		return nil, err
	}

	status := "success"
	if removed == 0 {
		status = "not_found"
	}

	return &RemoveResult{
		Status:  status,
		Removed: removed,
	}, nil
}
