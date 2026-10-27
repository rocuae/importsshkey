//! CLI 端到端测试：启动本地 HTTP 服务，覆盖 add/list/remove 与配置校验。

use std::fs;
use std::process::Command;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use tiny_http::{Response, Server};

const KEY1: &str = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO95iT5q2IE3X24g2Pfq0v8s9erHGOo2VYrl/nvAVV95 test@example.com";
const KEY2: &str = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIP5KPytIk1TuhtOv9L8vuU/kBy+Em39+YwPI0jU7F9HL b@example.com";

fn bin() -> &'static str {
    env!("CARGO_BIN_EXE_iskey")
}

/// 启动一个本地 HTTP 服务器，返回端口号
fn start_server() -> u16 {
    let server = Server::http("127.0.0.1:0").unwrap();
    let port = server.server_addr().to_ip().unwrap().port();
    let running = Arc::new(AtomicBool::new(true));
    let keep = running.clone();
    std::thread::spawn(move || {
        while keep.load(Ordering::Relaxed) {
            if let Ok(Some(request)) = server.recv_timeout(std::time::Duration::from_millis(100)) {
                let path = request.url().to_string();
                let body = match path.as_str() {
                    "/octocat" => KEY1.to_string(),
                    "/zhangsan" => KEY2.to_string(),
                    _ => {
                        let _ = request.respond(Response::empty(404));
                        continue;
                    }
                };
                let _ = request.respond(Response::from_string(body));
            }
        }
    });
    port
}

fn run(args: &[&str]) -> std::process::Output {
    Command::new(bin()).args(args).output().unwrap()
}

fn write_config(path: &std::path::Path, port: u16) {
    let cfg = format!(
        "version: v1\nsources:\n  my-server:\n    alias: work\n    url: \"http://127.0.0.1:{port}/{{{{ .User }}}}\"\n    format: plaintext\n"
    );
    fs::write(path, cfg).unwrap();
}

#[test]
fn add_list_remove_flow() {
    let port = start_server();
    let dir = tempfile::tempdir().unwrap();
    let cfg = dir.path().join("sources.yaml");
    let out = dir.path().join("authorized_keys");
    write_config(&cfg, port);
    let cfg = cfg.to_str().unwrap();
    let out = out.to_str().unwrap();

    // add
    let o = run(&["add", "work:octocat", "--config", cfg, "-o", out, "-q"]);
    assert!(o.status.success());
    let content = fs::read_to_string(out).unwrap();
    assert!(
        content.contains(
            "# iskey:my-server:octocat:SHA256:7MaIW0hEEsFKO7AbaY7AIztsc1Zzyf7jwHoWuUu54Rs="
        )
    );
    assert!(content.contains("ssh-ed25519"));

    // add second
    run(&["add", "work:zhangsan", "--config", cfg, "-o", out, "-q"]);

    // duplicate add skipped
    let o = run(&["add", "work:octocat", "--config", cfg, "-o", out]);
    let stdout = String::from_utf8_lossy(&o.stdout);
    assert!(stdout.contains("Action:   skipped"));

    // list --json 形状断言
    let o = run(&["list", "--json", "--config", cfg, "-o", out, "-q"]);
    assert!(o.status.success());
    let stdout = String::from_utf8_lossy(&o.stdout);
    let v: serde_json::Value = serde_json::from_str(&stdout).unwrap();
    assert_eq!(v["Total"], 2);
    assert_eq!(v["Entries"][0]["Source"], "my-server");
    assert_eq!(v["Entries"][0]["User"], "octocat");
    assert!(
        v["Entries"][0]["Fingerprint"]
            .as_str()
            .unwrap()
            .starts_with("SHA256:")
    );

    // remove by fingerprint
    let o = run(&[
        "remove",
        "--fingerprint",
        "SHA256:7MaIW0hEEsFKO7AbaY7AIztsc1Zzyf7jwHoWuUu54Rs=",
        "--config",
        cfg,
        "-o",
        out,
    ]);
    assert!(String::from_utf8_lossy(&o.stdout).contains("Removed:  1 key(s)"));

    // remove by target
    let o = run(&["remove", "work:zhangsan", "--config", cfg, "-o", out, "-q"]);
    assert!(o.status.success());

    // empty
    let o = run(&["list", "--config", cfg, "-o", out]);
    assert!(String::from_utf8_lossy(&o.stdout).contains("No keys managed by iskey."));
}

#[test]
fn dry_run_does_not_write() {
    let port = start_server();
    let dir = tempfile::tempdir().unwrap();
    let cfg = dir.path().join("sources.yaml");
    let out = dir.path().join("authorized_keys");
    write_config(&cfg, port);
    let cfg = cfg.to_str().unwrap();
    let out = out.to_str().unwrap();

    let o = run(&[
        "add",
        "--dry-run",
        "work:octocat",
        "--config",
        cfg,
        "-o",
        out,
    ]);
    assert!(o.status.success());
    let stdout = String::from_utf8_lossy(&o.stdout);
    assert!(stdout.contains("would-add") || stdout.contains("dry-run"));
    assert!(!std::path::Path::new(out).exists(), "dry-run 不应写入文件");
}

#[test]
fn config_set_verify() {
    let dir = tempfile::tempdir().unwrap();
    let cfg_path = dir.path().join("sources.yaml");
    let cfg = cfg_path.to_str().unwrap();

    // init
    let o = run(&["config", "init", "-o", cfg, "-q"]);
    assert!(o.status.success());
    assert!(cfg_path.exists());

    // set
    let o = run(&[
        "config",
        "set",
        "work",
        "http://x.com",
        "--config",
        cfg,
        "-q",
    ]);
    assert!(o.status.success());
    let content = fs::read_to_string(cfg).unwrap();
    assert!(content.contains("url: \"http://x.com/keys/{{ .User }}\""));

    // verify
    let o = run(&["config", "verify", "--config", cfg, "-q"]);
    assert!(o.status.success());
    assert!(String::from_utf8_lossy(&o.stdout).contains("Config file is valid."));

    // unset
    let o = run(&["config", "unset", "work", "--config", cfg, "-q"]);
    assert!(o.status.success());
    let content = fs::read_to_string(cfg).unwrap();
    assert!(!content.contains("http://x.com"));
}

#[test]
fn invalid_target_errors() {
    let o = run(&["add", "gh", "-q"]);
    assert!(!o.status.success());
    let stderr = String::from_utf8_lossy(&o.stderr);
    assert!(stderr.contains("invalid TARGET format"));
}

#[test]
fn missing_source_errors() {
    let o = run(&["add", "nope:user", "-o", "/tmp/x", "-q"]);
    assert!(!o.status.success());
    let stderr = String::from_utf8_lossy(&o.stderr);
    assert!(stderr.contains("source not found: nope"));
}
