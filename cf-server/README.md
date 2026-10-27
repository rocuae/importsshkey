# cf-server（Cloudflare Workers）

基于 [Hono](https://hono.dev/) + Cloudflare Workers + KV 的 iskey 服务端，提供 SSH 公钥的读写 API 和 Web 管理页面。接口与 [go-server](../go-server/README.md) 保持一致，可作为轻量、免运维的托管方案。

## 特性

- 无服务器部署，公钥存储在 Cloudflare KV
- **一个用户多把公钥**：POST 追加、PUT 全量替换、按指纹删除
- 与 iskey CLI 兼容：`GET /keys/:username` 返回全部公钥（每行一把）
- Bearer Token 保护写操作：SHA-256 摘要 + 恒定时间比较 + Workers 原生失败限流
- 用户名与 SSH 公钥完整校验（类型白名单 + blob 解析 + 指纹）
- 用户列表分页/搜索；`/stats` 仅返回聚合数据，不暴露用户名
- 内置 Web 管理页面（动态内容全部使用 textContent，避免 XSS）
- 静态资源由 Workers Assets 提供

## 前置条件

- Node.js 20+
- Cloudflare 账号
- Wrangler 4（`npm install` 已包含）

## 部署

```bash
cd cf-server

# 1. 安装依赖
npm install

# 2. 登录 Cloudflare
npx wrangler login

# 3. 创建 KV 命名空间
npx wrangler kv namespace create "iskey"

# 4. 把返回的 id 填入 wrangler.jsonc 的 kv_namespaces[].id

# 5. 设置管理员令牌（写操作必需）
npx wrangler secret put ADMIN_TOKEN

# 6. 本地开发 / 部署
npm run dev
npm run deploy
```

部署到自己的 Cloudflare 账号时，请把 `wrangler.jsonc` 中的 KV id 替换为自己的命名空间 ID。

也可以在仓库根目录使用 Makefile：

```bash
make cf-deps     # npm install
make cf-dev      # wrangler dev
make cf-deploy   # wrangler deploy
make cf-lint     # tsc --noEmit
```

## 配置

### `wrangler.jsonc`

| 配置 | 说明 |
|------|------|
| `name` | Worker 名称，决定 `<name>.workers.dev` |
| `main` | 入口 `src/index.ts` |
| `compatibility_date` | `2025-02-04` |
| `compatibility_flags` | `nodejs_compat` |
| `kv_namespaces[].binding` | 代码中的 `env.KV` |
| `kv_namespaces[].id` | 生产 KV 命名空间 ID |
| `ratelimits` | 写接口失败限流：`AUTH_RATE_LIMITER`，10 次/60 秒/IP |
| `vars.API_VERSION` | 健康检查与统计返回的版本号 |
| `vars.ALLOWED_ORIGINS` | CORS 白名单，`*` 或逗号分隔来源 |
| `assets.directory` | 静态资源目录 `./static` |
| `assets.binding` | 代码中的 `env.ASSETS` |

### 敏感配置

`ADMIN_TOKEN` 不写入 `wrangler.jsonc`，通过 Secret 注入：

```bash
npx wrangler secret put ADMIN_TOKEN        # 生产
npx wrangler secret put ADMIN_TOKEN --env dev
```

未设置时所有写操作返回 `403`。

## API

完整参数、响应示例与错误码见 [API.md](API.md)。

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/` | 无 | Web 管理页面 |
| GET | `/stats` | 无 | 用户总数与最近活动 |
| GET | `/health` | 无 | 健康检查 |
| GET | `/keys/:username` | 无 | 获取公钥（纯文本） |
| GET | `/keys/:username/metadata` | 无 | 获取元数据 |
| POST | `/keys/:username` | Bearer | 追加公钥 |
| PUT | `/keys/:username` | Bearer | 全量替换公钥 |
| DELETE | `/keys/:username` | Bearer | 删除全部或指定指纹 |
| GET | `/keys` | Bearer | 分页列出用户名 |

示例：

```bash
export BASE=https://iskey.<account>.workers.dev
export ADMIN_TOKEN=your-token

curl "$BASE/health"
curl "$BASE/keys/alice"

curl -X POST "$BASE/keys/alice" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"keys":[{"public_key":"ssh-ed25519 AAAA... alice@example.com","source":"manual"}]}'

