// 本文件实现 add 子命令：解析目标与模板变量，并调用服务层导入公钥。
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
	// addUser URL 目标使用的用户名
	addUser string
	// addFormat 覆盖源的解析格式
	addFormat string
)

// addCmd 添加公钥命令
var addCmd = &cobra.Command{
	Use:   "add <TARGET>",
	Short: "添加用户的 SSH 公钥",
	Long: `添加指定用户的 SSH 公钥到 authorized_keys。

TARGET 格式:
  SOURCE_ALIAS:USERNAME
  https://example.com/keys.pub  （需配合 --user）

示例:
  iskey add gh:octocat
  iskey add work:zhangsan
  iskey add --user octocat https://example.com/keys.pub`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, user, err := parseTarget(args[0], addUser)
		if err != nil {
			return err
		}

		vars, err := parseVars(addVars)
		if err != nil {
			return err
		}

		svc := service.NewAddService(cfg, mgr)
		result, err := svc.Run(context.Background(), service.AddOptions{
			Source: source,
			User:   user,
			Vars:   vars,
			Format: addFormat,
			Force:  addForce,
			DryRun: dryRun,
		})
		if err != nil {
			return fmt.Errorf("add failed: %w", err)
		}

		printResult(result)
		return nil
	},
}

// init 注册 add 子命令的参数。
func init() {
	addCmd.Flags().StringArrayVar(&addVars, "var", nil, "模板变量，格式 Key=Value（可重复）")
	addCmd.Flags().BoolVar(&addForce, "force", false, "用本次拉取结果替换同一 source:user 的已有公钥")
	addCmd.Flags().StringVar(&addUser, "user", "", "URL 目标对应的用户名")
	addCmd.Flags().StringVar(&addFormat, "format", "", "覆盖解析格式: plaintext/github_json/auto")
}

// parseTarget 解析 TARGET 参数。
// URL 目标返回 (url, --user)，由服务层校验用户名是否提供。
func parseTarget(target, urlUser string) (string, string, error) {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target, urlUser, nil
	}

	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid TARGET format: %s (expected SOURCE:USER)", target)
	}
	return parts[0], parts[1], nil
}

// parseVars 解析模板变量，格式错误时明确报错而不是静默忽略。
func parseVars(vars []string) (map[string]string, error) {
	result := make(map[string]string, len(vars))
	for _, item := range vars {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --var %q (expected Key=Value)", item)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid --var %q: key is empty", item)
		}
		switch key {
		case "User", "Source":
			return nil, fmt.Errorf("--var %s is reserved", key)
		}
		result[key] = parts[1]
	}
	return result, nil
}
