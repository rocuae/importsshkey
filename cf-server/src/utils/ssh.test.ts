/** 本文件覆盖用户名、公钥校验与 OpenSSH 指纹计算。 */
import { describe, expect, it } from 'vitest';
import { sshFingerprint, validateSshKey, validateUsername } from './ssh';

const VALID_KEY =
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@iskey';

describe('validateUsername', () => {
  it('accepts normal names', () => {
    expect(validateUsername('alice')).toBeNull();
    expect(validateUsername('alice.bob+ops')).toBeNull();
  });

  it('rejects KV namespace injection and empty names', () => {
    expect(validateUsername('')).not.toBeNull();
    expect(validateUsername('alice:metadata')).not.toBeNull();
    expect(validateUsername('../alice')).not.toBeNull();
    expect(validateUsername('a'.repeat(65))).not.toBeNull();
  });
});

describe('validateSshKey', () => {
  it('accepts a valid ed25519 key', () => {
    expect(validateSshKey(VALID_KEY)).toBeNull();
  });

  it('rejects garbage and mismatched blobs', () => {
    expect(validateSshKey('ssh-garbage')).not.toBeNull();
    expect(validateSshKey('ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl')).not.toBeNull();
    expect(validateSshKey('ecdsa-sha2-nistp256 !!!')).not.toBeNull();
    expect(validateSshKey('')).not.toBeNull();
  });
});

describe('sshFingerprint', () => {
  it('matches OpenSSH SHA256 format', async () => {
    const fingerprint = await sshFingerprint(VALID_KEY);
    expect(fingerprint).toMatch(/^SHA256:[A-Za-z0-9+/]+$/);
    expect(fingerprint).toBe(await sshFingerprint(VALID_KEY));
  });
});
