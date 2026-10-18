package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/subforge"
)

func importTestServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(filepath.Join(t.TempDir(), "subforge.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	cfg := Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	}
	srv := New(cfg)
	srv.SetSubManager(mgr)
	return srv, mgr
}

func addImportAirport(t *testing.T, mgr *subforge.Manager, name, url string) int64 {
	t.Helper()
	a, err := mgr.AddAirport(t.Context(), name, url, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	return a.ID
}

func TestImportRules_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/airports/1/import-rules", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET 状态码 = %d，应为 405", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "仅支持 POST" {
		t.Fatalf("error = %q，应为 仅支持 POST", e.Error)
	}
}

func TestImportRules_NoManager503(t *testing.T) {
	srv := New(Config{
		Engine:  &convert.Engine{Profile: render.Profile{Name: "test", Version: "test-version"}},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/airports/1/import-rules", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("未注入 manager 状态码 = %d，应为 503", rr.Code)
	}
}

func TestImportRules_AirportNotFound404(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/airports/99999/import-rules", nil)
	req.SetPathValue("id", "99999")
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("不存在机场状态码 = %d，应为 404", rr.Code)
	}
}

func TestImportRules_FetchFailure502(t *testing.T) {
	srv, mgr := importTestServer(t)
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()
	id := addImportAirport(t, mgr, "dead", dead.URL)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("拉取失败状态码 = %d，应为 502", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "导入失败") {
		t.Fatalf("body 应含 导入失败，got=%s", rr.Body.String())
	}
}

func TestImportRules_Success(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	}))
	defer up.Close()
	id := addImportAirport(t, mgr, "ok", up.URL)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("成功状态码 = %d，应为 200；body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		RulesAdded     int      `json:"rules_added"`
		RulesSkipped   int      `json:"rules_skipped"`
		ProvidersAdded int      `json:"providers_added"`
		Warnings       []string `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if resp.Warnings == nil {
		t.Fatalf("warnings 必须是非 nil 数组")
	}
}

func TestImportRules_IdempotentSecondImportSkips(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	}))
	defer up.Close()
	id := addImportAirport(t, mgr, "idem", up.URL)
	url := fmt.Sprintf("/api/airports/%d/import-rules", id)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.SetPathValue("id", fmt.Sprintf("%d", id))
		rr := httptest.NewRecorder()
		srv.handleAPIImportRules(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("第 %d 次导入状态码 = %d", i+1, rr.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	var resp struct {
		Skipped int `json:"rules_skipped"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	_ = resp
}
