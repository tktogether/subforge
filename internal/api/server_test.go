package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/subforge"
)

// quietLogger 让测试输出只留断言。
func quietLogger() *log.Logger { return log.New(io.Discard, "", 0) }

// fixtureURL 起一个假上游吐混合订阅 fixture，返回它的地址。
func fixtureURL(t *testing.T) string {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf)
	}))
	t.Cleanup(up.Close)
	return up.URL + "/sub.txt"
}

// newTestServer 起一个挂好全部中间件的 Server。
func newTestServer(t *testing.T, cfgMod func(*Config)) *httptest.Server {
	t.Helper()
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
	ts := httptest.NewServer(New(cfg).Routes())
	t.Cleanup(ts.Close)
	return ts
}

// doGet 发一个 GET，带回状态码、响应头与响应体。
func doGet(t *testing.T, u string, ua, bearer string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

// --- /sub 主路径 ---

func TestConvert_EndToEnd(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)

	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub), "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("Content-Type"); !strings.HasPrefix(got, "text/yaml") {
		t.Errorf("Content-Type = %q", got)
	}
	if got := hdr.Get("X-Node-Count"); got != "12" {
		t.Errorf("X-Node-Count = %q，应为 12", got)
	}
	if !strings.Contains(body, "proxies:") || !strings.Contains(body, "proxy-groups:") {
		t.Errorf("产物缺少 proxies / proxy-groups 段落")
	}
}

func TestConvert_Aliases(t *testing.T) {
	sub := fixtureURL(t)
	for _, path := range []string{"/sub", "/profile", "/render"} {
		ts := newTestServer(t, nil)
		status, _, body := doGet(t, ts.URL+path+"?url="+url.QueryEscape(sub), "", "")
		if status != http.StatusOK || !strings.Contains(body, "proxies:") {
			t.Errorf("%s: 状态 %d", path, status)
		}
	}
}

// TestConvert_TargetFromUserAgent 缺省 target 时按 UA 推断成对应配置形态。
func TestConvert_TargetFromUserAgent(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	u := ts.URL + "/sub?url=" + url.QueryEscape(sub)

	// Surge 的 UA 落到 surge 配置形态，产物带 [General] / [Proxy] 段。
	status, _, body := doGet(t, u, "Surge 6.2", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，应为 200，body=%s", status, body)
	}
	if !strings.Contains(body, "[General]") || !strings.Contains(body, "[Proxy]") {
		t.Errorf("Surge UA 应产出 Surge 配置形态，实际: %s", body)
	}

	status, _, body = doGet(t, u, "Clash Verge 2.0", "")
	if status != http.StatusOK || !strings.Contains(body, "proxies:") {
		t.Fatalf("Clash UA 应返回 200，实际 %d", status)
	}
}

// TestConvert_UnsupportedTarget 未实现的格式必须报 501，不能静默回落到默认配置。
func TestConvert_UnsupportedTarget(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	for _, target := range []string{"srr", "vles", "ssr2", "quantumult6"} {
		status, _, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&target="+target, "", "")
		if status != http.StatusNotImplemented {
			t.Errorf("target=%q 状态 = %d，应为 501，body=%s", target, status, body)
			continue
		}
		if !strings.Contains(body, target) {
			t.Errorf("target=%q 错误信息未指出缺失格式: %s", target, body)
		}
	}
}

// TestConvert_UnsupportedTargetSkipsFetch target 写错时应立刻报 501，而不是先
// 把上游拉一遍。状态码在两种实现里都是 501（渲染器查找终究会失败），真正的
// 差别是一次本该省掉的真实网络请求，所以断言在计数上。
func TestConvert_UnsupportedTargetSkipsFetch(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}
	var fetched atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched.Add(1)
		_, _ = w.Write(buf)
	}))
	defer up.Close()

	ts := newTestServer(t, nil)
	status, _, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(up.URL+"/sub.txt")+
		"&target=notarealformat", "", "")
	if status != http.StatusNotImplemented {
		t.Fatalf("状态 = %d，应为 501，body=%s", status, body)
	}
	if !strings.Contains(body, "notarealformat") {
		t.Errorf("错误信息未指出缺失格式: %s", body)
	}
	if n := fetched.Load(); n != 0 {
		t.Errorf("target 非法时不应尝试拉取上游，实际拉了 %d 次", n)
	}
}

// TestConvert_TemplateInlineSyntaxApplied 模板用 subconverter 的内联写法
// （[custom] 里的 custom_proxy_group= / ruleset=）声明分组与规则时，产物里必须真的
// 出现这些分组和规则——这是公开模板（Aethersailor Custom_Clash_Full.ini）唯一在用
// 的写法，读不懂等于整份分流失效。
func TestConvert_TemplateInlineSyntaxApplied(t *testing.T) {
	sub := fixtureURL(t)

	// 内联 DSL 写不进原样字符串（反引号是定界符），用普通字符串。
	// 🎯 全球直连 必须显式声明：它既是 ruleset 的目标，也会出现在产物的规则行里，
	// 少了这一行就等于让规则指向不存在的出站（mihomo 会整份拒绝加载）。
	inini := "[custom]\n" +
		"custom_proxy_group=🚀 手动选择`select`[]♻️ 自动选择`.*\n" +
		"custom_proxy_group=🎯 全球直连`select`[]DIRECT\n" +
		"ruleset=🎯 全球直连,[]GEOSITE,private\n" +
		"ruleset=🚀 手动选择,[]FINAL\n"

	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(inini))
	}))
	t.Cleanup(tpl.Close)

	ts := newTestServer(t, func(cfg *Config) {
		cfg.Engine.LoadTemplate = func(ctx context.Context, ref string) (*config.Template, error) {
			return config.LoadSource(ctx, ref, config.HTTPLoader(&http.Client{Timeout: 5 * time.Second}, nil))
		}
	})

	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+
		"&target=clash&config="+url.QueryEscape(tpl.URL+"/tpl.ini"), "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("X-Convert-Skipped"); got != "" {
		t.Errorf("X-Convert-Skipped = %q，内联写法已解析，不该有跳过项", got)
	}
	for _, want := range []string{"🚀 手动选择", "GEOSITE,private,🎯 全球直连", "MATCH,🚀 手动选择"} {
		if !strings.Contains(body, want) {
			t.Errorf("产物缺少 %q", want)
		}
	}
	// mihomo 没有 FINAL 这个规则形态，内联的 []FINAL 必须落成 MATCH。
	if strings.Contains(body, "FINAL,") {
		t.Error("产物里出现了 FINAL 规则，mihomo 会拒绝加载")
	}
}

