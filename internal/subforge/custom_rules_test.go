package subforge

// 从规则源挑选规则到自定义层：冲突、覆盖与 RULE-SET provider 复制。

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
)

func TestImportRulesToCustom(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "rules")
	writeRuleManifest(t, m, "https://example.test/remote", "clash-meta", outDir)
	airport, err := m.AddAirport(ctx, "守候", "https://up.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	src, err := m.SaveAirportRuleSource(ctx, airport.Name, airport.ID, airport.SubscriptionURL, []string{
		"DOMAIN-SUFFIX,a.com,DIRECT",
		"RULE-SET,p1,机场组",
		"DOMAIN-SUFFIX,b.com,机场组",
	}, []model.RuleProvider{{Name: "p1", Raw: map[string]any{
		"type": "http", "url": "https://rules.example/p1.yaml",
	}}})
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}
	group, err := m.AddProxyGroup(ctx, "机场组", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	res, err := m.ImportRulesToCustom(ctx, src.Name, []string{
		"DOMAIN-SUFFIX,a.com,DIRECT",
		"RULE-SET,p1,机场组",
		"DOMAIN-SUFFIX,b.com,机场组",
		"DOMAIN-SUFFIX,a.com,机场组", // 同批重复
		"RULE-SET,missing,机场组",    // 源里没有的 provider
		"# 注释",
		"",
	}, false)
	if err != nil {
		t.Fatalf("ImportRulesToCustom: %v", err)
	}
	if res.Added != 3 || res.Skipped != 2 || res.Overwrote != 0 {
		t.Fatalf("导入统计不符: %+v", res)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "missing") {
		t.Fatalf("应提示缺失的规则集: %v", res.Warnings)
	}
	providers, err := m.DB().ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("ListRuleProviders: %v", err)
	}
	found := false
	for _, p := range providers {
		if p.Name == "p1" && p.Source == "" && p.Enabled {
			found = true
		}
	}
	if !found {
		t.Fatalf("RULE-SET 的 provider 未复制到自定义层: %+v", providers)
	}

	custom, err := m.ListRulesFiltered(ctx, SourceFilter{Source: ""})
	if err != nil {
		t.Fatalf("ListRulesFiltered: %v", err)
	}
	if len(custom) != 3 {
		t.Fatalf("自定义规则数 = %d, want 3: %+v", len(custom), custom)
	}
	var target Rule
	for _, r := range custom {
		if r.RuleParam == "b.com" {
			target = r
		}
	}
	if target.ID == 0 || target.RuleConfig != group.ID {
		t.Fatalf("目标分组未解析: %+v", target)
	}

	// 已存在的同匹配规则默认跳过；overwrite=true 时整行覆盖（目标从分组变 DIRECT）。
	res, err = m.ImportRulesToCustom(ctx, src.Name, []string{"DOMAIN-SUFFIX,b.com,DIRECT"}, false)
	if err != nil {
		t.Fatalf("ImportRulesToCustom(no overwrite): %v", err)
	}
	if res.Skipped != 1 || res.Overwrote != 0 {
		t.Fatalf("默认应跳过冲突: %+v", res)
	}
	res, err = m.ImportRulesToCustom(ctx, src.Name, []string{"DOMAIN-SUFFIX,b.com,DIRECT"}, true)
	if err != nil {
		t.Fatalf("ImportRulesToCustom(overwrite): %v", err)
	}
	if res.Overwrote != 1 || res.Added != 0 {
		t.Fatalf("overwrite 应覆盖而非新增: %+v", res)
	}
	got, err := m.GetRule(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.RuleConfig != 0 || got.Raw != "DOMAIN-SUFFIX,b.com,DIRECT" {
		t.Fatalf("覆盖后规则未刷新: %+v", got)
	}
}
