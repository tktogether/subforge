// 订阅方案（Profile）与「规则/分组全局库」HTTP 层测试（v2 契约）。
//
// 覆盖：
//   - /api/profiles 的 CRUD、层数组校验（template 自动末位）、删除前令牌占用 409；
//   - /api/rules 与 /api/groups 的 ?source= / ?source_prefix= 过滤、按来源批量启停与删除；
//   - 单条规则/分组改来源（含跨来源重复 origin_key 的 409）与 profile_id 参数下线；
//   - 机场导入进全局库（source=airport:<名>，可选自动建组）；
//   - /subscribe 按方案的 rule_layers / group_layers 拼接，模板分组与 DB 分组合并（D-36）；
//   - /api/convert 的 rule_layers / group_layers 参数。
//
// 这些分支是 CI 函数覆盖率门禁（-min-func 100）不红的前提。
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/subforge"
)

// --- 响应结构 ---

type profileJSON struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Config      string   `json:"config"`
	Pipeline    string   `json:"pipeline"`
	Enabled     bool     `json:"enabled"`
	SortOrder   int      `json:"sort_order"`
	RuleLayers  []string `json:"rule_layers"`
	GroupLayers []string `json:"group_layers"`
	RuleCount   int      `json:"rule_count"`
	GroupCount  int      `json:"group_count"`
}

type ruleJSON struct {
	ID         int64  `json:"id"`
	RuleType   string `json:"rule_type"`
	RuleParam  string `json:"rule_param"`
	RuleConfig int64  `json:"rule_config"`
	Enabled    bool   `json:"enabled"`
	Source     string `json:"source"`
	Raw        string `json:"raw"`
	GroupName  string `json:"group_name"`
}

type groupJSON struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Regex  string `json:"regex"`
	Source string `json:"source"`
}

type tokenJSON struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Config      string `json:"config"`
	ProfileID   int64  `json:"profile_id"`
	ProfileName string `json:"profile_name"`
}

// --- 环境与请求助手 ---

// profileServer 起一个注入临时 SQLite 的方案测试服务。
// 额外配好 LoadTemplate，让方案 config 字段（INI 模板 URL）能真正被解析。
func profileServer(t *testing.T) (*httptest.Server, *Server, *subforge.Manager) {
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
			LoadTemplate: func(ctx context.Context, ref string) (*config.Template, error) {
				return config.LoadSource(ctx, ref, config.HTTPLoader(&http.Client{Timeout: 5 * time.Second}, nil))
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	}
	srv := New(cfg)
	srv.SetSubManager(mgr)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts, srv, mgr
}

func mustDecode(t *testing.T, body string, dst any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), dst); err != nil {
		t.Fatalf("解析响应失败: %v；body=%s", err, body)
	}
}

// postAPIJSON 把 payload 序列化成 JSON 后 POST；返回状态码与正文。
func postAPIJSON(t *testing.T, ts *httptest.Server, path string, payload any) (int, string) {
	t.Helper()
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("序列化请求体: %v", err)
	}
	st, _, body := uiPostJSON(t, ts, path, string(buf))
	return st, body
}

// doJSON 把 payload（nil 表示空 body）序列化后发指定方法；返回状态码与正文。
func doJSON(t *testing.T, ts *httptest.Server, method, path string, payload any) (int, string) {
	t.Helper()
	body := ""
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("序列化请求体: %v", err)
		}
		body = string(buf)
	}
	st, _, resp := uiDoJSON(t, ts, method, path, body)
	return st, resp
}

// mustProfilePayload 用给定字段创建方案，断言 201 并返回落库后的方案。
func mustProfilePayload(t *testing.T, ts *httptest.Server, payload map[string]any) profileJSON {
	t.Helper()
	st, body := postAPIJSON(t, ts, "/api/profiles", payload)
	if st != http.StatusCreated {
		t.Fatalf("创建方案状态 = %d，应为 201；payload=%v body=%s", st, payload, body)
	}
	var resp struct {
		Profile profileJSON `json:"profile"`
	}
	mustDecode(t, body, &resp)
	if resp.Profile.ID <= 0 {
		t.Fatalf("创建方案未返回 id：%s", body)
	}
	return resp.Profile
}

func mustProfile(t *testing.T, ts *httptest.Server, name string) profileJSON {
	t.Helper()
	return mustProfilePayload(t, ts, map[string]any{"name": name})
}

func listProfiles(t *testing.T, ts *httptest.Server) []profileJSON {
	t.Helper()
	st, _, body := uiGet(t, ts, "/api/profiles")
	if st != http.StatusOK {
		t.Fatalf("GET /api/profiles 状态 = %d；body=%s", st, body)
	}
	var resp struct {
		Profiles []profileJSON `json:"profiles"`
	}
	mustDecode(t, body, &resp)
	return resp.Profiles
}

func listRules(t *testing.T, ts *httptest.Server, query string) []ruleJSON {
	t.Helper()
	st, _, body := uiGet(t, ts, "/api/rules"+query)
	if st != http.StatusOK {
		t.Fatalf("GET /api/rules%s 状态 = %d；body=%s", query, st, body)
	}
	var resp struct {
		Rules []ruleJSON `json:"rules"`
	}
	mustDecode(t, body, &resp)
	return resp.Rules
}

func listGroups(t *testing.T, ts *httptest.Server, query string) []groupJSON {
	t.Helper()
	st, _, body := uiGet(t, ts, "/api/groups"+query)
	if st != http.StatusOK {
		t.Fatalf("GET /api/groups%s 状态 = %d；body=%s", query, st, body)
	}
	var resp struct {
		Groups []groupJSON `json:"groups"`
	}
	mustDecode(t, body, &resp)
	return resp.Groups
}

