import { ActivityItem, Env, StoredKey, UserEnvelope } from '../types/env';
import { sshFingerprint } from '../utils/ssh';

export interface KeyInput {
  /** SSH 公钥原文 */
  publicKey: string;
  /** 可选来源标识 */
  source?: string;
}

/**
 * Cloudflare KV 存储服务。
 * 用户数据以单键 envelope 存储：user:{username} -> { version: 1, keys: [...] }。
 * 兼容旧格式（裸公钥字符串 + user:{username}:metadata）。
 */
export class KVService {
  /** 绑定当前 Worker 的 KV/环境变量。 */
  constructor(private env: Env) {}

  /** 读取用户全部公钥；不存在返回 null */
  async getKeys(username: string): Promise<StoredKey[] | null> {
    const raw = await this.env.KV.get(`user:${username}`);
    if (raw === null) {
      return null;
    }

    try {
      const parsed = JSON.parse(raw) as Partial<UserEnvelope>;
      if (parsed && Array.isArray(parsed.keys)) {
        return parsed.keys.filter(
          (key): key is StoredKey =>
            !!key && typeof key.public_key === 'string' && typeof key.fingerprint === 'string'
        );
      }
    } catch {
      // 旧格式：值是裸公钥字符串，继续走兼容逻辑
    }

    const fingerprint = await sshFingerprint(raw).catch(() => null);
    if (!fingerprint) {
      return [];
    }
    const metadata = await this.env.KV.get(`user:${username}:metadata`);
    let updatedAt = new Date(0).toISOString();
    let source = 'legacy';
    if (metadata) {
      try {
        const parsed = JSON.parse(metadata) as { updated_at?: string; source?: string };
        if (parsed.updated_at) updatedAt = parsed.updated_at;
        if (parsed.source) source = parsed.source;
      } catch {
        // 忽略损坏的旧元数据
      }
    }
    return [
      {
        public_key: raw,
        fingerprint,
        source,
        created_at: updatedAt,
        updated_at: updatedAt,
      },
    ];
  }

  /** 全量替换用户公钥 */
  async replaceKeys(username: string, inputs: KeyInput[]): Promise<StoredKey[]> {
    const now = new Date().toISOString();
    const keys: StoredKey[] = [];
    const seen = new Set<string>();
    for (const input of inputs) {
      const fingerprint = await sshFingerprint(input.publicKey);
      if (seen.has(fingerprint)) {
        continue;
      }
      seen.add(fingerprint);
      keys.push({
        public_key: input.publicKey.trim(),
        fingerprint,
        source: input.source ?? '',
        created_at: now,
        updated_at: now,
      });
    }
    await this.write(username, keys);
    await this.recordActivity('replace');
    return keys;
  }

  /** 追加用户公钥；同指纹更新来源并保留创建时间 */
  async addKeys(username: string, inputs: KeyInput[]): Promise<{ keys: StoredKey[]; added: number }> {
    const existing = (await this.getKeys(username)) ?? [];
    const now = new Date().toISOString();
    const byFingerprint = new Map(existing.map((key) => [key.fingerprint, key]));
    let added = 0;

    for (const input of inputs) {
      const fingerprint = await sshFingerprint(input.publicKey);
      const current = byFingerprint.get(fingerprint);
      if (current) {
        current.source = input.source ?? current.source;
        current.updated_at = now;
        continue;
      }
      byFingerprint.set(fingerprint, {
        public_key: input.publicKey.trim(),
        fingerprint,
        source: input.source ?? '',
        created_at: now,
        updated_at: now,
      });
      added += 1;
    }

    const keys = Array.from(byFingerprint.values());
    await this.write(username, keys);
    await this.recordActivity('add');
    return { keys, added };
  }

