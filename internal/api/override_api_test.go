package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
)

func TestHandleAPIOverride503(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/override", nil)
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

func TestHandleAPIOverride405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/override", nil)
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestHandleAPIOverrideGetOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "override.yaml")
	if err := os.WriteFile(p, []byte("groups: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, overridePathKey, p)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/override", nil)
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["path"] != p {
		t.Fatalf("path = %v", body["path"])
	}
	content, _ := body["content"].(string)
	if !strings.Contains(content, "groups") {
		t.Fatalf("content = %q", content)
	}
}

func TestHandleAPIOverrideGet404(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedConfig(t, mgr, overridePathKey, filepath.Join(t.TempDir(), "missing.yaml"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/override", nil)
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestHandleAPIOverridePutYAML(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "override.yaml")
	if err := os.WriteFile(p, []byte("groups: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, overridePathKey, p)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{"content": "groups: []\nproxies: []\n"})
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleAPIOverridePutSyntaxError(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "override.yaml")
	if err := os.WriteFile(p, []byte("groups: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, overridePathKey, p)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{"content": "groups: [\n"})
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "override 语法错误") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleAPIOverridePutNoPath(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{"content": "groups: []\n"})
	srv.handleAPIOverride(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}
