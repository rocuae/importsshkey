//! 业务服务层：编排配置解析、抓取与本地文件写入。

use std::collections::HashMap;

use anyhow::{Result, bail};

use crate::config::{Config, Credential, SourceConfig};
use crate::domain::{self, KeyEntry};
use crate::fetcher;
use crate::manager::Manager;

/// 单把公钥的变更结果
#[allow(non_snake_case)]
#[derive(Debug, serde::Serialize)]
pub struct KeyResult {
    /// 公钥 SHA256 指纹
    #[serde(rename = "Fingerprint")]
    pub fingerprint: String,
    /// added/replaced/skipped，dry-run 时为 would-* 前缀
    #[serde(rename = "Action")]
    pub action: String,
    /// 公钥在最终文件中的 1-based 行号
    #[serde(rename = "Line")]
    pub line: usize,
}

/// 添加操作结果，字段名与 Go 版 JSON 输出一致
#[allow(non_snake_case)]
#[derive(Debug, serde::Serialize)]
pub struct AddResult {
    /// success 或错误状态
    #[serde(rename = "Status")]
    pub status: String,
    /// 整体动作：added/replaced/dry-run/skipped
    #[serde(rename = "Action")]
    pub action: String,
    /// 解析后的规范源名
    #[serde(rename = "Source")]
    pub source: String,
    /// 目标用户名
    #[serde(rename = "User")]
    pub user: String,
    /// 实际新增或替换的数量
    #[serde(rename = "Imported")]
    pub imported: usize,
    /// 跳过的数量
    #[serde(rename = "Skipped")]
    pub skipped: usize,
    /// 每把公钥的处理明细
    #[serde(rename = "Keys")]
    pub keys: Vec<KeyResult>,
    /// 保留错误字段，成功时为 None
    #[serde(rename = "Error")]
    pub error: Option<String>,
}

/// 移除操作结果
#[allow(non_snake_case)]
#[derive(Debug, serde::Serialize)]
pub struct RemoveResult {
    /// success 或 not_found
    #[serde(rename = "Status")]
    pub status: String,
    /// removed/dry-run/not_found
    #[serde(rename = "Action")]
    pub action: String,
    /// 解析后的源名
    #[serde(rename = "Source")]
    pub source: String,
    /// 目标用户名
    #[serde(rename = "User")]
    pub user: String,
    /// 按指纹删除时回显的指纹
    #[serde(rename = "Fingerprint")]
    pub fingerprint: String,
    /// 实际或预计删除数量
    #[serde(rename = "Removed")]
    pub removed: usize,
    /// 被删除条目的明细
    #[serde(rename = "Keys")]
    pub keys: Vec<KeyResult>,
    /// 保留错误字段，成功时为 None
    #[serde(rename = "Error")]
    pub error: Option<String>,
}

/// 列表操作结果
#[allow(non_snake_case)]
#[derive(Debug, serde::Serialize)]
pub struct ListResult {
    /// 匹配的托管条目
    #[serde(rename = "Entries")]
    pub entries: Vec<KeyEntry>,
    /// 条目总数
    #[serde(rename = "Total")]
    pub total: usize,
}

/// 添加命令的输入参数
pub struct AddOptions {
    /// 源别名、源名或完整 URL
    pub source: String,
    /// 目标用户名
    pub user: String,
    /// 命令行模板变量，优先级高于 default_vars
    pub vars: HashMap<String, String>,
    /// 覆盖源配置的解析格式
    pub format: String,
    /// 用本次结果替换同一 source:user 的旧公钥
    pub force: bool,
    /// 只计算变更，不写文件
    pub dry_run: bool,
}

/// 移除命令的输入参数
pub struct RemoveOptions {
    /// 源别名或规范源名
    pub source: String,
    /// 目标用户名
    pub user: String,
    /// 按指纹删除时的指纹
    pub fingerprint: String,
    /// 删除指定源下的全部条目
    pub all_from_source: bool,
    /// 只计算变更，不写文件
    pub dry_run: bool,
}

