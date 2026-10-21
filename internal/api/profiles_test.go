// 订阅方案（Profile）HTTP 层测试：/api/profiles CRUD、规则/分组/令牌的方案隔离、
// 导入到方案（含自动建组）、批量移动与 409 冲突、以及 /subscribe 的方案优先级。
//
// 这些用例覆盖 internal/api/profiles.go 与 subscription.go / rules_bulk.go /
// import_rules.go 里本轮新增的方案分支，是 CI 函数覆盖率门禁（-min-func 100）
// 不红的前提。
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
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Desc       string `json:"description"`
	Config     string `json:"config"`
	Pipeline   string `json:"pipeline"`
	Enabled    bool   `json:"enabled"`
	SortOrder  int    `json:"sort_order"`
	RuleCount  int    `json:"rule_count"`
	GroupCount int    `json:"group_count"`
}

type ruleJSON struct {
	ID          int64  `json:"id"`
	RuleType    string `json:"rule_type"`
	RuleParam   string `json:"rule_param"`
	RuleConfig  int64  `json:"rule_config"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"`
	Raw         string `json:"raw"`
	ProfileID   int64  `json:"profile_id"`
	ProfileName string `json:"profile_name"`
	GroupName   string `json:"group_name"`
}

type groupJSON struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Regex       string `json:"regex"`
	ProfileID   int64  `json:"profile_id"`
	ProfileName string `json:"profile_name"`
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

// postJSON 把 payload 序列化成 JSON 后 POST；返回状态码与正文。
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

func mustProfile(t *testing.T, ts *httptest.Server, name string) profileJSON {
	t.Helper()
	st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{"name": name})
	if st != http.StatusCreated {
		t.Fatalf("创建方案 %q 状态 = %d，应为 201；body=%s", name, st, body)
	}
	var resp struct {
		Profile profileJSON `json:"profile"`
	}
	mustDecode(t, body, &resp)
	if resp.Profile.ID <= 0 {
		t.Fatalf("创建方案 %q 未返回 id：%s", name, body)
	}
	return resp.Profile
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

