//! 命令行参数定义与各子命令的执行逻辑。

use std::collections::HashMap;
use std::path::Path;

use anyhow::{Result, bail};
use clap::{Args, Parser, Subcommand};

use crate::config::{self, Config, Loader, SourceConfig};
use crate::domain;
use crate::fetcher;
use crate::manager::Manager;
use crate::output;
use crate::service;

#[derive(Parser)]
#[command(
    name = "iskey",
    about = "SSH 公钥聚合管理工具",
    long_about = "iskey 是 ssh-import-id 的 Rust 重构版，支持多数据源的 SSH 公钥管理。\n\n支持的数据源：\n  gh:, github:     GitHub 用户公钥\n  lp:, launchpad:  Launchpad 用户公钥\n  自定义 URL:       任何返回纯文本或 GitHub JSON 格式的端点\n\n示例：\n  iskey add gh:octocat\n  iskey add work:zhangsan\n  iskey remove gh:octocat\n  iskey list --show-fingerprint",
    disable_version_flag = true
)]
pub struct Cli {
    /// 配置文件路径
    #[arg(short = 'c', long, global = true)]
    pub config: Option<String>,
    /// 目标 authorized_keys 文件路径
    #[arg(short = 'o', long, global = true)]
    pub output: Option<String>,
    /// 模拟运行，不写入文件
    #[arg(long, global = true)]
    pub dry_run: bool,
    /// 静默模式
    #[arg(short, long, global = true)]
    pub quiet: bool,
    /// JSON 格式输出
    #[arg(long, global = true)]
    pub json: bool,
    #[command(subcommand)]
    pub command: Command,
}

#[derive(Subcommand)]
pub enum Command {
    /// 添加用户的 SSH 公钥
    Add(AddArgs),
    /// 移除用户的 SSH 公钥
    Remove(RemoveArgs),
    /// 列出当前管理的公钥
    List(ListArgs),
    /// 管理配置文件
    Config(ConfigArgs),
    /// 显示版本信息
    Version,
}

#[derive(Args)]
pub struct AddArgs {
    /// TARGET 格式: SOURCE_ALIAS:USERNAME 或 URL
    pub target: String,
    /// URL 目标对应的用户名
    #[arg(long)]
    pub user: Option<String>,
    /// 覆盖解析格式: plaintext/github_json/auto
    #[arg(long)]
    pub format: Option<String>,
    /// 模板变量，格式 Key=Value
    #[arg(long, action = clap::ArgAction::Append)]
    pub var: Vec<String>,
    /// 用本次拉取结果替换同一 source:user 的已有公钥
    #[arg(long)]
    pub force: bool,
}

#[derive(Args)]
pub struct RemoveArgs {
    /// TARGET 格式: [SOURCE_ALIAS|URL][:USERNAME]
    pub target: Option<String>,
    /// 按 SHA256 指纹删除
    #[arg(long)]
    pub fingerprint: Option<String>,
    /// 删除指定源下所有用户
    #[arg(long = "all-from-source")]
    pub all_from_source: Option<String>,
}

#[derive(Args)]
pub struct ListArgs {
    /// 筛选特定源
    #[arg(long)]
    pub source: Option<String>,
    /// 显示完整 SHA256 指纹
    #[arg(long = "show-fingerprint")]
    pub show_fingerprint: bool,
}

#[derive(Args)]
pub struct ConfigArgs {
    #[command(subcommand)]
    pub sub: ConfigSub,
}

#[derive(Subcommand)]
pub enum ConfigSub {
    /// 生成默认配置文件
    Init {
        /// 输出路径（默认 ~/.config/iskey/sources.yaml）
        #[arg(short, long)]
        output: Option<String>,
        /// 强制覆盖已存在的文件
        #[arg(short, long)]
        force: bool,
    },
    /// 列出所有数据源
    List,
    /// 快速添加数据源
    Set { alias: String, url: String },
    /// 移除数据源
    Unset { alias: String },
    /// 校验配置文件语法和 URL 可达性
    Verify,
}

struct App {
    /// 已加载的配置
    cfg: Config,
    /// authorized_keys 管理器
    mgr: Manager,
}

