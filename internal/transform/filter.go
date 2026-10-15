package transform

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tktogether/subforge/internal/model"
)

// RegexFilter 按节点名正则做白名单/黑名单过滤（对应 Sub-Store 的 filter.regex）。
// 语义：先排除，再白名单；两项都为空时不过滤（保留全部）。
type RegexFilter struct {
	Include []*regexp.Regexp
	Exclude []*regexp.Regexp
}

// NewRegexFilter 构造正则过滤器，构造期即编译正则。
func NewRegexFilter(include, exclude []string) (RegexFilter, error) {
	inc, err := compilePatterns(include)
	if err != nil {
		return RegexFilter{}, fmt.Errorf("include: %w", err)
	}
	exc, err := compilePatterns(exclude)
	if err != nil {
		return RegexFilter{}, fmt.Errorf("exclude: %w", err)
	}
	return RegexFilter{Include: inc, Exclude: exc}, nil
}

func (f RegexFilter) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(f.Include) == 0 && len(f.Exclude) == 0 {
		return ns, nil
	}
	return ns.Filter(func(n model.Node) bool {
		if matchAny(f.Exclude, n.Name) {
			return false
		}
		if len(f.Include) > 0 && !matchAny(f.Include, n.Name) {
			return false
		}
		return true
	}), nil
}

// RegionFilter 按地区关键词过滤节点名（对应 Sub-Store 的 filter.region）。
// v0 用大小写不敏感的子串匹配（"hk" 匹配 "HK-香港"）；
// 基于 GeoIP/GeoLite2 的城市级解析留到 M3 的 F4 地理定位能力。
type RegionFilter struct {
	Include []string
	Exclude []string
}

// NewRegionFilter 构造地区过滤器；地区词一律小写化。
func NewRegionFilter(include, exclude []string) (RegionFilter, error) {
	return RegionFilter{
		Include: normalizeKeywords(include),
		Exclude: normalizeKeywords(exclude),
	}, nil
}

func (f RegionFilter) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(f.Include) == 0 && len(f.Exclude) == 0 {
		return ns, nil
	}
	return ns.Filter(func(n model.Node) bool {
		lower := strings.ToLower(n.Name)
		for _, ex := range f.Exclude {
			if strings.Contains(lower, ex) {
				return false
			}
		}
		if len(f.Include) == 0 {
			return true
		}
		for _, in := range f.Include {
			if strings.Contains(lower, in) {
				return true
			}
		}
		return false
	}), nil
}

// TypeFilter 按协议类型过滤（对应 Sub-Store 的 filter.type）。
// 类型名大小写不敏感；Include 与 Exclude 同时给出时 Exclude 优先。
type TypeFilter struct {
	Include map[string]bool
	Exclude map[string]bool
}

// kernelTypes 是 mihomo 实际写进节点 type 字段的取值（converter.go 里基本都是
// `x["type"] = scheme`，少数硬编码：hysteria2 / socks5 / anytls / mieru）。
// 用户按 "shadowsocks" 这种常见长名写 -type 时会一个节点都匹配不上、拿到空产物，
// 所以别名在构造期就归一掉；匹配端仍只认内核真名。mieru 是 mihomo 自己的拼写。
var kernelTypes = map[string]string{
	"shadowsocks":     "ss",
	"shadowsocksr":    "ssr",
	"socks":           "socks5",
	"socks4":          "socks5",
	"socks5h":         "socks5",
	"https":           "http",
	"hy2":             "hysteria2",
	"hysteria2+realm": "hysteria2",
	"mierus":          "mieru",
	"v2ray":           "vmess",
}

// NewTypeFilter 构造协议过滤器。
func NewTypeFilter(include, exclude []string) (TypeFilter, error) {
	return TypeFilter{
		Include: keywordSet(include, normalizeType),
		Exclude: keywordSet(exclude, normalizeType),
	}, nil
}

func (f TypeFilter) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(f.Include) == 0 && len(f.Exclude) == 0 {
		return ns, nil
	}
	return ns.Filter(func(n model.Node) bool {
		typ := strings.ToLower(n.Type)
		if f.Exclude[typ] {
			return false
		}
		if len(f.Include) == 0 {
			return true
		}
		return f.Include[typ]
	}), nil
}

// Remove 按节点名精确移除（对应 Sub-Store 的 node_pref.remove）。
type Remove struct {
	Names map[string]bool
}

// NewRemove 构造按名移除的步骤。
func NewRemove(names []string) Remove {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			set[n] = true
		}
	}
	return Remove{Names: set}
}

func (r Remove) Apply(ns model.NodeList) (model.NodeList, error) {
	if len(r.Names) == 0 {
		return ns, nil
	}
	return ns.Filter(func(n model.Node) bool { return !r.Names[n.Name] }), nil
}

func normalizeKeywords(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func keywordSet(in []string, fold ...func(string) string) map[string]bool {
	set := make(map[string]bool, len(in))
	for _, s := range normalizeKeywords(in) {
		for _, f := range fold {
			s = f(s)
		}
		set[s] = true
	}
	return set
}

// normalizeType 把用户写的类型名折到内核真名。
func normalizeType(s string) string {
	if aliased, ok := kernelTypes[s]; ok {
		return aliased
	}
	return s
}
