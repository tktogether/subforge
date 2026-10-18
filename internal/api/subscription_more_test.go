package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// apiSrvWithManager 构造一个最小 Server 并注入 subManager，不触发 Engine。
func apiSrvWithManager(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(t.TempDir() + "/subforge.db")
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

// ---------- A) SubscriptionManager ----------

func TestSubscriptionManager(t *testing.T) {
	srv, mgr := apiSrvWithManager(t)
	if srv.SubscriptionManager() != mgr {
		t.Fatalf("SubscriptionManager() 应返回注入的 mgr")
	}
	if got := New(Config{Engine: &convert.Engine{Profile: render.Profile{Name: "test", Version: "test-version"}}, Logger: quietLogger(), Timeout: 5 * time.Second}).SubscriptionManager(); got != nil {
		t.Fatalf("未注入时 SubscriptionManager() 应为 nil，got=%v", got)
	}
}

// ---------- B) handleAPISelfNodes ----------

func TestSelfNodesListEmpty(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, hdr, body := uiGet(t, ts, "/api/self-nodes")
	if status != http.StatusOK {
		t.Fatalf("GET 空库状态码 = %d，应为 200；body=%s", status, body)
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q，应以 application/json 开头", ct)
	}
	if !strings.Contains(body, "\"self_nodes\":[]") {
		t.Fatalf("空库应输出 self_nodes 空数组，body=%s", body)
	}
	var resp struct {
		SelfNodes []json.RawMessage `json:"self_nodes"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v; body=%s", err, body)
	}
	if len(resp.SelfNodes) != 0 {
		t.Fatalf("空库 self_nodes 长度 = %d，应为 0", len(resp.SelfNodes))
	}
}

func TestSelfNodesPostValid(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("POST 合法状态码 = %d，应为 201；body=%s", status, body)
	}
	var resp struct {
		SelfNode struct {
			Link    string `json:"link"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v; body=%s", err, body)
	}
	if resp.SelfNode.Link != "vmess://x" {
		t.Fatalf("self_node.link = %q，应为 vmess://x", resp.SelfNode.Link)
	}
	if resp.SelfNode.Name != "n1" {
		t.Fatalf("self_node.name = %q，应为 n1", resp.SelfNode.Name)
	}
	if !resp.SelfNode.Enabled {
		t.Fatalf("self_node.enabled 应为 true")
	}
}

func TestSelfNodesPostBlankLink(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"   ","name":"x"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("POST 空白 link 状态码 = %d，应为 400；body=%s", status, body)
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v; body=%s", err, body)
	}
	if resp.Error != "链接不能为空" {
		t.Fatalf("error = %q，应为 链接不能为空", resp.Error)
	}
}

func TestSelfNodesPostInvalidJSON(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, hdr, body := uiPostJSON(t, ts, "/api/self-nodes", `{`)
	if status != http.StatusBadRequest {
		t.Fatalf("POST 非法 JSON 状态码 = %d，应为 400；body=%s", status, body)
	}
	uiAssertJSONError(t, "POST 非法 JSON", body, hdr)
}

