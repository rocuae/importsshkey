import { Hono } from 'hono';
import { Env } from '../types/env';
import { KVService } from '../services/kv.service';
import { adminAuth } from '../middleware/auth';

const keys = new Hono<{ Bindings: Env }>();

keys.get('/:username', async (c) => {
  const username = c.req.param('username');
  
  if (!username || username.length === 0) {
    return c.json({ error: 'Username is required' }, 400);
  }

  const kv = new KVService(c.env);
  const publicKey = await kv.getPublicKey(username);

  if (!publicKey) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.text(publicKey);
});

keys.get('/:username/metadata', async (c) => {
  const username = c.req.param('username');
  const kv = new KVService(c.env);
  const { publicKey, metadata } = await kv.getPublicKeyWithMetadata(username);

  if (!publicKey) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.json({
    username,
    public_key_exists: true,
    metadata,
  });
});

keys.put('/:username', adminAuth, async (c) => {
  const username = c.req.param('username');
  const body = await c.req.json<{ public_key: string; source?: string }>();

  if (!body.public_key) {
    return c.json({ error: 'public_key is required' }, 400);
  }

  if (!body.public_key.trim().startsWith('ssh-')) {
    return c.json({ error: 'Invalid SSH public key format' }, 400);
  }

  const kv = new KVService(c.env);
  await kv.putPublicKey(username, body.public_key.trim(), body.source);

  return c.json({
    success: true,
    username,
    action: 'updated',
    updated_at: new Date().toISOString(),
  });
});

keys.delete('/:username', adminAuth, async (c) => {
  const username = c.req.param('username');
  const kv = new KVService(c.env);
  
  const existing = await kv.getPublicKey(username);
  if (!existing) {
    return c.json({ error: 'User not found' }, 404);
  }

  await kv.deletePublicKey(username);

  return c.json({
    success: true,
    username,
    action: 'deleted',
  });
});

keys.get('/', adminAuth, async (c) => {
  const kv = new KVService(c.env);
  const users = await kv.listUsers();
  
  return c.json({
    total: users.length,
    users,
  });
});

export default keys;
