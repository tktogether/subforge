package store

// internal/store/layers.go 的补测：来源过滤、按来源批量启停/删除、按层拼接与去重、来源枚举。
//
// 纪律：只测行为不测实现；每个测试用独立临时库（newTestDB）。

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// seedLayerFixture 建一个多来源的规则/分组样本库，返回 DB 与 ctx。
//
// 规则：
//
//	custom   c1.com (sort 5, 启用)   c2.com (sort 5, 启用)   c3.com (sort 9, 停用)
//	airport  hk1.com (sort 1, 启用)  jp1.com (sort 2, 启用)
//	manual   m1.com (sort 1, 启用)
//
// 分组：
//
//	custom   dup(1)  only-custom(2)
//	airport  dup(1)  only-airport(3)
//	manual   mgroup(1)
func seedLayerFixture(t *testing.T) (*DB, context.Context) {
	t.Helper()
	d, ctx := newTestDB(t)

	rules := []Rule{
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "c1.com", Enabled: true, SortOrder: 5},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "c2.com", Enabled: true, SortOrder: 5},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "c3.com", Enabled: false, SortOrder: 9},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "hk1.com", Enabled: true, Source: "airport:HK", SortOrder: 1},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "jp1.com", Enabled: true, Source: "airport:JP", SortOrder: 2},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "m1.com", Enabled: true, Source: "manual:x", SortOrder: 1},
	}
	for _, r := range rules {
		if _, err := d.AddRuleWithSource(ctx, r); err != nil {
			t.Fatalf("AddRuleWithSource(%s): %v", r.RuleParam, err)
		}
	}

	groups := []struct {
		name   string
		source string
		order  int
	}{
		{"dup", "", 1},
		{"only-custom", "", 2},
		{"dup", "airport:HK", 1},
		{"only-airport", "airport:HK", 3},
		{"mgroup", "manual:x", 1},
	}
	for _, g := range groups {
		if _, err := d.AddProxyGroupWithSource(ctx, g.name, "select", "", "", 0, g.order, g.source); err != nil {
			t.Fatalf("AddProxyGroupWithSource(%s): %v", g.name, err)
		}
	}
	return d, ctx
}

// ruleParams 把规则列表压成参数列表，便于断言顺序。
func ruleParams(rules []Rule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.RuleParam)
	}
	return out
}

// groupNames 把分组列表压成名字列表，便于断言顺序与去重。
func groupNames(groups []ProxyGroup) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.Name)
	}
	return out
}

func equalStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v，期望 %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %v，期望 %v", what, got, want)
		}
	}
}