// createGroup 在方案下建一个 select 分组，返回分组 id。
func createGroup(t *testing.T, ts *httptest.Server, name string, profileID int64) int64 {
	t.Helper()
	st, body := postAPIJSON(t, ts, "/api/groups", map[string]any{
		"name": name, "type": "select", "regex": ".", "profile_id": profileID,
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

// --- 用例 ---

func TestAPIProfiles_CRUD(t *testing.T) {
	ts, srv, _ := profileServer(t)

	if st, _, body := uiGet(t, ts, "/api/profiles"); st != http.StatusOK || !strings.Contains(body, "\"profiles\":[]") {
		t.Fatalf("空列表应为 200 且 profiles:[]，状态=%d body=%s", st, body)
	}

	p := mustProfile(t, ts, "方案 A")
	if !p.Enabled || p.Name != "方案 A" {
		t.Fatalf("新建方案应默认启用且名称正确：%+v", p)
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

	// 单条 GET / 不存在 404。
	if st, _, body := uiGet(t, ts, fmt.Sprintf("/api/profiles/%d", p.ID)); st != http.StatusOK || !strings.Contains(body, "方案 A") {
		t.Fatalf("GET 单条状态 = %d body=%s", st, body)
	}
	if st, _, _ := uiGet(t, ts, "/api/profiles/99999"); st != http.StatusNotFound {
		t.Fatalf("GET 不存在方案状态 = %d，应为 404", st)
	}

	// PUT 显式 enabled=false 生效；再次 PUT 省略 enabled 时保持 false。
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

	st, body = doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{"name": "方案 A2"})
	if st != http.StatusOK {
		t.Fatalf("PUT 省略 enabled 状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &got)
	if got.Profile.Enabled {
		t.Errorf("PUT 省略 enabled 应保持原值 false，实际 %+v", got.Profile)
	}

	// 改成已存在的名字 → 409；空名 → 400；不存在 → 404。
	other := mustProfile(t, ts, "方案 B")
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{"name": other.Name}); st != http.StatusConflict {
		t.Fatalf("PUT 重名状态 = %d，应为 409；body=%s", st, body)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p.ID), map[string]any{"name": ""}); st != http.StatusBadRequest {
		t.Fatalf("PUT 空名状态 = %d，应为 400", st)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, "/api/profiles/99999", map[string]any{"name": "x"}); st != http.StatusNotFound {
		t.Fatalf("PUT 不存在状态 = %d，应为 404", st)
	}

	// 有内容的方案不能删（先绑一个分组）。
	createGroup(t, ts, "组", p.ID)
	if st, body := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", p.ID), nil); st != http.StatusConflict {
		t.Fatalf("删除有内容方案状态 = %d，应为 409；body=%s", st, body)
	} else if !strings.Contains(body, "请先移动或删除") {
		t.Errorf("删除冲突文案缺少提示：%s", body)
	}

	// 空方案可删；再删 → 404。
	if st, body := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", other.ID), nil); st != http.StatusOK {
		t.Fatalf("删除空方案状态 = %d；body=%s", st, body)
	}
	if st, _ := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", other.ID), nil); st != http.StatusNotFound {
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
}

func TestAPIProfiles_RulesAndGroupsScoped(t *testing.T) {
	ts, srv, _ := profileServer(t)
	a := mustProfile(t, ts, "方案甲")
	b := mustProfile(t, ts, "方案乙")

	// 非法 profile_id 过滤参数。
	for _, q := range []string{"?profile_id=abc", "?profile_id=-1"} {
		if st, _, body := uiGet(t, ts, "/api/rules"+q); st != http.StatusBadRequest {
			t.Fatalf("GET /api/rules%s 状态 = %d，应为 400；body=%s", q, st, body)
		} else if !strings.Contains(body, "profile_id 必须是非负整数") {
			t.Errorf("非法过滤参数文案不对：%s", body)
		}
	}

	gid := createGroup(t, ts, "🅰 方案甲", a.ID)
	if st, body := postAPIJSON(t, ts, "/api/groups", map[string]any{
		"name": "🅱 方案乙", "type": "select", "regex": ".", "profile_id": 99999,
	}); st != http.StatusBadRequest {
		t.Fatalf("分组绑定不存在方案状态 = %d，应为 400；body=%s", st, body)
	}

	groupsA := listGroups(t, ts, fmt.Sprintf("?profile_id=%d", a.ID))
	if len(groupsA) != 1 || groupsA[0].ID != gid || groupsA[0].ProfileName != "方案甲" {
		t.Fatalf("方案甲分组应有 1 条且回填 profile_name：%+v", groupsA)
	}
	if got := listGroups(t, ts, fmt.Sprintf("?profile_id=%d", b.ID)); len(got) != 0 {
		t.Fatalf("方案乙不应看到方案甲的分组：%+v", got)
	}

	// 规则：带方案创建 → 列表过滤 + group_name/profile_name 回填。
	st, body := postAPIJSON(t, ts, "/api/rules", map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "a.test",
		"rule_config": gid, "enabled": true, "profile_id": a.ID,
	})
	if st != http.StatusCreated {
		t.Fatalf("创建规则状态 = %d；body=%s", st, body)
	}
	if st, body := postAPIJSON(t, ts, "/api/rules", map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "x.test", "profile_id": 99999,
	}); st != http.StatusBadRequest {
		t.Fatalf("规则绑定不存在方案状态 = %d，应为 400；body=%s", st, body)
	}
	rulesA := listRules(t, ts, fmt.Sprintf("?profile_id=%d", a.ID))
	if len(rulesA) != 1 {
		t.Fatalf("方案甲规则应有 1 条：%+v", rulesA)
	}
	rid := rulesA[0].ID
	if rulesA[0].ProfileName != "方案甲" || rulesA[0].GroupName != "🅰 方案甲" || !rulesA[0].Enabled {
		t.Fatalf("规则视图回填不对：%+v", rulesA[0])
	}
	if got := listRules(t, ts, fmt.Sprintf("?profile_id=%d", b.ID)); len(got) != 0 {
		t.Fatalf("方案乙不应看到方案甲的规则：%+v", got)
	}

	// 单条 PUT 改归属：规则移回方案乙；坏方案 400；不存在规则 404。
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", rid), map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "a.test",
		"rule_config": gid, "enabled": true, "profile_id": b.ID,
	}); st != http.StatusOK {
		t.Fatalf("规则改归属状态 = %d；body=%s", st, body)
	}
	if len(listRules(t, ts, fmt.Sprintf("?profile_id=%d", a.ID))) != 0 || len(listRules(t, ts, fmt.Sprintf("?profile_id=%d", b.ID))) != 1 {
		t.Fatalf("规则改归属后两方案计数不对")
	}
	if st, _ := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", rid), map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "a.test", "profile_id": 99999,
	}); st != http.StatusBadRequest {
		t.Fatalf("规则改到不存在方案状态 = %d，应为 400", st)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, "/api/rules/99999", map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "a.test", "profile_id": a.ID,
	}); st != http.StatusNotFound {
		t.Fatalf("不存在规则改归属状态 = %d，应为 404", st)
	}

	// 批量移动：ids 模式、互斥与非法参数。
	st, body = postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{"ids": []int64{rid}, "profile_id": a.ID})
	if st != http.StatusOK {
		t.Fatalf("批量移动 ids 状态 = %d；body=%s", st, body)
	}
	var bulkResp struct {
		Updated int64 `json:"updated"`
	}
	mustDecode(t, body, &bulkResp)
	if bulkResp.Updated != 1 {
		t.Fatalf("批量移动 updated = %d，应为 1", bulkResp.Updated)
	}
	cases := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{"delete 冲突", map[string]any{"ids": []int64{rid}, "delete": true, "profile_id": a.ID}, "profile_id 与 delete 不能同时使用"},
		{"ids 与 source 互斥", map[string]any{"ids": []int64{rid}, "source": "airport:x", "profile_id": a.ID}, "ids 与 source 不能同时给出"},
		{"缺少 ids/source", map[string]any{"profile_id": a.ID}, "必须给出 ids 或 source"},
		{"无效方案", map[string]any{"ids": []int64{1}, "profile_id": 99999}, "订阅方案 99999 不存在"},
	}
	for _, tc := range cases {
		st, body := postAPIJSON(t, ts, "/api/rules/bulk", tc.payload)
		if st != http.StatusBadRequest || !strings.Contains(body, tc.want) {
			t.Fatalf("%s 状态 = %d body=%s，应 400 且含 %q", tc.name, st, body, tc.want)
		}
	}

	// 分组改归属 / 批量改归属。
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/groups/%d", gid), map[string]any{
		"name": "🅰 方案甲", "type": "select", "regex": ".", "profile_id": b.ID,
	}); st != http.StatusOK {
		t.Fatalf("分组改归属状态 = %d；body=%s", st, body)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/groups/%d", gid), map[string]any{
		"name": "x", "type": "select", "regex": ".", "profile_id": 99999,
	}); st != http.StatusBadRequest {
		t.Fatalf("分组改到不存在方案状态 = %d，应为 400", st)
	}
	if st, _ := doJSON(t, ts, http.MethodPut, "/api/groups/99999", map[string]any{
		"name": "x", "type": "select", "regex": ".", "profile_id": a.ID,
	}); st != http.StatusNotFound {
		t.Fatalf("不存在分组改归属状态 = %d，应为 404", st)
	}
	st, body = postAPIJSON(t, ts, "/api/groups/bulk", map[string]any{"ids": []int64{gid}, "profile_id": a.ID})
	if st != http.StatusOK {
		t.Fatalf("批量移动分组状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &bulkResp)
	if bulkResp.Updated != 1 {
		t.Fatalf("批量移动分组 updated = %d，应为 1", bulkResp.Updated)
	}
	if st, body := postAPIJSON(t, ts, "/api/groups/bulk", map[string]any{"profile_id": a.ID}); st != http.StatusBadRequest {
		t.Fatalf("批量移动分组空 ids 状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "必须给出 ids") {
		t.Errorf("空 ids 文案不对：%s", body)
	}
	if st, _ := postAPIJSON(t, ts, "/api/groups/bulk", map[string]any{"ids": []int64{1}, "profile_id": 99999}); st != http.StatusBadRequest {
		t.Fatalf("批量移动分组到不存在方案状态 = %d，应为 400", st)
	}
	rr := httptest.NewRecorder()
	srv.handleAPIGroupsBulk(rr, httptest.NewRequest(http.MethodGet, "/api/groups/bulk", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/groups/bulk 状态 = %d，应为 405", rr.Code)
	}
}

func TestAPIProfiles_TokenBinding(t *testing.T) {
	ts, _, _ := profileServer(t)
	a := mustProfile(t, ts, "方案甲")
	b := mustProfile(t, ts, "方案乙")

	tok := mustToken(t, ts, "ta", a.ID)
	if tok.ProfileID != a.ID || tok.ProfileName != "方案甲" {
		t.Fatalf("新建令牌未回填方案：%+v", tok)
	}

	// 列表与单条 PUT 的改绑/解绑。
	toks := listTokens(t, ts)
	if len(toks) != 1 || toks[0].ProfileID != a.ID {
		t.Fatalf("令牌列表未带 profile_id：%+v", toks)
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

// autoGroupYAML 是一条目标分组尚不存在的最小订阅：导入并自动建组时应新增 1 个分组。
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

func TestAPIProfiles_ImportRulesToProfile(t *testing.T) {
	ts, _, mgr := profileServer(t)
	p := mustProfile(t, ts, "导入目标")
	id := addImportAirport(t, mgr, "auto", autoGroupUpstream(t).URL)

	st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id),
		map[string]any{"profile_id": p.ID, "create_groups": true})
	if st != http.StatusOK {
		t.Fatalf("导入到方案状态 = %d；body=%s", st, body)
	}
	var resp struct {
		RulesAdded    int `json:"rules_added"`
		GroupsCreated int `json:"groups_created"`
	}
	mustDecode(t, body, &resp)
	if resp.RulesAdded != 2 || resp.GroupsCreated != 1 {
		t.Fatalf("导入结果 rules_added=%d groups_created=%d，应为 2/1；body=%s", resp.RulesAdded, resp.GroupsCreated, body)
	}
	if got := listRules(t, ts, fmt.Sprintf("?profile_id=%d", p.ID)); len(got) != 2 {
		t.Fatalf("方案下应有 2 条导入规则：%+v", got)
	}
	groups := listGroups(t, ts, fmt.Sprintf("?profile_id=%d", p.ID))
	if len(groups) != 1 || groups[0].Name != "🚀 节点选择" || groups[0].ProfileID != p.ID {
		t.Fatalf("自动建组不对：%+v", groups)
	}

	// 坏方案 / 非法 JSON → 400；空 body 兼容旧调用（未分配 + 不建组）。
	if st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id),
		map[string]any{"profile_id": 99999}); st != http.StatusBadRequest {
		t.Fatalf("导入到不存在方案状态 = %d，应为 400；body=%s", st, body)
	}
	if st, _, body := uiPostJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id), "{"); st != http.StatusBadRequest {
		t.Fatalf("非法 JSON 状态 = %d，应为 400；body=%s", st, body)
	}
	if st, _, body := uiPostJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id), ""); st != http.StatusOK {
		t.Fatalf("空 body 导入状态 = %d，应为 200；body=%s", st, body)
	}
}

