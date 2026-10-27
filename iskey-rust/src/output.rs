//! 文本表格与 JSON 结果输出。

use anyhow::Result;

/// JSON 格式输出（2 空格缩进，对齐 Go json.MarshalIndent）
pub fn print_result_json<T: serde::Serialize>(result: &T) {
    match serde_json::to_string_pretty(result) {
        Ok(s) => println!("{}", s),
        Err(e) => {
            eprintln!("error marshaling JSON: {}", e);
            std::process::exit(1);
        }
    }
}

/// 简易 tabwriter：每列对齐到最大宽度 + padding，模仿 Go text/tabwriter(minwidth=0, padding=2)
pub fn tabwriter(rows: &[Vec<String>], padding: usize) -> Result<String> {
    let ncols = rows.iter().map(|r| r.len()).max().unwrap_or(0);
    let mut widths = vec![0usize; ncols];
    for row in rows {
        for (i, cell) in row.iter().enumerate() {
            widths[i] = widths[i].max(cell.len());
        }
    }
    let mut out = String::new();
    for row in rows {
        for (i, cell) in row.iter().enumerate() {
            out.push_str(cell);
            if i + 1 < ncols {
                let pad = widths[i] + padding - cell.len();
                for _ in 0..pad {
                    out.push(' ');
                }
            }
        }
        out.push('\n');
    }
    Ok(out)
}
