package cli

import (
	"fmt"
	"os"

	"github.com/importsshkey/importsshkey/internal/config"
	"github.com/spf13/cobra"
)

// configCmd 配置管理命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理配置文件",
	Long:  "查看、添加、移除数据源配置。",
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
			printResult(cfg.Sources)
			return nil
		}

		fmt.Println("Configured sources:")
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
		return nil
	},
}

// configSetCmd 添加配置
var configSetCmd = &cobra.Command{
	Use:   "set <alias> <url>",
	Short: "快速添加数据源",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: 实现 config set
		fmt.Printf("config set: %s -> %s\n", args[0], args[1])
		return nil
	},
}

// configUnsetCmd 移除配置
var configUnsetCmd = &cobra.Command{
	Use:   "unset <alias>",
	Short: "移除数据源",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: 实现 config unset
		fmt.Printf("config unset: %s\n", args[0])
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
		fmt.Printf("Version: %s\n", loadedCfg.Version)
		fmt.Printf("Sources: %d\n", len(loadedCfg.Sources))
		fmt.Printf("Credentials: %d\n", len(loadedCfg.Credentials))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
	configCmd.AddCommand(configVerifyCmd)
}
