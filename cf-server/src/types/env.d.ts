export interface Env {
  // KV 绑定
  PUBLIC_KEYS: KVNamespace;
  
  // 环境变量
  API_VERSION: string;
  ALLOWED_ORIGINS: string;
  
  // Secrets（通过 wrangler secret put 设置）
  ADMIN_TOKEN: string;
  
  // 索引签名，满足 Hono 的 Env 约束
  [key: string]: string | KVNamespace;
}

export interface KeyMetadata {
  updated_at: string;
  source?: string;
}

export interface ErrorResponse {
  error: string;
}