func TestConvert_NoSource(t *testing.T) {
	ts := newTestServer(t, nil)
	status, _, body := doGet(t, ts.URL+"/sub", "", "")
	if status != http.StatusBadRequest {
		t.Fatalf("状态 = %d，应为 400，body=%s", status, body)
	}
	if !strings.Contains(body, "至少需要") {
		t.Errorf("错误信息: %s", body)
	}
}

// TestConvert_ListMode list 模式输出分享链接清单：每行一条，纯文本无注释头，
// 过滤/改名/排序与配置形态共用同一条 Engine 路径。
func TestConvert_ListMode(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)

	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&list=1", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("X-Convert-Format"); got != "list" {
		t.Errorf("X-Convert-Format = %q，应为 list", got)
	}
	if got := hdr.Get("X-Node-Count"); got != "12" {
		t.Errorf("X-Node-Count = %q，应为 12", got)
	}
	if !strings.HasPrefix(hdr.Get("Content-Type"), "text/plain") {
		t.Errorf("Content-Type = %q，应为纯文本", hdr.Get("Content-Type"))
	}
	if got := hdr.Get("Content-Disposition"); !strings.Contains(got, "filename=list.txt") {
		t.Errorf("Content-Disposition = %q，应给出 list.txt", got)
	}

	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 12 {
		t.Fatalf("应输出 12 行链接，实际 %d 行:\n%s", len(lines), body)
	}
	var schemes []string
	for i, l := range lines {
		if strings.Contains(l, "proxies:") || strings.HasPrefix(l, "#") {
			t.Errorf("第 %d 行不应含 YAML 段落或注释头: %q", i+1, l)
		}
		if j := strings.Index(l, "://"); j > 0 {
			schemes = append(schemes, l[:j])
		} else {
			t.Errorf("第 %d 行不是分享链接: %q", i+1, l)
		}
	}
	for _, want := range []string{"vmess", "vless", "hysteria2", "hysteria", "tuic",
		"ss", "ssr", "trojan", "anytls", "socks5", "http"} {
		if !contains(schemes, want) {
			t.Errorf("缺少 %s:// 节点，实际协议: %v", want, schemes)
		}
	}
}

// TestConvert_ListModeOverridesTarget -list 决定输出形态，不能把输出拉回客户端配置。
func TestConvert_ListModeOverridesTarget(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&target=clash.meta&list=1", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("X-Convert-Format"); got != "list" {
		t.Errorf("X-Convert-Format = %q，-list 应覆盖 target", got)
	}
	if strings.Contains(body, "proxies:") {
		t.Error("-list 与 target 同时给出时应输出纯链接清单，而不是配置")
	}
}

// TestConvert_TargetList target=list 与 list 参数等价。
func TestConvert_TargetList(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&target=list", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("X-Convert-Format"); got != "list" {
		t.Errorf("X-Convert-Format = %q", got)
	}
	if strings.Count(body, "://") < 12 {
		t.Errorf("产物应为分享链接清单: %s", body)
	}
}

// TestConvert_LinkTargets subconverter 的 mixed / ss / ssd / ssr / trojan / v2ray 都是
// base64 包裹的分享链接清单，有的还要按协议收窄。收窄取值来自 subconverter 实机输出，
// 断言节点数与解包后的行数一致，防止收窄逻辑悄悄漂移。
func TestConvert_LinkTargets(t *testing.T) {
	sub := fixtureURL(t)
	cases := []struct {
		target string
		want   int
	}{
		{"mixed", 12},
		{"v2ray", 1},
		{"ss", 2},
		{"ssd", 2},
		{"ssr", 1},
		{"trojan", 1},
	}
	for _, c := range cases {
		ts := newTestServer(t, nil)
		status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&target="+c.target, "", "")
		if status != http.StatusOK {
			t.Errorf("%s: 状态 %d，body=%s", c.target, status, body)
			continue
		}
		if got := hdr.Get("X-Convert-Format"); got != "mixed" {
			t.Errorf("%s: X-Convert-Format = %q，应为 mixed", c.target, got)
		}
		blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(body))
		if err != nil {
			t.Errorf("%s: 产物不是合法 base64: %v", c.target, err)
			continue
		}
		lines := strings.Split(strings.TrimSuffix(string(blob), "\n"), "\n")
		if len(lines) != c.want {
			t.Errorf("%s: 解包后 %d 行，期望 %d\n%s", c.target, len(lines), c.want, blob)
		}
		if got := hdr.Get("X-Node-Count"); got != fmt.Sprintf("%d", c.want) {
			t.Errorf("%s: X-Node-Count = %q，收窄后应为 %d", c.target, got, c.want)
		}
	}
}

// TestConvert_LinkTargetTypeOverride 显式 -type 比 target 隐含的收窄更具体，应当胜出。
func TestConvert_LinkTargetTypeOverride(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	status, _, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&target=ssr&type=vless", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(body))
	if err != nil {
		t.Fatalf("产物不是合法 base64: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(blob), "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("期望 -type vless 生效（夹具里 2 个 vless 节点），实际 %d 行\n%s", len(lines), blob)
	}
}

// TestConvert_ListModeReportsSkipped 不支持分享链接的协议要作为警告上报，不能静默丢节点。
func TestConvert_ListModeReportsSkipped(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n" +
			"  - name: vless-ok\n" +
			"    type: vless\n" +
			"    server: 1.2.3.4\n" +
			"    port: \"443\"\n" +
			"    uuid: 11111111-2222-3333-4444-555555555555\n" +
			"  - name: ssh-bad\n" +
			"    type: ssh\n" +
			"    server: 1.2.3.4\n" +
			"    port: \"22\"\n"))
	}))
	t.Cleanup(up.Close)
	ts := newTestServer(t, nil)

	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(up.URL)+"&list=1", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("X-Convert-Skipped"); got != "1" {
		t.Errorf("X-Convert-Skipped = %q，应为 1", got)
	}
	if got := hdr.Get("X-Node-Count"); got != "2" {
		t.Errorf("X-Node-Count = %q，管线看到的是 2 个节点", got)
	}
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "vless://") {
		t.Errorf("应只输出可序列化的那一条:\n%s", body)
	}
}

