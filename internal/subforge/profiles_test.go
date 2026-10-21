package subforge

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// canceledCtx 返回一个已取消的 context，用来触发各读写路径的通用错误分支。
func canceledCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// --- 方案（Profile）基础读写：保持 v1 的 6 个包装语义 ---

func TestManagerListProfiles(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	profiles, err := m.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if profiles == nil {
		t.Fatal("ListProfiles 返回 nil 切片")
	}
	if len(profiles) != 0 {
		t.Fatalf("空库应返回 0 条方案, got %d", len(profiles))
	}

	// sort_order 回退时间戳后按 id 升序
	p1, err := m.AddProfile(ctx, "方案 A", "描述", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	p2, err := m.AddProfile(ctx, "方案 B", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}
	if _, err := m.AddProfile(ctx, "方案 C", "", "", "", false, 0); err != nil {
		t.Fatalf("AddProfile C: %v", err)
	}

	profiles, err = m.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("应返回 3 条方案, got %d", len(profiles))
	}
	// sort_order 显式给 1 的方案 B 排在回退时间戳的方案 A 之前
	if profiles[0].ID != p2.ID {
		t.Fatalf("sort_order=1 的方案 B 应排最前, 实际首条 ID=%d (期望 %d)", profiles[0].ID, p2.ID)
	}
	if profiles[1].ID != p1.ID {
		t.Fatalf("第二条应是方案 A, 实际 ID=%d (期望 %d)", profiles[1].ID, p1.ID)
	}
	for i := 1; i < len(profiles); i++ {
		if profiles[i-1].SortOrder > profiles[i].SortOrder ||
			(profiles[i-1].SortOrder == profiles[i].SortOrder && profiles[i-1].ID > profiles[i].ID) {
			t.Fatalf("ListProfiles 顺序不符: [%d]=%d/%d [%d]=%d/%d",
				i-1, profiles[i-1].SortOrder, profiles[i-1].ID, i, profiles[i].SortOrder, profiles[i].ID)
		}
	}
	for _, p := range profiles {
		if p.RuleCount != 0 || p.GroupCount != 0 {
			t.Fatalf("空库方案的计数应为 0, 实际 rule=%d group=%d", p.RuleCount, p.GroupCount)
		}
	}

	if _, err := m.ListProfiles(canceledCtx(t)); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerGetProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	p, err := m.AddProfile(ctx, "我的方案", "desc", "clash", "proxy", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Name != "我的方案" || got.Description != "desc" || got.Config != "clash" ||
		got.Pipeline != "proxy" || !got.Enabled {
		t.Fatalf("GetProfile 返回字段不符: %+v", got)
	}

	_, err = m.GetProfile(ctx, 999999)
	if err == nil || !strings.Contains(err.Error(), "订阅方案 999999 不存在") {
		t.Fatalf("GetProfile 不存在应报「订阅方案 999999 不存在」, 实际: %v", err)
	}

	if _, err := m.GetProfile(canceledCtx(t), p.ID); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerProfileCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	p, err := m.AddProfile(ctx, "方案名", "描述", "mihomo", "direct", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	err = m.UpdateProfile(ctx, p.ID, "新名", "新描述", "clash", "proxy", false, 5)
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Name != "新名" || got.Description != "新描述" || got.Config != "clash" ||
		got.Pipeline != "proxy" || got.Enabled || got.SortOrder != 5 {
		t.Fatalf("UpdateProfile 未生效: %+v", got)
	}

	if err := m.UpdateProfile(ctx, 999999, "x", "", "", "", true, 0); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateProfile 不存在应返回 sql.ErrNoRows, 实际: %v", err)
	}

	if err := m.DeleteProfile(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if _, err := m.GetProfile(ctx, p.ID); err == nil {
		t.Fatal("删除后 GetProfile 应报错")
	}
	if err := m.DeleteProfile(ctx, 999999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteProfile 不存在应返回 sql.ErrNoRows, 实际: %v", err)
	}
	if err := m.UpdateProfile(canceledCtx(t), p.ID, "x", "", "", "", true, 0); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerCreateSubTokenWithProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	p, err := m.AddProfile(ctx, "绑定方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	token := "tok-profile-1"
	if err := m.CreateSubTokenWithProfile(ctx, token, "令牌名", "https://api/sub", "proxy", "clash", "", "", p.ID); err != nil {
		t.Fatalf("CreateSubTokenWithProfile: %v", err)
	}
	got, err := m.GetSubToken(ctx, token)
	if err != nil {
		t.Fatalf("GetSubToken: %v", err)
	}
	if got.ProfileID != p.ID {
		t.Fatalf("令牌应绑定方案 %d, 实际 %d", p.ID, got.ProfileID)
	}
	if got.Name != "令牌名" || got.Pipeline != "proxy" || got.Config != "clash" {
		t.Fatalf("令牌字段不符: %+v", got)
	}
}

// --- 方案层（rule_layers / group_layers）---

func TestManagerSetProfileLayers(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	p, err := m.AddProfile(ctx, "层方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	if err := m.SetProfileLayers(ctx, p.ID, []string{"airport", "custom"}, []string{"custom"}); err != nil {
		t.Fatalf("SetProfileLayers: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.RuleLayers) != 2 || got.RuleLayers[0] != "airport" || got.RuleLayers[1] != "custom" {
		t.Fatalf("rule_layers 未落库: %+v", got.RuleLayers)
	}
	if len(got.GroupLayers) != 1 || got.GroupLayers[0] != "custom" {
		t.Fatalf("group_layers 未落库: %+v", got.GroupLayers)
	}

	// 空切片 = 默认三层
	if err := m.SetProfileLayers(ctx, p.ID, nil, []string{}); err != nil {
		t.Fatalf("SetProfileLayers 空: %v", err)
	}
	got, err = m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.RuleLayers) != 0 || len(got.GroupLayers) != 0 {
		t.Fatalf("空数组应落库为默认（读回为空切片）, 实际 rule=%+v group=%+v", got.RuleLayers, got.GroupLayers)
	}

	if err := m.SetProfileLayers(ctx, 999999, []string{"custom"}, nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProfileLayers 不存在应返回 sql.ErrNoRows, 实际: %v", err)
	}
	if err := m.SetProfileLayers(canceledCtx(t), p.ID, []string{"custom"}, nil); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerLayerCountsAndListing(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	customRule, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	airRule, err := m.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "air.example", Enabled: true, Source: "airport:HK",
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource: %v", err)
	}
	// 停用规则不计入统计
	if _, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "disabled.example", 0, false, false); err != nil {
		t.Fatalf("AddRule 停用: %v", err)
	}
	if _, err := m.AddProxyGroup(ctx, "custom-g", "select", ".", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if _, err := m.AddProxyGroupWithSource(ctx, "air-g", "select", ".", "", 0, 0, "airport:HK"); err != nil {
		t.Fatalf("AddProxyGroupWithSource: %v", err)
	}

	// 默认三层（custom + airport + template）→ 启用规则 2 条、分组 2 个
	p, err := m.AddProfile(ctx, "默认方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.RuleCount != 2 {
		t.Fatalf("默认方案 RuleCount 应为 2（含停用不计）, 实际 %d", got.RuleCount)
	}
	if got.GroupCount != 2 {
		t.Fatalf("默认方案 GroupCount 应为 2, 实际 %d", got.GroupCount)
	}

	// 只开 custom 层
	customOnly, err := m.AddProfile(ctx, "仅自定义", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := m.SetProfileLayers(ctx, customOnly.ID, []string{"custom"}, []string{"custom"}); err != nil {
		t.Fatalf("SetProfileLayers: %v", err)
	}
	got, err = m.GetProfile(ctx, customOnly.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.RuleCount != 1 || got.GroupCount != 1 {
		t.Fatalf("仅自定义方案计数应为 1/1, 实际 rule=%d group=%d", got.RuleCount, got.GroupCount)
	}

	// 层顺序拼接：airport 在前
	rules, err := m.ListRulesByLayers(ctx, []string{"airport", "custom"})
	if err != nil {
		t.Fatalf("ListRulesByLayers: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("层拼接应返回 2 条启用规则, got %d", len(rules))
	}
	if rules[0].Source != "airport:HK" || rules[1].Source != "" {
		t.Fatalf("层顺序拼接顺序不符: %+v", []Rule{rules[0], rules[1]})
	}
	if rules[0].ID != airRule.ID || rules[1].ID != customRule.ID {
		t.Fatalf("层拼接返回了错误的规则 ID: got %d/%d 期望 %d/%d",
			rules[0].ID, rules[1].ID, airRule.ID, customRule.ID)
	}
	// 停用规则被排除
	for _, r := range rules {
		if r.RuleParam == "disabled.example" {
			t.Fatal("停用规则不应出现在层拼接结果里")
		}
	}

	groups, err := m.ListProxyGroupsByLayers(ctx, []string{"custom", "airport"})
	if err != nil {
		t.Fatalf("ListProxyGroupsByLayers: %v", err)
	}
	if len(groups) != 2 || groups[0].Name != "custom-g" || groups[1].Name != "air-g" {
		t.Fatalf("分组层拼接不符: %+v", groups)
	}

	// 计数函数与 List* 一致；template 层不贡献任何行
	if n, err := m.CountRulesByLayers(ctx, []string{"airport", "custom"}); err != nil || n != 2 {
		t.Fatalf("CountRulesByLayers 应为 2, got %d err=%v", n, err)
	}
	if n, err := m.CountRulesByLayers(ctx, []string{"template"}); err != nil || n != 0 {
		t.Fatalf("template 层计数应为 0, got %d err=%v", n, err)
	}
	if n, err := m.CountGroupsByLayers(ctx, []string{"custom", "airport"}); err != nil || n != 2 {
		t.Fatalf("CountGroupsByLayers 应为 2, got %d err=%v", n, err)
	}
	if _, err := m.ListRulesByLayers(canceledCtx(t), []string{"custom"}); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerLayerGroupDedupe(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	custom, err := m.AddProxyGroup(ctx, "同名分组", "select", ".", "", 0, 1)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	air, err := m.AddProxyGroupWithSource(ctx, "同名分组", "select", ".", "", 0, 0, "airport:HK")
	if err != nil {
		t.Fatalf("AddProxyGroupWithSource: %v", err)
	}

	// 先到者胜：custom 在前 → 取 custom 那条
	groups, err := m.ListProxyGroupsByLayers(ctx, []string{"custom", "airport"})
	if err != nil {
		t.Fatalf("ListProxyGroupsByLayers: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("同名分组应去重为 1 条, got %d", len(groups))
	}
	if groups[0].ID != custom.ID {
		t.Fatalf("custom 层应优先, 实际拿到 ID=%d (custom=%d air=%d)", groups[0].ID, custom.ID, air.ID)
	}
	// 反过来：airport 在前 → 取 airport 那条
	groups, err = m.ListProxyGroupsByLayers(ctx, []string{"airport", "custom"})
	if err != nil {
		t.Fatalf("ListProxyGroupsByLayers: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != air.ID {
		t.Fatalf("airport 层应优先, 实际拿到 %+v", groups)
	}
	// 计数按 DB 行数（不去重）
	if n, err := m.CountGroupsByLayers(ctx, []string{"custom", "airport"}); err != nil || n != 2 {
		t.Fatalf("CountGroupsByLayers 按 DB 行数应为 2, got %d err=%v", n, err)
	}
}

// --- 来源过滤 ---

func TestManagerSourceFilterRules(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	custom, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	air, err := m.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "air.example", Enabled: true, Source: "airport:HK",
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource: %v", err)
	}

	// 零值 = 只看自定义（source = ''）
	rules, err := m.ListRulesFiltered(ctx, SourceFilter{})
	if err != nil {
		t.Fatalf("ListRulesFiltered 零值: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != custom.ID {
		t.Fatalf("零值过滤应只返回自定义规则, got %+v", rules)
	}
	// 精确匹配来源
	rules, err = m.ListRulesFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil {
		t.Fatalf("ListRulesFiltered 精确: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != air.ID {
		t.Fatalf("精确过滤应只返回 airport:HK 规则, got %+v", rules)
	}
	// 前缀匹配
	rules, err = m.ListRulesFiltered(ctx, SourceFilter{Source: "airport:", Prefix: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered 前缀: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != air.ID {
		t.Fatalf("前缀过滤应只返回 airport:* 规则, got %+v", rules)
	}
	// 前缀必须给非空来源
	if _, err := m.ListRulesFiltered(ctx, SourceFilter{Prefix: true}); err == nil ||
		!strings.Contains(err.Error(), "前缀过滤需要非空来源") {
		t.Fatalf("空前缀应报错「前缀过滤需要非空来源」, 实际: %v", err)
	}
	// All = 不过滤
	rules, err = m.ListRulesFiltered(ctx, SourceFilter{All: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered All: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("All 应返回全部来源, got %d", len(rules))
	}
	if _, err := m.ListRulesFiltered(canceledCtx(t), SourceFilter{}); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerSourceFilterGroups(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	custom, err := m.AddProxyGroup(ctx, "custom-g", "select", ".", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	air, err := m.AddProxyGroupWithSource(ctx, "air-g", "select", ".", "", 0, 0, "airport:HK")
	if err != nil {
		t.Fatalf("AddProxyGroupWithSource: %v", err)
	}

	groups, err := m.ListProxyGroupsFiltered(ctx, SourceFilter{})
	if err != nil {
		t.Fatalf("ListProxyGroupsFiltered 零值: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != custom.ID {
		t.Fatalf("零值过滤应只返回自定义分组, got %+v", groups)
	}
	groups, err = m.ListProxyGroupsFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil || len(groups) != 1 || groups[0].ID != air.ID {
		t.Fatalf("精确过滤应只返回 airport:HK 分组, got %+v err=%v", groups, err)
	}
	groups, err = m.ListProxyGroupsFiltered(ctx, SourceFilter{Source: "airport:", Prefix: true})
	if err != nil || len(groups) != 1 || groups[0].ID != air.ID {
		t.Fatalf("前缀过滤应只返回 airport 分组, got %+v err=%v", groups, err)
	}
	if _, err := m.ListProxyGroupsFiltered(ctx, SourceFilter{Prefix: true}); err == nil {
		t.Fatal("空前缀应报错")
	}
	groups, err = m.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
	if err != nil || len(groups) != 2 {
		t.Fatalf("All 应返回全部分组, got %d err=%v", len(groups), err)
	}
	if _, err := m.ListProxyGroupsFiltered(canceledCtx(t), SourceFilter{}); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

// --- 按来源批量操作 ---

func TestManagerBatchRulesBySource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	for _, param := range []string{"a.example", "b.example"} {
		if _, err := m.AddRuleWithSource(ctx, Rule{
			RuleType: "DOMAIN-SUFFIX", RuleParam: param, Enabled: true, Source: "airport:HK",
		}); err != nil {
			t.Fatalf("AddRuleWithSource: %v", err)
		}
	}
	if _, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "custom.example", 0, false, true); err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	n, err := m.BatchSetRulesEnabledBySource(ctx, SourceFilter{Source: "airport:HK"}, false)
	if err != nil || n != 2 {
		t.Fatalf("BatchSetRulesEnabledBySource 应更新 2 行, got %d err=%v", n, err)
	}
	disabled, err := m.ListRulesFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil || len(disabled) != 2 {
		t.Fatalf("按来源过滤应返回 2 条, got %d err=%v", len(disabled), err)
	}
	for _, r := range disabled {
		if r.Enabled {
			t.Fatal("批量停用未生效")
		}
	}
	// 自定义来源不受影响
	custom, err := m.ListRulesFiltered(ctx, SourceFilter{})
	if err != nil || len(custom) != 1 || !custom[0].Enabled {
		t.Fatalf("自定义规则不应被影响, got %+v err=%v", custom, err)
	}
	// 重新启用
	n, err = m.BatchSetRulesEnabledBySource(ctx, SourceFilter{Source: "airport:HK"}, true)
	if err != nil || n != 2 {
		t.Fatalf("重新启用应更新 2 行, got %d err=%v", n, err)
	}
	// All 拒绝
	if _, err := m.BatchSetRulesEnabledBySource(ctx, SourceFilter{All: true}, false); err == nil ||
		!strings.Contains(err.Error(), "必须指定来源才能批量操作") {
		t.Fatalf("All=true 应报错「必须指定来源才能批量操作」, 实际: %v", err)
	}
	// 按来源删除
	n, err = m.BatchDeleteRulesBySource(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil || n != 2 {
		t.Fatalf("BatchDeleteRulesBySource 应删除 2 行, got %d err=%v", n, err)
	}
	left, err := m.ListRulesFiltered(ctx, SourceFilter{All: true})
	if err != nil || len(left) != 1 || left[0].RuleParam != "custom.example" {
		t.Fatalf("删除后应只剩自定义规则, got %+v err=%v", left, err)
	}
	if _, err := m.BatchDeleteRulesBySource(ctx, SourceFilter{All: true}); err == nil {
		t.Fatal("All=true 删除应报错")
	}
	if _, err := m.BatchSetRulesEnabledBySource(canceledCtx(t), SourceFilter{Source: "airport:HK"}, false); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

// --- 来源清单与带来源新增 ---

func TestManagerAllSources(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if got, err := m.ListAllRuleSources(ctx); err != nil || len(got) != 0 {
		t.Fatalf("空库来源清单应为空, got %+v err=%v", got, err)
	}
	if _, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", 0, false, true); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if _, err := m.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "air.example", Enabled: true, Source: "airport:HK",
	}); err != nil {
		t.Fatalf("AddRuleWithSource: %v", err)
	}
	sources, err := m.ListAllRuleSources(ctx)
	if err != nil || len(sources) != 2 || sources[0] != "" || sources[1] != "airport:HK" {
		t.Fatalf("来源清单应为 空串 与 airport:HK, got %+v err=%v", sources, err)
	}

	if got, err := m.ListAllGroupSources(ctx); err != nil || len(got) != 0 {
		t.Fatalf("空库分组来源清单应为空, got %+v err=%v", got, err)
	}
	if _, err := m.AddProxyGroup(ctx, "g1", "select", ".", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if _, err := m.AddProxyGroupWithSource(ctx, "g2", "select", ".", "", 0, 0, "airport:HK"); err != nil {
		t.Fatalf("AddProxyGroupWithSource: %v", err)
	}
	gsources, err := m.ListAllGroupSources(ctx)
	if err != nil || len(gsources) != 2 || gsources[0] != "" || gsources[1] != "airport:HK" {
		t.Fatalf("分组来源清单应为 空串 与 airport:HK, got %+v err=%v", gsources, err)
	}
	if _, err := m.ListAllRuleSources(canceledCtx(t)); err == nil {
		t.Fatal("canceled context 应返回错误")
	}
}

func TestManagerAddRuleAndGroupWithSource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	// AddRule = 自定义来源（source 为空串）
	custom, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "custom.example", 0, true, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if custom.Source != "" {
		t.Fatalf("AddRule 的 source 应为空串, 实际 %q", custom.Source)
	}
	got, err := m.GetRule(ctx, custom.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.RuleType != "DOMAIN-SUFFIX" || got.RuleParam != "custom.example" ||
		!got.ResolveDNS || !got.Enabled || got.Source != "" {
		t.Fatalf("GetRule 字段不符: %+v", got)
	}
	if _, err := m.GetRule(ctx, 999999); err == nil {
		t.Fatal("GetRule 不存在应报错")
	}

	// AddRuleWithSource 保留来源与原始行
	air, err := m.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "air.example", Raw: "DOMAIN-SUFFIX,air.example",
		OriginKey: "airport:HK|DOMAIN-SUFFIX|air.example", Source: "airport:HK", Enabled: true, SortOrder: 7,
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource: %v", err)
	}
	if air.Source != "airport:HK" || air.Raw != "DOMAIN-SUFFIX,air.example" ||
		air.OriginKey != "airport:HK|DOMAIN-SUFFIX|air.example" || air.SortOrder != 7 {
		t.Fatalf("AddRuleWithSource 字段不符: %+v", air)
	}
	// 同来源同 origin_key 去重
	if _, err := m.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "dup.example", OriginKey: air.OriginKey, Source: "airport:HK",
	}); err == nil || !strings.Contains(err.Error(), "目标来源里已存在同来源同去重键的规则") {
		t.Fatalf("重复 origin_key 应报错固定文案, 实际: %v", err)
	}
	// 不同来源同 origin_key 允许
	if _, err := m.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "other.example", OriginKey: air.OriginKey, Source: "airport:JP",
	}); err != nil {
		t.Fatalf("不同来源同 origin_key 应允许: %v", err)
	}

	// AddProxyGroupWithSource 保留来源
	g, err := m.AddProxyGroupWithSource(ctx, "air-group", "select", ".", "", 0, 0, "airport:HK")
	if err != nil {
		t.Fatalf("AddProxyGroupWithSource: %v", err)
	}
	if g.Source != "airport:HK" || g.Name != "air-group" || g.Type != "select" || g.Regex != "." {
		t.Fatalf("AddProxyGroupWithSource 字段不符: %+v", g)
	}
	byID, err := m.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if byID.Source != "airport:HK" {
		t.Fatalf("GetProxyGroup source 不符: %+v", byID)
	}
	if _, err := m.GetProxyGroup(ctx, 999999); err == nil {
		t.Fatal("GetProxyGroup 不存在应报错")
	}
}
