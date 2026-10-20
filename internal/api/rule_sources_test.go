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
	"github.com/tktogether/subforge/internal/subforge"
)

// clashRuleSetYAML 是一份最小但合法的 Clash 规则集，可被转换成 domain behavior 的 MRS。
const clashRuleSetYAML = "payload:\n  - DOMAIN-SUFFIX,example.com\n"

func seedManifest(t *testing.T, mgr *subforge.Manager) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ruleDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 内容必须是合法的 Clash 规则集（payload 为规则列表），否则生成 MRS 时会解析失败。
	if err := os.WriteFile(filepath.Join(ruleDir, "foo-20240101.yaml"), []byte(clashRuleSetYAML), 0o644); err != nil {
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

// seedRuleVersions 造一个只有 foo 源的 manifest 和若干已存在的版本文件，返回规则输出目录。
func seedRuleVersions(t *testing.T, mgr *subforge.Manager, format string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	ruleDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ruleDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mf := filepath.Join(dir, "manifest.yaml")
	body := "sources:\n  - name: foo\n    url: http://example.com/foo.yaml\noutput:\n  format: " + format + "\n  path: " + filepath.ToSlash(ruleDir) + "\n"
	if err := os.WriteFile(mf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, mgr, subforge.RuleManifestPathKey, mf)
	return ruleDir
}

// TestRuleSourceMRSFormat 覆盖「由扩展名推导源格式」的决策表。
func TestRuleSourceMRSFormat(t *testing.T) {
	cases := []struct {
		ext      string
		explicit string
		want     string
		wantErr  bool
	}{
		{ext: ".yaml", want: "yaml"},
		{ext: ".yml", want: "yaml"},
		{ext: ".txt", want: "text"},
		{ext: ".mrs", want: "mrs"},
		{ext: ".json", wantErr: true},
		{ext: "", wantErr: true},
		{ext: ".yaml", explicit: "text", want: "text"},
		{ext: ".txt", explicit: "mrs", want: "mrs"},
		{ext: ".yaml", explicit: "bogus", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ruleSourceMRSFormat(tc.ext, tc.explicit)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ext=%q explicit=%q 应报错, got %q", tc.ext, tc.explicit, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ext=%q explicit=%q: %v", tc.ext, tc.explicit, err)
		}
		if got != tc.want {
			t.Fatalf("ext=%q explicit=%q = %q, want %q", tc.ext, tc.explicit, got, tc.want)
		}
	}
}

// TestHandleAPIRuleSourcesMRSOK 覆盖 .yaml Clash 规则集在 format 缺省时能生成 MRS。
func TestHandleAPIRuleSourcesMRSOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	ruleDir := seedRuleVersions(t, mgr, "clash-meta", map[string]string{"foo-20240101.yaml": clashRuleSetYAML})
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "foo", "behavior": "domain"})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	path, _ := body["path"].(string)
	if filepath.Base(path) != "foo-20240101.mrs" {
		t.Fatalf("path = %q, want 目标文件名为当前版本去扩展名 + .mrs", path)
	}
	if !strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(ruleDir)+"/") {
		t.Fatalf("path %q 不在规则输出目录 %q 下", path, ruleDir)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("生成文件不存在: %v", err)
	}
	if info.Size() <= 0 {
		t.Fatalf("生成文件大小 = %d, want > 0", info.Size())
	}
	if n, _ := body["bytes"].(float64); int64(n) != info.Size() {
		t.Fatalf("bytes = %v, want %d", body["bytes"], info.Size())
	}
}

// TestHandleAPIRuleSourcesMRSPlainTextOK 覆盖 .txt 源按扩展名推导成 text 格式后仍可生成 MRS。
func TestHandleAPIRuleSourcesMRSPlainTextOK(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedRuleVersions(t, mgr, "text", map[string]string{"foo-20240101.txt": "DOMAIN-SUFFIX,example.com\n"})
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "foo", "behavior": "domain"})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(body["path"].(string)) != "foo-20240101.mrs" {
		t.Fatalf("path = %v, want foo-20240101.mrs", body["path"])
	}
}

// TestHandleAPIRuleSourcesMRSUnsupportedExt400 覆盖无法推导源格式的扩展名返回 400 而不是 500。
func TestHandleAPIRuleSourcesMRSUnsupportedExt400(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedRuleVersions(t, mgr, "clash-json", map[string]string{"foo-20240101.json": "{\"payload\":[\"DOMAIN-SUFFIX,example.com\"]}"})
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "foo"})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "format") {
		t.Fatalf("错误信息应提示 format, body=%s", rec.Body.String())
	}
}

// TestHandleAPIRuleSourcesMRSUsesCurrentVersion 覆盖 MRS 以「当前版本」（状态文件指针）为源。
func TestHandleAPIRuleSourcesMRSUsesCurrentVersion(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	ruleDir := seedRuleVersions(t, mgr, "clash-meta", map[string]string{
		"foo-20240101.yaml": clashRuleSetYAML,
		"foo-20240102.yaml": clashRuleSetYAML,
	})
	if err := os.WriteFile(filepath.Join(ruleDir, subforge.CurrentVersionsFile), []byte("{\"foo\":\"foo-20240101.yaml\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "foo", "behavior": "domain"})
	srv.handleAPIRuleSourcesMRS(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(body["path"].(string)) != "foo-20240101.mrs" {
		t.Fatalf("path = %v, want 以当前版本 foo-20240101 为源", body["path"])
	}
}

// TestHandleAPIRuleSourcesRollbackSingleVersion400 覆盖只有 1 个版本时回滚返回 400 + 可回滚版本不足。
func TestHandleAPIRuleSourcesRollbackSingleVersion400(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedRuleVersions(t, mgr, "clash-meta", map[string]string{"foo-20240101.yaml": clashRuleSetYAML})
	rec := httptest.NewRecorder()
	req := mustJSONReq(t, http.MethodPost, map[string]any{"name": "foo"})
	srv.handleAPIRuleSourcesRollback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "可回滚版本不足") {
		t.Fatalf("错误信息应含「可回滚版本不足」, body=%s", rec.Body.String())
	}
}

// TestHandleAPIRuleSourcesGetUsesCurrentVersion 覆盖列表里的 latest 就是状态文件里的当前版本。
func TestHandleAPIRuleSourcesGetUsesCurrentVersion(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	ruleDir := seedRuleVersions(t, mgr, "clash-meta", map[string]string{
		"foo-20240101.yaml": clashRuleSetYAML,
		"foo-20240102.yaml": clashRuleSetYAML,
	})
	if err := os.WriteFile(filepath.Join(ruleDir, subforge.CurrentVersionsFile), []byte("{\"foo\":\"foo-20240101.yaml\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.handleAPIRuleSources(rec, httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Sources []struct {
			Name   string   `json:"name"`
			Latest string   `json:"latest"`
			Versions []string `json:"versions"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(body.Sources))
	}
	if body.Sources[0].Latest != "foo-20240101.yaml" {
		t.Fatalf("latest = %q, want 状态文件指针 foo-20240101.yaml", body.Sources[0].Latest)
	}
}