func TestConvert_BadRegexRejected(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	status, _, _ := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub)+"&include=%28", "", "")
	if status != http.StatusBadRequest {
		t.Fatalf("状态 = %d，应为 400", status)
	}
}

// TestConvert_FilterRenameSort 过滤、改名与排序在 HTTP 路径上同样生效。
func TestConvert_FilterRenameSort(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	u := ts.URL + "/sub?url=" + url.QueryEscape(sub) +
		"&type=socks5%2Chttp&rename=-node%2C节点&sort=name"
	status, hdr, body := doGet(t, u, "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d", status)
	}
	if got := hdr.Get("X-Node-Count"); got != "2" {
		t.Errorf("X-Node-Count = %q，应为 2", got)
	}
	if !strings.Contains(body, "http节点") || !strings.Contains(body, "socks节点") {
		t.Errorf("改名未生效:\n%s", body)
	}
	if strings.Contains(body, "vmess-node") {
		t.Errorf("类型过滤未生效")
	}
	if i, j := strings.Index(body, "http节点"), strings.Index(body, "socks节点"); !(i >= 0 && i < j) {
		t.Errorf("按名称排序后 http 节点应排在 socks 节点前面")
	}
}

// --- INI 模板覆盖链 ---

// tplINI 是一份各段落都占位的测试模板，模板链路的断言都建在它上面。
//
// 行尾空格是前缀本身的一部分（前缀原样拼接，不会自动补空格），
// 所以逐行写出而不是用裸字符串常量；规则写 MATCH 而不是 FINAL，因为 mihomo 没有 FINAL 规则类型。
var tplINI = strings.Join([]string{
	"[common]",
	"sort = name",
	"include = ^vless-",
	"rules = GEOIP,CN,DIRECT;MATCH,自动选择",
	"",
	"[node_pref]",
	"vless = [VLESS] ",
	"include = ^vless-reality",
	"",
	"[clash_proxy_group]",
	"name = 自动选择",
	"type = url-test",
	"proxies = vless",
	"interval = 300",
	"---",
	"name = 手动选择",
	"type = select",
	"proxies = .*",
	"",
}, "\n")

// writeTpl 把模板文本落盘并解析，返回可直接交给引擎的模板。
func writeTpl(t *testing.T, body string) *config.Template {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tpl.ini")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tpl, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("加载模板: %v", err)
	}
	return tpl
}

// TestConvert_TemplateApplied 本地模板的各段落在 HTTP 路径上全部生效。
func TestConvert_TemplateApplied(t *testing.T) {
	sub := fixtureURL(t)
	tpl := writeTpl(t, tplINI)
	ts := newTestServer(t, func(c *Config) {
		c.Engine.BaseTemplate = func() *config.Template { return tpl }
	})
	status, hdr, body := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub), "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	// [common] include 把 12 个节点收敛成 2 个 vless 节点。
	if got := hdr.Get("X-Node-Count"); got != "2" {
		t.Errorf("X-Node-Count = %q，应为 2", got)
	}
	// [node_pref] 只给 include 范围内的 vless 节点加前缀。
	if !strings.Contains(body, "[VLESS] vless-reality") {
		t.Errorf("include 范围内的 vless 节点应带 [VLESS] 前缀:\n%s", body)
	}
	if strings.Contains(body, "[VLESS] vless-ws") {
		t.Errorf("include 范围外的 vless-ws 不应加前缀")
	}
	// [clash_proxy_group] 两个组都在，成员按节点名展开成实际节点。
	if got := strings.Count(body, "name: 自动选择"); got != 1 {
		t.Errorf("第一个代理组应出现 1 次，实际 %d", got)
	}
	if !strings.Contains(body, "interval: 300") {
		t.Errorf("代理组 interval 未生效:\n%s", body)
	}
	if !strings.Contains(body, "name: 手动选择") {
		t.Errorf("第二个代理组（--- 分隔）缺失")
	}
	// [common] rules 取代内置默认规则。
	if !strings.Contains(body, "GEOIP,CN,DIRECT") || !strings.Contains(body, "MATCH,自动选择") {
		t.Errorf("模板规则未生效:\n%s", body)
	}
	if strings.Contains(body, "MATCH,手动选择") {
		t.Errorf("出现了内置默认兜底规则，模板 rules 应覆盖它")
	}
}

