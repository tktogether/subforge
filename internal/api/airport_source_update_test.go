package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subforge"
)

// TestAirportRuleSourceUpdateAndGuard 规则源页的更新必须走机场规则提取，
// 不能把机场完整 YAML（含节点配置）写进规则产物；机场源也不能单独删除。
func TestAirportRuleSourceUpdateAndGuard(t *testing.T) {
	ts, _, mgr := profileServer(t)
	up := rulesUpstream(t)
	id := addImportAirport(t, mgr, "src-up", up.URL)

	st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id), map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("首次导入状态 = %d；body=%s", st, body)
	}
	var imported struct {
		RuleSourceName string `json:"rule_source_name"`
	}
	mustDecode(t, body, &imported)
	if imported.RuleSourceName == "" {
		t.Fatalf("未生成机场规则源：%s", body)
	}

	// 单源更新：走机场订阅重新提取，返回版本。
	st, body = postAPIJSON(t, ts, "/api/rule-sources/update-one", map[string]any{"name": imported.RuleSourceName})
	if st != http.StatusOK {
		t.Fatalf("单源更新状态 = %d；body=%s", st, body)
	}
	var updated struct {
		Results []struct {
			Source  string `json:"source"`
			Version string `json:"version"`
			Err     string `json:"err"`
		} `json:"results"`
	}
	mustDecode(t, body, &updated)
	if len(updated.Results) != 1 || updated.Results[0].Err != "" || updated.Results[0].Version == "" {
		t.Fatalf("单源更新结果不对：%+v body=%s", updated, body)
	}

	// 产物是规则 YAML，不应包含机场节点配置。
	sources, err := mgr.ListRuleSources(t.Context())
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	var latestPath string
	for _, src := range sources {
		if src.Name == imported.RuleSourceName {
			latestPath = filepath.Join(src.Dir, src.Latest)
		}
	}
	if latestPath == "" {
		t.Fatalf("找不到机场规则源产物：%+v", sources)
	}
	b, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("读取规则源产物: %v", err)
	}
	if strings.Contains(string(b), "proxies:") {
		t.Fatalf("规则源产物混入了机场节点配置：%s", string(b))
	}
	if !strings.Contains(string(b), "rules:") {
		t.Fatalf("规则源产物缺少 rules：%s", string(b))
	}

	// 全部更新：机场源走机场提取，其它源走远程清单更新。
	st, body = postAPIJSON(t, ts, "/api/rule-sources/update", map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("全部更新状态 = %d；body=%s", st, body)
	}
	var all struct {
		Results []struct {
			Source string `json:"source"`
			Err    string `json:"err"`
		} `json:"results"`
	}
	mustDecode(t, body, &all)
	if len(all.Results) != 1 || all.Results[0].Source != imported.RuleSourceName || all.Results[0].Err != "" {
		t.Fatalf("全部更新结果不对：%+v body=%s", all, body)
	}

	// 机场源由机场维护，不能单独删除。
	st, body = postAPIJSON(t, ts, "/api/rule-sources/remove", map[string]any{"name": imported.RuleSourceName})
	if st != http.StatusBadRequest || !strings.Contains(body, "机场规则源") {
		t.Fatalf("删除机场规则源状态 = %d body=%s，应 400 且说明由机场维护", st, body)
	}
}

func TestAirportRuleSourceNaming(t *testing.T) {
	name := subforge.AirportRuleSourceName("a:b/c", 7)
	if name != "airport-7-a-b-c" {
		t.Fatalf("AirportRuleSourceName = %q", name)
	}
	if got := subforge.AirportRuleSourceLabel(name); got != "a-b-c" {
		t.Fatalf("AirportRuleSourceLabel = %q", got)
	}
	id, ok := subforge.AirportIDFromRuleSourceName(name)
	if !ok || id != 7 {
		t.Fatalf("AirportIDFromRuleSourceName = %d/%v", id, ok)
	}
	if name := subforge.AirportRuleSourceName("  ", 9); name != "airport-9" {
		t.Fatalf("空机场名 = %q，应为 airport-9", name)
	}
	if _, ok := subforge.AirportIDFromRuleSourceName("remote-source"); ok {
		t.Fatal("远程源不应被识别成机场源")
	}
	if _, ok := subforge.AirportIDFromRuleSourceName("airport-x-y"); ok {
		t.Fatal("非法 id 不应被识别")
	}
	// 防止文件名过长。
	long := strings.Repeat("长", 120)
	got := subforge.AirportRuleSourceName(long, 1)
	if len([]rune(got)) > 100 {
		t.Fatalf("规则源名过长: %d", len([]rune(got)))
	}
}

