// Package convert 统一一次"转换请求"的语义：参数解析与管线组装。
//
// CLI 与 HTTP API 共用这一层，避免两处各写一套解析与装配逻辑。
// 参数名与取值约定对齐 subconverter / SCE，现有订阅脚本与客户端配置可原样使用。
package convert

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/transform"
)

// Params 是一次转换请求的解析结果。
type Params struct {
	// Target 输出格式（clash.meta / surge / v2ray …）。
	Target string
	// URLs 订阅地址列表。
	URLs []string
	// Files 本地订阅或配置文件（CLI 入口使用，HTTP API 不填）。
	Files []string
	// Insert 额外插入的订阅地址。
	Insert []string
	// Include / Exclude 节点名正则。
	Include []string
	Exclude []string
	// Types 仅保留这些协议类型（与 Include/Exclude 是"与"关系）。
	Types []string
	// Rename / Emoji 正则替换规则。
	Rename []transform.RegexPair
	Emoji  []transform.RegexPair
	// NodePref 节点名前缀规则（[node_pref] 段落）。
	NodePref transform.NodePrefSpec
	// Groups 代理组定义（[clash_proxy_group] 段落）。
	Groups []render.Group
	// Rules 规则行（原样透传，如 "GEOIP,CN,DIRECT"）。
	Rules []string
	// List 仅输出分享链接列表。
	List bool
	// UDP 节点启用 udp 转发。
	UDP bool
	// TFO 启用 TCP Fast Open。
	TFO bool
	// SCV 输出 scv 扩展字段。
	SCV bool
	// TLS13 强制 TLS 1.3。
	TLS13 bool
	// Sort 排序键。
	Sort string
	// Config 模板名。
	Config string
	// Headers 附加到上游订阅请求的请求头（token 等）。
	// 不走 query：凭证不该出现在 URL 里，由调用方按来源设置。
	Headers map[string]string
	// Ruleset 规则集键值（ruleset:<name>:<type> 形式，I6 完整实现）。
	Ruleset map[string]string
	// Raw 全部原始参数（含未知项），便于排查与透传。
	Raw map[string][]string
	// Extensions 未识别的 x-* 扩展参数（预留扩展点）。
	Extensions map[string][]string
}

// HasSource 表示是否给出了至少一个订阅来源（URL 或本地文件）。
func (p Params) HasSource() bool { return len(p.URLs) > 0 || len(p.Files) > 0 }

func (p Params) HasURL() bool { return len(p.URLs) > 0 }

// ParseQuery 解析 HTTP query string。
func ParseQuery(q url.Values) (Params, error) {
	return ParseMulti(q)
}

// ParseMulti 解析 "参数名 -> 多个取值" 形式的输入。
// url.Values 本身就是这个形状；CLI 与 INI 也归一到这个形状再进来。
func ParseMulti(m map[string][]string) (Params, error) {
	p := Params{
		Ruleset:    map[string]string{},
		Raw:        map[string][]string{},
		Extensions: map[string][]string{},
	}
	for k, vals := range m {
		key := strings.ToLower(strings.TrimSpace(k))
		// 原始参数按小写键归并，同一 key 出现多次时值顺序保留。
		p.Raw[key] = append(p.Raw[key], vals...)
		if strings.HasPrefix(key, "x-") {
			p.Extensions[key] = append(p.Extensions[key], vals...)
			continue
		}
	}

	target := single(p.Raw, "target")
	if target != "" {
		p.Target = strings.ToLower(strings.TrimSpace(target))
	}
	p.URLs = take(p.Raw, "url", "?", ";")
	p.Insert = take(p.Raw, "insert", "?", ";")
	p.Include = take(p.Raw, "include", ";")
	p.Exclude = take(p.Raw, "exclude", ";")
	// type 常见写法既用逗号也用分号（subconverter 与 Sub-Store 各占一半），两者都吃。
	p.Types = take(p.Raw, "type", ",", ";")
	// include/exclude 的正则在这里就编译校验一遍，配置写错不用等到请求时才暴露。
	if len(p.Include) > 0 || len(p.Exclude) > 0 {
		if _, err := transform.NewRegexFilter(p.Include, p.Exclude); err != nil {
			return Params{}, err
		}
	}
	sort := single(p.Raw, "sort")
	if sort != "" {
		p.Sort = strings.ToLower(strings.TrimSpace(sort))
	}
	p.List = isTrue(p.Raw, "list")
	p.UDP = isTrue(p.Raw, "udp")
	p.TFO = isTrue(p.Raw, "tfo")
	p.SCV = isTrue(p.Raw, "scv")
	p.TLS13 = isTrue(p.Raw, "tls13")
	p.Config = single(p.Raw, "config")
	// 规则行按分号分隔；一行一条，形如 "GEOIP,CN,DIRECT"。
	p.Rules = take(p.Raw, "rules", ";")

	// rename/emoji 的正则在这里就编译校验一遍，配置写错不用等到请求时才暴露。
	if p.Rename = pairs(p.Raw, "rename"); len(p.Rename) > 0 {
		if _, err := transform.NewRename(p.Rename); err != nil {
			return Params{}, fmt.Errorf("rename: %w", err)
		}
	}
	if p.Emoji = pairs(p.Raw, "emoji"); len(p.Emoji) > 0 {
		if _, err := transform.NewEmoji(p.Emoji); err != nil {
			return Params{}, fmt.Errorf("emoji: %w", err)
		}
	}

	for _, v := range p.Raw["ruleset"] {
		if v == "" {
			continue
		}
		// ruleset 形如 ruleset:规则集名:类型，允许一条参数里带多条分号分隔的规则。
		for _, item := range strings.Split(v, ";") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			name, typ, ok := strings.Cut(item, ":")
			if !ok || name == "" {
				return Params{}, fmt.Errorf("ruleset 参数 %q 缺少类型（应为 ruleset:<名称>:<类型>）", item)
			}
			p.Ruleset[name] = typ
		}
	}

	return p, nil
}

