//! authorized_keys 的解析、批量添加/删除与原子写入。

use std::collections::{HashMap, HashSet};
use std::fs;
use std::fs::{File, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::thread;
use std::time::{Duration, Instant};

use anyhow::{Context, Result, anyhow, bail};
use fs2::FileExt;
use tempfile::NamedTempFile;

use crate::domain::{self, KeyEntry};

/// 单条公钥的添加结果
#[derive(Debug, Clone)]
pub struct AddOutcome {
    /// 公钥条目
    pub entry: KeyEntry,
    /// added/replaced/skipped，dry-run 时为 would-* 前缀
    pub action: String,
    /// 公钥在最终文件中的 1-based 行号
    pub line: usize,
}

#[derive(Debug, Clone)]
struct ManagedEntry {
    /// 解析后的公钥条目
    entry: KeyEntry,
    /// 标记注释所在行的 0-based 下标
    comment_line: usize,
    /// 公钥所在行的 0-based 下标
    key_line: usize,
    /// 注释与公钥之间空行的下标
    blank_lines: Vec<usize>,
}

#[derive(Debug, Default)]
struct FileState {
    /// 文件原始行，保留非托管内容
    lines: Vec<String>,
    /// 当前文件中所有托管条目
    entries: Vec<ManagedEntry>,
}

/// authorized_keys 文件管理器
pub struct Manager {
    /// authorized_keys 目标路径
    file_path: PathBuf,
}

impl Manager {
    /// 创建指向指定 authorized_keys 的管理器。
    pub fn new(file_path: &str) -> Self {
        Self {
            file_path: PathBuf::from(file_path),
        }
    }

    #[cfg(test)]
    /// 返回目标文件路径（仅测试使用）。
    pub fn file_path(&self) -> &str {
        self.file_path.to_str().unwrap_or("")
    }

    /// 检查 ~/.ssh 目录（0700）与 authorized_keys 文件（0600）权限。
    /// 目录不存在时不再报错，首次写入会自动创建。
    pub fn check_permissions(&self) -> (Vec<String>, Option<anyhow::Error>) {
        let mut warnings = Vec::new();
        let ssh_dir = self.file_path.parent().unwrap_or(Path::new("."));
        let info = match fs::metadata(ssh_dir) {
            Ok(i) => i,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return (warnings, None),
            Err(e) => return (warnings, Some(e.into())),
        };
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let perm = info.permissions().mode() & 0o7777;
            if perm != 0o700 {
                warnings.push(format!(
                    "WARNING: {} permissions are {:o}, should be 0700",
                    ssh_dir.display(),
                    perm
                ));
            }
        }
        match fs::metadata(&self.file_path) {
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
            Err(e) => return (warnings, Some(e.into())),
            Ok(info) => {
                #[cfg(unix)]
                {
                    use std::os::unix::fs::PermissionsExt;
                    let perm = info.permissions().mode() & 0o7777;
                    if perm != 0o600 {
                        warnings.push(format!(
                            "WARNING: {} permissions are {:o}, should be 0600",
                            self.file_path.display(),
                            perm
                        ));
                    }
                }
            }
        }
        (warnings, None)
    }

    /// 加载所有由 iskey 管理的条目
    pub fn load(&self) -> Result<Vec<KeyEntry>> {
        Ok(self
            .load_state()?
            .entries
            .into_iter()
            .map(|m| m.entry)
            .collect())
    }

    /// 加载文件所有行（包括非 iskey 管理的条目）
    #[cfg(test)]
    /// 读取文件全部行（仅测试使用）。
    pub fn load_all(&self) -> Result<Vec<String>> {
        Ok(self.load_state()?.lines)
    }

    /// 解析文件为行与托管条目，容忍注释后空行。
    fn load_state(&self) -> Result<FileState> {
        let text = match fs::read_to_string(&self.file_path) {
            Ok(t) => t,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(FileState::default()),
            Err(e) => bail!("read authorized_keys: {}", e),
        };
        let lines: Vec<String> = if text.is_empty() {
            Vec::new()
        } else {
            text.trim_end_matches('\n')
                .split('\n')
                .map(str::to_string)
                .collect()
        };

        let mut state = FileState {
            lines,
            entries: Vec::new(),
        };
        let mut i = 0;
        while i < state.lines.len() {
            let trimmed = state.lines[i].trim();
            if !trimmed.starts_with("# iskey:") {
                i += 1;
                continue;
            }
            let comment = trimmed.trim_start_matches("# ").to_string();
            let Some((source, user, fingerprint)) = domain::parse_comment(&comment) else {
                i += 1;
                continue;
            };

            let mut j = i + 1;
            let mut blanks = Vec::new();
            while j < state.lines.len() && state.lines[j].trim().is_empty() {
                blanks.push(j);
                j += 1;
            }
            if j >= state.lines.len() {
                break;
            }
            let key_line = state.lines[j].trim();
            if key_line.is_empty() || key_line.starts_with('#') {
                i = j;
                continue;
            }
            let Ok(mut entry) = KeyEntry::new(key_line, &source, &user) else {
                i = j;
                continue;
            };
            // 指纹与注释中的值不一致时，以公钥内容为准。
            entry.comment = comment;
            entry.fingerprint = if fingerprint.is_empty() {
                entry.fingerprint
            } else {
                domain::fingerprint(key_line).unwrap_or(fingerprint)
            };

            state.entries.push(ManagedEntry {
                entry,
                comment_line: i,
                key_line: j,
                blank_lines: blanks,
            });
            i = j + 1;
        }
        Ok(state)
    }

    /// 添加单条记录（兼容旧调用）
    #[cfg(test)]
    /// 兼容旧接口的单条添加。
    pub fn add(&self, entry: &KeyEntry, force: bool) -> Result<bool> {
        let outcomes = self.add_many(std::slice::from_ref(entry), force, false)?;
        Ok(outcomes
            .first()
            .map(|o| o.action == "added" || o.action == "replaced")
            .unwrap_or(false))
    }

    /// 批量添加公钥。dry_run 时只计算最终行号，不写磁盘。
    pub fn add_many(
        &self,
        entries: &[KeyEntry],
        force: bool,
        dry_run: bool,
    ) -> Result<Vec<AddOutcome>> {
        if entries.is_empty() {
            return Ok(Vec::new());
        }
        for entry in entries {
            domain::validate_target(&entry.source, &entry.user)?;
        }
        if dry_run {
            return Ok(self.plan_add(entries, force, true)?.1);
        }

        self.ensure_dir()?;
        let _lock = self.lock()?;
        let (lines, outcomes, changed) = self.plan_add(entries, force, false)?;
        if changed {
            self.write_lines(&lines)?;
        }
        Ok(outcomes)
    }

    /// 计算批量添加后的行与变更结果，不直接写文件。
    fn plan_add(
        &self,
        entries: &[KeyEntry],
        force: bool,
        dry_run: bool,
    ) -> Result<(Vec<String>, Vec<AddOutcome>, bool)> {
        let state = self.load_state()?;

        let mut existing_by_fp: HashMap<String, usize> = HashMap::new();
        let mut key_lines = HashSet::new();
        for (idx, item) in state.entries.iter().enumerate() {
            key_lines.insert(item.key_line);
            existing_by_fp
                .entry(item.entry.fingerprint.clone())
                .or_insert(idx);
        }

        let groups: HashSet<String> = entries.iter().map(group_of).collect();
        let mut removed = HashSet::new();
        let mut had_existing = HashSet::new();
        if force {
            for (idx, item) in state.entries.iter().enumerate() {
                let group = group_of(&item.entry);
                if !groups.contains(&group) {
                    continue;
                }
                had_existing.insert(group);
                removed.insert(item.comment_line);
                removed.extend(item.blank_lines.iter().copied());
                removed.insert(item.key_line);
                if existing_by_fp.get(&item.entry.fingerprint) == Some(&idx) {
                    existing_by_fp.remove(&item.entry.fingerprint);
                }
            }
        }

        let mut output = Vec::with_capacity(state.lines.len() + entries.len() * 2);
        let mut new_key_line = HashMap::new();
        for (idx, line) in state.lines.iter().enumerate() {
            if removed.contains(&idx) {
                continue;
            }
            output.push(line.clone());
            if key_lines.contains(&idx) {
                new_key_line.insert(idx, output.len());
            }
        }

        let mut outcomes: Vec<AddOutcome> = Vec::with_capacity(entries.len());
        let mut seen_batch: HashMap<String, usize> = HashMap::new();
        let mut changed = !removed.is_empty();

        for entry in entries {
            if let Some(prev) = seen_batch.get(&entry.fingerprint) {
                outcomes.push(AddOutcome {
                    entry: entry.clone(),
                    action: "skipped".to_string(),
                    line: outcomes[*prev].line,
                });
                continue;
            }

            if let Some(idx) = existing_by_fp.get(&entry.fingerprint) {
                let line = new_key_line
                    .get(&state.entries[*idx].key_line)
                    .copied()
                    .unwrap_or(0);
                outcomes.push(AddOutcome {
                    entry: entry.clone(),
                    action: "skipped".to_string(),
                    line,
                });
                seen_batch.insert(entry.fingerprint.clone(), outcomes.len() - 1);
                continue;
            }

            let comment = if entry.comment.starts_with("iskey:") {
                entry.comment.clone()
            } else {
                domain::format_comment(&entry.source, &entry.user, &entry.fingerprint)
            };
            output.push(format!("# {}", comment));
            output.push(entry.public_key.clone());
            let action = if force && had_existing.contains(&group_of(entry)) {
                "replaced"
            } else {
                "added"
            };
            outcomes.push(AddOutcome {
                entry: entry.clone(),
                action: action.to_string(),
                line: output.len(),
            });
            seen_batch.insert(entry.fingerprint.clone(), outcomes.len() - 1);
            changed = true;
        }

        if dry_run {
            for outcome in &mut outcomes {
                outcome.action = match outcome.action.as_str() {
                    "added" => "would-add".to_string(),
                    "replaced" => "would-replace".to_string(),
                    _ => "would-skip".to_string(),
                };
            }
        }

        Ok((output, outcomes, changed))
    }

    /// 按源+用户或指纹删除条目
    pub fn remove_by_target(
        &self,
        source: &str,
        user: &str,
        fingerprint: &str,
        dry_run: bool,
    ) -> Result<Vec<KeyEntry>> {
        let fingerprint = fingerprint.trim();
        if fingerprint.is_empty() && (source.is_empty() || user.is_empty()) {
            bail!("source and user are required when fingerprint is empty");
        }
        self.remove_matching(dry_run, |e| {
            if !fingerprint.is_empty() {
                e.entry.fingerprint == fingerprint
            } else {
                e.entry.source == source && e.entry.user == user
            }
        })
    }

    /// 删除指定源下的所有条目
    pub fn remove_all_from_source(&self, source: &str, dry_run: bool) -> Result<Vec<KeyEntry>> {
        if source.is_empty() {
            bail!("source is required");
        }
        self.remove_matching(dry_run, |e| e.entry.source == source)
    }

    /// 按条件收集并移除条目，返回被删除的公钥。
    fn remove_matching(
        &self,
        dry_run: bool,
        predicate: impl Fn(&ManagedEntry) -> bool,
    ) -> Result<Vec<KeyEntry>> {
        let state = self.load_state()?;
        let matched: Vec<&ManagedEntry> = state.entries.iter().filter(|e| predicate(e)).collect();
        if matched.is_empty() {
            return Ok(Vec::new());
        }
        let removed_entries: Vec<KeyEntry> = matched.iter().map(|m| m.entry.clone()).collect();
        if dry_run {
            return Ok(removed_entries);
        }

        self.ensure_dir()?;
        let _lock = self.lock()?;
        // 重新加载，避免在等待锁期间覆盖其他进程的修改。
        let state = self.load_state()?;
        let mut removed_lines = HashSet::new();
        let mut removed = Vec::new();
        for item in state.entries.iter().filter(|e| predicate(e)) {
            removed_lines.insert(item.comment_line);
            removed_lines.extend(item.blank_lines.iter().copied());
            removed_lines.insert(item.key_line);
            removed.push(item.entry.clone());
        }
        if removed.is_empty() {
            return Ok(Vec::new());
        }
        let lines: Vec<String> = state
            .lines
            .iter()
            .enumerate()
            .filter(|(idx, _)| !removed_lines.contains(idx))
            .map(|(_, line)| line.clone())
            .collect();
        self.write_lines(&lines)?;
        Ok(removed)
    }

    /// 列出所有管理条目，可按源筛选
    pub fn list(&self, source_filter: &str) -> Result<Vec<KeyEntry>> {
        let entries = self.load()?;
        if source_filter.is_empty() {
            return Ok(entries);
        }
        Ok(entries
            .into_iter()
            .filter(|e| e.source == source_filter)
            .collect())
    }

    /// 确保父目录存在且权限为 0700。
    fn ensure_dir(&self) -> Result<()> {
        let dir = self.file_path.parent().unwrap_or(Path::new("."));
        fs::create_dir_all(dir).context("create SSH directory")?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let _ = fs::set_permissions(dir, fs::Permissions::from_mode(0o700));
        }
        Ok(())
    }

    /// 获取跨进程文件锁，超时 10 秒。
    fn lock(&self) -> Result<File> {
        let lock_path = PathBuf::from(format!("{}.lock", self.file_path.display()));
        let file = OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(&lock_path)
            .context("open authorized_keys lock")?;
        let deadline = Instant::now() + Duration::from_secs(10);
        loop {
            match file.try_lock_exclusive() {
                Ok(()) => return Ok(file),
                Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                    if Instant::now() >= deadline {
                        bail!("timed out waiting for authorized_keys lock");
                    }
                    thread::sleep(Duration::from_millis(50));
                }
                Err(e) => return Err(anyhow!("lock authorized_keys: {}", e)),
            }
        }
    }

    /// 原子写入完整文件内容，并保留 .bak 备份。
    fn write_lines(&self, lines: &[String]) -> Result<()> {
        let dir = self.file_path.parent().unwrap_or(Path::new("."));
        let mut tmp = NamedTempFile::new_in(dir).context("create temp file")?;
        {
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                tmp.as_file()
                    .set_permissions(fs::Permissions::from_mode(0o600))
                    .context("set temp file permissions")?;
            }
            for line in lines {
                writeln!(tmp, "{}", line).context("write temp file")?;
            }
            tmp.flush().context("flush temp file")?;
            tmp.as_file().sync_all().context("sync temp file")?;
        }

        let backup = PathBuf::from(format!("{}.bak", self.file_path.display()));
        if self.file_path.exists() {
            fs::copy(&self.file_path, &backup).context("backup authorized_keys")?;
        }

        if let Err(e) = tmp.persist(&self.file_path) {
            return Err(anyhow!("atomic rename failed: {}", e));
        }

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&self.file_path, fs::Permissions::from_mode(0o600))
                .context("set file permissions")?;
        }
        if let Ok(dir_file) = File::open(dir) {
            let _ = dir_file.sync_all();
        }
        Ok(())
    }
}

