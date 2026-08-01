import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { Env } from './types/env';
import keysRoutes from './routes/keys';
import healthRoutes from './routes/health';
import pageRoutes from './routes/page';

// 创建 Hono 应用实例，绑定 Cloudflare Env 类型
const app = new Hono<{ Bindings: Env }>();

// 全局 CORS 中间件，允许跨域请求
app.use('/*', cors({
  origin: '*',
  allowMethods: ['GET', 'PUT', 'DELETE', 'OPTIONS'],
  allowHeaders: ['Authorization', 'Content-Type'],
}));

// 注册路由：页面、健康检查和 SSH 密钥管理
app.route('/', pageRoutes);
app.route('/health', healthRoutes);
app.route('/keys', keysRoutes);

// 404 兜底处理
app.notFound((c) => {
  return c.json({ error: 'Not Found' }, 404);
});

// 全局错误处理
app.onError((err, c) => {
  console.error('Unhandled error:', err);
  return c.json({ error: 'Internal Server Error' }, 500);
});

export default app;
