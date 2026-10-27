.PHONY: build build-cli build-server test test-e2e lint clean build-cli-rust test-rust lint-rust cf-test cf-lint

# 构建所有
build: build-cli build-server

# 构建 CLI
build-cli:
	cd iskey && go build -o ../bin/iskey cmd/iskey/main.go

# 构建 Go Server
build-server:
	cd go-server && go build -o ../bin/iskey-server .

# 测试
test:
	cd iskey && go test ./...
	cd go-server && go test ./...

# 端到端测试
test-e2e: build-server
	./test-e2e.sh

# 代码检查（固定 golangci-lint v2，避免本地版本差异）
GOLANGCI_LINT_VERSION ?= v2.15.0

lint:
	cd iskey && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...
	cd go-server && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

# 清理
clean:
	rm -rf bin/
	rm -f test.db iskey-server.log go-server/iskey-server.log
	rm -rf go-server/*.db data/

# 安装依赖
deps:
	cd iskey && go mod tidy
	cd go-server && go mod tidy

# 交叉编译 CLI（Linux amd64）
build-cli-linux:
	cd iskey && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/iskey-linux-amd64 cmd/iskey/main.go

# 交叉编译 CLI（macOS arm64）
build-cli-darwin:
	cd iskey && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ../bin/iskey-darwin-arm64 cmd/iskey/main.go

# 交叉编译 CLI（Windows amd64）
build-cli-windows:
	cd iskey && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../bin/iskey-windows-amd64.exe cmd/iskey/main.go

# 交叉编译 Go Server（Linux amd64）
build-server-linux:
	cd go-server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../bin/iskey-server-linux-amd64 .

# 所有平台
build-all: build-cli-linux build-cli-darwin build-cli-windows build-server-linux

# CF Server 依赖安装
cf-deps:
	cd cf-server && npm install

# CF Server 类型检查
cf-lint:
	cd cf-server && npx tsc --noEmit

# CF Server 单元测试
cf-test:
	cd cf-server && npm test -- --run

# CF Server 本地开发
cf-dev:
	cd cf-server && npx wrangler dev

# CF Server 部署
cf-deploy:
	cd cf-server && npx wrangler deploy

# Rust CLI 构建（输出 bin/iskey-rust，便于与 Go 版共存）
build-cli-rust:
	cd iskey-rust && cargo build --release
	cp iskey-rust/target/release/iskey bin/iskey-rust

# Rust CLI 测试
test-rust:
	cd iskey-rust && cargo test

# Rust CLI 检查（fmt + clippy）
lint-rust:
	cd iskey-rust && cargo fmt --check && cargo clippy --all-targets -- -D warnings
