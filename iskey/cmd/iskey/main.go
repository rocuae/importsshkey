// main 程序入口，负责初始化 CLI 并执行
package main

import (
	"os"

	"github.com/importsshkey/importsshkey/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
