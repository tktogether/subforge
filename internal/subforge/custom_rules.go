package subforge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/store"
)

// CustomImportResult 是从规则源挑选规则加入自定义层的结果。
type CustomImportResult struct {
	Added     int      `json:"added"`
	Skipped   int      `json:"skipped"`
	Overwrote int      `json:"overwrote"`
	Warnings  []string `json:"warnings"`
}

// ImportRulesToCustom 把用户从规则源挑选的规则复制到自定义层（source=""）。
//
// 冲突按 (source="", origin_key) 判断：已存在时 overwrite=true 覆盖目标/参数，
// 否则跳过。规则集引用（RULE-SET）依赖 provider 定义，当前自定义层不保存
// provider，先拒绝并给出可见提示，避免复制出一条加载不了的规则。
func (m *Manager) ImportRulesToCustom(ctx context.Context, rules []string, overwrite bool) (CustomImportResult, error) {
	out := CustomImportResult{Warnings: []string{}}
	groups, err := m.ListProxyGroups(ctx)
	if err != nil {
		return out, err
	}
	groupIDByName := make(map[string]int64, len(groups))
	for _, g := range groups {
		if _, ok := groupIDByName[g.Name]; !ok {
			groupIDByName[g.Name] = g.ID
		}
	}
	custom, err := m.ListRulesFiltered(ctx, SourceFilter{Source: "", Prefix: false})
	if err != nil {
		return out, err
	}
	// 冲突按 type+param 判断：同一条匹配、不同目标也应视为重复，
	// 否则规则页会出现两条同匹配规则，渲染时先到先得，用户很难理解。
	byKey := make(map[string]Rule, len(custom))
	for _, r := range custom {
		byKey[ruleMatchKey(r.RuleType, r.RuleParam)] = r
	}
	maxSort, err := m.db.MaxRulesSortOrder(ctx)
	if err != nil {
		return out, err
	}
	now := time.Now()
	pending := make([]store.ImportedRuleRow, 0, len(rules))
	batchKeys := make(map[string]bool, len(rules))
	for i, raw := range rules {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}
		typ, param, policy := "", "", ""
		if len(parts) > 0 {
			typ = parts[0]
		}
		if len(parts) > 1 {
			param = parts[1]
		}
		if len(parts) > 2 {
			policy = parts[2]
		}
		if strings.EqualFold(typ, "RULE-SET") {
			out.Skipped++
			out.Warnings = append(out.Warnings, fmt.Sprintf("规则 %q 依赖规则集 provider，暂不支持复制到自定义层", line))
			continue
		}
		matchKey := ruleMatchKey(typ, param)
		if batchKeys[matchKey] {
			out.Skipped++
			continue
		}
		batchKeys[matchKey] = true
		resolveDNS := false
		for _, p := range parts[3:] {
			if strings.EqualFold(p, "no-resolve") {
				resolveDNS = true
				break
			}
		}
		ruleConfig := int64(0)
		if policy != "" && !isBuiltinPolicy(policy) {
			ruleConfig = groupIDByName[policy]
		}
		if existing, ok := byKey[matchKey]; ok {
			if !overwrite {
				out.Skipped++
				continue
			}
			if err := m.UpdateRuleFromSource(ctx, existing.ID, typ, param, ruleConfig, resolveDNS, line, NormalizeRuleOriginKey(line)); err != nil {
				return out, err
			}
			out.Overwrote++
			continue
		}
		pending = append(pending, store.ImportedRuleRow{
			RuleType:   typ,
			RuleParam:  param,
			RuleConfig: ruleConfig,
			ResolveDNS: resolveDNS,
			Enabled:    true,
			SortOrder:  maxSort + i + 1,
			Source:     "",
			Raw:        line,
			OriginKey:  NormalizeRuleOriginKey(line),
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	if len(pending) > 0 {
		added, skipped, err := m.db.BatchInsertImportedRules(ctx, pending)
		if err != nil {
			return out, err
		}
		out.Added += added
		out.Skipped += skipped
	}
	return out, nil
}

// ruleMatchKey 是挑选冲突的判断键：类型 + 参数，不含目标。
func ruleMatchKey(typ, param string) string {
	return strings.ToUpper(strings.TrimSpace(typ)) + "," + strings.ToLower(strings.TrimSpace(param))
}
