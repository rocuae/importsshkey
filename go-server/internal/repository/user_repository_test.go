// 本文件覆盖多公钥仓储的幂等写入、替换、删除与分页查询。
package repository_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/rocuae/importsshkey/go-server/internal/model"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.UserKey{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func key(username, fingerprint, source string) *model.UserKey {
	return &model.UserKey{
		Username:    username,
		PublicKey:   "ssh-ed25519 AAAA" + fingerprint,
		Fingerprint: fingerprint,
		Source:      source,
	}
}

func TestAddUpdatesExistingFingerprint(t *testing.T) {
	repo := repository.NewGormUserKeyRepository(newTestDB(t))

	if err := repo.Add(key("alice", "SHA256:one", "github")); err != nil {
		t.Fatal(err)
	}
	if err := repo.Add(key("alice", "SHA256:one", "work")); err != nil {
		t.Fatal(err)
	}

	keys, err := repo.ListByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].Source != "work" {
		t.Fatalf("expected source updated, got %q", keys[0].Source)
	}
}

func TestReplaceAndDelete(t *testing.T) {
	repo := repository.NewGormUserKeyRepository(newTestDB(t))

	if err := repo.AddMany([]*model.UserKey{
		key("alice", "SHA256:one", "github"),
		key("alice", "SHA256:two", "github"),
	}); err != nil {
		t.Fatal(err)
	}

	if err := repo.Replace("alice", []*model.UserKey{key("alice", "SHA256:three", "work")}); err != nil {
		t.Fatal(err)
	}
	keys, err := repo.ListByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Fingerprint != "SHA256:three" {
		t.Fatalf("unexpected keys after replace: %+v", keys)
	}

	removed, err := repo.DeleteByFingerprint("alice", "SHA256:three")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	exists, err := repo.Exists("alice")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected user removed")
	}
}

func TestListUsersPaginationAndSearch(t *testing.T) {
	repo := repository.NewGormUserKeyRepository(newTestDB(t))

	for _, name := range []string{"alice", "bob", "carol"} {
		if err := repo.Add(key(name, "SHA256:"+name, "github")); err != nil {
			t.Fatal(err)
		}
	}

	users, total, err := repo.ListUsers("", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(users) != 2 {
		t.Fatalf("expected total=3 page=2, got total=%d len=%d", total, len(users))
	}

	users, total, err = repo.ListUsers("bo", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(users) != 1 || users[0] != "bob" {
		t.Fatalf("unexpected search result: %+v total=%d", users, total)
	}
}

func TestAuditLogRecent(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewGormAuditLogRepository(db)

	for _, action := range []string{"add", "replace", "delete"} {
		if err := repo.Create(&model.AuditLog{Username: "alice", Action: action}); err != nil {
			t.Fatal(err)
		}
	}

	logs, err := repo.ListRecent(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].Action != "delete" {
		t.Fatalf("unexpected recent logs: %+v", logs)
	}
}
