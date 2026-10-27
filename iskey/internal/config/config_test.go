// 本文件覆盖配置加载、环境变量展开、凭据解析与稳定序列化。
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoaderExpandsEnvWithoutBreakingYAML(t *testing.T) {
	t.Setenv("ISKEY_TEST_BASE", "http://example.com:8080")
	path := filepath.Join(t.TempDir(), "sources.yaml")
	yml := `sources:
  work:
    url: "$ISKEY_TEST_BASE/keys/{{ .User }}"
    format: plaintext
`
	if err := os.WriteFile(path, []byte(yml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewLoader(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := "http://example.com:8080/keys/{{ .User }}"
	if cfg.Sources["work"].URL != want {
		t.Fatalf("got %q, want %q", cfg.Sources["work"].URL, want)
	}
}

func TestLoadOrDefaultMissingFile(t *testing.T) {
	cfg, err := NewLoader(filepath.Join(t.TempDir(), "missing.yaml")).LoadOrDefault()
	if err != nil {
		t.Fatalf("LoadOrDefault: %v", err)
	}
	if cfg.Defaults.Timeout != 10 || cfg.Defaults.MaxRetries != 3 {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}

	_, err = NewLoader(filepath.Join(t.TempDir(), "missing.yaml")).Load()
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestResolveCredentialOnlyReferenced(t *testing.T) {
	t.Setenv("ISKEY_TEST_TOKEN", "secret")
	cfg := &Config{
		Credentials: map[string]Credential{
			"token":  {Type: "bearer", ValueFromEnv: "ISKEY_TEST_TOKEN"},
			"unused": {Type: "bearer", ValueFromEnv: "ISKEY_TEST_UNSET"},
		},
	}
	cred, err := cfg.ResolveCredential("token")
	if err != nil {
		t.Fatalf("ResolveCredential: %v", err)
	}
	if cred.ResolvedValue != "secret" {
		t.Fatalf("unexpected token: %q", cred.ResolvedValue)
	}
	if _, err := cfg.ResolveCredential("missing"); err == nil {
		t.Fatal("expected error for unknown credential")
	}
}

func TestSaveToFileIsStableAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sources.yaml")
	cfg := &Config{
		Defaults: Defaults{Output: "~/authorized_keys", Timeout: 15, MaxRetries: 2},
		Sources: map[string]SourceConfig{
			"zeta":  {Alias: "z", URL: "https://z.example.com/keys/{{ .User }}", Format: "plaintext"},
			"alpha": {Alias: "a", URL: "https://a.example.com/keys/{{ .User }}", Format: "plaintext"},
		},
	}
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "alpha") || !strings.Contains(string(first), "zeta") {
		t.Fatalf("missing sources:\n%s", first)
	}
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile second: %v", err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatalf("output is not stable:\n%s\n---\n%s", first, second)
	}
	loaded, err := NewLoader(path).Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Sources["alpha"].URL != cfg.Sources["alpha"].URL {
		t.Fatalf("roundtrip mismatch: %+v", loaded.Sources["alpha"])
	}
}

func TestValidateRejectsUnknownAuthRef(t *testing.T) {
	cfg := &Config{Sources: map[string]SourceConfig{
		"work": {URL: "https://example.com/{{ .User }}", Format: "plaintext", AuthRef: "missing"},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateAllowsAutoFormat(t *testing.T) {
	cfg := &Config{Sources: map[string]SourceConfig{
		"work": {URL: "https://example.com/keys", Format: "auto"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
