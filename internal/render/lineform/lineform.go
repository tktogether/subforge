// Package lineform 实现"一行一节点"族的客户端配置渲染器。
//
// Surge / Quantumult(X) / Loon / Surfboard 都属这一族：节点是形如
// `名字 = 协议字段` 的文本行，加上几个固定段落（General / Proxy / Proxy Group / Rule）。
// 字段顺序与关键名各不相同，但它们共享同一套骨架，所以这里只写一份组装逻辑，
// 每个格式只贡献自己的三行映射函数。
//
// 语法依据不是猜的：取自 subconverter v0.9.0 实机输出（tools/scv/goldens2/，
// GPL-3.0 仅作参照）。渲染器只读 Node.Raw，绝不改写节点（见 docs/adr/0001）。
package lineform

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// RuleOut 是一条规则在目标格式里的落点。
type RuleOut struct {
	// Local 本地规则段的一行（DOMAIN-SUFFIX / GEOIP / FINAL…）。
	Local string
	// Remote 远程规则段的一行（RULE-SET / rule-list / 裸 url…）。
	Remote string
}

// Format 描述一种"一行一节点"的配置格式。
type Format struct {
	// Name 渲染器名（也用作 X-Convert-Format）。
	Name string
	// CommentPrefix 注释前缀，如 "#"、";"、"//!"。
	CommentPrefix string
	// General 配置头段落（[General] 等），不含节点。
	General []string
	// ProxyIntro 节点段落里的伪节点行（如 DIRECT = direct）。
	ProxyIntro []string
	// ProxySection 节点段落的段落名（"Proxy"、"Proxies"…）。
	ProxySection string
	// GroupSection 代理组段落的段落名。
	GroupSection string
	// RuleSection 规则段落的段落名。
	RuleSection string
	// Node 把节点渲染成一行。ok=false 表示该协议在此格式下无法表达，节点应被跳过并告警。
	Node func(n model.Node) (string, bool)
	// Group 把代理组渲染成一行。valid 是产物里真实存在的名字集合，
	// 组用它过滤引用了被跳过节点的成员。返回空串表示该组被跳过（并计入告警）。
	Group func(g render.Group, valid map[string]bool) string
	// RemoteRuleSection 远程规则段落名；空表示该格式把规则集行也混在本地规则段里。
	// Loon 有独立的 [Remote Rule] 段，其余格式没有。
	RemoteRuleSection string
	// Rule 把一条 Clash 语义规则渲染成一行。两个都是空串表示该规则无法表达。
	Rule func(rule string) RuleOut
	// HasSections 决定段落是否用 [Proxy] 这种方括号包裹。
	HasSections bool
}

// Options 控制渲染的可选项。
type Options struct {
	// ManagedConfigURL 非空时在产物首行写 #!MANAGED-CONFIG，供 Surge 自动更新。
	// 转换引擎通常不知道自己的对外 URL，留空即不写。
	ManagedConfigURL string
	// UpdateInterval 自动更新间隔秒数。
	UpdateInterval int
}

// Renderer 是"一行一节点"族的渲染器实现。
type Renderer struct {
	format Format
	opts   Options
	warns  []string
}

// New 按格式构造渲染器。
func New(format Format, opts Options) *Renderer {
	return &Renderer{format: format, opts: opts}
}

func (r *Renderer) Name() string { return r.format.Name }

