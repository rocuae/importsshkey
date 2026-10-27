# iskey-server API 文档

- **Base URL**：`http://<host>:<port>`，默认 `http://localhost:8080`
- **数据格式**：除 `GET /`（HTML）和 `GET /keys/:username`（纯文本）外，均为 `application/json`
- **时间格式**：RFC 3339，例如 `2026-10-27T10:30:00+08:00`
- **认证**：写接口需要 `Authorization: Bearer <ADMIN_TOKEN>`

## 接口总览

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/` | 无 | Web 管理页面 |
| GET | `/stats` | 无 | 统计与最近活动（不含用户名） |
| GET | `/health` | 无 | 健康检查（含数据库探测） |
| GET | `/keys/:username` | 无 | 获取用户全部公钥（纯文本） |
| GET | `/keys/:username/metadata` | 无 | 获取全部公钥的元数据 |
| POST | `/keys/:username` | Bearer | 追加公钥 |
| PUT | `/keys/:username` | Bearer | 全量替换该用户的公钥 |
| DELETE | `/keys/:username` | Bearer | 删除全部或指定指纹的公钥 |
| GET | `/keys` | Bearer | 分页列出用户名 |

一个用户可保存多把公钥。公钥按 `(username, fingerprint)` 唯一，重复追加会更新来源与时间。

## 认证

写接口（`POST`、`PUT`、`DELETE`、`GET /keys`）由 `AdminAuth` 中间件保护：

```
Authorization: Bearer <ADMIN_TOKEN>
```

| 场景 | 状态码 | 响应 |
|------|--------|------|
| 未配置 `ADMIN_TOKEN` | 403 | `{"error":"Write operations disabled: ADMIN_TOKEN not configured"}` |
| 缺少 `Authorization` 头 | 401 | `{"error":"Missing Authorization header"}` |
| 认证头格式错误 | 401 | `{"error":"Invalid Authorization format.Use: Bearer <token>"}` |
| Token 不匹配 | 401 | `{"error":"Invalid token"}` |
| 同一 IP 每分钟失败 10 次 | 429 | `{"error":"Too many failed authentication attempts"}` |

Token 使用恒定时间比较；成功认证会清除该 IP 的失败计数。

## 错误响应

```json
{
  "error": "错误描述"
}
```

| 状态码 | 说明 |
|--------|------|
| 400 | 参数、用户名或公钥格式不合法 |
| 401 | 未认证或 Token 错误 |
| 403 | 未配置 `ADMIN_TOKEN` |
| 404 | 用户或公钥不存在 |
| 413 | 请求体超过 1 MiB |
| 429 | 认证失败次数过多 |
| 500 | 数据库或服务端内部错误 |

## 参数约束

- **username**：`^[A-Za-z0-9_-]+$`，不能为空
- **public_key(s)**：必须是合法 SSH 公钥（`ssh.ParseAuthorizedKey`），支持 ed25519 / RSA / ECDSA 及 `sk-*` 类型
- **source**：自由文本，可为空

---

## GET `/`

返回内置 Web 管理页面。

## GET `/stats`

公开统计接口，**不返回用户名、来源等隐私信息**。

```json
{
  "total_users": 3,
  "total_keys": 7,
  "version": "v1.1",
  "recent_activity": [
    { "action": "replace", "time": "2026-10-27T10:30:00+08:00" }
  ]
}
```

`recent_activity` 来自审计日志，按时间倒序，最多 5 条。

## GET `/health`

```json
{
  "status": "ok",
  "service": "iskey-server",
  "version": "v1.1",
  "timestamp": "2026-10-27T10:30:00+08:00"
}
```

数据库不可达时返回 `503`，`status` 为 `degraded`。

---

## GET `/keys/:username`

返回该用户全部公钥，每行一把，并以换行结尾，可直接作为 iskey 数据源。

```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@example.com
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@laptop
```

| 状态码 | 说明 |
|--------|------|
| 400 | 用户名为空或格式非法 |
| 404 | 用户不存在 |
| 500 | 数据库错误 |

## GET `/keys/:username/metadata`

```json
{
  "username": "alice",
  "public_key_exists": true,
  "count": 2,
  "keys": [
    {
      "fingerprint": "SHA256:...",
      "source": "manual",
      "created_at": "2026-10-27T10:00:00+08:00",
      "updated_at": "2026-10-27T10:30:00+08:00"
    }
  ]
}
```

## POST `/keys/:username`

向用户**追加**公钥；用户不存在时创建。请求体支持 `keys` 数组或旧字段 `public_key`。

```json
{
  "keys": [
    { "public_key": "ssh-ed25519 AAAA... alice@example.com", "source": "manual" },
    { "public_key": "ssh-ed25519 AAAA... alice@laptop", "source": "laptop" }
  ]
}
```

```bash
curl -X POST http://localhost:8080/keys/alice \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"keys":[{"public_key":"ssh-ed25519 AAAA... alice@example.com","source":"manual"}]}'
```

响应：

```json
{
  "success": true,
  "username": "alice",
  "action": "added",
  "count": 2,
  "updated_at": "2026-10-27T10:30:00+08:00"
}
```

`count` 是写入后该用户的公钥总数；同一指纹重复追加不会增加数量。

## PUT `/keys/:username`

**全量替换**该用户的公钥集合：未出现在请求中的旧公钥会被删除。请求体格式与 POST 相同。

```json
{
  "success": true,
  "username": "alice",
  "action": "replaced",
  "count": 2,
  "updated_at": "2026-10-27T10:30:00+08:00"
}
```

## DELETE `/keys/:username`

删除该用户全部公钥；使用 `?fingerprint=SHA256:...` 时只删除指定公钥。

```json
{
  "success": true,
  "username": "alice",
  "action": "deleted",
  "removed": 1
}
```

删除后无剩余公钥时返回 404。

## GET `/keys`

分页列出用户名，按用户名升序。

| 查询参数 | 默认值 | 说明 |
|----------|--------|------|
| `q` | 空 | 用户名模糊搜索 |
| `page` | `1` | 页码 |
| `page_size` | `50` | 每页数量，最大 `200` |

```json
{
  "total": 3,
  "page": 1,
  "page_size": 50,
  "users": ["alice", "bob", "carol"]
}
```

---

## CORS

- `Access-Control-Allow-Origin`：由 `security.allowed_origins` 决定，`*` 表示全部
- `Access-Control-Allow-Methods`：`GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers`：`Authorization, Content-Type`
- `OPTIONS` 预检返回 `204`

## 数据结构

### UserKey（表 `user_keys`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | integer | 自增主键 |
| `username` | string | 用户名（索引） |
| `public_key` | string | SSH 公钥 |
| `fingerprint` | string | SHA256 指纹（索引） |
| `source` | string | 来源标识 |
| `created_at` | string | 创建时间 |
| `updated_at` | string | 更新时间 |

### AuditLog（表 `audit_logs`）

记录 `add` / `replace` / `delete` 操作，含客户端 IP 与 User-Agent。用于 `/stats` 最近活动，暂无对外查询接口。

## 完整调用示例

```bash
export BASE=http://localhost:8080
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

curl "$BASE/keys/alice/metadata"
curl -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE/keys?page=1&page_size=20&q=ali"

curl -X DELETE "$BASE/keys/alice?fingerprint=SHA256:..." \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```
