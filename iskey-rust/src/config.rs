//! 配置模型、YAML 加载与原子保存。

use std::collections::HashMap;
use std::env;
use std::fmt::Write as _;
use std::fs;
use std::path::Path;

use anyhow::{Context, Result, anyhow, bail};
use serde::{Deserialize, Serialize};

use crate::domain;

/// 全局配置，字段名与 Go 版 iskey 对齐（JSON 输出 PascalCase，YAML 解析用 alias 小写下划线）
#[allow(non_snake_case)]
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Config {
    /// 全局默认值（输出路径、超时、重试、TLS）
    #[serde(default, alias = "defaults")]
    pub Defaults: Defaults,
    /// 命名认证凭据，供 source.auth_ref 引用
    #[serde(default, alias = "credentials")]
    pub Credentials: HashMap<String, Credential>,
    /// 自定义数据源，键为规范源名
    #[serde(default, alias = "sources")]
    pub Sources: HashMap<String, SourceConfig>,
}

impl Default for Config {
    /// 生成与 Go 版一致的默认配置。
    fn default() -> Self {
        Self {
            Defaults: Defaults {
                Output: "~/.ssh/authorized_keys".to_string(),
                Timeout: 10,
                MaxRetries: 3,
                InsecureSkipVerify: false,
            },
            Credentials: HashMap::new(),
            Sources: HashMap::new(),
        }
    }
}

#[allow(non_snake_case)]
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Defaults {
    /// 默认目标 authorized_keys 路径
    #[serde(default, alias = "output")]
    pub Output: String,
    /// HTTP 超时秒数；0 表示使用 10 秒
    #[serde(default, alias = "timeout")]
    pub Timeout: u64,
    /// 是否跳过 TLS 证书校验（仅建议内网自签名）
    #[serde(default, alias = "insecure_skip_verify")]
    pub InsecureSkipVerify: bool,
    /// 可重试错误的最大重试次数
    #[serde(default, alias = "max_retries")]
    pub MaxRetries: u32,
}

impl Defaults {
    /// 返回有效超时秒数；0 使用默认 10 秒。
    pub fn timeout(&self) -> u64 {
        if self.Timeout == 0 { 10 } else { self.Timeout }
    }
}

#[allow(non_snake_case)]
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Credential {
    /// 认证类型：bearer/basic/none
    #[serde(default, alias = "type")]
    pub Type: String,
    /// bearer token 所在环境变量名
    #[serde(default, alias = "value_from_env")]
    pub ValueFromEnv: String,
    /// basic 用户名所在环境变量名
    #[serde(default, alias = "username_from_env")]
    pub UsernameFromEnv: String,
    /// basic 密码所在环境变量名
    #[serde(default, alias = "password_from_env")]
    pub PasswordFromEnv: String,
    /// 运行时解析出的 token，不参与序列化
    #[serde(skip)]
    pub ResolvedValue: String,
    /// 运行时解析出的 basic 用户名，不参与序列化
    #[serde(skip)]
    pub ResolvedUsername: String,
    /// 运行时解析出的 basic 密码，不参与序列化
    #[serde(skip)]
    pub ResolvedPassword: String,
}

#[allow(non_snake_case)]
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct SourceConfig {
    /// 命令行使用的短别名，默认等于源名
    #[serde(default, alias = "alias")]
    pub Alias: String,
    /// 请求地址，支持 {{ .User }} 等模板变量
    #[serde(default, alias = "url")]
    pub URL: String,
    /// 响应格式：plaintext/github_json/auto
    #[serde(default, alias = "format")]
    pub Format: String,
    /// 引用的凭据名称
    #[serde(default, alias = "auth_ref")]
    pub AuthRef: String,
    /// 模板默认变量，可被 --var 覆盖
    #[serde(default, alias = "default_vars")]
    pub DefaultVars: HashMap<String, String>,
}

