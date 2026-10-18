// web_ui_test.go — M2 嵌入式 Web UI（internal/api/web/）与 /api/* JSON 端点的回归测试。
//
// 约束：全部走本地 httptest，不依赖外网、不依赖真实订阅。需要 Engine 的构造沿用
// server_test.go 的 newTestServer（最小 Engine，且以下用例都不触发上游拉取）。
//
// 覆盖四件事：
//  1. SPA fallback 与静态资源语义：真实文件按文件服务、带扩展名的缺失资源 404、无扩展名路径回落 index.html；
//  2. 排除前缀（/api、/sub、/render 等）不进入 fallback（spaExcluded 的前缀匹配语义）；
//  3. 无构建产物（干净 clone、只跑 go test）时退化为 404，而不是 500 或空的 200；
//  4. /api/targets、/api/diff、/api/convert、订阅管理 CRUD 与 /subscribe 的行为契约。
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/subforge"
)

// --- 小工具 ---

// uiGet 发一个 GET，返回状态码、响应头与正文。
func uiGet(t *testing.T, ts *httptest.Server, path string) (int, http.Header, string) {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取 %s 响应: %v", path, err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

// uiPostJSON 发一个 POST application/json，返回状态码、响应头与正文。
func uiPostJSON(t *testing.T, ts *httptest.Server, path, body string) (int, http.Header, string) {
	t.Helper()
	resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取 %s 响应: %v", path, err)
	}
	return resp.StatusCode, resp.Header, string(buf)
}

// uiDoJSON 发一个带 JSON body 的请求（覆盖 uiPostJSON 到不了的 PUT/DELETE）。
func uiDoJSON(t *testing.T, ts *httptest.Server, method, path, body string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取 %s 响应: %v", path, err)
	}
	return resp.StatusCode, resp.Header, string(buf)
}

// uiSubServer 起一个启用了订阅管理（临时 SQLite）的服务。
// /api/airports 等 CRUD 与 /subscribe 只有在 SetSubManager 之后才真正可用，否则一律 503；
// Engine 的组装沿用 server_test.go:newTestServer，额外注入一个临时库的 Manager。
func uiSubServer(t *testing.T) (*httptest.Server, *subforge.Manager) {
	return uiSubServerCfg(t, nil)
}

// uiSubServerCfg 与 uiSubServer 同源，额外允许调用方改 Config（例如打开访问令牌鉴权）。
func uiSubServerCfg(t *testing.T, cfgMod func(*Config)) (*httptest.Server, *subforge.Manager) {
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
	if cfgMod != nil {
		cfgMod(&cfg)
	}
	srv := New(cfg)
	srv.SetSubManager(mgr)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts, mgr
}

// uiAssertJSONError 断言错误响应是统一形状：Content-Type JSON 且带非空 error 字段。
func uiAssertJSONError(t *testing.T, what, body string, hdr http.Header) {
	t.Helper()
	if got := hdr.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("%s Content-Type = %q，错误体应是 JSON", what, got)
	}
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &e); err != nil || e.Error == "" {
		t.Errorf("%s 错误体缺少 error 字段: %q（err=%v）", what, body, err)
	}
}

// --- SPA 页面与 fallback ---

// uiFakeDist 把 webFS 换成一个最小的假 Vite 产物，让 SPA 路由可测。
//
// 为什么必须换：dist/ 是构建产物、不入库（仓库里只有 .gitkeep），干净 clone 上
// webFS 里没有 index.html。直接断言真实嵌入资源，等于在测"本地有没有跑前端构建"，
// 而不是在测路由逻辑——那种用例在 CI 上必挂。
func uiFakeDist(t *testing.T) {
	t.Helper()
	old := webFS
	webFS = fstest.MapFS{
		"index.html":              {Data: []byte("<!doctype html><html><head><title>SubForge</title></head><body><div id=\"app\"></div></body></html>")},
		"assets/index-abc123.js":  {Data: []byte("window.SF={};")},
		"assets/index-abc123.css": {Data: []byte(":root{--sf-x:1}")},
		"favicon.ico":             {Data: []byte("ICO")},
	}
	t.Cleanup(func() { webFS = old })
}

// TestWebUI_SPAFallback 无扩展名的未知路径必须回落 index.html ——
// createWebHistory 下 /airports 这类深链在服务端没有对应文件，只能交给前端路由。
func TestWebUI_SPAFallback(t *testing.T) {
	uiFakeDist(t)
	ts := newTestServer(t, nil)
	for _, p := range []string{
		"/", "/convert", "/pipelines", "/airports", "/self-nodes", "/rules", "/groups",
		"/subscriptions", "/logs", "/settings", "/scripts", "/tasks",
		"/deep/unknown/path",
	} {
		status, hdr, body := uiGet(t, ts, p)
		if status != http.StatusOK {
			t.Errorf("%s 状态 = %d，应为 200", p, status)
			continue
		}
		if ct := hdr.Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("%s Content-Type = %q，应为 text/html; charset=utf-8", p, ct)
		}
		if !strings.Contains(body, `<div id="app">`) {
			t.Errorf("%s 未回落 index.html（正文里没有 #app 挂载点）", p)
		}
	}
}

