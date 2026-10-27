// 本文件覆盖 authorized_keys 的解析、批量添加、dry-run、替换与删除。
package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocuae/importsshkey/internal/domain"
)

const (
	testKey1 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHrbxLszDHYgw5fKNqIisVPC7zt0DLCzrJ2U9UTUbyKT one@example.com"
	testKey2 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAYu05msHfRoQNjzFqs1E4fX0gi4moAtp8F0BnFpfToV two@example.com"
	testKey3 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAi48sTS1ulLGTbnPA4NKpaW9Npf3g7V+yFqKQjpfT6O three@example.com"
)

func mustEntry(t *testing.T, key, source, user string) *domain.KeyEntry {
	t.Helper()
	entry, err := domain.NewKeyEntry(key, source, user)
	if err != nil {
		t.Fatalf("NewKeyEntry: %v", err)
	}
	return entry
}

func TestAddManyImportsAllKeysAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")
	m := NewManager(path)

	entries := []*domain.KeyEntry{
		mustEntry(t, testKey1, "work", "alice"),
		mustEntry(t, testKey2, "work", "alice"),
	}
	outcomes, err := m.AddMany(entries, false, false)
	if err != nil {
		t.Fatalf("AddMany: %v", err)
	}
	if len(outcomes) != 2 || outcomes[0].Action != "added" || outcomes[1].Action != "added" {
		t.Fatalf("unexpected outcomes: %+v", outcomes)
	}
	if outcomes[0].Line != 2 || outcomes[1].Line != 4 {
		t.Fatalf("unexpected line numbers: %+v", outcomes)
	}

	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 managed keys, got %d", len(loaded))
	}

	outcomes, err = m.AddMany(entries, false, false)
	if err != nil {
		t.Fatalf("AddMany again: %v", err)
	}
	for _, outcome := range outcomes {
		if outcome.Action != "skipped" {
			t.Fatalf("expected skipped, got %+v", outcome)
		}
	}
}

func TestAddManyDryRunDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")
	m := NewManager(path)

	outcomes, err := m.AddMany([]*domain.KeyEntry{mustEntry(t, testKey1, "work", "alice")}, false, true)
	if err != nil {
		t.Fatalf("AddMany: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Action != "would-add" {
		t.Fatalf("unexpected outcomes: %+v", outcomes)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run created file: %v", err)
	}
}

func TestForceReplacesGroupAndPreservesNonManagedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	nonManaged := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQExample nonmanaged@example.com"
	if err := os.WriteFile(path, []byte(nonManaged+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(path)

	if _, err := m.AddMany([]*domain.KeyEntry{
		mustEntry(t, testKey1, "work", "alice"),
		mustEntry(t, testKey2, "work", "alice"),
		mustEntry(t, testKey3, "work", "bob"),
	}, false, false); err != nil {
		t.Fatalf("seed AddMany: %v", err)
	}

	outcomes, err := m.AddMany([]*domain.KeyEntry{mustEntry(t, testKey1, "work", "alice")}, true, false)
	if err != nil {
		t.Fatalf("force AddMany: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Action != "replaced" {
		t.Fatalf("unexpected outcomes: %+v", outcomes)
	}

	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected alice replaced + bob kept, got %+v", loaded)
	}
	if loaded[0].User != "bob" || loaded[1].User != "alice" {
		t.Fatalf("unexpected users: %+v", loaded)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), nonManaged) {
		t.Fatalf("non-managed line was lost:\n%s", data)
	}
	if strings.Contains(string(data), testKey2) {
		t.Fatalf("old alice key was not removed:\n%s", data)
	}
	if strings.Count(string(data), testKey1) != 1 {
		t.Fatalf("expected exactly one replacement key:\n%s", data)
	}
}

func TestRemoveByTargetDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")
	m := NewManager(path)
	if _, err := m.AddMany([]*domain.KeyEntry{
		mustEntry(t, testKey1, "work", "alice"),
		mustEntry(t, testKey2, "home", "alice"),
	}, false, false); err != nil {
		t.Fatalf("AddMany: %v", err)
	}

	removed, err := m.RemoveByTarget("work", "alice", "", true)
	if err != nil {
		t.Fatalf("RemoveByTarget dry-run: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 key, got %d", len(removed))
	}
	loaded, _ := m.Load()
	if len(loaded) != 2 {
		t.Fatalf("dry-run changed file: %+v", loaded)
	}

	removed, err = m.RemoveByTarget("work", "alice", "", false)
	if err != nil {
		t.Fatalf("RemoveByTarget: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 key, got %d", len(removed))
	}
	loaded, _ = m.Load()
	if len(loaded) != 1 || loaded[0].Source != "home" {
		t.Fatalf("unexpected remaining keys: %+v", loaded)
	}
}

func TestRemoveByFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")
	m := NewManager(path)
	entry := mustEntry(t, testKey1, "work", "alice")
	if _, err := m.AddMany([]*domain.KeyEntry{entry}, false, false); err != nil {
		t.Fatalf("AddMany: %v", err)
	}

	removed, err := m.RemoveByTarget("", "", entry.Fingerprint, false)
	if err != nil {
		t.Fatalf("RemoveByTarget: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 key, got %d", len(removed))
	}
}

func TestLoadErrorDoesNotTruncateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	m := NewManager(path)
	if _, err := m.AddMany([]*domain.KeyEntry{mustEntry(t, testKey1, "work", "alice")}, false, false); err == nil {
		t.Fatal("expected error when authorized_keys is a directory")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("path was modified: %v %v", info, err)
	}
}

func TestLoadAllowsBlankLineBetweenCommentAndKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")
	entry := mustEntry(t, testKey1, "work", "alice")
	content := "# iskey:work:alice:" + entry.Fingerprint + "\n\n" + testKey1 + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewManager(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Fingerprint != entry.Fingerprint {
		t.Fatalf("unexpected entries: %+v", loaded)
	}
}
