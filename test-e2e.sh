#!/bin/bash
# 端到端测试脚本

set -e

SERVER_URL="http://localhost:8080"
ADMIN_TOKEN="test-token-123"
TEST_USER="testuser"
TEST_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@iskey"

echo "=== iskey-server 端到端测试 ==="

# 启动服务器
export ADMIN_TOKEN="$ADMIN_TOKEN"
export DB_DSN="./test.db"
export SERVER_PORT="8080"

echo "启动服务器..."
./bin/iskey-server &
SERVER_PID=$!
sleep 2

# 测试健康检查
echo "1. 测试健康检查..."
HEALTH=$(curl -s "$SERVER_URL/health")
echo "   响应: $HEALTH"
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo "   ✓ 健康检查通过"
else
    echo "   ✗ 健康检查失败"
    exit 1
fi

# 测试获取不存在的用户
echo "2. 测试获取不存在的用户..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/keys/nonexistent")
if [ "$HTTP_CODE" = "404" ]; then
    echo "   ✓ 返回 404"
else
    echo "   ✗ 期望 404，实际 $HTTP_CODE"
    exit 1
fi

# 测试添加公钥（无认证）
echo "3. 测试添加公钥（无认证）..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$SERVER_URL/keys/$TEST_USER" \
    -H "Content-Type: application/json" \
    -d "{\"public_key\": \"$TEST_KEY\"}")
if [ "$HTTP_CODE" = "401" ]; then
    echo "   ✓ 返回 401（缺少 Authorization）"
else
    echo "   ✗ 期望 401，实际 $HTTP_CODE"
    exit 1
fi

# 测试添加公钥（带认证）
echo "4. 测试添加公钥（带认证）..."
RESPONSE=$(curl -s -X PUT "$SERVER_URL/keys/$TEST_USER" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"public_key\": \"$TEST_KEY\", \"source\": \"test\"}")
echo "   响应: $RESPONSE"
if echo "$RESPONSE" | grep -q '"success":true'; then
    echo "   ✓ 添加成功"
else
    echo "   ✗ 添加失败"
    exit 1
fi

# 测试获取公钥
echo "5. 测试获取公钥..."
RETRIEVED_KEY=$(curl -s "$SERVER_URL/keys/$TEST_USER")
if [ "$RETRIEVED_KEY" = "$TEST_KEY" ]; then
    echo "   ✓ 公钥匹配"
else
    echo "   ✗ 公钥不匹配"
    echo "   期望: $TEST_KEY"
    echo "   实际: $RETRIEVED_KEY"
    exit 1
fi

# 测试获取元数据
echo "6. 测试获取元数据..."
METADATA=$(curl -s "$SERVER_URL/keys/$TEST_USER/metadata")
echo "   响应: $METADATA"
if echo "$METADATA" | grep -q '"public_key_exists":true'; then
    echo "   ✓ 元数据获取成功"
else
    echo "   ✗ 元数据获取失败"
    exit 1
fi

# 测试列出用户
echo "7. 测试列出用户..."
LIST=$(curl -s -H "Authorization: Bearer $ADMIN_TOKEN" "$SERVER_URL/keys")
echo "   响应: $LIST"
if echo "$LIST" | grep -q "\"$TEST_USER\""; then
    echo "   ✓ 列出用户成功"
else
    echo "   ✗ 列出用户失败"
    exit 1
fi

# 测试删除公钥
echo "8. 测试删除公钥..."
DELETE_RESPONSE=$(curl -s -X DELETE "$SERVER_URL/keys/$TEST_USER" \
    -H "Authorization: Bearer $ADMIN_TOKEN")
echo "   响应: $DELETE_RESPONSE"
if echo "$DELETE_RESPONSE" | grep -q '"success":true'; then
    echo "   ✓ 删除成功"
else
    echo "   ✗ 删除失败"
    exit 1
fi

# 验证删除后获取
echo "9. 验证删除后获取..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/keys/$TEST_USER")
if [ "$HTTP_CODE" = "404" ]; then
    echo "   ✓ 返回 404"
else
    echo "   ✗ 期望 404，实际 $HTTP_CODE"
    exit 1
fi

# 清理
echo "清理..."
kill $SERVER_PID 2>/dev/null || true
rm -f test.db

echo ""
echo "=== 所有测试通过 ✓ ==="