// TestWebUI_StaticAssets 真实产物按文件服务，Content-Type 必须与扩展名匹配。
func TestWebUI_StaticAssets(t *testing.T) {
	uiFakeDist(t)
	ts := newTestServer(t, nil)
	cases := []struct {
		path string
		ct   string
		body string
	}{
		{"/assets/index-abc123.js", "application/javascript; charset=utf-8", "window.SF={};"},
		{"/assets/index-abc123.css", "text/css; charset=utf-8", ":root{--sf-x:1}"},
		{"/favicon.ico", "application/octet-stream", "ICO"},
	}
	for _, c := range cases {
		status, hdr, body := uiGet(t, ts, c.path)
		if status != http.StatusOK {
			t.Errorf("%s 状态 = %d，应为 200", c.path, status)
			continue
		}
		if got := hdr.Get("Content-Type"); got != c.ct {
			t.Errorf("%s Content-Type = %q，应为 %q", c.path, got, c.ct)
		}
		if body != c.body {
			t.Errorf("%s 正文 = %q，应为 %q", c.path, body, c.body)
		}
	}
}

// TestWebUI_MissingAssetIs404NotHTML 是本文件最重要的一条：
// 带扩展名却找不到的资源必须 404，**绝不能回落 index.html**。
// 一旦回落成 HTML，浏览器会把 HTML 当 JS 解析并报 "Unexpected token '<'" 时，
// 服务端日志里看不到任何异常（状态码是 200）。
func TestWebUI_MissingAssetIs404NotHTML(t *testing.T) {
	uiFakeDist(t)
	ts := newTestServer(t, nil)
	for _, p := range []string{
		"/assets/missing.js", "/assets/missing.css", "/nope.png", "/js/old.js", "/favicon.svg",
	} {
		status, _, body := uiGet(t, ts, p)
		if status != http.StatusNotFound {
			t.Errorf("%s 状态 = %d，应为 404", p, status)
		}
		if strings.Contains(body, `<div id="app">`) {
			t.Errorf("%s 回落成了 HTML：浏览器会把它当 JS 解析并报 Unexpected token '<'", p)
		}
	}
}

// TestWebUI_APIPrefixNeverFallsBackToHTML 排除前缀里的路径不得进入 SPA fallback。
// 否则一个写错的 API 路径会拿到 200 + HTML，前端 res.json() 抛出的错
// 会指向 JSON 解析，而不是"路径写错了"。
func TestWebUI_APIPrefixNeverFallsBackToHTML(t *testing.T) {
	uiFakeDist(t)
	ts := newTestServer(t, nil)
	for _, p := range []string{
		"/api/does-not-exist", "/api/", "/sub/does-not-exist", "/render/nope",
		"/inspect/nope", "/profile/nope", "/getruleset/nope", "/version/nope", "/metrics/nope",
	} {
		_, _, body := uiGet(t, ts, p)
		if strings.Contains(body, `<div id="app">`) {
			t.Errorf("%s 落进了 SPA fallback，该前缀必须排除", p)
		}
	}
}

// TestWebUI_NoDistDegradesTo404 没有构建产物时（干净 clone、只跑 go test）
// 必须退化为 404，而不是 500 或空的 200。
func TestWebUI_NoDistDegradesTo404(t *testing.T) {
	old := webFS
	webFS = fstest.MapFS{}
	t.Cleanup(func() { webFS = old })
	ts := newTestServer(t, nil)
	for _, p := range []string{"/", "/convert", "/deep/unknown/path"} {
		if status, _, _ := uiGet(t, ts, p); status != http.StatusNotFound {
			t.Errorf("%s 在无构建产物时状态 = %d，应为 404", p, status)
		}
	}
}

// TestWebUI_SPAMethodSemantics 只有 GET/HEAD 参与 fallback；HEAD 与 GET 同源。
func TestWebUI_SPAMethodSemantics(t *testing.T) {
	uiFakeDist(t)
	ts := newTestServer(t, nil)

	// POST 无扩展名路径：chi 的 /* 只注册了 GET/HEAD，POST 命中同一路由但方法不匹配，
	// chi 返回 405 Method Not Allowed（而非 404）；两种都表示"未回落成 HTML"，不得回落。
	if status, _, _ := uiDoJSON(t, ts, http.MethodPost, "/convert", "{}"); status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
		t.Errorf("POST /convert 状态 = %d，应为 404 或 405", status)
	}

	// HEAD 与 GET 走同一 handler。
	req, err := http.NewRequest(http.MethodHead, ts.URL+"/convert", nil)
	if err != nil {
		t.Fatalf("构造 HEAD 请求: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HEAD /convert: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HEAD /convert 状态 = %d，应为 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("HEAD /convert Content-Type = %q", ct)
	}
}

// TestSpaExcluded 前缀匹配是「等于前缀」或「前缀 + /」，不是 strings.HasPrefix。
// 用 HasPrefix 会让 /apiary 这类同前缀路径被误判成 API，直接 404。
func TestSpaExcluded(t *testing.T) {
	excluded := []string{
		"/api", "/api/x", "/sub", "/sub/x", "/subscribe", "/subscribe/x",
		"/profile", "/profile/x", "/render", "/render/x", "/getruleset", "/getruleset/x",
		"/inspect", "/inspect/x", "/version", "/version/x", "/healthz", "/healthz/x",
		"/debug/pprof", "/debug/pprof/heap", "/metrics", "/metrics/x",
	}
	for _, p := range excluded {
		if !spaExcluded(p) {
			t.Errorf("spaExcluded(%q) = false，应为 true", p)
		}
	}
	allowed := []string{"/", "/apiary", "/submarine", "/versions", "/healthz2", "/metric", "/convert"}
	for _, p := range allowed {
		if spaExcluded(p) {
			t.Errorf("spaExcluded(%q) = true，应为 false（同前缀不同路径不能误伤）", p)
		}
	}
}

