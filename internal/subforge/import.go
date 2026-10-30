package subforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/store"
)

// ErrEmptyImportSource 表示导入来源为空：导入必须说明这批规则来自哪个来源（层），
// 否则无法做同来源去重与层拼接。API 层可把它映射成 400。
var ErrEmptyImportSource = errors.New("导入来源不能为空")

// ImportOptions 控制一次导入的落库语义。
// 默认（零值）与旧行为一致：只进候选、不建组；调用方需要"导入即生效"时显式打开。
type ImportOptions struct {
	// CreateGroups 目标分组不存在时自动补建 select 分组。
	CreateGroups bool
	// EnableRules 新导入的规则直接 enabled=true。
	EnableRules bool
	// UpstreamGroups 是机场订阅自带的 proxy-groups；CreateGroups=true 时落库。
	UpstreamGroups []model.UpstreamGroup
	// UpstreamNodeNames 是同一份机场订阅解析出的节点名，用来判断某个上游分组
	// 是否覆盖全部机场节点：覆盖的组（通常是主节点组/漏网之鱼）默认在订阅里
	// 追加方案选中的自建节点，子集组保持原样。
	UpstreamNodeNames []string
	// SkipRules 只建分组/写 providers，不把规则写进 rules 表。
	// 机场规则改由版本化规则源产品承载；这里保留规则解析是为了按目标名补建分组。
	SkipRules bool
}

// ImportResult 汇总一次订阅规则导入的结果。
type ImportResult struct {
	RulesAdded     int `json:"rules_added"`
	RulesSkipped   int `json:"rules_skipped"`
	ProvidersAdded int `json:"providers_added"`
	// GroupsCreated 是本次导入自动新建的目标分组个数（仅 createGroups=true 时可能非 0）。
	GroupsCreated int `json:"groups_created"`
	// 机场规则源的写入结果（仅有机场订阅的导入路径会填）。
	RuleSourceName    string   `json:"rule_source_name,omitempty"`
	RuleSourceVersion string   `json:"rule_source_version,omitempty"`
	RuleSourceRules   int      `json:"rule_source_rules,omitempty"`
	Warnings          []string `json:"warnings"`
}

// coversAllNodes 判断分组成员是否覆盖全部给定节点名。
// 只认成员里逐字出现的节点名；成员里的子组名（如 PROXY）不展开。
// 没有可识别节点名时返回 false，避免把只有子组引用的分组误判成节点组。
func coversAllNodes(members, nodeNames []string) bool {
	if len(nodeNames) == 0 {
		return false
	}
	memberSet := make(map[string]bool, len(members))
	for _, m := range members {
		if v := strings.TrimSpace(m); v != "" {
			memberSet[v] = true
		}
	}
	seen := 0
	for _, n := range nodeNames {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		seen++
		if !memberSet[n] {
			return false
		}
	}
	return seen > 0
}

// normalizeUpstreamGroupType 把机场分组类型收敛到渲染器支持的范围；未知类型退化为 select。
func normalizeUpstreamGroupType(typ string) string {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "select", "url-test", "fallback", "load-balance":
		return strings.ToLower(strings.TrimSpace(typ))
	default:
		return "select"
	}
}

// NormalizeRuleOriginKey 把一行规则规范化为去重键。
// 去首尾空白、把连续空白压成一个空格、把 "," 两侧空白去掉。
func NormalizeRuleOriginKey(line string) string {
	// 连续空白压成一个空格（顺带去掉首尾空白）
	s := strings.Join(strings.Fields(line), " ")
	// 把 "," 两侧空白去掉
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, ", ", ",")
	return s
}

// isBuiltinPolicy 判断规则目标是不是内核内置出站——那些本来就不用建分组。
func isBuiltinPolicy(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE":
		return true
	}
	return false
}

