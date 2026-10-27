//! HTTP 抓取器工厂、URL 模板与重试/响应体限制。

use std::collections::HashMap;
use std::io::Read;
use std::thread;
use std::time::Duration;

use anyhow::{Context, Result, anyhow, bail};

use base64::Engine;
use base64::engine::general_purpose::STANDARD as B64;

use crate::config::{Config, Credential, SourceConfig};
use crate::domain::KeyEntry;

/// 响应体大小上限，防止恶意端点耗尽内存
const MAX_RESPONSE_BYTES: u64 = 4 << 20;

/// 公钥拉取器
pub trait Fetcher {
    /// 拉取并解析用户的全部公钥。
    fn fetch(&self) -> Result<Vec<KeyEntry>>;
}

/// 构建带全局超时与可选证书跳过校验的 ureq Agent。
fn build_agent(timeout: u64, insecure: bool) -> ureq::Agent {
    let mut tls = ureq::tls::TlsConfig::builder();
    if insecure {
        tls = tls.disable_verification(true);
    }
    ureq::Agent::config_builder()
        .tls_config(tls.build())
        .timeout_global(Some(Duration::from_secs(timeout)))
        .http_status_as_error(false)
        .build()
        .new_agent()
}

/// GitHub 公钥拉取器
pub struct GitHubFetcher {
    /// 共享的 ureq Agent（超时/TLS 已配置）
    agent: ureq::Agent,
    /// 可选 GitHub Token
    token: String,
    /// 目标用户名
    user: String,
    /// 最大重试次数
    max_retries: u32,
}

impl Fetcher for GitHubFetcher {
    fn fetch(&self) -> Result<Vec<KeyEntry>> {
        let url = format!(
            "https://api.github.com/users/{}/keys?per_page=100",
            self.user
        );
        let mut attempt = 0;
        let resp = loop {
            let mut req = self.agent.get(&url);
            if !self.token.is_empty() {
                req = req.header("Authorization", &format!("Bearer {}", self.token));
            }
            req = req.header("Accept", "application/vnd.github.v3+json");
            match req.call() {
                Ok(r) if retryable_status(r.status().as_u16()) && attempt < self.max_retries => {
                    attempt += 1;
                    backoff(attempt);
                }
                Ok(r) => break r,
                Err(_e) if attempt < self.max_retries => {
                    attempt += 1;
                    backoff(attempt);
                }
                Err(e) => {
                    return Err(e).with_context(|| {
                        format!("GitHub API request failed for user {}", self.user)
                    });
                }
            }
        };
        if resp.status() != 200 {
            bail!("GitHub API returned {}", resp.status());
        }
        let body = read_body(resp).context("read GitHub API response")?;
        let keys: Vec<GitHubKey> =
            serde_json::from_slice(&body).context("parse GitHub API response")?;
        let mut entries = Vec::new();
        for k in keys {
            if let Ok(e) = KeyEntry::new(&k.key, "github", &self.user) {
                entries.push(e);
            }
        }
        Ok(entries)
    }
}

/// Launchpad 公钥拉取器
pub struct LaunchpadFetcher {
    /// 共享的 ureq Agent
    agent: ureq::Agent,
    /// 目标用户名
    user: String,
    /// 最大重试次数
    max_retries: u32,
}

impl Fetcher for LaunchpadFetcher {
    fn fetch(&self) -> Result<Vec<KeyEntry>> {
        let url = format!("https://launchpad.net/~{}/+sshkeys", self.user);
        let mut attempt = 0;
        let resp = loop {
            match self.agent.get(&url).call() {
                Ok(r) if retryable_status(r.status().as_u16()) && attempt < self.max_retries => {
                    attempt += 1;
                    backoff(attempt);
                }
                Ok(r) => break r,
                Err(_e) if attempt < self.max_retries => {
                    attempt += 1;
                    backoff(attempt);
                }
                Err(e) => {
                    return Err(e).with_context(|| {
                        format!("Launchpad request failed for user {}", self.user)
                    });
                }
            }
        };
        if resp.status() != 200 {
            bail!("Launchpad returned {}", resp.status());
        }
        let body = read_body(resp).context("read Launchpad response")?;
        parse_plaintext(&body, "launchpad", &self.user)
    }
}

/// 通用 HTTP 公钥拉取器，支持 plaintext 和 github_json
pub struct HTTPFetcher {
    /// 共享的 ureq Agent
    agent: ureq::Agent,
    /// 已渲染模板变量的请求地址
    url: String,
    /// 响应格式：plaintext/github_json/auto
    format: String,
    /// 目标用户名
    user: String,
    /// 来源标识（写入 KeyEntry）
    source: String,
    /// Bearer Token（与 basic 二选一）
    bearer: Option<String>,
    /// Basic Auth 用户名与密码
    basic: Option<(String, String)>,
    /// 最大重试次数
    max_retries: u32,
}

