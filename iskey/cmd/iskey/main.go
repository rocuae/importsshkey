// main 程序入口，负责初始化 CLI 并执行
package main

import (
	"fmt"
	"os"

	"github.com/rocuae/importsshkey/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