impl Config {
    /// 校验超时、重试、源、凭据与别名的完整性。
    pub fn validate(&self) -> Result<()> {
        if self.Defaults.Timeout > 3600 {
            bail!("defaults.timeout must be <= 3600 seconds");
        }
        if self.Defaults.MaxRetries > 10 {
            bail!("defaults.max_retries must be <= 10");
        }

        let mut aliases = std::collections::HashSet::new();
        for (name, src) in &self.Sources {
            domain::validate_source(name).map_err(|e| anyhow!("source {:?}: {}", name, e))?;
            if src.URL.is_empty() {
                bail!("source {:?}: url is required", name);
            }
            match src.Format.as_str() {
                "" | "plaintext" | "github_json" | "auto" => {}
                other => bail!(
                    "source {:?}: invalid format {:?} (must be plaintext, github_json, or auto)",
                    name,
                    other
                ),
            }
            let alias = if src.Alias.is_empty() {
                name
            } else {
                &src.Alias
            };
            domain::validate_source(alias)
                .map_err(|e| anyhow!("source {:?}: alias: {}", name, e))?;
            if !aliases.insert(alias.to_string()) {
                bail!("source {:?}: duplicate alias {:?}", name, alias);
            }
            if !src.AuthRef.is_empty() && !self.Credentials.contains_key(&src.AuthRef) {
                bail!(
                    "source {:?}: auth_ref {:?} does not exist",
                    name,
                    src.AuthRef
                );
            }
        }

        for (name, cred) in &self.Credentials {
            match cred.Type.as_str() {
                "bearer" => {
                    if cred.ValueFromEnv.is_empty() {
                        bail!(
                            "credential {:?}: value_from_env is required for bearer type",
                            name
                        );
                    }
                }
                "basic" => {
                    if cred.UsernameFromEnv.is_empty() || cred.PasswordFromEnv.is_empty() {
                        bail!(
                            "credential {:?}: username_from_env and password_from_env are required for basic type",
                            name
                        );
                    }
                }
                "none" | "" => {}
                other => bail!(
                    "credential {:?}: unsupported type {:?} (must be bearer, basic, or none)",
                    name,
                    other
                ),
            }
        }
        Ok(())
    }

    /// 解析单个被引用的凭据（未引用的凭据不要求环境变量存在）
    pub fn resolved_credential(&self, name: &str) -> Result<Option<Credential>> {
        if name.is_empty() {
            return Ok(None);
        }
        let mut cred = self
            .Credentials
            .get(name)
            .cloned()
            .ok_or_else(|| anyhow!("credential {:?} does not exist", name))?;
        match cred.Type.as_str() {
            "bearer" => {
                if cred.ValueFromEnv.is_empty() {
                    bail!(
                        "credential {:?}: value_from_env is required for bearer type",
                        name
                    );
                }
                cred.ResolvedValue = env::var(&cred.ValueFromEnv).unwrap_or_default();
                if cred.ResolvedValue.is_empty() {
                    bail!(
                        "credential {:?}: environment variable {} is not set",
                        name,
                        cred.ValueFromEnv
                    );
                }
            }
            "basic" => {
                if cred.UsernameFromEnv.is_empty() || cred.PasswordFromEnv.is_empty() {
                    bail!(
                        "credential {:?}: username_from_env and password_from_env are required for basic type",
                        name
                    );
                }
                cred.ResolvedUsername = env::var(&cred.UsernameFromEnv).unwrap_or_default();
                if cred.ResolvedUsername.is_empty() {
                    bail!(
                        "credential {:?}: environment variable {} is not set",
                        name,
                        cred.UsernameFromEnv
                    );
                }
                cred.ResolvedPassword = env::var(&cred.PasswordFromEnv).unwrap_or_default();
                if cred.ResolvedPassword.is_empty() {
                    bail!(
                        "credential {:?}: environment variable {} is not set",
                        name,
                        cred.PasswordFromEnv
                    );
                }
            }
            "none" | "" => {}
            other => bail!(
                "credential {:?}: unsupported type {:?} (must be bearer, basic, or none)",
                name,
                other
            ),
        }
        Ok(Some(cred))
    }

    /// 按名称或别名查找数据源，返回 (源名称, 源配置)
    pub fn get_source(&self, alias: &str) -> Option<(String, &SourceConfig)> {
        let alias = match alias {
            "gh" => "github",
            "lp" => "launchpad",
            other => other,
        };
        if let Some(src) = self.Sources.get(alias) {
            return Some((alias.to_string(), src));
        }
        for (name, src) in &self.Sources {
            if src.Alias == alias {
                return Some((name.clone(), src));
            }
            let cleaned = name.strip_prefix("my-").unwrap_or(name);
            if cleaned == alias {
                return Some((name.clone(), src));
            }
        }
        None
    }

    /// 判断来源名或别名是否已存在。
    pub fn has_source(&self, alias: &str) -> bool {
        self.get_source(alias).is_some()
    }

    /// 新增或覆盖来源。
    pub fn add_source(&mut self, name: &str, src: SourceConfig) {
        self.Sources.insert(name.to_string(), src);
    }

