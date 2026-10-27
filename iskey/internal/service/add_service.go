// 本文件实现 add 服务：解析源、拉取全部公钥并批量写入 authorized_keys。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/domain"
	"github.com/rocuae/importsshkey/internal/fetcher"
	"github.com/rocuae/importsshkey/internal/manager"
)

// AddOptions 添加命令的输入参数
type AddOptions struct {
	// Source 源别名、源名称或完整的 http(s) URL
	Source string
	// User 目标用户名
	User string
	// Vars 命令行模板变量，优先级高于 default_vars
	Vars map[string]string
	// Format 覆盖源的解析格式（仅 URL 或自定义源使用）
	Format string
	// Force 用本次拉取结果替换同一 source:user 下的已有公钥
	Force bool
	// DryRun 只计算不写文件
	DryRun bool
}

// KeyResult 单把公钥的处理结果
type KeyResult struct {
	// Fingerprint 公钥的 SHA256 指纹
	Fingerprint string `json:"Fingerprint"`
	// Action added/replaced/skipped，dry-run 时为 would-* 前缀
	Action string `json:"Action"`
	// Line 公钥在最终文件中的 1-based 行号
	Line int `json:"Line"`
}

// AddResult 添加命令的结果
type AddResult struct {
	// Status success 或 not_found
	Status string `json:"Status"`
	// Action 整体动作：added/replaced/dry-run/skipped
	Action string `json:"Action"`
	// Source 解析后的规范源名
	Source string `json:"Source"`
	// User 目标用户名
	User string `json:"User"`
	// Imported 实际新增或替换的公钥数
	Imported int `json:"Imported"`
	// Skipped 因已存在或批内重复而跳过的数量
	Skipped int `json:"Skipped"`
	// Keys 每把公钥的处理明细
	Keys []KeyResult `json:"Keys"`
	// Error 保留字段，当前成功路径为 nil
	Error *string `json:"Error"`
}

// AddService 添加公钥服务
type AddService struct {
	// cfg 全局配置（超时、凭据、默认变量）
	cfg *config.Config
	// manager authorized_keys 文件管理器
	manager *manager.Manager
}

// NewAddService 创建添加服务
func NewAddService(cfg *config.Config, mgr *manager.Manager) *AddService {
	return &AddService{cfg: cfg, manager: mgr}
}

// Run 拉取并添加一个用户的所有公钥
func (s *AddService) Run(ctx context.Context, opts AddOptions) (*AddResult, error) {
	if err := domain.ValidateUser(opts.User); err != nil {
		return nil, err
	}

	srcCfg, sourceName, err := s.resolveSource(opts)
	if err != nil {
		return nil, err
	}

	format := srcCfg.Format
	if opts.Format != "" {
		format = opts.Format
	}
	if format == "" {
		format = "plaintext"
	}
	switch format {
	case "plaintext", "github_json", "auto":
	default:
		return nil, fmt.Errorf("invalid format %q (must be plaintext, github_json, or auto)", format)
	}
	srcCfg.Format = format

	creds, err := s.cfg.ResolveCredential(srcCfg.AuthRef)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(s.cfg.Defaults.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := make(map[string]string, len(srcCfg.DefaultVars)+len(opts.Vars)+2)
	for key, value := range srcCfg.DefaultVars {
		params[key] = value
	}
	for key, value := range opts.Vars {
		params[key] = value
	}
	params["User"] = opts.User
	params["Source"] = sourceName

	f, err := fetcher.Factory(srcCfg, sourceName, creds, opts.User, opts.Vars, fetcher.Options{
		Timeout:            timeout,
		InsecureSkipVerify: s.cfg.Defaults.InsecureSkipVerify,
		MaxRetries:         s.cfg.Defaults.MaxRetries,
	})
	if err != nil {
		return nil, err
	}

	entries, err := f.Fetch(requestCtx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch keys for %s:%s: %w", sourceName, opts.User, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no valid public keys found for %s:%s", sourceName, opts.User)
	}

	outcomes, err := s.manager.AddMany(entries, opts.Force, opts.DryRun)
	if err != nil {
		return nil, err
	}

	result := &AddResult{
		Status: "success",
		Action: "added",
		Source: sourceName,
		User:   opts.User,
		Keys:   make([]KeyResult, 0, len(outcomes)),
	}
	if opts.DryRun {
		result.Action = "dry-run"
	}
	replaced := false
	for _, outcome := range outcomes {
		result.Keys = append(result.Keys, KeyResult{
			Fingerprint: outcome.Entry.Fingerprint,
			Action:      outcome.Action,
			Line:        outcome.Line,
		})
		switch outcome.Action {
		case "skipped", "would-skip":
			result.Skipped++
		default:
			result.Imported++
		}
		if outcome.Action == "replaced" || outcome.Action == "would-replace" {
			replaced = true
		}
	}
	if result.Imported == 0 {
		result.Action = "skipped"
	} else if replaced && !opts.DryRun {
		result.Action = "replaced"
	}

	return result, nil
}

// resolveSource 解析 URL 目标或已配置的源。
func (s *AddService) resolveSource(opts AddOptions) (*config.SourceConfig, string, error) {
	if strings.HasPrefix(opts.Source, "http://") || strings.HasPrefix(opts.Source, "https://") {
		if opts.User == "" {
			return nil, "", fmt.Errorf("URL target requires --user")
		}
		format := opts.Format
		if format == "" {
			format = "auto"
		}
		return &config.SourceConfig{
			Alias:  "url",
			URL:    opts.Source,
			Format: format,
		}, "url", nil
	}

	name, src, err := fetcher.ResolveAlias(s.cfg, opts.Source)
	if err != nil {
		return nil, "", err
	}
	return src, name, nil
}