/// 执行命令入口
pub fn run(cli: Cli) -> Result<()> {
    let Cli {
        config,
        output,
        dry_run,
        quiet,
        json,
        command,
    } = cli;
    let config_path = expand_path(config.as_deref().unwrap_or(&config::default_config_path()));

    let is_config_verify = matches!(
        &command,
        Command::Config(ConfigArgs {
            sub: ConfigSub::Verify
        })
    );

    if is_config_verify {
        return cmd_config_verify(&config_path);
    }

    // 预加载配置与管理器（对齐 Go PersistentPreRunE）
    let loader = Loader::new(config_path.clone());
    let cfg = loader.load_or_default()?;
    let output_path = expand_path(output.as_deref().unwrap_or(cfg.Defaults.Output.as_str()));
    let mgr = Manager::new(&output_path);
    if !quiet {
        let (warnings, _) = mgr.check_permissions();
        for w in warnings {
            eprintln!("{}", w);
        }
    }
    let app = App { cfg, mgr };

    match command {
        Command::Add(args) => cmd_add(&app, args, dry_run, quiet, json),
        Command::Remove(args) => cmd_remove(&app, args, dry_run, quiet, json),
        Command::List(args) => cmd_list(&app, args, quiet, json),
        Command::Config(args) => cmd_config(&app, &config_path, args, json),
        Command::Version => {
            println!("iskey {}", env!("CARGO_PKG_VERSION"));
            Ok(())
        }
    }
}

/// 执行 add 子命令：解析目标与变量，调用服务层并输出结果。
fn cmd_add(app: &App, args: AddArgs, dry_run: bool, quiet: bool, json: bool) -> Result<()> {
    let (source, user) = parse_target(&args.target, args.user.as_deref())?;
    let vars = parse_vars(&args.var)?;
    let result = service::add(
        &app.cfg,
        &app.mgr,
        service::AddOptions {
            source: source.clone(),
            user: user.clone(),
            vars,
            format: args.format.unwrap_or_default(),
            force: args.force,
            dry_run,
        },
    )
    .map_err(|e| anyhow::anyhow!("add failed: {}", e))?;

    if json {
        output::print_result_json(&result);
        return Ok(());
    }
    if quiet {
        return Ok(());
    }
    println!("Status:   {}", result.status);
    println!("Action:   {}", result.action);
    println!("Source:   {}", result.source);
    println!("User:     {}", result.user);
    println!("Imported: {}", result.imported);
    println!("Skipped:  {}", result.skipped);
    for key in &result.keys {
        println!("  {}  {}  line {}", key.fingerprint, key.action, key.line);
    }
    Ok(())
}

/// 执行 remove 子命令：校验删除方式互斥后调用服务层。
fn cmd_remove(app: &App, args: RemoveArgs, dry_run: bool, quiet: bool, json: bool) -> Result<()> {
    let selected = [
        args.fingerprint.is_some(),
        args.all_from_source.is_some(),
        args.target.is_some(),
    ]
    .iter()
    .filter(|v| **v)
    .count();
    if selected != 1 {
        bail!("specify exactly one of TARGET, --fingerprint, or --all-from-source");
    }
    if args.fingerprint.is_some() && args.all_from_source.is_some() {
        bail!("--fingerprint and --all-from-source are mutually exclusive");
    }

    let (source, user, fingerprint, all_from_source) = if let Some(fp) = args.fingerprint {
        (String::new(), String::new(), fp, false)
    } else if let Some(src) = args.all_from_source {
        (src, String::new(), String::new(), true)
    } else {
        let target = args.target.as_deref().unwrap_or_default();
        let (source, user) = parse_target(target, None)?;
        (source, user, String::new(), false)
    };

    let result = service::remove(
        &app.cfg,
        &app.mgr,
        service::RemoveOptions {
            source,
            user,
            fingerprint,
            all_from_source,
            dry_run,
        },
    )
    .map_err(|e| anyhow::anyhow!("remove failed: {}", e))?;

    if quiet {
        return Ok(());
    }
    if json {
        output::print_result_json(&result);
        return Ok(());
    }
    println!("Status:   {}", result.status);
    println!("Action:   {}", result.action);
    if !result.source.is_empty() {
        println!("Source:   {}", result.source);
    }
    if !result.user.is_empty() {
        println!("User:     {}", result.user);
    }
    if !result.fingerprint.is_empty() {
        println!("Key:      {}", result.fingerprint);
    }
    println!("Removed:  {} key(s)", result.removed);
    for key in &result.keys {
        println!("  {}  {}", key.fingerprint, key.action);
    }
    Ok(())
}

