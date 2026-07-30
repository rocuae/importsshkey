import { Env, KeyMetadata } from '../types/env';

export class KVService {
  constructor(private env: Env) {}

  async getPublicKey(username: string): Promise<string | null> {
    const key = `user:${username}`;
    return await this.env.PUBLIC_KEYS.get(key);
  }

  async getPublicKeyWithMetadata(username: string): Promise<{
    publicKey: string | null;
    metadata: KeyMetadata | null;
  }> {
    const [publicKey, metadataRaw] = await Promise.all([
      this.env.PUBLIC_KEYS.get(`user:${username}`),
      this.env.PUBLIC_KEYS.get(`user:${username}:metadata`),
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

  async putPublicKey(username: string, publicKey: string, source?: string): Promise<void> {
    const now = new Date().toISOString();
    const metadata: KeyMetadata = {
      updated_at: now,
      source,
    };

    await Promise.all([
      this.env.PUBLIC_KEYS.put(`user:${username}`, publicKey),
      this.env.PUBLIC_KEYS.put(`user:${username}:metadata`, JSON.stringify(metadata)),
    ]);
  }

  async deletePublicKey(username: string): Promise<void> {
    await Promise.all([
      this.env.PUBLIC_KEYS.delete(`user:${username}`),
      this.env.PUBLIC_KEYS.delete(`user:${username}:metadata`),
    ]);
  }

  async listUsers(): Promise<string[]> {
    const list = await this.env.PUBLIC_KEYS.list({ prefix: 'user:' });
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
