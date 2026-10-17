package config

import (
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/template"
)

// [custom] 内联写法：custom_proxy_group= / ruleset= / rules=
//
// subconverter 的分组与规则有两种等价写法——独立的 [clash_proxy_group] / [ruleset]
// 段落，或者在 [custom] 里用自定义分隔符内联。后者是更流行的那份公开模板
// （Aethersailor Custom_OpenClash_Rules 的 Custom_Clash_Full.ini）用的写法：
// 整份模板 56 个分组和 93 条规则全靠它，只认段落写法等于读不懂这类配置。
//
// 这里全部按观察到的产物行为反推、独立实现，不引用任何第三方实现代码。

// RemoteRuleset 是内联规则里引用的一份远程规则集。Semantics() 是纯解析、
// 没有网络能力，所以这里只登记引用，交给有拉取能力的调用方去展开。
type RemoteRuleset struct {
	// Group 规则命中的代理组名。
	Group string
	// URL 规则集下载地址。
	URL string
	// Kind 内容形态：domain 是 mrs（一行一个域名），classic 是 Clash 经典规则清单。
	Kind string
	// Interval 缓存秒数，由模板声明；是否用于缓存策略由调用方决定。
	Interval int
}

// RuleItem 是模板规则流里的一项：一条直映规则行，或一份待展开的远程规则集。
//
// 顺序是语义的一部分——mihomo 按声明顺序匹配，模板作者把 MATCH 放最后才保证
// 前面的规则都能生效。把"直映行"和"远程规则集"拆成两个切片会丢掉位置信息，
// 展开后就变成了"全部直映行在前、全部展开行在后"，MATCH 之后的规则全部失效。
type RuleItem struct {
	// Line 直映规则行（rules= 的普通行、GEOSITE / GEOIP / MATCH 等）。
	Line string
	// Remote 是 RemoteRulesets 切片下标，表示这一项要拉取并展开；与 Line 二选一。
	// 直映规则行的 Remote 恒为 -1，别依赖 int 的零值判断。
	Remote int
}

// customControl 是 [custom] 里的模板控制键：它们指向分组或规则，但不产生普通参数。
// custom_rules 名字像内联声明，实际是"是否启用自定义规则"的布尔开关，必须挡在这里。
var customControl = map[string]bool{
	"custom_rules":             true,
	"custom_proxy_group":       true,
	"rules":                    true,
	"ruleset":                  true,
	"enable_rule_generator":    true,
	"overwrite_original_rules": true,
}

// inlineGroups 取全部段落里的 custom_proxy_group= 内联分组定义，按出现顺序返回。
func inlineGroups(cfg *template.Config) []render.Group {
	var out []render.Group
	for _, name := range cfg.SectionNames() {
		s := cfg.Section(name)
		if s == nil {
			continue
		}
		for _, e := range s.Entries() {
			if strings.ToLower(strings.TrimSpace(e.Key)) != "custom_proxy_group" {
				continue
			}
			v := strings.TrimSpace(e.Value.Raw)
			if v == "" {
				continue
			}
			if g, ok := groupFromInline(v); ok {
				out = append(out, g)
			}
		}
	}
	return out
}

// groupFromInline 解释一条 custom_proxy_group= 定义。
//
// 语法：<组名>`<类型>`<成员>`<成员>`…[`<测活地址>`<参数>]
// 成员前的 [] 表示引用另一个组；不带 [] 的是节点名正则。
// 测活地址之后的参数段只有 url-test / fallback 会读，select 组带了也会被丢弃。
func groupFromInline(v string) (render.Group, bool) {
	segs := strings.Split(v, "`")
	if len(segs) < 2 {
		return render.Group{}, false
	}
	name := strings.TrimSpace(segs[0])
	typ := strings.ToLower(strings.TrimSpace(segs[1]))
	if name == "" {
		return render.Group{}, false
	}
	if typ == "" {
		typ = "select"
	}

	rest := segs[2:]
	// 测活地址是最后一个"绝对 URL"段，它及之后都属于测活参数。
	// 判定必须要求 scheme（http:// 或 https://）：成员段常写成正则，
	// `http.*` 这种以 "http" 开头的成员正则一旦被误判成测活地址，
	// 它以及它之后的成员（如 []DIRECT）会整段被吞掉，组直接变成零成员。
	urlIdx := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if isAbsoluteURL(rest[i]) {
			urlIdx = i
			break
		}
	}
	members := rest
	if urlIdx >= 0 {
		members, rest = rest[:urlIdx], rest[urlIdx:]
	}

	g := render.Group{Name: name, Type: typ}
	for _, seg := range members {
		m := strings.TrimPrefix(strings.TrimSpace(seg), "[]")
		if m == "" {
			continue
		}
		g.Proxies = append(g.Proxies, m)
	}
	if typ == "url-test" || typ == "fallback" {
		if len(rest) >= 1 {
			g.Test = strings.TrimSpace(rest[0])
		}
		if len(rest) >= 2 {
			// 参数段是逗号分隔：测活间隔, 中段, 容忍延迟毫秒。
			// 中段按模板作者的意图留空表示不设上限，当前模型不映射该字段。
			fields := strings.Split(rest[1], ",")
			if n, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil && n > 0 {
				g.Options.Interval = n
			}
			if len(fields) >= 3 {
				if n, err := strconv.Atoi(strings.TrimSpace(fields[2])); err == nil && n > 0 {
					g.Options.Tolerance = n
				}
			}
		}
	}
	return g, true
}

