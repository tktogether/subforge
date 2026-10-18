package subforge

import (
	"context"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
)

// TestNormalizeRuleOriginKey 覆盖空白/逗号规范化：等价写法得到同一 key，不同行不同 key。
func TestNormalizeRuleOriginKey(t *testing.T) {
	want := "DOMAIN-SUFFIX,example.com,PROXY"
	same := []string{
		"DOMAIN-SUFFIX,example.com,PROXY",
		"  DOMAIN-SUFFIX,example.com,PROXY  ",
		"DOMAIN-SUFFIX , example.com , PROXY",
		"DOMAIN-SUFFIX,   example.com,   PROXY",
		"DOMAIN-SUFFIX,	example.com,	PROXY",
		"DOMAIN-SUFFIX, example.com, PROXY",
	}
	for _, in := range same {
		if got := NormalizeRuleOriginKey(in); got != want {
			t.Errorf("NormalizeRuleOriginKey(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NormalizeRuleOriginKey("DOMAIN,other.com,PROXY"); got == want {
		t.Errorf("不同行不应得到同一 key: %q", got)
	}
}

// TestImportRulesCandidateQueue 覆盖导入候选队列：注释行跳过、分组名映射、raw 保留、enabled=0。
func TestImportRulesCandidateQueue(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	g, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	rules := []string{
		"DOMAIN-SUFFIX,example.com,PROXY",
		"DOMAIN,missing.com,NOPE",
		"# 上游注释行",
	}
	res, err := m.ImportRules(ctx, "airport:HK", rules, nil)
	if err != nil {
		t.Fatalf("ImportRules: %v", err)
	}
	if res.RulesAdded != 2 || res.RulesSkipped != 0 {
		t.Fatalf("added/skipped = %d/%d, want 2/0", res.RulesAdded, res.RulesSkipped)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want 1 条", res.Warnings)
	}
	if !strings.Contains(res.Warnings[0], "NOPE") || !strings.Contains(res.Warnings[0], "missing.com") {
		t.Errorf("告警内容不符: %q", res.Warnings[0])
	}

	var total int
	if err := m.DB().Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules").Scan(&total); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if total != 2 {
		t.Fatalf("rules 行数 = %d, want 2（注释行不应入库）", total)
	}
	var enabledCount int
	if err := m.DB().Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules WHERE enabled = 1").Scan(&enabledCount); err != nil {
		t.Fatalf("count enabled: %v", err)
	}
	if enabledCount != 0 {
		t.Errorf("导入规则应全部 enabled=0，实际 %d 行启用", enabledCount)
	}

	var source, raw, originKey string
	var ruleConfig int64
	var resolveDNS bool
	err = m.DB().Raw().QueryRowContext(ctx,
		"SELECT source, raw, origin_key, rule_config, resolve_dns FROM rules WHERE rule_param = 'example.com'").
		Scan(&source, &raw, &originKey, &ruleConfig, &resolveDNS)
	if err != nil {
		t.Fatalf("读导入行: %v", err)
	}
	if source != "airport:HK" {
		t.Errorf("source = %q, want airport:HK", source)
	}
	if raw != "DOMAIN-SUFFIX,example.com,PROXY" {
		t.Errorf("raw = %q，应保留原行", raw)
	}
	if originKey != "DOMAIN-SUFFIX,example.com,PROXY" {
		t.Errorf("origin_key = %q", originKey)
	}
	if ruleConfig != g.ID {
		t.Errorf("rule_config = %d, want 分组 id %d", ruleConfig, g.ID)
	}
	if resolveDNS {
		t.Errorf("无 no-resolve 段时 resolve_dns 应为 false")
	}

	var missingConfig int64
	var missingRaw string
	if err := m.DB().Raw().QueryRowContext(ctx,
		"SELECT rule_config, raw FROM rules WHERE rule_param = 'missing.com'").
		Scan(&missingConfig, &missingRaw); err != nil {
		t.Fatalf("读缺失分组行: %v", err)
	}
	if missingConfig != 0 {
		t.Errorf("缺失分组的 rule_config = %d, want 0", missingConfig)
	}
	if missingRaw != "DOMAIN,missing.com,NOPE" {
		t.Errorf("缺失分组行的 raw = %q，不丢行", missingRaw)
	}
}