func TestAPIProfiles_MoveDuplicateRule409(t *testing.T) {
	ts, _, mgr := profileServer(t)
	a := mustProfile(t, ts, "方案甲")
	b := mustProfile(t, ts, "方案乙")

	lines := []string{"DOMAIN-SUFFIX,dup.test,DIRECT"}
	if _, err := mgr.ImportRulesToProfile(t.Context(), a.ID, "airport:dup", lines, nil, false); err != nil {
		t.Fatalf("导入方案甲: %v", err)
	}
	if _, err := mgr.ImportRulesToProfile(t.Context(), b.ID, "airport:dup", lines, nil, false); err != nil {
		t.Fatalf("导入方案乙: %v", err)
	}
	rulesB := listRules(t, ts, fmt.Sprintf("?profile_id=%d", b.ID))
	if len(rulesB) != 1 {
		t.Fatalf("方案乙应有 1 条规则：%+v", rulesB)
	}

	// 批量移动撞同源同去重键 → 409。
	if st, body := postAPIJSON(t, ts, "/api/rules/bulk", map[string]any{
		"ids": []int64{rulesB[0].ID}, "profile_id": a.ID,
	}); st != http.StatusConflict {
		t.Fatalf("批量移动重复规则状态 = %d，应为 409；body=%s", st, body)
	} else if !strings.Contains(body, "目标方案里已存在") {
		t.Errorf("409 文案缺少 目标方案里已存在：%s", body)
	}

	// 单条 PUT 撞唯一索引同样 409。
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/rules/%d", rulesB[0].ID), map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "dup.test", "enabled": false, "profile_id": a.ID,
	}); st != http.StatusConflict {
		t.Fatalf("单条改归属重复状态 = %d，应为 409；body=%s", st, body)
	}
}