/// 生成 source:user 分组键。
fn group_of(entry: &KeyEntry) -> String {
    format!("{}\u{0}{}", entry.source, entry.user)
}

#[cfg(test)]
mod tests {
    use super::*;

    const KEY: &str = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO95iT5q2IE3X24g2Pfq0v8s9erHGOo2VYrl/nvAVV95 test@example.com";
    const KEY2: &str = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIP5KPytIk1TuhtOv9L8vuU/kBy+Em39+YwPI0jU7F9HL b@example.com";
    const KEY3: &str = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAi48sTS1ulLGTbnPA4NKpaW9Npf3g7V+yFqKQjpfT6O c@example.com";

    fn tmp_manager(tag: &str) -> (Manager, tempfile::TempDir) {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join(tag);
        (Manager::new(path.to_str().unwrap()), dir)
    }

    #[test]
    fn test_add_and_load() {
        let (m, _dir) = tmp_manager("auth");
        let entry = KeyEntry::new(KEY, "github", "octocat").unwrap();
        assert!(m.add(&entry, false).unwrap());
        assert!(!m.add(&entry, false).unwrap());
        let loaded = m.load().unwrap();
        assert_eq!(loaded.len(), 1);
        assert_eq!(loaded[0].source, "github");

        let lines = m.load_all().unwrap();
        assert_eq!(lines.len(), 2);
        assert!(lines[0].starts_with("# iskey:github:octocat:"));
        assert_eq!(lines[1].split(' ').next().unwrap(), "ssh-ed25519");
    }

