/** 页面与公开统计路由。 */

import { Hono } from 'hono';
import { Env } from '../types/env';
import { KVService } from '../services/kv.service';

const page = new Hono<{ Bindings: Env }>();

// GET / - 主页面（使用静态资产）
page.get('/', async (c) => {
  const url = new URL('/index.html', c.req.url);
  return c.env.ASSETS.fetch(
    new Request(url.toString(), {
      method: 'GET',
      headers: c.req.raw.headers,
    })
  );
});

// GET /stats - 统计数据（公开，不暴露用户名）
page.get('/stats', async (c) => {
  const kv = new KVService(c.env);
  const { totalUsers, totalKeys, recent } = await kv.stats();

  return c.json({
    total_users: totalUsers,
    total_keys: totalKeys,
    version: c.env.API_VERSION || 'v1.1',
    recent_activity: recent,
  });
});

export default page;
