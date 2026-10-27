/** 本文件覆盖 envelope 读写、旧格式兼容、分页与统计。 */
import { describe, expect, it } from 'vitest';
import { KVService } from './kv.service';
import { Env } from '../types/env';

const KEY1 =
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINyfrYcpPetnMLL81j6pxRPIZZDRAUOHgLrutYpvRHFI one@example';
const KEY2 =
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIcomaB3MmmwPc8bXyTjkoiIM0ijekLG1iQEfrAsFX/j two@example';

class MockKV {
  store = new Map<string, string>();

  async get(key: string): Promise<string | null> {
    return this.store.get(key) ?? null;
  }

  async put(key: string, value: string): Promise<void> {
    this.store.set(key, value);
  }

  async delete(key: string): Promise<void> {
    this.store.delete(key);
  }

  async list(options: { prefix?: string; cursor?: string } = {}) {
    const prefix = options.prefix ?? '';
    const names = Array.from(this.store.keys())
      .filter((name) => name.startsWith(prefix))
      .sort();
    const start = options.cursor ? Number.parseInt(options.cursor, 10) : 0;
    const page = names.slice(start, start + 2);
    const next = start + 2;
    return {
      keys: page.map((name) => ({ name })),
      list_complete: next >= names.length,
      cursor: String(next),
    };
  }
}

function service(): { kv: KVService; store: MockKV } {
  const store = new MockKV();
  const env = { KV: store as unknown as KVNamespace } as Env;
  return { kv: new KVService(env), store };
}

describe('KVService', () => {
  it('replaces and reads multiple keys', async () => {
    const { kv } = service();
    await kv.replaceKeys('alice', [
      { publicKey: KEY1, source: 'github' },
      { publicKey: KEY2, source: 'work' },
    ]);

    const keys = await kv.getKeys('alice');
    expect(keys).toHaveLength(2);
    expect(keys?.[0].source).toBe('github');
    expect(keys?.[0].fingerprint).toMatch(/^SHA256:/);
  });

  it('appends without duplicating fingerprints and preserves created_at', async () => {
    const { kv } = service();
    const [first] = await kv.replaceKeys('alice', [{ publicKey: KEY1, source: 'github' }]);
    const { added } = await kv.addKeys('alice', [
      { publicKey: KEY1, source: 'updated' },
      { publicKey: KEY2, source: 'laptop' },
    ]);

    expect(added).toBe(1);
    const keys = await kv.getKeys('alice');
    expect(keys).toHaveLength(2);
    const same = keys?.find((key) => key.fingerprint === first.fingerprint);
    expect(same?.source).toBe('updated');
    expect(same?.created_at).toBe(first.created_at);
  });

  it('reads legacy raw-key format', async () => {
    const { kv, store } = service();
    store.store.set('user:legacy', KEY1);
    store.store.set('user:legacy:metadata', JSON.stringify({ updated_at: '2024-01-01T00:00:00.000Z', source: 'old' }));

    const keys = await kv.getKeys('legacy');
    expect(keys).toHaveLength(1);
    expect(keys?.[0].source).toBe('old');
    expect(keys?.[0].public_key).toBe(KEY1);
  });

  it('paginates users across KV cursor pages and ignores metadata keys', async () => {
    const { kv } = service();
    for (const name of ['alice', 'bob', 'carol']) {
      await kv.replaceKeys(name, [{ publicKey: KEY1 }]);
    }

    const first = await kv.listUsersPage('', 1, 2);
    expect(first.total).toBe(3);
    expect(first.users).toEqual(['alice', 'bob']);

    const second = await kv.listUsersPage('', 2, 2);
    expect(second.users).toEqual(['carol']);

    const filtered = await kv.listUsersPage('bo', 1, 10);
    expect(filtered.users).toEqual(['bob']);
  });

  it('deletes a single fingerprint or the whole user', async () => {
    const { kv } = service();
    const keys = await kv.replaceKeys('alice', [{ publicKey: KEY1 }, { publicKey: KEY2 }]);

    expect(await kv.deleteKeys('alice', keys[0].fingerprint)).toBe(1);
    expect(await kv.getKeys('alice')).toHaveLength(1);
    expect(await kv.deleteKeys('alice')).toBe(1);
    expect(await kv.getKeys('alice')).toBeNull();
  });

  it('reports stats without usernames', async () => {
    const { kv } = service();
    await kv.replaceKeys('alice', [{ publicKey: KEY1 }, { publicKey: KEY2 }]);
    await kv.replaceKeys('bob', [{ publicKey: KEY1 }]);

    const stats = await kv.stats();
    expect(stats.totalUsers).toBe(2);
    expect(stats.totalKeys).toBe(3);
    expect(stats.recent[0].action).toBe('replace');
    expect(JSON.stringify(stats)).not.toContain('alice');
  });
});
