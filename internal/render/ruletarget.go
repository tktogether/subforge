// Package render 的规则目标校正。
//
// 规则行的第三个字段是「出站」——也就是代理组名或节点名。内核加载配置时会逐条
// 校验这个引用，找不到就整份配置拒绝加载（mihomo 的报错形如
// `rules[1] [DOMAIN-SUFFIX,x,GROUP] error: proxy [GROUP] not found`）。
//
// 而规则与分组是两个独立来源：机场订阅自带的规则指向机场自己的分组名，用户叠加
// 的 INI 模板又定义了另一套分组名。两者拼在一起时，只要有一个分组名对不上，整
// 份产物就废了。要求用户先手工建分组、或者逐条改规则，是不现实的，所以这里在
// 渲染阶段做一次校正：目标不存在时先按名字相似度找同义分组，找不到就落到兜底
// 分组（MATCH 的目标），并给出警告——降级不能静默。
package render

import (
	"fmt"
	"strings"
	"unicode"
)

// builtinTargets 是内核内置的出站，不需要在 proxy-groups 里定义。
var builtinTargets = map[string]bool{
	"DIRECT":      true,
	"REJECT":      true,
	"REJECT-DROP": true,
	"PASS":        true,
	"COMPATIBLE":  true,
}

// ResolveRuleTargets 校正规则行的出站引用，返回校正后的规则与警告。
//
// valid 是产物里真实存在的出站名字集合（代理组名 + 节点名）。某条规则的目标既
// 不在 valid 里也不是内核内置出站时，按下面的顺序找替代：
//
//  1. 名字归一化后完全相同的分组（"📺 Netflix" 与 "🎥 Netflix" 归一化都是
//     "netflix"，是同一个意思）；
//  2. 一方是另一方子串、且较短的一方至少 4 个字符的分组（"Emby影院" 与
//     "Emby"）；
//  3. 兜底分组成员：MATCH 规则的目标；
//  4. 产物里第一个分组；连分组都没有时才退到 DIRECT。
//
// 被改写与找不到替代的规则都会产生一条警告，调用方要把警告回传给用户。
func ResolveRuleTargets(rules []string, valid []string, validSet map[string]bool) ([]string, []string) {
	if len(rules) == 0 {
		return rules, nil
	}
	if validSet == nil {
		validSet = make(map[string]bool, len(valid))
		for _, n := range valid {
			validSet[n] = true
		}
	}

	// 兜底出站：优先取 MATCH 的目标（那是所有未命中规则的最终去向），
	// 它自己也得是有效的才值得当兜底。
	fallback := ""
	for _, line := range rules {
		fields := splitRuleFields(line)
		if len(fields) < 2 || !strings.EqualFold(strings.TrimSpace(fields[0]), "MATCH") {
			continue
		}
		if t := strings.TrimSpace(fields[1]); validSet[t] || builtinTargets[t] {
			fallback = t
			break
		}
	}
	if fallback == "" && len(valid) > 0 {
		fallback = valid[0]
	}
	if fallback == "" {
		fallback = "DIRECT"
	}

	out := make([]string, 0, len(rules))
	var warns []string
	rewritten := 0
	unresolved := 0
	var samples []string

	for _, line := range rules {
		fields := splitRuleFields(line)
		if len(fields) < 2 {
			out = append(out, line)
			continue
		}
		typ := strings.ToUpper(strings.TrimSpace(fields[0]))
		idx := -1
		switch {
		case typ == "MATCH" || typ == "FINAL":
			idx = 1
			// FINAL 是旧语法，mihomo v1.19.31 的 parser 里没有这个类型，
			// 写 FINAL 会报 format invalid，统一改写成 MATCH。
			if typ == "FINAL" {
				fields[0] = "MATCH"
			}
		case typ == "AND" || typ == "OR" || typ == "NOT" || typ == "SUB-RULE":
			idx = len(fields) - 1
		default:
			// 标准形态：TYPE,payload,target[,params...]
			if len(fields) >= 3 {
				idx = 2
			}
		}
		if idx < 0 || idx >= len(fields) {
			out = append(out, line)
			continue
		}

		target := strings.TrimSpace(fields[idx])
		if target == "" || validSet[target] || builtinTargets[target] {
			fields[idx] = target
			out = append(out, joinRuleFields(fields))
			continue
		}

		replacement := resolveBySimilarName(target, valid)
		if replacement == "" {
			replacement = resolveBySynonym(target, valid, fallback)
		}
		if replacement == "" {
			replacement = fallback
			unresolved++
		} else {
			rewritten++
		}
		fields[idx] = replacement
		out = append(out, joinRuleFields(fields))
		if len(samples) < 3 {
			samples = append(samples, fmt.Sprintf("%s→%s", target, replacement))
		}
	}

	if rewritten > 0 {
		warns = append(warns, fmt.Sprintf(
			"%d 条规则的目标按名称相似度映射到了已有分组（示例：%s）",
			rewritten, strings.Join(samples, "、")))
	}
	if unresolved > 0 {
		warns = append(warns, fmt.Sprintf(
			"%d 条规则的目标分组在产物里不存在，已指向默认分组 %q；如需精确分流请在「规则」页改这些规则的目标",
			unresolved, fallback))
	}
	return out, warns
}

