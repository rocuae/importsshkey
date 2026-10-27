/** 公钥 CRUD 路由：GET 公开读取，写操作需管理员鉴权。 */

import { Context, Hono } from 'hono';
import { Env } from '../types/env';
import { KeyInput, KVService } from '../services/kv.service';
import { adminAuth } from '../middleware/auth';
import { validateSshKey, validateUsername } from '../utils/ssh';

const keys = new Hono<{ Bindings: Env }>();

/** 请求体上限：1 MiB，防止一次性写入过大公钥集合 */
const MAX_BODY_BYTES = 1024 * 1024;

/** GET /keys — 分页列出用户（需要 Bearer token 鉴权） */
keys.get('/', adminAuth, async (c) => {
  const page = parsePositiveInt(c.req.query('page'), 1);
  const rawPageSize = parsePositiveInt(c.req.query('page_size'), 50);
  const pageSize = Math.min(rawPageSize, 200);
  const query = (c.req.query('q') ?? '').trim();

  const kv = new KVService(c.env);
  const { users, total } = await kv.listUsersPage(query, page, pageSize);

  return c.json({
    total,
    page,
    page_size: pageSize,
    users,
  });
});

/** GET /keys/:username — 获取指定用户的全部公钥（纯文本，每行一把） */
keys.get('/:username', async (c) => {
  const username = c.req.param('username');
  const usernameError = validateUsername(username);
  if (usernameError) {
    return c.json({ error: usernameError }, 400);
  }

  const kv = new KVService(c.env);
  const stored = await kv.getKeys(username);
  if (!stored || stored.length === 0) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.text(stored.map((key) => key.public_key).join('\n') + '\n');
});

/** GET /keys/:username/metadata — 获取全部公钥的元数据 */
keys.get('/:username/metadata', async (c) => {
  const username = c.req.param('username');
  const usernameError = validateUsername(username);
  if (usernameError) {
    return c.json({ error: usernameError }, 400);
  }

  const kv = new KVService(c.env);
  const stored = await kv.getKeys(username);
  if (!stored || stored.length === 0) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.json({
    username,
    public_key_exists: true,
    count: stored.length,
    keys: stored.map((key) => ({
      fingerprint: key.fingerprint,
      source: key.source,
      created_at: key.created_at,
      updated_at: key.updated_at,
    })),
  });
});

/** PUT /keys/:username — 全量替换公钥（需要 Bearer token 鉴权） */
keys.put('/:username', adminAuth, async (c) => {
  return writeKeys(c, true);
});

/** POST /keys/:username — 追加公钥（需要 Bearer token 鉴权） */
keys.post('/:username', adminAuth, async (c) => {
  return writeKeys(c, false);
});

/**
 * PUT/POST 共用的写入逻辑。
 * replace=true 时全量替换，否则按指纹追加/更新。
 */
async function writeKeys(c: Context<{ Bindings: Env }>, replace: boolean) {
  const username = c.req.param('username');
  const usernameError = validateUsername(username);
  if (usernameError) {
    return c.json({ error: usernameError }, 400);
  }

  const parsed = await readJsonBody(c);
  if ('error' in parsed) {
    return c.json({ error: parsed.error }, parsed.status);
  }

  const inputs = collectInputs(parsed.body);
  if (inputs.length === 0) {
    return c.json({ error: 'public_key or keys is required' }, 400);
  }
  for (const input of inputs) {
    const keyError = validateSshKey(input.publicKey);
    if (keyError) {
      return c.json({ error: keyError }, 400);
    }
    input.publicKey = input.publicKey.trim();
  }

  const kv = new KVService(c.env);
  if (replace) {
    const stored = await kv.replaceKeys(username, inputs);
    return c.json({
      success: true,
      username,
      action: 'replaced',
      count: stored.length,
      updated_at: new Date().toISOString(),
    });
  }

  const { keys: stored, added } = await kv.addKeys(username, inputs);
  return c.json({
    success: true,
    username,
    action: 'added',
    count: stored.length,
    added,
    updated_at: new Date().toISOString(),
  });
}

/** DELETE /keys/:username — 删除全部或指定指纹的公钥（需要 Bearer token 鉴权） */
keys.delete('/:username', adminAuth, async (c) => {
  const username = c.req.param('username');
  const usernameError = validateUsername(username);
  if (usernameError) {
    return c.json({ error: usernameError }, 400);
  }

  const fingerprint = (c.req.query('fingerprint') ?? '').trim();
  const kv = new KVService(c.env);
  const removed = await kv.deleteKeys(username, fingerprint || undefined);
  if (removed === 0) {
    return c.json({ error: 'User not found' }, 404);
  }

  return c.json({
    success: true,
    username,
    action: 'deleted',
    removed,
  });
});

/** 解析正整数查询参数，非法或缺失时使用回退值。 */
function parsePositiveInt(raw: string | undefined, fallback: number): number {
  if (!raw) return fallback;
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

type WriteBody = {
  /** 多公钥输入，优先于 public_key */
  keys?: Array<{ public_key?: string; source?: string }>;
  /** 旧版单公钥字段，仅为兼容保留 */
  public_key?: string;
  /** 旧版单公钥来源字段 */
  source?: string;
};

/** 兼容 keys 数组与旧版 public_key 两种请求体。 */
function collectInputs(body: WriteBody): KeyInput[] {
  if (Array.isArray(body.keys)) {
    return body.keys
      .filter((key) => typeof key?.public_key === 'string')
      .map((key) => ({ publicKey: key.public_key as string, source: key.source }));
  }
  if (typeof body.public_key === 'string') {
    return [{ publicKey: body.public_key, source: body.source }];
  }
  return [];
}

/** 读取并解析 JSON 请求体，限制 1 MiB 并返回明确的状态码。 */
async function readJsonBody(
  c: Context<{ Bindings: Env }>
): Promise<{ body: WriteBody } | { error: string; status: 400 | 413 }> {
  const contentLength = Number.parseInt(c.req.header('content-length') ?? '0', 10);
  if (Number.isFinite(contentLength) && contentLength > MAX_BODY_BYTES) {
    return { error: 'Request body too large', status: 413 };
  }

  let raw: string;
  try {
    raw = await c.req.text();
  } catch {
    return { error: 'Failed to read request body', status: 400 };
  }
  if (raw.length > MAX_BODY_BYTES) {
    return { error: 'Request body too large', status: 413 };
  }
  try {
    return { body: JSON.parse(raw) as WriteBody };
  } catch {
    return { error: 'Invalid JSON body', status: 400 };
  }
}

export default keys;
