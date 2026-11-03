package render

import (
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subcfg"
)

func testFinalGroups() []Group {
	return []Group{
		{Name: "🔰 选择节点", Type: "select", Source: "airport:A", Proxies: []string{"A-1", "A-2", "DIRECT"}},
		{Name: "Netflix", Type: "select", Source: "airport:A", Proxies: []string{"🔰 选择节点", "A-1", "DIRECT"}},
		{Name: "Lite Connect", Type: "select", Source: "airport:B", Proxies: []string{"B-1", "B-2", "DIRECT"}},
		{Name: "Apple", Type: "select", Source: "airport:B", Proxies: []string{"Lite Connect", "B-2", "DIRECT"}},
		{Name: "🐟 漏网之鱼", Type: "select", Source: "airport:A", Proxies: []string{"🔰 选择节点", "DIRECT"}},
	}
}

func enabledConfig() subcfg.FinalGroupConfig {
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Enabled = true
	return cfg
}

func groupByName(groups []Group, name string) (Group, bool) {
	for _, g := range groups {
		if g.Name == name {
			return g, true
		}
	}
	return Group{}, false
}

func TestUnifyFinalGroupMergesSourceTerminals(t *testing.T) {
	rules := []string{
		"DOMAIN,a.example,🔰 选择节点",
		"DOMAIN,b.example,Lite Connect",
		"DOMAIN,c.example,Netflix",
		"MATCH,🐟 漏网之鱼",
	}
	groups, outRules, report, warns := UnifyFinalGroup(
		testFinalGroups(), rules, []string{"A-1", "A-2", "B-1", "B-2"}, enabledConfig(), nil)

	if len(report.Terminals) != 2 {
		t.Fatalf("Terminals = %+v，期望识别 2 个来源最终组", report.Terminals)
	}
	unified, ok := groupByName(groups, "🚀 节点选择")
	if !ok {
		t.Fatalf("未生成统一组，groups=%v", finalGroupNames(groups))
	}
	if groups[0].Name != unified.Name {
		t.Errorf("统一组不在首位: %v", finalGroupNames(groups))
	}
	for _, want := range []string{"A-1", "A-2", "B-1", "B-2", "DIRECT"} {
		if !containsString(unified.Proxies, want) {
			t.Errorf("统一组成员缺少 %q: %v", want, unified.Proxies)
		}
	}
	if _, exists := groupByName(groups, "🔰 选择节点"); exists {
		t.Errorf("来源最终组应被删除")
	}
	if _, exists := groupByName(groups, "Lite Connect"); exists {
		t.Errorf("来源最终组应被删除")
	}
	netflix, _ := groupByName(groups, "Netflix")
	if !containsString(netflix.Proxies, "🚀 节点选择") || containsString(netflix.Proxies, "🔰 选择节点") {
		t.Errorf("Netflix 引用未改写: %v", netflix.Proxies)
	}
	if !strings.Contains(strings.Join(outRules, "\n"), "DOMAIN,a.example,🚀 节点选择") {
		t.Errorf("规则目标未改写: %v", outRules)
	}
	if got := outRules[len(outRules)-1]; got != "MATCH,🚀 节点选择" {
		t.Errorf("末尾 MATCH = %q，期望指向统一组", got)
	}
	if len(warns) == 0 {
		t.Errorf("多来源归并应至少产生一条说明")
	}
	if report.GroupsAfter != 4 {
		t.Errorf("GroupsAfter = %d，期望 unified+Netflix+Apple+漏网之鱼 = 4", report.GroupsAfter)
	}
}

func TestUnifyFinalGroupCustomNameAndOverride(t *testing.T) {
	cfg := enabledConfig()
	cfg.Mode = subcfg.FinalGroupModeCustom
	cfg.Name = "🌐 统一出口"
	cfg.Overrides = map[string][]string{"group:airport:B": {"Lite Connect"}}
	groups, rules, report, _ := UnifyFinalGroup(testFinalGroups(), []string{"DOMAIN,x,Lite Connect"}, nil, cfg, nil)
	if report.UnifiedName != "🌐 统一出口" {
		t.Fatalf("UnifiedName = %q", report.UnifiedName)
	}
	if _, ok := groupByName(groups, "🌐 统一出口"); !ok {
		t.Fatalf("自定义统一组未生成: %v", finalGroupNames(groups))
	}
	if _, ok := groupByName(groups, "Lite Connect"); ok {
		t.Errorf("被覆盖的最终组应仍被识别并删除")
	}
	if got := rules[len(rules)-1]; got != "MATCH,🌐 统一出口" {
		t.Errorf("MATCH = %q", got)
	}
}

