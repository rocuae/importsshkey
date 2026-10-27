// manager 包负责本地 authorized_keys 文件的读写管理
package manager

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/rocuae/importsshkey/internal/domain"
)

const (
	// lockTimeout 等待文件锁的最长时间，避免并发 CLI 互相永久阻塞。
	lockTimeout = 10 * time.Second
)

// Manager authorized_keys 文件管理器
type Manager struct {
	// filePath authorized_keys 文件路径
	filePath string
}

// AddOutcome 描述一次批量添加中单把公钥的处理结果
type AddOutcome struct {
	// Entry 公钥条目
	Entry *domain.KeyEntry
	// Action added / replaced / skipped（dry-run 时为 would-add / would-replace / would-skip）
	Action string
	// Line 公钥在最终文件中的行号（1-based，dry-run 时为预计行号）
	Line int
}

// managedEntry 文件中由 iskey 管理的一组行
type managedEntry struct {
	// entry 解析后的公钥条目（指纹以公钥内容重算为准）
	entry *domain.KeyEntry
	// commentLine 标记注释所在行的 0-based 下标
	commentLine int
	// keyLine 公钥所在行的 0-based 下标
	keyLine int
	// blankLines 注释与公钥之间空行的下标，删除时一并清理
	blankLines []int
}

// fileState authorized_keys 的完整快照
type fileState struct {
	// lines 文件原始行，保留非托管内容
	lines []string
	// entries 当前文件中所有由 iskey 管理的条目
	entries []managedEntry
}

// addPlan 一次批量添加的完整计划
type addPlan struct {
	// lines 计划写入的完整文件内容
	lines []string
	// outcomes 与输入 entries 一一对应的处理结果
	outcomes []AddOutcome
	// changed 是否需要落盘（dry-run 时仅用于判断是否存在变更）
	changed bool
}

// NewManager 创建管理器
func NewManager(filePath string) *Manager {
	return &Manager{filePath: filePath}
}

// CheckPermissions 检查 ~/.ssh 目录和 authorized_keys 文件权限。
// 目录不存在属于正常情况（写入时会自动创建），只返回提示而不返回错误。
func (m *Manager) CheckPermissions() ([]string, error) {
	var warnings []string
	sshDir := filepath.Dir(m.filePath)

	info, err := os.Stat(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return warnings, nil
		}
		return warnings, err
	}
	if info.Mode().Perm() != 0700 {
		warnings = append(warnings, fmt.Sprintf("WARNING: %s permissions are %o, should be 0700", sshDir, info.Mode().Perm()))
	}

	info, err = os.Stat(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return warnings, nil
		}
		return warnings, err
	}
	if info.Mode().Perm() != 0600 {
		warnings = append(warnings, fmt.Sprintf("WARNING: %s permissions are %o, should be 0600", m.filePath, info.Mode().Perm()))
	}
	return warnings, nil
}

// Load 加载文件，解析出所有由 iskey 管理的条目。
// 读取失败会返回错误，调用方不得在错误后继续写入。
func (m *Manager) Load() ([]*domain.KeyEntry, error) {
	state, err := m.load()
	if err != nil {
		return nil, err
	}
	entries := make([]*domain.KeyEntry, 0, len(state.entries))
	for _, item := range state.entries {
		entries = append(entries, item.entry)
	}
	return entries, nil
}

// LoadAll 加载文件所有行（包括非 iskey 管理的条目）
func (m *Manager) LoadAll() ([]string, error) {
	state, err := m.load()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), state.lines...), nil
}

// load 读取并解析 authorized_keys，文件不存在视为空文件。
func (m *Manager) load() (*fileState, error) {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &fileState{}, nil
		}
		return nil, fmt.Errorf("read authorized_keys: %w", err)
	}

	text := string(data)
	var lines []string
	if text != "" {
		lines = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}

	state := &fileState{lines: lines}
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "# iskey:") {
			continue
		}

		comment := strings.TrimPrefix(trimmed, "# ")
		source, user, fp, ok := domain.ParseComment(comment)
		if !ok {
			continue
		}

		// 允许注释和公钥之间存在空行，但注释后面不能紧跟另一条注释。
		j := i + 1
		var blanks []int
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			blanks = append(blanks, j)
			j++
		}
		if j >= len(lines) {
			continue
		}
		keyLine := strings.TrimSpace(lines[j])
		if keyLine == "" || strings.HasPrefix(keyLine, "#") {
			continue
		}
		if err := domain.ValidatePublicKey(keyLine); err != nil {
			continue
		}

		// 指纹以公钥内容为准，注释中的指纹只作为展示/兼容保留。
		if computed := domain.Fingerprint(keyLine); computed != "" {
			fp = computed
		}

		state.entries = append(state.entries, managedEntry{
			entry: &domain.KeyEntry{
				PublicKey:   keyLine,
				Source:      source,
				User:        user,
				Fingerprint: fp,
				Comment:     comment,
			},
			commentLine: i,
			keyLine:     j,
			blankLines:  blanks,
		})
		i = j
	}

	return state, nil
}

