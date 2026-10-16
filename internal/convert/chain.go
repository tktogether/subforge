package convert

import (
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
	// Raw 里没有的字段需要手工带过来：请求头从不进 query，文件列表只有 CLI 入口会填。
	p.Headers = base.Headers
	p.Files = base.Files
	// 规则流不合并到 Rules：模板里声明的远程规则集要由 Engine 拉取后按原位置展开，
	// 拆成行再接上来会让展开出的规则落到模板的 MATCH 之后、变成永远命中不到的死规则。
	// 展开完成前 TemplateRules 保留，expandRemoteRulesets 会就地替换。
	p.TemplateRules = sem.RuleItems
	p.TemplateRemoteRulesets = sem.RemoteRulesets
	p.TemplateWarnings = sem.Warnings
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
// 成员项有三种：另一个组的名称（含 DIRECT / REJECT 这类内置出站）、节点名正则、
// 或裸节点名。只有正则参与展开——组名若被当正则处理，select 组引用的子组会整个丢光。
func ResolveGroups(groups []render.Group, names []string) []render.Group {
	if len(groups) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, g := range groups {
		if g.Name != "" {
			known[g.Name] = true
		}
	}
	out := make([]render.Group, 0, len(groups))
	for _, g := range groups {
		g.Proxies = resolveMembers(g.Proxies, names, known)
		out = append(out, g)
	}
	return out
}

// resolveMembers 展开成员项：同一节点被多个模式命中只出现一次，顺序按首次命中。
func resolveMembers(patterns, names []string, known map[string]bool) []string {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]string, 0, len(patterns))
	seen := map[string]bool{}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		matched, ok := expandMember(p, names)
		switch {
		case isMemberRef(p, known):
			addLiteral(p, &seen, &out)
		case !ok:
			// 无法解释成正则：可能是带正则元字符的裸节点名，原样保留。
			addLiteral(p, &seen, &out)
		case len(matched) > 0:
			for _, name := range matched {
				if !seen[name] {
					seen[name] = true
					out = append(out, name)
				}
			}
		default:
			// 正则合法但没有任何节点命中：给直连兜底。把正则串原样写进配置会被
			// mihomo 当成"引用了不存在的代理组"拒绝加载，整份配置一并报废。
			addLiteral("DIRECT", &seen, &out)
		}
	}
	return out
}

// addLiteral 按字面量追加成员，去重。
func addLiteral(p string, seen *map[string]bool, out *[]string) {
	if (*seen)[p] {
		return
	}
	(*seen)[p] = true
	*out = append(*out, p)
}

// isMemberRef 成员项是否是组名或内置出站——按字面量引用，不参与正则展开。
func isMemberRef(p string, known map[string]bool) bool {
	return known[p] || memberOutbounds[p]
}

// expandMember 用成员正则匹配节点名。
//
// ok 为假表示 pattern 无法解释成正则，调用方按字面量保留；
// ok 为真而返回空切片表示正则合法但没有节点命中，调用方给直连兜底。
func expandMember(p string, names []string) ([]string, bool) {
	re, negate, ok := CompileMemberPattern(p)
	if !ok {
		return nil, false
	}
	var out []string
	for _, name := range names {
		hit := re.MatchString(name)
		if negate {
			hit = !hit
		}
		if hit {
			out = append(out, name)
		}
	}
	return out, true
}