/// 执行 list 子命令，支持 JSON/静默输出。
fn cmd_list(app: &App, args: ListArgs, quiet: bool, json: bool) -> Result<()> {
    let source = args.source.as_deref().unwrap_or("");
    let result = service::list(&app.cfg, &app.mgr, source)
        .map_err(|e| anyhow::anyhow!("list failed: {}", e))?;

    if json {
        output::print_result_json(&result);
        return Ok(());
    }
    if quiet {
        return Ok(());
    }

    if result.total == 0 {
        println!("No keys managed by iskey.");
        return Ok(());
    }

    let header = if args.show_fingerprint {
        vec![
            vec![
                "SOURCE".to_string(),
                "USER".to_string(),
                "FINGERPRINT".to_string(),
            ],
            vec![
                "------".to_string(),
                "----".to_string(),
                "-----------".to_string(),
            ],
        ]
    } else {
        vec![
            vec!["SOURCE".to_string(), "USER".to_string()],
            vec!["------".to_string(), "----".to_string()],
        ]
    };
    let mut rows = header;
    for e in &result.entries {
        if args.show_fingerprint {
            rows.push(vec![
                e.source.clone(),
                e.user.clone(),
                e.fingerprint.clone(),
            ]);
        } else {
            rows.push(vec![e.source.clone(), e.user.clone()]);
        }
    }
    print!("{}", output::tabwriter(&rows, 2)?);
    println!("\nTotal: {}", result.total);
    Ok(())
}

/// 分发 config 子命令。
fn cmd_config(app: &App, config_path: &str, args: ConfigArgs, json: bool) -> Result<()> {
    match args.sub {
        ConfigSub::Init { output, force } => {
            let out = match output {
                Some(p) => expand_path(&p),
                None => config_path.to_string(),
            };
            if Path::new(&out).exists() && !force {
                bail!(
                    "config file already exists: {} (use --force to overwrite)",
                    out
                );
            }
            Config::default().save_to_file(&out)?;
            println!("Config file created: {}", out);
            Ok(())
        }
        ConfigSub::List => cmd_config_list(app, json),
        ConfigSub::Set { alias, url } => cmd_config_set(config_path, &alias, &url),
        ConfigSub::Unset { alias } => cmd_config_unset(config_path, &alias),
        ConfigSub::Verify => unreachable!("handled before preflight"),
    }
}

/// 打印当前配置中的来源列表。
fn cmd_config_list(app: &App, json: bool) -> Result<()> {
    if json {
        let builtin = builtin_sources();
        let result = serde_json::json!({ "builtin": builtin, "custom": app.cfg.Sources });
        println!("{}", serde_json::to_string_pretty(&result)?);
        return Ok(());
    }

    println!("Builtin sources (always available):");
    println!("  gh  - github    - https://api.github.com/users/{{user}}/keys");
    println!("  lp  - launchpad - https://launchpad.net/~{{user}}/+sshkeys");
    println!();

    if app.cfg.Sources.is_empty() {
        println!("Custom sources: none");
        println!();
        println!("Add a custom source:");
        println!("  iskey config set <alias> <url>");
    } else {
        println!("Custom sources:");
        for (name, src) in &app.cfg.Sources {
            println!("  {} ({})", name, src.Alias);
            println!("    URL: {}", src.URL);
            println!("    Format: {}", src.Format);
        }
    }
    Ok(())
}