func listTokens(t *testing.T, ts *httptest.Server) []tokenJSON {
	t.Helper()
	st, _, body := uiGet(t, ts, "/api/tokens")
	if st != http.StatusOK {
		t.Fatalf("GET /api/tokens 状态 = %d；body=%s", st, body)
	}
	var resp struct {
		Tokens []tokenJSON `json:"tokens"`
	}
	mustDecode(t, body, &resp)
	return resp.Tokens
}

func mustToken(t *testing.T, ts *httptest.Server, token string, profileID int64) tokenJSON {
	t.Helper()
	st, body := postAPIJSON(t, ts, "/api/tokens", map[string]any{
		"token": token, "name": "测试令牌", "profile_id": profileID,
	})
	if st != http.StatusCreated {
		t.Fatalf("创建令牌 %q 状态 = %d，应为 201；body=%s", token, st, body)
	}
	var resp struct {
		Token tokenJSON `json:"token"`
	}
	mustDecode(t, body, &resp)
	return resp.Token
}

// createGroup 建一个 select 分组（source 为空串表示自定义层），返回分组 id。
func createGroup(t *testing.T, ts *httptest.Server, name, source string) int64 {
	t.Helper()
	st, body := postAPIJSON(t, ts, "/api/groups", map[string]any{
		"name": name, "type": "select", "regex": ".", "source": source,
	})
	if st != http.StatusCreated {
		t.Fatalf("创建分组 %q 状态 = %d，应为 201；body=%s", name, st, body)
	}
	var resp struct {
		Group groupJSON `json:"group"`
	}
	mustDecode(t, body, &resp)
	return resp.Group.ID
}

// createRule 建一条规则并返回落库视图。
func createRule(t *testing.T, ts *httptest.Server, ruleType, param string, ruleConfig int64, source string, enabled bool) ruleJSON {
	t.Helper()
	st, body := postAPIJSON(t, ts, "/api/rules", map[string]any{
		"rule_type": ruleType, "rule_param": param, "rule_config": ruleConfig,
		"source": source, "enabled": enabled,
	})
	if st != http.StatusCreated {
		t.Fatalf("创建规则 %s,%s 状态 = %d，应为 201；body=%s", ruleType, param, st, body)
	}
	var resp struct {
		Rule ruleJSON `json:"rule"`
	}
	mustDecode(t, body, &resp)
	return resp.Rule
}

// subscribe 请求 /subscribe?token=...，返回状态码与正文。
func subscribe(t *testing.T, ts *httptest.Server, token string) (int, string) {
	t.Helper()
	st, _, body := doGet(t, ts.URL+"/subscribe?token="+token, "", "")
	return st, body
}

// --- 方案 CRUD 与层校验 ---

