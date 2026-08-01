import { Context, Next } from 'hono';
import { Env } from '../types/env';

/**
 * 管理员鉴权中间件
 * 校验请求头中的 Bearer token 是否与环境变量 ADMIN_TOKEN 匹配
 * 用于保护写操作（PUT / DELETE）
 */
export async function adminAuth(c: Context<{ Bindings: Env }>, next: Next) {
  const adminToken = c.env.ADMIN_TOKEN;

  // 未配置 ADMIN_TOKEN 时，禁止所有写操作
  if (!adminToken) {
    return c.json({ error: 'Write operations disabled: ADMIN_TOKEN not configured' }, 403);
  }

  const authHeader = c.req.header('Authorization');

  if (!authHeader) {
    return c.json({ error: 'Missing Authorization header' }, 401);
  }

  // 解析 Bearer token 格式
  const parts = authHeader.split(' ');
  if (parts.length !== 2 || parts[0] !== 'Bearer') {
    return c.json({ error: 'Invalid Authorization format. Use: Bearer <token>' }, 401);
  }

  // 恒定时间比较，防止时序攻击
  const token = parts[1];
  const isValid = await verifyToken(token, adminToken);
  if (!isValid) {
    return c.json({ error: 'Invalid token' }, 401);
  }

  await next();
}

/**
 * 恒定时间字符串比较
 * 通过逐字节异或，确保比较时间不因内容差异而变化，防止时序侧信道攻击
 */
async function verifyToken(provided: string, expected: string): Promise<boolean> {
  const encoder = new TextEncoder();
  const a = encoder.encode(provided);
  const b = encoder.encode(expected);

  if (a.length !== b.length) return false;

  let result = 0;
  for (let i = 0; i < a.length; i++) {
    result |= a[i] ^ b[i];
  }
  return result === 0;
}