// isAbsoluteURL 判定一段文本是否是带 scheme 的绝对 URL。
// 只认 http:// 与 https://：subconverter 的测活地址就是这两种，别的 scheme
// 在这个位置没有意义，而宽松前缀匹配会误伤以 http 开头的成员正则。
func isAbsoluteURL(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// inlineRules 取全部段落里的内联规则，按段落与条目顺序排成一条规则流。
//
// 直映部分（GEOSITE / GEOIP / FINAL）立刻展开成 mihomo 规则行；引用远程文件的规则集
// 没法在纯解析阶段展开，作为 RemoteRuleset 登记并把引用位置留在规则流里。
// [common] 里的 rules= 也在这里收口，不要在调用方再取一遍——会重复。
func inlineRules(cfg *template.Config) (items []RuleItem, remotes []RemoteRuleset) {
	for _, name := range cfg.SectionNames() {
		s := cfg.Section(name)
		if s == nil {
			continue
		}
		for _, e := range s.Entries() {
			key := strings.ToLower(strings.TrimSpace(e.Key))
			v := strings.TrimSpace(e.Value.Raw)
			switch {
			case key == "rules" && v != "":
				// 一行可以带分号分隔的多条经典规则。
				for _, part := range strings.Split(v, ";") {
					if p := strings.TrimSpace(part); p != "" {
						items = append(items, RuleItem{Line: p, Remote: -1})
					}
				}
			case key == "ruleset" && v != "":
				group, kind, body, flag, interval, ok := splitRulesetSpec(v)
				if !ok {
					continue
				}
				switch kind {
				case "FINAL":
					items = append(items, RuleItem{Line: ruleLine("MATCH", group, "", ""), Remote: -1})
				case "GEOSITE":
					items = append(items, RuleItem{Line: ruleLine("GEOSITE", body, group, flag), Remote: -1})
				case "GEOIP":
					// 规则集不查域名，没写 no-resolve 时默认补上——否则 mihomo 会把
					// 整个数据集当成要做 DNS 解析的目标，规则既慢又不生效。
					if flag == "" {
						flag = "no-resolve"
					}
					items = append(items, RuleItem{Line: ruleLine("GEOIP", body, group, flag), Remote: -1})
				case "domain", "classic":
					remotes = append(remotes, RemoteRuleset{
						Group: group, URL: body, Kind: kind, Interval: interval,
					})
					items = append(items, RuleItem{Remote: len(remotes) - 1})
				}
			}
		}
	}
	return items, remotes
}

// splitRulesetSpec 拆一条 ruleset= 定义：<组名>,<类型>[,<取值>][,<附加>]
//
// 类型分两类：[] 前缀是内置规则集（GEOSITE / GEOIP / FINAL），
// 冒号前缀是远程规则集（clash-domain: / clash-classic:）。
// 第四段在两类里语义不同：内置规则集的是 no-resolve 之类的标志位，
// 远程规则集的是缓存秒数。clash- 只是 subconverter 标记目标客户端，
// 不是内容形态的一部分，这里剥掉。
func splitRulesetSpec(v string) (group, kind, body, flag string, interval int, ok bool) {
	parts := strings.SplitN(v, ",", 4)
	if len(parts) < 2 {
		return "", "", "", "", 0, false
	}
	group = strings.TrimSpace(parts[0])
	spec := strings.TrimSpace(parts[1])
	if group == "" || spec == "" {
		return "", "", "", "", 0, false
	}
	if strings.HasPrefix(spec, "[]") {
		kind = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(spec, "[]")))
		if len(parts) >= 3 {
			body = strings.TrimSpace(parts[2])
		}
		if len(parts) >= 4 {
			flag = strings.TrimSpace(parts[3])
		}
		return group, kind, body, flag, 0, true
	}
	i := strings.Index(spec, ":")
	if i <= 0 {
		return "", "", "", "", 0, false
	}
	kind = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(spec[:i])), "clash-")
	body = strings.TrimSpace(spec[i+1:])
	if body == "" {
		return "", "", "", "", 0, false
	}
	// 远程规则集把地址写在类型里，所以缓存秒数落在第三段而不是第四段。
	if len(parts) >= 3 {
		interval, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	}
	return group, kind, body, "", interval, true
}

// ruleLine 拼一条 mihomo 规则行：形态,取值,命中组[,附加]
// MATCH / FINAL 的第二段本身就是出站名，没有取值，传空即可。
func ruleLine(kind, value, group, flag string) string {
	parts := []string{kind, value}
	if group != "" {
		parts = append(parts, group)
	}
	if flag != "" {
		parts = append(parts, flag)
	}
	return strings.Join(parts, ",")
}
