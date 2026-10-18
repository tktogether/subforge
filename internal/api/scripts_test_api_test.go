package api

import (
	"encoding/json"
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

func scr3TestServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(filepath.Join(t.TempDir(), "scr3.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	srv := New(Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	srv.SetSubManager(mgr)
	return srv, mgr
}

type scriptTestOut struct {
	OK         bool           `json:"ok"`
	Kept       bool           `json:"kept"`
	Before     map[string]any `json:"before"`
	After      map[string]any `json:"after"`
	DurationMs int64          `json:"duration_ms"`
	Error      string         `json:"error"`
}

func runScriptTest(t *testing.T, srv *Server, body string) (int, scriptTestOut, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/scripts/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIScriptsTest(rr, req)
	var out scriptTestOut
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out, rr.Body.String()
}

func TestAPIScriptsTestFilterTrue(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"node.type === 'ss'","kind":"filter","node":{"name":"a","type":"ss","server":"1.1.1.1","port":8388}}`
	code, out, raw := runScriptTest(t, srv, body)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !out.OK || !out.Kept {
		t.Fatalf("ok=%v kept=%v err=%s", out.OK, out.Kept, out.Error)
	}
	if len(out.Before) != 0 || len(out.After) != 0 {
		t.Fatalf("filter 的 before/after 应为空对象: %s", raw)
	}
}

func TestAPIScriptsTestFilterFalse(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"false","kind":"filter"}`
	code, out, raw := runScriptTest(t, srv, body)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !out.OK || out.Kept {
		t.Fatalf("ok=%v kept=%v err=%s", out.OK, out.Kept, out.Error)
	}
}

func TestAPIScriptsTestOperateChangesOneKey(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"node.name = 'renamed';","kind":"operate","node":{"name":"old","type":"ss","server":"1.1.1.1","port":8388}}`
	code, out, raw := runScriptTest(t, srv, body)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !out.OK {
		t.Fatalf("ok=false err=%s", out.Error)
	}
	if out.Before["name"] != "old" {
		t.Fatalf("before.name=%v", out.Before["name"])
	}
	if out.After["name"] != "renamed" {
		t.Fatalf("after.name=%v", out.After["name"])
	}
	if len(out.After) != 1 {
		t.Fatalf("after 应只含变化的键，实际 %v", out.After)
	}
}

func TestAPIScriptsTestSyntaxErrorHasPosition(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"return {","kind":"filter"}`
	code, out, raw := runScriptTest(t, srv, body)
	if code != http.StatusOK {
		t.Fatalf("编译错误应仍 200，实际 %d body=%s", code, raw)
	}
	if out.OK {
		t.Fatalf("ok 应为 false: %s", raw)
	}
	if !strings.Contains(out.Error, "编译失败") {
		t.Fatalf("error 应含编译失败: %q", out.Error)
	}
	if !strings.Contains(out.Error, "Line") {
		t.Fatalf("error 应带行列号: %q", out.Error)
	}
}

func TestAPIScriptsTestUnknownKind(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"return true","kind":"nope"}`
	code, _, raw := runScriptTest(t, srv, body)
	if code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !strings.Contains(raw, "未知的脚本类型") || !strings.Contains(raw, "nope") {
		t.Fatalf("body=%s", raw)
	}
}

func TestAPIScriptsTestTimeoutTooLarge(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"return true","kind":"filter","timeout":30001}`
	code, _, raw := runScriptTest(t, srv, body)
	if code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !strings.Contains(raw, "timeout") {
		t.Fatalf("body=%s", raw)
	}
}

func TestAPIScriptsTestMissingCode(t *testing.T) {
	srv, _ := scr3TestServer(t)
	code, _, raw := runScriptTest(t, srv, `{"kind":"filter"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !strings.Contains(raw, "缺少 code") {
		t.Fatalf("body=%s", raw)
	}
}

func TestAPIScriptsTestMethodNotAllowed(t *testing.T) {
	srv, _ := scr3TestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/scripts/test", nil)
	srv.handleAPIScriptsTest(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "仅支持 POST") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAPIScriptsTestDefaultSampleNode(t *testing.T) {
	srv, _ := scr3TestServer(t)
	body := `{"code":"node.type === 'ss'","kind":"filter"}`
	code, out, raw := runScriptTest(t, srv, body)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !out.OK || !out.Kept {
		t.Fatalf("缺省 node 应使用内置 ss 样例: ok=%v kept=%v err=%s", out.OK, out.Kept, out.Error)
	}
}
