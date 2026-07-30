import { Context, Next } from 'hono';
import { Env } from '../types/env';

export async function adminAuth(c: Context<{ Bindings: Env }>, next: Next) {
  const adminToken = c.env.ADMIN_TOKEN;
  
  // 如果未配置 ADMIN_TOKEN，禁止写操作
  if (!adminToken) {
    return c.json({ error: 'Write operations disabled: ADMIN_TOKEN not configured' }, 403);
  }

  const authHeader = c.req.header('Authorization');
  
  if (!authHeader) {
    return c.json({ error: 'Missing Authorization header' }, 401);
  }

  const parts = authHeader.split(' ');
  if (parts.length !== 2 || parts[0] !== 'Bearer') {
    return c.json({ error: 'Invalid Authorization format. Use: Bearer <token>' }, 401);
  }

  const token = parts[1];
  const isValid = await verifyToken(token, adminToken);
  if (!isValid) {
    return c.json({ error: 'Invalid token' }, 401);
  }

  await next();
}

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
