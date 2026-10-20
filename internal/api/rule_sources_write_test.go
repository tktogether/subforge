package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/subforge"
)

const seedManifestYAML = "sources:\n  - name: base\n    url: http://example.com/base.yaml\noutput:\n  format: clash-meta\n  path: ./rules\n"

// newWriteTestServer 复用 newCfgTestServer，但把 Config.Timeout 改成真实时长：
// newCfgTestServer 的 Timeout: 5 是 5ns，经 Routes() 的 chimw.Timeout 会立刻
// context deadline exceeded，只适合直接调 handler 的旧测试。
func newWriteTestServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	srv, mgr := newCfgTestServer(t)
	srv.cfg.Timeout = 5 * time.Second
	return srv, mgr
}

func seedWriteManifest(t *testing.T, mgr *subforge.Manager) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	seedConfig(t, mgr, subforge.RuleManifestPathKey, path)
	if err := os.WriteFile(path, []byte(seedManifestYAML), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return path
}

func postJSON(t *testing.T, srv *Server, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

// TestRuleSourcesUpdateAllFailedKeepsPerSourceErrors 全部源更新失败时，500 响应体里
// 必须仍然带上逐源结果：manifest 只回一句「所有 N 个 source 更新失败」，真正能
// 定位问题的原因（哪个源、上游报了什么）全在 results 里。丢掉它，界面上就只剩
// 一句笼统的失败，用户无从下手。
func TestRuleSourcesUpdateAllFailedKeepsPerSourceErrors(t *testing.T) {
	srv, mgr := newWriteTestServer(t)
	path := seedWriteManifest(t, mgr)
	// 127.0.0.1:1 是必然连不上的地址，用来稳定地制造"全部源失败"。
	unreachable := "sources:\n  - name: base\n    url: http://127.0.0.1:1/base.yaml\noutput:\n  format: clash-meta\n  path: ./rules\n"
	if err := os.WriteFile(path, []byte(unreachable), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	rec := postJSON(t, srv, "/api/rule-sources/update", "{}")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Error   string `json:"error"`
		Results []struct {
			Source string `json:"source"`
			Err    string `json:"err"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应: %v（body=%s）", err, rec.Body.String())
	}
	if got.Error == "" {
		t.Error("响应应带 error 字段")
	}
	if len(got.Results) != 1 {
		t.Fatalf("results 长度 = %d, want 1（逐源结果不能丢）: %s", len(got.Results), rec.Body.String())
	}
	if got.Results[0].Source != "base" {
		t.Errorf("source = %q, want base", got.Results[0].Source)
	}
	if got.Results[0].Err == "" {
		t.Errorf("失败源的 err 不能为空: %s", rec.Body.String())
	}
}

func TestRuleSourcesAdd503(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := postJSON(t, srv, "/api/rule-sources", "{\"url\":\"http://x/a.yaml\"}")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

func TestRuleSourcesAdd405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil)
	srv.handleAPIRuleSourcesAdd(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestRuleSourcesAddRemoveRoundTrip(t *testing.T) {
	srv, mgr := newWriteTestServer(t)
	path := seedWriteManifest(t, mgr)

	rec := postJSON(t, srv, "/api/rule-sources", "{\"name\":\"foo\",\"url\":\"http://example.com/foo.yaml\"}")
	if rec.Code != http.StatusOK {
		t.Fatalf("add code = %d, body=%s", rec.Code, rec.Body.String())
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "name: foo") {
		t.Fatalf("manifest missing foo: %s", string(b))
	}

	rec = postJSON(t, srv, "/api/rule-sources", "{\"name\":\"foo\",\"url\":\"http://example.com/foo2.yaml\"}")
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup code = %d, want 409", rec.Code)
	}

	rec = postJSON(t, srv, "/api/rule-sources/remove", "{\"name\":\"foo\"}")
	if rec.Code != http.StatusOK {
		t.Fatalf("remove code = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("missing .bak backup: %v", err)
	}

	rec = postJSON(t, srv, "/api/rule-sources/remove", "{\"name\":\"foo\"}")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("remove-missing code = %d, want 404", rec.Code)
	}

	rec = postJSON(t, srv, "/api/rule-sources/remove", "{\"name\":\"base\"}")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("remove-last code = %d, want 400", rec.Code)
	}
}

func TestRuleSourcesUpdateOneRoundTrip(t *testing.T) {
	srv, mgr := newWriteTestServer(t)
	path := seedWriteManifest(t, mgr)

	upd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies: []\nrules: []\n"))
	}))
	defer upd.Close()

	b, _ := os.ReadFile(path)
	nb := strings.Replace(string(b), "http://example.com/base.yaml", upd.URL+"/base.yaml", 1)
	if err := os.WriteFile(path, []byte(nb), 0o644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}

	rec := postJSON(t, srv, "/api/rule-sources/update-one", "{\"name\":\"base\"}")
	if rec.Code != http.StatusOK {
		t.Fatalf("update-one code = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"results\"") {
		t.Fatalf("update-one body missing results: %s", rec.Body.String())
	}

	rec = postJSON(t, srv, "/api/rule-sources/update-one", "{\"name\":\"nope\"}")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update-one missing code = %d, want 404", rec.Code)
	}
}
