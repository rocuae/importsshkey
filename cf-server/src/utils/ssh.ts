/** SSH 公钥校验与指纹计算工具 */

/** 用户名规则：首字符为字母或数字，后续允许 _ . @ + -，最长 64 */
const USERNAME_RE = /^[A-Za-z0-9][A-Za-z0-9_.@+-]{0,63}$/;
/** 支持的 SSH 公钥类型白名单（含 FIDO/security-key 变体） */
const KEY_TYPE_RE =
  /^(ssh-(ed25519|rsa|dss)|ecdsa-sha2-nistp(256|384|521)|sk-(ssh-ed25519|ecdsa-sha2-nistp256)@openssh\.com)$/;

/** 校验用户名（同时防止 KV 键名注入） */
export function validateUsername(username: string): string | null {
  if (!username || !USERNAME_RE.test(username)) {
    return 'Invalid username: use 1-64 letters, digits, or _ . @ + -';
  }
  return null;
}

/** 校验 SSH 公钥：类型白名单 + base64 解码 + 内嵌类型一致 */
export function validateSshKey(publicKey: string): string | null {
  const trimmed = publicKey.trim();
  if (!trimmed) {
    return 'public_key is required';
  }
  const parts = trimmed.split(/\s+/);
  if (parts.length < 2) {
    return 'Invalid SSH public key format';
  }
  const [type, encoded] = parts;
  if (!KEY_TYPE_RE.test(type)) {
    return `Unsupported SSH key type: ${type}`;
  }

  let bytes: Uint8Array;
  try {
    const binary = atob(encoded);
    bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i);
    }
  } catch {
    return 'Invalid SSH public key encoding';
  }

  if (bytes.length < 4) {
    return 'Invalid SSH public key encoding';
  }
  const typeLength = ((bytes[0] << 24) | (bytes[1] << 16) | (bytes[2] << 8) | bytes[3]) >>> 0;
  if (typeLength === 0 || 4 + typeLength > bytes.length) {
    return 'Invalid SSH public key blob';
  }
  const embeddedType = new TextDecoder().decode(bytes.slice(4, 4 + typeLength));
  if (embeddedType !== type) {
    return 'SSH public key type does not match its blob';
  }
  return null;
}

/** 计算 OpenSSH SHA256 指纹（SHA256:base64，无 padding） */
export async function sshFingerprint(publicKey: string): Promise<string> {
  const parts = publicKey.trim().split(/\s+/);
  if (parts.length < 2) {
    throw new Error('Invalid SSH public key');
  }
  const binary = atob(parts[1]);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return `SHA256:${bytesToBase64(new Uint8Array(digest)).replace(/=+$/, '')}`;
}

/** 分块把字节数组编码为 base64，避免大数组触发参数上限 */
function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
  }
  return btoa(binary);
}