    /// 删除来源，成功返回 true。
    pub fn remove_source(&mut self, name: &str) -> bool {
        if self.Sources.remove(name).is_some() {
            return true;
        }
        let key = self
            .Sources
            .iter()
            .find(|(_, s)| s.Alias == name)
            .map(|(k, _)| k.clone());
        if let Some(k) = key {
            self.Sources.remove(&k);
            return true;
        }
        false
    }

    /// 保存配置到文件（带注释 YAML，与 Go writer.go 输出一致）
    pub fn save_to_file(&self, path: &str) -> Result<()> {
        let dir = Path::new(path).parent().unwrap_or(Path::new("."));
        fs::create_dir_all(dir).context("create directory")?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let _ = fs::set_permissions(dir, fs::Permissions::from_mode(0o700));
        }
        let data = marshal_config(self);
        let mut tmp = tempfile::NamedTempFile::new_in(dir).context("create temp config")?;
        {
            use std::io::Write as _;
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                tmp.as_file()
                    .set_permissions(fs::Permissions::from_mode(0o600))
                    .context("set config permissions")?;
            }
            tmp.write_all(data.as_bytes()).context("write config")?;
            tmp.flush().context("flush config")?;
            tmp.as_file().sync_all().context("sync config")?;
        }
        tmp.persist(path).context("atomic rename config")?;
        Ok(())
    }
}

/// 默认配置文件路径 ~/.config/iskey/sources.yaml
pub fn default_config_path() -> String {
    dirs::home_dir()
        .map(|h| {
            h.join(".config/iskey/sources.yaml")
                .to_string_lossy()
                .to_string()
        })
        .unwrap_or_else(|| "~/.config/iskey/sources.yaml".to_string())
}

/// 配置加载器
pub struct Loader {
    config_file: String,
}

impl Loader {
    /// 创建配置加载器。
    pub fn new(config_file: String) -> Self {
        let config_file = if config_file.is_empty() {
            default_config_path()
        } else {
            config_file
        };
        Self { config_file }
    }

    /// 读取并解析配置；环境变量只在字符串节点内展开。
    pub fn load(&self) -> Result<Config> {
        let data = fs::read_to_string(&self.config_file).map_err(|e| {
            if e.kind() == std::io::ErrorKind::NotFound {
                anyhow!("config file not found: {}", self.config_file)
            } else {
                anyhow!("read config file: {}", e)
            }
        })?;
        let mut value: serde_yaml::Value =
            serde_yaml::from_str(&data).map_err(|e| anyhow!("parse config file: {}", e))?;
        expand_env_value(&mut value);
        let mut cfg: Config =
            serde_yaml::from_value(value).map_err(|e| anyhow!("parse config file: {}", e))?;
        set_defaults(&mut cfg);
        cfg.validate()
            .map_err(|e| anyhow!("validate config: {}", e))?;
        Ok(cfg)
    }

    /// 配置文件不存在时返回默认配置
    pub fn load_or_default(&self) -> Result<Config> {
        match self.load() {
            Ok(cfg) => Ok(cfg),
            Err(e) if e.to_string().contains("config file not found") => Ok(Config::default()),
            Err(e) => Err(e),
        }
    }

    /// 判断配置文件是否存在。
    pub fn config_exists(&self) -> bool {
        Path::new(&self.config_file).exists()
    }
}

/// 替换字符串中的环境变量引用，支持 ${VAR}、$VAR、${VAR:-default}
fn expand_env_vars(s: &str) -> String {
    let mut out = String::new();
    let mut rest = s;
    while let Some(start) = rest.find('$') {
        out.push_str(&rest[..start]);
        let after = &rest[start + 1..];
        if let Some(expr) = after.strip_prefix('{')
            && let Some(end) = expr.find('}')
        {
            let key = &expr[..end];
            let (name, default) = match key.split_once(":-") {
                Some((n, d)) => (n, Some(d)),
                None => (key, None),
            };
            let value = env::var(name).ok();
            match value {
                Some(v) if !v.is_empty() => out.push_str(&v),
                _ => {
                    if let Some(d) = default {
                        out.push_str(d);
                    }
                }
            }
            rest = &expr[end + 1..];
            continue;
        }
        let name: String = after
            .chars()
            .take_while(|c| c.is_ascii_alphanumeric() || *c == '_')
            .collect();
        if name.is_empty() {
            out.push('$');
            rest = after;
        } else {
            out.push_str(&env::var(&name).unwrap_or_default());
            rest = &after[name.len()..];
        }
    }
    out.push_str(rest);
    out
}

