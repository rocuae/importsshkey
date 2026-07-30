// manager 包负责本地 authorized_keys 文件的读写管理
package manager

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/importsshkey/importsshkey/internal/domain"
)

// Manager authorized_keys 文件管理器
type Manager struct {
	// filePath authorized_keys 文件路径
	filePath string
}

// NewManager 创建管理器
// 参数：
//   - filePath: authorized_keys 文件路径
//
// 返回：
//   - *Manager: 管理器实例
func NewManager(filePath string) *Manager {
	return &Manager{filePath: filePath}
}

// CheckPermissions 检查 ~/.ssh 目录和 authorized_keys 文件权限
// 返回：
//   - []string: 警告信息列表
//   - error: 检查失败错误
func (m *Manager) CheckPermissions() ([]string, error) {
	var warnings []string

	// 检查 ~/.ssh 目录权限
	sshDir := filepath.Dir(m.filePath)
	info, err := os.Stat(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return warnings, fmt.Errorf("SSH directory does not exist: %s", sshDir)
		}
		return warnings, err
	}

	// 目录权限应为 0700
	if info.Mode().Perm() != 0700 {
		warnings = append(warnings, fmt.Sprintf("WARNING: %s permissions are %o, should be 0700", sshDir, info.Mode().Perm()))
	}

	// 检查 authorized_keys 文件权限
	info, err = os.Stat(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return warnings, nil // 文件不存在是正常的
		}
		return warnings, err
	}

	// 文件权限应为 0600
	if info.Mode().Perm() != 0600 {
		warnings = append(warnings, fmt.Sprintf("WARNING: %s permissions are %o, should be 0600", m.filePath, info.Mode().Perm()))
	}

	return warnings, nil
}

// Load 加载文件，解析出所有由 iskey 管理的条目
// 返回：
//   - []*domain.KeyEntry: 公钥列表
//   - error: 文件读取错误
func (m *Manager) Load() ([]*domain.KeyEntry, error) {
	f, err := os.Open(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open authorized_keys: %w", err)
	}
	defer f.Close()

	var entries []*domain.KeyEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		entry, ok := ParseKeyLine(line)
		if ok {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read authorized_keys: %w", err)
	}

	return entries, nil
}

// LoadAll 加载文件所有行（包括非 iskey 管理的条目）
// 返回：
//   - []string: 所有行
//   - error: 文件读取错误
func (m *Manager) LoadAll() ([]string, error) {
	f, err := os.Open(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open authorized_keys: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read authorized_keys: %w", err)
	}

	return lines, nil
}

// ParseKeyLine 解析 authorized_keys 行，提取 iskey 管理的条目
// 参数：
//   - line: authorized_keys 行内容
//
// 返回：
//   - *domain.KeyEntry: 解析出的条目
//   - bool: 是否为 iskey 管理的条目
func ParseKeyLine(line string) (*domain.KeyEntry, bool) {
	// iskey 注释格式: iskey:<source>:<user>:<fingerprint>
	idx := strings.LastIndex(line, "# iskey:")
	if idx < 0 {
		return nil, false
	}

	comment := strings.TrimSpace(line[idx+2:])
	source, user, fingerprint, ok := domain.ParseComment(comment)
	if !ok {
		return nil, false
	}

	pubKey := strings.TrimSpace(line[:idx])
	return &domain.KeyEntry{
		PublicKey:   pubKey,
		Source:      source,
		User:        user,
		Fingerprint: fingerprint,
		Comment:     comment,
	}, true
}

// Add 添加新条目（若已存在相同指纹，根据 force 决定是否覆盖）
// 参数：
//   - entry: 要添加的公钥条目
//   - force: 是否强制覆盖已存在的相同指纹条目
//
// 返回：
//   - bool: 是否实际添加了新条目
//   - error: 文件读写错误
func (m *Manager) Add(entry *domain.KeyEntry, force bool) (bool, error) {
	existing, err := m.Load()
	if err != nil {
		return false, err
	}

	// 检查是否已存在相同指纹
	for i, e := range existing {
		if e.Fingerprint == entry.Fingerprint {
			if !force {
				return false, nil // 已存在，跳过
			}
			existing[i] = entry // 覆盖
			return true, m.writeEntries(existing)
		}
	}

	existing = append(existing, entry)
	return true, m.writeEntries(existing)
}

// RemoveByTarget 根据源+用户或指纹删除条目
// 参数：
//   - source: 来源标识（可为空）
//   - user: 用户名（可为空）
//   - fingerprint: 指纹（可为空，优先使用）
//
// 返回：
//   - int: 删除的条目数
//   - error: 文件读写错误
func (m *Manager) RemoveByTarget(source, user, fingerprint string) (int, error) {
	existing, err := m.Load()
	if err != nil {
		return 0, err
	}

	var kept []*domain.KeyEntry
	removed := 0
	for _, e := range existing {
		match := false
		if fingerprint != "" && e.Fingerprint == fingerprint {
			match = true
		} else if source != "" && user != "" && e.Source == source && e.User == user {
			match = true
		}
		if match {
			removed++
		} else {
			kept = append(kept, e)
		}
	}

	if removed > 0 {
		if err := m.writeEntries(kept); err != nil {
			return 0, err
		}
	}
	return removed, nil
}

