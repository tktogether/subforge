// web_ui_test.go — M2 嵌入式 Web UI（internal/api/web/）与 /api/* JSON 端点的回归测试。
//
// 约束：全部走本地 httptest，不依赖外网、不依赖真实订阅。需要 Engine 的构造沿用
// server_test.go 的 newTestServer（最小 Engine，且以下用例都不触发上游拉取）。
//
// 覆盖三件事：
//  1. 页面/静态资源真的能取到、Content-Type 与前端判断一致；
//  2. 前端契约（DOM id、fetch 路径）与后端路由表/HTML 对齐；
//  3. /api/targets、/api/diff、/api/convert 的行为契约。
package api

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
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

// uiReadWeb 读一份嵌入的 web 资源（页面或脚本）。
func uiReadWeb(t *testing.T, name string) string {
	t.Helper()
	buf, err := fs.ReadFile(webFS, name)
	if err != nil {
		t.Fatalf("读取嵌入资源 %s: %v", name, err)
	}
	return string(buf)
}

// --- 页面 ---

// TestWebUI_Pages 四个页面都必须 200、text/html，并带各自的标志性 DOM 标记。
// 标记取自 web/js/*.js 里 SF.qs("#...") 的实际取值：缺任何一个，页面脚本在
// DOMContentLoaded 里就会抛 TypeError，而服务端仍然返回 200——只看状态码看不出来。
func TestWebUI_Pages(t *testing.T) {
	ts := newTestServer(t, nil)
	cases := []struct {
		path    string
		title   string
		markers []string
	}{
		{"/", "SubForge · Dashboard", []string{
			`id="qcRun"`, `id="qcUrl"`, `id="qcTarget"`, `id="qcOut"`,
			`id="statsBox"`, `id="historyBox"`, `src="/js/dashboard.js"`,
		}},
		{"/convert", "SubForge · Convert", []string{
			`id="cvRun"`, `id="cvTarget"`, `id="cvOverride"`, `id="cvDiff"`,
			`id="cvDiffOut"`, `id="cvOut"`, `src="/js/convert.js"`,
		}},
		{"/rules", "SubForge · Rules", []string{
			`id="reEditor"`, `id="reMode"`, `id="rulesetsBox"`, `src="/js/rules.js"`,
		}},
		{"/logs", "SubForge · Logs", []string{
			`id="logBox"`, `id="logFilter"`, `id="logExportJson"`, `src="/js/logs.js"`,
		}},
	}
	for _, c := range cases {
		status, hdr, body := uiGet(t, ts, c.path)
		if status != http.StatusOK {
			t.Errorf("%s 状态 = %d，应为 200", c.path, status)
			continue
		}
		if got := hdr.Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("%s Content-Type = %q，应为 text/html; charset=utf-8", c.path, got)
		}
		if !strings.Contains(body, "<title>"+c.title+"</title>") {
			t.Errorf("%s 的 <title> 不是 %q", c.path, c.title)
		}
		for _, m := range c.markers {
			if !strings.Contains(body, m) {
				t.Errorf("%s 缺少关键标记 %q", c.path, m)
			}
		}
		// 共享运行时必须先于页面脚本：app.js 定义 window.SF，页面脚本在顶层就取它。
		if i, j := strings.Index(body, `src="/app.js"`), strings.Index(body, c.markers[len(c.markers)-1]); i < 0 || j < 0 || i > j {
			t.Errorf("%s 应先加载 /app.js 再加载页面脚本（app.js@%d, 页面脚本@%d）", c.path, i, j)
		}
	}
}

// --- 静态资源 ---