/// 递归展开 YAML 中的 ${VAR}/$VAR，保持结构不被注入。
fn expand_env_value(value: &mut serde_yaml::Value) {
    match value {
        serde_yaml::Value::String(s) => *s = expand_env_vars(s),
        serde_yaml::Value::Sequence(items) => {
            for item in items {
                expand_env_value(item);
            }
        }
        serde_yaml::Value::Mapping(map) => {
            for (_, v) in map.iter_mut() {
                expand_env_value(v);
            }
        }
        _ => {}
    }
}

/// 填充未设置的默认值与别名。
fn set_defaults(cfg: &mut Config) {
    if cfg.Defaults.Output.is_empty() {
        cfg.Defaults.Output = "~/.ssh/authorized_keys".to_string();
    }
    if cfg.Defaults.Timeout == 0 {
        cfg.Defaults.Timeout = 10;
    }
    if cfg.Defaults.MaxRetries == 0 {
        cfg.Defaults.MaxRetries = 3;
    }
    for (name, src) in &mut cfg.Sources {
        if src.Format.is_empty() {
            src.Format = "plaintext".to_string();
        }
        if src.Alias.is_empty() {
            src.Alias = name.clone();
        }
    }
}

/// 返回稳定排序的键，保证输出可复现。
fn sorted_keys<T>(map: &HashMap<String, T>) -> Vec<&String> {
    let mut keys: Vec<&String> = map.keys().collect();
    keys.sort();
    keys
}

