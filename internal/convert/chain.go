package convert

import (
	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/model"
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
	// 模板层开关是调用方（API/CLI）直接赋值的结构化字段，同样不在 Raw 里。
	p.DisableTemplateRules = base.DisableTemplateRules
	p.DisableTemplateGroups = base.DisableTemplateGroups
	// 规则源提供的 rule-providers 是结构化定义，原样带到渲染请求。
	p.RuleProviders = append([]model.RuleProvider{}, base.RuleProviders...)
	// 规则流不合并到 Rules：模板里声明的远程规则集要由 Engine 拉取后按原位置展开，
	// 拆成行再接上来会让展开出的规则落到模板的 MATCH 之后、变成永远命中不到的死规则。
	// 展开完成前 TemplateRules 保留，expandRemoteRulesets 会就地替换。
	p.TemplateRules = sem.RuleItems
	p.TemplateRemoteRulesets = sem.RemoteRulesets
	// 告警是「追加」语义而不是覆盖：调用方（订阅）已经攒了规则源产物注入之类的
	// 提示，直接赋值会把它们静默丢掉——控制台看不见，用户只看到产物没生效。
	p.TemplateWarnings = append(append([]string{}, base.TemplateWarnings...), sem.Warnings...)
	// 命名管线只由模板声明（[pipeline:<name>] 段落），请求侧用 pipeline=<name> 选一条。
	p.Pipelines = sem.PipelineMap()
	// 结构化字段与原始参数同一套规则：请求侧写了就用请求侧的，否则用模板的。
	// ParseMulti 不会填这两个字段，所以必须显式带过来，否则会静默丢掉请求侧的设置。
	p.NodePref = base.NodePref
	if p.NodePref.Empty() {
		p.NodePref = sem.NodePref
	}
	// 分组：高优先层（base.Groups，来自全局分组库并按方案层顺序排好）在前，
	// 模板分组只补不重名的。旧实现是「base 非空就整体替换模板分组」，用户新建
	// 一个分组就会丢掉模板的几十个分组（D-36），这里改成合并：同名保留高优先层的定义。
	var templateGroups []render.Group
	if !base.DisableTemplateGroups {
		templateGroups = sem.Groups
	}
	if p.DisableTemplateGroups {
		p.TemplateWarnings = append(p.TemplateWarnings, "订阅方案未启用 template 层：模板分组已跳过")
	}
	if p.DisableTemplateRules {
		p.TemplateWarnings = append(p.TemplateWarnings, "模板规则已跳过：订阅方案未启用 template 层，或规则源产物已替换模板规则")
	}
	p.Groups = mergeGroups(base.Groups, templateGroups)
	// 结构化规则（控制台「规则」页启用的规则）同样要带过来：ParseMulti 只认
	// Raw 里的 rules= 参数，base.Rules 是调用方直接赋值给的，不显式合并就会
	// 被模板路径静默丢掉——表现为「规则页明明启用了，订阅里一条都没有」。
	// 优先级仍是「请求 > 模板」，所以放在已解析出的 p.Rules 前面。
	if len(base.Rules) > 0 {
		p.Rules = append(append([]string{}, base.Rules...), p.Rules...)
	}
	// 模板层被关闭时连规则流一起丢掉：TemplateRules 会在 Engine 里被展开成规则行
	// 追加到 p.Rules 末尾，不清空的话「不用模板」的方案仍会带上模板的 MATCH。
	if base.DisableTemplateRules {
		p.TemplateRules = nil
		p.TemplateRemoteRulesets = nil
	}
	return p, nil
}

// mergeGroups 按名称合并两组代理分组：先出现的优先，重名只保留第一份定义。
// 空名字的分组直接丢掉——渲染器不接受无名分组，留着只会在产物里生成 null 项。
func mergeGroups(high, low []render.Group) []render.Group {
	if len(high) == 0 && len(low) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(high)+len(low))
	out := make([]render.Group, 0, len(high)+len(low))
	for _, list := range [][]render.Group{high, low} {
		for _, g := range list {
			if g.Name == "" || seen[g.Name] {
				continue
			}
			seen[g.Name] = true
			out = append(out, g)
		}
	}
	return out
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
		// 零成员分组不能渲染成 proxies: []——mihomo 会拒绝加载整份配置。
		// 与"正则无命中"走同一个兜底：给直连，至少产物可用且行为可预测。
		// subconverter 对零成员分组是直接丢弃；这里选择保留组名并兜底 DIRECT，
		// 因为组名往往被后续规则引用，丢组会让规则指向不存在的出站。
		return []string{"DIRECT"}
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