  /** 删除全部公钥，或仅删除指定指纹；返回实际删除数量 */
  async deleteKeys(username: string, fingerprint?: string): Promise<number> {
    if (!fingerprint) {
      const existing = await this.getKeys(username);
      if (!existing) {
        return 0;
      }
      await this.deleteUser(username);
      await this.recordActivity('delete');
      return existing.length;
    }

    const existing = await this.getKeys(username);
    if (!existing) {
      return 0;
    }
    const remaining = existing.filter((key) => key.fingerprint !== fingerprint);
    if (remaining.length === existing.length) {
      return 0;
    }
    await this.write(username, remaining);
    await this.recordActivity('delete');
    return existing.length - remaining.length;
  }

  /** 分页列出用户名，支持模糊搜索 */
  async listUsersPage(
    query: string,
    page: number,
    pageSize: number
  ): Promise<{ users: string[]; total: number }> {
    const all = await this.listAllUsers();
    const filtered = query ? all.filter((name) => name.includes(query)) : all;
    const offset = (page - 1) * pageSize;
    return {
      users: filtered.slice(offset, offset + pageSize),
      total: filtered.length,
    };
  }

  /** 统计信息（不含用户名） */
  async stats(): Promise<{ totalUsers: number; totalKeys: number; recent: ActivityItem[] }> {
    const users = await this.listAllUsers();
    let totalKeys = 0;
    for (const username of users) {
      const keys = await this.getKeys(username);
      totalKeys += keys?.length ?? 0;
    }

    const raw = await this.env.KV.get('stats:recent');
    let recent: ActivityItem[] = [];
    if (raw) {
      try {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) {
          recent = parsed.filter(
            (item): item is ActivityItem =>
              !!item && typeof item.action === 'string' && typeof item.time === 'string'
          );
        }
      } catch {
        // 忽略损坏的统计缓存
      }
    }
    return { totalUsers: users.length, totalKeys, recent };
  }

  /** 写入 envelope 与兼容元数据；keys 为空时删除用户键。 */
  private async write(username: string, keys: StoredKey[]): Promise<void> {
    if (keys.length === 0) {
      await this.deleteUser(username);
      return;
    }
    const envelope: UserEnvelope = { version: 1, keys };
    const updatedAt = keys.reduce(
      (latest, key) => (key.updated_at > latest ? key.updated_at : latest),
      new Date(0).toISOString()
    );
    const metadata = {
      updated_at: keys.length > 0 ? updatedAt : new Date().toISOString(),
      source: keys[0]?.source ?? '',
    };
    await Promise.all([
      this.env.KV.put(`user:${username}`, JSON.stringify(envelope)),
      this.env.KV.put(`user:${username}:metadata`, JSON.stringify(metadata)),
    ]);
  }

  /** 删除用户的主键与旧版元数据键。 */
  private async deleteUser(username: string): Promise<void> {
    await Promise.all([
      this.env.KV.delete(`user:${username}`),
      this.env.KV.delete(`user:${username}:metadata`),
    ]);
  }

  /** 使用 KV cursor 全量扫描用户名，过滤元数据键与非法键名。 */
  private async listAllUsers(): Promise<string[]> {
    const users = new Set<string>();
    let cursor: string | undefined;
    do {
      const list: KVNamespaceListResult<unknown> = await this.env.KV.list({
        prefix: 'user:',
        cursor,
      });
      for (const key of list.keys) {
        if (key.name.endsWith(':metadata')) {
          continue;
        }
        const username = key.name.slice('user:'.length);
        if (!username || username.includes(':')) {
          continue;
        }
        users.add(username);
      }
      cursor = list.list_complete ? undefined : list.cursor;
    } while (cursor);

    return Array.from(users).sort();
  }

  /** 记录最近 5 条活动，仅保存动作与时间，不保存用户名。 */
  private async recordActivity(action: string): Promise<void> {
    const raw = await this.env.KV.get('stats:recent');
    let recent: ActivityItem[] = [];
    if (raw) {
      try {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) recent = parsed;
      } catch {
        // 覆盖损坏的缓存
      }
    }
    recent.unshift({ action, time: new Date().toISOString() });
    await this.env.KV.put('stats:recent', JSON.stringify(recent.slice(0, 5)));
  }
}