// TestLastSegment 扩展名判定只看最后一段。
func TestLastSegment(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"/":           "",
		"a":           "a",
		"a/b":         "b",
		"a/b/c.txt":   "c.txt",
		"assets/x.js": "x.js",
	}
	for in, want := range cases {
		if got := lastSegment(in); got != want {
			t.Errorf("lastSegment(%q) = %q，应为 %q", in, got, want)
		}
	}
}

// --- /api/targets ---

// TestWebUI_Targets /api/targets 必须是非空 JSON 数组且含默认 target clash.meta。
// 只断言集合与首元素，不断言全序：registry.Available() 的尾段来自 map 迭代，
// 每次请求顺序都可能不同（验证报告 BUG-2），断言全序会让用例随机失败。
func TestWebUI_Targets(t *testing.T) {
	ts := newTestServer(t, nil)
	status, hdr, body := uiGet(t, ts, "/api/targets")
	if status != http.StatusOK {
		t.Fatalf("/api/targets 状态 = %d，应为 200，body=%s", status, body)
	}
	if got := hdr.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("/api/targets Content-Type = %q，应为 application/json", got)
	}
	var targets []string
	if err := json.Unmarshal([]byte(body), &targets); err != nil {
		t.Fatalf("/api/targets 不是 JSON 数组: %v\n%s", err, body)
	}
	if len(targets) == 0 {
		t.Fatal("/api/targets 返回空数组：Dashboard/Convert 的 target 下拉框会是空的")
	}
	if targets[0] != "clash.meta" {
		t.Errorf("/api/targets 首项 = %q，应为默认 target clash.meta", targets[0])
	}
	have := make(map[string]bool, len(targets))
	for _, x := range targets {
		have[x] = true
	}
	for _, want := range []string{"clash.meta", "surge", "loon", "list", "mixed", "sing-box", "v2ray"} {
		if !have[want] {
			t.Errorf("/api/targets 缺少 %q", want)
		}
	}
}

// --- /api/diff ---

// TestWebUI_DiffLineSemantics 行级 diff 语义：相等行 type=" "、左侧独有 "-"、
// 右侧独有 "+"，且顺序稳定（前端按数组顺序逐行渲染，顺序抖动会让人读错 diff）。
func TestWebUI_DiffLineSemantics(t *testing.T) {
	ts := newTestServer(t, nil)
	type diffLine struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	cases := []struct {
		name string
		a, b string
		want []diffLine
	}{
		{
			name: "替换并追加",
			a:    "alpha\nbeta\ngamma",
			b:    "alpha\nBETA\ngamma\ndelta",
			want: []diffLine{{" ", "alpha"}, {"-", "beta"}, {"+", "BETA"}, {" ", "gamma"}, {"+", "delta"}},
		},
		{
			name: "纯追加",
			a:    "a\nb",
			b:    "a\nb\nc",
			want: []diffLine{{" ", "a"}, {" ", "b"}, {"+", "c"}},
		},
		{
			name: "纯删除",
			a:    "a\nb",
			b:    "a",
			want: []diffLine{{" ", "a"}, {"-", "b"}},
		},
		{
			name: "完全相同",
			a:    "same",
			b:    "same",
			want: []diffLine{{" ", "same"}},
		},
	}
	for _, c := range cases {
		payload, err := json.Marshal(map[string]string{"a": c.a, "b": c.b})
		if err != nil {
			t.Fatalf("%s: 构造请求体: %v", c.name, err)
		}
		status, hdr, body := uiPostJSON(t, ts, "/api/diff", string(payload))
		if status != http.StatusOK {
			t.Errorf("%s: 状态 = %d，应为 200，body=%s", c.name, status, body)
			continue
		}
		if got := hdr.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Errorf("%s: Content-Type = %q，应为 application/json", c.name, got)
		}
		var got struct {
			Lines []diffLine `json:"lines"`
		}
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Errorf("%s: 响应不是 JSON: %v\n%s", c.name, err, body)
			continue
		}
		if len(got.Lines) != len(c.want) {
			t.Errorf("%s: 得到 %d 行，期望 %d 行\n实际: %+v", c.name, len(got.Lines), len(c.want), got.Lines)
			continue
		}
		for i := range c.want {
			if got.Lines[i] != c.want[i] {
				t.Errorf("%s: 第 %d 行 = %+v，期望 %+v", c.name, i, got.Lines[i], c.want[i])
			}
			if got.Lines[i].Type != " " && got.Lines[i].Type != "+" && got.Lines[i].Type != "-" {
				t.Errorf("%s: 第 %d 行 type = %q，只允许 \" \" / \"+\" / \"-\"", c.name, i, got.Lines[i].Type)
			}
		}
		// 顺序稳定：同一输入重放必须逐字节一致。
		if _, _, again := uiPostJSON(t, ts, "/api/diff", string(payload)); again != body {
			t.Errorf("%s: 同一输入两次结果不一致\n第一次: %s\n第二次: %s", c.name, body, again)
		}
	}
}

