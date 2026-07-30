package cli

import (
	"context"
	"fmt"

	"github.com/importsshkey/importsshkey/internal/service"
	"github.com/spf13/cobra"
)

var (
	// syncSource 仅同步指定源
	syncSource []string
	// syncPrune 清理孤立条目
	syncPrune bool
)

// syncCmd 全量同步命令
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "全量同步所有数据源",
	Long: `遍历配置中所有启用的源，拉取最新公钥。
本地存在但远程已删除的将被移除。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建服务
		svc := service.NewSyncService(cfg, mgr)

		// 执行同步
		ctx := context.Background()
		result, err := svc.Run(ctx, syncSource, syncPrune)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		// 输出结果
		if dryRun {
			fmt.Printf("[dry-run] Would sync sources: %v\n", syncSource)
			return nil
		}

		printResult(result)
		return nil
	},
}

func init() {
	syncCmd.Flags().StringArrayVar(&syncSource, "source", nil, "仅同步指定源别名")
	syncCmd.Flags().BoolVar(&syncPrune, "prune", false, "清理孤立条目")
}
