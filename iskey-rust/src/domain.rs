//! 公钥条目、目标校验与注释格式。

use anyhow::{Result, anyhow, bail};
use base64::Engine;
use base64::engine::general_purpose::STANDARD as B64;
use sha2::{Digest, Sha256};
use ssh_key::authorized_keys::Entry;

/// 一条 SSH 公钥记录，字段名与 Go 版 iskey 的 JSON 输出一致
#[derive(Debug, Clone, PartialEq, Default, serde::Serialize)]
pub struct KeyEntry {
    /// SSH 公钥原文（不含注释）
    #[serde(rename = "PublicKey")]
    pub public_key: String,
    /// 来源标识（github/launchpad/自定义源名）
    #[serde(rename = "Source")]
    pub source: String,
    /// 目标用户名
    #[serde(rename = "User")]
    pub user: String,
    /// SHA256 指纹（SHA256:base64，无 padding）
    #[serde(rename = "Fingerprint")]
    pub fingerprint: String,
    /// authorized_keys 中的完整注释
    #[serde(rename = "Comment")]
    pub comment: String,
}

impl KeyEntry {
    /// 创建公钥条目，自动校验格式并计算指纹
    pub fn new(public_key: &str, source: &str, user: &str) -> Result<Self> {
        validate_target(source, user)?;
        let public_key = public_key.trim().to_string();
        if public_key.is_empty() {
            anyhow::bail!("public key is empty");
        }
        let fingerprint = fingerprint(&public_key)?;
        let comment = format_comment(source, user, &fingerprint);
        Ok(Self {
            public_key,
            source: source.to_string(),
            user: user.to_string(),
            fingerprint,
            comment,
        })
    }
}

/// 校验数据源名称，规则与 Go 版 domain.ValidateSource 一致。
pub fn validate_source(source: &str) -> Result<()> {
    if source.is_empty() {
        bail!("source is required");
    }
    if source.len() > 64 {
        bail!("source is too long (max 64 characters)");
    }
    if !source
        .chars()
        .next()
        .is_some_and(|c| c.is_ascii_alphanumeric())
    {
        bail!("source must start with a letter or digit: {:?}", source);
    }
    if !source
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || matches!(c, '_' | '.' | '@' | '+' | '-'))
    {
        bail!("source contains invalid characters: {:?}", source);
    }
    Ok(())
}

/// 校验用户名，规则与 Go 版 domain.ValidateUser 一致。
pub fn validate_user(user: &str) -> Result<()> {
    if user.is_empty() {
        bail!("user is required");
    }
    if user.len() > 128 {
        bail!("user is too long (max 128 characters)");
    }
    if !user
        .chars()
        .next()
        .is_some_and(|c| c.is_ascii_alphanumeric())
    {
        bail!("user must start with a letter or digit: {:?}", user);
    }
    if !user
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || matches!(c, '_' | '.' | '@' | '+' | '~' | '-'))
    {
        bail!("user contains invalid characters: {:?}", user);
    }
    Ok(())
}

/// 同时校验 source 与 user。
pub fn validate_target(source: &str, user: &str) -> Result<()> {
    validate_source(source)?;
    validate_user(user)?;
    Ok(())
}

/// 计算 SHA256 指纹，格式 `SHA256:<base64>`（带 padding，与 Go base64.StdEncoding 一致）
pub fn fingerprint(public_key: &str) -> Result<String> {
    let entry = parse_entry(public_key.trim())?;
    let bytes = entry.public_key().to_bytes()?;
    let digest = Sha256::digest(&bytes);
    Ok(format!("SHA256:{}", B64.encode(digest)))
}

/// 解析单行 authorized_keys，失败时保留原始错误上下文。
fn parse_entry(public_key: &str) -> Result<Entry> {
    public_key
        .parse::<Entry>()
        .map_err(|e| anyhow!("invalid SSH public key format: {}", e))
}

/// 格式化 authorized_keys 注释，格式 `iskey:<source>:<user>:<fingerprint>`
pub fn format_comment(source: &str, user: &str, fingerprint: &str) -> String {
    format!("iskey:{source}:{user}:{fingerprint}")
}

/// 解析 authorized_keys 注释，返回 (source, user, fingerprint)
pub fn parse_comment(comment: &str) -> Option<(String, String, String)> {
    let comment = comment.trim();
    let mut parts = comment.splitn(4, ':');
    if parts.next()? != "iskey" {
        return None;
    }
    let source = parts.next()?.to_string();
    let user = parts.next()?.to_string();
    let fingerprint = parts.next()?.to_string();
    Some((source, user, fingerprint))
}

#[cfg(test)]
mod tests {
    use super::*;

    // 与 Go 版 iskey 计算一致：ssh-keygen -lf 输出为 SHA256:7MaIW0hEEsFKO7AbaY7AIztsc1Zzyf7jwHoWuUu54Rs（无 padding）
    // Go base64.StdEncoding 带一个 = 的 padding
    const KEY: &str = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO95iT5q2IE3X24g2Pfq0v8s9erHGOo2VYrl/nvAVV95 test@example.com";

    #[test]
    fn test_fingerprint_matches_go() {
        assert_eq!(
            fingerprint(KEY).unwrap(),
            "SHA256:7MaIW0hEEsFKO7AbaY7AIztsc1Zzyf7jwHoWuUu54Rs="
        );
    }

    #[test]
    fn test_comment_roundtrip() {
        let entry = KeyEntry::new(KEY, "github", "octocat").unwrap();
        assert_eq!(
            entry.comment,
            "iskey:github:octocat:SHA256:7MaIW0hEEsFKO7AbaY7AIztsc1Zzyf7jwHoWuUu54Rs="
        );
        let (source, user, fp) = parse_comment(&entry.comment).unwrap();
        assert_eq!(source, "github");
        assert_eq!(user, "octocat");
        assert_eq!(fp, entry.fingerprint);
    }

    #[test]
    fn test_parse_comment_invalid() {
        assert!(parse_comment("iskey:a:b").is_none());
        assert!(parse_comment("other:x:y:z").is_none());
        assert!(parse_comment("").is_none());
    }

    #[test]
    fn test_validate_target() {
        assert!(validate_target("work", "alice").is_ok());
        assert!(validate_target("my-source_1", "alice@example.com").is_ok());
        assert!(validate_target("", "alice").is_err());
        assert!(validate_target("work", "").is_err());
        assert!(validate_target("-work", "alice").is_err());
        assert!(validate_target("work", "alice:root").is_err());
        assert!(validate_target("work", "alice/root").is_err());
        assert!(validate_target(&"a".repeat(65), "alice").is_err());
        assert!(validate_target("work", &"a".repeat(129)).is_err());
    }
}