    #[test]
    fn test_add_many_imports_all_keys() {
        let (m, _dir) = tmp_manager("auth-many");
        let entries = vec![
            KeyEntry::new(KEY, "work", "alice").unwrap(),
            KeyEntry::new(KEY2, "work", "alice").unwrap(),
        ];
        let outcomes = m.add_many(&entries, false, false).unwrap();
        assert_eq!(outcomes.len(), 2);
        assert_eq!(outcomes[0].action, "added");
        assert_eq!(outcomes[0].line, 2);
        assert_eq!(outcomes[1].line, 4);
        assert_eq!(m.load().unwrap().len(), 2);

        let again = m.add_many(&entries, false, false).unwrap();
        assert!(again.iter().all(|o| o.action == "skipped"));
    }

    #[test]
    fn test_dry_run_does_not_create_file() {
        let (m, _dir) = tmp_manager("auth-dry");
        let entry = KeyEntry::new(KEY, "work", "alice").unwrap();
        let outcomes = m.add_many(&[entry], false, true).unwrap();
        assert_eq!(outcomes[0].action, "would-add");
        assert!(!Path::new(m.file_path()).exists());

        m.add_many(
            &[KeyEntry::new(KEY, "work", "alice").unwrap()],
            false,
            false,
        )
        .unwrap();
        let removed = m.remove_by_target("work", "alice", "", true).unwrap();
        assert_eq!(removed.len(), 1);
        assert_eq!(m.load().unwrap().len(), 1);
    }