func TestAPIProfiles_CRUDAndLayers(t *testing.T) {
	ts, srv, _ := profileServer(t)

	if st, _, body := uiGet(t, ts, "/api/profiles"); st != http.StatusOK || !strings.Contains(body, `"profiles":[]`) {
		t.Fatalf("空列表应为 200 且 profiles:[]，状态=%d body=%s", st, body)
	}

	p := mustProfile(t, ts, "方案 A")
	if !p.Enabled || p.Name != "方案 A" {
		t.Fatalf("新建方案应默认启用且名称正确：%+v", p)
	}
	if len(p.RuleLayers) != 0 || len(p.GroupLayers) != 0 {
		t.Fatalf("未给层时应为空（= 默认三层）：%+v", p)
	}

	// 重名 → 409；空名 → 400。
	if st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{"name": "方案 A"}); st != http.StatusConflict {
		t.Fatalf("重名状态 = %d，应为 409；body=%s", st, body)
	} else if !strings.Contains(body, "已存在") {
		t.Errorf("重名错误文案缺少 已存在：%s", body)
	}
	if st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{"name": "   "}); st != http.StatusBadRequest {
		t.Fatalf("空名状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "方案名称不能为空") {
		t.Errorf("空名错误文案缺少 方案名称不能为空：%s", body)
	}

	// 层数组：规则层的 template 自动末位，分组层不强制。
	layered := mustProfilePayload(t, ts, map[string]any{
		"name": "方案层", "rule_layers": []string{"template", "custom"},
		"group_layers": []string{"template", "custom"},
	})
	if got := strings.Join(layered.RuleLayers, ","); got != "custom,template" {
		t.Fatalf("规则层应把 template 移到末位，实际 %q", got)
	}
	if got := strings.Join(layered.GroupLayers, ","); got != "template,custom" {
		t.Fatalf("分组层不应被重排，实际 %q", got)
	}

	// 非法层键 / 重复层 → 400，文案带前缀。
	if st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name": "坏层", "rule_layers": []string{"bogus"},
	}); st != http.StatusBadRequest {
		t.Fatalf("非法规则层状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "规则层顺序非法：未知的层 bogus") {
		t.Errorf("非法规则层文案不对：%s", body)
	}
	if st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name": "坏层2", "group_layers": []string{"custom", "custom"},
	}); st != http.StatusBadRequest {
		t.Fatalf("重复分组层状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "分组层顺序非法：重复的层 custom") {
		t.Errorf("重复分组层文案不对：%s", body)
	}

	// 单条 GET / 不存在 404。
	if st, _, body := uiGet(t, ts, fmt.Sprintf("/api/profiles/%d", p.ID)); st != http.StatusOK || !strings.Contains(body, "方案 A") {
		t.Fatalf("GET 单条状态 = %d body=%s", st, body)
	}
	if st, _, _ := uiGet(t, ts, "/api/profiles/99999"); st != http.StatusNotFound {
		t.Fatalf("GET 不存在方案状态 = %d，应为 404", st)
	}

	// PUT 是全量覆盖（enabled 缺省保持原值）：先写 config + 停用，再只传 name。
	st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{
		"name": "方案 A", "config": "http://example.invalid/tpl.ini", "enabled": false,
	})
	if st != http.StatusOK {
		t.Fatalf("PUT 状态 = %d；body=%s", st, body)
	}
	var got struct {
		Profile profileJSON `json:"profile"`
	}
	mustDecode(t, body, &got)
	if got.Profile.Enabled || got.Profile.Config != "http://example.invalid/tpl.ini" {
		t.Fatalf("PUT 未写入 enabled/config：%+v", got.Profile)
	}
	st, body = doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{"name": "方案 A"})
	if st != http.StatusOK {
		t.Fatalf("PUT 省略 enabled 状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &got)
	if got.Profile.Enabled {
		t.Errorf("PUT 省略 enabled 应保持原值 false，实际 %+v", got.Profile)
	}
	if got.Profile.Config != "" {
		t.Errorf("PUT 是全量覆盖，省略 config 应清空，实际 %q", got.Profile.Config)
	}

	// PUT 层数组生效。
	st, body = doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{
		"name": "方案 A", "rule_layers": []string{"airport"}, "group_layers": []string{"airport"},
	})
	if st != http.StatusOK {
		t.Fatalf("PUT 层状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &got)
	if strings.Join(got.Profile.RuleLayers, ",") != "airport" || strings.Join(got.Profile.GroupLayers, ",") != "airport" {
		t.Fatalf("PUT 层未落库：%+v", got.Profile)
	}

	// 重名 409（与 POST 文案一致）/ 空名 400 / 不存在 404。
	other := mustProfile(t, ts, "方案 B")
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{"name": other.Name}); st != http.StatusConflict {
		t.Fatalf("PUT 重名状态 = %d，应为 409；body=%s", st, body)
	} else if !strings.Contains(body, "订阅方案 “方案 B” 已存在") {
		t.Errorf("PUT 重名文案应与 POST 一致：%s", body)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{"name": ""}); st != http.StatusBadRequest {
		t.Fatalf("PUT 空名状态 = %d，应为 400", st)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, "/api/profiles/99999", map[string]any{"name": "x"}); st != http.StatusNotFound {
		t.Fatalf("PUT 不存在状态 = %d，应为 404", st)
	}

	// 有令牌绑着的方案不能删；解绑后可删，再删 404。
	tok := mustToken(t, ts, "tp", p.ID)
	if tok.ProfileID != p.ID {
		t.Fatalf("令牌未绑到方案：%+v", tok)
	}
	if st, body := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", p.ID), nil); st != http.StatusConflict {
		t.Fatalf("删除有令牌的方案状态 = %d，应为 409；body=%s", st, body)
	} else if !strings.Contains(body, "仍有 1 个令牌在用") {
		t.Errorf("删除冲突文案缺少令牌数：%s", body)
	}
	if st, body := doJSON(t, ts, http.MethodDelete, "/api/tokens/tp", nil); st != http.StatusOK {
		t.Fatalf("删除令牌状态 = %d；body=%s", st, body)
	}
	if st, body := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", p.ID), nil); st != http.StatusOK {
		t.Fatalf("删除空方案状态 = %d；body=%s", st, body)
	}
	if st, _ := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", p.ID), nil); st != http.StatusNotFound {
		t.Fatalf("重复删除状态 = %d，应为 404", st)
	}

	// 方法不允许（直接打 handler，路由只放行部分方法）。
	rr := httptest.NewRecorder()
	srv.handleAPIProfiles(rr, httptest.NewRequest(http.MethodPatch, "/api/profiles", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /api/profiles 状态 = %d，应为 405", rr.Code)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/profiles/1", nil)
	req.SetPathValue("id", "1")
	rr = httptest.NewRecorder()
	srv.handleAPIProfile(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /api/profiles/1 状态 = %d，应为 405", rr.Code)
	}

	// 列表里剩下 方案 B / 方案层（方案 A 已删）。
	all := listProfiles(t, ts)
	if len(all) != 2 {
		t.Fatalf("方案列表应剩 2 个：%+v", all)
	}
}

// --- 来源过滤与按来源批量 ---

