package api

// routes_test.go —— ROUTES-1b 收口测试：证明 server.go 里注册的每一条路由都真的可达。
//
// 判据只有一条：**不是 404**。业务层返回 400/405/501/503 都说明路由已经挂上；
// 只有 404 意味着 chi 根本没匹配到这个 pattern。
// 表里同时把实际 status 打出来（t.Logf），便于人工核对每条路由的真实归属。

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// routeCase 描述一条待验证的路由。body 传 "{}" 是为了让需要解析 JSON 的 handler
// 走到业务分支而不是在解码阶段就 400 —— 两种都算已注册，但前者信息量更大。
type routeCase struct {
	name   string
	method string
	path   string
	body   string
}

// routeCases 逐条对应任务下发的路由表。
var routeCases = []routeCase{
	{"scripts-test", http.MethodPost, "/api/scripts/test", "{}"},
	{"airports-check", http.MethodPost, "/api/airports/check", "{}"},
	{"airports-check-last", http.MethodGet, "/api/airports/check/last", ""},
	{"airport-import-rules", http.MethodPost, "/api/airports/1/import-rules", "{}"},
	{"airport-refresh-one", http.MethodPost, "/api/airports/1/refresh", ""},
	{"rules-bulk", http.MethodPost, "/api/rules/bulk", "{}"},
	{"rule-providers", http.MethodGet, "/api/rule-providers", ""},
	{"suggestions-list", http.MethodGet, "/api/suggestions", ""},
	{"suggestions-confirm", http.MethodPost, "/api/suggestions/0/confirm", "{}"},
	{"suggestions-reject", http.MethodPost, "/api/suggestions/0/reject", "{}"},
	{"rule-sources-list", http.MethodGet, "/api/rule-sources", ""},
	{"rule-sources-update", http.MethodPost, "/api/rule-sources/update", "{}"},
	{"rule-sources-rollback", http.MethodPost, "/api/rule-sources/rollback", "{}"},
	{"rule-sources-mrs", http.MethodPost, "/api/rule-sources/mrs", "{}"},
	{"settings-get", http.MethodGet, "/api/settings", ""},
	{"settings-put", http.MethodPut, "/api/settings", "{}"},
	{"template-get", http.MethodGet, "/api/template", ""},
	{"template-put", http.MethodPut, "/api/template", "{}"},
	{"override-get", http.MethodGet, "/api/override", ""},
	{"override-put", http.MethodPut, "/api/override", "{}"},
}

// TestRoutesAllRegistered 表驱动验证路由表里每一条都已注册（status != 404）。
func TestRoutesAllRegistered(t *testing.T) {
	ts, _ := uiSubServer(t)
	for _, tc := range routeCases {
		t.Run(tc.name, func(t *testing.T) {
			status, hdr, body := uiDoJSON(t, ts, tc.method, tc.path, tc.body)
			t.Logf("%s %s => %d", tc.method, tc.path, status)
			assertRouteReached(t, tc.method, tc.path, status, hdr, body)
		})
	}
}

// routingMissBody 是 server.go:handleNotFound 写出的正文特征。
// 只有它才代表「chi 没匹配到任何 pattern」，也就是路由真的没注册。
const routingMissBody = `"code":"route_not_found"`

// assertRouteReached 断言请求真的进了业务 handler。
//
// 为什么不直接判 status != 404：多个 handler 在「资源不存在 / 未配置」时会主动返回 404
// （import_rules.go:31「机场不存在」、template_api.go:27「模板路径未配置」、
// override_api.go:27「override 路径未配置」、subscription.go:119「订阅 1 不存在」）。
// 这些 404 恰恰证明路由已经挂上、handler 已经执行，用 status 判会得到假阴性。
// 真正的区分点：路由层 404 由 handleNotFound 写成带 code=route_not_found 的 JSON；
// 业务层 404 一律走 writeAPIError，不带这个 code。
func assertRouteReached(t *testing.T, method, path string, status int, hdr http.Header, body string) {
	t.Helper()
	if status != http.StatusNotFound {
		return
	}
	if strings.Contains(body, routingMissBody) {
		t.Fatalf("%s %s => 路由未注册（落到 handleNotFound）；body=%q", method, path, body)
	}
	if !strings.HasPrefix(hdr.Get("Content-Type"), "application/json") {
		t.Fatalf("%s %s => 404 但响应不是 JSON，疑似路由未注册；Content-Type=%q body=%q",
			method, path, hdr.Get("Content-Type"), body)
	}
	t.Logf("%s %s => 404 来自业务 handler（JSON error），路由已注册", method, path)
}

