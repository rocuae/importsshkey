# iskey（Go CLI）

SSH 公钥聚合管理工具，`ssh-import-id` 的 Go 重构版。从 GitHub、Launchpad 或任意自定义 HTTP 端点拉取公钥，去重后写入 `authorized_keys`，并可通过注释标记精确列出、删除由它管理的条目。

## 特性

- 内置 `gh` / `github`、`lp` / `launchpad` 数据源，开箱即用
- 自定义数据源：URL 模板、`plaintext` / `github_json` / `auto` 响应格式
- **一个用户的多把公钥全部导入**，按 SHA256 指纹去重
- 支持直接对 `http(s)://` 端点执行 `add`，配合 `--user` 指定用户名
- 只管理自己的条目，保留 `authorized_keys` 中其他内容
- 临时文件 + rename 原子写入、`.bak` 备份、文件锁，防止并发损坏
- 文本 / JSON 两种输出，`--dry-run` 真正不落盘，`--quiet` 抑制非 JSON 文本
- 可作为 `iskey-server`（Go）或 `cf-server`（Cloudflare Workers）的客户端

## 构建

要求 Go 1.25.6+。

```bash
# 在仓库根目录
make build-cli        # 输出 bin/iskey

# 或直接构建
cd iskey
go build -o ../bin/iskey ./cmd/iskey
```

注入版本号：

```bash
go build -ldflags "-X github.com/rocuae/importsshkey/internal/cli.Version=v1.0.0" \
  -o ../bin/iskey ./cmd/iskey
```

## 快速开始

```bash
# 添加 GitHub 用户公钥
iskey add gh:octocat

# 查看已管理的公钥
iskey list --show-fingerprint

# 删除
iskey remove gh:octocat
```

首次运行会检查 `~/.ssh` 目录（0700）和 `authorized_keys`（0600）权限，不符合时在 stderr 输出警告。

## 命令参考

### 全局参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config <path>` | `~/.config/iskey/sources.yaml` | 配置文件路径 |
| `-o, --output <path>` | `~/.ssh/authorized_keys` | 目标文件路径 |
| `--dry-run` | `false` | 只拉取并计算变更，不写入文件 |
| `-q, --quiet` | `false` | 只输出 JSON（配合 `--json`）；不输出权限警告与文本结果 |
| `--json` | `false` | 以 JSON 输出结果 |

路径中的 `~` 会自动展开。

### `iskey add <SOURCE>:<USER>`

从指定数据源拉取该用户的**全部**公钥并写入目标文件。

```bash
iskey add gh:octocat
iskey add lp:ubuntu
iskey add work:zhangsan --var Team=infra
iskey add --user alice https://example.com/keys.pub
iskey add gh:octocat --force
```

| 参数 | 说明 |
|------|------|
| `--var Key=Value` | 覆盖 URL 模板变量，可重复 |
| `--force` | 用本次结果替换同一来源/用户的旧公钥 |
| `--user <name>` | URL 目标对应的用户名 |
| `--format <fmt>` | 覆盖解析格式：`plaintext` / `github_json` / `auto` |

`TARGET` 可以是 `SOURCE:USER`，也可以是完整 `http(s)://` URL（此时必须提供 `--user`）。URL 目标会写入 `source=url` 的托管条目。

示例输出：

```
Status:   success
Action:   added
Source:   github
User:     octocat
Imported: 2
Skipped:  0
Keys:
  SHA256:abc123...  added    2
  SHA256:def456...  added    4
```

`--json` 输出 `AddResult`（`Status/Action/Source/User/Imported/Skipped/Keys/Error`），可直接被脚本解析。

### `iskey remove [TARGET]`

```bash
iskey remove gh:octocat                 # 按源 + 用户删除
iskey remove --fingerprint SHA256:abc123...
iskey remove --all-from-source github # 删除某源下全部用户
```

| 参数 | 说明 |
|------|------|
| `--fingerprint <SHA256:...>` | 按指纹删除 |
| `--all-from-source <source>` | 删除指定源的全部条目 |

`--fingerprint`、`--all-from-source` 与 `TARGET` 三选一，提供多个或都未提供时报错。`--dry-run` 同样适用于删除。

### `iskey list`

```bash
iskey list
iskey list --source github
iskey list --show-fingerprint
iskey list --json
```

| 参数 | 说明 |
|------|------|
| `--source <source>` | 只看指定来源 |
| `--show-fingerprint` | 显示完整 SHA256 指纹 |

无条目时输出 `No keys managed by iskey.`。

### `iskey config`

```bash
iskey config init                  # 生成默认配置
iskey config init -o ./sources.yaml -f
iskey config list                  # 内置源 + 自定义源
iskey config verify                # 校验配置语法
iskey config set work 'https://keys.example.com/keys/{{ .User }}'
iskey config unset work
```

