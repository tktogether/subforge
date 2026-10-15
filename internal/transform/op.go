package transform

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tktogether/subforge/internal/model"
)

// Dedupe 按节点身份指纹去重（type|server|port|凭据），保留首次出现者。
type Dedupe struct{}

func (Dedupe) Apply(ns model.NodeList) (model.NodeList, error) {
	return ns.Dedupe(), nil
}

// RegexPair 是一条正则替换规则（pattern 与 replacement 均来自配置文本）。
type RegexPair struct {
	Pattern     string
	Replacement string
}

// regexRule 是一条已编译的正则替换规则（Rename 与 Emoji 共用）。
type regexRule struct {
	re   *regexp.Regexp
	repl string
	pat  string
}

// compileRules 编译一组替换规则；仅以 TrimSpace 判空，正则本身按原文编译——
// 正则里的空格有语义（如 ` - 10Gbps` 的分隔空格），值两侧多余空白由配置层去掉。
func compileRules(kind string, pairs []RegexPair) ([]regexRule, error) {
	out := make([]regexRule, 0, len(pairs))
	for _, p := range pairs {
		if strings.TrimSpace(p.Pattern) == "" {
			continue
		}
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s 正则 %q 编译失败: %w", kind, p.Pattern, err)
		}
		out = append(out, regexRule{re: re, repl: p.Replacement, pat: p.Pattern})
	}
	return out, nil
}

// Rename 按顺序对节点名做正则替换（对应 Sub-Store 的 op.rename）。
// 规则之间串行作用：后一条看到的名字是前一条改完之后的结果。
type Rename struct {
	Rules []regexRule
}

// NewRename 构造重命名步骤，构造期即编译正则。
func NewRename(pairs []RegexPair) (Rename, error) {
	rules, err := compileRules("重命名", pairs)
	if err != nil {
		return Rename{}, err
	}
	return Rename{Rules: rules}, nil
}

func (r Rename) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(r.Rules) == 0 {
		return ns, nil
	}
	out := make(model.NodeList, len(ns))
	for i, n := range ns {
		name := n.Name
		for _, rule := range r.Rules {
			name = rule.re.ReplaceAllString(name, rule.repl)
		}
		out[i] = withName(n, name)
	}
	return out, nil
}

// Emoji 按顺序匹配节点名并前置国旗/图标（对应 Sub-Store 的 op.emoji），
// 第一条命中的规则生效；全部未命中则名字不变。
type Emoji struct {
	Rules []regexRule
}

// NewEmoji 构造 emoji 步骤，构造期即编译正则。
func NewEmoji(pairs []RegexPair) (Emoji, error) {
	rules, err := compileRules("emoji", pairs)
	if err != nil {
		return Emoji{}, err
	}
	return Emoji{Rules: rules}, nil
}

func (e Emoji) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(e.Rules) == 0 {
		return ns, nil
	}
	out := make(model.NodeList, len(ns))
	for i, n := range ns {
		name := n.Name
		for _, rule := range e.Rules {
			if rule.re.MatchString(name) {
				name = rule.repl + name
				break
			}
		}
		out[i] = withName(n, name)
	}
	return out, nil
}

// SortBy 指定排序键。
type SortBy string

const (
	SortByOriginal SortBy = "original" // 保持原顺序（不做任何事）
	SortByName     SortBy = "name"     // 按名称字典序
	SortByType     SortBy = "type"     // 按协议类型（同类型按名称）
	SortByServer   SortBy = "server"   // 按服务器地址
	SortByPort     SortBy = "port"     // 按端口
)

// Sort 稳定排序（对应 Sub-Store 的 op.sort）。
type Sort struct {
	By SortBy
}

// NewSort 构造排序步骤，未知排序键直接报错（配置拼写错误早暴露）。
func NewSort(by SortBy) (Sort, error) {
	switch by {
	case SortByOriginal, SortByName, SortByType, SortByServer, SortByPort:
		return Sort{By: by}, nil
	default:
		return Sort{}, fmt.Errorf("未知排序键 %q（可选: original/name/type/server/port）", by)
	}
}