impl Fetcher for HTTPFetcher {
    fn fetch(&self) -> Result<Vec<KeyEntry>> {
        let mut attempt = 0;
        let resp = loop {
            let mut req = self.agent.get(&self.url);
            if let Some(token) = &self.bearer {
                req = req.header("Authorization", &format!("Bearer {}", token));
            }
            if let Some((user_raw, pass)) = &self.basic {
                let cred = B64.encode(format!("{user_raw}:{pass}"));
                req = req.header("Authorization", &format!("Basic {}", cred));
            }
            match req.call() {
                Ok(r) if retryable_status(r.status().as_u16()) && attempt < self.max_retries => {
                    attempt += 1;
                    backoff(attempt);
                }
                Ok(r) => break r,
                Err(_e) if attempt < self.max_retries => {
                    attempt += 1;
                    backoff(attempt);
                }
                Err(e) => {
                    return Err(e).with_context(|| format!("request failed: {}", self.url));
                }
            }
        };
        if resp.status() != 200 {
            bail!("HTTP {}", resp.status());
        }
        let body = read_body(resp).context("read response body")?;
        match self.format.as_str() {
            "github_json" => parse_github_json(&body, &self.source, &self.user),
            "auto" => {
                if body.iter().find(|b| !b.is_ascii_whitespace()) == Some(&b'[') {
                    parse_github_json(&body, &self.source, &self.user)
                } else {
                    parse_plaintext(&body, &self.source, &self.user)
                }
            }
            _ => parse_plaintext(&body, &self.source, &self.user),
        }
    }
}

/// 读取响应体并限制最大 4 MiB，超限直接报错。
fn read_body(resp: ureq::http::Response<ureq::Body>) -> Result<Vec<u8>> {
    let mut body = Vec::new();
    resp.into_body()
        .into_reader()
        .take(MAX_RESPONSE_BYTES + 1)
        .read_to_end(&mut body)?;
    if body.len() as u64 > MAX_RESPONSE_BYTES {
        bail!("response body exceeds {} bytes", MAX_RESPONSE_BYTES);
    }
    Ok(body)
}

/// 429 与 5xx 视为可重试状态。
fn retryable_status(status: u16) -> bool {
    status == 429 || status >= 500
}

/// 指数退避：200ms 起，单次上限 5 秒。
fn backoff(attempt: u32) {
    let millis = 200u64.saturating_mul(1u64 << attempt.min(4));
    thread::sleep(Duration::from_millis(millis.min(5_000)));
}

#[derive(serde::Deserialize)]
struct GitHubKey {
    key: String,
}

/// 解析纯文本格式公钥，空行与 # 注释跳过，无效公钥忽略
fn parse_plaintext(body: &[u8], source: &str, user: &str) -> Result<Vec<KeyEntry>> {
    let text = String::from_utf8_lossy(body);
    let mut entries = Vec::new();
    for raw in text.lines() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if let Ok(e) = KeyEntry::new(line, source, user) {
            entries.push(e);
        }
    }
    Ok(entries)
}

/// 解析 GitHub /keys JSON，跳过无效公钥。
fn parse_github_json(body: &[u8], source: &str, user: &str) -> Result<Vec<KeyEntry>> {
    let keys: Vec<GitHubKey> = serde_json::from_slice(body).context("parse JSON response")?;
    let mut entries = Vec::new();
    for k in keys {
        if let Ok(e) = KeyEntry::new(&k.key, source, user) {
            entries.push(e);
        }
    }
    Ok(entries)
}

/// 根据源配置创建拉取器
/// - `source_name`: 源在配置文件中的名称（用于自定义源打标签）
#[allow(clippy::too_many_arguments)]
pub fn factory(
    source: &SourceConfig,
    source_name: &str,
    creds: Option<&Credential>,
    user: &str,
    cli_vars: &HashMap<String, String>,
    timeout: u64,
    insecure: bool,
    max_retries: u32,
) -> Result<Box<dyn Fetcher>> {
    let agent = build_agent(timeout, insecure);
    match source_name {
        "github" => {
            let token = creds
                .filter(|c| c.Type == "bearer")
                .map(|c| c.ResolvedValue.clone())
                .unwrap_or_default();
            Ok(Box::new(GitHubFetcher {
                agent,
                token,
                user: user.to_string(),
                max_retries,
            }))
        }
        "launchpad" => Ok(Box::new(LaunchpadFetcher {
            agent,
            user: user.to_string(),
            max_retries,
        })),
        _ => {
            let url = resolve_url(&source.URL, user, &source.DefaultVars, cli_vars)?;
            validate_url(&url)?;
            let format = if source.Format.is_empty() {
                "plaintext"
            } else {
                &source.Format
            };
            let (bearer, basic) = match creds {
                Some(c) if c.Type == "bearer" => (Some(c.ResolvedValue.clone()), None),
                Some(c) if c.Type == "basic" => (
                    None,
                    Some((c.ResolvedUsername.clone(), c.ResolvedPassword.clone())),
                ),
                _ => (None, None),
            };
            Ok(Box::new(HTTPFetcher {
                agent,
                url,
                format: format.to_string(),
                user: user.to_string(),
                source: source_name.to_string(),
                bearer,
                basic,
                max_retries,
            }))
        }
    }
}