/// 写入或更新一个来源；未包含模板变量时自动补全 /keys 路径。
fn cmd_config_set(config_path: &str, alias: &str, url: &str) -> Result<()> {
    if fetcher::is_builtin_source(alias) {
        bail!("cannot override builtin source: {}", alias);
    }

    fetcher::validate_url(url)?;
    // 无模板变量的 URL 自动追加 /keys/{{ .User }}
    let url = if contains_template_var(url) {
        url.trim_end_matches('/').to_string()
    } else {
        let base = url.trim_end_matches('/');
        if base.ends_with("/keys") {
            format!("{base}/{{{{ .User }}}}")
        } else {
            format!("{base}/keys/{{{{ .User }}}}")
        }
    };

    let src = SourceConfig {
        Alias: alias.to_string(),
        URL: url.clone(),
        Format: "plaintext".to_string(),
        ..Default::default()
    };

    let loader = Loader::new(config_path.to_string());
    let mut loaded = loader.load_or_default()?;
    if loaded.has_source(alias) {
        bail!(
            "source {:?} already exists (use 'iskey config unset {}' to remove first)",
            alias,
            alias
        );
    }
    loaded.add_source(alias, src);
    loaded.save_to_file(config_path)?;

    println!("Source {:?} added to {}", alias, config_path);
    println!();
    println!("  Alias: {}", alias);
    println!("  URL: {}", url);
    println!("  Format: plaintext");
    Ok(())
}

/// 删除指定来源或别名。
fn cmd_config_unset(config_path: &str, alias: &str) -> Result<()> {
    if fetcher::is_builtin_source(alias) {
        bail!("cannot remove builtin source: {}", alias);
    }
    let loader = Loader::new(config_path.to_string());
    let mut loaded = loader.load()?;
    if !loaded.remove_source(alias) {
        bail!("source {:?} not found", alias);
    }
    loaded.save_to_file(config_path)?;
    println!("Source {:?} removed from {}", alias, config_path);
    Ok(())
}

/// 校验配置语法、URL 与所有被引用凭据的环境变量。
fn cmd_config_verify(config_path: &str) -> Result<()> {
    let loader = Loader::new(config_path.to_string());
    if !loader.config_exists() {
        eprintln!("Config file not found: {}", config_path);
        bail!("config file not found");
    }

    let cfg = match loader.load() {
        Ok(c) => c,
        Err(e) => {
            eprintln!("Config validation failed: {}", e);
            return Err(e);
        }
    };

    let mut failed = false;
    println!("Config file is valid.");
    println!();
    println!("Builtin sources:");
    println!("  gh (github) - builtin");
    println!("  lp (launchpad) - builtin");

    if !cfg.Sources.is_empty() {
        println!();
        println!("Custom sources:");
        let mut names: Vec<&String> = cfg.Sources.keys().collect();
        names.sort();
        for name in names {
            let src = &cfg.Sources[name];
            match cfg.resolved_credential(&src.AuthRef) {
                Ok(_) => {
                    if !src.URL.contains("{{") {
                        if let Err(e) = fetcher::validate_url(&src.URL) {
                            println!("  {} ({}) - FAILED: {}", name, src.Alias, e);
                            failed = true;
                        } else {
                            println!("  {} ({}) - ok", name, src.Alias);
                        }
                    } else {
                        println!("  {} ({}) - ok", name, src.Alias);
                    }
                }
                Err(e) => {
                    println!("  {} ({}) - FAILED: {}", name, src.Alias, e);
                    failed = true;
                }
            }
        }
    }

    if !cfg.Credentials.is_empty() {
        println!();
        println!("Credentials:");
        let mut names: Vec<&String> = cfg.Credentials.keys().collect();
        names.sort();
        for name in names {
            let cred = &cfg.Credentials[name];
            match cfg.resolved_credential(name) {
                Ok(_) => println!("  {} - {}", name, cred.Type),
                Err(e) => {
                    println!("  {} - {} FAILED: {}", name, cred.Type, e);
                    failed = true;
                }
            }
        }
    }
    if failed {
        bail!("config verification failed");
    }
    Ok(())
}

/// 返回内置 GitHub/Launchpad 源定义。
fn builtin_sources() -> HashMap<String, SourceConfig> {
    let mut m = HashMap::new();
    m.insert(
        "github".to_string(),
        SourceConfig {
            Alias: "gh".to_string(),
            URL: "https://api.github.com/users/{{ .User }}/keys".to_string(),
            Format: "github_json".to_string(),
            ..Default::default()
        },
    );
    m.insert(
        "launchpad".to_string(),
        SourceConfig {
            Alias: "lp".to_string(),
            URL: "https://launchpad.net/~{{ .User }}/+sshkeys".to_string(),
            Format: "plaintext".to_string(),
            ..Default::default()
        },
    );
    m
}