func TestAPIProfiles_SourcesAndBulk(t *testing.T) {
	ts, _, _ := profileServer(t)

	gcustom := createGroup(t, ts, "自定义组", "")
	p := mustProfile(t, ts, "方案 X")
	if st, body := postAPIJSON(t, ts, "/api/groups", map[string]any{
		"name": "旧客户端组", "type": "select", "regex": ".", "profile_id": p.ID,
	}); st != http.StatusBadRequest {
		t.Fatalf("分组带 profile_id 状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "分组不再归属订阅方案") {
		t.Errorf("profile_id 下线文案不对：%s", body)
	}

	gx := createGroup(t, ts, "机场组X", "airport:x")
	createGroup(t, ts, "机场组Y", "airport:y")

	// 分组过滤：精确 / 空串（自定义）/ 前缀。
	if got := listGroups(t, ts, "?source=airport:x"); len(got) != 1 || got[0].ID != gx || got[0].Source != "airport:x" {
		t.Fatalf("按来源精确过滤分组不对：%+v", got)
	}
	if got := listGroups(t, ts, "?source="); len(got) != 1 || got[0].ID != gcustom {
		t.Fatalf("空 source 应只返回自定义分组：%+v", got)
	}
	if got := listGroups(t, ts, "?source=airport&source_prefix=1"); len(got) != 2 {
		t.Fatalf("前缀过滤应命中 2 个机场分组：%+v", got)
	}
	if st, _, body := uiGet(t, ts, "/api/groups?source=airport:x&source_prefix=bogus"); st != http.StatusBadRequest {
		t.Fatalf("坏 source_prefix 状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "source_prefix 需要布尔值") {
		t.Errorf("坏 source_prefix 文案不对：%s", body)
	}
	if st, _, body := uiGet(t, ts, "/api/groups?source=&source_prefix=1"); st != http.StatusBadRequest {
		t.Fatalf("空 source 带前缀状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "source_prefix 需要非空 source") {
		t.Errorf("空 source 前缀文案不对：%s", body)
	}

	// 分组 PUT：profile_id → 400；不存在 → 404。
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/groups/%d", gcustom), map[string]any{
		"name": "自定义组", "type": "select", "regex": ".", "profile_id": 1,
	}); st != http.StatusBadRequest {
		t.Fatalf("分组 PUT 带 profile_id 状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "分组不再归属订阅方案") {
		t.Errorf("分组 PUT profile_id 文案不对：%s", body)
	}
	if st, body := doJSON(t, ts, http.MethodPut, "/api/groups/99999", map[string]any{
		"name": "不存在", "type": "select", "regex": ".",
	}); st != http.StatusNotFound {
		t.Fatalf("不存在分组 PUT 状态 = %d，应为 404；body=%s", st, body)
	} else if !strings.Contains(body, "代理分组 99999 不存在") {
		t.Errorf("不存在分组 PUT 文案不对：%s", body)
	}

	// 规则：带 group_name 回填、来源过滤、profile_id 下线。
	r1 := createRule(t, ts, "DOMAIN-SUFFIX", "custom.test", gcustom, "", true)
	createRule(t, ts, "DOMAIN-SUFFIX", "x1.test", 0, "airport:x", false)
	createRule(t, ts, "DOMAIN-SUFFIX", "x2.test", 0, "airport:x", false)
	if st, body := postAPIJSON(t, ts, "/api/rules", map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "bad.test", "profile_id": p.ID,
	}); st != http.StatusBadRequest {
		t.Fatalf("规则带 profile_id 状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "规则不再归属订阅方案") {
		t.Errorf("规则 profile_id 下线文案不对：%s", body)
	}
	gotCustom := listRules(t, ts, "?source=")
	if len(gotCustom) != 1 || gotCustom[0].ID != r1.ID || gotCustom[0].GroupName != "自定义组" {
		t.Fatalf("自定义来源规则应 1 条且带 group_name：%+v", gotCustom)
	}
	if got := listRules(t, ts, "?source=airport:x"); len(got) != 2 {
		t.Fatalf("airport:x 规则应 2 条：%+v", got)
	}
	if st, _, body := uiGet(t, ts, "/api/rules?source=&source_prefix=1"); st != http.StatusBadRequest {
		t.Fatalf("规则空 source 前缀状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "source_prefix 需要非空 source") {
		t.Errorf("规则空 source 前缀文案不对：%s", body)
	}

	// 批量：参数校验。
	cases := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{"profile_id 下线", map[string]any{"ids": []int64{r1.ID}, "profile_id": p.ID}, "规则不再归属订阅方案"},
		{"ids 与 source 互斥", map[string]any{"ids": []int64{r1.ID}, "source": "airport:x"}, "ids 与 source 不能同时给出"},
		{"两者都缺", map[string]any{}, "必须给出 ids 或 source"},
		{"前缀需要非空 source", map[string]any{"source": "", "source_prefix": true}, "source_prefix 需要非空 source"},
	}
	for _, tc := range cases {
		st, body := postAPIJSON(t, ts, "/api/rules/bulk", tc.payload)
		if st != http.StatusBadRequest || !strings.Contains(body, tc.want) {
			t.Fatalf("%s 状态 = %d body=%s，应 400 且含 %q", tc.name, st, body, tc.want)
		}
	}

	// 按 ids 停用 → updated=1。
	st, body := postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{"ids": []int64{r1.ID}, "enabled": false})
	if st != http.StatusOK {
		t.Fatalf("按 ids 停用状态 = %d；body=%s", st, body)
	}
	var bulkResp struct {
		Updated int64 `json:"updated"`
		Deleted int64 `json:"deleted"`
	}
	mustDecode(t, body, &bulkResp)
	if bulkResp.Updated != 1 {
		t.Fatalf("按 ids 停用 updated = %d，应为 1", bulkResp.Updated)
	}
	if got := listRules(t, ts, "?source="); len(got) != 1 || got[0].Enabled {
		t.Fatalf("按 ids 停用未落库：%+v", got)
	}

	// 按来源启用 airport:x 全部 → updated=2（一条 SQL）。
	st, body = postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{"source": "airport:x", "enabled": true})
	if st != http.StatusOK {
		t.Fatalf("按来源启用状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &bulkResp)
	if bulkResp.Updated != 2 {
		t.Fatalf("按来源启用 updated = %d，应为 2", bulkResp.Updated)
	}
	for _, r := range listRules(t, ts, "?source=airport:x") {
		if !r.Enabled {
			t.Fatalf("规则 %d 应已启用：%+v", r.ID, r)
		}
	}

	// 按来源删除 airport:x → deleted=2。
	st, body = postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{"source": "airport:x", "delete": true})
	if st != http.StatusOK {
		t.Fatalf("按来源删除状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &bulkResp)
	if bulkResp.Deleted != 2 {
		t.Fatalf("按来源删除 deleted = %d，应为 2", bulkResp.Deleted)
	}
	if got := listRules(t, ts, "?source=airport:x"); len(got) != 0 {
		t.Fatalf("按来源删除后仍有规则：%+v", got)
	}

	// 按 ids 删除 → 删掉自定义规则。
	st, body = postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{"ids": []int64{r1.ID}, "delete": true})
	if st != http.StatusOK {
		t.Fatalf("按 ids 删除状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &bulkResp)
	if bulkResp.Deleted != 1 {
		t.Fatalf("按 ids 删除 deleted = %d，应为 1", bulkResp.Deleted)
	}

	// 单条改来源：成功后按新来源可查到；改到不存在的 id → 404。
	r4 := createRule(t, ts, "DOMAIN-SUFFIX", "move.test", 0, "", true)
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", r4.ID), map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "move.test", "enabled": true, "source": "manual:mine",
	}); st != http.StatusOK {
		t.Fatalf("改规则来源状态 = %d；body=%s", st, body)
	}
	if got := listRules(t, ts, "?source=manual:mine"); len(got) != 1 || got[0].ID != r4.ID {
		t.Fatalf("改来源后按新来源查不到：%+v", got)
	}
	if st, body := doJSON(t, ts, http.MethodPut, "/api/rules/99999", map[string]any{
		"rule_type": "DOMAIN", "rule_param": "x.test", "source": "manual:mine",
	}); st != http.StatusNotFound {
		t.Fatalf("不存在规则改来源状态 = %d，应为 404；body=%s", st, body)
	} else if !strings.Contains(body, "规则 99999 不存在") {
		t.Errorf("不存在规则改来源文案不对：%s", body)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", r4.ID), map[string]any{
		"rule_type": "DOMAIN", "rule_param": "x.test", "profile_id": 1,
	}); st != http.StatusBadRequest {
		t.Fatalf("规则 PUT 带 profile_id 状态 = %d，应为 400", st)
	}
}

// --- 机场导入进全局库 ---

const autoGroupYAML = "proxies:\n" +
	"  - name: p1\n" +
	"    type: ss\n" +
	"    server: s.example.com\n" +
	"    port: 8388\n" +
	"    cipher: aes-256-gcm\n" +
	"    password: pw\n" +
	"rules:\n" +
	"  - DOMAIN-SUFFIX,auto.test,🚀 节点选择\n" +
	"  - IP-CIDR,2.2.2.0/24,DIRECT\n"

func autoGroupUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(autoGroupYAML))
	}))
	t.Cleanup(up.Close)
	return up
}