func (s Sort) Apply(ns model.NodeList) (model.NodeList, error) {
	if s.By == SortByOriginal || ns.Len() < 2 {
		return ns, nil
	}
	out := make(model.NodeList, ns.Len())
	copy(out, ns)
	var less func(i, j int) bool
	switch s.By {
	case SortByName:
		less = func(i, j int) bool { return out[i].Name < out[j].Name }
	case SortByType:
		less = func(i, j int) bool {
			if out[i].Type != out[j].Type {
				return out[i].Type < out[j].Type
			}
			return out[i].Name < out[j].Name
		}
	case SortByServer:
		less = func(i, j int) bool { return out[i].Server < out[j].Server }
	case SortByPort:
		less = func(i, j int) bool {
			a, _ := out[i].PortInt()
			b, _ := out[j].PortInt()
			return a < b
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return less(i, j) })
	return out, nil
}

// NodePrefSpec 描述"按协议类型给节点名加前缀"的完整配置。
// 放在 transform 而不是 config，是因为它描述的就是一条管线步骤的语义——
// 配置层与参数层都引用它，避免两边各定义一份。
type NodePrefSpec struct {
	// Prefixes 协议类型（小写）-> 前缀；前缀原样拼接，是否带空格由配置自己写。
	Prefixes map[string]string
	// Include 仅对名称匹配这些正则的节点生效。
	Include []string
	// Exclude 名称匹配这些正则的节点不生效（优先于 Include）。
	Exclude []string
}

// Empty 判断配置是否什么都没写。
func (s NodePrefSpec) Empty() bool {
	return len(s.Prefixes) == 0 && len(s.Include) == 0 && len(s.Exclude) == 0
}

// NodePref 按协议类型给节点名前加前缀（对应 Subconverter 的 [node_pref] 段落）。
// 典型用法：vmess = [V2Ray] 让节点显示为 "[V2Ray]HK-香港-01"，便于客户端按协议分组。
// Include/Exclude 按节点名限定哪些节点受影响（与 RegexFilter 同款语义：先排除再白名单）。
type NodePref struct {
	Prefixes map[string]string
	include  []*regexp.Regexp
	exclude  []*regexp.Regexp
}

// NewNodePref 构造节点前缀步骤，构造期即编译正则。
func NewNodePref(spec NodePrefSpec) (NodePref, error) {
	inc, err := compilePatterns(spec.Include)
	if err != nil {
		return NodePref{}, fmt.Errorf("node_pref include: %w", err)
	}
	exc, err := compilePatterns(spec.Exclude)
	if err != nil {
		return NodePref{}, fmt.Errorf("node_pref exclude: %w", err)
	}
	return NodePref{Prefixes: spec.Prefixes, include: inc, exclude: exc}, nil
}

func (n NodePref) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(n.Prefixes) == 0 {
		return ns, nil
	}
	out := make(model.NodeList, len(ns))
	for i, node := range ns {
		name := node.Name
		if n.appliesTo(name) {
			if prefix := n.Prefixes[strings.ToLower(node.Type)]; prefix != "" {
				name = prefix + name
			}
		}
		out[i] = withName(node, name)
	}
	return out, nil
}

func (n NodePref) appliesTo(name string) bool {
	if matchAny(n.exclude, name) {
		return false
	}
	if len(n.include) > 0 && !matchAny(n.include, name) {
		return false
	}
	return true
}

// Split 按分隔符截断节点名（对应 Sub-Store 的 node_pref.split）。
// "HK-香港-01 - 10Gbps" 用 " - " 截断为 "HK-香港-01"，便于按地区建组。
type Split struct {
	Sep string
}

// NewSplit 构造截断步骤；空分隔符视为 " - "。
func NewSplit(sep string) Split {
	if strings.TrimSpace(sep) == "" {
		sep = " - "
	}
	return Split{Sep: sep}
}

func (s Split) Apply(ns model.NodeList) (model.NodeList, error) {
	out := make(model.NodeList, len(ns))
	for i, n := range ns {
		if idx := strings.Index(n.Name, s.Sep); idx > 0 {
			out[i] = withName(n, strings.TrimSpace(n.Name[:idx]))
		} else {
			out[i] = n
		}
	}
	return out, nil
}
