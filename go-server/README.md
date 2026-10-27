# iskey-server（Go 自托管服务端）

基于 Go + Gin + GORM 的 SSH 公钥托管服务，为 `iskey` CLI 提供 HTTP 数据源，也可直接作为团队内部的公钥分发中心。支持 SQLite / PostgreSQL，内置 Web 管理页面与审计日志。

## 特性

- **一个用户可保存多把公钥**：POST 追加、PUT 全量替换、按指纹删除
- 写入前使用 `golang.org/x/crypto/ssh` 严格校验格式，并按 SHA256 指纹去重
- 读接口公开，写接口使用 Bearer Token 认证（恒定时间比较 + 失败限流）
- 支持 SQLite（纯 Go 驱动，无需 CGO）与 PostgreSQL
- 启动时自动迁移表结构（`user_keys`、`audit_logs`），并迁移旧 `users` 表
- 用户列表分页/搜索，`/stats` 仅返回聚合数据，不暴露用户名
- 内置 Web 管理界面，支持多公钥查看与追加/替换
- 支持前台运行、守护进程、Docker（非 root）、systemd 部署

## 快速开始

### 构建

```bash
# 在仓库根目录执行，产物为 bin/iskey-server
make build-server
```

### 配置

```bash
# 仓库自带 go-server/config.yaml，直接编辑其中的 security.admin_token
# 如需重新生成模板（文件已存在时会询问是否覆盖）
./bin/iskey-server config init -c go-server/config.yaml

# 查看当前生效配置（Token 会脱敏）
./bin/iskey-server config show -c go-server/config.yaml
```

至少需要设置 `security.admin_token`，否则所有写操作返回 403。

### 运行

```bash
# 默认以守护进程方式启动
./bin/iskey-server server

# 前台运行（推荐用于容器与 systemd）
./bin/iskey-server server -f

# 指定配置文件
./bin/iskey-server server -f -c /etc/iskey/config.yaml
```

启动日志示例：

```
启动 iskey-server 于 0.0.0.0:8080
```

访问 `http://localhost:8080/` 即可打开 Web 管理页面。

## 配置

配置文件默认为工作目录下的 `config.yaml`，也可通过 `-c/--config` 指定。

```yaml
# 服务监听配置
server:
  # 监听地址，0.0.0.0 表示所有网卡
  host: 0.0.0.0
  # 监听端口
  port: 8080

# 数据库配置
database:
  # 数据库驱动：sqlite / postgres
  driver: sqlite
  # SQLite 数据文件路径（driver=sqlite 时生效）
  path: ./iskey.db
  # PostgreSQL 配置（driver=postgres 时生效）
  # host: localhost
  # port: 5432
  # user: postgres
  # password: ""
  # dbname: iskey
  # sslmode: disable

# 安全配置
security:
  # 管理员 Token，为空时禁止所有写操作
  admin_token: "your-super-secure-admin-token-here"
  # 允许的 CORS 来源，逗号分隔，* 表示全部
  allowed_origins: "*"

# 日志配置
log:
  # 日志级别：debug / info / warn / error
  level: info
```

### 配置优先级

内置默认值 < `config.yaml` < 环境变量。环境变量适合容器和 systemd 注入密钥，避免把 Token 写进配置文件。

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `SERVER_HOST` | 监听地址 | `0.0.0.0` |
| `SERVER_PORT` | 监听端口 | `8080` |
| `DB_DRIVER` | 数据库驱动 | `sqlite` |
| `DB_PATH` | SQLite 文件路径 | `./iskey.db` |
| `DB_HOST` | PostgreSQL 主机 | `localhost` |
| `DB_PORT` | PostgreSQL 端口 | `5432` |
| `DB_USER` | PostgreSQL 用户 | `postgres` |
| `DB_PASSWORD` | PostgreSQL 密码 | 空 |
| `DB_NAME` | PostgreSQL 数据库名 | `iskey` |
| `DB_SSLMODE` | PostgreSQL SSL 模式 | `disable` |
| `ADMIN_TOKEN` | 管理员 Token | 空（写操作禁用） |
| `ALLOWED_ORIGINS` | CORS 允许来源 | `*` |
| `LOG_LEVEL` | 日志级别 | `info` |

> 注意：服务不会自动加载 `.env` 文件，请通过 shell 导出、systemd `Environment=` 或 Docker `environment` 注入。

## API

完整接口说明、参数约束与响应示例见 [API.md](API.md)。

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/` | 无 | Web 管理页面 |
| GET | `/stats` | 无 | 用户总数与最近活动 |
| GET | `/health` | 无 | 健康检查 |
| GET | `/keys/:username` | 无 | 获取公钥（纯文本） |
| GET | `/keys/:username/metadata` | 无 | 获取公钥元数据 |
| POST | `/keys/:username` | Bearer | 追加公钥 |
| PUT | `/keys/:username` | Bearer | 全量替换该用户公钥 |
| DELETE | `/keys/:username` | Bearer | 删除全部或指定指纹 |
| GET | `/keys` | Bearer | 分页列出用户名 |

### 认证

写接口需要请求头：

```
Authorization: Bearer <ADMIN_TOKEN>
```

- 未配置 `ADMIN_TOKEN`：返回 `403 Write operations disabled`
- 缺少或格式错误：返回 `401`
- Token 不匹配：返回 `401 Invalid token`
- 同一 IP 每分钟失败 10 次：返回 `429`

### 示例

```bash
# 追加公钥（可一次传多把）
curl -X POST http://localhost:8080/keys/alice \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"keys":[{"public_key":"ssh-ed25519 AAAAC3Nza... alice@example.com","source":"manual"}]}'

