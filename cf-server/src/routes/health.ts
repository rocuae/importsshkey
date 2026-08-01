import { Hono } from 'hono';
import { Env } from '../types/env';

const health = new Hono<{ Bindings: Env }>();

/** GET /health — 健康检查端点，返回服务状态和版本信息 */
health.get('/', async (c) => {
  return c.json({
    status: 'ok',
    service: 'iskey-server',
    version: c.env.API_VERSION || 'v1',
    timestamp: new Date().toISOString(),
  });
});

export default health;