func (r *Renderer) Render(req render.Request) ([]byte, error) {
	r.warns = nil

	var b strings.Builder
	writeSection := func(name string, lines []string) {
		if len(lines) == 0 {
			return
		}
		if r.format.HasSections {
			b.WriteString("[" + name + "]\n")
		}
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	// 注释头
	if r.format.CommentPrefix != "" {
		head := []string{fmt.Sprintf("SubForge %s 生成", req.Profile.Version)}
		if req.Profile.Name != "" {
			head = append(head, "配置名: "+req.Profile.Name)
		}
		if req.Profile.GeneratedAt != "" {
			head = append(head, "生成时间: "+req.Profile.GeneratedAt)
		}
		for _, l := range head {
			b.WriteString(r.format.CommentPrefix)
			b.WriteByte(' ')
			b.WriteString(l)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	// Managed-CONFIG
	if r.opts.ManagedConfigURL != "" {
		iv := r.opts.UpdateInterval
		if iv <= 0 {
			iv = 86400
		}
		b.WriteString(fmt.Sprintf("#!MANAGED-CONFIG %s interval=%d strict=false\n\n",
			r.opts.ManagedConfigURL, iv))
	}

	// General
	if len(r.format.General) > 0 {
		writeSection(r.format.GeneralSectionName(), r.format.General)
	}

	// 节点
	nodes := make([]string, 0, req.Nodes.Len())
	if r.format.ProxyIntro != nil {
		nodes = append(nodes, r.format.ProxyIntro...)
	}
	// 产物里真实存在的名字：节点 + 伪节点 + 代理组名。
	// 组用它过滤成员，避免产出引用被跳过节点的坏配置。
	valid := make(map[string]bool, req.Nodes.Len()+len(req.Groups)+4)
	for name := range reservedNames {
		valid[name] = true
	}
	for _, g := range req.Groups {
		valid[g.Name] = true
	}
	for _, n := range req.Nodes {
		nline, ok := r.format.Node(n)
		if !ok {
			r.warns = append(r.warns, fmt.Sprintf("节点 %q（%s）在 %s 中无法表达，已跳过",
				label(n), n.Type, r.format.Name))
			continue
		}
		if nline == "" {
			continue
		}
		nodes = append(nodes, nline)
		valid[n.Name] = true
	}

	// 默认组与规则在节点渲染之后注入：它们要引用产物里真实存在的名字，
	// 而哪些节点能被这个格式表达要到渲染完才知道。
	if len(req.Groups) == 0 {
		req.Groups = defaultGroups(valid)
	}
	if len(req.Rules) == 0 {
		req.Rules = defaultRules(req.Groups)
	}

	// 代理组
	groups := make([]string, 0, len(req.Groups))
	for _, g := range req.Groups {
		gline := r.format.Group(g, valid)
		if gline == "" {
			r.warns = append(r.warns, fmt.Sprintf("代理组 %q（%s）在 %s 中无法表达，已跳过",
				g.Name, g.Type, r.format.Name))
			continue
		}
		groups = append(groups, gline)
	}

	// 规则。规则集行在 Loon 里要落到独立的 [Remote Rule] 段，其余格式混在本地规则段。
	rules := make([]string, 0, len(req.Rules))
	remote := make([]string, 0)
	for _, rule := range req.Rules {
		out := r.format.Rule(rule)
		if out.Remote != "" {
			if r.format.RemoteRuleSection != "" {
				remote = append(remote, out.Remote)
				continue
			}
			r.warns = append(r.warns, fmt.Sprintf("规则 %q 需要远程规则段，%s 不支持，已跳过",
				rule, r.format.Name))
			continue
		}
		if out.Local != "" {
			rules = append(rules, out.Local)
			continue
		}
		r.warns = append(r.warns, fmt.Sprintf("规则 %q 在 %s 中无法表达，已跳过", rule, r.format.Name))
	}

	writeSection(r.format.ProxySection, nodes)
	writeSection(r.format.GroupSection, groups)
	writeSection(r.format.RuleSection, rules)
	writeSection(r.format.RemoteRuleSection, remote)

	body := []byte(b.String())
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	return body, nil
}

// Warnings 返回最近一次 Render 中被跳过的节点与组。
func (r *Renderer) Warnings() []string { return r.warns }

// GeneralSectionName 给 General 段落一个统一名字。
func (f Format) GeneralSectionName() string {
	if f.HasSections {
		return "General"
	}
	return ""
}

func label(n model.Node) string {
	if n.Name != "" {
		return n.Name
	}
	return "<unnamed " + n.Type + ">"
}

// defaultGroups 生成最小可用的两组结构（无组配置时的兜底）。
//
// 成员只能引用 valid 里的名字：valid 是"本次产物真实存在的名字"集合，
// 已经过滤掉了该格式无法表达的节点。否则会自动产出一条引用悬空成员的组，
// 客户端整个组直接失效——那比没有组更糟。
// 组名刻意不用 emoji：这几个客户端对组名的字符集限制比 mihomo 严。
func defaultGroups(valid map[string]bool) []render.Group {
	// 只看真实节点名：伪节点不算，否则无节点时也会生成一组只有 DIRECT 的空组。
	// 那种组在客户端里点不开，比不生成更糟。
	names := validNodes(valid)
	if len(names) == 0 {
		return nil
	}
	members := make([]string, 0, len(names)+1)
	if valid["DIRECT"] {
		members = append(members, "DIRECT")
	}
	members = append(members, names...)
	return []render.Group{
		{Name: "节点选择", Type: "select", Proxies: members},
		{Name: "自动测速", Type: "url-test", Proxies: members,
			Test:    "https://www.google.com/generate_204",
			Options: render.UrlTestOptions{Interval: 300, Tolerance: 50, Timeout: 5000}},
	}
}

// defaultRules 生成最小可用的兜底规则。
//
// 收尾规则一律写 FINAL 而不是 MATCH：这是 Surge / QX / Loon / Surfboard 的通用写法，
// mihomo 的 MATCH 是内核专属语法，写进这些客户端的配置不会被识别。
// 没有组时兜底到内置 DIRECT，而不是留空——FINAL,DIRECT 在四种格式里都成立。
func defaultRules(groups []render.Group) []string {
	catchAll := "DIRECT"
	for _, g := range groups {
		if strings.EqualFold(g.Type, "select") {
			catchAll = g.Name
			break
		}
	}
	return []string{"GEOIP,CN,DIRECT", "FINAL," + catchAll}
}

// validNodes 返回 valid 里除内置伪节点外的节点名，按字典序。
// 字典序保证 golden 字节级稳定（map 遍历顺序是随机的）。
func validNodes(valid map[string]bool) []string {
	names := make([]string, 0, len(valid))
	for name := range valid {
		if !reservedNames[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
