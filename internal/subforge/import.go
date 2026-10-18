package subforge

import (
	"context"
	"encoding/json"
	"fmt"
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

	var maxSort int
	existing, err := m.ListRules(ctx)
	if err != nil {
		return result, err
	}
	for _, r := range existing {
		if r.SortOrder > maxSort {
			maxSort = r.SortOrder
		}
	}

	now := time.Now()
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
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("规则「%s」的目标分组「%s」不存在", line, policyName))
			}
		}

		sortOrder := maxSort + i + 1
		_, err := m.db.Raw().ExecContext(ctx,
			"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, source, raw, origin_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			ruleType, ruleParam, ruleConfig, resolveDNS, false, sortOrder, source, line, originKey, now, now)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				result.RulesSkipped++
				_, _ = m.db.Raw().ExecContext(ctx,
					"UPDATE rules SET updated_at = ? WHERE source = ? AND origin_key = ?",
					now, source, originKey)
				continue
			}
			return result, err
		}
		result.RulesAdded++
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
