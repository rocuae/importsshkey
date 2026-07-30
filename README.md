# iskey

SSH 公钥聚合管理工具，ssh-import-id 的 Go 重构版。

## 项目结构

```
importsshkey/
├── iskey/                  # CLI 项目
│   ├── cmd/iskey/          # CLI 入口
│   ├── internal/           # 内部实现
│   │   ├── cli/            # Cobra 命令
│   │   ├── config/         # 配置加载
│   │   ├── domain/         # 领域模型
│   │   ├── fetcher/        # 数据拉取
│   │   ├── manager/        # authorized_keys 管理
│   │   └── service/        # 业务逻辑
│   └── pkg/utils/          # 工具函数
├── cf-server/              # Cloudflare Workers 服务端
├── go-server/              # 自托管 Go 服务端
├── Makefile                # 构建脚本
└── .docs/                  # 设计文档（不提交）
```

## 快速开始

### CLI

```bash
# 构建
make build-cli

# 使用
./bin/iskey add gh:octocat
./bin/iskey list
./bin/iskey remove gh:octocat
```

### Go Server

```bash
# 构建
make build-server

# 配置
cp go-server/.env.example go-server/.env
# 编辑 .env 设置 ADMIN_TOKEN

# 运行
./bin/iskey-server
```

### Cloudflare Workers

```bash
# 安装依赖
make cf-deps

# 本地开发
make cf-dev

# 部署
make cf-deploy
```

## 命令速查

| 命令 | 说明 |
|------|------|
| `iskey add <SOURCE>:<USER>` | 添加公钥 |
| `iskey remove <SOURCE>:<USER>` | 移除公钥 |
| `iskey sync` | 全量同步 |
| `iskey list` | 列出管理的公钥 |
| `iskey config list` | 列出数据源 |
| `iskey config verify` | 校验配置 |

## 数据源

| 别名 | 说明 |
|------|------|
| `gh:` / `github:` | GitHub 用户公钥 |
| `lp:` / `launchpad:` | Launchpad 用户公钥 |
| 自定义 | 在 `~/.config/iskey/sources.yaml` 配置 |

## API 端点

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/keys/:username` | 无 | 获取公钥 |
| GET | `/keys/:username/metadata` | 无 | 获取元数据 |
| PUT | `/keys/:username` | Bearer Token | 添加/更新 |
| DELETE | `/keys/:username` | Bearer Token | 删除 |
| GET | `/keys` | Bearer Token | 列出所有用户 |
| GET | `/health` | 无 | 健康检查 |

## 配置文件

`~/.config/iskey/sources.yaml`:

```yaml
version: v1

defaults:
  output: "~/.ssh/authorized_keys"
  timeout: 10
  max_retries: 3

credentials:
  my-token:
    type: bearer
    value_from_env: MY_TOKEN_ENV

sources:
  github:
    alias: gh
    url_template: "https://api.github.com/users/{{ .User }}/keys"
    format: github_json
    enabled: true

  my-server:
    alias: work
    url_template: "https://keys.internal.com/keys/{{ .User }}"
    format: plaintext
    auth_ref: my-token
```

## 开发

```bash
# 构建所有
make build

# 测试
make test

# 端到端测试
make test-e2e

# 代码检查
make lint

# 交叉编译
make build-all
```

## License

MIT
