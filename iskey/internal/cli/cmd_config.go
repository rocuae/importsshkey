package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/fetcher"
	"github.com/spf13/cobra"
)

var (
	// configOutput 输出路径
	configOutput string
	// configForce 强制覆盖
	configForce bool
)

// configCmd 配置管理命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理配置文件",
	Long:  "查看、添加、移除数据源配置。",
}

// configInitCmd 初始化配置
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "生成默认配置文件",
	Long: `生成默认配置文件到指定路径。

默认路径: ~/.config/iskey/sources.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath := configOutput
		if outputPath == "" {
			outputPath = config.DefaultConfigPath()
		}

		// 检查文件是否存在
		if _, err := os.Stat(outputPath); err == nil && !configForce {
			return fmt.Errorf("config file already exists: %s (use --force to overwrite)", outputPath)
		}

		// 生成默认配置
		defaultCfg := config.DefaultConfig()

		// 保存到文件
		if err := defaultCfg.SaveToFile(outputPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Config file created: %s\n", outputPath)
		return nil
	},
}

// configListCmd 列出配置
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有数据源",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("config not loaded")
		}

		if jsonOutput {
			result := map[string]interface{}{
				"builtin": getBuiltinSources(),
				"custom":  cfg.Sources,
			}
			printResult(result)
			return nil
		}

		// 显示内置源
		fmt.Println("Builtin sources (always available):")
		fmt.Println("  gh  - github    - https://api.github.com/users/{user}/keys")
		fmt.Println("  lp  - launchpad - https://launchpad.net/~{user}/+sshkeys")
		fmt.Println()

		// 显示用户配置的源
		if len(cfg.Sources) > 0 {
			fmt.Println("Custom sources:")
			for name, src := range cfg.Sources {
				enabled := "enabled"
				if !src.IsEnabled() {
					enabled = "disabled"
				}
				fmt.Printf("  %s (%s) - %s\n", name, src.Alias, enabled)
				if src.URL != "" {
					fmt.Printf("    URL: %s\n", src.URL)
				} else if src.URLTemplate != "" {
					fmt.Printf("    URL Template: %s\n", src.URLTemplate)
				}
				fmt.Printf("    Format: %s\n", src.Format)
			}
		} else {
			fmt.Println("Custom sources: none")
			fmt.Println()
			fmt.Println("Add a custom source:")
			fmt.Println("  iskey config set <alias> <url>")
		}

		return nil
	},
}

// getBuiltinSources 获取内置源配置
func getBuiltinSources() map[string]config.SourceConfig {
	enabled := true
	return map[string]config.SourceConfig{
		"github": {
			Alias:       "gh",
			URLTemplate: "https://api.github.com/users/{{ .User }}/keys",
			Format:      "github_json",
			Enabled:     &enabled,
		},
		"launchpad": {
			Alias:       "lp",
			URLTemplate: "https://launchpad.net/~{{ .User }}/+sshkeys",
			Format:      "plaintext",
			Enabled:     &enabled,
		},
	}
}

// configSetCmd 添加配置
var configSetCmd = &cobra.Command{
	Use:   "set <alias> <url>",
	Short: "快速添加数据源",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		url := args[1]

		// 检查是否为内置源
		if fetcher.IsBuiltinSource(alias) {
			return fmt.Errorf("cannot override builtin source: %s", alias)
		}

		// 自动补充模板变量：无模板变量的 URL 自动追加 /keys/{{ .User }}
		if !containsTemplateVar(url) {
			url = strings.TrimRight(url, "/")
			url += "/keys/{{ .User }}"
		}

		// 创建源配置
		src := config.SourceConfig{
			Alias:       alias,
			URLTemplate: url,
			Format:      "plaintext",
		}

		// 加载现有配置
		configPath := expandPath(configFile)
		loader := config.NewLoader(configPath)
		loadedCfg, err := loader.LoadOrDefault()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// 检查是否已存在
		if loadedCfg.HasSource(alias) {
			return fmt.Errorf("source %q already exists (use 'iskey config unset %s' to remove first)", alias, alias)
		}

		// 添加源
		loadedCfg.AddSource(alias, src)

		// 保存配置
		if err := loadedCfg.SaveToFile(configPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		// 显示添加成功信息
		fmt.Printf("Source %q added to %s\n", alias, configPath)
		fmt.Println()
		fmt.Printf("  Alias: %s\n", alias)
		if src.URL != "" {
			fmt.Printf("  URL: %s\n", src.URL)
		} else {
			fmt.Printf("  URL Template: %s\n", src.URLTemplate)
		}
		fmt.Printf("  Format: %s\n", src.Format)
		return nil
	},
}

// configUnsetCmd 移除配置
var configUnsetCmd = &cobra.Command{
	Use:   "unset <alias>",
	Short: "移除数据源",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		// 检查是否为内置源
		if fetcher.IsBuiltinSource(alias) {
			return fmt.Errorf("cannot remove builtin source: %s", alias)
		}

		// 加载配置
		configPath := expandPath(configFile)
		loader := config.NewLoader(configPath)
		loadedCfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// 移除源
		if !loadedCfg.RemoveSource(alias) {
			return fmt.Errorf("source %q not found", alias)
		}

		// 保存配置
		if err := loadedCfg.SaveToFile(configPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("Source %q removed from %s\n", alias, configPath)
		return nil
	},
}

// configVerifyCmd 校验配置
var configVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "校验配置文件语法和 URL 可达性",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := expandPath(configFile)
		loader := config.NewLoader(configPath)

		if !loader.ConfigExists() {
			fmt.Fprintf(os.Stderr, "Config file not found: %s\n", configPath)
			return fmt.Errorf("config file not found")
		}

		loadedCfg, err := loader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config validation failed: %v\n", err)
			return err
		}

		fmt.Println("Config file is valid.")
		fmt.Println()

		// 显示内置源
		fmt.Println("Builtin sources:")
		fmt.Println("  gh (github) - builtin")
		fmt.Println("  lp (launchpad) - builtin")

		// 显示自定义源
		if len(loadedCfg.Sources) > 0 {
			fmt.Println()
			fmt.Println("Custom sources:")
			for name, src := range loadedCfg.Sources {
				enabled := "enabled"
				if !src.IsEnabled() {
					enabled = "disabled"
				}
				fmt.Printf("  %s (%s) - %s\n", name, src.Alias, enabled)
			}
		}

		// 显示凭证
		if len(loadedCfg.Credentials) > 0 {
			fmt.Println()
			fmt.Println("Credentials:")
			for name, cred := range loadedCfg.Credentials {
				fmt.Printf("  %s - %s\n", name, cred.Type)
			}
		}

		return nil
	},
}

func init() {
	// init 命令参数
	configInitCmd.Flags().StringVarP(&configOutput, "output", "o", "", "输出路径（默认 ~/.config/iskey/sources.yaml）")
	configInitCmd.Flags().BoolVarP(&configForce, "force", "f", false, "强制覆盖已存在的文件")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
	configCmd.AddCommand(configVerifyCmd)
}

// containsTemplateVar 检查字符串是否包含模板变量
// 参数：
//   - s: 字符串
//
// 返回：
//   - bool: 是否包含 {{ .Var }} 格式的模板变量
func containsTemplateVar(s string) bool {
	for i := 0; i < len(s)-4; i++ {
		if s[i] == '{' && s[i+1] == '{' && s[i+2] == ' ' && s[i+3] == '.' {
			return true
		}
	}
	return false
}
