#!/usr/bin/env bash
# 端到端测试：真实启动 iskey-server，验证多公钥 CRUD 与鉴权
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
PORT="${E2E_PORT:-18080}"
BASE_URL="http://127.0.0.1:${PORT}"
ADMIN_TOKEN="e2e-token-$(date +%s)"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

fail() {
  echo "   ✗ $*" >&2
  if [[ -f "$TMP_DIR/server.log" ]]; then
    echo "--- server.log ---" >&2
    tail -50 "$TMP_DIR/server.log" >&2
  fi
  exit 1
}

echo "=== iskey-server 端到端测试 ==="

ssh-keygen -q -t ed25519 -N '' -C 'e2e-1' -f "$TMP_DIR/key1"
ssh-keygen -q -t ed25519 -N '' -C 'e2e-2' -f "$TMP_DIR/key2"
ssh-keygen -q -t ed25519 -N '' -C 'e2e-3' -f "$TMP_DIR/key3"
KEY1="$(cat "$TMP_DIR/key1.pub")"
KEY2="$(cat "$TMP_DIR/key2.pub")"
KEY3="$(cat "$TMP_DIR/key3.pub")"

echo "启动服务器 ($BASE_URL)..."
DB_PATH="$TMP_DIR/iskey.db" \
SERVER_PORT="$PORT" \
ADMIN_TOKEN="$ADMIN_TOKEN" \
ALLOWED_ORIGINS="*" \
"$ROOT_DIR/bin/iskey-server" server -f >"$TMP_DIR/server.log" 2>&1 &
SERVER_PID=$!

READY=0
for _ in $(seq 1 50); do
  if curl -fsS "$BASE_URL/health" >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.2
done
[[ "$READY" = "1" ]] || fail "服务器未就绪"

echo "1. 健康检查"
HEALTH="$(curl -fsS "$BASE_URL/health")"
echo "$HEALTH" | grep -q '"status":"ok"' || fail "健康检查失败: $HEALTH"
echo "   ✓"

echo "2. 不存在的用户返回 404"
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/keys/nonexistent")"
[[ "$CODE" = "404" ]] || fail "期望 404，实际 $CODE"
echo "   ✓"

echo "3. 未认证写入返回 401"
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE_URL/keys/e2euser" \
  -H 'Content-Type: application/json' \
  -d "{\"public_key\":\"$KEY1\"}")"
[[ "$CODE" = "401" ]] || fail "期望 401，实际 $CODE"
echo "   ✓"

echo "4. 写入两把公钥（PUT 全量替换）"
RESP="$(curl -fsS -X PUT "$BASE_URL/keys/e2euser" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"keys\":[{\"public_key\":\"$KEY1\",\"source\":\"e2e\"},{\"public_key\":\"$KEY2\",\"source\":\"e2e\"}]}")"
echo "$RESP" | grep -q '"count":2' || fail "写入失败: $RESP"
echo "   ✓"

echo "5. GET 返回两把公钥且以换行结尾"
curl -fsS -o "$TMP_DIR/body" "$BASE_URL/keys/e2euser"
BODY="$(cat "$TMP_DIR/body")"
[[ "$(printf '%s\n' "$BODY" | grep -c '^ssh-')" = "2" ]] || fail "公钥数量不是 2"
[[ "$(tail -c 1 "$TMP_DIR/body" | od -An -t u1 | tr -d ' ')" = "10" ]] || fail "响应缺少结尾换行"
echo "   ✓"

echo "6. POST 追加第三把公钥"
RESP="$(curl -fsS -X POST "$BASE_URL/keys/e2euser" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"public_key\":\"$KEY3\",\"source\":\"e2e\"}")"
echo "$RESP" | grep -q '"count":3' || fail "追加失败: $RESP"
echo "   ✓"

echo "7. 元数据包含 3 把公钥"
META="$(curl -fsS "$BASE_URL/keys/e2euser/metadata")"
echo "$META" | grep -q '"count":3' || fail "元数据错误: $META"
FINGERPRINT="$(printf '%s' "$META" | grep -o '"fingerprint":"[^"]*"' | head -1 | cut -d'"' -f4)"
[[ -n "$FINGERPRINT" ]] || fail "无法提取指纹"
echo "   ✓ ($FINGERPRINT)"

echo "8. 按指纹删除单把公钥"
ENCODED_FP="$(python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1]))' "$FINGERPRINT")"
RESP="$(curl -fsS -X DELETE "$BASE_URL/keys/e2euser?fingerprint=$ENCODED_FP" \
  -H "Authorization: Bearer $ADMIN_TOKEN")"
echo "$RESP" | grep -q '"removed":1' || fail "按指纹删除失败: $RESP"
echo "   ✓"

echo "9. 剩余 2 把公钥"
META="$(curl -fsS "$BASE_URL/keys/e2euser/metadata")"
echo "$META" | grep -q '"count":2' || fail "删除后元数据错误: $META"
echo "   ✓"

echo "10. 分页列表包含用户"
LIST="$(curl -fsS -H "Authorization: Bearer $ADMIN_TOKEN" "$BASE_URL/keys?page=1&page_size=10&q=e2e")"
echo "$LIST" | grep -q '"e2euser"' || fail "列表缺少用户: $LIST"
echo "   ✓"

echo "11. 非法公钥返回 400"
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE_URL/keys/baduser" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"public_key":"ssh-garbage"}')"
[[ "$CODE" = "400" ]] || fail "期望 400，实际 $CODE"
echo "   ✓"

echo "12. 统计接口不暴露用户名"
STATS="$(curl -fsS "$BASE_URL/stats")"
echo "$STATS" | grep -q '"total_keys":2' || fail "统计错误: $STATS"
if echo "$STATS" | grep -q 'e2euser'; then fail "统计泄露用户名"; fi
echo "   ✓"

echo "13. 删除用户"
RESP="$(curl -fsS -X DELETE "$BASE_URL/keys/e2euser" \
  -H "Authorization: Bearer $ADMIN_TOKEN")"
echo "$RESP" | grep -q '"removed":2' || fail "删除用户失败: $RESP"
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/keys/e2euser")"
[[ "$CODE" = "404" ]] || fail "删除后期望 404，实际 $CODE"
echo "   ✓"

echo ""
echo "=== 所有端到端测试通过 ✓ ==="