func TestAPIProfiles_ImportRulesToLibrary(t *testing.T) {
	ts, _, mgr := profileServer(t)
	id := addImportAirport(t, mgr, "auto", autoGroupUpstream(t).URL)

	st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id),
		map[string]any{"create_groups": true})
	if st != http.StatusOK {
		t.Fatalf("导入状态 = %d；body=%s", st, body)
	}
	var resp struct {
		RulesAdded    int `json:"rules_added"`
		RulesSkipped  int `json:"rules_skipped"`
		GroupsCreated int `json:"groups_created"`
	}
	mustDecode(t, body, &resp)
	if resp.RulesAdded != 2 || resp.GroupsCreated != 1 {
		t.Fatalf("导入结果 rules_added=%d groups_created=%d，应为 2/1；body=%s", resp.RulesAdded, resp.GroupsCreated, body)
	}

	// 规则进全局库，来源是机场；自动建的组来源跟随规则来源，规则指向该组。
	rules := listRules(t, ts, "?source=airport:auto")
	if len(rules) != 2 {
		t.Fatalf("airport:auto 应有 2 条规则：%+v", rules)
	}
	groups := listGroups(t, ts, "?source=airport:auto")
	if len(groups) != 1 || groups[0].Name != "🚀 节点选择" {
		t.Fatalf("自动建组不对：%+v", groups)
	}
	if groups[0].ID != rules[0].RuleConfig && groups[0].ID != rules[1].RuleConfig {
		t.Fatalf("导入规则未指向自动建的组：groups=%+v rules=%+v", groups, rules)
	}

	// profile_id 下线 → 400；非法 JSON → 400；空 body 兼容（重复导入全部跳过）。
	if st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id),
		map[string]any{"profile_id": 1}); st != http.StatusBadRequest {
		t.Fatalf("导入带 profile_id 状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "不再归属订阅方案") {
		t.Errorf("导入 profile_id 下线文案不对：%s", body)
	}
	if st, _, body := uiPostJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id), "{"); st != http.StatusBadRequest {
		t.Fatalf("非法 JSON 状态 = %d，应为 400；body=%s", st, body)
	}
	st, body = postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id), map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("空 body 导入状态 = %d，应为 200；body=%s", st, body)
	}
	mustDecode(t, body, &resp)
	if resp.RulesAdded != 0 || resp.RulesSkipped != 2 || resp.GroupsCreated != 0 {
		t.Fatalf("重复导入应全部跳过，实际 %+v", resp)
	}
	if len(listRules(t, ts, "?source=airport:auto")) != 2 {
		t.Fatalf("重复导入不该新增规则")
	}
}

// --- 跨来源移动撞唯一索引 → 409 ---

func TestAPIProfiles_CrossSourceMove409(t *testing.T) {
	ts, _, mgr := profileServer(t)
	up := autoGroupUpstream(t)

	// 同一条规则原文导入两个来源：origin_key 相同，但 source 不同，允许共存。
	for _, name := range []string{"dup", "dup2"} {
		id := addImportAirport(t, mgr, name, up.URL)
		if st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id),
			map[string]any{}); st != http.StatusOK {
			t.Fatalf("导入 %s 状态 = %d；body=%s", name, st, body)
		}
	}
	rulesB := listRules(t, ts, "?source=airport:dup2")
	if len(rulesB) != 2 {
		t.Fatalf("airport:dup2 应有 2 条规则：%+v", rulesB)
	}

	// 把 dup2 的一条规则改到 dup：同来源同去重键 → 409。
	target := rulesB[0]
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", target.ID), map[string]any{
		"rule_type": target.RuleType, "rule_param": target.RuleParam,
		"rule_config": target.RuleConfig, "enabled": false, "source": "airport:dup",
	}); st != http.StatusConflict {
		t.Fatalf("跨来源重复状态 = %d，应为 409；body=%s", st, body)
	} else if !strings.Contains(body, "目标来源里已存在同来源同去重键的规则") {
		t.Errorf("409 文案不对：%s", body)
	}

	// 改成自定义来源（没有同 origin 冲突）→ 200。
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", target.ID), map[string]any{
		"rule_type": target.RuleType, "rule_param": target.RuleParam,
		"rule_config": target.RuleConfig, "enabled": false, "source": "",
	}); st != http.StatusOK {
		t.Fatalf("改到自定义来源状态 = %d，应为 200；body=%s", st, body)
	}
	if len(listRules(t, ts, "?source=")) != 1 {
		t.Fatalf("改到自定义来源后应能在 ?source= 查到")
	}
}