// TestImportRulesIdempotent 覆盖幂等：重复导入只跳过、不加行。
func TestImportRulesIdempotent(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	rules := []string{
		"DOMAIN-SUFFIX,example.com,PROXY",
		"DOMAIN,missing.com,NOPE",
		"# 注释",
	}
	if _, err := m.ImportRules(ctx, "airport:HK", rules, nil); err != nil {
		t.Fatalf("首次 ImportRules: %v", err)
	}
	res, err := m.ImportRules(ctx, "airport:HK", rules, nil)
	if err != nil {
		t.Fatalf("二次 ImportRules: %v", err)
	}
	if res.RulesAdded != 0 || res.RulesSkipped != 2 {
		t.Fatalf("added/skipped = %d/%d, want 0/2", res.RulesAdded, res.RulesSkipped)
	}
	var total int
	if err := m.DB().Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules").Scan(&total); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if total != 2 {
		t.Fatalf("幂等导入后 rules 行数 = %d, want 2", total)
	}
}

// TestImportRuleProviders 覆盖 provider 幂等写入且 enabled=false。
func TestImportRuleProviders(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	providers := []model.RuleProvider{
		{Name: "rp1", Raw: map[string]any{"url": "https://example.com/rule.yaml", "type": "http"}},
	}
	res, err := m.ImportRules(ctx, "airport:HK", nil, providers)
	if err != nil {
		t.Fatalf("ImportRules provider: %v", err)
	}
	if res.ProvidersAdded != 1 {
		t.Fatalf("ProvidersAdded = %d, want 1", res.ProvidersAdded)
	}
	list, err := m.DB().ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("ListRuleProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("provider 行数 = %d, want 1", len(list))
	}
	if list[0].Enabled {
		t.Errorf("导入 provider 应 enabled=false")
	}
	if list[0].Source != "airport:HK" {
		t.Errorf("provider source = %q, want airport:HK", list[0].Source)
	}
	if !strings.Contains(list[0].RawJSON, "example.com/rule.yaml") {
		t.Errorf("raw_json = %q，应保留 provider 原始定义", list[0].RawJSON)
	}

	res2, err := m.ImportRules(ctx, "airport:HK", nil, providers)
	if err != nil {
		t.Fatalf("二次 ImportRules provider: %v", err)
	}
	if res2.ProvidersAdded != 1 {
		t.Fatalf("二次 ProvidersAdded = %d, want 1", res2.ProvidersAdded)
	}
	list2, err := m.DB().ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("ListRuleProviders 二次: %v", err)
	}
	if len(list2) != 1 {
		t.Fatalf("重复导入后 provider 行数 = %d, want 1", len(list2))
	}
}

// TestImportRulesEmpty 覆盖空行跳过与空导入：全 0 且 Warnings 非 nil。
func TestImportRulesEmpty(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	res, err := m.ImportRules(ctx, "airport:HK", []string{"", "   "}, nil)
	if err != nil {
		t.Fatalf("ImportRules: %v", err)
	}
	if res.RulesAdded != 0 || res.RulesSkipped != 0 || res.ProvidersAdded != 0 {
		t.Fatalf("空导入应全 0: %+v", res)
	}
	if res.Warnings == nil {
		t.Fatal("Warnings 应为非 nil 空切片")
	}
	var total int
	if err := m.DB().Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules").Scan(&total); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	if total != 0 {
		t.Fatalf("空导入不应写行, got %d", total)
	}
}

