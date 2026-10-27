//! iskey Rust 版入口：解析参数并把错误统一输出到 stderr。

mod cli;
mod config;
mod domain;
mod fetcher;
mod manager;
mod output;
mod service;

use clap::Parser;

/// 解析命令行参数；出错时打印错误并以非零码退出。
fn main() {
    let cli = cli::Cli::parse();
    if let Err(e) = cli::run(cli) {
        eprintln!("Error: {}", e);
        std::process::exit(1);
    }
}