// BuildPipeline 把参数映射为 filter -> op 管线。
// 正则与排序键在这里就编译，配置写错能在拉取订阅之前暴露出来。
func BuildPipeline(p Params) (transform.Pipeline, error) {
	regex, err := transform.NewRegexFilter(p.Include, p.Exclude)
	if err != nil {
		return nil, err
	}
	steps := []transform.Stage{regex}

	if len(p.Types) > 0 {
		typeFilter, err := transform.NewTypeFilter(p.Types, nil)
		if err != nil {
			return nil, err
		}
		steps = append(steps, typeFilter)
	}

	if !p.NodePref.Empty() {
		nodePref, err := transform.NewNodePref(p.NodePref)
		if err != nil {
			return nil, err
		}
		steps = append(steps, nodePref)
	}
	// 前缀必须排在 rename / emoji 前面：它按协议类型加前缀，而 include / exclude
	// 是按节点名判定的。先改名会让判定基准漂移，一个本来该加前缀的节点就静默漏掉了；
	// 对齐 subconverter 也在改名之前处理前缀。改名之后再动前缀也能被后续规则继续改。
	if len(p.Rename) > 0 {
		rename, err := transform.NewRename(p.Rename)
		if err != nil {
			return nil, fmt.Errorf("rename: %w", err)
		}
		steps = append(steps, rename)
	}
	if len(p.Emoji) > 0 {
		emoji, err := transform.NewEmoji(p.Emoji)
		if err != nil {
			return nil, fmt.Errorf("emoji: %w", err)
		}
		steps = append(steps, emoji)
	}
	// 去重按 server+port+type 指纹判重（与名称无关），对齐 Sub-Store 的 op.dedupe。
	steps = append(steps, transform.Dedupe{})
	if p.Sort != "" && p.Sort != string(transform.SortByOriginal) {
		sort, err := transform.NewSort(transform.SortBy(p.Sort))
		if err != nil {
			return nil, err
		}
		steps = append(steps, sort)
	}
	return transform.Pipeline(steps), nil
}

// single 取某参数的第一个非空值。
func single(raw map[string][]string, key string) string {
	for _, v := range raw[key] {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// isTrue 判定布尔参数。1 / true / yes / on 视为真，其余一律假。
func isTrue(raw map[string][]string, key string) bool {
	v := single(raw, key)
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// take 取某参数并按多个分隔符拆开。
func take(raw map[string][]string, key string, seps ...string) []string {
	return SplitMulti(raw[key], seps...)
}

// SplitMulti 把一组取值按多个分隔符拆开并去掉空值。
// CLI 的重复 flag、INI 的值列表都归一成同一个形状再交给 Params。
func SplitMulti(vals []string, seps ...string) []string {
	out := []string{}
	for _, v := range vals {
		parts := []string{v}
		for _, sep := range seps {
			next := []string{}
			for _, part := range parts {
				next = append(next, strings.Split(part, sep)...)
			}
			parts = next
		}
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// pairs 解析 "正则,替换;正则,替换" 形式的规则参数。
func pairs(raw map[string][]string, key string) []transform.RegexPair {
	out := []transform.RegexPair{}
	for _, v := range raw[key] {
		for _, item := range strings.Split(v, ";") {
			// 不对 item 整体 TrimSpace：正则的首尾空格是有语义的（如 ` - .*` 的分隔空格），
			// 只以 TrimSpace 结果判空，正则本身按原文保留。
			if strings.TrimSpace(item) == "" {
				continue
			}
			pat, repl, _ := strings.Cut(item, ",")
			// 没有逗号时 replacement 就是空串，语义为"仅保留匹配"。
			if strings.TrimSpace(pat) == "" {
				continue
			}
			out = append(out, transform.RegexPair{Pattern: pat, Replacement: repl})
		}
	}
	return out
}