func TestSelfNodesListNonEmpty(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	if status != http.StatusCreated {
		t.Fatalf("前置 POST 失败: %d; %s", status, body)
	}
	status, _, body = uiGet(t, ts, "/api/self-nodes")
	if status != http.StatusOK {
		t.Fatalf("GET 非空状态码 = %d，应为 200；body=%s", status, body)
	}
	var resp struct {
		SelfNodes []json.RawMessage `json:"self_nodes"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v; body=%s", err, body)
	}
	if len(resp.SelfNodes) != 1 {
		t.Fatalf("非空库 self_nodes 长度 = %d，应为 1", len(resp.SelfNodes))
	}
}

func TestSelfNodesDefault405(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiDoJSON(t, ts, "PATCH", "/api/self-nodes", `{"link":"a"}`)
	t.Logf("路由层 PATCH /api/self-nodes: status=%d body=%q", status, body)

	srv, _ := apiSrvWithManager(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/self-nodes", nil)
	rr := httptest.NewRecorder()
	srv.handleAPISelfNodes(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("handleAPISelfNodes PATCH 直调状态码 = %d，应为 405；body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.Error != "仅支持 GET/POST" {
		t.Fatalf("handleAPISelfNodes PATCH 直调 error = %q，应为 仅支持 GET/POST；body=%s", resp.Error, rr.Body.String())
	}
}

// ---------- C) handleAPISelfNode ----------

func TestSelfNodeGetDetail(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	var created struct {
		SelfNode struct {
			ID int64 `json:"id"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	id := created.SelfNode.ID
	idStr := fmt.Sprintf("%d", id)

	status, _, body := uiGet(t, ts, "/api/self-nodes/"+idStr)
	if status != http.StatusOK {
		t.Fatalf("GET 详情状态码 = %d，应为 200；body=%s", status, body)
	}
	var resp struct {
		SelfNode struct {
			ID   int64  `json:"id"`
			Link string `json:"link"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v; body=%s", err, body)
	}
	if resp.SelfNode.ID != id {
		t.Fatalf("self_node.id = %d，应为 %d", resp.SelfNode.ID, id)
	}
	if resp.SelfNode.Link != "vmess://x" {
		t.Fatalf("self_node.link = %q，应为 vmess://x", resp.SelfNode.Link)
	}
}

func TestSelfNodeGetNotFound(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, hdr, body := uiGet(t, ts, "/api/self-nodes/999999")
	if status != http.StatusNotFound {
		t.Fatalf("GET 不存在 id 状态码 = %d，应为 404；body=%s", status, body)
	}
	uiAssertJSONError(t, "GET 不存在", body, hdr)
}

func TestSelfNodeGetInvalidID(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, hdr, body := uiGet(t, ts, "/api/self-nodes/abc")
	if status != http.StatusBadRequest {
		t.Fatalf("GET 非法 id 状态码 = %d，应为 400；body=%s", status, body)
	}
	uiAssertJSONError(t, "GET 非法 id", body, hdr)
}

func TestSelfNodePutValid(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	var created struct {
		SelfNode struct {
			ID int64 `json:"id"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	idStr := fmt.Sprintf("%d", created.SelfNode.ID)

	status, _, body := uiDoJSON(t, ts, "PUT", "/api/self-nodes/"+idStr, `{"link":"trojan://y","name":"n2","enabled":false}`)
	if status != http.StatusOK {
		t.Fatalf("PUT 合法状态码 = %d，应为 200；body=%s", status, body)
	}
	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.OK {
		t.Fatalf("PUT 响应 ok 应为 true；body=%s", body)
	}
	status, _, body = uiGet(t, ts, "/api/self-nodes/"+idStr)
	if status != http.StatusOK {
		t.Fatalf("PUT 后 GET 状态码 = %d，应为 200；body=%s", status, body)
	}
	var detail struct {
		SelfNode struct {
			Link    string `json:"link"`
			Enabled bool   `json:"enabled"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &detail); err != nil {
		t.Fatalf("解析详情失败: %v; body=%s", err, body)
	}
	if detail.SelfNode.Link != "trojan://y" {
		t.Fatalf("PUT 后 link = %q，应为 trojan://y", detail.SelfNode.Link)
	}
	if detail.SelfNode.Enabled {
		t.Fatalf("PUT 后 enabled 应为 false")
	}
}

func TestSelfNodePutBlankLink(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	var created struct {
		SelfNode struct {
			ID int64 `json:"id"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	idStr := fmt.Sprintf("%d", created.SelfNode.ID)

	status, _, body := uiDoJSON(t, ts, "PUT", "/api/self-nodes/"+idStr, `{"link":"   ","name":"x","enabled":true}`)
	if status != http.StatusBadRequest {
		t.Fatalf("PUT 空白 link 状态码 = %d，应为 400；body=%s", status, body)
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil || resp.Error != "链接不能为空" {
		t.Fatalf("PUT 空白 link error = %q，应为 链接不能为空；body=%s", resp.Error, body)
	}
}

func TestSelfNodeDelete(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	var created struct {
		SelfNode struct {
			ID int64 `json:"id"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	idStr := fmt.Sprintf("%d", created.SelfNode.ID)

	status, _, body := uiDoJSON(t, ts, "DELETE", "/api/self-nodes/"+idStr, "")
	if status != http.StatusOK {
		t.Fatalf("DELETE 状态码 = %d，应为 200；body=%s", status, body)
	}
	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.OK {
		t.Fatalf("DELETE 响应 ok 应为 true；body=%s", body)
	}
	status, _, body = uiGet(t, ts, "/api/self-nodes/"+idStr)
	if status != http.StatusNotFound {
		t.Fatalf("DELETE 后 GET 状态码 = %d，应为 404；body=%s", status, body)
	}
}

func TestSelfNodeDefault405(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/self-nodes", `{"link":"vmess://x","name":"n1","enabled":true}`)
	var created struct {
		SelfNode struct {
			ID int64 `json:"id"`
		} `json:"self_node"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v; body=%s", err, body)
	}
	idStr := fmt.Sprintf("%d", created.SelfNode.ID)

	status, _, body := uiDoJSON(t, ts, "PATCH", "/api/self-nodes/"+idStr, `{"link":"a"}`)
	t.Logf("路由层 PATCH /api/self-nodes/%s: status=%d body=%q", idStr, status, body)

	srv, _ := apiSrvWithManager(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/self-nodes/"+idStr, nil)
	req.SetPathValue("id", idStr)
	rr := httptest.NewRecorder()
	srv.handleAPISelfNode(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("handleAPISelfNode PATCH 直调状态码 = %d，应为 405；body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.Error != "仅支持 GET/PUT/DELETE" {
		t.Fatalf("handleAPISelfNode PATCH 直调 error = %q，应为 仅支持 GET/PUT/DELETE；body=%s", resp.Error, rr.Body.String())
	}
}

// ---------- D) serveAsset ----------

func TestServeAsset(t *testing.T) {
	uiFakeDist(t)
	srv := New(Config{Engine: &convert.Engine{Profile: render.Profile{Name: "test", Version: "test-version"}}, Logger: quietLogger(), Timeout: 5 * time.Second})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil)
	srv.serveAsset(rr, req, "assets/index-abc123.js")
	if rr.Code != http.StatusOK {
		t.Fatalf("JS serveAsset 状态码 = %d，应为 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
		t.Fatalf("JS Content-Type = %q，应为 application/javascript; charset=utf-8", ct)
	}
	if rr.Body.String() != "window.SF={};" {
		t.Fatalf("JS body = %q，应为 window.SF={};", rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/assets/index-abc123.css", nil)
	srv.serveAsset(rr2, req2, "assets/index-abc123.css")
	if rr2.Code != http.StatusOK {
		t.Fatalf("CSS serveAsset 状态码 = %d，应为 200", rr2.Code)
	}
	if ct := rr2.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Fatalf("CSS Content-Type = %q，应为 text/css; charset=utf-8", ct)
	}
	if rr2.Body.String() != ":root{--sf-x:1}" {
		t.Fatalf("CSS body = %q，应为 :root{--sf-x:1}", rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	srv.serveAsset(rr3, req3, "index.html")
	if rr3.Code != http.StatusOK {
		t.Fatalf("HTML serveAsset 状态码 = %d，应为 200", rr3.Code)
	}
	if ct := rr3.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("HTML Content-Type = %q，应为 text/html; charset=utf-8", ct)
	}

	rr4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/assets/nope.js", nil)
	srv.serveAsset(rr4, req4, "assets/nope.js")
	if rr4.Code != http.StatusNotFound {
		t.Fatalf("未命中 serveAsset 状态码 = %d，应为 404", rr4.Code)
	}
	if strings.Contains(rr4.Body.String(), "<div id=\"app\">") {
		t.Fatalf("未命中 serveAsset 不应返回 SPA 的 #app")
	}
}