// ImportRules 把解析出来的规则行与规则集写入候选队列。
// source 形如 "airport:<机场名>"；providers 用 UpsertRuleProvider 写入，enabled 恒为 false。
// 导入的规则一律 enabled=0（候选队列，不自动生效）。
//
// 等价于 ImportRulesToLibrary(ctx, source, rules, providers, false)：导入全局自定义库，
// 不自动建组，与引入方案前的行为完全一致。
func (m *Manager) ImportRules(ctx context.Context, source string, rules []string, providers []model.RuleProvider) (ImportResult, error) {
	return m.ImportRulesToLibrary(ctx, source, rules, providers, false)
}

// ImportRulesToLibrary 把解析出来的规则行与规则集导入全局自定义库。
//
// source 必填：机场导入由调用方传 airport:<机场名>；为空或全是空白时返回
// ErrEmptyImportSource（「导入来源不能为空」），且不落任何规则。
// 导入的规则一律 enabled=0（候选队列，不自动生效）。
//
// createGroups=true 时，规则目标（策略名）既不是内核内置出站、当前分组库里又没有同名分组，
// 就自动建一个 select 分组（regex="."、sort_order=0），新分组的 source 跟随本次导入的来源，
// 并把 rule_config 指向新组；分组库里已有同名分组时直接复用，不重复建组。
// providers 不分方案（订阅渲染不使用），仍写全局，来源跟随 source。
func (m *Manager) ImportRulesToLibrary(ctx context.Context, source string, rules []string, providers []model.RuleProvider, createGroups bool) (ImportResult, error) {
	return m.ImportRulesToLibraryWithOptions(ctx, source, rules, providers, ImportOptions{CreateGroups: createGroups})
}

