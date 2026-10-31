package subforge

import (
	"context"
	"errors"
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

// TestImportRulesWithOptionsEnableRules 覆盖“导入即启用”：新规则 enabled=true，
// 同时按 CreateGroups 补建目标分组，供机场页默认勾选的“立即启用”使用。
func TestImportRulesWithOptionsEnableRules(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	res, err := m.ImportRulesToLibraryWithOptions(ctx, "airport:HK",
		[]string{"DOMAIN-SUFFIX,enabled.test,自动启用组"}, nil,
		ImportOptions{CreateGroups: true, EnableRules: true})
	if err != nil {
		t.Fatalf("ImportRulesToLibraryWithOptions: %v", err)
	}
	if res.RulesAdded != 1 || res.GroupsCreated != 1 {
		t.Fatalf("added/groups = %d/%d，应为 1/1", res.RulesAdded, res.GroupsCreated)
	}
	rules, err := m.ListRulesByLayers(ctx, []string{"airport"})
	if err != nil {
		t.Fatalf("ListRulesByLayers: %v", err)
	}
	if len(rules) != 1 || !rules[0].Enabled {
		t.Fatalf("规则应 enabled=true，实际 %+v", rules)
	}
	groups, err := m.ListProxyGroups(ctx)
	if err != nil {
		t.Fatalf("ListProxyGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "自动启用组" {
		t.Fatalf("目标分组未创建，实际 %+v", groups)
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

// TestImportRulesToLibraryEmptySource 覆盖导入来源必填：空串与纯空白都返回
// ErrEmptyImportSource，且不落任何规则、不建分组。
func TestImportRulesToLibraryEmptySource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	for _, source := range []string{"", "   ", "\t "} {
		res, err := m.ImportRulesToLibrary(ctx, source, []string{"DOMAIN-SUFFIX,example.com,DIRECT"}, nil, true)
		if !errors.Is(err, ErrEmptyImportSource) {
			t.Fatalf("source=%q 应返回 ErrEmptyImportSource, 实际: %v", source, err)
		}
		if res.RulesAdded != 0 || res.GroupsCreated != 0 {
			t.Fatalf("source=%q 不应落任何数据, 实际 added=%d groups=%d", source, res.RulesAdded, res.GroupsCreated)
		}
	}
	// 旧薄封装同样校验来源
	if _, err := m.ImportRules(ctx, "", []string{"DOMAIN-SUFFIX,example.com,DIRECT"}, nil); !errors.Is(err, ErrEmptyImportSource) {
		t.Fatalf("ImportRules 空来源应返回 ErrEmptyImportSource, 实际: %v", err)
	}
	if got := countImportRules(t, m); got != 0 {
		t.Fatalf("空来源导入后 rules 行数 = %d, want 0", got)
	}
}

// TestImportRulesToLibraryCreateGroups 覆盖 createGroups=true：
// 目标不是内置出站且分组库里没有同名分组时自动建 select 分组，
// 新分组 source 跟随本次导入来源，规则 rule_config 指向新组。
func TestImportRulesToLibraryCreateGroups(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	res, err := m.ImportRulesToLibrary(ctx, "airport:HK", []string{
		"DOMAIN-SUFFIX,example.com,MyPolicy",
		"DOMAIN-SUFFIX,other.com,DIRECT",
	}, nil, true)
	if err != nil {
		t.Fatalf("ImportRulesToLibrary: %v", err)
	}
	if res.RulesAdded != 2 {
		t.Fatalf("RulesAdded = %d, want 2", res.RulesAdded)
	}
	if res.GroupsCreated != 1 {
		t.Fatalf("GroupsCreated = %d, want 1（DIRECT 是内置出站，不建组）", res.GroupsCreated)
	}

	groups, err := m.ListProxyGroupsFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil {
		t.Fatalf("ListProxyGroupsFiltered: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("应只建 1 个分组, got %d", len(groups))
	}
	if groups[0].Name != "MyPolicy" || groups[0].Type != "select" || groups[0].Regex != "." ||
		groups[0].Source != "airport:HK" {
		t.Fatalf("自动建分组字段不符: %+v", groups[0])
	}

	rules, err := m.ListRulesFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil || len(rules) != 2 {
		t.Fatalf("导入规则应按来源可查, got %d err=%v", len(rules), err)
	}
	for _, r := range rules {
		if r.Source != "airport:HK" || r.Enabled {
			t.Fatalf("导入规则 source/enabled 不符: %+v", r)
		}
		if r.Raw == "" || r.OriginKey == "" {
			t.Fatalf("导入规则应保留 raw 与 origin_key: %+v", r)
		}
	}
	var my *Rule
	for i := range rules {
		if rules[i].RuleParam == "example.com" {
			my = &rules[i]
		}
	}
	if my == nil {
		t.Fatal("未找到 example.com 规则")
	}
	if my.RuleConfig != groups[0].ID {
		t.Fatalf("规则 rule_config 应指向自动建的组 %d, 实际 %d", groups[0].ID, my.RuleConfig)
	}
}

// TestImportRulesToLibraryReusesExistingGroup 覆盖同名分组复用：
// 分组库里已有同名分组时不重建（GroupsCreated=0），规则直接指向已有分组。
func TestImportRulesToLibraryReusesExistingGroup(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	existing, err := m.AddProxyGroup(ctx, "MyPolicy", "url-test", "example\\.(com|net)", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	res, err := m.ImportRulesToLibrary(ctx, "airport:HK", []string{
		"DOMAIN-SUFFIX,example.com,MyPolicy",
	}, nil, true)
	if err != nil {
		t.Fatalf("ImportRulesToLibrary: %v", err)
	}
	if res.GroupsCreated != 0 {
		t.Fatalf("已有同名分组时不应新建, GroupsCreated = %d", res.GroupsCreated)
	}
	all, err := m.ListProxyGroups(ctx)
	if err != nil || len(all) != 1 || all[0].ID != existing.ID {
		t.Fatalf("分组库应仍只有 1 个分组, got %+v err=%v", all, err)
	}

	rules, err := m.ListRulesFiltered(ctx, SourceFilter{Source: "airport:HK"})
	if err != nil || len(rules) != 1 {
		t.Fatalf("导入规则应为 1 条, got %d err=%v", len(rules), err)
	}
	if rules[0].RuleConfig != existing.ID {
		t.Fatalf("规则应指向已有分组 %d, 实际 %d", existing.ID, rules[0].RuleConfig)
	}
	// createGroups=false 时不建组，只汇总提示
	res, err = m.ImportRulesToLibrary(ctx, "airport:JP", []string{
		"DOMAIN-SUFFIX,fresh.com,NewPolicy",
	}, nil, false)
	if err != nil {
		t.Fatalf("ImportRulesToLibrary(createGroups=false): %v", err)
	}
	if res.GroupsCreated != 0 || len(res.Warnings) != 1 {
		t.Fatalf("createGroups=false 应只提示不建组, groups=%d warnings=%v", res.GroupsCreated, res.Warnings)
	}
}

// TestImportRulesWithUpstreamGroups 机场自带分组应先落库，规则目标直接指向该组，
// 不再为同名目标建 regex="." 的占位组。
func TestImportRulesWithUpstreamGroups(t *testing.T) {
	m := newTestManager(t)
	upstream := []model.UpstreamGroup{
		{Name: "PROXY", Type: "select", Members: []string{"HK-1", "JP-1"}},
		{Name: "HK-ONLY", Type: "select", Members: []string{"HK-1"}},
		{Name: "GROUP-ONLY", Type: "select", Members: []string{"PROXY", "DIRECT"}},
	}
	res, err := m.ImportRulesToLibraryWithOptions(t.Context(), "airport:HK",
		[]string{"DOMAIN-SUFFIX,x.test,PROXY"}, nil,
		ImportOptions{
			CreateGroups:      true,
			EnableRules:       true,
			UpstreamGroups:    upstream,
			UpstreamNodeNames: []string{"HK-1", "JP-1"},
		})
	if err != nil {
		t.Fatalf("ImportRulesToLibraryWithOptions: %v", err)
	}
	if res.GroupsCreated != 3 {
		t.Fatalf("GroupsCreated = %d，应为 3", res.GroupsCreated)
	}
	groups, err := m.ListProxyGroups(t.Context())
	if err != nil || len(groups) != 3 {
		t.Fatalf("分组数不符: %+v err=%v", groups, err)
	}
	byName := make(map[string]ProxyGroup, len(groups))
	for _, g := range groups {
		byName[g.Name] = g
	}
	if g := byName["PROXY"]; strings.Join(g.Members, ",") != "HK-1,JP-1" || !g.IncludeSelf || !g.IncludeAllSources {
		t.Fatalf("覆盖全部机场节点的 PROXY 应开启自建/全部来源扩展: %+v", g)
	}
	if g := byName["HK-ONLY"]; g.IncludeSelf || g.IncludeAllSources {
		t.Fatalf("子集组 HK-ONLY 不应开启扩展: %+v", g)
	}
	if g := byName["GROUP-ONLY"]; g.IncludeSelf || g.IncludeAllSources {
		t.Fatalf("只引用子组的 GROUP-ONLY 不应开启扩展: %+v", g)
	}
	rules, err := m.ListRulesByLayers(t.Context(), []string{"airport"})
	if err != nil || len(rules) != 1 || rules[0].RuleConfig != byName["PROXY"].ID || !rules[0].Enabled {
		t.Fatalf("规则未指向上游分组: %+v err=%v", rules, err)
	}
}

// TestCoversAllNodes 覆盖全节点判断的边界：空名单、空成员、子组名、部分覆盖。
func TestCoversAllNodes(t *testing.T) {
	cases := []struct {
		name      string
		members   []string
		nodeNames []string
		want      bool
	}{
		{"空节点名单", []string{"HK-1"}, nil, false},
		{"空成员", nil, []string{"HK-1"}, false},
		{"全部覆盖", []string{"HK-1", "JP-1"}, []string{"HK-1", "JP-1"}, true},
		{"含子组名也覆盖", []string{"PROXY", "HK-1", "JP-1", "DIRECT"}, []string{"HK-1", "JP-1"}, true},
		{"只缺一个", []string{"HK-1"}, []string{"HK-1", "JP-1"}, false},
		{"节点名单只有空白", []string{"HK-1"}, []string{"  "}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := coversAllNodes(c.members, c.nodeNames); got != c.want {
				t.Fatalf("coversAllNodes(%v,%v) = %v，应为 %v", c.members, c.nodeNames, got, c.want)
			}
		})
	}
}

// TestImportRefreshUpdatesAllSourceFlag 同一机场再次导入时，派生的成员与
// 「全部来源」标志要跟随订阅变化；用户改过的类型保留。
func TestImportRefreshUpdatesAllSourceFlag(t *testing.T) {
	m := newTestManager(t)
	nodes := []string{"HK-1", "JP-1"}
	upstream := []model.UpstreamGroup{{Name: "ALL", Type: "select", Members: nodes}}
	opts := ImportOptions{
		CreateGroups:      true,
		UpstreamGroups:    upstream,
		UpstreamNodeNames: nodes,
	}
	if _, err := m.ImportRulesToLibraryWithOptions(t.Context(), "airport:fx", nil, nil, opts); err != nil {
		t.Fatalf("首次导入: %v", err)
	}
	groups, _ := m.ListProxyGroups(t.Context())
	if len(groups) != 1 || !groups[0].IncludeAllSources {
		t.Fatalf("首次导入应开启全部来源: %+v", groups)
	}
	if err := m.UpdateProxyGroup(t.Context(), groups[0].ID, "ALL", "url-test", "", "https://test", 300, 0); err != nil {
		t.Fatalf("UpdateProxyGroup: %v", err)
	}

	// 第二次导入变成子集组：标志应关闭，成员跟随订阅，类型保留。
	opts.UpstreamGroups = []model.UpstreamGroup{{Name: "ALL", Type: "select", Members: []string{"HK-1"}}}
	opts.UpstreamNodeNames = []string{"HK-1", "JP-1"}
	res, err := m.ImportRulesToLibraryWithOptions(t.Context(), "airport:fx", nil, nil, opts)
	if err != nil {
		t.Fatalf("二次导入: %v", err)
	}
	if res.GroupsCreated != 0 {
		t.Fatalf("同源分组应刷新而不是新建，GroupsCreated = %d", res.GroupsCreated)
	}
	got, err := m.GetProxyGroup(t.Context(), groups[0].ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if got.IncludeAllSources || len(got.Members) != 1 || got.Members[0] != "HK-1" {
		t.Fatalf("二次导入未刷新扩展/成员: %+v", got)
	}
	if got.Type != "url-test" || got.URL != "https://test" {
		t.Fatalf("用户编辑的类型/测活不应被覆盖: %+v", got)
	}
}

// TestImportRefreshPromotesFullGroupToSelf 分组从子集升级为全节点时恢复完整扩展，
// 修复旧版本把「自动选择」判成子集后留下的 all=false/self=false 状态。
func TestImportRefreshPromotesFullGroupToSelf(t *testing.T) {
	m := newTestManager(t)
	opts := ImportOptions{
		CreateGroups:      true,
		UpstreamGroups:    []model.UpstreamGroup{{Name: "AUTO", Type: "url-test", Members: []string{"HK-1"}}},
		UpstreamNodeNames: []string{"HK-1", "JP-1"},
	}
	if _, err := m.ImportRulesToLibraryWithOptions(t.Context(), "airport:fx", nil, nil, opts); err != nil {
		t.Fatalf("首次导入: %v", err)
	}
	groups, _ := m.ListProxyGroups(t.Context())
	if len(groups) != 1 || groups[0].IncludeAllSources || groups[0].IncludeSelf {
		t.Fatalf("子集组不应扩展: %+v", groups)
	}
	opts.UpstreamGroups = []model.UpstreamGroup{{Name: "AUTO", Type: "url-test", Members: []string{"HK-1", "JP-1"}}}
	if _, err := m.ImportRulesToLibraryWithOptions(t.Context(), "airport:fx", nil, nil, opts); err != nil {
		t.Fatalf("二次导入: %v", err)
	}
	got, err := m.GetProxyGroup(t.Context(), groups[0].ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if !got.IncludeAllSources || !got.IncludeSelf {
		t.Fatalf("升级为全节点组后应恢复完整扩展: %+v", got)
	}
}

func TestSplitRuleFieldsLogical(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"DOMAIN-SUFFIX,a.com,PROXY", []string{"DOMAIN-SUFFIX", "a.com", "PROXY"}},
		{"AND,((DST-PORT,443),(NETWORK,UDP)),PROXY", []string{"AND", "((DST-PORT,443),(NETWORK,UDP))", "PROXY"}},
		{"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve", []string{"IP-CIDR", "10.0.0.0/8", "DIRECT", "no-resolve"}},
	}
	for _, c := range cases {
		got := splitRuleFields(c.line)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("splitRuleFields(%q) = %#v, want %#v", c.line, got, c.want)
		}
	}
}

// TestImportLogicalRuleDoesNotCreateBogusGroup 逻辑规则参数里的逗号不能被当成目标分组。
func TestImportLogicalRuleDoesNotCreateBogusGroup(t *testing.T) {
	m := newTestManager(t)
	res, err := m.ImportRulesToLibraryWithOptions(t.Context(), "airport:x", []string{
		"AND,((DST-PORT,443),(NETWORK,UDP)),PROXY",
	}, nil, ImportOptions{
		CreateGroups:      true,
		EnableRules:       true,
		UpstreamGroups:    []model.UpstreamGroup{{Name: "PROXY", Type: "select", Members: []string{"A-1"}}},
		UpstreamNodeNames: []string{"A-1"},
	})
	if err != nil {
		t.Fatalf("导入逻辑规则: %v", err)
	}
	if res.GroupsCreated != 1 {
		t.Fatalf("应只创建 PROXY 一个分组，GroupsCreated=%d", res.GroupsCreated)
	}
	groups, err := m.ListProxyGroups(t.Context())
	if err != nil {
		t.Fatalf("ListProxyGroups: %v", err)
	}
	for _, g := range groups {
		if g.Name == "443)" || g.Name == "PROXY" && len(groups) != 1 {
			t.Fatalf("出现异常分组: %+v", groups)
		}
	}
	rules, err := m.ListRulesFiltered(t.Context(), SourceFilter{Source: "airport:x"})
	if err != nil || len(rules) != 1 {
		t.Fatalf("逻辑规则未正确导入: %+v err=%v", rules, err)
	}
	if rules[0].RuleType != "AND" || rules[0].RuleParam != "((DST-PORT,443),(NETWORK,UDP))" {
		t.Fatalf("逻辑规则字段错误: %+v", rules[0])
	}
	if rules[0].RuleConfig == 0 {
		t.Fatalf("逻辑规则目标未解析到 PROXY: %+v", rules[0])
	}
}
