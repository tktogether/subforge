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

// TestHandleAPIRuleSourcesNoManifest 覆盖未配置 manifest 时仍返回 200 + exists=false + 非空 hint。
func TestHandleAPIRuleSourcesNoManifest(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil)
	srv.handleAPIRuleSources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["exists"] != false {
		t.Fatalf("exists = %v, want false", body["exists"])
	}
	hint, _ := body["hint"].(string)
	if hint == "" {
		t.Fatalf("hint 不应为空, body=%v", body)
	}
	tpl, _ := body["manifest_template"].(string)
	if tpl == "" {
		t.Fatalf("manifest_template 不应为空")
	}
	if body["manifest_path"] != body["default_manifest_path"] {
		t.Fatalf("manifest_path 应等于 default_manifest_path, body=%v", body)
	}
	if _, ok := body["output_dir"]; !ok {
		t.Fatalf("应含 output_dir, body=%v", body)
	}
	sources, _ := body["sources"].([]any)
	if len(sources) != 0 {
		t.Fatalf("sources 应为空, got %v", body["sources"])
	}
}

// TestHandleAPIRuleSourcesDefaultPathDiffersFromConfigured 覆盖配置了自定义 manifest 路径时 default_manifest_path 仍为默认路径。
func TestHandleAPIRuleSourcesDefaultPathDiffersFromConfigured(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	custom := filepath.Join(t.TempDir(), "custom.yaml")
	seedConfig(t, mgr, subforge.RuleManifestPathKey, custom)
	rec := httptest.NewRecorder()
	srv.handleAPIRuleSources(rec, httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["manifest_path"] != custom {
		t.Fatalf("manifest_path = %v, want %q", body["manifest_path"], custom)
	}
	wantDefault := filepath.Join(mgr.DataDir(), "manifest.yaml")
	if body["default_manifest_path"] != wantDefault {
		t.Fatalf("default_manifest_path = %v, want %q", body["default_manifest_path"], wantDefault)
	}
	if body["manifest_path"] == body["default_manifest_path"] {
		t.Fatalf("配置自定义路径后两字段不应相等, body=%v", body)
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

// TestHandleAPIRuleSourcesInit503 覆盖无订阅管理器时 init 返回 503。
func TestHandleAPIRuleSourcesInit503(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/rule-sources/init", nil)
	srv.handleAPIRuleSourcesInit(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

// TestHandleAPIRuleSourcesInit405 覆盖非 POST 返回 405。
func TestHandleAPIRuleSourcesInit405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources/init", nil)
	srv.handleAPIRuleSourcesInit(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

// TestHandleAPIRuleSourcesInitOK 覆盖 init 创建默认清单后 GET 变 exists=true。
func TestHandleAPIRuleSourcesInitOK(t *testing.T) {
	srv, _ := newCfgTestServer(t)

	rec := httptest.NewRecorder()
	srv.handleAPIRuleSourcesInit(rec, httptest.NewRequest(http.MethodPost, "/api/rule-sources/init", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("init code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var initBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &initBody); err != nil {
		t.Fatal(err)
	}
	if initBody["created"] != true {
		t.Fatalf("created = %v, want true", initBody["created"])
	}
	path, _ := initBody["path"].(string)
	if path == "" {
		t.Fatal("path 不应为空")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("生成的文件不存在: %v", err)
	}

	rec2 := httptest.NewRecorder()
	srv.handleAPIRuleSources(rec2, httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("get code = %d, body=%s", rec2.Code, rec2.Body.String())
	}
	var getBody map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &getBody); err != nil {
		t.Fatal(err)
	}
	if getBody["exists"] != true {
		t.Fatalf("exists = %v, want true", getBody["exists"])
	}
	sources, _ := getBody["sources"].([]any)
	if len(sources) != 2 {
		t.Fatalf("sources 长度 = %d, want 2（模板里的源数）", len(sources))
	}
}

// TestHandleAPIRuleSourcesInitExistingAndForce 覆盖已存在时 created=false、?force=1 时 created=true。
func TestHandleAPIRuleSourcesInitExistingAndForce(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	srv.handleAPIRuleSourcesInit(rec, httptest.NewRequest(http.MethodPost, "/api/rule-sources/init", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("首次 init code = %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	srv.handleAPIRuleSourcesInit(rec2, httptest.NewRequest(http.MethodPost, "/api/rule-sources/init", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("二次 init code = %d", rec2.Code)
	}
	var body2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &body2); err != nil {
		t.Fatal(err)
	}
	if body2["created"] != false {
		t.Fatalf("已存在时 created = %v, want false", body2["created"])
	}

	rec3 := httptest.NewRecorder()
	srv.handleAPIRuleSourcesInit(rec3, httptest.NewRequest(http.MethodPost, "/api/rule-sources/init?force=1", nil))
	if rec3.Code != http.StatusOK {
		t.Fatalf("force init code = %d", rec3.Code)
	}
	var body3 map[string]any
	if err := json.Unmarshal(rec3.Body.Bytes(), &body3); err != nil {
		t.Fatal(err)
	}
	if body3["created"] != true {
		t.Fatalf("force 时 created = %v, want true", body3["created"])
	}
	if _, err := os.Stat(body3["path"].(string) + ".bak"); err != nil {
		t.Fatalf("force 应产生 .bak: %v", err)
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