/// 解析 TARGET 参数为 (SOURCE, USER)。URL target 必须配合 --user。
fn parse_target(target: &str, url_user: Option<&str>) -> Result<(String, String)> {
    if target.starts_with("http://") || target.starts_with("https://") {
        let user = url_user.unwrap_or("").trim();
        if user.is_empty() {
            bail!("URL format requires --user: {}", target);
        }
        domain::validate_user(user)?;
        return Ok((target.to_string(), user.to_string()));
    }
    let mut parts = target.splitn(2, ':');
    let source = parts.next().unwrap_or_default().trim();
    let user = parts.next().unwrap_or_default().trim();
    if source.is_empty() || user.is_empty() {
        bail!("invalid TARGET format: {} (expected SOURCE:USER)", target);
    }
    domain::validate_target(source, user)?;
    Ok((source.to_string(), user.to_string()))
}

/// 解析模板变量列表 ["Key=Value", ...]
fn parse_vars(vars: &[String]) -> Result<HashMap<String, String>> {
    let mut m = HashMap::new();
    for v in vars {
        let Some((k, val)) = v.split_once('=') else {
            bail!("invalid --var {:?} (expected Key=Value)", v);
        };
        let key = k.trim();
        if key.is_empty() {
            bail!("invalid --var {:?}: key is empty", v);
        }
        if key == "User" || key == "Source" {
            bail!("--var {key} is reserved");
        }
        m.insert(key.to_string(), val.to_string());
    }
    Ok(m)
}

/// 展开路径中的 ~ 为用户主目录
pub fn expand_path(path: &str) -> String {
    if let Some(rest) = path.strip_prefix('~')
        && let Some(home) = dirs::home_dir()
    {
        return home
            .join(rest.trim_start_matches('/'))
            .to_string_lossy()
            .to_string();
    }
    path.to_string()
}

/// 判断字符串中是否含有 {{ ... }} 模板变量。
fn contains_template_var(s: &str) -> bool {
    s.contains("{{ .")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_target() {
        assert_eq!(
            parse_target("gh:octocat", None).unwrap(),
            ("gh".to_string(), "octocat".to_string())
        );
        assert!(parse_target("work:zhangsan:extra", None).is_err());
        assert!(parse_target("gh", None).is_err());
        assert!(parse_target("gh:", None).is_err());
        assert!(parse_target("https://example.com/keys.pub", None).is_err());
        assert_eq!(
            parse_target("https://example.com/keys.pub", Some("alice")).unwrap(),
            (
                "https://example.com/keys.pub".to_string(),
                "alice".to_string()
            )
        );
    }

    #[test]
    fn test_parse_vars() {
        let v = parse_vars(&["Team=infra".to_string(), "Env=prod".to_string()]).unwrap();
        assert_eq!(v.len(), 2);
        assert_eq!(v["Team"], "infra");
        assert_eq!(v["Env"], "prod");
        assert!(parse_vars(&["bad".to_string()]).is_err());
        assert!(parse_vars(&["=x".to_string()]).is_err());
        assert!(parse_vars(&["User=bob".to_string()]).is_err());
    }

    #[test]
    fn test_expand_path() {
        assert_eq!(expand_path("/abs/path"), "/abs/path");
        assert!(expand_path("~/x").starts_with('/'));
        assert!(expand_path("~/x").ends_with("/x"));
    }

    #[test]
    fn test_tabwriter() {
        let rows = vec![
            vec![
                "SOURCE".to_string(),
                "USER".to_string(),
                "FINGERPRINT".to_string(),
            ],
            vec![
                "------".to_string(),
                "----".to_string(),
                "-----------".to_string(),
            ],
            vec!["gh".to_string(), "octocat".to_string(), "x".to_string()],
        ];
        let out = output::tabwriter(&rows, 2).unwrap();
        assert_eq!(
            out,
            "SOURCE  USER     FINGERPRINT\n------  ----     -----------\ngh      octocat  x\n"
        );
    }
}
