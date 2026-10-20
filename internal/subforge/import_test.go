package subforge

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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

	g, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0, 0)
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
	// 目标分组不存在时汇总成一条提示，列出缺失的分组名；规则本身照常入库，
	// 目标名保留在 raw 里，交给渲染阶段按名称映射（不丢规则、不逐条刷警告）。
	if !strings.Contains(res.Warnings[0], "NOPE") {
		t.Errorf("告警应点名缺失的目标分组: %q", res.Warnings[0])
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
	if _, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0, 0); err != nil {
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
	if _, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0, 0); err != nil {
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
	if _, err := m.AddProxyGroup(ctx, "PROXY", "select", "", "", 0, 0); err != nil {
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

// countImportRules 返回 rules 表总行数。
func countImportRules(t *testing.T, m *Manager) int {
	t.Helper()
	var n int
	if err := m.DB().Raw().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM rules").Scan(&n); err != nil {
		t.Fatalf("count rules: %v", err)
	}
	return n
}

// TestImportRulesBulk10000 覆盖 D-04：一次性导入 10000 条规则必须秒级完成（原来逐条
// INSERT 每条一次 fsync，9638 条需要 >30s）。同时断言计数、幂等跳过与 sort_order 连续性。
func TestImportRulesBulk10000(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.AddProxyGroup(ctx, "RELAY", "select", "", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	const n = 10000
	rules := make([]string, 0, n)
	for i := 0; i < n; i++ {
		rules = append(rules, fmt.Sprintf("DOMAIN-SUFFIX,uxt-%d.test,RELAY", i))
	}

	// 第一次：全部新增。目标「5000 条 < 3s」，这里按 10000 条放宽到 5s，避免慢 CI 误报。
	start := time.Now()
	res, err := m.ImportRules(ctx, "airport:bulk", rules, nil)
	first := time.Since(start)
	if err != nil {
		t.Fatalf("首次 ImportRules: %v", err)
	}
	if res.RulesAdded != n || res.RulesSkipped != 0 {
		t.Fatalf("首次 added/skipped = %d/%d, want %d/0", res.RulesAdded, res.RulesSkipped, n)
	}
	if first > 5*time.Second {
		t.Fatalf("首次导入 %d 条耗时 %v，应 < 5s（批量写在单事务里）", n, first)
	}
	if got := countImportRules(t, m); got != n {
		t.Fatalf("rules 行数 = %d, want %d", got, n)
	}

	// sort_order 必须从 1 开始严格递增（步长 1，无空洞）。
	rows, err := m.DB().Raw().QueryContext(ctx, "SELECT sort_order FROM rules ORDER BY sort_order")
	if err != nil {
		t.Fatalf("查询 sort_order: %v", err)
	}
	defer rows.Close()
	idx := 0
	for rows.Next() {
		var so int
		if err := rows.Scan(&so); err != nil {
			t.Fatalf("扫描 sort_order: %v", err)
		}
		if so != idx+1 {
			t.Fatalf("第 %d 行的 sort_order = %d, want %d（应严格递增且从 1 开始）", idx, so, idx+1)
		}
		idx++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历 sort_order: %v", err)
	}
	if idx != n {
		t.Fatalf("sort_order 行数 = %d, want %d", idx, n)
	}

	// 第二次：同样 10000 条，全部命中 (source, origin_key) 唯一索引 → 跳过、不新增行。
	start = time.Now()
	res2, err := m.ImportRules(ctx, "airport:bulk", rules, nil)
	second := time.Since(start)
	if err != nil {
		t.Fatalf("二次 ImportRules: %v", err)
	}
	if res2.RulesAdded != 0 || res2.RulesSkipped != n {
		t.Fatalf("二次 added/skipped = %d/%d, want 0/%d", res2.RulesAdded, res2.RulesSkipped, n)
	}
	if second > 5*time.Second {
		t.Fatalf("二次导入 %d 条耗时 %v，应 < 5s", n, second)
	}
	if got := countImportRules(t, m); got != n {
		t.Fatalf("二次导入后 rules 行数 = %d, want %d", got, n)
	}
	t.Logf("首次导入 %d 条耗时 %v；二次（全跳过）耗时 %v", n, first, second)

	// 追加导入：新序号必须从「已有最大值 + 1」继续，而不是从 1 重来。
	appendRules := []string{
		"DOMAIN-SUFFIX,uxt-new-1.test,RELAY",
		"DOMAIN-SUFFIX,uxt-new-2.test,RELAY",
	}
	res3, err := m.ImportRules(ctx, "airport:bulk", appendRules, nil)
	if err != nil {
		t.Fatalf("追加 ImportRules: %v", err)
	}
	if res3.RulesAdded != 2 || res3.RulesSkipped != 0 {
		t.Fatalf("追加 added/skipped = %d/%d, want 2/0", res3.RulesAdded, res3.RulesSkipped)
	}
	var minNew int
	if err := m.DB().Raw().QueryRowContext(ctx,
		"SELECT MIN(sort_order) FROM rules WHERE rule_param LIKE 'uxt-new-%'").Scan(&minNew); err != nil {
		t.Fatalf("查询追加行序号: %v", err)
	}
	if minNew != n+1 {
		t.Fatalf("追加行最小 sort_order = %d, want %d（原最大值 + 1）", minNew, n+1)
	}
}

// TestImportRulesBatchDuplicateWithinOneImport 覆盖同一批里出现完全重复行：
// INSERT OR IGNORE 让第二条跳过并计入 skipped，整批不失败。
func TestImportRulesBatchDuplicateWithinOneImport(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	rules := []string{
		"DOMAIN-SUFFIX,dup.test,DIRECT",
		"DOMAIN-SUFFIX,dup.test,DIRECT",
		"DOMAIN-SUFFIX,other.test,DIRECT",
	}
	res, err := m.ImportRules(ctx, "airport:dup", rules, nil)
	if err != nil {
		t.Fatalf("ImportRules: %v", err)
	}
	if res.RulesAdded != 2 || res.RulesSkipped != 1 {
		t.Fatalf("added/skipped = %d/%d, want 2/1", res.RulesAdded, res.RulesSkipped)
	}
	if got := countImportRules(t, m); got != 2 {
		t.Fatalf("rules 行数 = %d, want 2", got)
	}
}
