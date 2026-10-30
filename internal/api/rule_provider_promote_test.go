package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/store"
)

// TestPromoteRuleSetCopiesProvider RULE-SET 规则挑选到自定义层时，必须把 provider
// 定义一起带过去，否则订阅里的 RULE-SET 找不到规则集；同名 provider 定义冲突时
// 默认跳过，overwrite 才覆盖。
func TestPromoteRuleSetCopiesProvider(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()
	providerRaw := map[string]any{
		"type": "http", "behavior": "domain",
		"url": "https://example.com/ads.yaml", "path": "./ads.yaml",
	}
	src, err := mgr.SaveAirportRuleSource(ctx, "P", 1, "http://example.com/sub",
		[]string{"RULE-SET,ads,REJECT", "DOMAIN-SUFFIX,p.test,DIRECT"},
		[]model.RuleProvider{{Name: "ads", Raw: providerRaw}})
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}

	res, body := postPickImport(t, ts, map[string]any{
		"name": src.Name, "rules": []string{"RULE-SET,ads,REJECT"},
	})
	if res.Added != 1 || res.Skipped != 0 {
		t.Fatalf("RULE-SET 应连同 provider 导入：%+v body=%s", res, body)
	}
	providers, err := mgr.DB().ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("ListRuleProviders: %v", err)
	}
	foundCustom := false
	for _, p := range providers {
		if p.Name == "ads" && p.Source == "" && p.Enabled && strings.Contains(p.RawJSON, "example.com/ads.yaml") {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatalf("自定义 provider 未启用/未写入：%+v", providers)
	}

	// 不绑定源也能渲染：自定义规则 + 启用的自定义 provider。
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30001?allowInsecure=1#orig", "NODE-RP", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	mustToken(t, ts, "t-rp", 0)
	st, sub := subscribe(t, ts, "t-rp")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d；body=%s", st, sub)
	}
	if !strings.Contains(sub, "rule-providers:") || !strings.Contains(sub, "ads:") {
		t.Fatalf("订阅缺少 rule-providers：%s", sub)
	}
	if !strings.Contains(sub, "RULE-SET,ads,REJECT") {
		t.Fatalf("订阅缺少 RULE-SET 规则：%s", sub)
	}

	// 同名 provider 定义不同：默认跳过；overwrite 覆盖。
	if err := mgr.DB().UpsertRuleProvider(ctx, store.RuleProvider{
		Name: "ads", RawJSON: `{"type":"http","behavior":"domain","url":"https://other.example/ads.yaml","path":"./ads.yaml"}`,
		Source: "", Enabled: true,
	}); err != nil {
		t.Fatalf("预置冲突 provider: %v", err)
	}
	res, body = postPickImport(t, ts, map[string]any{
		"name": src.Name, "rules": []string{"RULE-SET,ads,REJECT"},
	})
	if res.Added != 0 || res.Skipped != 1 || len(res.Warnings) == 0 {
		t.Fatalf("provider 冲突应跳过并告警：%+v body=%s", res, body)
	}
	res, body = postPickImport(t, ts, map[string]any{
		"name": src.Name, "rules": []string{"RULE-SET,ads,REJECT"}, "conflict": "overwrite",
	})
	if res.Added != 0 || res.Overwrote != 1 {
		t.Fatalf("overwrite 应覆盖规则：%+v body=%s", res, body)
	}
	providers, _ = mgr.DB().ListRuleProviders(ctx)
	foundCustom = false
	for _, p := range providers {
		if p.Name == "ads" && p.Source == "" && strings.Contains(p.RawJSON, "example.com/ads.yaml") {
			foundCustom = true
		}
	}
	if !foundCustom {
		t.Fatalf("overwrite 后 provider 未恢复为源定义：%+v", providers)
	}
}