// synonymRules 是「同一个意思换个说法」的对照表。
//
// 机场与公开模板是两套人写的配置，同一类流量往往一边写英文、一边写中文：
// 机场叫 "Apple"，模板叫 "🍎 苹果服务"；机场叫 "国内直连"，模板叫 "🎯 全球直连"。
// 光靠名字相似度匹配不到，但语义是明确的，所以按关键词给一张小表。
//
// builtin 非空表示直接落到内核内置出站（如广告类 → REJECT）；
// special 为 "fallback" 表示用兜底分组（"其他/默认"这类本来就不指定分组的名字）。
type synonymRule struct {
	keys    []string
	wants   []string // 候选分组名里要出现的关键词（归一化后）
	builtin string
	// fallbackBuiltin 在候选分组里找不到语义分组时使用（如「直连」没有对应
	// 分组时应落到内核内置 DIRECT，而不是兜底代理分组）。
	fallbackBuiltin string
	special         string
}

var synonymRules = []synonymRule{
	{keys: []string{"广告", "拦截", "adblock", "adguard", "ads"}, builtin: "REJECT"},
	{keys: []string{"国内", "直连", "direct", "mainland"}, wants: []string{"直连"}, fallbackBuiltin: "DIRECT"},
	{keys: []string{"漏网", "其他", "默认", "兜底", "final", "default", "other"}, special: "fallback"},
	{keys: []string{"苹果", "apple", "icloud"}, wants: []string{"苹果", "apple"}},
	{keys: []string{"电报", "telegram"}, wants: []string{"电报", "telegram"}},
	{keys: []string{"微软", "microsoft"}, wants: []string{"微软", "microsoft"}},
	{keys: []string{"谷歌", "google"}, wants: []string{"谷歌", "google"}},
	{keys: []string{"openai", "chatgpt", "claude", "gemini", "perplexity", "copilot"}, wants: []string{"ai", "chatgpt", "copilot"}},
	{keys: []string{"巴哈", "bahamut", "动画疯"}, wants: []string{"bahamut", "巴哈"}},
	{keys: []string{"priv", "pt站"}, wants: []string{"pt"}},
	{keys: []string{"youtube", "油管"}, wants: []string{"youtube"}},
	{keys: []string{"游戏", "game"}, wants: []string{"游戏", "steam"}},
}

// resolveBySynonym 按关键词表把目标名翻译成产物里存在的分组或内置出站。
func resolveBySynonym(target string, candidates []string, fallback string) string {
	nt := normalizeName(target)
	if nt == "" {
		return ""
	}
	for _, rule := range synonymRules {
		hit := false
		for _, k := range rule.keys {
			if strings.Contains(nt, k) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		if rule.builtin != "" {
			return rule.builtin
		}
		if rule.special == "fallback" {
			return fallback
		}
		for _, c := range candidates {
			nc := normalizeName(c)
			for _, w := range rule.wants {
				if strings.Contains(nc, w) {
					return c
				}
			}
		}
		if rule.fallbackBuiltin != "" {
			return rule.fallbackBuiltin
		}
	}
	return ""
}

// resolveBySimilarName 在候选名字里找与 target 语义相同的分组。
func resolveBySimilarName(target string, candidates []string) string {
	nt := normalizeName(target)
	if nt == "" {
		return ""
	}
	for _, c := range candidates {
		if normalizeName(c) == nt {
			return c
		}
	}
	// 子串匹配要求较短一方足够长，避免 "AI" 这类短名把一堆分组吸过去。
	for _, c := range candidates {
		nc := normalizeName(c)
		if nc == "" {
			continue
		}
		short, long := nc, nt
		if len([]rune(short)) > len([]rune(long)) {
			short, long = long, short
		}
		if len([]rune(short)) >= 4 && strings.Contains(long, short) {
			return c
		}
	}
	return ""
}

// normalizeName 去掉表情符号、标点、空白并统一小写，只保留字母、数字与汉字。
func normalizeName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) && r < 0x2000:
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.Is(unicode.Han, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitRuleFields 按顶层逗号切分规则行，逻辑规则里的括号内容不参与切分。
func splitRuleFields(line string) []string {
	var fields []string
	depth := 0
	start := 0
	for i, r := range line {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				fields = append(fields, line[start:i])
				start = i + 1
			}
		}
	}
	fields = append(fields, line[start:])
	return fields
}

// joinRuleFields 是 splitRuleFields 的逆操作。
func joinRuleFields(fields []string) string {
	return strings.Join(fields, ",")
}