func TestUnifyFinalGroupReusesCustomGroup(t *testing.T) {
	cfg := enabledConfig()
	cfg.Mode = subcfg.FinalGroupModeCustom
	cfg.Name = "🌐 统一出口"
	groups := append(testFinalGroups(), Group{
		Name: "🌐 统一出口", Type: "select", Source: "", Proxies: []string{"A-1", "DIRECT"},
	})
	out, _, report, _ := UnifyFinalGroup(groups, nil, []string{"A-1", "A-2", "B-1", "B-2"}, cfg, nil)
	if report.Generated {
		t.Errorf("已存在自定义组时应复用而不是新建")
	}
	reused, ok := groupByName(out, "🌐 统一出口")
	if !ok {
		t.Fatalf("复用组丢失: %v", finalGroupNames(out))
	}
	for _, want := range []string{"A-1", "A-2", "B-1", "B-2", "DIRECT"} {
		if !containsString(reused.Proxies, want) {
			t.Errorf("复用组缺少节点 %s: %v", want, reused.Proxies)
		}
	}
	if out[0].Name != "🌐 统一出口" {
		t.Errorf("复用组应在首位: %v", finalGroupNames(out))
	}
}

func TestUnifyFinalGroupRuleSourceSelection(t *testing.T) {
	metas := []subcfg.RuleSourceMeta{{
		Name:         "Aethersailor",
		RuleCount:    3,
		FinalTargets: []string{"🐟 漏网之鱼"},
		TargetCounts: map[string]int{"🚀 手动选择": 2, "🎯 全球直连": 1},
	}}
	rules := []string{"DOMAIN,a,🚀 手动选择", "DOMAIN,b,🚀 手动选择"}
	_, outRules, report, _ := UnifyFinalGroup(testFinalGroups(), rules, nil, enabledConfig(), metas)
	if report.RewrittenRules != 2 {
		t.Errorf("RewrittenRules = %d，期望 2", report.RewrittenRules)
	}
	for _, line := range outRules[:2] {
		if !strings.HasSuffix(line, ",🚀 节点选择") {
			t.Errorf("规则源选择目标未改写: %s", line)
		}
	}
}

func TestUnifyFinalGroupKeepTerminalsAndNoneOverride(t *testing.T) {
	cfg := enabledConfig()
	cfg.KeepSourceTerminals = true
	cfg.Overrides = map[string][]string{"group:airport:B": {"none"}}
	groups, _, report, _ := UnifyFinalGroup(testFinalGroups(), nil, nil, cfg, nil)
	if _, ok := groupByName(groups, "🔰 选择节点"); !ok {
		t.Errorf("keep_source_terminals 应保留来源最终组")
	}
	if _, ok := groupByName(groups, "Lite Connect"); !ok {
		t.Errorf("none 覆盖应跳过该来源，Lite Connect 应保留: %v", finalGroupNames(groups))
	}
	unified, _ := groupByName(groups, "🚀 节点选择")
	if !containsString(unified.Proxies, "🔰 选择节点") {
		t.Errorf("保留的来源最终组应成为统一组成员: %v", unified.Proxies)
	}
	if len(report.Terminals) != 1 {
		t.Errorf("Terminals = %+v，期望只识别 A", report.Terminals)
	}
}

func TestUnifyFinalGroupNameConflictSkips(t *testing.T) {
	cfg := enabledConfig()
	cfg.Mode = subcfg.FinalGroupModeCustom
	cfg.Name = "Netflix"
	groups, rules, _, warns := UnifyFinalGroup(testFinalGroups(), []string{"DOMAIN,x,Netflix"}, nil, cfg, nil)
	if len(groups) != len(testFinalGroups()) || len(rules) != 1 {
		t.Errorf("名称冲突时应原样返回")
	}
	if len(warns) == 0 || !strings.Contains(strings.Join(warns, " "), "已被来源") {
		t.Errorf("名称冲突应产生 warning: %v", warns)
	}
}

func TestUnifyFinalGroupDisabledUnchanged(t *testing.T) {
	groups := testFinalGroups()
	rules := []string{"MATCH,DIRECT"}
	cfg := subcfg.DefaultFinalGroupConfig()
	outGroups, outRules, report, warns := UnifyFinalGroup(groups, rules, nil, cfg, nil)
	if len(outGroups) != len(groups) || len(outRules) != len(rules) || report.UnifiedName == "" || len(warns) != 0 {
		t.Errorf("关闭配置应原样返回: %d/%d %v", len(outGroups), len(outRules), warns)
	}
}

