import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { Env } from './types/env';
import keysRoutes from './routes/keys';
import healthRoutes from './routes/health';

const app = new Hono<{ Bindings: Env }>();

app.use('/*', cors({
  origin: '*',
  allowMethods: ['GET', 'PUT', 'DELETE', 'OPTIONS'],
  allowHeaders: ['Authorization', 'Content-Type'],
}));

app.route('/health', healthRoutes);
app.route('/keys', keysRoutes);

app.notFound((c) => {
  return c.json({ error: 'Not Found' }, 404);
});

app.onError((err, c) => {
  console.error('Unhandled error:', err);
  return c.json({ error: 'Internal Server Error' }, 500);
});

export default app;