// TestConvert_QueryOverridesTemplate URL 参数压过模板：同名键整键替换，结构化字段不受影响。
func TestConvert_QueryOverridesTemplate(t *testing.T) {
	sub := fixtureURL(t)
	tpl := writeTpl(t, tplINI)
	ts := newTestServer(t, func(c *Config) {
		c.Engine.BaseTemplate = func() *config.Template { return tpl }
	})
	u := ts.URL + "/sub?url=" + url.QueryEscape(sub) + "&include=vless-reality"
	status, hdr, body := doGet(t, u, "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	// 模板 include(^vless-) 给 2 个节点，query include(vless-reality) 只给 1 个：
	// 若覆盖语义错了、变成追加，这里会看到 2。
	if got := hdr.Get("X-Node-Count"); got != "1" {
		t.Errorf("X-Node-Count = %q，应为 1（query 应整键覆盖模板 include）", got)
	}
	// 未被 query 覆盖的结构化字段仍然生效。
	if !strings.Contains(body, "[VLESS] vless-reality") {
		t.Errorf("模板的 node_pref 前缀应保留:\n%s", body)
	}
}

// TestConvert_RemoteConfigParam 请求 config= 指向的模板叠在本地模板之上。
func TestConvert_RemoteConfigParam(t *testing.T) {
	sub := fixtureURL(t)
	base := writeTpl(t, tplINI)
	// 叠加模板改 node_pref 的前缀与范围；未提及的段落（rules、代理组）沿用本地模板。
	overlay := writeTpl(t, "[node_pref]\nvless = <VLESS> \ninclude = ^vless\n")
	ts := newTestServer(t, func(c *Config) {
		c.Engine.BaseTemplate = func() *config.Template { return base }
		c.Engine.LoadTemplate = func(ctx context.Context, ref string) (*config.Template, error) {
			return config.LoadSource(ctx, ref, nil)
		}
	})
	u := ts.URL + "/sub?url=" + url.QueryEscape(sub) +
		"&config=" + url.QueryEscape(overlay.Origin())
	status, hdr, body := doGet(t, u, "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	// config= 的前缀与范围都覆盖本地模板，两个 vless 节点都加上。
	if !strings.Contains(body, "<VLESS> vless-reality") || !strings.Contains(body, "<VLESS> vless-ws") {
		t.Errorf("config 参数的 node_pref 应覆盖本地模板:\n%s", body)
	}
	if strings.Contains(body, "[VLESS] vless") {
		t.Errorf("本地模板的前缀应被覆盖")
	}
	// [common] include 与代理组来自本地模板，没有被空段落清掉。
	if got := hdr.Get("X-Node-Count"); got != "2" {
		t.Errorf("X-Node-Count = %q，本地模板的 [common] include 应保留", got)
	}
	if !strings.Contains(body, "name: 自动选择") || !strings.Contains(body, "MATCH,自动选择") {
		t.Errorf("本地模板的代理组与规则应保留")
	}
}

// TestConvert_RemoteConfigViaHTTP 远程模板走请求参数 config= 时经 loader 拉取。
func TestConvert_RemoteConfigViaHTTP(t *testing.T) {
	sub := fixtureURL(t)
	srv := newTestServer(t, func(c *Config) {
		c.Engine.LoadTemplate = func(ctx context.Context, ref string) (*config.Template, error) {
			return config.LoadSource(ctx, ref, config.HTTPLoader(&http.Client{Timeout: 5 * time.Second}, nil))
		}
	})
	cfg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[common]
type = socks5
`))
	}))
	t.Cleanup(cfg.Close)

	u := srv.URL + "/sub?url=" + url.QueryEscape(sub) + "&config=" + url.QueryEscape(cfg.URL+"/tpl.ini")
	status, hdr, body := doGet(t, u, "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if got := hdr.Get("X-Node-Count"); got != "1" {
		t.Errorf("X-Node-Count = %q，远程模板的 type 过滤应生效", got)
	}
	if strings.Contains(body, "vmess-node") {
		t.Errorf("远程模板的 type=socks5 未生效")
	}
}

// TestConvert_RemoteConfigNoLoader 请求指定 config 但引擎没有 loader 时要明确报错。
func TestConvert_RemoteConfigNoLoader(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	u := ts.URL + "/sub?url=" + url.QueryEscape(sub) + "&config=http%3A%2F%2Fexample.com%2Ftpl.ini"
	status, _, body := doGet(t, u, "", "")
	if status != http.StatusBadGateway {
		t.Fatalf("状态 = %d，应为 502，body=%s", status, body)
	}
	if !strings.Contains(body, "config") {
		t.Errorf("错误信息应指出 config 参数: %s", body)
	}
}

// TestConvert_PartialFailure 部分来源失败时仍返回可用配置，并在响应头标注。
func TestConvert_PartialFailure(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	bad := "http://127.0.0.1:1/sub" // 必然连不上
	status, hdr, _ := doGet(t,
		ts.URL+"/sub?url="+url.QueryEscape(sub)+"%3B"+url.QueryEscape(bad), "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，部分失败也应返回 200", status)
	}
	if hdr.Get("X-Convert-Warning") == "" {
		t.Error("应通过 X-Convert-Warning 标注失败来源")
	}
	if got := hdr.Get("X-Node-Count"); got != "12" {
		t.Errorf("X-Node-Count = %q", got)
	}
}

// --- 鉴权 ---

func TestAuth_RequiredAndAccepted(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, func(c *Config) { c.Token = func() string { return "s3cr3t" } })
	u := ts.URL + "/sub?url=" + url.QueryEscape(sub)

	status, hdr, _ := doGet(t, u, "", "")
	if status != http.StatusUnauthorized {
		t.Errorf("无令牌应为 401，实际 %d", status)
	}
	if hdr.Get("WWW-Authenticate") == "" {
		t.Error("401 响应应带 WWW-Authenticate")
	}
	if status, _, _ = doGet(t, u+"&token=wrong", "", ""); status != http.StatusUnauthorized {
		t.Errorf("错误令牌应为 401，实际 %d", status)
	}
	if status, _, _ = doGet(t, u, "", "s3cr3t"); status != http.StatusOK {
		t.Errorf("Authorization: Bearer 应为 200，实际 %d", status)
	}
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("X-Token", "s3cr3t")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("X-Token 应为 200，实际 %d", resp.StatusCode)
	}

	_, _, body := doGet(t, u, "", "")
	if strings.Contains(body, "proxies:") {
		t.Error("未带令牌的响应不应包含配置内容")
	}
}

// TestTokenNotLeaked 令牌不能混进转换参数，否则可能被当成上游订阅的认证信息。
func TestTokenNotLeaked(t *testing.T) {
	ts := newTestServer(t, func(c *Config) { c.Token = func() string { return "s3cr3t" } })
	status, _, body := doGet(t, ts.URL+"/inspect?token=s3cr3t&target=clash&nope=1", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d", status)
	}
	var got map[string]any
	mustJSON(t, body, &got)
	for _, k := range []string{"target", "unknown", "time"} {
		if _, ok := got[k]; !ok {
			t.Errorf("回显缺少字段 %q: %v", k, got)
		}
	}
	for _, item := range toSlice(got["unknown"]) {
		if item == "token" {
			t.Errorf("token 不应被当作普通参数: %v", got["unknown"])
		}
		if item == "nope" {
			return
		}
	}
	t.Errorf("未知参数 nope 未列入 unknown: %v", got["unknown"])
}

// --- 请求 ID 与响应元信息 ---

func TestRequestID_Echoed(t *testing.T) {
	ts := newTestServer(t, nil)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/healthz", nil)
	req.Header.Set("X-Request-ID", "rid-42")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Request-ID"); got != "rid-42" {
		t.Errorf("X-Request-ID = %q，应回显 rid-42", got)
	}
}

func TestRequestID_Generated(t *testing.T) {
	ts := newTestServer(t, nil)
	_, hdr, _ := doGet(t, ts.URL+"/healthz", "", "")
	if hdr.Get("X-Request-ID") == "" {
		t.Error("请求未带 X-Request-ID 时应自动生成")
	}
}

func TestResponseHeaders(t *testing.T) {
	sub := fixtureURL(t)
	ts := newTestServer(t, nil)
	_, hdr, _ := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(sub), "", "")
	// 配置里带着节点凭证，任何一层缓存都不该留底。
	if got := hdr.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if hdr.Get("Server") == "" {
		t.Error("缺少 Server 头")
	}
}

// --- 其余端点 ---

func TestVersionHealthIndexNotFound(t *testing.T) {
	ts := newTestServer(t, nil)

	// /version 是给 sub-web 的兼容契约：纯文本、以 "backend\n" 结尾。
	// 返回 JSON 会让 sub-web 的 formatVersion 抛异常并静默吞掉，版本号显示为空。
	status, hdr, body := doGet(t, ts.URL+"/version", "", "")
	if status != http.StatusOK {
		t.Fatalf("/version 状态 = %d", status)
	}
	if got := string(body); got != "subforge test-version backend\n" {
		t.Errorf("/version 内容 = %q，want %q", got, "subforge test-version backend\n")
	}
	if ct := hdr.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("/version Content-Type = %q，want text/plain", ct)
	}

	// 结构化版本信息在 /api/version。
	status, hdr, body = doGet(t, ts.URL+"/api/version", "", "")
	if status != http.StatusOK {
		t.Fatalf("/api/version 状态 = %d", status)
	}
	var v map[string]any
	mustJSON(t, body, &v)
	if v["version"] != "test-version" || v["name"] != "subforge" {
		t.Errorf("/api/version 内容: %v", v)
	}
	if !strings.HasPrefix(hdr.Get("Content-Type"), "application/json") {
		t.Errorf("/api/version Content-Type = %q", hdr.Get("Content-Type"))
	}

	status, _, body = doGet(t, ts.URL+"/healthz", "", "")
	if status != http.StatusOK {
		t.Fatalf("/healthz 状态 = %d", status)
	}
	var h map[string]any
	mustJSON(t, body, &h)
	if h["status"] != "ok" {
		t.Errorf("/healthz 内容: %v", h)
	}

	// 前端是 SPA：/ 返回 index.html 外壳。这里用假产物（真实 dist 不入库，
	// uiFakeDist 定义在 web_ui_test.go，同包可直接调用）。
	uiFakeDist(t)
	status, _, body = doGet(t, ts.URL+"/", "", "")
	if status != http.StatusOK || !strings.Contains(body, `<div id="app">`) {
		t.Errorf("/ 应返回 SPA 外壳（含 #app 挂载点），状态 %d", status)
	}

	// 注意：上一步已装上假 dist，所以「无扩展名的未知路径」现在会正常回落 index.html（200），
	// 这是 SPA 的预期行为。要测 404 必须用「带扩展名却找不到」的资源——
	// 这类资源绝不能回落 HTML，否则浏览器会把 HTML 当 JS 解析报 Unexpected token '<'。
	if status, _, _ = doGet(t, ts.URL+"/nope.js", "", ""); status != http.StatusNotFound {
		t.Errorf("未知静态资源应为 404，实际 %d", status)
	}
}

// rsRules 覆盖六种输出形态都会用到的规则取值。
const rsRules = "DOMAIN,ai.openai.com;DOMAIN-KEYWORD,hk;DOMAIN-REGEX,^\\.google\\.com$;" +
	"DOMAIN-SUFFIX,openai.com;IP-CIDR,9.9.9.9/32;GEOIP,CN,MATCH"

func TestGetRuleset_RejectsBadRequests(t *testing.T) {
	ts := newTestServer(t, nil)
	for _, path := range []string{
		"/getruleset",                                                             // 缺 name
		"/getruleset?name=ads",                                                    // 没有类型
		"/getruleset?name=ads&type=1",                                             // 有类型但没有任何规则来源
		"/getruleset?name=ads&type=99&rules=DOMAIN,a.com",                         // 未知形态
		"/getruleset?name=ads&type=1&rules=DOMAIN-REGEX,(",                        // 规则里带坏正则
		"/getruleset?name=ads&type=1&include=(",                                   // 坏 include 正则
		"/getruleset?name=ads&type=1&exclude=.&rules=" + url.QueryEscape(rsRules), // 过滤后为空
		// Clash 经典规则必须带出站：缺了就报 400，而不是产出内核拒载的配置。
		"/getruleset?name=ads&type=6&rules=DOMAIN,a.com",
	} {
		status, _, body := doGet(t, ts.URL+path, "", "")
		if status < 400 || status >= 500 {
			t.Errorf("%s: 状态 = %d，期望 4xx", path, status)
		}
		if !strings.Contains(body, "error") {
			t.Errorf("%s: 响应不像错误体: %s", path, body)
		}
	}
}

func TestGetRuleset_AllFormats(t *testing.T) {
	ts := newTestServer(t, nil)
	base := ts.URL + "/getruleset?name=ads&rules=" + url.QueryEscape(rsRules)

	cases := []struct {
		typeStr string
		extra   string
		ct      string
		want    string
		file    string
	}{
		{"1", "", "application/x-ruleset; charset=utf-8", "[General]\nCNAME = 1\n\n[Rule]\nDOMAIN,ai.openai.com\n", "ads.surge-rules"},
		{"2", "", "text/plain; charset=utf-8", "[Header]\nname = ads\ntype = remote\napp = Quantumult X\n", "ads.qxt"},
		{"3", "", "text/yaml; charset=utf-8", "payload:\n", "ads.yaml"},
		{"4", "", "text/yaml; charset=utf-8", "payload:\n", "ads.yaml"},
		{"5", "", "text/plain; charset=utf-8", "ai.openai.com\n", "ads.txt"},
		{"6", "&out=auto-select", "text/yaml; charset=utf-8", "rules:\n", "ads.yaml"},
	}
	for _, c := range cases {
		status, h, body := doGet(t, base+"&type="+c.typeStr+c.extra, "", "")
		if status != http.StatusOK {
			t.Fatalf("type=%s 状态 = %d: %s", c.typeStr, status, body)
		}
		if got := h.Get("Content-Type"); got != c.ct {
			t.Errorf("type=%s Content-Type = %q，期望 %q", c.typeStr, got, c.ct)
		}
		if got := h.Get("Content-Disposition"); !strings.Contains(got, "filename="+c.file) {
			t.Errorf("type=%s Content-Disposition = %q，期望含 filename=%s", c.typeStr, got, c.file)
		}
		if !strings.Contains(body, c.want) {
			t.Errorf("type=%s 正文缺少 %q:\n%s", c.typeStr, c.want, body)
		}
		if h.Get("X-Ruleset-Count") == "" || h.Get("X-Ruleset-Format") == "" {
			t.Errorf("type=%s 缺少统计响应头: %v", c.typeStr, h)
		}
	}
}

func TestGetRuleset_ProviderPayloads(t *testing.T) {
	ts := newTestServer(t, nil)
	base := ts.URL + "/getruleset?name=ads&rules=" + url.QueryEscape(rsRules)

	status, _, body := doGet(t, base+"&type=3", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	var domDoc map[string][]string
	if err := yaml.Unmarshal([]byte(body), &domDoc); err != nil {
		t.Fatalf("domain provider 不是合法 YAML: %v\n%s", err, body)
	}
	want := []string{"ai.openai.com", "hk", `^\.google\.com$`, "openai.com"}
	if len(domDoc["payload"]) != len(want) {
		t.Fatalf("domain provider payload = %v，期望 %v", domDoc["payload"], want)
	}
	for i := range want {
		if domDoc["payload"][i] != want[i] {
			t.Errorf("domain provider 第 %d 条 = %q，期望 %q", i, domDoc["payload"][i], want[i])
		}
	}

	status, _, body = doGet(t, base+"&type=4", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	var ipDoc map[string][]string
	if err := yaml.Unmarshal([]byte(body), &ipDoc); err != nil {
		t.Fatalf("ipcidr provider 不是合法 YAML: %v\n%s", err, body)
	}
	if len(ipDoc["payload"]) != 1 || ipDoc["payload"][0] != "9.9.9.9/32" {
		t.Errorf("ipcidr provider payload = %v", ipDoc["payload"])
	}
}

// TestGetRuleset_TemplateTypeAndRules 模板 [ruleset] 给类型、[common] rules 给内容，
// 请求只带 name 就该出结果。
func TestGetRuleset_TemplateTypeAndRules(t *testing.T) {
	ts := newTestServer(t, func(c *Config) {
		tpl := writeTpl(t, strings.Join([]string{
			"[common]",
			rsRulesParam,
			"",
			"[ruleset]",
			"ads = 5",
			"geo = 4",
		}, "\n"))
		c.Engine.BaseTemplate = func() *config.Template { return tpl }
	})

	status, h, body := doGet(t, ts.URL+"/getruleset?name=ads", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	if body != "ai.openai.com\nopenai.com\n" {
		t.Errorf("DOMAIN-SET 正文不符:\n%s", body)
	}
	if got := h.Get("X-Ruleset-Skipped"); got != "4" {
		t.Errorf("X-Ruleset-Skipped = %q，期望 4（关键词 + 域名正则 + IP-CIDR + GEOIP 都表达不了）", got)
	}
	if got := h.Get("X-Ruleset-Format"); got != "Surge DOMAIN-SET" {
		t.Errorf("X-Ruleset-Format = %q", got)
	}

	status, _, body = doGet(t, ts.URL+"/getruleset?name=geo", "", "")
	if status != http.StatusOK {
		t.Fatalf("ipcidr 状态 = %d: %s", status, body)
	}
	if !strings.Contains(body, "9.9.9.9/32") {
		t.Errorf("ipcidr 正文不符:\n%s", body)
	}
}

// TestGetRuleset_QueryOverridesTemplate URL 参数的 type 与 rules 压过模板取值。
func TestGetRuleset_QueryOverridesTemplate(t *testing.T) {
	ts := newTestServer(t, func(c *Config) {
		tpl := writeTpl(t, strings.Join([]string{
			"[common]",
			rsRulesParam,
			"",
			"[ruleset]",
			"ads = 5",
		}, "\n"))
		c.Engine.BaseTemplate = func() *config.Template { return tpl }
	})

	status, _, body := doGet(t, ts.URL+"/getruleset?name=ads&type=6&rules=GEOIP,CN,MATCH", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	if !strings.Contains(body, "GEOIP,CN,MATCH") || strings.Contains(body, "openai") {
		t.Errorf("URL 参数没有压过模板:\n%s", body)
	}

	// include 只在匹配到的规则上生效，其余丢弃。
	status, _, body = doGet(t, ts.URL+"/getruleset?name=ads&include=openai", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	if body != "ai.openai.com\nopenai.com\n" {
		t.Errorf("include 过滤后正文不符:\n%s", body)
	}
}

func TestGetRuleset_RemoteURL(t *testing.T) {
	rules := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("DOMAIN,remote.example.com\nDOMAIN-SUFFIX,example.com\n"))
	}))
	t.Cleanup(rules.Close)

	ts := newTestServer(t, nil)
	status, _, body := doGet(t, ts.URL+"/getruleset?name=remote&type=5&url="+url.QueryEscape(rules.URL+"/rules.txt"), "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	if body != "remote.example.com\nexample.com\n" {
		t.Errorf("正文不符:\n%s", body)
	}
}

func TestGetRuleset_RemoteURLFailure(t *testing.T) {
	ts := newTestServer(t, nil)
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(dead.Close)
	status, _, body := doGet(t, ts.URL+"/getruleset?name=remote&type=1&url="+dead.URL, "", "")
	if status != http.StatusBadGateway {
		t.Fatalf("状态 = %d，期望 502: %s", status, body)
	}
}

// TestGetRuleset_ClassicalDefaultOutbound 经典形态的规则缺出站时，
// 用 out 参数补一个默认出站，已经写了出站的规则保持原样。
func TestGetRuleset_ClassicalDefaultOutbound(t *testing.T) {
	ts := newTestServer(t, nil)
	status, _, body := doGet(t, ts.URL+"/getruleset?name=ads&type=6&rules="+
		url.QueryEscape("DOMAIN,example.com;GEOIP,CN,DIRECT")+"&out=auto-select", "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d: %s", status, body)
	}
	for _, want := range []string{"DOMAIN,example.com,auto-select", "GEOIP,CN,DIRECT"} {
		if !strings.Contains(body, want) {
			t.Errorf("缺少 %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "auto-select,DIRECT") {
		t.Errorf("已写出站的规则被改写了:\n%s", body)
	}
}

// rsRulesParam 是模板 [common] 段落里的规则行写法。
const rsRulesParam = "rules = " + rsRules

func TestInspect(t *testing.T) {
	ts := newTestServer(t, nil)
	u := ts.URL + "/inspect?target=Clash.Meta&include=HK%3BJP&type=vless&rename=HK%2C🇭🇰&sort=NAME&nope=1"
	status, _, body := doGet(t, u, "", "")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d", status)
	}
	var got map[string]any
	mustJSON(t, body, &got)
	if got["target"] != "clash.meta" {
		t.Errorf("target = %v（应归一为小写）", got["target"])
	}
	if inc := toSlice(got["include"]); len(inc) != 2 || inc[0] != "HK" {
		t.Errorf("include = %v", got["include"])
	}
	if got["sort"] != "name" {
		t.Errorf("sort = %v", got["sort"])
	}
	unknown := toSlice(got["unknown"])
	if !contains(unknown, "nope") {
		t.Errorf("未知参数应列在 unknown 里: %v", unknown)
	}
	if contains(unknown, "target") {
		t.Errorf("已识别的参数不应算未知: %v", unknown)
	}

	if status, _, _ = doGet(t, ts.URL+"/inspect?include=%28", "", ""); status != http.StatusBadRequest {
		t.Errorf("坏正则应为 400，实际 %d", status)
	}
}

// --- UA 推断 ---

func TestGuessTarget(t *testing.T) {
	cases := map[string]string{
		"":                 "clash.meta",
		"curl/8.4.0":       "clash.meta",
		"Clash Verge 2.1":  "clash.meta",
		"ClashX Meta 1.5":  "clash.meta",
		"mihomo 1.19.31":   "clash.meta",
		"Surge 6.2":        "surge",
		"Loon 2.2":         "loon",
		"Stash 2.9":        "stash",
		"Quantumult X 2.2": "quantumult",
		"Shadowrocket 2.2": "quantumult",
		"sing-box 1.11":    "sing-box",
		"v2rayN 6.48":      "v2ray",
		"hiddify 2.0":      "v2ray",
		"Mihomo Party 1.6": "mihomo-party",
	}
	for ua, want := range cases {
		if got := guessTarget(ua); got != want {
			t.Errorf("guessTarget(%q) = %q，应为 %q", ua, got, want)
		}
	}
}

// --- 超时 ---

// TestTimeout_ReturnsQuickly 慢上游不能拖住连接：请求要在超时时间附近返回错误。
func TestTimeout_ReturnsQuickly(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	t.Cleanup(slow.Close)

	ts := newTestServer(t, func(c *Config) { c.Timeout = 300 * time.Millisecond })
	start := time.Now()
	status, _, _ := doGet(t, ts.URL+"/sub?url="+url.QueryEscape(slow.URL+"/sub"), "", "")
	elapsed := time.Since(start)
	if status == http.StatusOK {
		t.Fatalf("慢上游不应返回 200")
	}
	if elapsed > 2*time.Second {
		t.Errorf("超时未生效，耗时 %v", elapsed)
	}
}

// TestSubscribeTimeout_DeadSourceDoesNotBlock 订阅端点必须在客户端容忍度内返回：
// 死源在拉取预算内被截断，健康来源照常进产物。线上 OpenClash 的 curl 在 30 秒
// 零字节后中断并无限重试，如果死源吃满请求总超时，订阅就永远下不下来。
func TestSubscribeTimeout_DeadSourceDoesNotBlock(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(dead.Close)

	ts, mgr := uiSubServerCfg(t, func(c *Config) {
		c.Timeout = 5 * time.Second
		c.SubscribeTimeout = 300 * time.Millisecond
	})
	ctx := context.Background()
	// 死源先入库：sort_order 按入库时间递增，它排在健康来源前面，复现线上顺序。
	if _, err := mgr.AddAirport(ctx, "dead", dead.URL+"/sub", "", true); err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "ok", fixtureURL(t), "", true); err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-budget", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	start := time.Now()
	status, _, body := uiGet(t, ts, "/subscribe?token=tok-budget&target=clash.meta")
	elapsed := time.Since(start)
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if elapsed > 2*time.Second {
		t.Errorf("订阅被死源拖住，耗时 %v", elapsed)
	}
	if !strings.Contains(body, "proxies:") {
		t.Error("健康来源的产物缺失")
	}
}

// TestSubscribeFailurePolicy 机场失败策略：error 时整份订阅报错（客户端保留旧配置），
// ignore 时跳过失败来源继续产出。
func TestSubscribeFailurePolicy(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(bad.Close)

	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	critical, err := mgr.AddAirport(ctx, "关键机场", bad.URL+"/sub", "", true)
	if err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if err := mgr.SetAirportFailurePolicy(ctx, critical.ID, subforge.AirportFailureError); err != nil {
		t.Fatalf("设置失败策略: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "正常机场", fixtureURL(t), "", true); err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-policy", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	status, _, body := uiGet(t, ts, "/subscribe?token=tok-policy&target=clash.meta")
	if status != http.StatusBadGateway {
		t.Fatalf("error 策略下状态 = %d，body=%s", status, body)
	}
	if !strings.Contains(body, "关键机场") {
		t.Errorf("错误信息应点名失败机场: %s", body)
	}

	if err := mgr.SetAirportFailurePolicy(ctx, critical.ID, subforge.AirportFailureIgnore); err != nil {
		t.Fatalf("切回 ignore: %v", err)
	}
	status, _, body = uiGet(t, ts, "/subscribe?token=tok-policy&target=clash.meta")
	if status != http.StatusOK {
		t.Fatalf("ignore 策略下状态 = %d，body=%s", status, body)
	}
	if !strings.Contains(body, "proxies:") {
		t.Error("ignore 策略应照常产出健康来源的配置")
	}
}

// TestAirportFailurePolicyAPI 机场 CRUD 的 failure_policy：创建可带、更新省略时保持原值、
// 非法值 400。
func TestAirportFailurePolicyAPI(t *testing.T) {
	ts, mgr := uiSubServer(t)
	sub := fixtureURL(t)
	node, err := mgr.AddSelfNode(context.Background(), "socks5://user:pass@127.0.0.1:1080", "节点A", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}

	st, _, body := uiPostJSON(t, ts, "/api/airports", fmt.Sprintf(
		`{"name":"A","subscription_url":%q,"remarks":"","enabled":true,"failure_policy":"error","outbound_node":%q}`,
		sub, fmt.Sprint(node.ID)))
	if st != http.StatusCreated {
		t.Fatalf("创建状态 = %d，body=%s", st, body)
	}
	var created struct {
		Airport struct {
			ID            int64  `json:"id"`
			FailurePolicy string `json:"failure_policy"`
			OutboundNode  string `json:"outbound_node"`
		} `json:"airport"`
	}
	mustJSON(t, body, &created)
	if created.Airport.FailurePolicy != "error" {
		t.Fatalf("创建后 failure_policy = %q", created.Airport.FailurePolicy)
	}
	if created.Airport.OutboundNode != fmt.Sprint(node.ID) {
		t.Fatalf("创建后 outbound_node = %q", created.Airport.OutboundNode)
	}
	path := fmt.Sprintf("/api/airports/%d", created.Airport.ID)

	// 省略 failure_policy 的更新不能把策略重置回 ignore。
	st, _, body = uiDoJSON(t, ts, "PUT", path, fmt.Sprintf(
		`{"name":"A2","subscription_url":%q,"remarks":"","enabled":true}`, sub))
	if st != http.StatusOK {
		t.Fatalf("更新状态 = %d，body=%s", st, body)
	}
	_, _, body = uiGet(t, ts, path)
	if !strings.Contains(body, `"failure_policy":"error"`) {
		t.Fatalf("省略字段后策略被重置: %s", body)
	}

	st, _, _ = uiDoJSON(t, ts, "PUT", path, fmt.Sprintf(
		`{"name":"A2","subscription_url":%q,"remarks":"","enabled":true,"failure_policy":"boom"}`, sub))
	if st != http.StatusBadRequest {
		t.Fatalf("非法策略应 400，得到 %d", st)
	}

	st, _, _ = uiDoJSON(t, ts, "PUT", path, fmt.Sprintf(
		`{"name":"A2","subscription_url":%q,"remarks":"","enabled":true,"outbound_node":"direct"}`, sub))
	if st != http.StatusOK {
		t.Fatalf("设置直连状态 = %d", st)
	}
	_, _, body = uiGet(t, ts, path)
	if !strings.Contains(body, `"outbound_node":"direct"`) {
		t.Fatalf("出网节点未更新: %s", body)
	}

	st, _, _ = uiDoJSON(t, ts, "PUT", path, fmt.Sprintf(
		`{"name":"A2","subscription_url":%q,"remarks":"","enabled":true,"outbound_node":"9999"}`, sub))
	if st != http.StatusBadRequest {
		t.Fatalf("不存在的节点应 400，得到 %d", st)
	}
	st, _, _ = uiPostJSON(t, ts, "/api/airports", fmt.Sprintf(
		`{"name":"B","subscription_url":%q,"remarks":"","enabled":true,"outbound_node":"9999"}`, sub))
	if st != http.StatusBadRequest {
		t.Fatalf("创建时非法节点应 400，得到 %d", st)
	}
}

// TestSubscribeUsesSourceCache 间隔内 /subscribe 直接用缓存，不再请求上游；间隔 0 时恢复实时拉取。
func TestSubscribeUsesSourceCache(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}
	var hits int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write(buf)
	}))
	t.Cleanup(up.Close)

	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if err := mgr.SetSettings(ctx, map[string]string{subforge.SourceRefreshKey: "30"}); err != nil {
		t.Fatalf("设置刷新间隔: %v", err)
	}
	a, err := mgr.AddAirport(ctx, "缓存机场", up.URL+"/sub.txt", "", true)
	if err != nil {
		t.Fatalf("插入机场: %v", err)
	}
	if err := mgr.PutSourceCache(ctx, a.SubscriptionURL, buf); err != nil {
		t.Fatalf("写缓存: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-cache", "测试", "", "", "", "", ""); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	status, _, body := uiGet(t, ts, "/subscribe?token=tok-cache&target=clash.meta")
	if status != http.StatusOK {
		t.Fatalf("状态 = %d，body=%s", status, body)
	}
	if !strings.Contains(body, "proxies:") {
		t.Error("缓存内容未产出配置")
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("间隔内不应拉取上游，hits=%d", got)
	}

	if err := mgr.SetSettings(ctx, map[string]string{subforge.SourceRefreshKey: "0"}); err != nil {
		t.Fatalf("关闭缓存: %v", err)
	}
	status, _, _ = uiGet(t, ts, "/subscribe?token=tok-cache&target=clash.meta")
	if status != http.StatusOK {
		t.Fatalf("关闭缓存后状态 = %d", status)
	}
	if atomic.LoadInt32(&hits) == 0 {
		t.Fatal("关闭缓存后应重新拉取上游")
	}
}

// TestRuleSourceOutboundAPI 规则源的新增/设置出网节点接口。
func TestRuleSourceOutboundAPI(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if err := mgr.SetRuleManifestPath(ctx, filepath.Join(t.TempDir(), "manifest.yaml")); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	n, err := mgr.AddSelfNode(ctx, "socks5://user:pass@127.0.0.1:1080", "节点A", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}

	st, _, body := uiPostJSON(t, ts, "/api/rule-sources", fmt.Sprintf(
		`{"name":"src","url":"https://example.com/rules.yaml","outbound_node":%q}`, fmt.Sprint(n.ID)))
	if st != http.StatusOK {
		t.Fatalf("新增规则源状态 = %d，body=%s", st, body)
	}
	_, _, listBody := uiGet(t, ts, "/api/rule-sources")
	if !strings.Contains(listBody, `"outbound_node":"`+fmt.Sprint(n.ID)+`"`) {
		t.Fatalf("列表未带出网节点: %s", listBody)
	}

	st, _, body = uiPostJSON(t, ts, "/api/rule-sources/outbound", `{"name":"src","outbound_node":"direct"}`)
	if st != http.StatusOK {
		t.Fatalf("设置出网状态 = %d，body=%s", st, body)
	}
	_, _, listBody = uiGet(t, ts, "/api/rule-sources")
	if !strings.Contains(listBody, `"outbound_node":"direct"`) {
		t.Fatalf("出网节点未更新: %s", listBody)
	}

	st, _, _ = uiPostJSON(t, ts, "/api/rule-sources/outbound", `{"name":"src","outbound_node":"9999"}`)
	if st != http.StatusBadRequest {
		t.Fatalf("不存在的节点应 400，得到 %d", st)
	}
}

// --- 小工具 ---

func mustJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("JSON 解析失败 %q: %v", body, err)
	}
}

func toSlice(v any) []string {
	out := []string{}
	for _, item := range v.([]any) {
		out = append(out, item.(string))
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
