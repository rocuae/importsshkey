# cf-server API 文档

- **Base URL**：`https://<worker-name>.<account>.workers.dev`，本地默认 `http://localhost:8787`
- **数据格式**：除 `GET /`（HTML）与 `GET /keys/:username`（纯文本）外均为 `application/json`
- **时间格式**：ISO 8601 UTC，例如 `2026-10-27T02:30:00.000Z`
- **认证**：写接口需要 `Authorization: Bearer <ADMIN_TOKEN>`

## 接口总览

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/` | 无 | Web 管理页面 |
| GET | `/stats` | 无 | 统计与最近活动（不含用户名） |
| GET | `/health` | 无 | 健康检查 |
| GET | `/keys/:username` | 无 | 获取全部公钥（纯文本） |
| GET | `/keys/:username/metadata` | 无 | 获取全部公钥元数据 |
| POST | `/keys/:username` | Bearer | 追加公钥 |
| PUT | `/keys/:username` | Bearer | 全量替换公钥 |
| DELETE | `/keys/:username` | Bearer | 删除全部或指定指纹 |
| GET | `/keys` | Bearer | 分页列出用户名 |

一个用户可保存多把公钥，存储在单个 KV envelope 中。同一指纹重复追加会更新来源与时间。

## 认证

写接口（`POST`、`PUT`、`DELETE`、`GET /keys`）由 `adminAuth` 保护。

| 场景 | 状态码 | 响应 |
|------|--------|------|
| 未配置 `ADMIN_TOKEN` | 403 | `{"error":"Write operations disabled: ADMIN_TOKEN not configured"}` |
| 缺少 `Authorization` 头 | 401 | `{"error":"Missing Authorization header"}` |
| 不是 `Bearer <token>` | 401 | `{"error":"Invalid Authorization format. Use: Bearer <token>"}` |
| Token 不匹配 | 401 | `{"error":"Invalid token"}` |
| 同一 IP 每分钟失败 10 次 | 429 | `{"error":"Too many failed authentication attempts"}` |

Token 先做 SHA-256 摘要再恒定时间比较，不泄露长度；成功认证会清除失败计数。

## 错误响应

```json
{ "error": "错误描述" }
```

| 状态码 | 说明 |
|--------|------|
| 400 | 参数、JSON 或公钥格式不合法 |
| 401 | 未认证或 Token 错误 |
| 403 | 未配置 `ADMIN_TOKEN` |
| 404 | 用户或路由不存在 |
| 413 | 请求体超过 1 MiB |
| 429 | 认证失败次数过多 |
| 500 | 未捕获异常 |

## 参数约束

- **username**：`^[A-Za-z0-9][A-Za-z0-9_.@+-]{0,63}$`
- **public_key**：必须是合法 SSH 公钥（解析 blob 并校验内嵌类型），支持 ed25519 / RSA / ECDSA / `sk-*`
- **source**：可选字符串

---

## GET `/stats`

公开接口，不返回用户名或来源。

```json
{
  "total_users": 3,
  "total_keys": 7,
  "version": "v1",
  "recent_activity": [
    { "action": "replace", "time": "2026-10-27T02:30:00.000Z" }
  ]
}
```

`recent_activity` 保存在 KV 键 `stats:recent`，最多 5 条。

## GET `/health`

```json
{
  "status": "ok",
  "service": "iskey-server",
  "version": "v1",
  "timestamp": "2026-10-27T02:30:00.000Z"
}
```

## GET `/keys/:username`

返回全部公钥，每行一把，以换行结尾。

```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@example.com
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@laptop
```

| 状态码 | 说明 |
|--------|------|
| 400 | 用户名格式非法 |
| 404 | 用户不存在 |

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
      "created_at": "2026-10-27T02:00:00.000Z",
      "updated_at": "2026-10-27T02:30:00.000Z"
    }
  ]
}
```

## POST `/keys/:username`

追加公钥；用户不存在时创建。请求体支持 `keys` 数组或旧字段 `public_key`。

```json
{
  "keys": [
    { "public_key": "ssh-ed25519 AAAA... alice@example.com", "source": "manual" }
  ]
}
```

响应：

```json
{
  "success": true,
  "username": "alice",
  "action": "added",
  "count": 2,
  "added": 1,
  "updated_at": "2026-10-27T02:30:00.000Z"
}
```

`count` 是写入后的总数，`added` 是本次新增数量（重复指纹不计数）。

## PUT `/keys/:username`

全量替换该用户的公钥集合，未出现在请求中的旧公钥会被删除。

```json
{
  "success": true,
  "username": "alice",
  "action": "replaced",
  "count": 1,
  "updated_at": "2026-10-27T02:30:00.000Z"
}
```

## DELETE `/keys/:username`

删除全部公钥；`?fingerprint=SHA256:...` 只删除指定公钥。

```json
{
  "success": true,
  "username": "alice",
  "action": "deleted",
  "removed": 1
}
```

## GET `/keys`

分页列出用户名（KV cursor 全量扫描后排序）。

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

## CORS

由 `ALLOWED_ORIGINS` 控制：`*` 或逗号分隔白名单。支持 `GET, POST, PUT, DELETE, OPTIONS`，预检返回 `204`。

## 存储模型

| 键 | 值 |
|----|----|
| `user:{username}` | `{"version":1,"keys":[StoredKey...]}` |
| `user:{username}:metadata` | 兼容旧版本的摘要 |
| `stats:recent` | 最近 5 条活动（不含用户名） |

兼容旧版裸公钥值：读取时会自动包装为单公钥 envelope。

## 完整调用示例

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
  -d '{"keys":[{"public_key":"ssh-ed25519 AAAA... alice@new"}]}'

curl -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE/keys?page=1&page_size=20&q=ali"

curl -X DELETE "$BASE/keys/alice?fingerprint=SHA256:..." \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```
