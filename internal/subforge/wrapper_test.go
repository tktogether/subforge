package subforge

// Manager 包装层的补测：方案渲染/来源、来源计数、带成员分组与 include_self。

import (
	"context"
	"strings"
	"testing"
)

func TestProfileWrapperOptions(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	p, err := m.AddProfile(ctx, "包装", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	target, include := "clash.meta", "港"
	if err := m.SetProfileRenderOptions(ctx, p.ID, &target, &include, nil); err != nil {
		t.Fatalf("SetProfileRenderOptions: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Target != "clash.meta" || got.Include != "港" {
		t.Fatalf("渲染选项未保存: %+v", got)
	}
	if err := m.SetProfileRenderOptions(ctx, 9999, &target, nil, nil); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("不存在方案应报错, got %v", err)
	}

	airports := []int64{2, 1}
	selfNodes := []int64{3}
	if err := m.SetProfileSources(ctx, p.ID, &airports, &selfNodes); err != nil {
		t.Fatalf("SetProfileSources: %v", err)
	}
	got, _ = m.GetProfile(ctx, p.ID)
	if len(got.AirportIDs) != 2 || len(got.SelfNodeIDs) != 1 || got.SelfNodeIDs[0] != 3 {
		t.Fatalf("来源未保存: %+v", got)
	}
	if err := m.SetProfileSources(ctx, 9999, &airports, nil); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("不存在方案应报错, got %v", err)
	}

	if _, err := m.DB().AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN", RuleParam: "a.com", Enabled: true}); err != nil {
		t.Fatalf("AddRuleWithSource: %v", err)
	}
	if _, err := m.DB().AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN", RuleParam: "b.com", Enabled: true, Source: "airport:守候"}); err != nil {
		t.Fatalf("AddRuleWithSource(airport): %v", err)
	}
	counts, err := m.ListRuleSourceCounts(ctx)
	if err != nil {
		t.Fatalf("ListRuleSourceCounts: %v", err)
	}
	if len(counts) != 2 {
		t.Fatalf("来源计数不符: %+v", counts)
	}
}

func TestProxyGroupWrappers(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	g, err := m.AddProxyGroupWithMembersAndFlags(ctx, "香港", "select", "", "", 0, 0, "airport:守候", []string{"HK-01", "HK-02"}, true, true)
	if err != nil {
		t.Fatalf("AddProxyGroupWithMembersAndFlags: %v", err)
	}
	if g.Name != "香港" || len(g.Members) != 2 || g.Source != "airport:守候" || !g.IncludeSelf || !g.IncludeAllSources {
		t.Fatalf("分组字段不符: %+v", g)
	}
	// 子集组：两个开关都关。
	if _, err := m.AddProxyGroupWithMembersAndIncludeSelf(ctx, "香港子集", "select", "", "", 0, 0, "airport:守候", []string{"HK-01"}, false); err != nil {
		t.Fatalf("AddProxyGroupWithMembersAndIncludeSelf: %v", err)
	}
	if err := m.SetProxyGroupIncludeSelf(ctx, g.ID, false); err != nil {
		t.Fatalf("SetProxyGroupIncludeSelf: %v", err)
	}
	if err := m.SetProxyGroupIncludeAllSources(ctx, g.ID, false); err != nil {
		t.Fatalf("SetProxyGroupIncludeAllSources: %v", err)
	}
	got, err := m.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if got.IncludeSelf || got.IncludeAllSources {
		t.Fatalf("扩展标志未关闭: %+v", got)
	}
	if err := m.SetProxyGroupIncludeSelf(ctx, 9999, true); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("不存在分组应报错, got %v", err)
	}
	if err := m.SetProxyGroupIncludeAllSources(ctx, 9999, true); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("不存在分组应报错, got %v", err)
	}
}
