// 本文件覆盖公钥解析、指纹计算、注释格式与目标校验。
package domain

import (
	"strings"
	"testing"
)

const domainTestKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHrbxLszDHYgw5fKNqIisVPC7zt0DLCzrJ2U9UTUbyKT one@example.com"

func TestNewKeyEntry(t *testing.T) {
	entry, err := NewKeyEntry(domainTestKey, "work", "alice")
	if err != nil {
		t.Fatalf("NewKeyEntry: %v", err)
	}
	if entry.Source != "work" || entry.User != "alice" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if !strings.HasPrefix(entry.Fingerprint, "SHA256:") || !strings.HasSuffix(entry.Fingerprint, "=") {
		t.Fatalf("unexpected fingerprint: %s", entry.Fingerprint)
	}
	if entry.Comment != "iskey:work:alice:"+entry.Fingerprint {
		t.Fatalf("unexpected comment: %s", entry.Comment)
	}

	source, user, fingerprint, ok := ParseComment(entry.Comment)
	if !ok {
		t.Fatal("ParseComment failed")
	}
	if source != "work" || user != "alice" || fingerprint != entry.Fingerprint {
		t.Fatalf("unexpected parsed comment: %s %s %s", source, user, fingerprint)
	}
}

func TestNewKeyEntryRejectsInvalidTarget(t *testing.T) {
	cases := []struct{ source, user string }{
		{"", "alice"},
		{"work", ""},
		{"bad source", "alice"},
		{"work", "bad user"},
		{"work", "alice:root"},
	}
	for _, tc := range cases {
		if _, err := NewKeyEntry(domainTestKey, tc.source, tc.user); err == nil {
			t.Fatalf("expected error for source=%q user=%q", tc.source, tc.user)
		}
	}
}

func TestNewKeyEntryRejectsInvalidKey(t *testing.T) {
	if _, err := NewKeyEntry("not-a-key", "work", "alice"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTarget(t *testing.T) {
	if err := ValidateTarget("gh", "octocat"); err != nil {
		t.Fatalf("valid target rejected: %v", err)
	}
	if err := ValidateTarget("gh", "alice@example.com"); err != nil {
		t.Fatalf("email-like user rejected: %v", err)
	}
}
