package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/subforge"
)

func seedManifest(t *testing.T, mgr *subforge.Manager) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ruleDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, "foo-20240101.yaml"), []byte("payload: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mf := filepath.Join(dir, "manifest.yaml")
	body := "sources:\n  - name: foo\n    url: http://example.com/foo.yaml\noutput:\n  format: clash-meta\n  path: " + ruleDir + "\n"
	if err := os.WriteFile(mf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, subforge.RuleManifestPathKey, mf)
	return ruleDir, mf
}

func TestHandleAPIRuleSources503(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil)
	srv.handleAPIRuleSources(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

func TestHandleAPIRuleSources405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/rule-sources", nil)
	srv.handleAPIRuleSources(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestHandleAPIRuleSourcesNoManifest(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil)
	srv.handleAPIRuleSources(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500", rec.Code)
	}
}

func TestHandleAPIRuleSourcesGetOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedManifest(t, mgr)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil)
	srv.handleAPIRuleSources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	sources, _ := body["sources"].([]any)
	if len(sources) != 1 {
		t.Fatalf("sources = %v", body["sources"])
	}
	src, _ := sources[0].(map[string]any)
	if src["name"] != "foo" {
		t.Fatalf("name = %v", src["name"])
	}
	if src["latest"] != "foo-20240101.yaml" {
		t.Fatalf("latest = %v", src["latest"])
	}
}

func TestHandleAPIRuleSourcesUpdate405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources/update", nil)
	srv.handleAPIRuleSourcesUpdate(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestHandleAPIRuleSourcesUpdateOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedManifest(t, mgr)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/rule-sources/update", nil)
	srv.handleAPIRuleSourcesUpdate(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500 (network unavailable), body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleAPIRuleSourcesRollback400(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": ""})
	srv.handleAPIRuleSourcesRollback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestHandleAPIRuleSourcesRollback404(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedManifest(t, mgr)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "nope"})
	srv.handleAPIRuleSourcesRollback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestHandleAPIRuleSourcesMRS400(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": ""})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestHandleAPIRuleSourcesMRS404(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedManifest(t, mgr)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "nope"})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}

func TestHandleAPIRuleSourcesMRSOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedManifest(t, mgr)
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "foo"})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (invalid input), body=%s", rec.Code, rec.Body.String())
	}
}
