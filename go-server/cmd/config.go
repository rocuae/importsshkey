package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocuae/importsshkey/go-server/internal/config"
	"github.com/spf13/cobra"
)

var configConfigFile string

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long:  `管理服务器配置，包括生成默认配置文件和查看当前配置。`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "生成默认配置文件",
	Long:  `生成默认的 config.yaml 配置文件。如果文件已存在，提示是否覆盖。`,
	Run: func(cmd *cobra.Command, args []string) {
		generateConfig(configConfigFile)
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	Long:  `显示当前加载的配置信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		showConfig(configConfigFile)
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.PersistentFlags().StringVarP(&configConfigFile, "config", "c", "", "指定配置文件路径（默认为 config.yaml）")
}

func generateConfig(configFile string) {
	if configFile == "" {
		configFile = "config.yaml"
	}

	// 检查文件是否已存在
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("配置文件 %s 已存在。\n", configFile)
		fmt.Print("是否覆盖？(y/N): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("操作已取消。")
			return
		}
	}

	// 默认配置内容（YAML 格式）
	defaultConfig := `# 服务配置
server:
  host: 0.0.0.0
  port: 8080

# 数据库配置
database:
  driver: sqlite
  # SQLite 配置
  path: ./iskey.db
  # PostgreSQL 配置（取消注释并修改）
  # driver: postgres
  # host: localhost
  # port: 5432
  # user: postgres
  # password: ""
  # dbname: iskey
  # sslmode: disable

# 安全配置
security:
  # 管理员 Token（为空时禁止写操作）
  admin_token: "your-super-secure-admin-token-here"
  allowed_origins: "*"

# 日志配置
log:
  level: info
`

	// 写入文件
	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		fmt.Printf("无法写入配置文件: %v\n", err)
		os.Exit(1)
	}

	// 获取绝对路径
	absPath, _ := filepath.Abs(configFile)
	fmt.Printf("配置文件已生成: %s\n", absPath)
	fmt.Println("请根据实际情况修改配置，特别是 security.admin_token。")
}

func showConfig(configFile string) {
	// 使用 internal/config 包加载配置
	cfg := config.Load(configFile)

	// 显示配置
	fmt.Println("当前配置:")
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("SERVER:          %s:%d\n", cfg.ServerHost, cfg.ServerPort)
	fmt.Printf("DB_DRIVER:       %s\n", cfg.DBDriver)
	if cfg.DBDriver == "sqlite" {
		fmt.Printf("DB_PATH:         %s\n", cfg.DBPath)
	} else {
		fmt.Printf("DB_HOST:         %s\n", cfg.DBHost)
		fmt.Printf("DB_PORT:         %d\n", cfg.DBPort)
		fmt.Printf("DB_USER:         %s\n", cfg.DBUser)
		fmt.Printf("DB_NAME:         %s\n", cfg.DBName)
		fmt.Printf("DB_SSLMODE:      %s\n", cfg.DBSSLMode)
	}
	fmt.Printf("ADMIN_TOKEN:     %s\n", maskToken(cfg.AdminToken))
	fmt.Printf("ALLOWED_ORIGINS: %s\n", cfg.AllowedOrigins)
	fmt.Printf("LOG_LEVEL:       %s\n", cfg.LogLevel)
	fmt.Println("────────────────────────────────────────")

	// 检查配置文件是否存在
	configPath := configFile
	if configPath == "" {
		configPath = "config.yaml"
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("\n提示: 未找到 %s 配置文件，使用环境变量或默认值。\n", configPath)
		fmt.Println("运行 'go-server config init' 生成默认配置文件。")
	}
}

func maskToken(token string) string {
	if token == "" {
		return "(未设置)"
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}