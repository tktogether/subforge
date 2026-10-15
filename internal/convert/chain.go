package convert

import (
	"regexp"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/render"
)

// MergeParams 合并多份原始参数，后者覆盖前者。
//
// 覆盖粒度是"整个键"而不是"追加取值"：URL 上写了 include 就应该完全取代模板里的
// include，而不是把两套正则叠加起来——否则用户想临时关掉某个过滤会做不到。
func MergeParams(sources ...map[string][]string) map[string][]string {
	out := map[string][]string{}
	for _, src := range sources {
		if src == nil {
			continue
		}
		for k, v := range src {
			out[k] = v
		}
	}
	return out
}

// ApplyTemplate 把模板语义压到请求参数下面，落地覆盖链里"模板 < 请求参数"这一级。
//
// 原始参数在 map 层合并完统一交给 ParseMulti，正则与排序键只校验一次，
// CLI 与 HTTP 两条入口走同一段代码。
func ApplyTemplate(sem config.Semantics, base Params) (Params, error) {
	p, err := ParseMulti(MergeParams(sem.Params, base.Raw))
	if err != nil {
		return Params{}, err
	}
	// Raw 里没有的两个字段需要手工带过来：请求头从不进 query，文件列表只有 CLI 入口会填。
	p.Headers = base.Headers
	p.Files = base.Files

	// 结构化字段与原始参数同一套规则：请求侧写了就用请求侧的，否则用模板的。
	// ParseMulti 不会填这两个字段，所以必须显式带过来，否则会静默丢掉请求侧的设置。
	p.NodePref = base.NodePref
	if p.NodePref.Empty() {
		p.NodePref = sem.NodePref
	}
	p.Groups = base.Groups
	if len(p.Groups) == 0 {
		p.Groups = sem.Groups
	}
	return p, nil
}

// ResolveGroups 把代理组成员里的正则展开成实际节点名。
//
// 成员项可能是节点名正则（如 ".*HK.*"）、另一个组的名称，或 DIRECT / REJECT 这类
// 内置出站。匹配不到任何节点的条目一律按字面量保留——后两种情况本来就不该被展开。
func ResolveGroups(groups []render.Group, names []string) []render.Group {
	if len(groups) == 0 {
		return nil
	}
	out := make([]render.Group, 0, len(groups))
	for _, g := range groups {
		g.Proxies = resolveMembers(g.Proxies, names)
		out = append(out, g)
	}
	return out
}

// resolveMembers 展开成员项：同一节点被多个模式命中只出现一次，顺序按首次命中。
func resolveMembers(patterns, names []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]string, 0, len(patterns))
	seen := map[string]bool{}
	for _, p := range patterns {
		if matched := expandMember(p, names); len(matched) > 0 {
			for _, name := range matched {
				if seen[name] {
					continue
				}
				seen[name] = true
				out = append(out, name)
			}
			continue
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// expandMember 用正则 p 匹配节点名。返回 nil 表示没有命中（或正则非法），
// 调用方会把 p 当字面量保留。
func expandMember(p string, names []string) []string {
	re, err := regexp.Compile(p)
	if err != nil {
		return nil
	}
	var out []string
	for _, name := range names {
		if re.MatchString(name) {
			out = append(out, name)
		}
	}
	return out
}
