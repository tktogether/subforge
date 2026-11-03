package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tktogether/subforge/internal/subcfg"
)

// TerminalHit 是一个被识别为「来源最终选择组」的分组。
type TerminalHit struct {
	Source   string  `json:"source"`
	Group    string  `json:"group"`
	Evidence string  `json:"evidence"`
	Score    float64 `json:"score"`
}

// RuleSourceHit 是规则源产物里识别到的选择目标。
type RuleSourceHit struct {
	Name             string   `json:"name"`
	FinalTarget      string   `json:"final_target,omitempty"`
	SelectionTargets []string `json:"selection_targets,omitempty"`
	AffectedRules    int      `json:"affected_rules"`
}

// FinalGroupReport 描述一次统一最终选择组的识别与改写结果。
type FinalGroupReport struct {
	UnifiedName        string          `json:"unified_name"`
	Mode               string          `json:"mode"`
	Generated          bool            `json:"generated"`
	Terminals          []TerminalHit   `json:"terminals"`
	RuleSources        []RuleSourceHit `json:"rule_sources"`
	GroupsBefore       int             `json:"groups_before"`
	GroupsAfter        int             `json:"groups_after"`
	RemovedGroups      []string        `json:"removed_groups"`
	RewrittenGroupRefs int             `json:"rewritten_group_refs"`
	RewrittenRules     int             `json:"rewritten_rules"`
}

type sourceGroups struct {
	source      string
	groups      []Group
	indeg       map[string]int
	targetHits  map[string]int
	totalTarget int
}

