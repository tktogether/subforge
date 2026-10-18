package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/subforge"
)

func seedConfig(t *testing.T, mgr *subforge.Manager, key, val string) {
	t.Helper()
	_, err := mgr.DB().Raw().ExecContext(context.Background(),
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, val)
	if err != nil {
		t.Fatalf("seedConfig %s: %v", key, err)
	}
}

func TestHandleAPITemplate503(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	srv.handleAPITemplate(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

func TestHandleAPITemplate405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/template", nil)
	srv.handleAPITemplate(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestHandleAPITemplateGetOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "tpl.ini")
	if err := os.WriteFile(p, []byte("foo=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, templatePathKey, p)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	srv.handleAPITemplate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["path"] != p {
		t.Fatalf("path = %v, want %v", body["path"], p)
	}
	content, _ := body["content"].(string)
	if !strings.Contains(content, "foo=bar") {
		t.Fatalf("content = %q", content)
	}
}

func TestHandleAPITemplateGet404(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedConfig(t, mgr, templatePathKey, filepath.Join(t.TempDir(), "missing.ini"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	srv.handleAPITemplate(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestHandleAPITemplatePutOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "tpl.ini")
	if err := os.WriteFile(p, []byte("foo=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, templatePathKey, p)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{"content": "[General]\nfoo=bar\n"})
	srv.handleAPITemplate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "foo=bar") {
		t.Fatalf("file not updated: %s", string(b))
	}
}

func TestHandleAPITemplatePutSyntaxError(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "tpl.ini")
	if err := os.WriteFile(p, []byte("foo=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, templatePathKey, p)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{"content": "[General\nfoo=bar"})
	srv.handleAPITemplate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "模板语法错误") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