// RemoveAllFromSource 删除指定源下的所有条目
// 参数：
//   - source: 来源标识
//
// 返回：
//   - int: 删除的条目数
//   - error: 文件读写错误
func (m *Manager) RemoveAllFromSource(source string) (int, error) {
	existing, err := m.Load()
	if err != nil {
		return 0, err
	}

	var kept []*domain.KeyEntry
	removed := 0
	for _, e := range existing {
		if e.Source == source {
			removed++
		} else {
			kept = append(kept, e)
		}
	}

	if removed > 0 {
		if err := m.writeEntries(kept); err != nil {
			return 0, err
		}
	}
	return removed, nil
}

// List 列出所有管理的条目，可按源筛选
// 参数：
//   - sourceFilter: 源筛选（空字符串表示全部）
//
// 返回：
//   - []*domain.KeyEntry: 条目列表
//   - error: 文件读取错误
func (m *Manager) List(sourceFilter string) ([]*domain.KeyEntry, error) {
	entries, err := m.Load()
	if err != nil {
		return nil, err
	}
	if sourceFilter == "" {
		return entries, nil
	}

	var filtered []*domain.KeyEntry
	for _, e := range entries {
		if e.Source == sourceFilter {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// Sync 全量同步：给定远程列表，计算差集并执行新增/删除
// 参数：
//   - remoteEntries: 远程公钥列表
//   - prune: 是否清理本地孤立条目
//
// 返回：
//   - *SyncResult: 同步结果
//   - error: 文件读写错误
func (m *Manager) Sync(remoteEntries []*domain.KeyEntry, prune bool) (*SyncResult, error) {
	local, err := m.Load()
	if err != nil {
		return nil, fmt.Errorf("load local keys: %w", err)
	}

	result := &SyncResult{}

	// 计算差异
	toAdd, toRemove, unchanged := Diff(local, remoteEntries)

	// 添加新条目
	for _, entry := range toAdd {
		if _, err := m.Add(entry, false); err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Added = append(result.Added, entry.Fingerprint)
	}

	// 删除远程已移除的条目
	if prune {
		for _, entry := range toRemove {
			if _, err := m.RemoveByTarget("", "", entry.Fingerprint); err != nil {
				result.Errors = append(result.Errors, err)
				continue
			}
			result.Removed = append(result.Removed, entry.Fingerprint)
		}
	}

	// 未变更的条目
	for _, entry := range unchanged {
		result.Skipped = append(result.Skipped, entry.Fingerprint)
	}

	return result, nil
}

// writeEntries 原子写入条目到文件（先写临时文件，再 rename）
// 参数：
//   - entries: 要写入的条目列表
//
// 返回：
//   - error: 文件写入错误
func (m *Manager) writeEntries(entries []*domain.KeyEntry) error {
	dir := filepath.Dir(m.filePath)
	tmpFile, err := os.CreateTemp(dir, ".iskey-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// 写入失败时清理临时文件
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	// 读取非 iskey 管理的行，保留原样
	allLines, loadErr := m.LoadAll()
	if loadErr == nil {
		for _, line := range allLines {
			if _, ok := ParseKeyLine(line); !ok {
				fmt.Fprintln(tmpFile, line)
			}
		}
	}

	// 写入 iskey 管理的条目
	for _, e := range entries {
		line := fmt.Sprintf("%s # %s", e.PublicKey, e.Comment)
		if _, err := fmt.Fprintln(tmpFile, line); err != nil {
			return fmt.Errorf("write to temp file: %w", err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// 备份原文件
	backupPath := m.filePath + ".bak"
	if _, statErr := os.Stat(m.filePath); statErr == nil {
		if err := copyFile(m.filePath, backupPath); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}

	// 原子替换
	if err := os.Rename(tmpPath, m.filePath); err != nil {
		// 失败时回滚
		if _, statErr := os.Stat(backupPath); statErr == nil {
			if rollbackErr := copyFile(backupPath, m.filePath); rollbackErr != nil {
				return fmt.Errorf("atomic rename failed and rollback failed: %w, rollback error: %v", err, rollbackErr)
			}
		}
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	// 设置文件权限为 0600
	if err := os.Chmod(m.filePath, 0600); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}

	// 清理备份
	os.Remove(backupPath)
	return nil
}

// copyFile 复制文件
// 参数：
//   - src: 源文件路径
//   - dst: 目标文件路径
//
// 返回：
//   - error: 复制错误
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// Diff 计算本地与远程公钥集合的差异
// 参数：
//   - local: 本地条目列表
//   - remote: 远程条目列表
//
// 返回：
//   - toAdd: 需要新增的条目
//   - toRemove: 需要删除的条目
//   - unchanged: 未变更的条目
func Diff(local, remote []*domain.KeyEntry) (toAdd, toRemove, unchanged []*domain.KeyEntry) {
	localMap := make(map[string]*domain.KeyEntry)
	for _, e := range local {
		localMap[e.Fingerprint] = e
	}
	remoteMap := make(map[string]*domain.KeyEntry)
	for _, e := range remote {
		remoteMap[e.Fingerprint] = e
	}

	for fp, entry := range remoteMap {
		if _, exists := localMap[fp]; !exists {
			toAdd = append(toAdd, entry)
		} else {
			unchanged = append(unchanged, entry)
		}
	}
	for fp, entry := range localMap {
		if _, exists := remoteMap[fp]; !exists {
			toRemove = append(toRemove, entry)
		}
	}
	return
}

// SyncResult 同步操作的结果
type SyncResult struct {
	// Added 新增的指纹列表
	Added []string
	// Removed 移除的指纹列表
	Removed []string
	// Skipped 已存在且未变更的指纹列表
	Skipped []string
	// Errors 同步过程中的错误
	Errors []error
}
