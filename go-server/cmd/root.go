// 本文件定义 iskey-server 的 Cobra 根命令与子命令注册。
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "go-server",
	Short: "SSH 密钥管理服务器",
	Long:  `一个用于管理 SSH 密钥的 REST API 服务器，支持用户密钥的增删改查操作。`,
}

// Execute 执行根命令；失败时输出错误并以非零状态退出。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// init 注册 server 与 config 子命令。
func init() {
	// 注册子命令
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(configCmd)
}
