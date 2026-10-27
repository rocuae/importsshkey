// 本文件覆盖多公钥 CRUD、分页、鉴权与错误响应。
package handler_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/rocuae/importsshkey/go-server/internal/config"
	"github.com/rocuae/importsshkey/go-server/internal/handler"
	"github.com/rocuae/importsshkey/go-server/internal/middleware"
	"github.com/rocuae/importsshkey/go-server/internal/model"
	"github.com/rocuae/importsshkey/go-server/internal/repository"
)

func newKey(t *testing.T, comment string) string {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshKey))) + " " + comment
}

func newRouter(t *testing.T, token string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserKey{}, &model.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewGormUserKeyRepository(db)
	audit := repository.NewGormAuditLogRepository(db)
	keys := handler.NewKeyHandler(repo, audit)
	cfg := &config.Config{AdminToken: token}

	r := gin.New()
	r.GET("/keys/:username", keys.GetKey)
	r.GET("/keys/:username/metadata", keys.GetKeyMetadata)
	admin := r.Group("/keys")
	admin.Use(middleware.AdminAuth(cfg))
	admin.PUT("/:username", keys.PutKey)
	admin.POST("/:username", keys.PostKey)
	admin.DELETE("/:username", keys.DeleteKey)
	admin.GET("", keys.ListKeys)
	return r
}

func request(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPutPostGetDeleteFlow(t *testing.T) {
	r := newRouter(t, "secret")
	first := newKey(t, "first")
	second := newKey(t, "second")

	w := request(t, r, http.MethodPut, "/keys/alice", "secret", map[string]any{
		"keys": []map[string]string{
			{"public_key": first, "source": "github"},
			{"public_key": second, "source": "work"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put status %d: %s", w.Code, w.Body.String())
	}
	var putResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &putResp); err != nil {
		t.Fatal(err)
	}
	if putResp["count"].(float64) != 2 || putResp["action"] != "replaced" {
		t.Fatalf("unexpected put response: %+v", putResp)
	}

	w = request(t, r, http.MethodGet, "/keys/alice", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get status %d", w.Code)
	}
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 2 || !strings.HasSuffix(w.Body.String(), "\n") {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}

	third := newKey(t, "third")
	w = request(t, r, http.MethodPost, "/keys/alice", "secret", map[string]any{
		"keys": []map[string]string{{"public_key": third, "source": "laptop"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("post status %d: %s", w.Code, w.Body.String())
	}

	w = request(t, r, http.MethodGet, "/keys/alice/metadata", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("metadata status %d", w.Code)
	}
	var meta struct {
		Count int `json:"count"`
		Keys  []struct {
			Fingerprint string `json:"fingerprint"`
			Source      string `json:"source"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Count != 3 || len(meta.Keys) != 3 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	w = request(t, r, http.MethodDelete, "/keys/alice?fingerprint="+url.QueryEscape(meta.Keys[0].Fingerprint), "secret", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status %d: %s", w.Code, w.Body.String())
	}
	w = request(t, r, http.MethodGet, "/keys/alice/metadata", "", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Count != 2 {
		t.Fatalf("expected 2 keys after delete, got %d", meta.Count)
	}
}

func TestPutRejectsInvalidKeyAndDeduplicates(t *testing.T) {
	r := newRouter(t, "secret")
	valid := newKey(t, "valid")

	w := request(t, r, http.MethodPut, "/keys/alice", "secret", map[string]any{
		"keys": []map[string]string{{"public_key": valid}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put status %d: %s", w.Code, w.Body.String())
	}

	w = request(t, r, http.MethodPut, "/keys/alice", "secret", map[string]any{
		"keys": []map[string]string{{"public_key": valid}, {"public_key": valid}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("dedupe put status %d: %s", w.Code, w.Body.String())
	}
	w = request(t, r, http.MethodGet, "/keys/alice/metadata", "", nil)
	var meta struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &meta)
	if meta.Count != 1 {
		t.Fatalf("expected 1 key, got %d", meta.Count)
	}

	w = request(t, r, http.MethodPut, "/keys/alice", "secret", map[string]any{
		"keys": []map[string]string{{"public_key": "ssh-rsa not-a-key"}},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListKeysPagination(t *testing.T) {
	r := newRouter(t, "secret")
	for _, name := range []string{"alice", "bob", "carol"} {
		w := request(t, r, http.MethodPut, "/keys/"+name, "secret", map[string]any{
			"public_key": newKey(t, name),
		})
		if w.Code != http.StatusOK {
			t.Fatalf("put %s status %d: %s", name, w.Code, w.Body.String())
		}
	}

	w := request(t, r, http.MethodGet, "/keys?page=1&page_size=2", "secret", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list status %d", w.Code)
	}
	var resp struct {
		Total int      `json:"total"`
		Users []string `json:"users"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 || len(resp.Users) != 2 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func TestAdminAuthRejectsAndRateLimits(t *testing.T) {
	r := newRouter(t, "secret")

	w := request(t, r, http.MethodGet, "/keys", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	w = request(t, r, http.MethodGet, "/keys", "wrong", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	for i := 0; i < 12; i++ {
		req := httptest.NewRequest(http.MethodGet, "/keys", nil)
		req.RemoteAddr = "203.0.113.9:1234"
		req.Header.Set("Authorization", "Bearer wrong")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i >= 10 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d: expected 429, got %d", i, w.Code)
		}
	}
}
