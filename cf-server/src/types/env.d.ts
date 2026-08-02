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

  // 索引签名，满足 Hono 的 Env 泛型约束
  [key: string]: string | KVNamespace | Fetcher;
}

/** SSH 公钥的元数据信息 */
export interface KeyMetadata {
  updated_at: string;   // 最后更新时间（ISO 8601）
  source?: string;      // 密钥来源（可选，如 "github"、"manual"）
}

/** 统一错误响应格式 */
export interface ErrorResponse {
  error: string;
}
