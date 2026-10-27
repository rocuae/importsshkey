// iskey-server 可执行程序入口。
package main

import "github.com/rocuae/importsshkey/go-server/cmd"

// main 将控制权交给 Cobra 根命令，错误由 cmd.Execute 统一处理。
func main() {
	cmd.Execute()
}
