package cli

import (
	"fmt"

	"github.com/importsshkey/importsshkey/internal/service"
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
	Use:   "remove <TARGET>",
	Short: "移除用户的 SSH 公钥",
	Long: `移除指定用户的 SSH 公钥。

TARGET 格式: [SOURCE_ALIAS|URL][:USERNAME]
或使用 --fingerprint 直接按指纹删除。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建服务
		svc := service.NewRemoveService(cfg, mgr)

		// 按指纹删除
		if removeFingerprint != "" {
			result, err := svc.Run("", "", removeFingerprint)
			if err != nil {
				return fmt.Errorf("remove failed: %w", err)
			}
			printResult(result)
			return nil
		}

		// 删除指定源下所有用户
		if removeAllFromSource != "" {
			removed, err := mgr.RemoveAllFromSource(removeAllFromSource)
			if err != nil {
				return fmt.Errorf("remove failed: %w", err)
			}
			if jsonOutput {
				printResult(map[string]interface{}{
					"status":  "success",
					"removed": removed,
					"source":  removeAllFromSource,
				})
			} else {
				fmt.Printf("Removed %d keys from source %s\n", removed, removeAllFromSource)
			}
			return nil
		}

		// 按 TARGET 删除
		if len(args) == 0 {
			return fmt.Errorf("TARGET argument is required (or use --fingerprint/--all-from-source)")
		}

		source, user, err := parseTarget(args[0])
		if err != nil {
			return err
		}

		result, err := svc.Run(source, user, "")
		if err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}

		printResult(result)
		return nil
	},
}

func init() {
	removeCmd.Flags().StringVar(&removeFingerprint, "fingerprint", "", "按 SHA256 指纹删除")
	removeCmd.Flags().StringVar(&removeAllFromSource, "all-from-source", "", "删除指定源下所有用户")
}
