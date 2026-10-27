// cli 包定义所有 CLI 子命令和全局参数
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/manager"
	"github.com/rocuae/importsshkey/internal/service"
	"github.com/spf13/cobra"
)

var (
	// configFile 配置文件路径
	configFile string
	// outputFile 目标 authorized_keys 文件路径
	outputFile string
	// dryRun 模拟运行，仅打印变更
	dryRun bool
	// quiet 静默模式
	quiet bool
	// jsonOutput JSON 格式输出
	jsonOutput bool
	// cfg 全局配置对象
	cfg *config.Config
	// mgr authorized_keys 管理器
	mgr *manager.Manager
	// Version 版本号（构建时注入）
	Version = "dev"
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "iskey",
	Short: "SSH 公钥聚合管理工具",
	Long: `iskey 是 ssh-import-id 的 Go 重构版，支持多数据源的 SSH 公钥管理。

支持的数据源：
  gh:, github:     GitHub 用户公钥
  lp:, launchpad:  Launchpad 用户公钥
  自定义 URL:       任何返回纯文本或 GitHub JSON 格式的端点

示例：
  iskey add gh:octocat
  iskey add work:zhangsan
  iskey remove gh:octocat
  iskey list --show-fingerprint`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// config 子命令自行加载配置：verify/init 需要在配置损坏时仍能运行。
		if cmd.Parent() != nil && cmd.Parent().Name() == "config" {
			return nil
		}
		if cmd.Name() == "version" {
			return nil
		}

		configPath := expandPath(configFile)
		loader := config.NewLoader(configPath)
		var err error
		cfg, err = loader.LoadOrDefault()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// --output 显式传入时优先；否则使用配置中的 defaults.output。
		outputPath := outputFile
		if !cmd.Root().PersistentFlags().Changed("output") && cfg.Defaults.Output != "" {
			outputPath = cfg.Defaults.Output
		}
		mgr = manager.NewManager(expandPath(outputPath))

		// 检查权限（非静默模式下输出警告）
		if !quiet {
			warnings, _ := mgr.CheckPermissions()
			for _, w := range warnings {
				fmt.Fprintln(os.Stderr, w)
			}
		}

		return nil
	},
}

// init 注册全局参数与所有子命令。
func init() {
	// 获取默认配置路径
	defaultConfig := config.DefaultConfigPath()

	// 获取默认 authorized_keys 路径
	home, _ := os.UserHomeDir()
	defaultOutput := filepath.Join(home, ".ssh", "authorized_keys")

	rootCmd.PersistentFlags().StringVar(&configFile, "config", defaultConfig, "配置文件路径")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", defaultOutput, "目标 authorized_keys 文件路径")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "模拟运行，仅打印变更")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "静默模式")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON 格式输出")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// versionCmd 版本命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("iskey %s\n", Version)
	},
}

// Execute 执行根命令
func Execute() error {
	return rootCmd.Execute()
}

// expandPath 展开路径中的 ~ 为用户主目录
// 参数：
//   - path: 路径字符串
//
// 返回：
//   - string: 展开后的路径
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// printResult 输出结果（支持 JSON 和文本格式）
// 参数：
//   - result: 结果对象
func printResult(result interface{}) {
	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}
	if quiet {
		return
	}
	printTextResult(result)
}

// printTextResult 输出文本格式结果
// 参数：
//   - result: 结果对象
func printTextResult(result interface{}) {
	switch r := result.(type) {
	case *service.AddResult:
		fmt.Printf("Status:   %s\n", r.Status)
		fmt.Printf("Action:   %s\n", r.Action)
		fmt.Printf("User:     %s\n", r.User)
		fmt.Printf("Source:   %s\n", r.Source)
		fmt.Printf("Imported: %d key(s)\n", r.Imported)
		fmt.Printf("Skipped:  %d key(s)\n", r.Skipped)
		printKeyResults(r.Keys)
	case *service.RemoveResult:
		fmt.Printf("Status:   %s\n", r.Status)
		fmt.Printf("Action:   %s\n", r.Action)
		if r.Source != "" {
			fmt.Printf("Source:   %s\n", r.Source)
		}
		if r.User != "" {
			fmt.Printf("User:     %s\n", r.User)
		}
		if r.Fingerprint != "" {
			fmt.Printf("Key:      %s\n", r.Fingerprint)
		}
		fmt.Printf("Removed:  %d key(s)\n", r.Removed)
		printKeyResults(r.Keys)
	default:
		fmt.Printf("%+v\n", result)
	}
}

// printKeyResults 以缩进列表输出每把公钥的变更结果与最终行号。
// printKeyResults 以缩进列表打印每把公钥的动作与行号。
func printKeyResults(keys []service.KeyResult) {
	if len(keys) == 0 {
		return
	}
	fmt.Println("Keys:")
	for _, key := range keys {
		if key.Line > 0 {
			fmt.Printf("  %s  %s  line %d\n", key.Fingerprint, key.Action, key.Line)
		} else {
			fmt.Printf("  %s  %s\n", key.Fingerprint, key.Action)
		}
	}
}