// TestWebUI_StaticAssets Content-Type 必须与前端判断匹配：SF.api 只在
// Content-Type 含 application/json 时走 res.json()，其余一律 res.text()。
func TestWebUI_StaticAssets(t *testing.T) {
	ts := newTestServer(t, nil)
	cases := []struct {
		path string
		ct   string
	}{
		{"/app.css", "text/css; charset=utf-8"},
		{"/app.js", "application/javascript; charset=utf-8"},
		{"/js/dashboard.js", "application/javascript; charset=utf-8"},
		{"/js/convert.js", "application/javascript; charset=utf-8"},
		{"/js/rules.js", "application/javascript; charset=utf-8"},
		{"/js/logs.js", "application/javascript; charset=utf-8"},
	}
	bodies := map[string]string{}
	for _, c := range cases {
		status, hdr, body := uiGet(t, ts, c.path)
		if status != http.StatusOK {
			t.Errorf("%s 状态 = %d，应为 200", c.path, status)
			continue
		}
		if got := hdr.Get("Content-Type"); got != c.ct {
			t.Errorf("%s Content-Type = %q，应为 %q", c.path, got, c.ct)
		}
		if strings.TrimSpace(body) == "" {
			t.Errorf("%s 正文为空", c.path)
		}
		bodies[c.path] = body
	}
	app := bodies["/app.js"]
	if !strings.Contains(app, "window.SF") {
		t.Error("/app.js 未暴露 window.SF，页面脚本拿不到共享运行时")
	}
	if !strings.Contains(app, `indexOf("application/json")`) {
		t.Error("/app.js 的 SF.api 不再按 application/json 判断 res.json()，前端契约变了")
	}
	if !strings.Contains(bodies["/app.css"], "data-theme") {
		t.Error("/app.css 缺少 data-theme 主题钩子")
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

// --- 前端契约：DOM id 与 fetch 路径 ---

var (
	uiQSSel   = regexp.MustCompile(`SF\.qs\(\s*"#([A-Za-z0-9_-]+)"`)
	uiGetByID = regexp.MustCompile(`getElementById\(\s*"([A-Za-z0-9_-]+)"`)
	uiAPIPath = regexp.MustCompile(`SF\.api\(\s*"(/[^"]*)"`)
	uiIDAttr  = regexp.MustCompile(`id="([A-Za-z0-9_-]+)"`)
)

// uiIDs 从正则子匹配里取第 1 组。
func uiIDs(matches [][]string) []string {
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// TestWebUI_DomIDsMatchScripts 前端脚本引用的每个 DOM id 必须真实存在于对应页面。
// 这是“页面能点”的最低契约：SF.qs 返回 null 后紧接着 .value / .addEventListener
// 会抛 TypeError，该行之后的初始化全部失效，而服务端仍然返回 200。
func TestWebUI_DomIDsMatchScripts(t *testing.T) {
	pages := []struct{ html, js string }{
		{"index.html", "js/dashboard.js"},
		{"convert.html", "js/convert.js"},
		{"rules.html", "js/rules.js"},
		{"logs.html", "js/logs.js"},
	}
	appIDs := uiIDs(uiGetByID.FindAllStringSubmatch(uiReadWeb(t, "app.js"), -1))
	if len(appIDs) == 0 {
		t.Fatal("app.js 未解析出任何 DOM id，检查提取正则")
	}
	for _, p := range pages {
		html := uiReadWeb(t, p.html)
		js := uiReadWeb(t, p.js)
		have := map[string]bool{}
		for _, m := range uiIDAttr.FindAllStringSubmatch(html, -1) {
			have[m[1]] = true
		}
		want := uiIDs(uiQSSel.FindAllStringSubmatch(js, -1))
		want = append(want, uiIDs(uiGetByID.FindAllStringSubmatch(js, -1))...)
		want = append(want, appIDs...)
		if len(want) == 0 {
			t.Errorf("%s 未解析出任何 DOM id，检查提取正则", p.js)
			continue
		}
		for _, id := range want {
			if !have[id] {
				t.Errorf("%s 引用了 %s 里不存在的 id=%q", p.js, p.html, id)
			}
		}
	}
}

// TestWebUI_ScriptAPIPathsAreRouted 前端 fetch 的每个路径都必须真的挂在路由表上。
// 前端契约与后端路由表分处两个文件，路径写错时页面只会静默 catch，没人会发现。
func TestWebUI_ScriptAPIPathsAreRouted(t *testing.T) {
	ts := newTestServer(t, nil)
	seen := map[string]bool{}
	for _, s := range []string{"app.js", "js/dashboard.js", "js/convert.js", "js/rules.js", "js/logs.js"} {
		for _, m := range uiAPIPath.FindAllStringSubmatch(uiReadWeb(t, s), -1) {
			p := m[1]
			if i := strings.IndexByte(p, '?'); i >= 0 {
				p = p[:i]
			}
			seen[p] = true
		}
	}
	// 期望集合写死：既防“提取不到”时静默通过，也锁住前端用到的端点清单。
	for _, want := range []string{
		"/api/targets", "/api/stats", "/api/history", "/api/pipelines",
		"/api/rulesets", "/api/logs", "/api/convert", "/api/diff",
	} {
		if !seen[want] {
			t.Errorf("前端脚本未引用 %s（契约漂移：端点被删或改名）", want)
		}
	}
	for p := range seen {
		var status int
		if p == "/api/convert" || p == "/api/diff" {
			status, _, _ = uiPostJSON(t, ts, p, "{}")
		} else {
			status, _, _ = uiGet(t, ts, p)
		}
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			t.Errorf("前端引用的 %s 没有对应路由（状态 %d）", p, status)
		}
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
