package convert

import (
	"regexp"
	"strings"
)

// memberOutbounds 是 mihomo 的内置出站名。模板把它们当组成员引用，展开时必须原样保留。
var memberOutbounds = map[string]bool{
	"DIRECT":      true,
	"REJECT":      true,
	"REJECT-DROP": true,
	"PASS":        true,
	"COMPATIBLE":  true,
}

// 模板作者写的是 PCRE 方言，Go 的 RE2 只认子集，两种语法要翻译过来才能编译。
var lookaroundPrefixes = []string{"(?<=", "(?<!", "(?=", "(?!"}

// CompileMemberPattern 把成员项正则编译成 RE2 可用形式。
//
// 返回的 negate 为真表示结果语义是"不匹配"，由 matchNode 取反使用。
// ok 为假表示 pattern 无法解释成正则（例如带正则元字符的裸节点名），调用方按字面量保留。
func CompileMemberPattern(pattern string) (re *regexp.Regexp, negate bool, ok bool) {
	if compiled, err := regexp.Compile(pattern); err == nil {
		return compiled, false, true
	}
	// 整段取反是唯一"剥掉就反义"的写法，必须单独翻译，不能一并剥掉。
	if inner, matched := splitNegateAll(pattern); matched {
		if compiled, err := regexp.Compile(stripLookaround(inner)); err == nil {
			return compiled, true, true
		}
	}
	compiled, err := regexp.Compile(stripLookaround(pattern))
	if err != nil {
		return nil, false, false
	}
	return compiled, false, true
}

// stripLookaround 剥掉环视断言。
//
// 模板里这些断言都是排除性护栏（"白"前的"俄"不算俄罗斯、"0.20x"里的点不算小数点），
// 剥掉后匹配范围只会放宽不会收紧，对分组结果的影响是良性偏差。
func stripLookaround(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); {
		if p[i] == '\\' && i+1 < len(p) {
			b.WriteString(p[i : i+2])
			i += 2
			continue
		}
		if skip := lookaroundSpan(p[i:]); skip > 0 {
			i += skip
			continue
		}
		b.WriteByte(p[i])
		i++
	}
	return b.String()
}

// lookaroundSpan 若 s 以环视断言开头，返回含左右括号的整段长度；否则返回 0。
func lookaroundSpan(s string) int {
	for _, pre := range lookaroundPrefixes {
		if !strings.HasPrefix(s, pre) {
			continue
		}
		depth := 1
		for i := len(pre); i < len(s); i++ {
			if s[i] == '\\' && i+1 < len(s) {
				i++
				continue
			}
			switch s[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
		}
		return 0
	}
	return 0
}

// splitNegateAll 识别 `^(?!.*(排除项)).*` 这种整段取反写法，返回排除项正则。
//
// 模板用它做"已分类之外的其他节点"（🌐 其他地区）：先列出所有已归类关键词，
// 再整体取反。RE2 没有前瞻，只能翻译成"不匹配排除项"的匹配式。
func splitNegateAll(p string) (string, bool) {
	body := p
	// 外层常见的裸分组包裹 `(...)` 语义与内容相同，先剥掉再判断。
	for body != "" && body[0] == '(' && matchingParen(body, 0) == len(body)-1 {
		body = body[1 : len(body)-1]
	}
	if !strings.HasPrefix(body, "^(?!") {
		return "", false
	}
	// `(` 在第 1 位、`!` 在第 3 位，前瞻内容从第 4 位开始；end 指向前瞻的右括号。
	end := matchingParen(body, 1)
	if end < 0 {
		return "", false
	}
	// 前瞻后面只能跟 `.*`（可选 `$`），否则不是"整段取反"的写法。
	if strings.TrimSuffix(body[end+1:], "$") != ".*" {
		return "", false
	}
	inner := strings.TrimPrefix(body[4:end], ".*")
	if strings.HasSuffix(inner, ".*") {
		inner = inner[:len(inner)-2]
	}
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return "", false
	}
	return inner, true
}

// matchingParen 返回 s 中位置 openIdx 处的左括号对应的右括号位置；不成对返回 -1。
func matchingParen(s string, openIdx int) int {
	depth := 1
	for i := openIdx + 1; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
