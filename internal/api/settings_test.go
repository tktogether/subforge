package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/subforge"
)

func newCfgTestServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(t.TempDir() + "/subforge.db")
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	srv.SetSubManager(mgr)
	return srv, mgr
}

func TestHandleAPISettings503(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

func TestHandleAPISettings405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/settings", nil)
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestHandleAPISettingsGet(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	settings, ok := body["settings"].([]any)
	if !ok || settings == nil {
		t.Fatal("settings should not be null")
	}
	if len(settings) == 0 {
		t.Fatal("settings should not be empty")
	}
}

func TestHandleAPISettingsPutNested(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{
		"settings": map[string]string{"script.enabled": "true"},
	})
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	v, err := mgr.GetConfig(context.Background(), "script.enabled")
	if err != nil || v != "true" {
		t.Fatalf("script.enabled = %q, err=%v", v, err)
	}
}

func TestHandleAPISettingsPutFlat(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{
		"subscheck.enabled": "false",
	})
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	v, _ := mgr.GetConfig(context.Background(), "subscheck.enabled")
	if v != "false" {
		t.Fatalf("subscheck.enabled = %q", v)
	}
}

func TestHandleAPISettingsPutUnknownKey(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{
		"settings": map[string]string{"no.such.key": "x"},
	})
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestHandleAPISettingsPutBadBool(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPut, map[string]any{
		"settings": map[string]string{"script.enabled": "maybe"},
	})
	srv.handleAPISettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func mustJSONReq(t *testing.T, method string, v any) *http.Request {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, "/", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	return r
}
