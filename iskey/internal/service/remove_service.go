// 本文件实现 remove 服务：按目标、指纹或整源删除托管条目。
package service

import (
	"fmt"
	"strings"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/domain"
	"github.com/rocuae/importsshkey/internal/fetcher"
	"github.com/rocuae/importsshkey/internal/manager"
)

// RemoveOptions 删除命令的输入参数
type RemoveOptions struct {
	// Source 源别名或名称（按 source+user 删除时必填）
	Source string
	// User 用户名（按 source+user 删除时必填）
	User string
	// Fingerprint 按指纹删除
	Fingerprint string
	// AllFromSource 删除指定源下的全部公钥
	AllFromSource bool
	// DryRun 只计算不写文件
	DryRun bool
}

// RemoveResult 删除命令的结果
type RemoveResult struct {
	// Status success 或 not_found
	Status string `json:"Status"`
	// Action removed/dry-run/not_found
	Action string `json:"Action"`
	// Source 解析后的源名
	Source string `json:"Source"`
	// User 目标用户名（按指纹删除时可能为空）
	User string `json:"User"`
	// Fingerprint 按指纹删除时回显的指纹
	Fingerprint string `json:"Fingerprint"`
	// Removed 实际（或 dry-run 预计）删除的数量
	Removed int `json:"Removed"`
	// Keys 被删除条目的明细
	Keys []KeyResult `json:"Keys"`
	// Error 保留字段，当前成功路径为 nil
	Error *string `json:"Error"`
}

// RemoveService 删除公钥服务
type RemoveService struct {
	// cfg 全局配置（用于别名解析）
	cfg *config.Config
	// manager authorized_keys 文件管理器
	manager *manager.Manager
}

// NewRemoveService 创建删除服务
func NewRemoveService(cfg *config.Config, mgr *manager.Manager) *RemoveService {
	return &RemoveService{cfg: cfg, manager: mgr}
}

// Run 删除匹配的公钥
func (s *RemoveService) Run(opts RemoveOptions) (*RemoveResult, error) {
	sourceName := opts.Source
	if sourceName != "" && !strings.HasPrefix(sourceName, "http://") && !strings.HasPrefix(sourceName, "https://") {
		name, _, err := fetcher.ResolveAlias(s.cfg, sourceName)
		if err != nil {
			// 允许删除已从配置中移除、但仍在 authorized_keys 中的源（含 URL 目标的 "url"）
			if domain.ValidateSource(sourceName) != nil {
				return nil, err
			}
		} else {
			sourceName = name
		}
	}

	var (
		removed []*domain.KeyEntry
		err     error
	)
	if opts.AllFromSource {
		removed, err = s.manager.RemoveAllFromSource(sourceName, opts.DryRun)
	} else {
		if opts.Fingerprint == "" && (sourceName == "" || opts.User == "") {
			return nil, fmt.Errorf("source and user are required when --fingerprint is not set")
		}
		removed, err = s.manager.RemoveByTarget(sourceName, opts.User, opts.Fingerprint, opts.DryRun)
	}
	if err != nil {
		return nil, err
	}

	result := &RemoveResult{
		Status:      "success",
		Action:      "removed",
		Source:      sourceName,
		User:        opts.User,
		Fingerprint: opts.Fingerprint,
		Removed:     len(removed),
		Keys:        make([]KeyResult, 0, len(removed)),
	}
	if opts.DryRun {
		result.Action = "dry-run"
	}
	for _, entry := range removed {
		action := "removed"
		if opts.DryRun {
			action = "would-remove"
		}
		result.Keys = append(result.Keys, KeyResult{
			Fingerprint: entry.Fingerprint,
			Action:      action,
		})
	}
	if len(removed) == 0 {
		result.Status = "not_found"
		result.Action = "not_found"
	}
	return result, nil
}