// TestWebUI_DiffEmptyBaselineIsClean 是 BUG-1 的回归守门（缺陷已修复，用例已打开）：
// a="" 且 b 非空时 /api/diff 会多出一行 {"type":"-","text":""}，因为 splitLines("")
// 返回 [""] 而不是空切片。Convert 页默认 baseline 为空，点“生成 diff”就会看到这行
// 幽灵删除。修复（splitLines 对空串返回 nil）后打开本用例。
func TestWebUI_DiffEmptyBaselineIsClean(t *testing.T) {
	// BUG-1 已修复：splitLines 对空串返回 nil，空基线不再产生幽灵删除行。
	ts := newTestServer(t, nil)
	status, _, body := uiPostJSON(t, ts, "/api/diff", `{"a":"","b":"only-right"}`)
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，应为 200", status)
	}
	if strings.Contains(body, `{"type":"-","text":""}`) {
		t.Errorf("空基线不应产出空删除行: %s", body)
	}
}

// TestWebUI_PipelinesConfigErrorNotSwallowed 是 BUG-3 的回归守门（缺陷已修复，用例已打开）：
// handleAPIPipelines / handleAPIRulesets 用 ` + "`p, _ := s.engine.ResolveTemplates(...)`" + ` 丢掉了
// 错误，config 指向的模板加载失败时仍返回 200 + 空结果，Rules 页会显示"当前模板未声明
// [ruleset] 规则集映射。"而不是报错。修复（把错误按 400/502 写出）后打开本用例。
func TestWebUI_PipelinesConfigErrorNotSwallowed(t *testing.T) {
	// BUG-3 已修复：ResolveTemplates 的错误按 502 写出，不再伪装成空模板。
	ts := newTestServer(t, nil)
	// 引擎没配 LoadTemplate，ResolveTemplates 会立刻报错，不会发起任何网络请求。
	for _, path := range []string{
		"/api/pipelines?config=" + url.QueryEscape("http://example.invalid/tpl.ini"),
		"/api/rulesets?config=" + url.QueryEscape("http://example.invalid/tpl.ini"),
	} {
		status, _, body := uiGet(t, ts, path)
		if status == http.StatusOK {
			t.Errorf("%s: config 无法解析时不应返回 200：%s", path, body)
		}
	}
}

// --- /api/convert ---

// TestWebUI_ConvertValidation /api/convert 参数非法必须是 400（而不是 500/200）。
// 前端 SF.api 靠 !res.ok 走错误分支，并从 JSON 的 error 字段取提示文本。
func TestWebUI_ConvertValidation(t *testing.T) {
	ts := newTestServer(t, nil)
	cases := []struct {
		name string
		body string
	}{
		{"空 body", ""},
		{"空 JSON 对象（无来源）", "{}"},
		{"只有 target 没有来源", `{"target":"clash.meta"}`},
		{"空 url 列表", `{"urls":[]}`},
		{"非法 JSON", "{oops"},
	}
	for _, c := range cases {
		status, hdr, body := uiPostJSON(t, ts, "/api/convert", c.body)
		if status != http.StatusBadRequest {
			t.Errorf("%s: 状态 = %d，应为 400，body=%s", c.name, status, body)
			continue
		}
		if got := hdr.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Errorf("%s: Content-Type = %q，错误体应是 JSON（前端按 JSON 解析 error）", c.name, got)
		}
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &e); err != nil || e.Error == "" {
			t.Errorf("%s: 错误体缺少 error 字段: %q（err=%v）", c.name, body, err)
		}
	}
	// /api/diff 的坏 body 同样是 400，不能是 500。
	if status, _, body := uiPostJSON(t, ts, "/api/diff", "{oops"); status != http.StatusBadRequest {
		t.Errorf("/api/diff 坏 body 状态 = %d，应为 400，body=%s", status, body)
	}
}

// --- 其余 /api/* 的形状 ---

// TestWebUI_JSONEndpointShapes 页面脚本读取的字段必须真实存在，且都是 JSON。
// Dashboard 读 stats.history.*，Rules 读 rulesets，Logs 读 logs，Convert 读 pipelines。
func TestWebUI_JSONEndpointShapes(t *testing.T) {
	ts := newTestServer(t, nil)
	for _, path := range []string{"/api/stats", "/api/history", "/api/pipelines", "/api/rulesets", "/api/logs"} {
		status, hdr, body := uiGet(t, ts, path)
		if status != http.StatusOK {
			t.Errorf("%s 状态 = %d，应为 200，body=%s", path, status, body)
			continue
		}
		if got := hdr.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Errorf("%s Content-Type = %q，应为 application/json", path, got)
		}
		var v any
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Errorf("%s 不是合法 JSON: %v\n%s", path, err, body)
		}
	}

	// /api/stats：dashboard.js 读 s.history.total / cachedHits / urlTotal 与 s.logs。
	_, _, body := uiGet(t, ts, "/api/stats")
	var stats struct {
		History map[string]int `json:"history"`
		Logs    *int           `json:"logs"`
	}
	if err := json.Unmarshal([]byte(body), &stats); err != nil {
		t.Fatalf("/api/stats 解析失败: %v\n%s", err, body)
	}
	for _, k := range []string{"total", "cachedHits", "urlTotal"} {
		if _, ok := stats.History[k]; !ok {
			t.Errorf("/api/stats 缺少 history.%s: %s", k, body)
		}
	}
	if stats.Logs == nil {
		t.Errorf("/api/stats 缺少 logs 字段: %s", body)
	}

	// /api/history 与 /api/pipelines 必须是数组（前端直接 .forEach / .length）。
	for _, path := range []string{"/api/history", "/api/pipelines"} {
		_, _, body := uiGet(t, ts, path)
		if !strings.HasPrefix(strings.TrimSpace(body), "[") {
			t.Errorf("%s 应为 JSON 数组，实际: %s", path, body)
		}
	}

	// /api/rulesets：rules.js 读 res.rulesets。
	_, _, body = uiGet(t, ts, "/api/rulesets")
	var rs struct {
		Rulesets map[string]string `json:"rulesets"`
	}
	if err := json.Unmarshal([]byte(body), &rs); err != nil {
		t.Errorf("/api/rulesets 缺少 rulesets 对象: %v\n%s", err, body)
	}

	// /api/logs：logs.js 读 res.logs[].id / .ts / .line。
	_, _, body = uiGet(t, ts, "/api/logs?since=0")
	var lg struct {
		Logs []struct {
			ID   int64  `json:"id"`
			Ts   string `json:"ts"`
			Line string `json:"line"`
		} `json:"logs"`
	}
	if err := json.Unmarshal([]byte(body), &lg); err != nil {
		t.Errorf("/api/logs 缺少 logs 数组: %v\n%s", err, body)
	}
}