// --- 令牌绑方案 ---

func TestAPIProfiles_TokenBinding(t *testing.T) {
	ts, _, _ := profileServer(t)
	a := mustProfile(t, ts, "方案甲")
	b := mustProfile(t, ts, "方案乙")

	tok := mustToken(t, ts, "ta", a.ID)
	if tok.ProfileID != a.ID || tok.ProfileName != "方案甲" {
		t.Fatalf("新建令牌未回填方案：%+v", tok)
	}

	toks := listTokens(t, ts)
	if len(toks) != 1 || toks[0].ProfileID != a.ID || toks[0].ProfileName != "方案甲" {
		t.Fatalf("令牌列表未带 profile_id/profile_name：%+v", toks)
	}

	st, body := doJSON(t, ts, http.MethodPut, "/api/tokens/ta", map[string]any{"profile_id": b.ID})
	if st != http.StatusOK {
		t.Fatalf("令牌改绑状态 = %d；body=%s", st, body)
	}
	var got struct {
		Token tokenJSON `json:"token"`
	}
	mustDecode(t, body, &got)
	if got.Token.ProfileID != b.ID || got.Token.ProfileName != "方案乙" {
		t.Fatalf("令牌改绑后不对：%+v", got.Token)
	}

	st, body = doJSON(t, ts, http.MethodPut, "/api/tokens/ta", map[string]any{"profile_id": 0})
	if st != http.StatusOK {
		t.Fatalf("令牌解绑状态 = %d；body=%s", st, body)
	}
	// 解绑响应里 profile_name 为空串且带 omitempty 会被省略，重新解码前先清零，
	// 否则会残留上一次「方案乙」的值（json.Unmarshal 不重置缺失字段）。
	got.Token = tokenJSON{}
	mustDecode(t, body, &got)
	if got.Token.ProfileID != 0 || got.Token.ProfileName != "" {
		t.Fatalf("令牌解绑后不对：%+v", got.Token)
	}

	if st, body := doJSON(t, ts, http.MethodPut, "/api/tokens/ta", map[string]any{"profile_id": 99999}); st != http.StatusBadRequest {
		t.Fatalf("令牌绑定不存在方案状态 = %d，应为 400；body=%s", st, body)
	}
	if st, body := postAPIJSON(t, ts, "/api/tokens", map[string]any{
		"token": "bad", "name": "x", "profile_id": 99999,
	}); st != http.StatusBadRequest {
		t.Fatalf("新建令牌绑定不存在方案状态 = %d，应为 400；body=%s", st, body)
	}
}

// --- /subscribe 按层拼接 ---

// profileTemplateINI 是带内联 DSL 的最小 INI 模板：一个自建分组 + 一条 FINAL 规则。
const profileTemplateINI = "[custom]\n" +
	"rules=DOMAIN-SUFFIX,tplonly.test,🚀 手动选择\n" +
	"custom_proxy_group=🚀 手动选择\x60select\x60[]♻️ 自动选择\x60.*\n" +
	"ruleset=🚀 手动选择,[]FINAL\n"

// layersSubServer 起一个方案订阅测试服务：
//   - fixture 机场（45 节点）作为订阅来源；
//   - 自定义层：1 组 + 1 条已启用规则；
//   - 机场层：1 条导入规则（目标自动建组）并已启用；
//   - 模板层：profileTemplateINI（🚀 手动选择 / ♻️ 自动选择 + FINAL）。
func layersSubServer(t *testing.T) (*httptest.Server, string, *subforge.Manager) {
	t.Helper()
	ts, _, mgr := profileServer(t)
	if _, err := mgr.AddAirport(t.Context(), "fx", fixtureURL(t), "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}

	gid := createGroup(t, ts, "🅰 自定义组", "")
	createRule(t, ts, "DOMAIN-SUFFIX", "c.test", gid, "", true)

	up := addImportAirport(t, mgr, "auto", autoGroupUpstream(t).URL)
	if st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", up),
		map[string]any{"create_groups": true}); st != http.StatusOK {
		t.Fatalf("导入机场规则状态 = %d；body=%s", st, body)
	}
	if st, body := postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{
		"source": "airport:auto", "enabled": true,
	}); st != http.StatusOK {
		t.Fatalf("启用机场规则状态 = %d；body=%s", st, body)
	}

	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(profileTemplateINI))
	}))
	t.Cleanup(tpl.Close)
	return ts, tpl.URL + "/tpl.ini", mgr
}