/// 生成带注释的 YAML 配置（对齐 Go marshalConfig）
fn marshal_config(cfg: &Config) -> String {
    let mut b = String::new();
    b.push_str("# iskey 配置文件\n");
    b.push_str("# 默认路径: ~/.config/iskey/sources.yaml\n");
    b.push_str("# 使用 iskey config init 生成此文件\n");
    b.push('\n');

    let has_defaults = !cfg.Defaults.Output.is_empty()
        || cfg.Defaults.Timeout != 0
        || cfg.Defaults.MaxRetries != 0
        || cfg.Defaults.InsecureSkipVerify;
    if has_defaults {
        b.push_str("# 全局默认配置\n");
        b.push_str("defaults:\n");
        if !cfg.Defaults.Output.is_empty() {
            let _ = writeln!(b, "  output: \"{}\"", cfg.Defaults.Output);
        }
        if cfg.Defaults.Timeout != 0 {
            let _ = writeln!(b, "  timeout: {}", cfg.Defaults.Timeout);
        }
        if cfg.Defaults.MaxRetries != 0 {
            let _ = writeln!(b, "  max_retries: {}", cfg.Defaults.MaxRetries);
        }
        if cfg.Defaults.InsecureSkipVerify {
            b.push_str("  insecure_skip_verify: true\n");
        }
    } else {
        b.push_str("# 全局默认配置（选填，系统已有默认值）\n");
        b.push_str("# defaults:\n");
        b.push_str("#   output: \"~/.ssh/authorized_keys\"  # 默认值\n");
        b.push_str("#   timeout: 10                        # 默认值\n");
        b.push_str("#   max_retries: 3                     # 默认值\n");
    }
    b.push('\n');

    if !cfg.Credentials.is_empty() {
        b.push_str("# 认证凭据定义\n");
        b.push_str("# 重要: Token 值必须从环境变量读取，YAML 中严禁硬编码\n");
        b.push_str("credentials:\n");
        for name in sorted_keys(&cfg.Credentials) {
            let cred = &cfg.Credentials[name];
            let _ = writeln!(b, "  {}:", name);
            let _ = writeln!(b, "    type: {}", cred.Type);
            if !cred.ValueFromEnv.is_empty() {
                let _ = writeln!(b, "    value_from_env: {}", cred.ValueFromEnv);
            }
            if !cred.UsernameFromEnv.is_empty() {
                let _ = writeln!(b, "    username_from_env: {}", cred.UsernameFromEnv);
            }
            if !cred.PasswordFromEnv.is_empty() {
                let _ = writeln!(b, "    password_from_env: {}", cred.PasswordFromEnv);
            }
        }
    } else {
        b.push_str("# 认证凭据定义（选填，用于私有源）\n");
        b.push_str("# 重要: Token 值必须从环境变量读取，YAML 中严禁硬编码\n");
        b.push_str("# credentials:\n");
        b.push_str("#   my-gitlab-token:\n");
        b.push_str("#     type: bearer\n");
        b.push_str("#     value_from_env: GITLAB_PRIVATE_TOKEN\n");
    }
    b.push('\n');

    if !cfg.Sources.is_empty() {
        b.push_str("# 数据源定义\n");
        b.push_str("# 内置源 gh/lp 始终可用，无需配置\n");
        b.push_str("# 使用方式: iskey add <alias>:<username>\n");
        b.push_str("sources:\n");
        for name in sorted_keys(&cfg.Sources) {
            let src = &cfg.Sources[name];
            let _ = writeln!(b, "  {}:", name);
            let _ = writeln!(b, "    alias: {}", src.Alias);
            let _ = writeln!(b, "    url: \"{}\"", src.URL);
            let _ = writeln!(b, "    format: {}", src.Format);
            if !src.AuthRef.is_empty() {
                let _ = writeln!(b, "    auth_ref: {}", src.AuthRef);
            }
            if !src.DefaultVars.is_empty() {
                b.push_str("    default_vars:\n");
                for k in sorted_keys(&src.DefaultVars) {
                    let _ = writeln!(b, "      {}: {}", k, src.DefaultVars[k]);
                }
            }
        }
    } else {
        b.push_str("# 数据源定义（选填）\n");
        b.push_str("# 内置源 gh/lp 始终可用，无需配置\n");
        b.push_str("# 使用方式: iskey add <alias>:<username>\n");
        b.push_str("sources: {}\n");
    }
    b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_expand_env_vars() {
        unsafe {
            env::set_var("ISKEY_TEST_VAR", "hello");
        }
        assert_eq!(expand_env_vars("${ISKEY_TEST_VAR}"), "hello");
        assert_eq!(expand_env_vars("$ISKEY_TEST_VAR world"), "hello world");
        assert_eq!(expand_env_vars("${ISKEY_UNSET:-fallback}"), "fallback");
        assert_eq!(expand_env_vars("${ISKEY_TEST_VAR:-fallback}"), "hello");
        assert_eq!(expand_env_vars("$ISKEY_UNSET"), "");
    }

    #[test]
    fn test_parse_config() {
        let yaml = r#"
version: v1
defaults:
  output: "~/.ssh/authorized_keys"
  timeout: 5
sources:
  github:
    alias: gh
    url: "https://api.github.com/users/{{ .User }}/keys"
    format: github_json
"#;
        let cfg: Config = serde_yaml::from_str(yaml).unwrap();
        assert_eq!(cfg.Defaults.Timeout, 5);
        assert_eq!(cfg.Sources["github"].Alias, "gh");
        assert_eq!(
            cfg.Sources["github"].URL,
            "https://api.github.com/users/{{ .User }}/keys"
        );
        assert_eq!(cfg.Sources["github"].Format, "github_json");
    }

    #[test]
    fn test_get_source_by_alias_and_my_prefix() {
        let mut cfg = Config::default();
        cfg.add_source(
            "my-work",
            SourceConfig {
                Alias: "work".to_string(),
                URL: "https://x/{{ .User }}".to_string(),
                ..Default::default()
            },
        );
        assert!(cfg.get_source("work").is_some());
        assert!(cfg.get_source("my-work").is_some());
        assert!(cfg.get_source("nope").is_none());
    }

    #[test]
    fn test_credentials_resolve() {
        unsafe {
            env::set_var("ISKEY_TEST_TOKEN", "secret");
        }
        let mut cfg = Config::default();
        cfg.Credentials.insert(
            "t".to_string(),
            Credential {
                Type: "bearer".to_string(),
                ValueFromEnv: "ISKEY_TEST_TOKEN".to_string(),
                ..Default::default()
            },
        );
        let resolved = cfg.resolved_credential("t").unwrap().unwrap();
        assert_eq!(resolved.ResolvedValue, "secret");

        // 缺失环境变量报错
        let mut cfg2 = Config::default();
        cfg2.Credentials.insert(
            "t".to_string(),
            Credential {
                Type: "bearer".to_string(),
                ValueFromEnv: "ISKEY_NOT_SET_VAR".to_string(),
                ..Default::default()
            },
        );
        assert!(cfg2.resolved_credential("t").is_err());
    }

    #[test]
    fn test_marshal_config_roundtrip() {
        let cfg = Config::default();
        let out = marshal_config(&cfg);
        assert!(out.contains("sources: {}"));
        assert!(out.starts_with("# iskey 配置文件\n"));
        // 生成的 YAML 可被重新解析
        let parsed: Config = serde_yaml::from_str(&out).unwrap();
        assert!(parsed.Sources.is_empty());
    }
}
