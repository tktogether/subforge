package store

// 规则源生命周期相关列的补测：方案输出定制、来源计数与挑选覆盖。

import (
	"database/sql"
	"errors"
	"testing"
)

// TestSetProfileRenderOptions 覆盖 target/include/exclude 的按列更新与不存在方案。
func TestSetProfileRenderOptions(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "渲染选项", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := d.SetProfileRenderOptions(ctx, p.ID, nil, nil, nil); err != nil {
		t.Fatalf("全 nil 应是无操作: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Target != "" || got.Include != "" || got.Exclude != "" {
		t.Fatalf("全 nil 不应改列: %+v", got)
	}

	target, include := " clash.meta ", "香港"
	if err := d.SetProfileRenderOptions(ctx, p.ID, &target, &include, nil); err != nil {
		t.Fatalf("SetProfileRenderOptions: %v", err)
	}
	got, _ = d.GetProfile(ctx, p.ID)
	if got.Target != "clash.meta" || got.Include != "香港" || got.Exclude != "" {
		t.Fatalf("部分更新不符: %+v", got)
	}

	exclude := "过期"
	if err := d.SetProfileRenderOptions(ctx, p.ID, nil, nil, &exclude); err != nil {
		t.Fatalf("SetProfileRenderOptions(exclude): %v", err)
	}
	got, _ = d.GetProfile(ctx, p.ID)
	if got.Target != "clash.meta" || got.Exclude != "过期" {
		t.Fatalf("未给出的列应保持原值: %+v", got)
	}

	if err := d.SetProfileRenderOptions(ctx, 9999, &target, nil, nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("不存在方案应返回 ErrNoRows, got %v", err)
	}
}

// TestListRuleSourceCounts 覆盖按来源聚合与排序。
func TestListRuleSourceCounts(t *testing.T) {
	d, ctx := newTestDB(t)
	for _, r := range []Rule{
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "a.com", Enabled: true, Source: ""},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "b.com", Enabled: true, Source: ""},
		{RuleType: "DOMAIN-SUFFIX", RuleParam: "c.com", Enabled: true, Source: "airport:守候"},
	} {
		if _, err := d.AddRuleWithSource(ctx, r); err != nil {
			t.Fatalf("AddRuleWithSource(%s): %v", r.Source, err)
		}
	}
	got, err := d.ListRuleSourceCounts(ctx)
	if err != nil {
		t.Fatalf("ListRuleSourceCounts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("来源数 = %d, want 2: %+v", len(got), got)
	}
	want := map[string]int64{"": 2, "airport:守候": 1}
	for _, item := range got {
		if want[item.Source] != item.Count {
			t.Fatalf("来源 %q 计数 = %d, want %d", item.Source, item.Count, want[item.Source])
		}
	}
}

// TestUpdateRuleFromSource 覆盖自定义规则的整行覆盖与非自定义规则的保护。
func TestUpdateRuleFromSource(t *testing.T) {
	d, ctx := newTestDB(t)
	custom, err := d.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN", RuleParam: "old.com", RuleConfig: 3, Enabled: false,
		Source: "", Raw: "DOMAIN,old.com,旧组", OriginKey: "old",
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource: %v", err)
	}
	if err := d.UpdateRuleFromSource(ctx, custom.ID, "DOMAIN-SUFFIX", "new.com", 0, true, "DOMAIN-SUFFIX,new.com,DIRECT", "new"); err != nil {
		t.Fatalf("UpdateRuleFromSource: %v", err)
	}
	got, err := d.GetRule(ctx, custom.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.RuleType != "DOMAIN-SUFFIX" || got.RuleParam != "new.com" || got.RuleConfig != 0 || !got.ResolveDNS || !got.Enabled {
		t.Fatalf("覆盖后字段不符: %+v", got)
	}
	if got.Raw != "DOMAIN-SUFFIX,new.com,DIRECT" || got.OriginKey != "new" {
		t.Fatalf("raw/origin_key 未刷新: %+v", got)
	}

	airport, err := d.AddRuleWithSource(ctx, Rule{
		RuleType: "DOMAIN", RuleParam: "airport.com", Enabled: true, Source: "airport:守候",
	})
	if err != nil {
		t.Fatalf("AddRuleWithSource(airport): %v", err)
	}
	if err := d.UpdateRuleFromSource(ctx, airport.ID, "DOMAIN", "x.com", 0, false, "", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("非自定义规则应返回 ErrNoRows, got %v", err)
	}
	if err := d.UpdateRuleFromSource(ctx, 9999, "DOMAIN", "x.com", 0, false, "", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("不存在规则应返回 ErrNoRows, got %v", err)
	}
}