/// 执行添加操作；dry_run 时只计算变更，不写文件。
pub fn add(cfg: &Config, manager: &Manager, opts: AddOptions) -> Result<AddResult> {
    domain::validate_user(&opts.user)?;

    let is_url = opts.source.starts_with("http://") || opts.source.starts_with("https://");
    let (source_name, src_cfg, creds): (String, SourceConfig, Option<Credential>) = if is_url {
        let format = if opts.format.is_empty() {
            "auto".to_string()
        } else {
            opts.format.clone()
        };
        (
            "url".to_string(),
            SourceConfig {
                Alias: "url".to_string(),
                URL: opts.source.clone(),
                Format: format,
                ..Default::default()
            },
            None,
        )
    } else {
        let (name, mut src) = fetcher::resolve_alias(cfg, &opts.source)?;
        if !opts.format.is_empty() {
            src.Format = opts.format.clone();
        }
        let creds = cfg.resolved_credential(&src.AuthRef)?;
        (name, src, creds)
    };

    match src_cfg.Format.as_str() {
        "" | "plaintext" | "github_json" | "auto" => {}
        other => bail!(
            "invalid format {:?} (must be plaintext, github_json, or auto)",
            other
        ),
    }

    let f = fetcher::factory(
        &src_cfg,
        &source_name,
        creds.as_ref(),
        &opts.user,
        &opts.vars,
        cfg.Defaults.timeout(),
        cfg.Defaults.InsecureSkipVerify,
        cfg.Defaults.MaxRetries,
    )?;
    let entries = f.fetch()?;
    if entries.is_empty() {
        bail!(
            "no valid public keys found for {}:{}",
            source_name,
            opts.user
        );
    }

    let outcomes = manager.add_many(&entries, opts.force, opts.dry_run)?;
    let mut result = AddResult {
        status: "success".to_string(),
        action: if opts.dry_run {
            "dry-run".to_string()
        } else {
            "added".to_string()
        },
        source: source_name,
        user: opts.user,
        imported: 0,
        skipped: 0,
        keys: Vec::with_capacity(outcomes.len()),
        error: None,
    };
    for outcome in outcomes {
        if outcome.action.contains("skip") {
            result.skipped += 1;
        } else {
            result.imported += 1;
        }
        result.keys.push(KeyResult {
            fingerprint: outcome.entry.fingerprint,
            action: outcome.action,
            line: outcome.line,
        });
    }
    if result.imported == 0 {
        result.action = "skipped".to_string();
    }
    Ok(result)
}

/// 执行移除操作。
pub fn remove(cfg: &Config, manager: &Manager, opts: RemoveOptions) -> Result<RemoveResult> {
    let mut source_name = opts.source.clone();
    if !source_name.is_empty()
        && !source_name.starts_with("http://")
        && !source_name.starts_with("https://")
    {
        match fetcher::resolve_alias(cfg, &source_name) {
            Ok((name, _)) => source_name = name,
            Err(e) => {
                if domain::validate_source(&source_name).is_err() {
                    return Err(e);
                }
            }
        }
    }

    let removed = if opts.all_from_source {
        manager.remove_all_from_source(&source_name, opts.dry_run)?
    } else {
        if opts.fingerprint.is_empty() && (source_name.is_empty() || opts.user.is_empty()) {
            bail!("source and user are required when --fingerprint is not set");
        }
        manager.remove_by_target(&source_name, &opts.user, &opts.fingerprint, opts.dry_run)?
    };

    let mut result = RemoveResult {
        status: "success".to_string(),
        action: if opts.dry_run {
            "dry-run".to_string()
        } else {
            "removed".to_string()
        },
        source: source_name,
        user: opts.user,
        fingerprint: opts.fingerprint,
        removed: removed.len(),
        keys: Vec::with_capacity(removed.len()),
        error: None,
    };
    for entry in removed {
        result.keys.push(KeyResult {
            fingerprint: entry.fingerprint,
            action: if opts.dry_run {
                "would-remove".to_string()
            } else {
                "removed".to_string()
            },
            line: 0,
        });
    }
    if result.removed == 0 {
        result.status = "not_found".to_string();
        result.action = "not_found".to_string();
    }
    Ok(result)
}

/// 执行列表查询
pub fn list(cfg: &Config, manager: &Manager, source_filter: &str) -> Result<ListResult> {
    let mut filter = source_filter.to_string();
    if !filter.is_empty() {
        match fetcher::resolve_alias(cfg, &filter) {
            Ok((name, _)) => filter = name,
            Err(e) => {
                if domain::validate_source(&filter).is_err() {
                    return Err(e);
                }
            }
        }
    }
    let entries = manager.list(&filter)?;
    Ok(ListResult {
        total: entries.len(),
        entries,
    })
}
