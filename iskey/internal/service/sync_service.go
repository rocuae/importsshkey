package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/domain"
	"github.com/rocuae/importsshkey/internal/manager"
)

// SyncResult 同步操作的结果
type SyncResult struct {
	// Added 新增的指纹列表
	Added []string
	// Removed 移除的指纹列表
	Removed []string
	// Skipped 已存在且未变更的指纹列表
	Skipped []string
	// Errors 同步过程中的错误
	Errors []error
}

// SyncService 全量同步服务
type SyncService struct {
	cfg     *config.Config
	manager *manager.Manager
}

// NewSyncService 创建同步服务
// 参数：
//   - cfg: 全局配置
//   - mgr: authorized_keys 管理器
//
// 返回：
//   - *SyncService: 服务实例
func NewSyncService(cfg *config.Config, mgr *manager.Manager) *SyncService {
	return &SyncService{cfg: cfg, manager: mgr}
}

// Run 执行全量同步
// 参数：
//   - ctx: 上下文
//   - sources: 要同步的源别名列表（空表示全部）
//   - prune: 是否清理孤立条目
//
// 返回：
//   - *SyncResult: 同步结果
//   - error: 执行错误
func (s *SyncService) Run(ctx context.Context, sources []string, prune bool) (*SyncResult, error) {
	result := &SyncResult{}

	// 获取远程公钥
	var remoteEntries []*domain.KeyEntry
	for name, srcCfg := range s.cfg.Sources {
		if srcCfg.Enabled != nil && !*srcCfg.Enabled {
			continue
		}
		// 筛选指定源
		if len(sources) > 0 {
			found := false
			for _, s := range sources {
				if srcCfg.Alias == s || name == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 拉取该源下所有用户的公钥
		// TODO: 需要从配置或命令行获取用户列表
		_ = srcCfg
	}

	// 计算差异
	local, err := s.manager.Load()
	if err != nil {
		return nil, fmt.Errorf("load local keys: %w", err)
	}

	toAdd, toRemove, unchanged := manager.Diff(local, remoteEntries)

	// 添加新条目
	for _, entry := range toAdd {
		if _, err := s.manager.Add(entry, false); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Added = append(result.Added, entry.Fingerprint)
	}

	// 删除远程已移除的条目
	for _, entry := range toRemove {
		if _, err := s.manager.RemoveByTarget("", "", entry.Fingerprint); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Removed = append(result.Removed, entry.Fingerprint)
	}

	// 未变更的条目
	for _, entry := range unchanged {
		result.Skipped = append(result.Skipped, entry.Fingerprint)
	}

	// 清理孤立条目（prune）
	// TODO: 清理不在配置文件中的 iskey 管理条目
	_ = prune

	return result, nil
}

// SyncWithTimeout 带超时的同步
func (s *SyncService) SyncWithTimeout(ctx context.Context, sources []string, prune bool, timeout time.Duration) (*SyncResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.Run(ctx, sources, prune)
}
