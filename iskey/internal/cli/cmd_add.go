package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/rocuae/importsshkey/internal/service"
	"github.com/spf13/cobra"
)

var (
	// addVars 模板变量
	addVars []string
	// addForce 强制覆盖
	addForce bool
)

// addCmd 添加公钥命令
var addCmd = &cobra.Command{
	Use:   "add <TARGET>",
	Short: "添加用户的 SSH 公钥",
	Long: `添加指定用户的 SSH 公钥到 authorized_keys。

TARGET 格式: [SOURCE_ALIAS|URL][:USERNAME]
示例:
  iskey add gh:octocat
  iskey add work:zhangsan
  iskey add https://example.com/keys.pub`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 解析 TARGET 参数
		source, user, err := parseTarget(args[0])
		if err != nil {
			return err
		}

		// 解析模板变量
		vars := parseVars(addVars)

		// 创建服务
		svc := service.NewAddService(cfg, mgr)

		// 执行添加
		ctx := context.Background()
		result, err := svc.Run(ctx, source, user, vars, addForce)
		if err != nil {
			return fmt.Errorf("add failed: %w", err)
		}

		// 输出结果
		if dryRun {
			fmt.Printf("[dry-run] Would add key for %s:%s\n", source, user)
			return nil
		}

		printResult(result)
		return nil
	},
}

func init() {
	addCmd.Flags().StringArrayVar(&addVars, "var", nil, "模板变量，格式 Key=Value")
	addCmd.Flags().BoolVar(&addForce, "force", false, "强制覆盖已存在的公钥")
}

// parseTarget 解析 TARGET 参数
// 参数：
//   - target: TARGET 字符串，格式为 [SOURCE_ALIAS|URL][:USERNAME]
//
// 返回：
//   - string: 数据源别名或 URL
//   - string: 用户名
//   - error: 解析错误
func parseTarget(target string) (string, string, error) {
	// 处理 URL 格式（以 http:// 或 https:// 开头）
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		// URL 格式：从 URL 推断用户名
		// 这里需要用户提供用户名，或者从 URL 中提取
		return target, "", fmt.Errorf("URL format requires explicit username: %s", target)
	}

	// 处理 SOURCE:USER 格式
	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("invalid TARGET format: %s (expected SOURCE:USER)", target)
	}

	return parts[0], parts[1], nil
}

// parseVars 解析模板变量
// 参数：
//   - vars: 变量列表，格式为 ["Key=Value", ...]
//
// 返回：
//   - map[string]string: 变量映射
func parseVars(vars []string) map[string]string {
	result := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
