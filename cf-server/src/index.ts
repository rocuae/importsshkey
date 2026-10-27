/** Workers 入口：注册 CORS、安全响应头、路由与统一错误处理。 */

import { Hono } from 'hono';
import { Env } from './types/env';
import keysRoutes from './routes/keys';
import healthRoutes from './routes/health';
import pageRoutes from './routes/page';

const app = new Hono<{ Bindings: Env }>();

// CORS：按 ALLOWED_ORIGINS 白名单放行；* 表示全部。
// OPTIONS 预检直接返回 204，避免进入业务处理器。
app.use('/*', async (c, next) => {
  const allowed = (c.env.ALLOWED_ORIGINS || '*')
    .split(',')
    .map((origin) => origin.trim())
    .filter(Boolean);
  const origin = c.req.header('Origin');

  if (allowed.includes('*')) {
    c.header('Access-Control-Allow-Origin', '*');
  } else if (origin && allowed.includes(origin)) {
    c.header('Access-Control-Allow-Origin', origin);
    c.header('Vary', 'Origin');
  }

  c.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  c.header('Access-Control-Allow-Headers', 'Authorization, Content-Type');
  c.header('Access-Control-Max-Age', '86400');
  c.header('X-Content-Type-Options', 'nosniff');
  c.header('Referrer-Policy', 'no-referrer');

  if (c.req.method === 'OPTIONS') {
    return c.body(null, 204);
  }

  await next();
});

app.route('/', pageRoutes);
app.route('/health', healthRoutes);
app.route('/keys', keysRoutes);

/** 未匹配路由统一返回 JSON 404。 */
app.notFound((c) => {
  return c.json({ error: 'Not Found' }, 404);
});

/** 未捕获异常统一记录并返回 500，避免泄露内部错误。 */
app.onError((err, c) => {
  console.error('Unhandled error:', err);
  return c.json({ error: 'Internal Server Error' }, 500);
});

export default app;