// Add 添加单个条目，保留旧 API 供调用方使用。
func (m *Manager) Add(entry *domain.KeyEntry, force bool) (bool, error) {
	outcomes, err := m.AddMany([]*domain.KeyEntry{entry}, force, false)
	if err != nil {
		return false, err
	}
	if len(outcomes) == 0 {
		return false, nil
	}
	return outcomes[0].Action == "added" || outcomes[0].Action == "replaced", nil
}

// AddMany 批量添加公钥。dryRun 为 true 时只计算最终文件与行号，不写磁盘。
// 所有条目在同一个读-改-写周期内完成，失败时原文件保持不变。
func (m *Manager) AddMany(entries []*domain.KeyEntry, force, dryRun bool) ([]AddOutcome, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	for _, entry := range entries {
		if entry == nil {
			return nil, fmt.Errorf("nil key entry")
		}
		if err := domain.ValidateTarget(entry.Source, entry.User); err != nil {
			return nil, err
		}
	}

	if dryRun {
		plan, err := m.planAdd(entries, force, true)
		if err != nil {
			return nil, err
		}
		return plan.outcomes, nil
	}

	if err := m.ensureDir(); err != nil {
		return nil, err
	}
	unlock, err := m.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	plan, err := m.planAdd(entries, force, false)
	if err != nil {
		return nil, err
	}
	if plan.changed {
		if err := m.writeLines(plan.lines); err != nil {
			return nil, err
		}
	}
	return plan.outcomes, nil
}

// planAdd 计算批量添加后的文件内容、行号和每把 key 的动作。
func (m *Manager) planAdd(entries []*domain.KeyEntry, force, dryRun bool) (*addPlan, error) {
	state, err := m.load()
	if err != nil {
		return nil, err
	}

	existingByFP := make(map[string]*managedEntry)
	keyLines := make(map[int]bool)
	for i := range state.entries {
		item := &state.entries[i]
		keyLines[item.keyLine] = true
		if _, ok := existingByFP[item.entry.Fingerprint]; !ok {
			existingByFP[item.entry.Fingerprint] = item
		}
	}

	groups := make(map[string]bool)
	for _, entry := range entries {
		groups[targetGroup(entry.Source, entry.User)] = true
	}

	removed := make(map[int]bool)
	hadExisting := make(map[string]bool)
	if force {
		for i := range state.entries {
			item := &state.entries[i]
			group := targetGroup(item.entry.Source, item.entry.User)
			if !groups[group] {
				continue
			}
			hadExisting[group] = true
			markRemoved(removed, item)
			if existingByFP[item.entry.Fingerprint] == item {
				delete(existingByFP, item.entry.Fingerprint)
			}
		}
	}

	// 先重建保留行，得到每把已有 key 在最终文件中的行号。
	output := make([]string, 0, len(state.lines)+len(entries)*2)
	newKeyLine := make(map[int]int)
	for i, line := range state.lines {
		if removed[i] {
			continue
		}
		output = append(output, line)
		if keyLines[i] {
			newKeyLine[i] = len(output)
		}
	}

	outcomes := make([]AddOutcome, len(entries))
	seenBatch := make(map[string]int)
	changed := len(removed) > 0

	for i, entry := range entries {
		if prev, ok := seenBatch[entry.Fingerprint]; ok {
			outcomes[i] = AddOutcome{
				Entry:  entry,
				Action: "skipped",
				Line:   outcomes[prev].Line,
			}
			continue
		}

		if existing, ok := existingByFP[entry.Fingerprint]; ok {
			line := newKeyLine[existing.keyLine]
			action := "skipped"
			if force && groups[targetGroup(entry.Source, entry.User)] {
				action = "skipped"
			}
			outcomes[i] = AddOutcome{Entry: entry, Action: action, Line: line}
			seenBatch[entry.Fingerprint] = i
			continue
		}

		comment := entry.Comment
		if !strings.HasPrefix(comment, "iskey:") {
			comment = domain.FormatComment(entry.Source, entry.User, entry.Fingerprint)
		}
		output = append(output, "# "+comment)
		output = append(output, entry.PublicKey)

		action := "added"
		if force && hadExisting[targetGroup(entry.Source, entry.User)] {
			action = "replaced"
		}
		outcomes[i] = AddOutcome{Entry: entry, Action: action, Line: len(output)}
		seenBatch[entry.Fingerprint] = i
		changed = true
	}

	if dryRun {
		for i := range outcomes {
			switch outcomes[i].Action {
			case "added":
				outcomes[i].Action = "would-add"
			case "replaced":
				outcomes[i].Action = "would-replace"
			case "skipped":
				outcomes[i].Action = "would-skip"
			}
		}
	}

	return &addPlan{lines: output, outcomes: outcomes, changed: changed}, nil
}