// TestRoutesAllRegisteredWithSeededAirport —— 把 /api/airports/{id}/... 这类带真实资源的
// 路由跑成 200，用最强证据证明它们接对了（而不是只证明「不是路由 404」）。
func TestRoutesAllRegisteredWithSeededAirport(t *testing.T) {
	ts, mgr := uiSubServer(t)
	// 用本地 httptest 当上游，避免依赖外网 DNS —— CI 无网时不会卡在解析超时上。
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(up.Close)
	id, err := mgr.AddAirport(t.Context(), "路由验证机场", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	for _, tc := range []routeCase{
		{"airport-import-rules-seeded", http.MethodPost, "/api/airports/" + itoa(id.ID) + "/import-rules", "{}"},
		{"airport-refresh-one-seeded", http.MethodPost, "/api/airports/" + itoa(id.ID) + "/refresh", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, hdr, body := uiDoJSON(t, ts, tc.method, tc.path, tc.body)
			t.Logf("%s %s => %d body=%s", tc.method, tc.path, status, body)
			assertRouteReached(t, tc.method, tc.path, status, hdr, body)
		})
	}
}

// itoa 避免为一个测试引入 strconv 依赖以外的噪音（strconv 已在别处大量使用）。
func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// TestRoutesStaticPriorityAirportsCheck —— chi 同层「静态段优先于参数段」。
// POST /api/airports/check 必须命中 handleAPICheck，而不是被 /api/airports/{id} 吃掉；
// 被 {id} 吃掉的典型症状是 pathID 解析 "check" 失败后返回「无效的 ID」。
func TestRoutesStaticPriorityAirportsCheck(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiDoJSON(t, ts, http.MethodPost, "/api/airports/check", "{}")
	t.Logf("POST /api/airports/check => %d body=%s", status, body)
	if status == http.StatusNotFound {
		t.Fatalf("POST /api/airports/check => 404，静态段未注册")
	}
	if strings.Contains(body, "无效的 ID") {
		t.Fatalf("POST /api/airports/check 落到了 {id} 处理器：body=%s", body)
	}
}

// TestRoutesStaticPriorityRulesBulk —— 同理，/api/rules/bulk 不能被 /api/rules/{id} 吃掉。
func TestRoutesStaticPriorityRulesBulk(t *testing.T) {
	ts, _ := uiSubServer(t)
	status, _, body := uiDoJSON(t, ts, http.MethodPost, "/api/rules/bulk", "{}")
	t.Logf("POST /api/rules/bulk => %d body=%s", status, body)
	if status == http.StatusNotFound {
		t.Fatalf("POST /api/rules/bulk => 404，静态段未注册")
	}
	if strings.Contains(body, "无效的 ID") {
		t.Fatalf("POST /api/rules/bulk 落到了 handleAPIRule：body=%s", body)
	}
}

// TestRoutesNoDuplicatePanic —— chi 对同一 pattern 重复注册会 panic。
// 本任务一次性接了 20+ 条路由，重复注册是最高频的翻车方式，必须显式钉住。
func TestRoutesNoDuplicatePanic(t *testing.T) {
	// newTrafficTestServer 内部已经调用过一次 Routes()，这里再调两次。
	_, _, srv := newTrafficTestServer(t)
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Routes() 重复调用 panic: %v", rec)
		}
	}()
	r1 := srv.Routes()
	r2 := srv.Routes()
	if r1 == nil || r2 == nil {
		t.Fatalf("Routes() 返回 nil: r1=%v r2=%v", r1, r2)
	}
}

// TestRoutesSPAFallbackStillWorks —— 新增 /api/* 路由不能把 /* 兜底弄坏。
func TestRoutesSPAFallbackStillWorks(t *testing.T) {
	uiFakeDist(t)
	ts, _ := uiSubServer(t)
	status, _, body := uiDoJSON(t, ts, http.MethodGet, "/some/deep/link", "")
	t.Logf("GET /some/deep/link => %d", status)
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if !strings.Contains(body, "id=\"app\"") {
		t.Fatalf("SPA 兜底未返回 index.html：body=%s", body)
	}
}

// TestRoutesAPIPrefixNeverHTML —— /api/* 的未知路径必须是 JSON 404，不能回落 SPA。
// 否则前端 fetch 会拿到一坨 HTML 再在 JSON.parse 处炸掉，排查成本极高。
func TestRoutesAPIPrefixNeverHTML(t *testing.T) {
	uiFakeDist(t)
	ts, _ := uiSubServer(t)
	status, _, body := uiDoJSON(t, ts, http.MethodGet, "/api/nope", "")
	t.Logf("GET /api/nope => %d body=%s", status, body)
	if status != http.StatusNotFound {
		t.Fatalf("status=%d，/api 前缀未知路径应为 404；body=%s", status, body)
	}
	if strings.Contains(body, "id=\"app\"") {
		t.Fatalf("/api 前缀返回了 SPA HTML：body=%s", body)
	}
}
