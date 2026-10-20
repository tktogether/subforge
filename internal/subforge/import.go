package subforge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/store"
)

// ImportResult 汇总一次订阅规则导入的结果。
type ImportResult struct {
	RulesAdded     int      `json:"rules_added"`
	RulesSkipped   int      `json:"rules_skipped"`
	ProvidersAdded int      `json:"providers_added"`
	Warnings       []string `json:"warnings"`
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
func (m *Manager) ImportRules(ctx context.Context, source string, rules []string, providers []model.RuleProvider) (ImportResult, error) {
	result := ImportResult{
		Warnings: []string{},
	}

	groups, err := m.ListProxyGroups(ctx)
	if err != nil {
		return result, err
	}
	groupIDByName := make(map[string]int64, len(groups))
	for _, g := range groups {
		groupIDByName[g.Name] = g.ID
	}

	// 只取当前最大 sort_order（一行聚合），不再为了新序号把全部规则读进内存。
	maxSort, err := m.db.MaxRulesSortOrder(ctx)
	if err != nil {
		return result, err
	}

	now := time.Now()
	missingTargets := map[string]int{}
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
				// 目标分组还没在「分组」页建过：不丢规则，也不逐条刷警告。
				// 渲染时会按名称把目标映射到已有分组（见 internal/render/ruletarget.go），
				// 这里只统计一次，最后汇总成一条提示。
				missingTargets[policyName]++
			}
		}

		pending = append(pending, store.ImportedRuleRow{
			RuleType:   ruleType,
			RuleParam:  ruleParam,
			RuleConfig: ruleConfig,
			ResolveDNS: resolveDNS,
			Enabled:    false,
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
