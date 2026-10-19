package api

// token_fields_test.go — 订阅令牌五字段定制（target/pipeline/config/include/exclude）的 HTTP 层契约测试。
//
// 覆盖：
//  1. POST /api/tokens 接受五字段并持久化；GET /api/tokens/{token} 能读回（新注册的 GET 路由）；
//  2. PUT /api/tokens/{token} 部分更新语义：省略字段不动、空串清空、{} 幂等、不存在 404；
//  3. /subscribe 取值优先级：query 显式参数 > 令牌字段 > 原回落链（target→clash.meta、pipeline→config 表）；
//  4. 空字段回归：五字段全空的令牌与改动前逐字节一致。
//
// 全部走 httptest + 本地 fixture（fixtureURL），不碰外网。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subforge"
)

// apiSubToken 是 /api/tokens 返回的 token 对象里我们关心的字段。
type apiSubToken struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Target   string `json:"target"`
	Pipeline string `json:"pipeline"`
	Config   string `json:"config"`
	Include  string `json:"include"`
	Exclude  string `json:"exclude"`
}

// decodeAPIToken 从 {"token": {...}} 里解出 token 对象。
func decodeAPIToken(t *testing.T, what, body string) apiSubToken {
	t.Helper()
	var resp struct {
		Token apiSubToken `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("%s 响应不是预期 JSON: %v; body=%s", what, err, body)
	}
	return resp.Token
}

// firstDiffSnippet 定位两段文本的首个差异位置并给出上下文，便于断言失败时直接看出差异。
func firstDiffSnippet(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	lo := i - 40
	if lo < 0 {
		lo = 0
	}
	hi := i + 40
	if hi > n {
		hi = n
	}
	return fmt.Sprintf("首个差异 @%d: A[%d:%d]=%q B[%d:%d]=%q (lenA=%d lenB=%d)", i, lo, hi, a[lo:hi], lo, hi, b[lo:hi], len(a), len(b))
}

// randomUARe 匹配 mihomo 转换层随机注入的浏览器 User-Agent。
// 来源：vendor/github.com/metacubex/mihomo/common/convert/util.go 的 UA 列表——
// 节点没写死 UA 时每次转换都会随机挑一个（ws 类节点尤其明显），
// 因此订阅正文天然不是逐字节稳定的：比较前必须先归一化，否则断言会随机失败。
// 同时兼容 list 分享链接里 URL 编码的形式（Mozilla%2F5.0%20...&）。
var randomUARe = regexp.MustCompile("Mozilla(?:/|%2F)5\\.0[^&\n\"']*")

// generatedAtRe 匹配注释头里的秒级生成时间（RFC3339）。两次渲染跨秒时该字面
// 不同，逐字节比较会随机失败（CI linux 慢，比本地更易跨秒）——比较前折叠。
var generatedAtRe = regexp.MustCompile("生成时间: \\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:Z|[+-]\\d{2}:\\d{2})")

// normalizeRandomUA 把随机 UA 折叠成固定占位符，供订阅正文比较使用。
func normalizeRandomUA(s string) string {
	return generatedAtRe.ReplaceAllString(randomUARe.ReplaceAllString(s, "UA"), "生成时间: TS")
}

// tokenTestEnv 造一个「一个启用机场 + 若干令牌」的订阅服务。
func tokenTestEnv(t *testing.T) (*httptest.Server, *subforge.Manager) {
	t.Helper()
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	return ts, mgr
}

// mustCreateToken 直插一个令牌（绕过 HTTP，专注被测语义）。
func mustCreateToken(t *testing.T, mgr *subforge.Manager, token, target, pipeline, configName, include, exclude string) {
	t.Helper()
	if err := mgr.CreateSubToken(context.Background(), token, "测试", target, pipeline, configName, include, exclude); err != nil {
		t.Fatalf("插入 token %s: %v", token, err)
	}
}

// --- 1. /api/tokens 的五字段读写 ---

func TestAPITokensCreateWithFields(t *testing.T) {
	ts, _ := uiSubServer(t)
	body := `{"token":"tok-http","name":"n1","target":"list","pipeline":"p1","config":"c1","include":"vless","exclude":"vmess"}`
	status, _, respBody := uiPostJSON(t, ts, "/api/tokens", body)
	if status != http.StatusCreated {
		t.Fatalf("POST /api/tokens status=%d body=%s，期望 201", status, respBody)
	}
	got := decodeAPIToken(t, "POST /api/tokens", respBody)
	if got.Target != "list" || got.Pipeline != "p1" || got.Config != "c1" || got.Include != "vless" || got.Exclude != "vmess" {
		t.Fatalf("POST 回显五字段 = %+v，与请求不符", got)
	}

	// GET 单项（本次新注册的路由）应读回同样的值。
	status, _, respBody = uiGet(t, ts, "/api/tokens/tok-http")
	if status != http.StatusOK {
		t.Fatalf("GET /api/tokens/tok-http status=%d body=%s", status, respBody)
	}
	got = decodeAPIToken(t, "GET /api/tokens/{token}", respBody)
	if got.Target != "list" || got.Pipeline != "p1" || got.Config != "c1" || got.Include != "vless" || got.Exclude != "vmess" {
		t.Fatalf("GET 读回五字段 = %+v，与写入不符", got)
	}

	// 列表里也应带五字段（回归：ListSubTokens 的 Scan 列数）。
	status, _, respBody = uiGet(t, ts, "/api/tokens")
	if status != http.StatusOK {
		t.Fatalf("GET /api/tokens status=%d body=%s", status, respBody)
	}
	var list struct {
		Tokens []apiSubToken `json:"tokens"`
	}
	if err := json.Unmarshal([]byte(respBody), &list); err != nil {
		t.Fatalf("列表响应不是预期 JSON: %v; body=%s", err, respBody)
	}
	if len(list.Tokens) != 1 || list.Tokens[0].Include != "vless" || list.Tokens[0].Target != "list" {
		t.Fatalf("列表 token = %+v，期望带五字段", list.Tokens)
	}
}

func TestAPITokenPutPartialAndClear(t *testing.T) {
	ts, mgr := uiSubServer(t)
	mustCreateToken(t, mgr, "tok-put", "list", "p1", "c1", "vless", "vmess")

	// (a) 只给 target：其余五列保持原值。
	status, _, body := uiDoJSON(t, ts, "PUT", "/api/tokens/tok-put", `{"target":"clash.meta"}`)
	if status != http.StatusOK {
		t.Fatalf("PUT target status=%d body=%s", status, body)
	}
	got := decodeAPIToken(t, "PUT target", body)
	if got.Target != "clash.meta" {
		t.Fatalf("PUT 后 target=%q，期望 clash.meta", got.Target)
	}
	if got.Name != "测试" || got.Pipeline != "p1" || got.Config != "c1" || got.Include != "vless" || got.Exclude != "vmess" {
		t.Fatalf("部分更新误伤其他列: %+v", got)
	}

	// (b) 显式空串 = 清空 include，且不动 target。
	status, _, body = uiDoJSON(t, ts, "PUT", "/api/tokens/tok-put", `{"include":""}`)
	if status != http.StatusOK {
		t.Fatalf("PUT include=\"\" status=%d body=%s", status, body)
	}
	got = decodeAPIToken(t, "PUT include=''", body)
	if got.Include != "" {
		t.Fatalf("PUT include=\"\" 后 include=%q，期望清空", got.Include)
	}
	if got.Target != "clash.meta" {
		t.Fatalf("清空 include 误伤 target=%q", got.Target)
	}

	// (c) 空对象 = 幂等 no-op。
	status, _, body = uiDoJSON(t, ts, "PUT", "/api/tokens/tok-put", `{}`)
	if status != http.StatusOK {
		t.Fatalf("PUT {} status=%d body=%s，期望 200", status, body)
	}
	got = decodeAPIToken(t, "PUT {}", body)
	if got.Target != "clash.meta" || got.Pipeline != "p1" || got.Config != "c1" || got.Exclude != "vmess" {
		t.Fatalf("PUT {} 改变了行: %+v", got)
	}

	// (d) 不存在的令牌：带字段与空对象都 404，且错误体是统一 JSON 形状。
	status, hdr, body := uiDoJSON(t, ts, "PUT", "/api/tokens/nope", `{"target":"list"}`)
	if status != http.StatusNotFound {
		t.Fatalf("PUT 不存在 token status=%d body=%s，期望 404", status, body)
	}
	uiAssertJSONError(t, "PUT 不存在 token", body, hdr)
	if !strings.Contains(body, "不存在") {
		t.Errorf("404 错误体未说明原因: %s", body)
	}
	if status, _, _ = uiDoJSON(t, ts, "PUT", "/api/tokens/nope", `{}`); status != http.StatusNotFound {
		t.Fatalf("PUT {} 不存在 token status=%d，期望 404", status)
	}

	// (e) 未注册方法仍是 405。注意：chi 对已注册路由的未支持方法走它自带的
	// MethodNotAllowedHandler（空 body），handler 内的 default 分支通常不可达，
	// 因此这里只锁状态码，不锁文案。
	status, _, body = uiDoJSON(t, ts, "PATCH", "/api/tokens/tok-put", `{}`)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH status=%d body=%s，期望 405", status, body)
	}
	if body != "" && !strings.Contains(body, "GET/PUT/DELETE") {
		t.Errorf("405 文案异常: %s", body)
	}
}

// --- 2. /subscribe 令牌字段生效 ---

func TestSubscribeTokenFieldsEffective(t *testing.T) {
	base, mgr := tokenTestEnv(t)
	mustCreateToken(t, mgr, "tok-fields", "list", "", "", "vless", "")
	mustCreateToken(t, mgr, "tok-empty", "", "", "", "", "")

	// (1) 令牌 target=list 生效：输出不再是 Clash YAML，而是分享链接。
	status, _, body := uiGet(t, base, "/subscribe?token=tok-fields")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Contains(body, "proxies:") {
		t.Fatalf("令牌 target=list 未生效，输出仍是 Clash YAML")
	}
	if !strings.Contains(body, "://") {
		t.Fatalf("令牌 target=list 未生效，输出不含分享链接")
	}

	// (2) 令牌 target+include 与显式 query 逐字节等价。
	status2, _, body2 := uiGet(t, base, "/subscribe?token=tok-empty&target=list&include=vless")
	if status2 != http.StatusOK {
		t.Fatalf("对照请求 status=%d body=%s", status2, body2)
	}
	if normalizeRandomUA(body) != normalizeRandomUA(body2) {
		t.Fatalf("令牌字段与显式 query 不等价: %s",
			firstDiffSnippet(normalizeRandomUA(body), normalizeRandomUA(body2)))
	}

	// (3) 令牌 include 确实在过滤：与不过滤的 list 输出不同。
	_, _, all := uiGet(t, base, "/subscribe?token=tok-empty&target=list")
	if normalizeRandomUA(all) == normalizeRandomUA(body) {
		t.Fatalf("令牌 include 未生效：过滤前后输出相同（%d 字节）", len(all))
	}

	// (4) 五字段全空的令牌保持现状：默认 target=clash.meta。
	statusE, _, bodyE := uiGet(t, base, "/subscribe?token=tok-empty")
	if statusE != http.StatusOK || !strings.Contains(bodyE, "proxies:") {
		t.Fatalf("空字段令牌未回落默认 target: status=%d body=%s", statusE, bodyE)
	}
}

// --- 3. query 显式参数覆盖令牌字段 ---

func TestSubscribeTokenQueryOverrides(t *testing.T) {
	base, mgr := tokenTestEnv(t)
	mustCreateToken(t, mgr, "tok-ov", "list", "", "", "vless", "")
	mustCreateToken(t, mgr, "tok-ex", "", "", "", "", "vmess")
	mustCreateToken(t, mgr, "tok-empty", "", "", "", "", "")

	// (a) query target 覆盖令牌 target=list。
	status, _, body := uiGet(t, base, "/subscribe?token=tok-ov&target=clash.meta&include=vless")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if !strings.Contains(body, "proxies:") {
		t.Fatalf("query target=clash.meta 未覆盖令牌 target=list")
	}

	// (b) query include 覆盖令牌 include=vless：结果应与显式 vmess 基线一致，且与 vless 基线不同。
	// 注意：tok-ov 的令牌 target=list 会改变输出格式，因此三个对照都显式钉死 target=clash.meta。
	_, _, ov := uiGet(t, base, "/subscribe?token=tok-ov&target=clash.meta&include=vmess")
	_, _, baseVM := uiGet(t, base, "/subscribe?token=tok-empty&target=clash.meta&include=vmess")
	_, _, baseVL := uiGet(t, base, "/subscribe?token=tok-empty&target=clash.meta&include=vless")
	if normalizeRandomUA(ov) != normalizeRandomUA(baseVM) {
		t.Fatalf("query include 未覆盖令牌 include: %s",
			firstDiffSnippet(normalizeRandomUA(ov), normalizeRandomUA(baseVM)))
	}
	if normalizeRandomUA(ov) == normalizeRandomUA(baseVL) {
		t.Fatalf("include 过滤无区分度：vmess 与 vless 输出相同")
	}

	// (c) 令牌 exclude 生效。
	_, _, exTok := uiGet(t, base, "/subscribe?token=tok-ex")
	if strings.Contains(exTok, "vmess-node") {
		t.Fatalf("令牌 exclude=vmess 未生效，vmess-node 仍在输出")
	}
	// (d) query exclude 覆盖令牌 exclude=vmess。
	_, _, exQuery := uiGet(t, base, "/subscribe?token=tok-ex&exclude=vless")
	if !strings.Contains(exQuery, "vmess-node") {
		t.Fatalf("query exclude=vless 未覆盖令牌 exclude=vmess")
	}
}

// --- 4. 令牌 pipeline / config 生效（与显式 query 行为一致） ---

func TestSubscribeTokenPipelineAndConfigEffective(t *testing.T) {
	base, mgr := tokenTestEnv(t)
	mustCreateToken(t, mgr, "tok-pipe", "", "nope", "", "", "")
	mustCreateToken(t, mgr, "tok-cfg", "", "", "__no_such_template__", "", "")

	// pipeline：令牌字段应等价于显式 query=pipeline。
	statusTok, _, _ := uiGet(t, base, "/subscribe?token=tok-pipe")
	statusQuery, _, _ := uiGet(t, base, "/subscribe?token=tok-pipe&pipeline=nope")
	if statusQuery == http.StatusOK {
		t.Fatalf("前置假设失败：显式 pipeline=nope 返回 200，用例失去区分度")
	}
	if statusTok != statusQuery {
		t.Fatalf("令牌 pipeline 与显式 query 响应码不一致: %d vs %d", statusTok, statusQuery)
	}

	// config：测试引擎没有模板加载器，非空 config 必然报错 → 可用来证明令牌 config 确实传到了引擎。
	statusTokCfg, _, _ := uiGet(t, base, "/subscribe?token=tok-cfg")
	statusQueryCfg, _, _ := uiGet(t, base, "/subscribe?token=tok-cfg&config=__no_such_template__")
	if statusQueryCfg == http.StatusOK {
		t.Fatalf("前置假设失败：显式 config=__no_such_template__ 返回 200，用例失去区分度")
	}
	if statusTokCfg != statusQueryCfg {
		t.Fatalf("令牌 config 与显式 query 响应码不一致: %d vs %d", statusTokCfg, statusQueryCfg)
	}
}

// --- 5. 空字段回归：令牌不定制时与改动前逐字节一致 ---

func TestSubscribeTokenEmptyFieldsRegression(t *testing.T) {
	base, mgr := tokenTestEnv(t)
	mustCreateToken(t, mgr, "tok-reg", "", "", "", "", "")

	// (1) 无 query 与显式 target=clash.meta 逐字节一致。
	statusA, _, bodyA := uiGet(t, base, "/subscribe?token=tok-reg")
	statusB, _, bodyB := uiGet(t, base, "/subscribe?token=tok-reg&target=clash.meta")
	if statusA != http.StatusOK || statusB != http.StatusOK {
		t.Fatalf("statusA=%d statusB=%d", statusA, statusB)
	}
	// 整份正文比较前先归一化随机 UA（见 randomUARe 注释）。
	if normalizeRandomUA(bodyA) != normalizeRandomUA(bodyB) {
		t.Fatalf("空字段令牌改变了默认行为: %s",
			firstDiffSnippet(normalizeRandomUA(bodyA), normalizeRandomUA(bodyB)))
	}
	if !strings.Contains(bodyA, "proxies:") {
		t.Fatalf("空字段令牌未产出默认 Clash YAML")
	}

	// (1b) 确定性字节回归：include=tuic-node 只渲染无随机 UA 的 tuic 节点，
	// 此时「无 target」与「显式 target=clash.meta」必须逐字节相同（无需归一化）。
	_, _, ssA := uiGet(t, base, "/subscribe?token=tok-reg&include=tuic-node")
	_, _, ssB := uiGet(t, base, "/subscribe?token=tok-reg&target=clash.meta&include=tuic-node")
	if !strings.Contains(ssA, "tuic-node") {
		t.Fatalf("include=tuic-node 未产出目标节点，用例失效: %s", ssA)
	}
	if ssA != ssB {
		t.Fatalf("确定性字节回归失败: %s", firstDiffSnippet(ssA, ssB))
	}

	// (2) pipeline 的 config 表回落链仍然生效（令牌 pipeline 为空时）。
	if _, err := mgr.DB().Raw().ExecContext(context.Background(),
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		"subscribe.pipeline", "nope"); err != nil {
		t.Fatalf("写 config: %v", err)
	}
	statusFb, _, _ := uiGet(t, base, "/subscribe?token=tok-reg")
	statusEx, _, _ := uiGet(t, base, "/subscribe?token=tok-reg&pipeline=nope")
	if statusFb != statusEx {
		t.Fatalf("空字段令牌破坏了 config 表 pipeline 回落: %d vs %d", statusFb, statusEx)
	}
	if statusEx == http.StatusOK {
		t.Logf("提示：显式 pipeline=nope 返回 200，回落链用例区分度有限")
	}
}
