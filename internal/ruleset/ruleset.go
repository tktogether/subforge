// Package ruleset 把一批分流规则渲染成六种主流形态。
//
// 规则内容一律由调用方给出（请求的 rules 参数，或本地模板 [common] 的 rules）。
// 第三方规则数据集（Loyalsoldier / blackmatrix7 / ACL4SSR 等）是 CC-BY-SA / GPL 授权，
// 按 R3 约束不静态打包进发行物；需要时由用户显式配置远程源再下载。
//
// 规则形态对齐 mihomo 规则行：一行一条，"TYPE,VALUE[,OVERRIDE]"。
package ruleset

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Kind 是单条规则的形态，取值为规则行第一段的字面量，与 Clash / mihomo
// 的写法完全一致（大写）。直接用字面量当类型值，渲染时不必再做大小写转换。
type Kind string

const (
	Domain        Kind = "DOMAIN"
	DomainSuffix  Kind = "DOMAIN-SUFFIX"
	DomainKeyword Kind = "DOMAIN-KEYWORD"
	DomainRegex   Kind = "DOMAIN-REGEX"
	IPCIDR        Kind = "IP-CIDR"
	IPCIDR6       Kind = "IP-CIDR6"
	GeoIP         Kind = "GEOIP"
	GeoSite       Kind = "GEOSITE"

	// Match / Final 是收尾规则（"MATCH,组名"），provider 形态里没有对应物，
	// 只在 Clash 经典规则段里有意义，因此只有 clash-classical 会保留它们。
	// mihomo 只认 MATCH，FINAL 是 Clash Premium 的叫法——两者都原样放行，
	// 选哪个由配置者按目标客户端决定（见 docs/compatibility.md）。
	Match Kind = "MATCH"
	Final Kind = "FINAL"
)

// kinds 是全部已知形态，供校验与错误信息使用；顺序固定，别当枚举顺序依赖它。
var kinds = []Kind{Domain, DomainSuffix, DomainKeyword, DomainRegex, IPCIDR, IPCIDR6, GeoIP, GeoSite, Match, Final}

func (k Kind) known() bool {
	for _, v := range kinds {
		if k == v {
			return true
		}
	}
	return false
}

// Rule 是一条规则的形态与取值。
type Rule struct {
	Kind  Kind
	Value string
	// Action 是规则行第三段：Clash 里是出站组名，Surge / QX 里是动作关键词。
	// 原样保留而不解释——不同客户端的词表不一样，我们只负责不弄丢。
	// 只有带规则行的形态（Surge list / QX / Clash 经典）会写出它；
	// provider 与 DOMAIN-SET 只表达取值，天然不带动作。
	Action string
}

// String 还原成规则行；没有第三段时不补空字段。
func (r Rule) String() string {
	if r.Action == "" {
		return string(r.Kind) + "," + r.Value
	}
	return string(r.Kind) + "," + r.Value + "," + r.Action
}

// needsOutbound 判断这条规则还缺一个出站。
// MATCH / FINAL 的第二段本身就是出站（"MATCH,组名"），所以它们不算缺。
func (r Rule) needsOutbound() bool {
	return r.Action == "" && r.Kind != Match && r.Kind != Final
}

// Set 是一批规则，已按形态与取值去重排序，保证渲染结果可复现。
type Set []Rule

// NewSet 解析并归并规则行；正则在此编译校验，配置写错能立刻报出来。
func NewSet(lines ...string) (Set, error) {
	out := make(Set, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		r, err := ParseLine(line)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", line, err)
		}
		out = append(out, r)
	}
	return out.Normalize(), nil
}

// ParseLine 解析一行规则。没有逗号且能当域名时按 DOMAIN 处理——
// 公共规则文件（.list）通常是裸域名，直接报语法错误会挡住整批导入。
func ParseLine(line string) (Rule, error) {
	parts := strings.SplitN(line, ",", 3)
	if len(parts) == 1 {
		v := strings.TrimSpace(parts[0])
		if v == "" || !isPlainDomain(v) {
			return Rule{}, fmt.Errorf("无法解析的规则（应为 TYPE,VALUE）")
		}
		return Rule{Kind: Domain, Value: v}, nil
	}
	// 形态名大小写不敏感，统一归一成字面量。
	kind := Kind(strings.ToUpper(strings.TrimSpace(parts[0])))
	value := strings.TrimSpace(parts[1])
	if !kind.known() {
		return Rule{}, fmt.Errorf("未知规则形态 %q（支持: %s）", kind,
			strings.Join(kindNames(), ", "))
	}
	if value == "" {
		return Rule{}, fmt.Errorf("取值不能为空")
	}
	switch kind {
	case DomainRegex:
		if _, err := regexp.Compile(value); err != nil {
			return Rule{}, fmt.Errorf("正则无效: %w", err)
		}
	case GeoIP, GeoSite:
		// 取值是数据集标识，不做正则处理。
	}
	action := ""
	if len(parts) == 3 {
		action = strings.TrimSpace(parts[2])
	}
	return Rule{Kind: kind, Value: value, Action: action}, nil
}

// kindNames 返回形态名的可读列表。
func kindNames() []string {
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, strings.ToUpper(string(k)))
	}
	return out
}

// isPlainDomain 粗判是否像一个域名：不含通配、正则元字符与空白。
// 只用作"裸行按 DOMAIN 处理"的开关，不当域名合法性校验。
func isPlainDomain(v string) bool {
	if v == "" || strings.ContainsAny(v, " \t*?^$|\\()[]+{,") {
		return false
	}
	return !strings.Contains(v, ":")
}

// Normalize 归并重复并固定顺序，让同一批规则的输出逐字节可复现。
func (s Set) Normalize() Set {
	seen := make(map[Rule]struct{}, len(s))
	out := make(Set, 0, len(s))
	for _, r := range s {
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// Dedup 只去掉重复项，保持调用方给定的顺序。
func (s Set) Dedup() Set {
	seen := make(map[Rule]struct{}, len(s))
	out := make(Set, 0, len(s))
	for _, r := range s {
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	return out
}

// Count 返回规则条数。
func (s Set) Count() int { return len(s) }

// Empty 判断是否为空集。
func (s Set) Empty() bool { return len(s) == 0 }

// Filter 按取值正则做白名单 / 黑名单筛选，用于按需瘦身规则集。
func (s Set) Filter(include, exclude []string) (Set, error) {
	inc, err := compileAll(include)
	if err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	exc, err := compileAll(exclude)
	if err != nil {
		return nil, fmt.Errorf("exclude: %w", err)
	}
	out := make(Set, 0, len(s))
	for _, r := range s {
		if matchAny(exc, r.Value) {
			continue
		}
		if len(inc) > 0 && !matchAny(inc, r.Value) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func compileAll(pats []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(pats))
	for _, p := range pats {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func matchAny(pats []*regexp.Regexp, v string) bool {
	for _, re := range pats {
		if re.MatchString(v) {
			return true
		}
	}
	return false
}
