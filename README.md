# iskey

SSH 公钥聚合管理工具，ssh-import-id 的 Go 重构版。

## 项目结构

```
importsshkey/
├── iskey/                  # CLI 项目（Go 版，见 iskey/README.md）
│   ├── cmd/iskey/          # CLI 入口
│   ├── internal/           # 内部实现
│   │   ├── cli/            # Cobra 命令
│   │   ├── config/         # 配置加载
│   │   ├── domain/         # 领域模型
│   │   ├── fetcher/        # 数据拉取
│   │   ├── manager/        # authorized_keys 管理
│   │   └── service/        # 业务逻辑
│   └── pkg/utils/          # 工具函数
├── iskey-rust/             # CLI 项目（Rust 版，drop-in 兼容，见 iskey-rust/README.md）
│   ├── src/                # 模块（domain/config/fetcher/manager/service/cli）
│   └── tests/              # 集成测试
├── cf-server/              # Cloudflare Workers 服务端（见 cf-server/README.md、API.md）
├── go-server/              # 自托管 Go 服务端（见 go-server/README.md、API.md）
├── Makefile                # 构建脚本
└── .docs/                  # 设计文档（不提交）
```

## 快速开始

### CLI

```bash
# 构建
make build-cli

# 使用（一个用户的多把公钥会全部导入）
./bin/iskey add gh:octocat
./bin/iskey add --user alice https://example.com/keys.pub
./bin/iskey list --show-fingerprint
./bin/iskey remove gh:octocat
```

Rust 版（drop-in 兼容，命令/flag/配置/输出一致）：

```bash
# 构建
make build-cli-rust   # 输出 bin/iskey-rust

# 测试 / 检查
make test-rust
make lint-rust
```

### Go Server

```bash
# 构建
make build-server

# 编辑 go-server/config.yaml，设置 security.admin_token
# 或使用环境变量 ADMIN_TOKEN 覆盖
cd go-server
export ADMIN_TOKEN=your-token
../bin/iskey-server server -f
```

完整配置、Docker、systemd 与接口说明见 [go-server/README.md](go-server/README.md) 和 [go-server/API.md](go-server/API.md)。

### Cloudflare Workers

```bash
# 安装依赖
make cf-deps

# 本地开发
make cf-dev

# 部署前需先创建 KV 并设置 ADMIN_TOKEN，详见 cf-server/README.md
make cf-deploy
```

## 命令速查

| 命令 | 说明 |
|------|------|
| `iskey add <SOURCE>:<USER>` | 导入该用户的全部公钥 |
| `iskey add --user <USER> <URL>` | 从任意 HTTP 端点导入 |
| `iskey remove <SOURCE>:<USER>` | 移除某来源/用户的公钥 |
| `iskey remove --fingerprint <SHA256:...>` | 按指纹移除 |
| `iskey remove --all-from-source <SOURCE>` | 移除某来源全部条目 |
| `iskey list` | 列出管理的公钥 |
| `iskey config list` | 列出数据源 |
| `iskey config verify [--network]` | 校验配置（可选探测 URL） |
| `iskey version` | 显示版本 |

## 数据源

| 别名 | 说明 |
|------|------|
| `gh:` / `github:` | GitHub 用户公钥 |
| `lp:` / `launchpad:` | Launchpad 用户公钥 |
| 自定义 | 在 `~/.config/iskey/sources.yaml` 配置 |

## API 端点

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/keys/:username` | 无 | 获取全部公钥（纯文本，每行一把） |
| GET | `/keys/:username/metadata` | 无 | 获取全部公钥元数据 |
| POST | `/keys/:username` | Bearer Token | 追加公钥 |
| PUT | `/keys/:username` | Bearer Token | 全量替换该用户公钥 |
| DELETE | `/keys/:username` | Bearer Token | 删除全部或指定指纹 |
| GET | `/keys?page=&page_size=&q=` | Bearer Token | 分页列出用户 |
| GET | `/health` | 无 | 健康检查 |

## 配置文件

`~/.config/iskey/sources.yaml`:

```yaml
# 配置版本号，固定为 v1
version: v1

# 全局默认配置（选填，未配置的项使用内置默认值）
defaults:
  # 公钥拉取后写入的目标文件，默认 ~/.ssh/authorized_keys
  output: "~/.ssh/authorized_keys"
  # HTTP 请求超时时间，单位：秒，默认 10
  timeout: 10
  # 拉取失败时的最大重试次数，默认 3
  max_retries: 3

# 认证凭据定义，供 sources.*.auth_ref 引用；Token 只能从环境变量读取，禁止硬编码
credentials:
  # 凭据名称，被 auth_ref 引用
  my-token:
    # 认证类型：bearer / basic / none
    type: bearer
    # 存放 Token 的环境变量名（bearer 类型必填）
    value_from_env: MY_TOKEN_ENV

# 数据源定义，键名为源名称
sources:
  github:
    # 短别名，命令行可用 iskey add gh:<用户名> 引用
    alias: gh
    # 请求地址，{{ .User }} 会被替换为命令行传入的用户名
    url: "https://api.github.com/users/{{ .User }}/keys"
    # 响应解析格式：plaintext / github_json / auto
    format: github_json

  # 自定义私有数据源
  my-server:
    # 短别名，可用 iskey add work:<用户名> 引用
    alias: work
    url: "https://keys.internal.com/keys/{{ .User }}"
    format: plaintext
    # 引用 credentials 下的凭据名称
    auth_ref: my-token
```

## 开发

```bash
# 构建（Go CLI + Go Server）
make build

# 测试
make test

# 端到端测试
make test-e2e

# 代码检查（Go CLI + Go Server，固定 golangci-lint v2）
make lint

# 交叉编译
make build-all

# Rust CLI
make build-cli-rust
make test-rust
make lint-rust

# Cloudflare Workers
make cf-deps
make cf-lint
make cf-test
make cf-dev
```

## 文档

| 项目 | 文档 |
|------|------|
| Go CLI | [iskey/README.md](iskey/README.md) |
| Rust CLI | [iskey-rust/README.md](iskey-rust/README.md) |
| Go Server | [go-server/README.md](go-server/README.md) · [API.md](go-server/API.md) |
| Cloudflare Workers | [cf-server/README.md](cf-server/README.md) · [API.md](cf-server/API.md) |

## License

MIT
