import { Hono } from 'hono';
import { Env } from '../types/env';
import { KVService } from '../services/kv.service';

const page = new Hono<{ Bindings: Env }>();

// GET / - 主页面（使用静态资产）
page.get('/', async (c) => {
  // 使用 ASSETS 绑定获取静态 HTML 文件
  // ASSETS 绑定只关心路径部分，主机名会被忽略
  const url = new URL('/index.html', c.req.url);
  return c.env.ASSETS.fetch(new Request(url, c.req.raw));
});

// GET /stats - 统计数据（公开）
page.get('/stats', async (c) => {
  const kv = new KVService(c.env);
  const users = await kv.listUsers();

  // 获取最近活动（取最后5个用户）
  const recentUsers = users.slice(-5).reverse();
  const recentActivity = [];

  for (const username of recentUsers) {
    const { metadata } = await kv.getPublicKeyWithMetadata(username);
    if (metadata) {
      recentActivity.push({
        user: username,
        action: 'update',
        time: metadata.updated_at,
        source: metadata.source || 'unknown',
      });
    }
  }

  return c.json({
    total_users: users.length,
    version: c.env.API_VERSION || 'v1',
    recent_activity: recentActivity,
  });
});



export default page;