// targetGroup 生成 source+user 的分组键，使用 NUL 避免分隔符冲突。
func targetGroup(source, user string) string {
	return source + "\x00" + user
}

// markRemoved 将托管条目的注释、空行和公钥行一起标记为删除。
func markRemoved(removed map[int]bool, item *managedEntry) {
	removed[item.commentLine] = true
	for _, line := range item.blankLines {
		removed[line] = true
	}
	removed[item.keyLine] = true
}

// RemoveByTarget 删除匹配的公钥。fingerprint 非空时优先按指纹匹配，
// 否则按 source+user 删除。dryRun 为 true 时不写磁盘。
func (m *Manager) RemoveByTarget(source, user, fingerprint string, dryRun bool) ([]*domain.KeyEntry, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" && (source == "" || user == "") {
		return nil, fmt.Errorf("source and user are required when fingerprint is empty")
	}

	return m.removeMatching(dryRun, func(item *managedEntry) bool {
		if fingerprint != "" {
			return item.entry.Fingerprint == fingerprint
		}
		return item.entry.Source == source && item.entry.User == user
	})
}

// RemoveAllFromSource 删除指定源下的所有条目。
func (m *Manager) RemoveAllFromSource(source string, dryRun bool) ([]*domain.KeyEntry, error) {
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	return m.removeMatching(dryRun, func(item *managedEntry) bool {
		return item.entry.Source == source
	})
}

// removeMatching 按谓词删除条目；dry-run 只返回将被删除的条目，不触碰文件。
func (m *Manager) removeMatching(dryRun bool, match func(*managedEntry) bool) ([]*domain.KeyEntry, error) {
	if dryRun {
		state, err := m.load()
		if err != nil {
			return nil, err
		}
		var removed []*domain.KeyEntry
		for i := range state.entries {
			if match(&state.entries[i]) {
				removed = append(removed, state.entries[i].entry)
			}
		}
		return removed, nil
	}

	if _, err := os.Stat(filepath.Dir(m.filePath)); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	unlock, err := m.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	state, err := m.load()
	if err != nil {
		return nil, err
	}

	removedSet := make(map[int]bool)
	var removed []*domain.KeyEntry
	for i := range state.entries {
		item := &state.entries[i]
		if match(item) {
			removed = append(removed, item.entry)
			markRemoved(removedSet, item)
		}
	}
	if len(removed) == 0 {
		return nil, nil
	}

	lines := make([]string, 0, len(state.lines))
	for i, line := range state.lines {
		if !removedSet[i] {
			lines = append(lines, line)
		}
	}
	if err := m.writeLines(lines); err != nil {
		return nil, err
	}
	return removed, nil
}

// List 列出所有管理的条目，可按源筛选
func (m *Manager) List(sourceFilter string) ([]*domain.KeyEntry, error) {
	entries, err := m.Load()
	if err != nil {
		return nil, err
	}
	if sourceFilter == "" {
		return entries, nil
	}

	filtered := make([]*domain.KeyEntry, 0, len(entries))
	for _, e := range entries {
		if e.Source == sourceFilter {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// writeLines 原子写入完整文件内容，并在替换前保存 .bak 备份。
func (m *Manager) writeLines(lines []string) error {
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create SSH directory: %w", err)
	}

	// 备份原文件；备份失败则中止，避免没有回滚点。
	if data, err := os.ReadFile(m.filePath); err == nil {
		if writeErr := os.WriteFile(m.filePath+".bak", data, 0600); writeErr != nil {
			return fmt.Errorf("backup authorized_keys: %w", writeErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read authorized_keys for backup: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".iskey-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(0600); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	writer := bufio.NewWriter(tmpFile)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("write temp file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, m.filePath); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	success = true

	if err := os.Chmod(m.filePath, 0600); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

// ensureDir 确保目标目录存在且权限为 0700。
func (m *Manager) ensureDir() error {
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0700); err != nil {
		return fmt.Errorf("create SSH directory: %w", err)
	}
	return nil
}

// lock 获取跨进程文件锁，返回释放函数；等待超过 lockTimeout 则报错。
func (m *Manager) lock() (func(), error) {
	fileLock := flock.New(m.filePath + ".lock")
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("lock authorized_keys: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("lock authorized_keys: timed out after %s", lockTimeout)
	}
	return func() { _ = fileLock.Unlock() }, nil
}
