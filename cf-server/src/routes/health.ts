import { Hono } from 'hono';
import { Env } from '../types/env';

const health = new Hono<{ Bindings: Env }>();

health.get('/', async (c) => {
  return c.json({
    status: 'ok',
    service: 'iskey-server',
    version: c.env.API_VERSION || 'v1',
    timestamp: new Date().toISOString(),
  });
});

export default health;
