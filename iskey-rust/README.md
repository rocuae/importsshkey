# iskey（Rust CLI）

Go 版 `iskey` 的 Rust 重写，目标是与 Go 版 **drop-in 兼容**：命令、参数、配置文件、输出格式一致，可以直接替换二进制使用。用于从 GitHub、Launchpad 或自定义 HTTP 端点拉取 SSH 公钥，去重后写入 `authorized_keys`。

## 特性

- 命令 / flag / 配置 / 文本输出与 Go 版对齐
- 内置 `gh` / `github`、`lp` / `launchpad` 数据源
- 自定义数据源：URL 模板、`plaintext` / `github_json` / `auto`
- 一个用户的多把公钥全部导入，按 SHA256 指纹去重
- 支持直接对 `http(s)://` 端点执行 `add`（配合 `--user`）
- Bearer 与 Basic 认证，凭据仅从环境变量读取，且只解析被引用的凭据
- `--force` 替换同一来源/用户的旧公钥
- 文件锁 + 原子写入 + `.bak` 备份，只管理带 iskey 标记的条目
- `--dry-run` 只计算变更、不写入
- JSON 输出字段与 Go 版对齐

## 构建

要求 Rust 1.85+（`edition = "2024"`）。

```bash
# 仓库根目录：构建并复制为 bin/iskey-rust
make build-cli-rust

# 或直接构建（产物 target/release/iskey）
cd iskey-rust
cargo build --release

# 安装到 ~/.cargo/bin/iskey
cargo install --path .
```

## 快速开始

```bash
iskey add gh:octocat
iskey list --show-fingerprint
iskey remove gh:octocat
```

默认配置文件 `~/.config/iskey/sources.yaml`，默认写入 `~/.ssh/authorized_keys`。

## 命令参考

### 全局参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-c, --config <path>` | `~/.config/iskey/sources.yaml` | 配置文件 |
| `-o, --output <path>` | `~/.ssh/authorized_keys` | 目标文件 |
| `--dry-run` | `false` | 只计算变更，不写入 |
| `-q, --quiet` | `false` | 只输出 JSON（配合 `--json`）；不输出权限警告与文本结果 |
| `--json` | `false` | JSON 输出 |

### `add`

```bash
iskey add gh:octocat
iskey add work:zhangsan --var Team=infra
iskey add --user alice https://example.com/keys.pub
iskey add gh:octocat --force
```

- `TARGET` 可为 `SOURCE:USER` 或完整 `http(s)://` URL（URL 需 `--user`）
- `--var Key=Value` 可重复，覆盖模板变量
- `--force` 用本次结果替换同一 `source:user` 的旧公钥
- `--dry-run` 仍会发起请求，但不修改文件
- 响应中的多把公钥会全部导入

### `remove`

```bash
iskey remove gh:octocat
iskey remove --fingerprint SHA256:abc123...
iskey remove --all-from-source github
```

`--fingerprint`、`--all-from-source` 与 `TARGET` 三选一；`--all-from-source` 支持别名（如 `gh`），内部会先解析为真实源名。`--dry-run` 同样适用于删除。

### `list`

```bash
iskey list
iskey list --source github
iskey list --show-fingerprint
iskey list --json
```

### `config`

```bash
iskey config init
iskey config init -o ./sources.yaml -f
iskey config list
iskey config verify
iskey config set work 'https://keys.example.com/keys/{{ .User }}'
iskey config unset work
```

- `init`：生成配置模板，已存在时需 `-f`
- `list`：内置源 + 自定义源；`--json` 输出 `{builtin, custom}`
- `set`：URL 无模板变量时自动追加 `/keys/{{ .User }}`；不允许覆盖内置源
- `unset`：删除自定义源，不允许删除内置源
- `verify`：校验配置语法与被引用凭据，不检查 URL 可达性（与 Go 版一致，Go 版可选 `--network`）

### `version`

```bash
iskey version   # iskey 0.1.0
```

## 配置文件

与 Go 版共用同一份 YAML：

```yaml
version: v1

defaults:
  output: "~/.ssh/authorized_keys"
  timeout: 10
  max_retries: 3
  insecure_skip_verify: false

credentials:
  my-token:
    type: bearer
    value_from_env: MY_TOKEN_ENV

  my-basic:
    type: basic
    username_from_env: BASIC_USER
    password_from_env: BASIC_PASS

sources:
  my-server:
    alias: work
    url: "https://keys.example.com/keys/{{ .User }}"
    format: plaintext
    auth_ref: my-token
    default_vars:
      Team: infra
```

| 字段 | 说明 |
|------|------|
| `defaults.output` | 目标文件，默认 `~/.ssh/authorized_keys` |
| `defaults.timeout` | 超时秒数，默认 10 |
| `defaults.max_retries` | 重试次数，默认 3 |
| `defaults.insecure_skip_verify` | 跳过 TLS 校验，默认 false |
| `credentials.*.type` | `bearer` / `basic` / `none` |
| `sources.*.alias` | 命令行别名，默认等于源名称 |
| `sources.*.url` | 请求地址，支持 `{{ .User }}` 与 `default_vars` |
| `sources.*.format` | `plaintext`（默认）/ `github_json` |
| `sources.*.auth_ref` | 引用凭据名称 |

环境变量支持 `$VAR`、`${VAR}`、`${VAR:-default}`。内置源无需配置：

| 别名 | 地址 |
|------|------|
| `gh` / `github` | `https://api.github.com/users/{user}/keys` |
| `lp` / `launchpad` | `https://launchpad.net/~{user}/+sshkeys` |

## authorized_keys 格式

```
# iskey:<source>:<user>:<fingerprint>
ssh-ed25519 AAAA... user@example.com
```

- 非 iskey 管理的行原样保留
- 写入使用临时文件 + rename，生成 `.bak` 备份，权限 `0600`
- 目录权限建议 `0700`，文件 `0600`

## 对接服务端

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

Go Server 与 Cloudflare Workers 版接口互通，替换 URL 即可。

## 开发

```bash
cargo test                          # 单元 + 集成测试（tests/cli_integration.rs）
cargo fmt --check
cargo clippy --all-targets -- -D warnings
```

或使用根目录 Makefile：

```bash
make test-rust
make lint-rust
```

## 与 Go 版的差异

- 两者均支持多公钥、URL target、bearer/basic 凭据与真正的 `--dry-run`
- Go 版 `config verify --network` 会探测静态 URL；Rust 版只做语法与凭据校验
- 文本输出列宽由各自实现对齐，JSON 字段保持一致
- 其余命令、参数、配置与输出语义对齐

## License

MIT