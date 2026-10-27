// 本文件实现 remove 子命令：支持按 TARGET、指纹或整源删除，并透传 dry-run。
package cli

import (
	"fmt"
	"strings"

	"github.com/rocuae/importsshkey/internal/service"
	"github.com/spf13/cobra"
)

var (
	// removeFingerprint 按指纹删除
	removeFingerprint string
	// removeAllFromSource 删除指定源下所有用户
	removeAllFromSource string
)

// removeCmd 移除公钥命令
var removeCmd = &cobra.Command{
	Use:   "remove [TARGET]",
	Short: "移除用户的 SSH 公钥",
	Long: `移除指定用户的 SSH 公钥。

TARGET 格式: SOURCE_ALIAS:USERNAME
或使用 --fingerprint 按指纹删除，或使用 --all-from-source 删除整个源。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if removeFingerprint != "" && removeAllFromSource != "" {
			return fmt.Errorf("--fingerprint and --all-from-source cannot be used together")
		}
		if (removeFingerprint != "" || removeAllFromSource != "") && len(args) > 0 {
			return fmt.Errorf("TARGET cannot be combined with --fingerprint or --all-from-source")
		}

		opts := service.RemoveOptions{
			Fingerprint:   removeFingerprint,
			AllFromSource: removeAllFromSource != "",
			Source:        removeAllFromSource,
			DryRun:        dryRun,
		}

		switch {
		case removeAllFromSource != "":
			// 源名由服务层解析别名
		case removeFingerprint != "":
			// 仅按指纹删除
		case len(args) == 0:
			return fmt.Errorf("TARGET argument is required (or use --fingerprint/--all-from-source)")
		default:
			if strings.HasPrefix(args[0], "http://") || strings.HasPrefix(args[0], "https://") {
				return fmt.Errorf("URL TARGET is not supported for remove; use --fingerprint")
			}
			source, user, err := parseTarget(args[0], "")
			if err != nil {
				return err
			}
			opts.Source = source
			opts.User = user
		}

		svc := service.NewRemoveService(cfg, mgr)
		result, err := svc.Run(opts)
		if err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
		printResult(result)
		return nil
	},
}

// init 注册 remove 子命令的删除方式参数。
func init() {
	removeCmd.Flags().StringVar(&removeFingerprint, "fingerprint", "", "按 SHA256 指纹删除")
	removeCmd.Flags().StringVar(&removeAllFromSource, "all-from-source", "", "删除指定源下所有用户")
}