/// 校验静态 URL 的协议与主机名
pub fn validate_url(raw: &str) -> Result<()> {
    if !(raw.starts_with("http://") || raw.starts_with("https://")) {
        bail!("invalid URL {:?}: scheme must be http or https", raw);
    }
    let rest = raw
        .strip_prefix("http://")
        .or_else(|| raw.strip_prefix("https://"))
        .unwrap_or("");
    let host = rest.split(['/', '?', '#']).next().unwrap_or("");
    if host.is_empty() {
        bail!("invalid URL {:?}: host is required", raw);
    }
    Ok(())
}

/// 解析模板 URL，支持 {{ .Var }} 与 {{ Var }} 语法
/// 变量优先级：User < default_vars < CLI --var
pub fn resolve_url(
    template: &str,
    user: &str,
    default_vars: &HashMap<String, String>,
    cli_vars: &HashMap<String, String>,
) -> Result<String> {
    if template.is_empty() {
        bail!("source URL is empty");
    }
    if !template.contains("{{") {
        return Ok(template.to_string());
    }
    let mut vars = HashMap::new();
    vars.insert("User".to_string(), user.to_string());
    for (k, v) in default_vars {
        vars.insert(k.clone(), v.clone());
    }
    for (k, v) in cli_vars {
        vars.insert(k.clone(), v.clone());
    }
    expand_template(template, &vars)
}

/// 用 {{ .Key }} 语法替换模板变量，缺失变量报错。
fn expand_template(input: &str, vars: &HashMap<String, String>) -> Result<String> {
    let mut out = String::new();
    let mut rest = input;
    while let Some(start) = rest.find("{{") {
        out.push_str(&rest[..start]);
        let after = &rest[start + 2..];
        let end = after
            .find("}}")
            .ok_or_else(|| anyhow!("unclosed template variable in URL template"))?;
        let name = after[..end].trim().trim_start_matches('.');
        let value = vars
            .get(name)
            .ok_or_else(|| anyhow!("template variable {} not provided", name))?;
        out.push_str(value);
        rest = &after[end + 2..];
    }
    out.push_str(rest);
    Ok(out)
}

/// 按名称或别名解析源，未找到时回退内置源
pub fn resolve_alias(cfg: &Config, alias: &str) -> Result<(String, SourceConfig)> {
    if let Some((name, src)) = cfg.get_source(alias) {
        return Ok((name, src.clone()));
    }
    if let Some(builtin) = get_builtin_source(alias) {
        let name = match alias {
            "gh" | "github" => "github",
            _ => "launchpad",
        };
        return Ok((name.to_string(), builtin));
    }
    bail!("source not found: {}", alias)
}

/// 内置数据源配置（无需配置文件即可使用）
pub fn get_builtin_source(alias: &str) -> Option<SourceConfig> {
    match alias {
        "gh" | "github" => Some(SourceConfig {
            Alias: "gh".to_string(),
            URL: "https://api.github.com/users/{{ .User }}/keys".to_string(),
            Format: "github_json".to_string(),
            ..Default::default()
        }),
        "lp" | "launchpad" => Some(SourceConfig {
            Alias: "lp".to_string(),
            URL: "https://launchpad.net/~{{ .User }}/+sshkeys".to_string(),
            Format: "plaintext".to_string(),
            ..Default::default()
        }),
        _ => None,
    }
}

/// 判断别名是否指向内置源。
pub fn is_builtin_source(alias: &str) -> bool {
    get_builtin_source(alias).is_some()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_expand_template() {
        let mut vars = HashMap::new();
        vars.insert("User".to_string(), "octocat".to_string());
        vars.insert("Team".to_string(), "backend".to_string());
        assert_eq!(
            expand_template("https://x/{{ .User }}", &vars).unwrap(),
            "https://x/octocat"
        );
        assert_eq!(
            expand_template("https://x/{{ User }}/{{.Team}}", &vars).unwrap(),
            "https://x/octocat/backend"
        );
        assert!(expand_template("https://x/{{ .Missing }}", &vars).is_err());
        assert!(expand_template("https://x/{{ .User", &vars).is_err());
    }

    #[test]
    fn test_resolve_url_static() {
        let vars = HashMap::new();
        assert_eq!(
            resolve_url("https://static.example.com/keys.pub", "u", &vars, &vars).unwrap(),
            "https://static.example.com/keys.pub"
        );
        assert!(resolve_url("", "u", &vars, &vars).is_err());
    }

    #[test]
    fn test_builtin_sources() {
        assert!(is_builtin_source("gh"));
        assert!(is_builtin_source("github"));
        assert!(is_builtin_source("lp"));
        assert!(!is_builtin_source("work"));
    }
}
