// service 包编排领域模型与基础设施，实现业务逻辑
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/fetcher"
	"github.com/rocuae/importsshkey/internal/manager"
)

// AddResult 添加操作的结果
type AddResult struct {
	// Status 操作状态: success / skipped / error
	Status string
	// Action 执行的动作: added / updated / skipped
	Action string
	// Source 来源标识
	Source string
	// User 用户名
	User string
	// Fingerprint 公钥指纹
	Fingerprint string
	// Line 添加到的行号
	Line int
	// Error 错误信息（如有）
	Error error
}

// AddService 添加公钥服务
type AddService struct {
	cfg     *config.Config
	manager *manager.Manager
}

// NewAddService 创建添加服务
// 参数：
//   - cfg: 全局配置
//   - mgr: authorized_keys 管理器
//
// 返回：
//   - *AddService: 服务实例
func NewAddService(cfg *config.Config, mgr *manager.Manager) *AddService {
	return &AddService{cfg: cfg, manager: mgr}
}

// Run 执行添加操作
// 参数：
//   - ctx: 上下文
//   - sourceAlias: 数据源别名
//   - user: 目标用户名
//   - vars: 模板变量覆盖
//   - force: 是否强制覆盖
//
// 返回：
//   - *AddResult: 操作结果
//   - error: 执行错误
func (s *AddService) Run(ctx context.Context, sourceAlias, user string, vars map[string]string, force bool) (*AddResult, error) {
	// 查找源配置
	_, srcCfg, err := fetcher.ResolveAlias(s.cfg, sourceAlias)
	if err != nil {
		return nil, err
	}

	// 创建 Fetcher
	f, err := fetcher.Factory(srcCfg, nil, user)
	if err != nil {
		return nil, err
	}

	// 合并模板变量
	params := map[string]string{"User": user}
	for k, v := range srcCfg.DefaultVars {
		params[k] = v
	}
	for k, v := range vars {
		params[k] = v
	}

	// 拉取公钥
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Defaults.Timeout)*time.Second)
	defer cancel()

	entries, err := f.Fetch(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no keys found for user %s", user)
	}

	// 添加到 authorized_keys
	entry := entries[0] // 取第一个公钥
	added, err := s.manager.Add(entry, force)
	if err != nil {
		return nil, err
	}

	action := "skipped"
	if added {
		action = "added"
	}

	return &AddResult{
		Status:      "success",
		Action:      action,
		Source:      entry.Source,
		User:        entry.User,
		Fingerprint: entry.Fingerprint,
	}, nil
}
