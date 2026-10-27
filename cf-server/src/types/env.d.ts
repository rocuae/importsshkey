/** Workers 原生 Rate Limiting 绑定接口 */
export interface RateLimitBinding {
  /** 按 key 消费一个限流令牌；success=false 表示已超出限制 */
  limit(options: { key: string }): Promise<{ success: boolean }>;
}

/** Cloudflare Workers 环境变量接口 */
export interface Env {
  // KV 命名空间绑定，用于存储 SSH 公钥数据
  KV: KVNamespace;

  // 静态资源绑定，用于服务 HTML 等静态文件
  ASSETS: Fetcher;

  // 环境变量（在 wrangler.jsonc 中配置）
  API_VERSION: string;
  ALLOWED_ORIGINS: string;

  // 管理员令牌（通过 wrangler secret put 设置，用于写操作鉴权）
  ADMIN_TOKEN: string;

  // Workers 原生 Rate Limiting 绑定；未配置时使用内存兜底
  AUTH_RATE_LIMITER?: RateLimitBinding;

  // 索引签名，满足 Hono 的 Env 泛型约束
  [key: string]: string | KVNamespace | Fetcher | RateLimitBinding | undefined;
}

/** 单把公钥的存储结构 */
export interface StoredKey {
  /** 公钥原文 */
  public_key: string;
  /** OpenSSH SHA256 指纹 */
  fingerprint: string;
  /** 来源标识，可为空 */
  source: string;
  /** 首次写入时间（ISO8601） */
  created_at: string;
  /** 最近更新时间（ISO8601） */
  updated_at: string;
}

/** 每个用户的 KV 值：单键 envelope，避免多键不一致 */
export interface UserEnvelope {
  /** envelope 版本，当前固定为 1 */
  version: 1;
  /** 该用户的全部公钥 */
  keys: StoredKey[];
}

/** 活动记录（不含用户名） */
export interface ActivityItem {
  /** 动作：add/replace/delete */
  action: string;
  /** 发生时间（ISO8601） */
  time: string;
}

/** 旧版元数据格式（兼容读取） */
export interface KeyMetadata {
  /** 最近更新时间 */
  updated_at: string;
  /** 来源标识 */
  source?: string;
}

/** 统一错误响应格式 */
export interface ErrorResponse {
  error: string;
}
