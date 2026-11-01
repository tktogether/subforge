package render

import (
	"reflect"
	"testing"
)

func groupNames(groups []Group) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.Name)
	}
	return out
}

func TestPruneUnusedGroups(t *testing.T) {
	groups := []Group{
		{Name: "A", Proxies: []string{"B", "node-1"}},
		{Name: "B", Proxies: []string{"node-1"}},
		{Name: "C", Proxies: []string{"node-1"}},
		{Name: "D", Proxies: []string{"A"}},
	}
	rules := []string{
		"DOMAIN-SUFFIX,example.com,A",
		"GEOIP,CN,DIRECT",
	}
	got := PruneUnusedGroups(groups, rules)
	want := []string{"A", "B"}
	if !reflect.DeepEqual(groupNames(got), want) {
		t.Fatalf("groups = %v, want %v（A 被规则引用，B 是 A 的成员；D 无人引用）", groupNames(got), want)
	}
}

func TestPruneUnusedGroupsMatchAndLogical(t *testing.T) {
	groups := []Group{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	rules := []string{
		"MATCH,A",
		"AND,((DOMAIN-SUFFIX,x),(NETWORK,tcp)),B",
		"# comment,C",
	}
	got := PruneUnusedGroups(groups, rules)
	if want := []string{"A", "B"}; !reflect.DeepEqual(groupNames(got), want) {
		t.Fatalf("groups = %v, want %v", groupNames(got), want)
	}
}

func TestPruneUnusedGroupsBuiltinOnly(t *testing.T) {
	got := PruneUnusedGroups([]Group{{Name: "A"}}, []string{"MATCH,DIRECT", "GEOIP,CN,REJECT"})
	if len(got) != 0 {
		t.Fatalf("只有内置出站时应清空分组, got %v", groupNames(got))
	}
}

func TestPruneUnusedGroupsKeepsCycles(t *testing.T) {
	groups := []Group{
		{Name: "A", Proxies: []string{"B"}},
		{Name: "B", Proxies: []string{"A"}},
		{Name: "C", Proxies: []string{"A"}},
	}
	got := PruneUnusedGroups(groups, []string{"MATCH,A"})
	if want := []string{"A", "B"}; !reflect.DeepEqual(groupNames(got), want) {
		t.Fatalf("groups = %v, want %v（循环引用不能死循环，C 无引用应删除）", groupNames(got), want)
	}
}

func TestPruneUnusedGroupsEmptyAndMalformed(t *testing.T) {
	if got := PruneUnusedGroups(nil, nil); len(got) != 0 {
		t.Fatalf("空输入应返回空, got %v", got)
	}
	got := PruneUnusedGroups([]Group{{Name: "A"}}, []string{"MATCH", "", "GEOIP,CN", "#注释"})
	if len(got) != 0 {
		t.Fatalf("无法解析目标或注释不应保留分组, got %v", groupNames(got))
	}
}
