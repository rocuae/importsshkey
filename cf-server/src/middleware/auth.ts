/** 写接口鉴权中间件：Bearer Token + 按 IP 的失败限流。 */

import { Context, Next } from 'hono';
import { Env } from '../types/env';

/** 单窗口内允许的最大失败次数 */
const MAX_FAILURES = 10;
/** 失败计数窗口：1 分钟 */
const WINDOW_MS = 60_000;
/** 本地开发/未配置限流绑定时使用的兜底计数（仅单 isolate 内有效） */
const failures = new Map<string, { count: number; reset: number }>();

/**
 * 管理员鉴权中间件
 * 校验请求头中的 Bearer token 是否与环境变量 ADMIN_TOKEN 匹配
 * 用于保护写操作（POST / PUT / DELETE）
 */
export async function adminAuth(c: Context<{ Bindings: Env }>, next: Next) {
  const adminToken = c.env.ADMIN_TOKEN;

  // 未配置 ADMIN_TOKEN 时，禁止所有写操作
  if (!adminToken) {
    return c.json({ error: 'Write operations disabled: ADMIN_TOKEN not configured' }, 403);
  }

  const clientKey = c.req.header('CF-Connecting-IP') || 'unknown';
  if (isBlocked(clientKey)) {
    return c.json({ error: 'Too many failed authentication attempts' }, 429);
  }

  const authHeader = c.req.header('Authorization');
  if (!authHeader) {
    return rejectUnauthorized(c, clientKey, 'Missing Authorization header');
  }

  const parts = authHeader.split(' ');
  if (parts.length !== 2 || parts[0] !== 'Bearer') {
    return rejectUnauthorized(c, clientKey, 'Invalid Authorization format. Use: Bearer <token>');
  }

  const isValid = await verifyToken(parts[1], adminToken);
  if (!isValid) {
    return rejectUnauthorized(c, clientKey, 'Invalid token');
  }

  clearFailures(clientKey);
  await next();
}

/** 返回 401；若该 IP 已超出失败窗口则返回 429。
 * 优先使用 Workers 原生 Rate Limiting 绑定（跨 isolate 生效），
 * 未配置绑定时退回单 isolate 内存计数（仅用于本地开发）。
 */
async function rejectUnauthorized(
  c: Context<{ Bindings: Env }>,
  key: string,
  message: string,
) {
  if (c.env.AUTH_RATE_LIMITER) {
    const { success } = await c.env.AUTH_RATE_LIMITER.limit({ key });
    if (!success) {
      return c.json({ error: 'Too many failed authentication attempts' }, 429);
    }
  } else {
    recordFailure(key);
    if (isBlocked(key)) {
      return c.json({ error: 'Too many failed authentication attempts' }, 429);
    }
  }
  return c.json({ error: message }, 401);
}

/** 判断 IP 当前是否已被限流（仅内存兜底路径）。 */
function isBlocked(key: string): boolean {
  const entry = failures.get(key);
  if (!entry) return false;
  if (Date.now() > entry.reset) {
    failures.delete(key);
    return false;
  }
  return entry.count >= MAX_FAILURES;
}

/** 记录一次鉴权失败；窗口过期则重新计数。 */
function recordFailure(key: string): void {
  const now = Date.now();
  const entry = failures.get(key);
  if (!entry || now > entry.reset) {
    failures.set(key, { count: 1, reset: now + WINDOW_MS });
    return;
  }
  entry.count += 1;
}

/** 鉴权成功后清除该 IP 的失败计数。 */
function clearFailures(key: string): void {
  failures.delete(key);
}

/**
 * 恒定时间比较：先哈希成等长摘要，再逐字节异或。
 * 哈希同时消除了长度差异带来的信息泄露。
 */
async function verifyToken(provided: string, expected: string): Promise<boolean> {
  const [a, b] = await Promise.all([digest(provided), digest(expected)]);
  let result = 0;
  for (let i = 0; i < a.length; i += 1) {
    result |= a[i] ^ b[i];
  }
  return result === 0;
}

/** 计算 SHA-256 摘要，用于等长且恒时的 Token 比较。 */
async function digest(value: string): Promise<Uint8Array> {
  const bytes = new TextEncoder().encode(value);
  return new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
}