// UnifyFinalGroup 把各来源的最终节点选择组收敛到一个统一组。
//
// 输入 groups 应已展开 include 标志并解析成员（Engine 在 ResolveGroups 之后调用）；
// nodeNames 是本次产物里的最终节点名，为空时从分组成员推导（供预览使用）。
// 函数不读写数据库，纯内存变换，关闭配置时原样返回。
func UnifyFinalGroup(groups []Group, rules []string, nodeNames []string, cfg subcfg.FinalGroupConfig, metas []subcfg.RuleSourceMeta) ([]Group, []string, FinalGroupReport, []string) {
	report := FinalGroupReport{
		UnifiedName:  cfg.ResolvedName(),
		Mode:         cfg.Mode,
		GroupsBefore: len(groups),
		GroupsAfter:  len(groups),
	}
	if !cfg.Enabled {
		return groups, rules, report, nil
	}
	if err := cfg.Normalize(); err != nil {
		return groups, rules, report, []string{"统一选择组配置无效：" + err.Error()}
	}
	report.UnifiedName = cfg.ResolvedName()
	report.Mode = cfg.Mode

	if len(nodeNames) == 0 {
		nodeNames = deriveNodeNames(groups)
	}
	nodeSet := make(map[string]bool, len(nodeNames))
	for _, n := range nodeNames {
		if n != "" {
			nodeSet[n] = true
		}
	}
	groupByName := make(map[string]Group, len(groups))
	sourceOrder := make([]string, 0, 4)
	sources := make(map[string]*sourceGroups, 4)
	for _, g := range groups {
		if g.Name == "" {
			continue
		}
		if _, exists := groupByName[g.Name]; !exists {
			groupByName[g.Name] = g
		}
		src := groupSource(g)
		info, ok := sources[src]
		if !ok {
			info = &sourceGroups{source: src, indeg: map[string]int{}, targetHits: map[string]int{}}
			sources[src] = info
			sourceOrder = append(sourceOrder, src)
		}
		info.groups = append(info.groups, g)
	}
	for _, info := range sources {
		for _, g := range info.groups {
			for _, m := range g.Proxies {
				if m == "" || m == g.Name {
					continue
				}
				if other, ok := groupByName[m]; ok && groupSource(other) == info.source {
					info.indeg[m]++
				}
			}
		}
	}
	for _, line := range rules {
		if target, typ, ok := finalGroupRuleTarget(line); ok && typ == "" {
			if g, exists := groupByName[target]; exists {
				info := sources[groupSource(g)]
				if info != nil {
					info.targetHits[target]++
					info.totalTarget++
				}
			}
		}
	}

	terminals := map[string]bool{}
	terminalNamesBySource := map[string][]string{}
	var warnings []string
	for _, src := range sourceOrder {
		info := sources[src]
		if info == nil || len(info.groups) == 0 {
			continue
		}
		if names, ok := cfg.OverrideFor("group:" + src); ok {
			if subcfg.IsNoneOverride(names) {
				continue
			}
			valid := make([]string, 0, len(names))
			for _, name := range names {
				if _, exists := groupByName[name]; exists && groupSource(groupByName[name]) == src {
					valid = append(valid, name)
					continue
				}
				warnings = append(warnings, fmt.Sprintf("统一选择组覆盖：来源 %s 里没有分组 %q，已忽略", src, name))
			}
			for _, name := range valid {
				terminals[name] = true
				terminalNamesBySource[src] = append(terminalNamesBySource[src], name)
				report.Terminals = append(report.Terminals, TerminalHit{Source: src, Group: name, Evidence: "手动覆盖"})
			}
			continue
		}
		best, hit, candidates := detectTerminal(info, nodeSet)
		if best == "" {
			continue
		}
		if len(candidates) > 1 {
			warnings = append(warnings, fmt.Sprintf(
				"来源 %s 识别到多个候选最终组（%s），已选择 %q；可在方案里手动覆盖",
				src, strings.Join(candidates, "、"), best))
		}
		terminals[best] = true
		terminalNamesBySource[src] = []string{best}
		report.Terminals = append(report.Terminals, TerminalHit{
			Source: src, Group: best, Evidence: hit.evidence, Score: hit.score,
		})
	}
	sort.SliceStable(report.Terminals, func(i, j int) bool {
		if report.Terminals[i].Source == report.Terminals[j].Source {
			return report.Terminals[i].Group < report.Terminals[j].Group
		}
		return report.Terminals[i].Source < report.Terminals[j].Source
	})

	ruleSelection := map[string]bool{}
	for _, meta := range metas {
		if strings.TrimSpace(meta.Name) == "" {
			continue
		}
		if names, ok := cfg.OverrideFor("rule:" + meta.Name); ok {
			if subcfg.IsNoneOverride(names) {
				continue
			}
			for _, name := range names {
				ruleSelection[name] = true
			}
			hit := RuleSourceHit{Name: meta.Name, SelectionTargets: append([]string{}, names...)}
			for _, name := range names {
				hit.AffectedRules += meta.TargetCounts[name]
			}
			if len(meta.FinalTargets) > 0 {
				hit.FinalTarget = meta.FinalTargets[len(meta.FinalTargets)-1]
			}
			report.RuleSources = append(report.RuleSources, hit)
			continue
		}
		hit := RuleSourceHit{Name: meta.Name}
		if len(meta.FinalTargets) > 0 {
			hit.FinalTarget = meta.FinalTargets[len(meta.FinalTargets)-1]
		}
		targets := make([]string, 0, len(meta.TargetCounts))
		for target := range meta.TargetCounts {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		for _, target := range targets {
			if meta.TargetCounts[target] <= 0 || isBuiltinTarget(target) || finalGroupExcludedName(target) {
				continue
			}
			if selectionNameScore(target) > 0 {
				ruleSelection[target] = true
				hit.SelectionTargets = append(hit.SelectionTargets, target)
				hit.AffectedRules += meta.TargetCounts[target]
			}
		}
		for _, target := range meta.FinalTargets {
			if selectionNameScore(target) > 0 && !finalGroupExcludedName(target) && !isBuiltinTarget(target) {
				ruleSelection[target] = true
				if !containsString(hit.SelectionTargets, target) {
					hit.SelectionTargets = append(hit.SelectionTargets, target)
					hit.AffectedRules += meta.TargetCounts[target]
				}
			}
		}
		sort.Strings(hit.SelectionTargets)
		if len(hit.SelectionTargets) > 0 || hit.FinalTarget != "" {
			report.RuleSources = append(report.RuleSources, hit)
		}
	}

	unifiedName := cfg.ResolvedName()
	reuseIndex := -1
	if _, exists := groupByName[unifiedName]; exists {
		existing := groupByName[unifiedName]
		if terminals[unifiedName] || groupSource(existing) == "custom" {
			reuseIndex = indexOfGroup(groups, unifiedName)
		} else {
			warnings = append(warnings, fmt.Sprintf(
				"统一选择组名称 %q 已被来源 %s 的分组占用；本次跳过统一，请换一个名称",
				unifiedName, groupSource(existing)))
			report.GroupsAfter = len(groups)
			return groups, rules, report, warnings
		}
	}
	if _, exists := terminals[unifiedName]; exists {
		delete(terminals, unifiedName)
	}

	var unified Group
	generated := true
	if reuseIndex >= 0 {
		unified = groups[reuseIndex]
		generated = false
	} else {
		unified = Group{Name: unifiedName, Type: "select", Source: "generated"}
	}
	if unified.Type == "" {
		unified.Type = "select"
	}
	if !finalGroupTypeSupported(unified.Type) {
		warnings = append(warnings, fmt.Sprintf("统一组类型 %q 不受支持，已改为 select", unified.Type))
		unified.Type = "select"
	}
	// provider 只存在于组定义里，不展开成节点名；统一组必须把它们带上，否则会丢节点。
	useSeen := make(map[string]bool, len(unified.Use))
	for _, u := range unified.Use {
		useSeen[u] = true
	}
	var unifiedUse []string
	for _, u := range unified.Use {
		unifiedUse = append(unifiedUse, u)
	}
	for _, src := range sourceOrder {
		for _, name := range terminalNamesBySource[src] {
			if g, ok := groupByName[name]; ok {
				for _, u := range g.Use {
					if u != "" && !useSeen[u] {
						useSeen[u] = true
						unifiedUse = append(unifiedUse, u)
					}
				}
			}
		}
	}
	unified.Use = unifiedUse

	memberSeen := make(map[string]bool, len(unified.Proxies)+len(nodeNames)+8)
	members := make([]string, 0, len(unified.Proxies)+len(nodeNames)+8)
	addMember := func(name string) {
		if name == "" || name == unifiedName || memberSeen[name] {
			return
		}
		memberSeen[name] = true
		members = append(members, name)
	}
	for _, m := range unified.Proxies {
		addMember(m)
	}
	if cfg.KeepSourceTerminals {
		for _, src := range sourceOrder {
			for _, name := range terminalNamesBySource[src] {
				addMember(name)
			}
		}
	}
	for _, n := range nodeNames {
		addMember(n)
	}
	if cfg.IncludeDirect {
		addMember("DIRECT")
	}
	if len(members) == 0 {
		addMember("DIRECT")
	}
	unified.Proxies = members

	outGroups := make([]Group, 0, len(groups)+1)
	outGroups = append(outGroups, unified)
	removed := make([]string, 0, len(terminals))
	for i, g := range groups {
		if i == reuseIndex {
			continue
		}
		if terminals[g.Name] {
			if !cfg.KeepSourceTerminals {
				removed = append(removed, g.Name)
				continue
			}
		}
		ng := g
		ng.Proxies = rewriteMembers(g.Proxies, g.Name, terminals, unifiedName)
		if ng.Name != unifiedName {
			outGroups = append(outGroups, ng)
		}
	}
	report.RemovedGroups = removed
	report.Generated = generated
	report.GroupsAfter = len(outGroups)

	outRules := make([]string, 0, len(rules)+1)
	rewrittenRules := 0
	for _, line := range rules {
		target, typ, ok := finalGroupRuleTarget(line)
		if !ok {
			outRules = append(outRules, line)
			continue
		}
		if typ != "" {
			// 旧 FINAL/MATCH 一律收口到统一的末尾 MATCH；多来源兜底不能各自生效。
			continue
		}
		if terminals[target] || ruleSelection[target] {
			fields := splitRuleFields(line)
			idx := ruleTargetIndex(fields)
			if idx >= 0 && idx < len(fields) {
				fields[idx] = unifiedName
				line = joinRuleFields(fields)
				rewrittenRules++
			}
		}
		outRules = append(outRules, line)
	}
	outRules = append(outRules, "MATCH,"+unifiedName)
	report.RewrittenRules = rewrittenRules
	report.RewrittenGroupRefs = rewrittenGroupRefs(groups, terminals)
	if len(report.Terminals) > 0 {
		names := make([]string, 0, len(report.Terminals))
		for _, hit := range report.Terminals {
			names = append(names, hit.Source+":"+hit.Group)
		}
		warnings = append(warnings, fmt.Sprintf("已把 %d 个来源最终组收敛到 %q（%s）",
			len(report.Terminals), unifiedName, strings.Join(names, "、")))
	}
	if len(report.Terminals) == 0 {
		warnings = append(warnings, "未识别到任何来源的最终选择组；仍已生成统一组并把 MATCH 指向它")
	}
	if len(report.RuleSources) > 0 {
		names := make([]string, 0, len(report.RuleSources))
		for _, rs := range report.RuleSources {
			names = append(names, rs.Name)
		}
		warnings = append(warnings, fmt.Sprintf("规则源选择目标已检查：%s", strings.Join(names, "、")))
	}
	return outGroups, outRules, report, warnings
}

type terminalScore struct {
	name     string
	score    float64
	evidence string
}

func detectTerminal(info *sourceGroups, nodeSet map[string]bool) (string, terminalScore, []string) {
	groupNames := make(map[string]bool, len(info.groups))
	for _, g := range info.groups {
		groupNames[g.Name] = true
	}
	var best terminalScore
	var candidates []string
	for _, g := range info.groups {
		if !finalGroupTypeSupported(finalGroupType(g)) || finalGroupExcludedName(g.Name) {
			continue
		}
		members := g.Proxies
		nodeMembers := 0
		for _, m := range members {
			if nodeSet[m] || builtinTargets[m] {
				nodeMembers++
			}
		}
		nodeRatio := 0.0
		if len(members) > 0 {
			nodeRatio = float64(nodeMembers) / float64(len(members))
		}
		allNodes := g.IncludeAllSources || g.IncludeSelf || nodeRatio >= 0.8
		if len(g.Use) == 0 && nodeRatio < 0.5 && !allNodes {
			continue
		}
		indeg := info.indeg[g.Name]
		ratio := 0.0
		if n := len(info.groups) - 1; n > 0 {
			ratio = float64(indeg) / float64(n)
		}
		nameScore := selectionNameScore(g.Name)
		ruleShare := 0.0
		if info.totalTarget > 0 {
			ruleShare = float64(info.targetHits[g.Name]) / float64(info.totalTarget)
		}
		// 规则占比只作为佐证：服务组（如「国内服务」）也可能占大量规则，
		// 没有入度或选择语义名字时不能单独据此认定最终组。
		if !(ratio >= 0.2 && indeg >= 1) && nameScore == 0 {
			continue
		}
		score := ratio*3 + float64(nameScore)*2 + ruleShare*2
		if allNodes {
			score += 1
		}
		evidence := fmt.Sprintf("被同来源 %d/%d 个分组引用；%d 条规则指向；成员覆盖 %d/%d",
			indeg, maxInt(1, len(info.groups)-1), info.targetHits[g.Name], nodeMembers, len(members))
		if nameScore > 0 {
			evidence += "；名字命中选择语义"
		}
		candidates = append(candidates, g.Name)
		if score > best.score+1e-9 {
			best = terminalScore{name: g.Name, score: score, evidence: evidence}
		}
	}
	if best.name == "" {
		return "", terminalScore{}, nil
	}
	return best.name, best, candidates
}

func rewriteMembers(members []string, self string, terminals map[string]bool, unified string) []string {
	out := make([]string, 0, len(members))
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		if m == "" || m == self {
			continue
		}
		if terminals[m] {
			m = unified
		}
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if len(out) == 0 {
		out = append(out, "DIRECT")
	}
	return out
}

func rewrittenGroupRefs(groups []Group, terminals map[string]bool) int {
	n := 0
	for _, g := range groups {
		for _, m := range g.Proxies {
			if terminals[m] {
				n++
			}
		}
	}
	return n
}

// deriveNodeNames 供预览使用：从分组成员里推导节点名（排除组名与内置出站）。
func deriveNodeNames(groups []Group) []string {
	groupNames := make(map[string]bool, len(groups))
	for _, g := range groups {
		groupNames[g.Name] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, g := range groups {
		for _, m := range g.Proxies {
			if m == "" || groupNames[m] || builtinTargets[m] || seen[m] {
				continue
			}
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func finalGroupType(g Group) string {
	if g.Type == "" {
		return "select"
	}
	return g.Type
}

func finalGroupTypeSupported(typ string) bool {
	switch typ {
	case "select", "url-test", "fallback", "load-balance":
		return true
	}
	return false
}

func groupSource(g Group) string {
	if strings.TrimSpace(g.Source) == "" {
		return "custom"
	}
	return g.Source
}

func selectionNameScore(name string) int {
	n := normalizeName(name)
	if n == "" {
		return 0
	}
	strong := []string{"节点选择", "手动选择", "选择节点", "代理选择", "选择代理", "主节点", "出口", "proxy", "select", "manual"}
	for _, k := range strong {
		if strings.Contains(n, k) {
			return 2
		}
	}
	weak := []string{"节点", "代理", "connect", "auto", "自动", "选择"}
	for _, k := range weak {
		if strings.Contains(n, k) {
			return 1
		}
	}
	return 0
}

func finalGroupExcludedName(name string) bool {
	n := normalizeName(name)
	if n == "" {
		return false
	}
	for _, k := range []string{"直连", "拦截", "广告", "漏网", "默认", "其他", "direct", "reject", "adblock", "adguard", "final", "other", "default"} {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

func isBuiltinTarget(name string) bool {
	return builtinTargets[name] || builtinTargets[strings.ToUpper(strings.TrimSpace(name))]
}

// finalGroupRuleTarget 返回规则行的目标与类型；typ 非空表示 MATCH/FINAL。
func finalGroupRuleTarget(line string) (target, typ string, ok bool) {
	fields := splitRuleFields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	head := strings.ToUpper(strings.TrimSpace(fields[0]))
	if head == "MATCH" || head == "FINAL" {
		return strings.TrimSpace(fields[1]), head, true
	}
	idx := ruleTargetIndex(fields)
	if idx < 0 || idx >= len(fields) {
		return "", "", false
	}
	return strings.TrimSpace(fields[idx]), "", true
}

func indexOfGroup(groups []Group, name string) int {
	for i, g := range groups {
		if g.Name == name {
			return i
		}
	}
	return -1
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