// TestImportRulesResolveDNS 覆盖第 4 段 no-resolve 的解析口径：
// ResolveDNS=true 表示渲染时追加 ,no-resolve（见 internal/api/subscription.go:615-618）。
func TestImportRulesResolveDNS(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	rules := []string{
		"DOMAIN,nores.com,PROXY,no-resolve",
		"DOMAIN,res.com,PROXY",
	}
	if _, err := m.ImportRules(ctx, "airport:HK", rules, nil); err != nil {
		t.Fatalf("ImportRules: %v", err)
	}
	var noRes, res bool
	if err := m.DB().Raw().QueryRowContext(ctx, "SELECT resolve_dns FROM rules WHERE rule_param = 'nores.com'").Scan(&noRes); err != nil {
		t.Fatalf("读 nores.com: %v", err)
	}
	if err := m.DB().Raw().QueryRowContext(ctx, "SELECT resolve_dns FROM rules WHERE rule_param = 'res.com'").Scan(&res); err != nil {
		t.Fatalf("读 res.com: %v", err)
	}
	if !noRes {
		t.Errorf("no-resolve 行的 resolve_dns 应为 true（ResolveDNS=true 才渲染 ,no-resolve）")
	}
	if res {
		t.Errorf("无 no-resolve 行的 resolve_dns 应为 false")
	}
}

// TestImportRulesResolveDNSRoundTrip 覆盖「导入 → 渲染」往返一致性。
// 渲染口径内联自 internal/api/subscription.go:609-618（rule_config=0 或找不到分组 → DIRECT；
// ResolveDNS=true 才追加 ,no-resolve）。此处不 import internal/api，避免包成环。
func TestImportRulesResolveDNSRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	rules := []string{
		"IP-CIDR,1.1.1.1/32,DIRECT,no-resolve",
		"DOMAIN-SUFFIX,example.com,DIRECT",
	}
	res, err := m.ImportRules(ctx, "airport:HK", rules, nil)
	if err != nil {
		t.Fatalf("ImportRules: %v", err)
	}
	if res.RulesAdded != 2 {
		t.Fatalf("RulesAdded = %d, want 2", res.RulesAdded)
	}

	type ruleRow struct {
		raw        string
		ruleType   string
		ruleParam  string
		ruleConfig int64
		resolveDNS bool
	}
	read := func(param string) ruleRow {
		t.Helper()
		var r ruleRow
		if err := m.DB().Raw().QueryRowContext(ctx,
			"SELECT raw, rule_type, rule_param, rule_config, resolve_dns FROM rules WHERE rule_param = ?", param).
			Scan(&r.raw, &r.ruleType, &r.ruleParam, &r.ruleConfig, &r.resolveDNS); err != nil {
			t.Fatalf("读 %s: %v", param, err)
		}
		return r
	}
	// render 内联 internal/api/subscription.go:609-618：rule_config 为 0 时策略名落成 DIRECT。
	render := func(r ruleRow) string {
		policy := "DIRECT"
		if r.ruleConfig > 0 {
			t.Fatalf("本用例的 DIRECT 不是分组，rule_config 应为 0，实际 %d", r.ruleConfig)
		}
		line := strings.Join([]string{r.ruleType, r.ruleParam, policy}, ",")
		if r.resolveDNS {
			line += ",no-resolve"
		}
		return line
	}

	noRes := read("1.1.1.1/32")
	if !noRes.resolveDNS {
		t.Errorf("no-resolve 行的 resolve_dns 应为 true")
	}
	if noRes.raw != "IP-CIDR,1.1.1.1/32,DIRECT,no-resolve" {
		t.Errorf("raw = %q，应保留原行", noRes.raw)
	}
	if got := render(noRes); got != "IP-CIDR,1.1.1.1/32,DIRECT,no-resolve" {
		t.Errorf("往返渲染 = %q, want %q", got, "IP-CIDR,1.1.1.1/32,DIRECT,no-resolve")
	}

	plain := read("example.com")
	if plain.resolveDNS {
		t.Errorf("无 no-resolve 段的 resolve_dns 应为 false")
	}
	if plain.raw != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Errorf("raw = %q，应保留原行", plain.raw)
	}
	if got := render(plain); got != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Errorf("往返渲染 = %q, want %q", got, "DOMAIN-SUFFIX,example.com,DIRECT")
	}
}