// TestListRulesFilteredBySource 覆盖 All / 精确 / 前缀三种过滤与空结果。
func TestListRulesFilteredBySource(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	all, err := d.ListRulesFiltered(ctx, SourceFilter{All: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered(All): %v", err)
	}
	equalStrings(t, "All 过滤", ruleParams(all), []string{"hk1.com", "m1.com", "jp1.com", "c1.com", "c2.com", "c3.com"})

	custom, err := d.ListRulesFiltered(ctx, SourceFilter{Source: ""})
	if err != nil {
		t.Fatalf("ListRulesFiltered(custom): %v", err)
	}
	equalStrings(t, "source='' 过滤", ruleParams(custom), []string{"c1.com", "c2.com", "c3.com"})

	hk, err := d.ListRulesFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil {
		t.Fatalf("ListRulesFiltered(airport:HK): %v", err)
	}
	equalStrings(t, "精确 source 过滤", ruleParams(hk), []string{"hk1.com"})

	prefixed, err := d.ListRulesFiltered(ctx, SourceFilter{Source: "airport:", Prefix: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered(prefix): %v", err)
	}
	equalStrings(t, "前缀过滤", ruleParams(prefixed), []string{"hk1.com", "jp1.com"})

	none, err := d.ListRulesFiltered(ctx, SourceFilter{Source: "nope"})
	if err != nil {
		t.Fatalf("ListRulesFiltered(nope): %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("不存在的来源应返回非 nil 空切片，实际 %#v", none)
	}

	if _, err := d.ListRulesFiltered(ctx, SourceFilter{Prefix: true}); err == nil {
		t.Fatal("前缀过滤缺少来源应报错")
	} else if err.Error() != "前缀过滤需要非空来源" {
		t.Fatalf("前缀过滤错误文案=%q，期望「前缀过滤需要非空来源」", err.Error())
	}
}

// TestListRulesFilteredEscapesLike 覆盖来源里的 % 与 _ 不被当成通配符。
func TestListRulesFilteredEscapesLike(t *testing.T) {
	d, ctx := newTestDB(t)
	for _, src := range []string{"100%done", "100x", "100_done"} {
		if _, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN-SUFFIX", RuleParam: "p-" + src, Enabled: true, Source: src}); err != nil {
			t.Fatalf("AddRuleWithSource(%s): %v", src, err)
		}
	}
	got, err := d.ListRulesFiltered(ctx, SourceFilter{Source: "100%", Prefix: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered(100%%): %v", err)
	}
	equalStrings(t, "含 %% 的前缀过滤", ruleParams(got), []string{"p-100%done"})

	// "_" 也不该是通配符：前缀过滤只命中 "100_done"，不命中 "100x"
	got, err = d.ListRulesFiltered(ctx, SourceFilter{Source: "100_", Prefix: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered(100_ prefix): %v", err)
	}
	equalStrings(t, "含 _ 的前缀过滤", ruleParams(got), []string{"p-100_done"})
}

// TestListProxyGroupsFilteredBySource 覆盖分组的三种过滤。
func TestListProxyGroupsFilteredBySource(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	all, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
	if err != nil {
		t.Fatalf("ListProxyGroupsFiltered(All): %v", err)
	}
	equalStrings(t, "分组 All 过滤", groupNames(all), []string{"dup", "dup", "mgroup", "only-custom", "only-airport"})

	custom, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{Source: ""})
	if err != nil {
		t.Fatalf("ListProxyGroupsFiltered(custom): %v", err)
	}
	equalStrings(t, "分组 source='' 过滤", groupNames(custom), []string{"dup", "only-custom"})

	hk, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{Source: "airport:HK", Prefix: true})
	if err != nil {
		t.Fatalf("ListProxyGroupsFiltered(prefix): %v", err)
	}
	equalStrings(t, "分组前缀过滤", groupNames(hk), []string{"dup", "only-airport"})
}

// TestBatchRulesBySource 覆盖批量启停与批量删除，以及 All 的保护性报错。
func TestBatchRulesBySource(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	// 批量停用机场 HK 的规则（2 条），自定义规则不受影响
	n, err := d.BatchSetRulesEnabledBySource(ctx, SourceFilter{Source: "airport:HK"}, false)
	if err != nil {
		t.Fatalf("BatchSetRulesEnabledBySource: %v", err)
	}
	if n != 1 {
		t.Fatalf("停用 airport:HK 影响 %d 行，期望 1", n)
	}
	hk, err := d.ListRulesFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil {
		t.Fatalf("ListRulesFiltered: %v", err)
	}
	if len(hk) != 1 || hk[0].Enabled {
		t.Fatalf("airport:HK 规则应被停用，实际 %+v", hk)
	}
	custom, err := d.ListRulesFiltered(ctx, SourceFilter{Source: ""})
	if err != nil {
		t.Fatalf("ListRulesFiltered(custom): %v", err)
	}
	for _, r := range custom {
		if r.RuleParam == "c1.com" && !r.Enabled {
			t.Fatal("批量操作不应影响自定义规则")
		}
	}

	// 前缀形式批量启用所有机场规则
	n, err = d.BatchSetRulesEnabledBySource(ctx, SourceFilter{Source: "airport:", Prefix: true}, true)
	if err != nil {
		t.Fatalf("BatchSetRulesEnabledBySource(prefix): %v", err)
	}
	if n != 2 {
		t.Fatalf("前缀启用影响 %d 行，期望 2", n)
	}

	// All 不允许批量操作
	if _, err := d.BatchSetRulesEnabledBySource(ctx, SourceFilter{All: true}, true); err == nil {
		t.Fatal("All 批量启用应报错")
	} else if err.Error() != "必须指定来源才能批量操作" {
		t.Fatalf("All 批量启用错误文案=%q", err.Error())
	}

	// 批量删除 manual 来源
	n, err = d.BatchDeleteRulesBySource(ctx, SourceFilter{Source: "manual:x"})
	if err != nil {
		t.Fatalf("BatchDeleteRulesBySource: %v", err)
	}
	if n != 1 {
		t.Fatalf("删除 manual:x 影响 %d 行，期望 1", n)
	}
	rest, err := d.ListRulesFiltered(ctx, SourceFilter{All: true})
	if err != nil {
		t.Fatalf("ListRulesFiltered(All): %v", err)
	}
	if len(rest) != 5 {
		t.Fatalf("删除后应剩 5 条规则，实际 %d", len(rest))
	}

	// All 不允许批量删除
	if _, err := d.BatchDeleteRulesBySource(ctx, SourceFilter{All: true}); err == nil {
		t.Fatal("All 批量删除应报错")
	} else if err.Error() != "必须指定来源才能批量操作" {
		t.Fatalf("All 批量删除错误文案=%q", err.Error())
	}
}

// TestListAllSources 覆盖来源枚举：DISTINCT、含空串、按来源排序。
func TestListAllSources(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	ruleSources, err := d.ListAllRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListAllRuleSources: %v", err)
	}
	equalStrings(t, "规则来源", ruleSources, []string{"", "airport:HK", "airport:JP", "manual:x"})

	groupSources, err := d.ListAllGroupSources(ctx)
	if err != nil {
		t.Fatalf("ListAllGroupSources: %v", err)
	}
	equalStrings(t, "分组来源", groupSources, []string{"", "airport:HK", "manual:x"})

	// 空库返回非 nil 空切片
	empty, _ := newTestDB(t)
	got, err := empty.ListAllRuleSources(ctx)
	if err != nil {
		t.Fatalf("空库 ListAllRuleSources: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("空库来源应是非 nil 空切片，实际 %#v", got)
	}
}

// TestListRulesByLayersOrder 覆盖按层顺序拼接、层内 sort_order,id、只取已启用、template 不贡献行。
func TestListRulesByLayersOrder(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	// custom 在前
	got, err := d.ListRulesByLayers(ctx, []string{"custom", "airport"})
	if err != nil {
		t.Fatalf("ListRulesByLayers: %v", err)
	}
	equalStrings(t, "custom→airport", ruleParams(got), []string{"c1.com", "c2.com", "hk1.com", "jp1.com"})

	// airport 在前
	got, err = d.ListRulesByLayers(ctx, []string{"airport", "custom"})
	if err != nil {
		t.Fatalf("ListRulesByLayers: %v", err)
	}
	equalStrings(t, "airport→custom", ruleParams(got), []string{"hk1.com", "jp1.com", "c1.com", "c2.com"})

	// 只给 template：DB 不贡献任何行
	got, err = d.ListRulesByLayers(ctx, []string{"template"})
	if err != nil {
		t.Fatalf("ListRulesByLayers(template): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("template 层不应有 DB 行，实际 %v", ruleParams(got))
	}

	// 空层 = 默认三层（custom + airport + template）
	got, err = d.ListRulesByLayers(ctx, nil)
	if err != nil {
		t.Fatalf("ListRulesByLayers(nil): %v", err)
	}
	equalStrings(t, "默认层", ruleParams(got), []string{"c1.com", "c2.com", "hk1.com", "jp1.com"})

	// manual 来源不属于任何层
	got, err = d.ListRulesByLayers(ctx, []string{"custom", "airport", "manual"})
	if err != nil {
		t.Fatalf("ListRulesByLayers(manual): %v", err)
	}
	if strings.Contains(strings.Join(ruleParams(got), ","), "m1.com") {
		t.Fatalf("manual 来源不应出现在层结果里：%v", ruleParams(got))
	}
}

// TestListProxyGroupsByLayersDedup 覆盖分组的层顺序与同名先到者胜。
func TestListProxyGroupsByLayersDedup(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	got, err := d.ListProxyGroupsByLayers(ctx, []string{"custom", "airport"})
	if err != nil {
		t.Fatalf("ListProxyGroupsByLayers: %v", err)
	}
	equalStrings(t, "custom→airport 去重", groupNames(got), []string{"dup", "only-custom", "only-airport"})
	// 先到者胜：保留 custom 层的 dup（ID 更小）
	for _, g := range got {
		if g.Name == "dup" && g.Source != "" {
			t.Fatalf("同名分组应保留先到层（custom）的那条，实际 source=%q", g.Source)
		}
	}

	got, err = d.ListProxyGroupsByLayers(ctx, []string{"airport", "custom"})
	if err != nil {
		t.Fatalf("ListProxyGroupsByLayers(airport→custom): %v", err)
	}
	equalStrings(t, "airport→custom 去重", groupNames(got), []string{"dup", "only-airport", "only-custom"})
	for _, g := range got {
		if g.Name == "dup" && g.Source != "airport:HK" {
			t.Fatalf("airport 在前时应保留机场那条 dup，实际 source=%q", g.Source)
		}
	}

	// 计数不去重：custom 2 + airport 2 = 4
	n, err := d.CountGroupsByLayers(ctx, []string{"custom", "airport"})
	if err != nil {
		t.Fatalf("CountGroupsByLayers: %v", err)
	}
	if n != 4 {
		t.Fatalf("CountGroupsByLayers=%d，期望 4（不去重）", n)
	}
}

// TestCountRulesByLayers 覆盖按层计数：仅 enabled=true、template 不计、manual 不计。
func TestCountRulesByLayers(t *testing.T) {
	d, ctx := seedLayerFixture(t)

	// 默认三层：custom 2 条启用（c3 停用不计）+ airport 2 条启用 = 4
	n, err := d.CountRulesByLayers(ctx, nil)
	if err != nil {
		t.Fatalf("CountRulesByLayers(nil): %v", err)
	}
	if n != 4 {
		t.Fatalf("默认层计数=%d，期望 4", n)
	}

	n, err = d.CountRulesByLayers(ctx, []string{"custom"})
	if err != nil {
		t.Fatalf("CountRulesByLayers(custom): %v", err)
	}
	if n != 2 {
		t.Fatalf("custom 计数=%d，期望 2（停用的 c3 不计）", n)
	}

	n, err = d.CountRulesByLayers(ctx, []string{"template"})
	if err != nil {
		t.Fatalf("CountRulesByLayers(template): %v", err)
	}
	if n != 0 {
		t.Fatalf("template 计数=%d，期望 0", n)
	}

	n, err = d.CountRulesByLayers(ctx, []string{"custom", "custom", "airport"})
	if err != nil {
		t.Fatalf("CountRulesByLayers(重复层): %v", err)
	}
	if n != 4 {
		t.Fatalf("重复层键应只算一次，实际 %d，期望 4", n)
	}

	// 空库计数为 0，不报错
	empty, _ := newTestDB(t)
	n, err = empty.CountRulesByLayers(ctx, nil)
	if err != nil || n != 0 {
		t.Fatalf("空库计数 n=%d err=%v，期望 0/nil", n, err)
	}
	n, err = empty.CountGroupsByLayers(ctx, nil)
	if err != nil || n != 0 {
		t.Fatalf("空库分组计数 n=%d err=%v，期望 0/nil", n, err)
	}
}

// TestSetRuleSource 覆盖「改单条规则的来源」：成功改来自定义/撞唯一索引报固定文案/不存在返回 sql.ErrNoRows。
func TestSetRuleSource(t *testing.T) {
	d, ctx := newTestDB(t)

	// 两条同 origin_key、不同来源的规则：把 B 改到 A 的来源会撞 (source, origin_key) 唯一索引。
	a, err := d.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "a.com", Source: "airport:A", OriginKey: "same-key", Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource(a): %v", err)
	}
	b, err := d.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN-SUFFIX", RuleParam: "b.com", Source: "airport:B", OriginKey: "same-key", Enabled: true,
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource(b): %v", err)
	}

	// ① 撞唯一索引 → 固定文案（不是 sql.ErrNoRows）
	if err := d.SetRuleSource(ctx, b.ID, "airport:A"); !errors.Is(err, errDuplicateOriginInSource) {
		t.Fatalf("SetRuleSource(冲突) err=%v，期望 errDuplicateOriginInSource", err)
	}

	// ② 改成新来源成功，且来源被真实写回
	if err := d.SetRuleSource(ctx, b.ID, "manual:mine"); err != nil {
		t.Fatalf("SetRuleSource(manual:mine): %v", err)
	}
	got, err := d.GetRule(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetRule(b): %v", err)
	}
	if got.Source != "manual:mine" {
		t.Fatalf("来源=%q，期望 manual:mine", got.Source)
	}

	// ③ 改回空串（自定义）也合法
	if err := d.SetRuleSource(ctx, a.ID, ""); err != nil {
		t.Fatalf("SetRuleSource(空串): %v", err)
	}
	got, err = d.GetRule(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetRule(a): %v", err)
	}
	if got.Source != "" {
		t.Fatalf("来源=%q，期望空串（自定义）", got.Source)
	}

	// ④ 不存在 → sql.ErrNoRows
	if err := d.SetRuleSource(ctx, 99999, "x"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetRuleSource(99999) err=%v，期望 sql.ErrNoRows", err)
	}
}
