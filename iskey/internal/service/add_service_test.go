// 本文件覆盖 add 服务的多密钥、URL 目标、认证、变量替换与 dry-run。
package service

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rocuae/importsshkey/internal/config"
	"github.com/rocuae/importsshkey/internal/manager"
)

const (
	svcKey1 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHrbxLszDHYgw5fKNqIisVPC7zt0DLCzrJ2U9UTUbyKT one@example.com"
	svcKey2 = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAYu05msHfRoQNjzFqs1E4fX0gi4moAtp8F0BnFpfToV two@example.com"
)

func TestAddURLTargetImportsAllKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(svcKey1 + "\n" + svcKey2 + "\n"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "authorized_keys")
	mgr := manager.NewManager(path)
	svc := NewAddService(&config.Config{}, mgr)

	result, err := svc.Run(context.Background(), AddOptions{Source: server.URL + "/keys", User: "alice"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	entries, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(entries))
	}
}

func TestAddURLTargetRequiresUser(t *testing.T) {
	svc := NewAddService(&config.Config{}, manager.NewManager(filepath.Join(t.TempDir(), "authorized_keys")))
	if _, err := svc.Run(context.Background(), AddOptions{Source: "https://example.com/keys"}); err == nil {
		t.Fatal("expected error without user")
	}
}

func TestAddCustomSourceWithBasicAuthAndVars(t *testing.T) {
	const user, pass = "bob", "s3cret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keys.pub" {
			http.NotFound(w, r)
			return
		}
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
		if r.Header.Get("Authorization") != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(svcKey1 + "\n"))
	}))
	defer server.Close()

	t.Setenv("ISKEY_TEST_USER", user)
	t.Setenv("ISKEY_TEST_PASS", pass)

	cfg := &config.Config{
		Credentials: map[string]config.Credential{
			"private": {
				Type:            "basic",
				UsernameFromEnv: "ISKEY_TEST_USER",
				PasswordFromEnv: "ISKEY_TEST_PASS",
			},
		},
		Sources: map[string]config.SourceConfig{
			"private": {
				Alias:   "work",
				URL:     server.URL + "/{{ .File }}",
				Format:  "plaintext",
				AuthRef: "private",
			},
		},
	}

	path := filepath.Join(t.TempDir(), "authorized_keys")
	mgr := manager.NewManager(path)
	svc := NewAddService(cfg, mgr)

	result, err := svc.Run(context.Background(), AddOptions{
		Source: "work",
		User:   "alice",
		Vars:   map[string]string{"File": "keys.pub"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Source != "private" || result.Imported != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAddDryRunDoesNotWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(svcKey1 + "\n"))
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "authorized_keys")
	svc := NewAddService(&config.Config{}, manager.NewManager(path))
	result, err := svc.Run(context.Background(), AddOptions{Source: server.URL, User: "alice", DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Action != "dry-run" || len(result.Keys) != 1 || result.Keys[0].Action != "would-add" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run created file: %v", err)
	}
}

func TestAddReportsMissingCredentialEnv(t *testing.T) {
	cfg := &config.Config{
		Credentials: map[string]config.Credential{
			"missing": {Type: "bearer", ValueFromEnv: "ISKEY_TEST_MISSING_TOKEN"},
		},
		Sources: map[string]config.SourceConfig{
			"private": {URL: "https://example.com/{{ .User }}", Format: "plaintext", AuthRef: "missing"},
		},
	}
	os.Unsetenv("ISKEY_TEST_MISSING_TOKEN")
	svc := NewAddService(cfg, manager.NewManager(filepath.Join(t.TempDir(), "authorized_keys")))
	_, err := svc.Run(context.Background(), AddOptions{Source: "private", User: "alice"})
	if err == nil || !strings.Contains(err.Error(), "ISKEY_TEST_MISSING_TOKEN") {
		t.Fatalf("expected missing env error, got %v", err)
	}
}