// ImportRulesToLibraryWithOptions 是导入的完整入口；ImportRulesToLibrary 保留旧签名，
// 等价于只控制 CreateGroups、规则仍进候选。API 的“导入后启用”走这里。
func (m *Manager) ImportRulesToLibraryWithOptions(ctx context.Context, source string, rules []string, providers []model.RuleProvider, opts ImportOptions) (ImportResult, error) {
	createGroups := opts.CreateGroups
	result := ImportResult{
		Warnings: []string{},
	}

	// 来源必填：没有来源就没法把规则归到某一层，也无法做同来源去重。
	source = strings.TrimSpace(source)
	if source == "" {
		return result, ErrEmptyImportSource
	}

	// 目标分组表是全局库：同名分组先到者胜（按 sort_order,id 排序），
	// 已有同名分组就复用，避免出现指向同一个名字的死分组。
	groups, err := m.ListProxyGroups(ctx)
	if err != nil {
		return result, err
	}
	groupIDByName := make(map[string]int64, len(groups))
	for _, g := range groups {
		if _, ok := groupIDByName[g.Name]; !ok {
			groupIDByName[g.Name] = g.ID
		}
	}

	now := time.Now()
	missingTargets := map[string]int{}
	createdGroups := map[string]int{}

	// 机场自带分组先落库：规则目标按名字映射到这些组，避免被当成缺失目标
	// 再建一个 regex="." 的占位组。已有同名组（用户自建）优先，不覆盖。
	if createGroups {
		for _, ug := range opts.UpstreamGroups {
			name := strings.TrimSpace(ug.Name)
			if name == "" || len(ug.Members) == 0 {
				continue
			}
			if _, exists := groupIDByName[name]; exists {
				continue
			}
			includeSelf := coversAllNodes(ug.Members, opts.UpstreamNodeNames)
			g, gErr := m.AddProxyGroupWithMembersAndIncludeSelf(ctx, name, normalizeUpstreamGroupType(ug.Type),
				"", ug.URL, ug.Interval, 0, source, ug.Members, includeSelf)
			if gErr != nil {
				return result, gErr
			}
			groupIDByName[name] = g.ID
			result.GroupsCreated++
			createdGroups[name]++
		}
	}

	// 只取当前最大 sort_order（一行聚合），不再为了新序号把全部规则读进内存。
	maxSort, err := m.db.MaxRulesSortOrder(ctx)
	if err != nil {
		return result, err
	}

	pending := make([]store.ImportedRuleRow, 0, len(rules))
	for i, line := range rules {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		originKey := NormalizeRuleOriginKey(line)
		parts := strings.Split(line, ",")
		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}
		ruleType := ""
		ruleParam := ""
		policyName := ""
		if len(parts) > 0 {
			ruleType = parts[0]
		}
		if len(parts) > 1 {
			ruleParam = parts[1]
		}
		if len(parts) > 2 {
			policyName = parts[2]
		}
		// 渲染口径见 internal/api/subscription.go:615-618：ResolveDNS=true 才追加 ",no-resolve"。
		// 因此第 4 段（或其后任意一段）为 no-resolve 时置 true，否则导入往返会丢掉 no-resolve。
		resolveDNS := false
		if len(parts) >= 4 {
			for _, p := range parts[3:] {
				if strings.EqualFold(p, "no-resolve") {
					resolveDNS = true
					break
				}
			}
		}

		ruleConfig := int64(0)
		if policyName != "" {
			if id, ok := groupIDByName[policyName]; ok {
				ruleConfig = id
			} else if !isBuiltinPolicy(policyName) {
				if createGroups {
					// 自动建组：补一个 select 分组（来源跟随本次导入），规则直接指向新组。
					g, gErr := m.AddProxyGroupWithSource(ctx, policyName, "select", ".", "", 0, 0, source)
					if gErr != nil {
						return result, gErr
					}
					groupIDByName[policyName] = g.ID
					ruleConfig = g.ID
					result.GroupsCreated++
					createdGroups[policyName]++
				} else {
					// 目标分组还没在「分组」页建过：不丢规则，也不逐条刷警告。
					// 渲染时会按名称把目标映射到已有分组（见 internal/render/ruletarget.go），
					// 这里只统计一次，最后汇总成一条提示。
					missingTargets[policyName]++
				}
			}
		}

		if opts.SkipRules {
			continue
		}
		pending = append(pending, store.ImportedRuleRow{
			RuleType:   ruleType,
			RuleParam:  ruleParam,
			RuleConfig: ruleConfig,
			ResolveDNS: resolveDNS,
			Enabled:    opts.EnableRules,
			SortOrder:  maxSort + i + 1,
			Source:     source,
			Raw:        line,
			OriginKey:  originKey,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// 批量写：单事务一次提交，把 N 次 fsync 合成 1 次（原实现逐条 INSERT 让导入 9638 条耗时 >30s）。
	added, skipped, err := m.db.BatchInsertImportedRules(ctx, pending)
	if err != nil {
		// 事务已整体回滚：这次导入没有落库任何规则，计数保持 0。
		return result, err
	}
	result.RulesAdded += added
	result.RulesSkipped += skipped

	if len(missingTargets) > 0 {
		names := make([]string, 0, len(missingTargets))
		total := 0
		for name, n := range missingTargets {
			names = append(names, name)
			total += n
		}
		sort.Strings(names)
		if len(names) > 3 {
			names = append(names[:3], "…")
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%d 条规则的目标分组（如 %s）还没在「分组」页里建，转换时会按名称映射到已有分组、映射不到则指向兜底分组；要精确分流请先建同名分组",
			total, strings.Join(names, "、")))
	}
	if len(createdGroups) > 0 {
		names := make([]string, 0, len(createdGroups))
		total := 0
		for name, n := range createdGroups {
			names = append(names, name)
			total += n
		}
		sort.Strings(names)
		if len(names) > 3 {
			names = append(names[:3], "…")
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"已创建 %d 个分组（%s，含机场自带分组），导入规则的分组目标已指向这些新分组",
			total, strings.Join(names, "、")))
	}

	for _, p := range providers {
		rawJSONBytes, err := json.Marshal(p.Raw)
		if err != nil {
			return result, err
		}
		rawJSON := string(rawJSONBytes)
		if err := m.db.UpsertRuleProvider(ctx, store.RuleProvider{
			Name:      p.Name,
			RawJSON:   rawJSON,
			Source:    source,
			Enabled:   false,
			SortOrder: 0,
		}); err != nil {
			return result, err
		}
		result.ProvidersAdded++
	}

	return result, nil
}