| 子命令 | 说明 |
|--------|------|
| `init [-o path] [-f]` | 生成配置模板；已存在时需 `-f` 覆盖 |
| `list` | 列出内置源与自定义源；`--json` 输出 `{builtin, custom}` |
| `set <alias> <url>` | 新增源；URL 不含模板变量时自动追加 `/keys/{{ .User }}` |
| `unset <alias>` | 删除自定义源 |
| `verify [--network]` | 校验语法与被引用凭据；`--network` 额外探测静态 URL |

内置源 `gh`、`lp` 不允许 `set` / `unset`。

### `iskey version`

```bash
iskey version   # iskey dev（未注入版本号时）
```

## 配置文件

默认路径 `~/.config/iskey/sources.yaml`，不存在时使用内置默认值（仅 gh/lp）。完整模板见 [sources.yaml.example](sources.yaml.example)。

```yaml
version: v1

defaults:
  output: "~/.ssh/authorized_keys"  # 写入目标
  timeout: 10                       # HTTP 超时（秒）
  max_retries: 3                    # 拉取重试次数
  insecure_skip_verify: false       # 跳过 TLS 证书校验

credentials:
  my-token:
    type: bearer                    # bearer / basic / none
    value_from_env: MY_TOKEN_ENV    # 从环境变量读取，禁止硬编码

sources:
  my-server:
    alias: work
    url: "https://keys.example.com/keys/{{ .User }}"
    format: plaintext               # plaintext / github_json
    auth_ref: my-token
    default_vars:
      Team: infra
```

### defaults

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `output` | `~/.ssh/authorized_keys` | 目标文件 |
| `timeout` | `10` | 单次请求超时（秒） |
| `max_retries` | `3` | 失败重试次数 |
| `insecure_skip_verify` | `false` | 跳过 TLS 校验（不建议开启） |

### credentials

| 字段 | 说明 |
|------|------|
| `type` | `bearer`、`basic` 或 `none` |
| `value_from_env` | bearer token 所在环境变量 |
| `username_from_env` | basic 用户名所在环境变量 |
| `password_from_env` | basic 密码所在环境变量 |

凭据只能从环境变量读取。只有被某个源通过 `auth_ref` 实际引用的凭据才会检查环境变量；无关凭据缺失不会阻止命令执行。

### sources

| 字段 | 必填 | 说明 |
|------|------|------|
| `alias` | 否 | 命令行使用的短别名，默认等于源名称 |
| `url` | 是 | 请求地址，支持 `{{ .User }}` 等模板变量 |
| `format` | 否 | `plaintext`（默认）、`github_json` 或 `auto` |
| `auth_ref` | 否 | 引用 `credentials` 中的名称 |
| `default_vars` | 否 | 模板默认变量，可被 `--var` 覆盖 |

环境变量引用支持 `$VAR`、`${VAR}`、`${VAR:-default}` 三种写法。

### 响应格式

- `plaintext`：每行一个公钥，忽略空行和 `#` 开头的行
- `github_json`：GitHub `/users/{user}/keys` 的 JSON 数组，读取每项的 `key` 字段
- `auto`：按响应首个非空白字符判断：`[` 视为 `github_json`，否则 `plaintext`

内置源：

| 别名 | 地址 |
|------|------|
| `gh` / `github` | `https://api.github.com/users/{user}/keys` |
| `lp` / `launchpad` | `https://launchpad.net/~{user}/+sshkeys` |

## authorized_keys 管理

iskey 管理的每个条目占两行：

```
# iskey:<source>:<user>:<fingerprint>
ssh-ed25519 AAAA... user@example.com
```

- `list` / `remove` 只解析带 `# iskey:` 标记的条目，其他行原样保留
- 同一用户的多把公钥会全部写入；跨来源/用户按指纹去重
- `--force` 会用本次拉取结果替换同一 `source:user` 的旧条目
- 写入采用文件锁 + 临时文件 + `rename` 原子替换，并生成 `authorized_keys.bak`
- 文件权限设为 `0600`；`~/.ssh` 不存在时自动创建为 `0700`

## 对接 iskey-server

在配置中把服务端加为普通数据源即可：

```yaml
sources:
  keys:
    alias: keys
    url: "https://keys.example.com/keys/{{ .User }}"
    format: plaintext
```

```bash
iskey add keys:alice
```

Cloudflare Workers 版（`cf-server`）接口相同，把 URL 换成 `<worker>.workers.dev/keys/{{ .User }}` 即可。

## 开发

```bash
make test        # Go CLI + Go Server 单元测试
make test-e2e    # 端到端测试（需先构建 server）
make lint        # golangci-lint（iskey + go-server）
```

## 已知行为

- `--dry-run` 会正常拉取并计算变更，但不会写入文件
- `--force` 按 `source:user` 全量替换该组公钥；未出现在本次响应中的旧公钥会被移除
- `--var` 的值会直接参与 URL 模板替换，包含特殊字符时建议在配置中使用 `urlquery`/`pathEscape`
- `config verify --network` 只探测不含模板变量的静态 URL，不会校验凭据有效性
- 托管条目会统一移动到文件末尾，非托管行原样保留

## License

MIT