func TestAPIProfiles_SubscribeLayers(t *testing.T) {
	ts, tplURL, mgr := layersSubServer(t)

	// 1) 只有自定义层：自定义规则/分组在，模板与机场层都被排除。
	p1 := mustProfilePayload(t, ts, map[string]any{
		"name": "自定义层", "config": tplURL,
		"rule_layers": []string{"custom"}, "group_layers": []string{"custom"},
	})
	mustToken(t, ts, "t1", p1.ID)
	st, body := subscribe(t, ts, "t1")
	if st != http.StatusOK {
		t.Fatalf("自定义层订阅状态 = %d；body=%s", st, body)
	}
	if !strings.Contains(body, "DOMAIN-SUFFIX,c.test,🅰 自定义组") {
		t.Errorf("自定义层订阅缺少自己的规则")
	}
	if !strings.Contains(body, `name: "🅰 自定义组"`) {
		t.Errorf("自定义层订阅缺少自定义分组")
	}
	if strings.Contains(body, `name: "🚀 手动选择"`) {
		t.Errorf("自定义层订阅串进了模板分组（应关闭 template 层）")
	}
	if strings.Contains(body, "auto.test") {
		t.Errorf("自定义层订阅串进了机场导入规则（应关闭 airport 层）")
	}

	// 2) 只有模板层：模板分组/规则在，自定义与机场层都不在。
	p2 := mustProfilePayload(t, ts, map[string]any{
		"name": "模板层", "config": tplURL,
		"rule_layers": []string{"template"}, "group_layers": []string{"template"},
	})
	mustToken(t, ts, "t2", p2.ID)
	st, body = subscribe(t, ts, "t2")
	if st != http.StatusOK {
		t.Fatalf("模板层订阅状态 = %d；body=%s", st, body)
	}
	if !strings.Contains(body, `name: "🚀 手动选择"`) {
		t.Errorf("模板层订阅缺少模板分组")
	}
	if !strings.Contains(body, "tplonly.test") {
		t.Errorf("模板层订阅缺少模板专属规则 tplonly.test")
	}
	if strings.Contains(body, "c.test") || strings.Contains(body, "auto.test") {
		t.Errorf("模板层订阅串进了 DB 规则")
	}
	if strings.Contains(body, `name: "🅰 自定义组"`) {
		t.Errorf("模板层订阅串进了自定义分组")
	}

	// 3) 默认三层（不设层）：三层叠加，模板规则排在 DB 规则之后（MATCH 必须最后）。
	p3 := mustProfilePayload(t, ts, map[string]any{"name": "默认层", "config": tplURL})
	mustToken(t, ts, "t3", p3.ID)
	st, body = subscribe(t, ts, "t3")
	if st != http.StatusOK {
		t.Fatalf("默认层订阅状态 = %d；body=%s", st, body)
	}
	for _, want := range []string{"DOMAIN-SUFFIX,c.test,🅰 自定义组", "auto.test", "tplonly.test", `name: "🅰 自定义组"`, `name: "🚀 手动选择"`} {
		if !strings.Contains(body, want) {
			t.Errorf("默认层订阅缺少 %q", want)
		}
	}
	if strings.Index(body, "tplonly.test") > strings.Index(body, "MATCH,") {
		t.Errorf("模板规则应排在 DB 规则之后（MATCH 最后）")
	}

	// 4) 规则层给 template,custom → 归一化成 custom,template。
	p4 := mustProfilePayload(t, ts, map[string]any{
		"name": "顺序", "config": tplURL, "rule_layers": []string{"template", "custom"},
	})
	if got := strings.Join(p4.RuleLayers, ","); got != "custom,template" {
		t.Fatalf("规则层未归一化：%q", got)
	}

	// 5) 停用方案 → 400；令牌指向被删的方案 → 404。
	p5 := mustProfile(t, ts, "停用层")
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p5.ID), map[string]any{
		"name": p5.Name, "enabled": false,
	}); st != http.StatusOK {
		t.Fatalf("停用方案状态 = %d；body=%s", st, body)
	}
	mustToken(t, ts, "t5", p5.ID)
	if st, body := subscribe(t, ts, "t5"); st != http.StatusBadRequest {
		t.Fatalf("停用方案订阅状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "已停用") {
		t.Errorf("停用方案错误文案不对：%s", body)
	}

	p6 := mustProfile(t, ts, "会消失的层")
	mustToken(t, ts, "t6", p6.ID)
	// 绕过 API 的令牌占用保护，直接从库里删掉方案，验证 /subscribe 的 404 分支。
	if _, err := mgr.DB().Raw().ExecContext(t.Context(), "DELETE FROM profiles WHERE id = ?", p6.ID); err != nil {
		t.Fatalf("直接删方案失败: %v", err)
	}
	if st, body := subscribe(t, ts, "t6"); st != http.StatusNotFound {
		t.Fatalf("令牌绑定的方案已被删，订阅状态 = %d，应为 404；body=%s", st, body)
	} else if !strings.Contains(body, "令牌绑定的订阅方案") {
		t.Errorf("404 文案不对：%s", body)
	}

	// 6) 未绑定方案的旧令牌：默认三层，能拿到自定义规则。
	mustToken(t, ts, "t7", 0)
	st, body = subscribe(t, ts, "t7")
	if st != http.StatusOK {
		t.Fatalf("旧令牌订阅状态 = %d；body=%s", st, body)
	}
	if !strings.Contains(body, "c.test") {
		t.Errorf("旧令牌订阅缺少自定义规则（默认层应含 custom）")
	}
}

// --- /api/convert 层参数 ---

func TestAPIProfiles_ConvertLayers(t *testing.T) {
	_, srv, _ := profileServer(t)
	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(profileTemplateINI))
	}))
	t.Cleanup(tpl.Close)
	tplURL := tpl.URL + "/tpl.ini"

	post := func(payload map[string]any) (int, string) {
		buf, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("序列化: %v", err)
		}
		return postAPIConvert(t, srv, string(buf))
	}

	if st, body := post(map[string]any{
		"target": "clash.meta", "urls": []string{fixtureURL(t)},
		"rule_layers": []string{"bogus"},
	}); st != http.StatusBadRequest {
		t.Fatalf("非法规则层状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "规则层顺序非法") {
		t.Errorf("非法规则层文案不对：%s", body)
	}
	if st, body := post(map[string]any{
		"target": "clash.meta", "urls": []string{fixtureURL(t)},
		"group_layers": []string{"custom", "custom"},
	}); st != http.StatusBadRequest {
		t.Fatalf("重复分组层状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "分组层顺序非法") {
		t.Errorf("重复分组层文案不对：%s", body)
	}

	// 关分组模板层：产物里没有模板分组。
	st, raw := post(map[string]any{
		"target": "clash.meta", "urls": []string{fixtureURL(t)}, "config": tplURL,
		"group_layers": []string{"custom"},
	})
	if st != http.StatusOK {
		t.Fatalf("关分组模板层状态 = %d；body=%s", st, raw)
	}
	var out apiConvertOut
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("解析转换响应: %v", err)
	}
	if strings.Contains(out.Body, `name: "🚀 手动选择"`) {
		t.Errorf("group_layers 不含 template 时不应出现模板分组")
	}

	// 关规则模板层：产物里没有模板 FINAL 规则。
	st, raw = post(map[string]any{
		"target": "clash.meta", "urls": []string{fixtureURL(t)}, "config": tplURL,
		"rule_layers": []string{"custom"},
	})
	if st != http.StatusOK {
		t.Fatalf("关规则模板层状态 = %d；body=%s", st, raw)
	}
	out = apiConvertOut{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("解析转换响应: %v", err)
	}
	if strings.Contains(out.Body, "tplonly.test") {
		t.Errorf("rule_layers 不含 template 时不应出现模板规则")
	}
}

