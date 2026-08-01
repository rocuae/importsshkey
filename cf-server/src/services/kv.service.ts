import { Env, KeyMetadata } from '../types/env';

/**
 * Cloudflare KV 存储服务
 * 封装 SSH 公钥的 CRUD 操作
 * KV 键名规则：user:{username} 存公钥，user:{username}:metadata 存元数据
 */
export class KVService {
  constructor(private env: Env) {}

  /** 根据用户名获取 SSH 公钥 */
  async getPublicKey(username: string): Promise<string | null> {
    const key = `user:${username}`;
    return await this.env.KV.get(key);
  }

  /** 获取公钥及其元数据（并行读取，减少延迟） */
  async getPublicKeyWithMetadata(username: string): Promise<{
    publicKey: string | null;
    metadata: KeyMetadata | null;
  }> {
    const [publicKey, metadataRaw] = await Promise.all([
      this.env.KV.get(`user:${username}`),
      this.env.KV.get(`user:${username}:metadata`),
    ]);

    let metadata: KeyMetadata | null = null;
    if (metadataRaw) {
      try {
        metadata = JSON.parse(metadataRaw);
      } catch {
        // 元数据格式错误，忽略
      }
    }

    return { publicKey, metadata };
  }

  /** 写入公钥，同时记录更新时间和来源 */
  async putPublicKey(username: string, publicKey: string, source?: string): Promise<void> {
    const now = new Date().toISOString();
    const metadata: KeyMetadata = {
      updated_at: now,
      source,
    };

    // 并行写入公钥和元数据
    await Promise.all([
      this.env.KV.put(`user:${username}`, publicKey),
      this.env.KV.put(`user:${username}:metadata`, JSON.stringify(metadata)),
    ]);
  }

  /** 删除公钥及其元数据 */
  async deletePublicKey(username: string): Promise<void> {
    await Promise.all([
      this.env.KV.delete(`user:${username}`),
      this.env.KV.delete(`user:${username}:metadata`),
    ]);
  }

  /** 列出所有用户（过滤掉 metadata 键） */
  async listUsers(): Promise<string[]> {
    const list = await this.env.KV.list({ prefix: 'user:' });
    const users = new Set<string>();
    for (const key of list.keys) {
      if (!key.name.endsWith(':metadata')) {
        const username = key.name.replace('user:', '');
        users.add(username);
      }
    }
    return Array.from(users);
  }
}