    #[test]
    fn test_force_replaces_group_and_preserves_non_iskey_lines() {
        let (m, _dir) = tmp_manager("auth-force");
        fs::write(
            &m.file_path,
            "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQXYZ user@host\n",
        )
        .unwrap();
        m.add_many(
            &[
                KeyEntry::new(KEY, "work", "alice").unwrap(),
                KeyEntry::new(KEY2, "work", "alice").unwrap(),
                KeyEntry::new(KEY3, "work", "bob").unwrap(),
            ],
            false,
            false,
        )
        .unwrap();
        let outcomes = m
            .add_many(&[KeyEntry::new(KEY, "work", "alice").unwrap()], true, false)
            .unwrap();
        assert_eq!(outcomes[0].action, "replaced");

        let loaded = m.load().unwrap();
        assert_eq!(loaded.len(), 2);
        let raw = fs::read_to_string(&m.file_path).unwrap();
        assert!(raw.contains("user@host"));
        assert!(!raw.contains(KEY2));
        assert_eq!(raw.matches(KEY).count(), 1);
    }

    #[test]
    fn test_remove_by_target_and_fingerprint() {
        let (m, _dir) = tmp_manager("auth3");
        let gh = KeyEntry::new(KEY, "github", "octocat").unwrap();
        let work = KeyEntry::new(KEY2, "work", "zhangsan").unwrap();
        m.add(&gh, false).unwrap();
        m.add(&work, false).unwrap();

        assert_eq!(
            m.remove_by_target("github", "octocat", "", false)
                .unwrap()
                .len(),
            1
        );
        let loaded = m.load().unwrap();
        assert_eq!(loaded.len(), 1);
        assert_eq!(loaded[0].source, "work");

        assert_eq!(
            m.remove_by_target("", "", &work.fingerprint, false)
                .unwrap()
                .len(),
            1
        );
        assert!(m.load().unwrap().is_empty());
    }

    #[test]
    fn test_remove_all_from_source() {
        let (m, _dir) = tmp_manager("auth4");
        m.add(&KeyEntry::new(KEY, "work", "a").unwrap(), false)
            .unwrap();
        m.add(&KeyEntry::new(KEY2, "work", "b").unwrap(), false)
            .unwrap();
        assert_eq!(m.remove_all_from_source("work", false).unwrap().len(), 2);
        assert!(m.load().unwrap().is_empty());
    }

    #[test]
    fn test_load_allows_blank_line_between_comment_and_key() {
        let (m, _dir) = tmp_manager("auth-blank");
        let entry = KeyEntry::new(KEY, "work", "alice").unwrap();
        fs::write(&m.file_path, format!("# {}\n\n{}\n", entry.comment, KEY)).unwrap();
        let loaded = m.load().unwrap();
        assert_eq!(loaded.len(), 1);
        assert_eq!(loaded[0].fingerprint, entry.fingerprint);
    }
}