// --- 第 5 页 /subscriptions 与 /subscribe ---

// TestWebUI_SubscriptionCRUDShapes 第 5 页 /subscriptions 用的 CRUD 端点必须按冻结
// 契约返回包装对象（{"airport":...} / {"group":...} / {"rule":...} / {"token":...}），
// 失败一律 JSON {"error":...}。修复前这些处理器回显裸对象、GET/PUT /{id} 直接 405、
// GET /api/tokens 硬编码空数组——脚本读 r.data.airport 只会得到 undefined。
//
// POST 未约定 200/201，这里只要求 2xx；字段集合之外的额外字段不算违约。
func TestWebUI_SubscriptionCRUDShapes(t *testing.T) {
	ts, _ := uiSubServer(t)

	// --- /api/airports ---
	status, _, body := uiPostJSON(t, ts, "/api/airports",
		`{"name":"机场A","subscription_url":"http://127.0.0.1:1/sub","remarks":"备注","enabled":true}`)
	if status < 200 || status >= 300 {
		t.Fatalf("POST /api/airports 状态 = %d，应为 2xx，body=%s", status, body)
	}
	var aCreate struct {
		Airport struct {
			ID              int64  `json:"id"`
			Name            string `json:"name"`
			SubscriptionURL string `json:"subscription_url"`
			Remarks         string `json:"remarks"`
			Enabled         bool   `json:"enabled"`
		} `json:"airport"`
	}
	mustJSON(t, body, &aCreate)
	if aCreate.Airport.ID == 0 || aCreate.Airport.Name != "机场A" ||
		aCreate.Airport.SubscriptionURL != "http://127.0.0.1:1/sub" || !aCreate.Airport.Enabled {
		t.Fatalf("POST /api/airports 返回体不符契约: %s", body)
	}
	airportID := aCreate.Airport.ID

	status, _, body = uiGet(t, ts, "/api/airports")
	if status != http.StatusOK {
		t.Fatalf("GET /api/airports 状态 = %d，body=%s", status, body)
	}
	var aList struct {
		Airports []map[string]any `json:"airports"`
	}
	mustJSON(t, body, &aList)
	if len(aList.Airports) != 1 {
		t.Fatalf("GET /api/airports 应返回 1 条，实际 %d: %s", len(aList.Airports), body)
	}
	for _, k := range []string{"id", "name", "subscription_url", "remarks", "enabled", "sort_order"} {
		if _, ok := aList.Airports[0][k]; !ok {
			t.Errorf("GET /api/airports 元素缺少 %q: %s", k, body)
		}
	}

	status, _, body = uiGet(t, ts, fmt.Sprintf("/api/airports/%d", airportID))
	if status != http.StatusOK {
		t.Fatalf("GET /api/airports/{id} 状态 = %d，body=%s", status, body)
	}
	var aGet struct {
		Airport struct {
			ID int64 `json:"id"`
		} `json:"airport"`
	}
	mustJSON(t, body, &aGet)
	if aGet.Airport.ID != airportID {
		t.Errorf("GET /api/airports/{id} 返回 id=%d，期望 %d: %s", aGet.Airport.ID, airportID, body)
	}

	status, _, body = uiDoJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/airports/%d", airportID),
		`{"name":"机场B","subscription_url":"http://127.0.0.1:2/sub","remarks":"","enabled":false}`)
	if status < 200 || status >= 300 {
		t.Fatalf("PUT /api/airports/{id} 状态 = %d，body=%s", status, body)
	}
	var aPut struct {
		Airport struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"airport"`
	}
	mustJSON(t, body, &aPut)
	if aPut.Airport.Name != "机场B" || aPut.Airport.Enabled {
		t.Errorf("PUT /api/airports/{id} 未生效: %s", body)
	}

	status, _, body = uiDoJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/airports/%d", airportID), "")
	if status < 200 || status >= 300 {
		t.Fatalf("DELETE /api/airports/{id} 状态 = %d，body=%s", status, body)
	}
	var okAirport struct {
		OK bool `json:"ok"`
	}
	mustJSON(t, body, &okAirport)
	if !okAirport.OK {
		t.Errorf("DELETE /api/airports/{id} 应返回 {\"ok\":true}: %s", body)
	}

	// --- /api/groups ---
	status, _, body = uiPostJSON(t, ts, "/api/groups",
		`{"name":"AutoSelect","type":"select","regex":".*","url":"http://www.gstatic.com/generate_204","interval":300}`)
	if status < 200 || status >= 300 {
		t.Fatalf("POST /api/groups 状态 = %d，应为 2xx，body=%s", status, body)
	}
	var gCreate struct {
		Group struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			Regex    string `json:"regex"`
			URL      string `json:"url"`
			Interval int    `json:"interval"`
		} `json:"group"`
	}
	mustJSON(t, body, &gCreate)
	if gCreate.Group.ID == 0 || gCreate.Group.Name != "AutoSelect" ||
		gCreate.Group.Type != "select" || gCreate.Group.Interval != 300 {
		t.Fatalf("POST /api/groups 返回体不符契约: %s", body)
	}
	groupID := gCreate.Group.ID

	status, _, body = uiGet(t, ts, "/api/groups")
	if status != http.StatusOK {
		t.Fatalf("GET /api/groups 状态 = %d，body=%s", status, body)
	}
	var gList struct {
		Groups []map[string]any `json:"groups"`
	}
	mustJSON(t, body, &gList)
	if len(gList.Groups) != 1 {
		t.Fatalf("GET /api/groups 应返回 1 条，实际 %d: %s", len(gList.Groups), body)
	}
	for _, k := range []string{"id", "name", "type", "regex", "url", "interval"} {
		if _, ok := gList.Groups[0][k]; !ok {
			t.Errorf("GET /api/groups 元素缺少 %q: %s", k, body)
		}
	}

	status, _, body = uiGet(t, ts, fmt.Sprintf("/api/groups/%d", groupID))
	if status != http.StatusOK {
		t.Fatalf("GET /api/groups/{id} 状态 = %d，body=%s", status, body)
	}
	var gGet struct {
		Group struct {
			ID int64 `json:"id"`
		} `json:"group"`
	}
	mustJSON(t, body, &gGet)
	if gGet.Group.ID != groupID {
		t.Errorf("GET /api/groups/{id} 返回 id=%d，期望 %d: %s", gGet.Group.ID, groupID, body)
	}

	status, _, body = uiDoJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/groups/%d", groupID),
		`{"name":"AutoSelect2","type":"url-test","regex":".*","url":"http://www.gstatic.com/generate_204","interval":60}`)
	if status < 200 || status >= 300 {
		t.Fatalf("PUT /api/groups/{id} 状态 = %d，body=%s", status, body)
	}
	var gPut struct {
		Group struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Interval int    `json:"interval"`
		} `json:"group"`
	}
	mustJSON(t, body, &gPut)
	if gPut.Group.Name != "AutoSelect2" || gPut.Group.Type != "url-test" || gPut.Group.Interval != 60 {
		t.Errorf("PUT /api/groups/{id} 未生效: %s", body)
	}

	// --- /api/rules ---
	status, _, body = uiPostJSON(t, ts, "/api/rules",
		fmt.Sprintf(`{"rule_type":"DOMAIN-SUFFIX","rule_param":"example.com","rule_config":%d,"resolve_dns":true,"enabled":true}`, groupID))
	if status < 200 || status >= 300 {
		t.Fatalf("POST /api/rules 状态 = %d，应为 2xx，body=%s", status, body)
	}
	var rCreate struct {
		Rule struct {
			ID         int64  `json:"id"`
			RuleType   string `json:"rule_type"`
			RuleParam  string `json:"rule_param"`
			RuleConfig int64  `json:"rule_config"`
			ResolveDNS bool   `json:"resolve_dns"`
			Enabled    bool   `json:"enabled"`
		} `json:"rule"`
	}
	mustJSON(t, body, &rCreate)
	if rCreate.Rule.ID == 0 || rCreate.Rule.RuleType != "DOMAIN-SUFFIX" ||
		rCreate.Rule.RuleParam != "example.com" || rCreate.Rule.RuleConfig != groupID ||
		!rCreate.Rule.ResolveDNS || !rCreate.Rule.Enabled {
		t.Fatalf("POST /api/rules 返回体不符契约: %s", body)
	}
	ruleID := rCreate.Rule.ID

	status, _, body = uiGet(t, ts, "/api/rules")
	if status != http.StatusOK {
		t.Fatalf("GET /api/rules 状态 = %d，body=%s", status, body)
	}
	var rList struct {
		Rules []map[string]any `json:"rules"`
	}
	mustJSON(t, body, &rList)
	if len(rList.Rules) != 1 {
		t.Fatalf("GET /api/rules 应返回 1 条，实际 %d: %s", len(rList.Rules), body)
	}
	for _, k := range []string{"id", "rule_type", "rule_param", "rule_config", "resolve_dns", "enabled", "sort_order"} {
		if _, ok := rList.Rules[0][k]; !ok {
			t.Errorf("GET /api/rules 元素缺少 %q: %s", k, body)
		}
	}
	if _, ok := rList.Rules[0]["rule_config"].(float64); !ok {
		t.Errorf("GET /api/rules 的 rule_config 必须是数字（目标 group id），实际 %T: %s", rList.Rules[0]["rule_config"], body)
	}

	status, _, body = uiGet(t, ts, fmt.Sprintf("/api/rules/%d", ruleID))
	if status != http.StatusOK {
		t.Fatalf("GET /api/rules/{id} 状态 = %d，body=%s", status, body)
	}
	var rGet struct {
		Rule struct {
			ID int64 `json:"id"`
		} `json:"rule"`
	}
	mustJSON(t, body, &rGet)
	if rGet.Rule.ID != ruleID {
		t.Errorf("GET /api/rules/{id} 返回 id=%d，期望 %d: %s", rGet.Rule.ID, ruleID, body)
	}

	status, _, body = uiDoJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", ruleID),
		fmt.Sprintf(`{"rule_type":"GEOIP","rule_param":"CN","rule_config":%d,"resolve_dns":false,"enabled":false}`, groupID))
	if status < 200 || status >= 300 {
		t.Fatalf("PUT /api/rules/{id} 状态 = %d，body=%s", status, body)
	}
	var rPut struct {
		Rule struct {
			RuleType   string `json:"rule_type"`
			RuleParam  string `json:"rule_param"`
			ResolveDNS bool   `json:"resolve_dns"`
			Enabled    bool   `json:"enabled"`
		} `json:"rule"`
	}
	mustJSON(t, body, &rPut)
	if rPut.Rule.RuleType != "GEOIP" || rPut.Rule.RuleParam != "CN" || rPut.Rule.ResolveDNS || rPut.Rule.Enabled {
		t.Errorf("PUT /api/rules/{id} 未生效: %s", body)
	}

	// --- /api/tokens ---
	status, _, body = uiPostJSON(t, ts, "/api/tokens", `{"name":"手机订阅"}`)
	if status < 200 || status >= 300 {
		t.Fatalf("POST /api/tokens 状态 = %d，应为 2xx，body=%s", status, body)
	}
	var tCreate struct {
		Token struct {
			Token     string `json:"token"`
			Name      string `json:"name"`
			CreatedAt string `json:"created_at"`
		} `json:"token"`
	}
	mustJSON(t, body, &tCreate)
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(tCreate.Token.Token) {
		t.Errorf("POST /api/tokens 留空 token 时服务端应生成 32 位 hex，实际 %q: %s", tCreate.Token.Token, body)
	}
	if tCreate.Token.Name != "手机订阅" {
		t.Errorf("POST /api/tokens 返回的 name = %q，期望 手机订阅: %s", tCreate.Token.Name, body)
	}
	generated := tCreate.Token.Token

	status, _, body = uiPostJSON(t, ts, "/api/tokens", `{"name":"固定令牌","token":"fixed-token-1"}`)
	if status < 200 || status >= 300 {
		t.Fatalf("POST /api/tokens(指定 token) 状态 = %d，body=%s", status, body)
	}
	var tFixed struct {
		Token struct {
			Token string `json:"token"`
		} `json:"token"`
	}
	mustJSON(t, body, &tFixed)
	if tFixed.Token.Token != "fixed-token-1" {
		t.Errorf("POST /api/tokens 指定 token 时应原样返回，实际 %q: %s", tFixed.Token.Token, body)
	}

	status, _, body = uiGet(t, ts, "/api/tokens")
	if status != http.StatusOK {
		t.Fatalf("GET /api/tokens 状态 = %d，body=%s", status, body)
	}
	var tList struct {
		Tokens []struct {
			Token     string `json:"token"`
			Name      string `json:"name"`
			CreatedAt string `json:"created_at"`
		} `json:"tokens"`
	}
	mustJSON(t, body, &tList)
	if len(tList.Tokens) != 2 {
		t.Fatalf("GET /api/tokens 应返回 2 条，实际 %d: %s", len(tList.Tokens), body)
	}
	found := false
	for _, tok := range tList.Tokens {
		if tok.Token == generated {
			found = true
		}
		if tok.Token == "" || tok.CreatedAt == "" {
			t.Errorf("GET /api/tokens 元素缺少 token/created_at: %s", body)
		}
	}
	if !found {
		t.Errorf("GET /api/tokens 未包含刚生成的令牌 %q: %s", generated, body)
	}

	status, _, body = uiDoJSON(t, ts, http.MethodDelete, "/api/tokens/fixed-token-1", "")
	if status < 200 || status >= 300 {
		t.Fatalf("DELETE /api/tokens/{token} 状态 = %d，body=%s", status, body)
	}
	var okToken struct {
		OK bool `json:"ok"`
	}
	mustJSON(t, body, &okToken)
	if !okToken.OK {
		t.Errorf("DELETE /api/tokens/{token} 应返回 {\"ok\":true}: %s", body)
	}

	// --- 统一 JSON 错误 ---
	for _, p := range []string{"/api/airports/999999", "/api/groups/999999", "/api/rules/999999"} {
		status, hdr, body := uiGet(t, ts, p)
		if status != http.StatusNotFound {
			t.Errorf("GET %s 状态 = %d，应为 404，body=%s", p, status, body)
			continue
		}
		uiAssertJSONError(t, "GET "+p, body, hdr)
	}
	for _, p := range []string{"/api/airports", "/api/groups", "/api/rules", "/api/tokens"} {
		status, hdr, body := uiPostJSON(t, ts, p, "{oops")
		if status != http.StatusBadRequest {
			t.Errorf("POST %s 坏 body 状态 = %d，应为 400，body=%s", p, status, body)
			continue
		}
		uiAssertJSONError(t, "POST "+p, body, hdr)
	}

	// 收尾：删除规则与分组，顺带验证 DELETE 契约。
	status, _, body = uiDoJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/rules/%d", ruleID), "")
	if status < 200 || status >= 300 {
		t.Fatalf("DELETE /api/rules/{id} 状态 = %d，body=%s", status, body)
	}
	var okRule struct {
		OK bool `json:"ok"`
	}
	mustJSON(t, body, &okRule)
	if !okRule.OK {
		t.Errorf("DELETE /api/rules/{id} 应返回 {\"ok\":true}: %s", body)
	}
	status, _, body = uiDoJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/groups/%d", groupID), "")
	if status < 200 || status >= 300 {
		t.Fatalf("DELETE /api/groups/{id} 状态 = %d，body=%s", status, body)
	}
	var okGroup struct {
		OK bool `json:"ok"`
	}
	mustJSON(t, body, &okGroup)
	if !okGroup.OK {
		t.Errorf("DELETE /api/groups/{id} 应返回 {\"ok\":true}: %s", body)
	}
}

// TestWebUI_SubscribeLiveCompile /subscribe 必须每次请求按 DB 当前状态实时编译出配置：
// 用临时 SQLite 插入一条 airport + 一个 group + 一条 rule，断言产物包含 group 名，
// 且 rule 行是 TYPE,PARAM,POLICY 三列（POLICY = rule_config 指向的 group 名），
// resolve_dns 为真时追加 ,no-resolve；rule_config=0 落成 DIRECT。
//
// 只断言"行内三列"这一语义，不断言 YAML 缩进/换行等排版细节。
func TestWebUI_SubscribeLiveCompile(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	// 本地夹具当上游（12 个节点，见 server_test.go:fixtureURL），不依赖外网。
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	group, err := mgr.AddProxyGroup(ctx, "AutoSelect", "select", ".*", "", 0)
	if err != nil {
		t.Fatalf("插入 group: %v", err)
	}
	if _, err := mgr.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", group.ID, true, true); err != nil {
		t.Fatalf("插入 rule: %v", err)
	}
	if _, err := mgr.AddRule(ctx, "GEOIP", "CN", 0, false, true); err != nil {
		t.Fatalf("插入 DIRECT rule: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	status, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	if status != http.StatusOK {
		t.Fatalf("/subscribe 状态 = %d，应为 200，body=%s", status, body)
	}
	// 实时编译的配置是文本，不是 JSON：SF.api 按 Content-Type 分支解析。
	if ct := hdr.Get("Content-Type"); strings.Contains(ct, "application/json") {
		t.Errorf("/subscribe Content-Type = %q，应为配置文本而不是 JSON", ct)
	}
	if !strings.Contains(body, "AutoSelect") {
		t.Errorf("/subscribe 产物缺少分组名 AutoSelect:\n%s", body)
	}
	// rule 行 = TYPE,PARAM,POLICY；policy 必须解析成 rule_config 指向的 group 名。
	if !strings.Contains(body, "DOMAIN-SUFFIX,example.com,AutoSelect,no-resolve") {
		t.Errorf("/subscribe 产物缺少规则行 DOMAIN-SUFFIX,example.com,AutoSelect,no-resolve:\n%s", body)
	}
	// rule_config=0 → DIRECT。
	if !strings.Contains(body, "GEOIP,CN,DIRECT") {
		t.Errorf("/subscribe 产物缺少规则行 GEOIP,CN,DIRECT:\n%s", body)
	}
}

// TestWebUI_SubscribeBypassesAccessToken 锁住 /subscribe 的鉴权边界。
//
// 服务端配了访问令牌时，页面与 /api/* 都要 Bearer；但 /subscribe 用的是订阅令牌
// （sub_token），若被全局 auth 拦下，客户端永远拿到 401——这正是修复前的行为：
// middleware.go 的 presented() 会把 query 上的 token 拿去和服务端令牌比对。
func TestWebUI_SubscribeBypassesAccessToken(t *testing.T) {
	const access = "access-token-1"
	ts, mgr := uiSubServerCfg(t, func(cfg *Config) {
		cfg.Token = func() string { return access }
	})
	ctx := context.Background()

	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	// 不带访问令牌访问 /api/*：仍然必须 401（证明全局鉴权没有被削弱）。
	if status, _, _ := uiGet(t, ts, "/api/airports"); status != http.StatusUnauthorized {
		t.Errorf("GET /api/airports 无令牌状态 = %d，应为 401", status)
	}
	// /subscribe 带订阅令牌：必须 200，不能被访问令牌校验拦成 401。
	status, _, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if status != http.StatusOK {
		t.Fatalf("/subscribe 带订阅令牌状态 = %d，应为 200，body=%s", status, body)
	}
	// 错的订阅令牌：404（订阅令牌校验的语义），不是 401。
	if status, _, _ := uiGet(t, ts, "/subscribe?token=nope"); status != http.StatusNotFound {
		t.Errorf("/subscribe 错误订阅令牌状态 = %d，应为 404", status)
	}
}
