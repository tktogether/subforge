package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/subforge"
)

type renderedGroup struct {
	Name    string   `yaml:"name"`
	Proxies []string `yaml:"proxies"`
}

func parseRenderedGroups(t *testing.T, body string) map[string]renderedGroup {
	t.Helper()
	var doc struct {
		Groups []renderedGroup `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("解析订阅 YAML: %v\n%s", err, body)
	}
	out := make(map[string]renderedGroup, len(doc.Groups))
	for _, g := range doc.Groups {
		out[g.Name] = g
	}
	return out
}

func containsName(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// TestSubscribeIncludeSelfInGroups 开 include_self 的分组要在原成员后追加当前方案
// 选中的自建节点；关闭的组、以及方案明确排除自建节点时都不能追加。
func TestSubscribeIncludeSelfInGroups(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: AIR-1, type: ss, server: 127.0.0.1, port: 30003, cipher: aes-256-gcm, password: pass}\n"))
	}))
	t.Cleanup(up.Close)
	ap, err := mgr.AddAirport(ctx, "fx", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30001?allowInsecure=1#orig", "SELF-A", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	allGroup, err := mgr.AddProxyGroupWithMembersAndIncludeSelf(ctx, "ALL", "select", "", "", 0, 0, "airport:fx", []string{"AIR-1"}, true)
	if err != nil {
		t.Fatalf("AddProxyGroupWithMembersAndIncludeSelf(ALL): %v", err)
	}
	offGroup, err := mgr.AddProxyGroupWithMembersAndIncludeSelf(ctx, "OFF", "select", "", "", 0, 0, "airport:fx", []string{"AIR-1"}, false)
	if err != nil {
		t.Fatalf("AddProxyGroupWithMembersAndIncludeSelf(OFF): %v", err)
	}
	// 订阅会剪掉没有规则引用的分组，这两个组各挂一条规则以保留。
	for _, g := range []struct {
		id   int64
		rule string
	}{{allGroup.ID, "all.test"}, {offGroup.ID, "off.test"}} {
		if _, err := mgr.AddRuleWithSource(ctx, subforge.Rule{
			RuleType: "DOMAIN-SUFFIX", RuleParam: g.rule, RuleConfig: g.id, Enabled: true,
		}); err != nil {
			t.Fatalf("AddRuleWithSource(%s): %v", g.rule, err)
		}
	}
	mustToken(t, ts, "t-inc", 0)
	st, body := subscribe(t, ts, "t-inc")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d，应为 200；body=%s", st, body)
	}
	groups := parseRenderedGroups(t, body)
	all := groups["ALL"]
	if !containsName(all.Proxies, "AIR-1") || !containsName(all.Proxies, "SELF-A") {
		t.Fatalf("ALL 成员 = %v，应同时包含 AIR-1 与 SELF-A", all.Proxies)
	}
	off := groups["OFF"]
	if !containsName(off.Proxies, "AIR-1") || containsName(off.Proxies, "SELF-A") {
		t.Fatalf("OFF 成员 = %v，应只有 AIR-1", off.Proxies)
	}

	// 方案明确排除自建节点时，即使分组开着 include_self 也不能追加。
	p := mustProfilePayload(t, ts, map[string]any{
		"name": "仅机场", "airport_ids": []int64{ap.ID}, "self_node_ids": []int64{},
	})
	mustToken(t, ts, "t-inc-airport", p.ID)
	st, body = subscribe(t, ts, "t-inc-airport")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d，应为 200；body=%s", st, body)
	}
	all = parseRenderedGroups(t, body)["ALL"]
	if containsName(all.Proxies, "SELF-A") {
		t.Fatalf("方案排除自建节点后 ALL 仍包含 SELF-A: %v", all.Proxies)
	}
}

// TestAPIGroupIncludeSelfCreateUpdate 分组 CRUD 要能保存 include_self；
// PUT 省略该字段时保持原值，避免旧客户端静默关掉开关。
func TestAPIGroupIncludeSelfCreateUpdate(t *testing.T) {
	ts, _, _ := profileServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/groups",
		`{"name":"INC-G","type":"select","regex":".","include_self":true,"include_all_sources":true}`)
	if status != http.StatusCreated {
		t.Fatalf("POST 状态 = %d，应为 201；body=%s", status, body)
	}
	var created struct {
		Group struct {
			ID                int64 `json:"id"`
			IncludeSelf       bool  `json:"include_self"`
			IncludeAllSources bool  `json:"include_all_sources"`
		} `json:"group"`
	}
	mustDecode(t, body, &created)
	if !created.Group.IncludeSelf || !created.Group.IncludeAllSources {
		t.Fatalf("POST 未保存扩展开关: %s", body)
	}
	id := created.Group.ID

	status, _, body = uiDoJSON(t, ts, "PUT", fmt.Sprintf("/api/groups/%d", id),
		`{"name":"INC-G","type":"select","regex":"."}`)
	if status != http.StatusOK {
		t.Fatalf("PUT(省略) 状态 = %d，应为 200；body=%s", status, body)
	}
	var afterOmit struct {
		Group struct {
			IncludeSelf       bool `json:"include_self"`
			IncludeAllSources bool `json:"include_all_sources"`
		} `json:"group"`
	}
	mustDecode(t, body, &afterOmit)
	if !afterOmit.Group.IncludeSelf || !afterOmit.Group.IncludeAllSources {
		t.Fatalf("PUT 省略扩展开关后应保持 true: %s", body)
	}

	status, _, body = uiDoJSON(t, ts, "PUT", fmt.Sprintf("/api/groups/%d", id),
		`{"name":"INC-G","type":"select","regex":".","include_self":false,"include_all_sources":false}`)
	if status != http.StatusOK {
		t.Fatalf("PUT(false) 状态 = %d，应为 200；body=%s", status, body)
	}
	var afterFalse struct {
		Group struct {
			IncludeSelf       bool `json:"include_self"`
			IncludeAllSources bool `json:"include_all_sources"`
		} `json:"group"`
	}
	mustDecode(t, body, &afterFalse)
	if afterFalse.Group.IncludeSelf || afterFalse.Group.IncludeAllSources {
		t.Fatalf("PUT(false) 后扩展开关应为 false: %s", body)
	}
}

// TestSubscribeAllSourcesGroupSpansProfile 机场「全部节点」组要覆盖方案里所有机场，
// 自建是否并入仍由 include_self 控制。
func TestSubscribeAllSourcesGroupSpansProfile(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()
	mkUpstream := func(node, port string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("proxies:\n  - {name: " + node +
				", type: ss, server: 127.0.0.1, port: " + port + ", cipher: aes-256-gcm, password: pass}\n"))
		}))
	}
	upA, upB := mkUpstream("A-1", "30003"), mkUpstream("B-1", "30004")
	t.Cleanup(upA.Close)
	t.Cleanup(upB.Close)
	if _, err := mgr.AddAirport(ctx, "A", upA.URL, "", true); err != nil {
		t.Fatalf("AddAirport(A): %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "B", upB.URL, "", true); err != nil {
		t.Fatalf("AddAirport(B): %v", err)
	}
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30001?allowInsecure=1#orig", "SELF-A", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	groups := []struct {
		name        string
		members     []string
		includeSelf bool
		includeAll  bool
	}{
		{"ALL", []string{"A-1"}, true, true},
		{"ALL_NO_SELF", []string{"A-1"}, false, true},
		{"A_ONLY", []string{"A-1"}, false, false},
	}
	for _, g := range groups {
		pg, err := mgr.AddProxyGroupWithMembersAndFlags(ctx, g.name, "select", "", "", 0, 0, "airport:A", g.members, g.includeSelf, g.includeAll)
		if err != nil {
			t.Fatalf("AddProxyGroupWithMembersAndFlags(%s): %v", g.name, err)
		}
		// 订阅会剪掉没有规则引用的分组，每个组挂一条规则以保留。
		if _, err := mgr.AddRuleWithSource(ctx, subforge.Rule{
			RuleType: "DOMAIN-SUFFIX", RuleParam: g.name + ".test", RuleConfig: pg.ID, Enabled: true,
		}); err != nil {
			t.Fatalf("AddRuleWithSource(%s): %v", g.name, err)
		}
	}
	mustToken(t, ts, "t-all-sources", 0)
	st, body := subscribe(t, ts, "t-all-sources")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d，应为 200；body=%s", st, body)
	}
	rendered := parseRenderedGroups(t, body)
	all := rendered["ALL"]
	for _, want := range []string{"A-1", "B-1", "SELF-A"} {
		if !containsName(all.Proxies, want) {
			t.Fatalf("ALL 成员 = %v，应包含 %s", all.Proxies, want)
		}
	}
	noSelf := rendered["ALL_NO_SELF"]
	for _, want := range []string{"A-1", "B-1"} {
		if !containsName(noSelf.Proxies, want) {
			t.Fatalf("ALL_NO_SELF 成员 = %v，应包含 %s", noSelf.Proxies, want)
		}
	}
	if containsName(noSelf.Proxies, "SELF-A") {
		t.Fatalf("关闭 include_self 后 ALL_NO_SELF 不应包含 SELF-A: %v", noSelf.Proxies)
	}
	only := rendered["A_ONLY"]
	if containsName(only.Proxies, "B-1") || containsName(only.Proxies, "SELF-A") {
		t.Fatalf("子集组不应扩展: %v", only.Proxies)
	}
}