curl -X PUT "$BASE/keys/alice" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"keys":[{"public_key":"ssh-ed25519 AAAA... alice@new","source":"manual"}]}'

curl -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE/keys?page=1&page_size=20"
```

## 存储模型

每个用户一个 envelope 键：

| 键 | 值 |
|----|----|
| `user:{username}` | `{"version":1,"keys":[{"public_key","fingerprint","source","created_at","updated_at"}]}` |
| `user:{username}:metadata` | 兼容旧版的摘要 |
| `stats:recent` | 最近 5 条活动（不含用户名） |

- 同指纹重复追加只更新 `source` / `updated_at`，保留 `created_at`
- 删除指定指纹时只移除该公钥，空集合删除整个键
- `GET /keys` 使用 `KV.list` 的 cursor 全量扫描后排序、分页
- 兼容旧版裸公钥值，读取时自动包装为单公钥 envelope
- KV 为最终一致存储，跨区域读可能短暂读到旧值；同一用户并发写入为最后写入者胜

## 与 iskey CLI 集成

把 Worker 地址配置为自定义源：

```yaml
sources:
  keys:
    alias: keys
    url: "https://iskey.example.workers.dev/keys/{{ .User }}"
    format: plaintext
```

```bash
iskey add keys:alice
iskey list
```

## 项目结构

```
cf-server/
├── src/
│   ├── index.ts            # Hono 应用、CORS、路由注册
│   ├── middleware/auth.ts  # Bearer Token 鉴权
│   ├── routes/
│   │   ├── page.ts         # GET / 与 /stats
│   │   ├── keys.ts         # /keys CRUD
│   │   └── health.ts       # GET /health
│   ├── services/
│   │   └── kv.service.ts   # KV 读写封装
│   └── types/env.d.ts      # Env / Metadata 类型
├── static/index.html       # Web 管理页面
├── wrangler.jsonc          # Worker 与 KV 配置
├── package.json
└── tsconfig.json
```

## 开发

```bash
npm run dev        # 本地启动，KV 数据保存在 .wrangler/
npm run deploy     # 部署到 Cloudflare
npm run set        # 等价于 wrangler secret put ADMIN_TOKEN
npm run test       # vitest（utils + KV service 单元测试）
npx tsc --noEmit   # 类型检查
```

本地开发时 `ADMIN_TOKEN` 可放在 `cf-server/.dev.vars`（不要提交）：

```
ADMIN_TOKEN=local-token
```

## 与 go-server 的差异

| 项目 | cf-server | go-server |
|------|-----------|-----------|
| 存储 | Cloudflare KV | SQLite / PostgreSQL |
| 部署 | Workers | 自托管二进制 / Docker |
| 公钥校验 | 类型白名单 + blob 解析 + SHA256 指纹 | `ssh.ParseAuthorizedKey` |
| 用户名校验 | `^[A-Za-z0-9][A-Za-z0-9_.@+-]{0,63}$` | 仅字母数字 `_` `-` |
| 审计日志 | 仅最近 5 条活动（无用户名） | `audit_logs` 表 |
| CORS | `ALLOWED_ORIGINS` | 由 `allowed_origins` 配置 |
| 运维 | 无服务器 | 需要数据库与进程管理 |

## 已知限制

- KV 为最终一致存储，跨区域读取可能短暂返回旧值
- 无完整审计查询与密钥轮换机制
- 限流依赖 Workers 原生 Rate Limiting 绑定；本地未配置绑定时退回单 isolate 内存计数
- `GET /keys` 需要扫描全部 `user:` 键，用户量很大时延迟上升

## License

MIT