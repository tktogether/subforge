package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/subforge"
)

func finalGroupFixtureAirport(t *testing.T, node string, port int) string {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(
			"proxies:\n  - {name: %s, type: ss, server: 127.0.0.1, port: %d, cipher: aes-256-gcm, password: pass}\n",
			node, port)))
	}))
	t.Cleanup(up.Close)
	return up.URL
}

func addFinalGroup(t *testing.T, mgr *subforge.Manager, source, name string, members []string) *subforge.ProxyGroup {
	t.Helper()
	g, err := mgr.AddProxyGroupWithMembersAndFlags(t.Context(), name, "select", "", "", 0, 0, source, members, false, false)
	if err != nil {
		t.Fatalf("AddProxyGroup(%s): %v", name, err)
	}
	return g
}

func addKeepRule(t *testing.T, mgr *subforge.Manager, group *subforge.ProxyGroup, domain string) {
	t.Helper()
	if _, err := mgr.AddRuleWithSource(t.Context(), subforge.Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: domain, RuleConfig: group.ID, Enabled: true,
	}); err != nil {
		t.Fatalf("AddRule(%s): %v", domain, err)
	}
}

// TestProfileFinalGroupRoundTrip 覆盖方案配置的保存/读取与形状。
func TestProfileFinalGroupRoundTrip(t *testing.T) {
	ts, _, _ := profileServer(t)
	st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name": "统一组方案",
		"final_group": map[string]any{
			"enabled":               true,
			"mode":                  "custom",
			"name":                  "🌐 统一出口",
			"include_direct":        false,
			"keep_source_terminals": true,
			"overrides":             map[string]any{"group:airport:fx": []string{"选择节点"}},
		},
	})
	if st != http.StatusCreated {
		t.Fatalf("POST 状态 = %d；body=%s", st, body)
	}
	for _, want := range []string{`"final_group"`, `"name":"🌐 统一出口"`, `"include_direct":false`, `"keep_source_terminals":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("响应缺少 %s：%s", want, body)
		}
	}
	var resp struct {
		Profile struct {
			ID int64 `json:"id"`
		} `json:"profile"`
	}
	mustDecode(t, body, &resp)
	st, _, got := uiGet(t, ts, "/api/profiles/"+jsonInt(resp.Profile.ID))
	if st != http.StatusOK || !strings.Contains(got, `"name":"🌐 统一出口"`) {
		t.Fatalf("GET 方案未保留 final_group：%d %s", st, got)
	}
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func finalGroupTestSetup(t *testing.T) (*httptest.Server, *subforge.Manager, *subforge.Profile) {
	t.Helper()
	ts, _, mgr := profileServer(t)
	if _, err := mgr.AddAirport(t.Context(), "fx", finalGroupFixtureAirport(t, "N1", 30003), "", true); err != nil {
		t.Fatalf("AddAirport(fx): %v", err)
	}
	if _, err := mgr.AddAirport(t.Context(), "fy", finalGroupFixtureAirport(t, "N2", 30004), "", true); err != nil {
		t.Fatalf("AddAirport(fy): %v", err)
	}
	termA := addFinalGroup(t, mgr, "airport:fx", "🔰 选择节点", []string{"N1", "DIRECT"})
	serviceA := addFinalGroup(t, mgr, "airport:fx", "Netflix", []string{termA.Name, "N1", "DIRECT"})
	addKeepRule(t, mgr, serviceA, "netflix.example")
	termB := addFinalGroup(t, mgr, "airport:fy", "Lite Connect", []string{"N2", "DIRECT"})
	serviceB := addFinalGroup(t, mgr, "airport:fy", "Apple", []string{termB.Name, "N2", "DIRECT"})
	addKeepRule(t, mgr, serviceB, "apple.example")

	st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name":         "统一组端到端",
		"rule_layers":  []string{"custom"},
		"group_layers": []string{"airport"},
		"final_group": map[string]any{
			"enabled":        true,
			"mode":           "custom",
			"name":           "🚀 节点选择",
			"include_direct": true,
		},
	})
	if st != http.StatusCreated {
		t.Fatalf("创建方案状态 = %d；body=%s", st, body)
	}
	var resp struct {
		Profile *subforge.Profile `json:"profile"`
	}
	mustDecode(t, body, &resp)
	return ts, mgr, resp.Profile
}

// TestSubscribeUnifiesFinalGroups 端到端：两个机场最终组合并为一个，服务组改为引用统一组。
func TestSubscribeUnifiesFinalGroups(t *testing.T) {
	ts, _, p := finalGroupTestSetup(t)
	mustToken(t, ts, "t-final", p.ID)
	st, body := subscribe(t, ts, "t-final")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d；body=%s", st, body)
	}
	groups := parseRenderedGroups(t, body)
	unified, ok := groups["🚀 节点选择"]
	if !ok {
		t.Fatalf("产物缺少统一组：%s", body)
	}
	for _, node := range []string{"N1", "N2", "DIRECT"} {
		if !containsName(unified.Proxies, node) {
			t.Errorf("统一组成员缺少 %s：%v", node, unified.Proxies)
		}
	}
	for _, gone := range []string{"🔰 选择节点", "Lite Connect"} {
		if _, exists := groups[gone]; exists {
			t.Errorf("来源最终组 %s 应被删除", gone)
		}
	}
	for _, service := range []string{"Netflix", "Apple"} {
		g, exists := groups[service]
		if !exists {
			t.Fatalf("服务组 %s 丢失", service)
		}
		if !containsName(g.Proxies, "🚀 节点选择") {
			t.Errorf("服务组 %s 未引用统一组：%v", service, g.Proxies)
		}
	}
	if !strings.Contains(body, "MATCH,🚀 节点选择") {
		t.Errorf("MATCH 未指向统一组")
	}
}

// TestProfileFinalGroupPreview 预览返回两个来源最终组与影响计数。
func TestProfileFinalGroupPreview(t *testing.T) {
	ts, _, p := finalGroupTestSetup(t)
	st, body := postAPIJSON(t, ts, "/api/profiles/"+jsonInt(p.ID)+"/final-group/preview", map[string]any{
		"final_group": map[string]any{
			"enabled":        true,
			"mode":           "auto",
			"include_direct": true,
		},
	})
	if st != http.StatusOK {
		t.Fatalf("预览状态 = %d；body=%s", st, body)
	}
	var resp struct {
		Report render.FinalGroupReport `json:"report"`
	}
	mustDecode(t, body, &resp)
	if resp.Report.UnifiedName != "🚀 节点选择" {
		t.Errorf("UnifiedName = %q", resp.Report.UnifiedName)
	}
	if len(resp.Report.Terminals) != 2 {
		t.Errorf("Terminals = %+v，期望 2 个", resp.Report.Terminals)
	}
	if resp.Report.GroupsAfter != 3 {
		t.Errorf("GroupsAfter = %d，期望 unified+Netflix+Apple=3", resp.Report.GroupsAfter)
	}
}