// --- 纯函数助手 ---

func TestAPIProfiles_Helpers(t *testing.T) {
	if got, err := normalizeLayers(nil, true); got != nil || err != nil {
		t.Fatalf("nil 层应返回 nil,nil，实际 %v,%v", got, err)
	}
	if got, err := normalizeLayers([]string{" ", ""}, true); got != nil || err != nil {
		t.Fatalf("全空白层应返回 nil,nil，实际 %v,%v", got, err)
	}
	if got, err := normalizeLayers([]string{"custom", "airport"}, true); err != nil || strings.Join(got, ",") != "custom,airport" {
		t.Fatalf("正常层被改动：%v,%v", got, err)
	}
	if _, err := normalizeLayers([]string{"nope"}, true); err == nil || !strings.Contains(err.Error(), "未知的层 nope") {
		t.Fatalf("未知层未报错：%v", err)
	}
	if _, err := normalizeLayers([]string{"custom", "custom"}, true); err == nil || !strings.Contains(err.Error(), "重复的层 custom") {
		t.Fatalf("重复层未报错：%v", err)
	}
	if got, err := normalizeLayers([]string{"template", "custom"}, true); err != nil || strings.Join(got, ",") != "custom,template" {
		t.Fatalf("规则层 template 应移到末位：%v,%v", got, err)
	}
	if got, err := normalizeLayers([]string{"template", "custom"}, false); err != nil || strings.Join(got, ",") != "template,custom" {
		t.Fatalf("分组层不应重排：%v,%v", got, err)
	}

	if !isKnownLayer("custom") || !isKnownLayer("airport") || !isKnownLayer("template") || isKnownLayer("bad") {
		t.Fatalf("isKnownLayer 判定错误")
	}
	if !hasLayer([]string{"custom", "template"}, "template") || hasLayer([]string{"custom"}, "template") {
		t.Fatalf("hasLayer 判定错误")
	}

	// 未绑定方案 / 方案没配层 → 默认三层；配了层 → 用方案的。
	if g, r := subscribeLayers(nil); strings.Join(g, ",") != "custom,airport,template" || strings.Join(r, ",") != "custom,airport,template" {
		t.Fatalf("nil 方案应回默认三层：%v %v", g, r)
	}
	if g, r := subscribeLayers(&subforge.Profile{}); strings.Join(g, ",") != "custom,airport,template" || strings.Join(r, ",") != "custom,airport,template" {
		t.Fatalf("空层方案应回默认三层：%v %v", g, r)
	}
	if g, r := subscribeLayers(&subforge.Profile{
		GroupLayers: []string{"template"}, RuleLayers: []string{"custom"},
	}); strings.Join(g, ",") != "template" || strings.Join(r, ",") != "custom" {
		t.Fatalf("层没有按方案取值：%v %v", g, r)
	}

	if isUniqueConstraintError(nil) {
		t.Error("nil 不应判为唯一约束冲突")
	}
	if !isUniqueConstraintError(fmt.Errorf("UNIQUE constraint failed: profiles.name")) {
		t.Error("含 UNIQUE 的错误应判为唯一约束冲突")
	}
	if isUniqueConstraintError(fmt.Errorf("database is locked")) {
		t.Error("普通错误不应判为唯一约束冲突")
	}
	if isDuplicateRuleSourceError(nil) {
		t.Error("nil 不应判为重复来源")
	}
	if !isDuplicateRuleSourceError(fmt.Errorf("目标来源里已存在同来源同去重键的规则，无法移动")) {
		t.Error("固定文案应判为重复来源")
	}
	if isDuplicateRuleSourceError(fmt.Errorf("别的错")) {
		t.Error("其他错误不应判为重复来源")
	}

	// parseSourceFilter 的三种合法输入与两种非法输入。
	srv, _ := importTestServer(t)
	cases := []struct {
		query  string
		want   subforge.SourceFilter
		status int
	}{
		{"", subforge.SourceFilter{All: true}, 0},
		{"?source=airport:x", subforge.SourceFilter{Source: "airport:x"}, 0},
		{"?source=airport&source_prefix=true", subforge.SourceFilter{Source: "airport", Prefix: true}, 0},
		{"?source=airport:x&source_prefix=bogus", subforge.SourceFilter{}, http.StatusBadRequest},
		{"?source=&source_prefix=1", subforge.SourceFilter{}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/rules"+tc.query, nil)
		got, ok := srv.parseSourceFilter(rr, req)
		if tc.status != 0 {
			if ok || rr.Code != tc.status {
				t.Fatalf("%q 应失败且 400，实际 ok=%v code=%d", tc.query, ok, rr.Code)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Fatalf("%q 解析结果 %+v ok=%v，期望 %+v", tc.query, got, ok, tc.want)
		}
	}
}