func TestUnifyFinalGroupBranches(t *testing.T) {
	groups := []Group{
		{Name: "", Type: "select", Source: "airport:A", Proxies: []string{"n1"}},
		{Name: "选择节点A", Type: "select", Source: "airport:A", Proxies: []string{"n1", "DIRECT"}, Use: []string{"prov-a"}},
		{Name: "手动选择", Type: "select", Source: "airport:A", Proxies: []string{"n1", "DIRECT"}},
		{Name: "服务A", Type: "select", Source: "airport:A", Proxies: []string{"选择节点A", "n1", ""}},
		{Name: "服务B", Type: "select", Source: "airport:A", Proxies: []string{"手动选择", "n1"}},
		{Name: "无来源组", Type: "select", Source: "airport:B", Proxies: []string{"n2", "DIRECT"}},
		{Name: "🌐 统一出口", Type: "weird", Source: "", Proxies: []string{"n1", "DIRECT"}, Use: []string{"prov-custom"}},
	}
	cfg := enabledConfig()
	cfg.Mode = subcfg.FinalGroupModeCustom
	cfg.Name = "🌐 统一出口"
	cfg.Overrides = map[string][]string{
		"group:airport:A": {"missing", "选择节点A", "手动选择"},
		"rule:src":        {"手动选择"},
	}
	metas := []subcfg.RuleSourceMeta{
		{Name: "", TargetCounts: map[string]int{"x": 1}},
		{Name: "src", FinalTargets: []string{"手动选择", "🐟 漏网之鱼"}, TargetCounts: map[string]int{"手动选择": 2}},
	}
	rules := []string{"X", "DOMAIN,a,手动选择", "MATCH,DIRECT"}
	out, outRules, report, warns := UnifyFinalGroup(groups, rules, []string{"n1", "n2"}, cfg, metas)
	if _, ok := groupByName(out, "🌐 统一出口"); !ok {
		t.Fatalf("复用组丢失: %v", finalGroupNames(out))
	}
	if report.Generated {
		t.Errorf("应复用自定义组")
	}
	if len(report.Terminals) != 2 || report.Terminals[0].Evidence != "手动覆盖" {
		t.Errorf("覆盖应只保留有效组: %+v", report.Terminals)
	}
	if len(report.RuleSources) == 0 || report.RuleSources[0].AffectedRules != 2 {
		t.Errorf("规则源覆盖未生效: %+v", report.RuleSources)
	}
	if !containsString(outRules, "DOMAIN,a,🌐 统一出口") {
		t.Errorf("规则源选择目标未改写: %v", outRules)
	}
	if !containsString(outRules, "X") {
		t.Errorf("无法解析的规则行应原样保留: %v", outRules)
	}
	if len(warns) == 0 {
		t.Errorf("应产生覆盖/多候选告警")
	}
}

func TestUnifyFinalGroupNoTerminalAndDirectOnly(t *testing.T) {
	groups := []Group{
		{Name: "Netflix", Type: "select", Source: "airport:A", Proxies: []string{"n1", "DIRECT"}},
	}
	cfg := enabledConfig()
	cfg.IncludeDirect = false
	out, rules, report, warns := UnifyFinalGroup(groups, []string{"GEOIP,CN,DIRECT"}, []string{"n1"}, cfg, nil)
	if len(report.Terminals) != 0 {
		t.Fatalf("不应识别到最终组: %+v", report.Terminals)
	}
	unified, ok := groupByName(out, "🚀 节点选择")
	if !ok || containsString(unified.Proxies, "DIRECT") {
		t.Errorf("include_direct=false 时统一组不应含 DIRECT: %+v", unified.Proxies)
	}
	if !containsString(rules, "GEOIP,CN,DIRECT") || rules[len(rules)-1] != "MATCH,🚀 节点选择" {
		t.Errorf("规则处理异常: %v", rules)
	}
	if len(warns) == 0 {
		t.Errorf("未识别到最终组应告警")
	}
}

func TestFinalGroupHelpers(t *testing.T) {
	if selectionNameScore("") != 0 || finalGroupExcludedName("") {
		t.Errorf("空名字应返回 0/false")
	}
	if _, _, ok := finalGroupRuleTarget("X"); ok {
		t.Errorf("单字段规则不应解析出目标")
	}
	if _, _, ok := finalGroupRuleTarget("MATCH"); ok {
		t.Errorf("MATCH 缺目标不应解析")
	}
	if idx := indexOfGroup(testFinalGroups(), "missing"); idx != -1 {
		t.Errorf("indexOfGroup(missing) = %d", idx)
	}
}

func TestSelectionNameScore(t *testing.T) {
	for _, name := range []string{"🔰 选择节点", "🚀 手动选择", "Lite Connect", "PROXY"} {
		if selectionNameScore(name) == 0 {
			t.Errorf("%q 应被识别为选择语义", name)
		}
	}
	for _, name := range []string{"Netflix", "国内服务", "🐟 漏网之鱼", "Apple"} {
		if selectionNameScore(name) != 0 {
			t.Errorf("%q 不应被识别为选择语义", name)
		}
	}
}

func finalGroupNames(groups []Group) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.Name)
	}
	return out
}
