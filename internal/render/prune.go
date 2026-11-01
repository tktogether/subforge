package render

import "strings"

// PruneUnusedGroups 删除没有被最终规则或保留分组成员（递归）引用的分组。
//
// 订阅方案可以只选部分规则源；模板/其他来源带来的分组如果没有任何规则指向，
// 在客户端里就是无效条目。规则目标指向节点名或内核内置出站时不会保留分组。
func PruneUnusedGroups(groups []Group, rules []string) []Group {
	if len(groups) == 0 {
		return groups
	}
	byName := make(map[string]bool, len(groups))
	members := make(map[string][]string, len(groups))
	for _, g := range groups {
		byName[g.Name] = true
		members[g.Name] = g.Proxies
	}

	used := make(map[string]bool, len(groups))
	var visit func(name string)
	visit = func(name string) {
		if used[name] || !byName[name] {
			return
		}
		used[name] = true
		for _, member := range members[name] {
			visit(member)
		}
	}
	for _, line := range rules {
		fields := splitRuleFields(line)
		if len(fields) == 0 || strings.HasPrefix(strings.TrimSpace(fields[0]), "#") {
			continue
		}
		idx := ruleTargetIndex(fields)
		if idx < 0 || idx >= len(fields) {
			continue
		}
		visit(strings.TrimSpace(fields[idx]))
	}

	out := make([]Group, 0, len(groups))
	for _, g := range groups {
		if used[g.Name] {
			out = append(out, g)
		}
	}
	return out
}