# 全量替换该用户公钥
curl -X PUT http://localhost:8080/keys/alice \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"keys":[{"public_key":"ssh-ed25519 AAAAC3Nza... alice@new","source":"manual"}]}'

# 读取全部公钥（可直接作为 iskey 自定义数据源）
curl http://localhost:8080/keys/alice

# 分页列出用户
curl -H "Authorization: Bearer $ADMIN_TOKEN" "http://localhost:8080/keys?page=1&page_size=20&q=ali"
```

### 校验规则

- 用户名：仅允许字母、数字、下划线、连字符，且不能为空
- 公钥：必须能被 `ssh.ParseAuthorizedKey` 解析；单次请求体不超过 1 MiB
- 同一用户下按 SHA256 指纹唯一

## 与 iskey CLI 集成

把本服务配置为 CLI 数据源后，即可通过 `iskey add` 从服务端导入公钥：

```yaml
# ~/.config/iskey/sources.yaml
sources:
  work:
    alias: work
    url: "http://your-server:8080/keys/{{ .User }}"
    format: plaintext
```

```bash
./bin/iskey add work:alice
```

## Docker

```bash
# 构建镜像
docker build -t iskey-server .

# 运行（非 root uid 10001，需先让挂载目录可写）
mkdir -p data && sudo chown -R 10001:10001 data
docker run -d --name iskey-server \
  -p 8080:8080 \
  -e ADMIN_TOKEN=your-token \
  -v "$PWD/data:/data" \
  iskey-server
```

### docker-compose

```bash
export ADMIN_TOKEN=your-token
mkdir -p data && sudo chown -R 10001:10001 data
docker compose up -d
```

`docker-compose.yml` 会把数据库文件持久化到 `./data`，并挂载 `config.yaml`；其中 `DB_DRIVER`、`DB_PATH`、`ADMIN_TOKEN`、`ALLOWED_ORIGINS` 由环境变量注入。

## systemd 部署

```bash
# 1. 编译 Linux 二进制
make build-server-linux

# 2. 准备目录与用户
sudo useradd --system --home /opt/iskey-server --shell /usr/sbin/nologin iskey-server
sudo mkdir -p /opt/iskey-server
sudo cp bin/iskey-server-linux-amd64 /opt/iskey-server/iskey-server
sudo chown -R iskey-server:iskey-server /opt/iskey-server

# 3. 安装服务单元
sudo cp go-server/systemd/go-server.service /etc/systemd/system/iskey-server.service
sudo systemctl daemon-reload
sudo systemctl enable --now iskey-server
```

Token 不再写入 unit 文件。部署时创建 `/etc/iskey-server.env`（权限 0600）：

```bash
sudo install -m 600 /dev/null /etc/iskey-server.env
sudo tee /etc/iskey-server.env >/dev/null <<'ENV'
ADMIN_TOKEN=your-token
DB_PATH=/opt/iskey-server/data/iskey.db
ENV
```

确认 `WorkingDirectory`、`DB_PATH` 与实际目录一致。服务处理 `SIGTERM` 优雅退出，不再响应 `SIGHUP` 重载。

Docker 运行镜像以 uid/gid 10001 的非 root 用户 `iskey` 运行，`/data` 需可写。

## 项目结构

```
go-server/
├── main.go                 # 入口
├── cmd/                    # Cobra 命令
│   ├── root.go             # 根命令
│   ├── server.go           # server 子命令：守护进程/前台启动、路由注册
│   └── config.go           # config init / config show
├── internal/
│   ├── config/             # 配置加载（YAML + 环境变量）
│   ├── handler/            # HTTP 处理器（keys/health/page）
│   ├── middleware/         # 认证、CORS、日志中间件
│   ├── model/              # UserKey、AuditLog 数据模型
│   └── repository/         # GORM 仓储实现
├── pkg/validator/          # 用户名与 SSH 公钥校验
├── systemd/                # systemd 服务单元
├── config.yaml             # 配置示例
├── Dockerfile
└── docker-compose.yml
```

## 开发

```bash
# 运行测试（仓库根目录的 make test 会同时测试 CLI 与服务端）
make test

# 仅测试服务端
cd go-server && go test ./...

# 静态检查
cd go-server && go vet ./...

# 交叉编译 Linux amd64
make build-server-linux
```

## 安全建议

- `ADMIN_TOKEN` 使用高强度随机值，并通过环境变量注入，不要提交到仓库
- 生产环境将 `allowed_origins` 收窄到可信域名，避免使用 `*`
- 对外暴露时建议置于 HTTPS 反向代理之后
- 定期备份 SQLite 数据文件或 PostgreSQL 数据库