// profileTemplateINI 是带内联 DSL 的最小 INI 模板：一个自建分组 + 一条 FINAL 规则。
// 方案乙绑定它，断言 /subscribe 真的用了方案自己的模板。
const profileTemplateINI = "[custom]\n" +
	"custom_proxy_group=🚀 手动选择\x60select\x60[]♻️ 自动选择\x60.*\n" +
	"ruleset=🚀 手动选择,[]FINAL\n"

func TestAPIProfiles_SubscribeIsolation(t *testing.T) {
	ts, _, mgr := profileServer(t)
	if _, err := mgr.AddAirport(t.Context(), "fx", fixtureURL(t), "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}

	// 方案甲：DB 分组 + DB 规则（无模板）。
	a := mustProfile(t, ts, "方案甲")
	gid := createGroup(t, ts, "🅰 方案甲", a.ID)
	if st, body := postAPIJSON(t, ts, "/api/rules", map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "a.test",
		"rule_config": gid, "enabled": true, "profile_id": a.ID,
	}); st != http.StatusCreated {
		t.Fatalf("创建方案甲规则状态 = %d；body=%s", st, body)
	}
	mustToken(t, ts, "ta", a.ID)
	st, _, body := doGet(t, ts.URL+"/subscribe?token=ta", "", "")
	if st != http.StatusOK {
		t.Fatalf("/subscribe ta 状态 = %d；body=%s", st, body)
	}
	if !strings.Contains(body, "DOMAIN-SUFFIX,a.test,🅰 方案甲") {
		t.Errorf("方案甲订阅缺少自己的规则：\n%s", body)
	}

	// 方案乙：只有 INI 模板（config 落在方案上），没有 DB 规则/分组。
	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(profileTemplateINI))
	}))
	t.Cleanup(tpl.Close)
	b := mustProfile(t, ts, "方案乙")
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", b.ID), map[string]any{
		"name": b.Name, "config": tpl.URL + "/tpl.ini",
	}); st != http.StatusOK {
		t.Fatalf("给方案乙写 config 状态 = %d；body=%s", st, body)
	}
	mustToken(t, ts, "tb", b.ID)
	st, _, body = doGet(t, ts.URL+"/subscribe?token=tb", "", "")
	if st != http.StatusOK {
		t.Fatalf("/subscribe tb 状态 = %d；body=%s", st, body)
	}
	if !strings.Contains(body, "🚀 手动选择") {
		t.Errorf("方案乙订阅未应用自己的 INI 模板：\n%s", body)
	}
	if strings.Contains(body, "a.test") {
		t.Errorf("方案乙订阅串进了方案甲的 DB 规则：\n%s", body)
	}

	// 停用方案 → 400；方案被删 → 404（令牌指向不存在的方案）。
	c := mustProfile(t, ts, "方案丙")
	if st, body := doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", c.ID), map[string]any{
		"name": c.Name, "enabled": false,
	}); st != http.StatusOK {
		t.Fatalf("停用方案状态 = %d；body=%s", st, body)
	}
	mustToken(t, ts, "tc", c.ID)
	if st, _, body := doGet(t, ts.URL+"/subscribe?token=tc", "", ""); st != http.StatusBadRequest {
		t.Fatalf("/subscribe 停用方案状态 = %d，应为 400；body=%s", st, body)
	} else if !strings.Contains(body, "已停用") {
		t.Errorf("停用方案错误文案不对：%s", body)
	}

	d := mustProfile(t, ts, "方案丁")
	mustToken(t, ts, "td", d.ID)
	if st, body := doJSON(t, ts, http.MethodDelete, fmt.Sprintf("/api/profiles/%d", d.ID), nil); st != http.StatusOK {
		t.Fatalf("删除空方案状态 = %d；body=%s", st, body)
	}
	if st, _, body := doGet(t, ts.URL+"/subscribe?token=td", "", ""); st != http.StatusNotFound {
		t.Fatalf("/subscribe 指向已删方案状态 = %d，应为 404；body=%s", st, body)
	} else if !strings.Contains(body, "令牌绑定的订阅方案") {
		t.Errorf("方案缺失错误文案不对：%s", body)
	}

	// 回归：未绑定方案的旧令牌仍能拿到全局规则。
	if st, body := postAPIJSON(t, ts, "/api/rules", map[string]any{
		"rule_type": "DOMAIN-SUFFIX", "rule_param": "legacy.test", "enabled": true,
	}); st != http.StatusCreated {
		t.Fatalf("创建全局规则状态 = %d；body=%s", st, body)
	}
	mustToken(t, ts, "tl", 0)
	st, _, body = doGet(t, ts.URL+"/subscribe?token=tl", "", "")
	if st != http.StatusOK || !strings.Contains(body, "legacy.test") {
		t.Fatalf("旧令牌回归失败：状态=%d\n%s", st, body)
	}
}

func TestAPIProfiles_ErrorHelpers(t *testing.T) {
	if isUniqueConstraintError(nil) {
		t.Error("nil 不应判为唯一约束冲突")
	}
	if !isUniqueConstraintError(fmt.Errorf("UNIQUE constraint failed: profiles.name")) {
		t.Error("含 UNIQUE 的错误应判为唯一约束冲突")
	}
	if isUniqueConstraintError(fmt.Errorf("database is locked")) {
		t.Error("普通错误不应判为唯一约束冲突")
	}
	if isDuplicateRuleMoveError(nil) {
		t.Error("nil 不应判为重复移动")
	}
	if !isDuplicateRuleMoveError(fmt.Errorf("目标方案里已存在同来源同去重键的规则，无法移动")) {
		t.Error("固定文案应判为重复移动")
	}
	if isDuplicateRuleMoveError(fmt.Errorf("别的错")) {
		t.Error("其他错误不应判为重复移动")
	}
}
