package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rocuae/importsshkey/internal/service"
	"github.com/spf13/cobra"
)

var (
	// listSource 筛选特定源
	listSource string
	// listShowFingerprint 显示指纹
	listShowFingerprint bool
)

// listCmd 列出公钥命令
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出当前管理的公钥",
	Long:  "展示当前 authorized_keys 中由 iskey 管理的条目。",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 创建服务
		svc := service.NewListService(cfg, mgr)

		// 执行查询
		result, err := svc.Run(listSource, listShowFingerprint)
		if err != nil {
			return fmt.Errorf("list failed: %w", err)
		}

		// 输出结果
		if jsonOutput {
			printResult(result)
			return nil
		}

		if result.Total == 0 {
			fmt.Println("No keys managed by iskey.")
			return nil
		}

		// 文本格式输出
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if listShowFingerprint {
			fmt.Fprintln(w, "SOURCE\tUSER\tFINGERPRINT")
			fmt.Fprintln(w, "------\t----\t-----------")
		} else {
			fmt.Fprintln(w, "SOURCE\tUSER")
			fmt.Fprintln(w, "------\t----")
		}

		for _, entry := range result.Entries {
			if listShowFingerprint {
				fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Source, entry.User, entry.Fingerprint)
			} else {
				fmt.Fprintf(w, "%s\t%s\n", entry.Source, entry.User)
			}
		}

		w.Flush()
		fmt.Printf("\nTotal: %d\n", result.Total)
		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listSource, "source", "", "筛选特定源")
	listCmd.Flags().BoolVar(&listShowFingerprint, "show-fingerprint", false, "显示完整 SHA256 指纹")
}