// TestAirportRuleSourceRenameReusesSource 机场改名后复用同一规则源，
// 展示名跟随新机场名，避免生成第二个源、方案绑定留在旧源上。
func TestAirportRuleSourceRenameReusesSource(t *testing.T) {
	_, _, mgr := profileServer(t)
	ctx := t.Context()
	a, err := mgr.AddAirport(ctx, "旧名", "http://example.com/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	first, err := mgr.SaveAirportRuleSource(ctx, "旧名", a.ID, a.SubscriptionURL, []string{"DOMAIN,a.test,DIRECT"}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource(旧名): %v", err)
	}
	if _, err := mgr.UpdateAirport(ctx, a.ID, "新名", a.SubscriptionURL, "", true); err != nil {
		t.Fatalf("UpdateAirport: %v", err)
	}
	second, err := mgr.SaveAirportRuleSource(ctx, "新名", a.ID, a.SubscriptionURL, []string{"DOMAIN,b.test,DIRECT"}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource(新名): %v", err)
	}
	if second.Name != first.Name {
		t.Fatalf("机场改名后规则源名变化：%q -> %q", first.Name, second.Name)
	}
	sources, err := mgr.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	airportSources := 0
	label := ""
	for _, src := range sources {
		if src.Origin == "airport" {
			airportSources++
			label = src.Label
		}
	}
	if airportSources != 1 {
		t.Fatalf("机场规则源数量 = %d，应为 1：%+v", airportSources, sources)
	}
	if label != "新名" {
		t.Fatalf("规则源展示名 = %q，应为新名", label)
	}
	rules, _ := mgr.RuleSourceRules(ctx, []string{second.Name})
	if len(rules) != 1 || rules[0] != "DOMAIN,b.test,DIRECT" {
		t.Fatalf("改名后最新规则不对：%v", rules)
	}
}

// TestDeleteAirportUnbindsRuleSource 删除机场要解绑所有方案里的该规则源，
// 源作为历史快照保留；之后允许手动删除，避免留下无法清理的孤儿源。
func TestDeleteAirportUnbindsRuleSource(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()
	a, err := mgr.AddAirport(ctx, "待删机场", "http://example.com/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	src, err := mgr.SaveAirportRuleSource(ctx, a.Name, a.ID, a.SubscriptionURL, []string{"DOMAIN,del.test,DIRECT"}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}
	keep, err := mgr.AddAirport(ctx, "保留机场", "http://example.com/keep", "", true)
	if err != nil {
		t.Fatalf("AddAirport(keep): %v", err)
	}
	if _, err := mgr.SaveAirportRuleSource(ctx, keep.Name, keep.ID, keep.SubscriptionURL, []string{"DOMAIN,keep.test,DIRECT"}, nil); err != nil {
		t.Fatalf("SaveAirportRuleSource(keep): %v", err)
	}
	mustProfilePayload(t, ts, map[string]any{
		"name": "绑定待删源", "rule_sources": []string{src.Name},
	})

	if st, _, body := uiDoJSON(t, ts, "DELETE", fmt.Sprintf("/api/airports/%d", a.ID), ""); st != http.StatusOK {
		t.Fatalf("删除机场状态 = %d；body=%s", st, body)
	}
	profiles, err := mgr.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	for _, prof := range profiles {
		for _, used := range prof.RuleSources {
			if used == src.Name {
				t.Fatalf("删除机场后方案仍绑定源 %q：%+v", src.Name, prof)
			}
		}
	}
	sources, err := mgr.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	missing := false
	for _, s := range sources {
		if s.Name == src.Name {
			missing = s.AirportMissing
		}
	}
	if !missing {
		t.Fatalf("机场删除后规则源未标记 airport_missing：%+v", sources)
	}
	if st, _, body := uiPostJSON(t, ts, "/api/rule-sources/remove", `{"name":"`+src.Name+`"}`); st != http.StatusOK {
		t.Fatalf("删除孤儿机场源状态 = %d；body=%s", st, body)
	}
}
