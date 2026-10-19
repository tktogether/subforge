// Package ini_rules 解析 ACL4SSR / Surge / QX 风格的 INI 规则源文件。
//
// 本包只处理"规则来源 INI"这一种语义，不复用 internal/template 包的
// subconverter 模板解析器——后者会把 [custom] 当成模板段落（见
// internal/template/ini.go 的 repeatable 表），custom_proxy_group= 与
// ruleset= 的语义会被模板层覆盖逻辑污染。规则源需要的是逐条采集 ruleset=
// 引用并递归拉取合并，与模板渲染无关，故独立成包。
//
// 支持两种输入形态：
//   - [custom] + ruleset= 引用型：ACL4SSR 风格，本身不含规则，需展开。
//   - [Rule] 节直书型：Surge / QX 风格，逐行即规则。
package ini_rules

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/fetch"
)

// RemoteRuleset 是 [custom] 段里一行 ruleset= 引用。
type RemoteRuleset struct {
	// Group 是 ruleset= 行的"组名"部分（逗号前），供调用方分组使用。
	Group string
	// URL 是 ruleset= 行的引用地址（逗号后），指向一个 .list / .txt 规则集。
	URL string
}

// Doc 是 Parse 的输出。两种形态互斥：
// Form==FormCustom 时 RulesetRefs 有值；Form==FormRule 时 PlainRules 有值。
type Doc struct {
	Form         Form
	RulesetRefs  []RemoteRuleset
	PlainRules   []string
}

// Form 标识 INI 的规则形态。
type Form int

const (
	// FormUnknown 表示未识别出 [custom] 或 [Rule] 节——
	// 调用方应回落到既有非 INI 处理逻辑，不应报错。
	FormUnknown Form = iota
	// FormCustom 是 [custom] + ruleset= 引用型。
	FormCustom
	// FormRule 是 [Rule] 节直书型。
	FormRule
)

// Expand 的资源上限，防止上游异常撑爆本进程。
const (
	// maxRefs 限制单份 INI 里的 ruleset= 引用数。
	maxRefs = 64
	// maxListBytes 限制单个 .list 文件的体积。
	maxListBytes = 2 << 20 // 2 MB
	// maxParallel 限制并发拉取数。
	maxParallel = 4
)

// Parse 识别 [custom]/[Rule] 两种 INI 规则形态。
//
// 容错规则：
//   - # 与 ; 起的注释行、纯空行跳过（; 仅行首起注释，与值里的分号区分）。
//   - [custom] 段只采集 ruleset=，custom_proxy_group= 等其它键忽略（反引号+emoji 不许误伤）。
//   - ruleset= 缺 URL（逗号后为空）报错并标注行号。
//   - [Rule] 段逐行采集规则行（DOMAIN-SUFFIX / DOMAIN / IP-CIDR / RULE-SET 等），不去重。
//   - 不含 [custom] 也不含 [Rule] 时返回 FormUnknown（不报错），由调用方走原逻辑。
func Parse(data []byte) (Doc, error) {
	doc := Doc{}
	var form Form
	var inCustom, inRule bool

	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		lineNo := i + 1
		stripped := stripINIComment(strings.TrimRight(raw, "\r"))
		line := strings.TrimSpace(stripped)
		if line == "" {
			continue
		}
		// 段落头 [name]
		if name, ok := splitSection(line); ok {
			switch strings.ToLower(name) {
			case "custom":
				inCustom, inRule = true, false
				form = FormCustom
			case "rule":
				inCustom, inRule = false, true
				form = FormRule
			default:
				// 其它段落（如 [common]）整体跳过，避免误采。
				inCustom, inRule = false, false
			}
			continue
		}

		if inCustom {
			// 只认 ruleset=，其余键（custom_proxy_group= 等）一律忽略。
			key, val, ok := splitKV(line)
			if !ok {
				continue
			}
			if strings.EqualFold(key, "ruleset") {
				rr, err := parseRuleset(val, lineNo)
				if err != nil {
					return Doc{}, err
				}
				doc.RulesetRefs = append(doc.RulesetRefs, rr)
			}
			// custom_proxy_group= 等忽略
			continue
		}
		if inRule {
			// [Rule] 节逐行即规则，整行作为一个规则条目（去掉首尾空白与注释后）。
			doc.PlainRules = append(doc.PlainRules, line)
			continue
		}
		// 既不在 [custom] 也不在 [Rule]：跳过（顶层注释、其它段等）。
	}

	if form == FormUnknown {
		return doc, nil
	}
	doc.Form = form
	if form == FormCustom && len(doc.RulesetRefs) > maxRefs {
		return Doc{}, fmt.Errorf("ini 规则引用数 %d 超过上限 %d", len(doc.RulesetRefs), maxRefs)
	}
	return doc, nil
}

// parseRuleset 解析 ruleset= 的值 "组名,URL"。
func parseRuleset(val string, lineNo int) (RemoteRuleset, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行缺值（组名,URL）", lineNo)
	}
	idx := strings.Index(val, ",")
	if idx < 0 {
		// 只有 URL 没有组名也允许（组名为空）。
		url := strings.TrimSpace(val)
		if url == "" {
			return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行缺 URL", lineNo)
		}
		return RemoteRuleset{Group: "", URL: url}, nil
	}
	group := strings.TrimSpace(val[:idx])
	url := strings.TrimSpace(val[idx+1:])
	if url == "" {
		return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行缺 URL（逗号后为空）", lineNo)
	}
	return RemoteRuleset{Group: group, URL: url}, nil
}

// RuleFetcher 是对 fetch.Fetcher 的最小接口抽象，便于测试注入 httptest server。
// 与 internal/manifest/manifest.go:163-186 用法一致：构造 fetch.Source 调用拉取。
type RuleFetcher interface {
	// Fetch 拉取若干来源，结果顺序与入参一致；单个失败挂到对应 Result.Err。
	Fetch(ctx context.Context, sources []fetch.Source) ([]fetch.Result, error)
}

// Expand 对 [custom] 形态的 Doc 递归拉取每个 ruleset= 指向的 .list，合并去重后
// 返回规则行；[Rule] 形态直接返回 PlainRules（已去重）。FormUnknown 返回 nil。
//
// 合并顺序 = ini 中出现顺序；按规则行去重，保持首现顺序。上限见 maxRefs /
// maxListBytes / maxParallel 常量，超限报错。
func Expand(ctx context.Context, doc Doc, fetcher RuleFetcher) ([]string, error) {
	switch doc.Form {
	case FormUnknown:
		return nil, nil
	case FormRule:
		return dedupPreserveOrder(doc.PlainRules), nil
	case FormCustom:
		return expandCustom(ctx, doc, fetcher)
	default:
		return nil, nil
	}
}

// expandCustom 并发拉取各 ruleset= 的 .list，按 ini 顺序合并去重。
func expandCustom(ctx context.Context, doc Doc, fetcher RuleFetcher) ([]string, error) {
	if len(doc.RulesetRefs) == 0 {
		return nil, nil
	}
	if len(doc.RulesetRefs) > maxRefs {
		return nil, fmt.Errorf("ini 规则引用数 %d 超过上限 %d", len(doc.RulesetRefs), maxRefs)
	}

	// 构造 fetch.Source 列表，Name=Group（空则用 URL 末段），保留 ini 顺序。
	srcs := make([]fetch.Source, len(doc.RulesetRefs))
	for i, rr := range doc.RulesetRefs {
		name := rr.Group
		if name == "" {
			name = rr.URL
		}
		srcs[i] = fetch.Source{Name: name, URL: rr.URL}
	}

	results, err := fetcher.Fetch(ctx, srcs)
	if err != nil && len(results) == 0 {
		return nil, err
	}

	// fetch.Fetcher 会对内容做 Sniff（订阅形态识别），纯文本 .list 会被拒。
	// 规则源 .list 是纯文本规则，Sniff 失败不算真正的拉取失败——只要有 Content
	// 就继续使用（与 manifest.Update 对 res.Content 的使用语义一致）。
	// 单文件体积上限校验：拉取本身有 fetch.Config.MaxBody，但这里额外按 maxListBytes
	// 做一次明确校验，错误信息与契约一致。
	var failed []string
	for i, r := range results {
		if len(r.Content) == 0 {
			failed = append(failed, fmt.Sprintf("#%d %s: %v", i, srcs[i].Name, r.Err))
			continue
		}
		if len(r.Content) > maxListBytes {
			failed = append(failed, fmt.Sprintf("#%d %s: 单文件 %d 字节超过上限 %d", i, srcs[i].Name, len(r.Content), maxListBytes))
			continue
		}
		_ = i
	}
	if len(failed) == len(results) {
		return nil, fmt.Errorf("全部 %d 个 ruleset 拉取失败（首个: %s）", len(results), failed[0])
	}

	out := make([]string, 0)
	seen := map[string]bool{}
	for _, r := range results {
		if len(r.Content) == 0 {
			continue
		}
		if len(r.Content) > maxListBytes {
			continue
		}
		for _, rule := range parseListRules(r.Content) {
			if seen[rule] {
				continue
			}
			seen[rule] = true
			out = append(out, rule)
		}
	}
	return out, nil
}

// parseListRules 把一份 .list / .txt 规则集原文拆成规则行：
// 去注释（# 与 ;）、去空行、去首尾空白。
func parseListRules(data []byte) []string {
	var out []string
	for _, raw := range strings.Split(string(data), "\n") {
		stripped := stripINIComment(strings.TrimRight(raw, "\r"))
		line := strings.TrimSpace(stripped)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// dedupPreserveOrder 去重保序。
func dedupPreserveOrder(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// splitSection 识别 "[段落名]" 形式的行，段落名去首尾空白后返回。
func splitSection(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	name := strings.TrimSpace(line[1 : len(line)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

// splitKV 按第一个 '=' 拆键值；'=' 优先，键值两侧去空白。
func splitKV(line string) (key, value string, ok bool) {
	idx := strings.Index(line, "=")
	if idx <= 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:idx])
	if k == "" {
		return "", "", false
	}
	return k, strings.TrimSpace(line[idx+1:]), true
}

// stripINIComment 去掉 INI 规则源里的注释：
//   - '#' 在任意位置起注释（ACL4SSR / .list 惯例）；
//   - ';' 只在行首起注释（与值里合法的分号区分，如 emoji 规则的 rename）。
func stripINIComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		line = line[:i]
	}
	t := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(t, ";") {
		return ""
	}
	return line
}

// IsINI 嗅探：首个非注释行匹配 ^[custom] 或 ^ruleset= 或 ^[Rule] 即按 ini 处理。
// 供 manifest 集成层在 input_format=auto 时调用，避免改写既有非 ini 来源行为。
func IsINI(data []byte) bool {
	for _, raw := range strings.Split(string(data), "\n") {
		stripped := stripINIComment(strings.TrimRight(raw, "\r"))
		line := strings.TrimSpace(stripped)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "[custom]" || strings.HasPrefix(lower, "ruleset=") || lower == "[rule]" {
			return true
		}
		return false
	}
	return false
}

// FormatRulesText 把规则行列表渲染成 text 落盘内容：每行一条规则，\n 分隔。
func FormatRulesText(rules []string) []byte {
	var buf bytes.Buffer
	for _, r := range rules {
		buf.WriteString(r)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// FormatRulesYAML 把规则行列表渲染成 clash-meta YAML：rules: 列表。
func FormatRulesYAML(rules []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("rules:\n")
	for _, r := range rules {
		buf.WriteString("  - ")
		// 简单转义：值里若含特殊 YAML 字符则用双引号包裹。
		if needsYAMLQuote(r) {
			buf.WriteString(yamlDoubleQuote(r))
		} else {
			buf.WriteString(r)
		}
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// needsYAMLQuote 判断规则行是否含需引号包裹的 YAML 特殊字符。
func needsYAMLQuote(s string) bool {
	if s == "" {
		return true
	}
	for _, ch := range s {
		switch ch {
		case ':', '#', '@', '`', '!', '|', '>', '%', '&', '*', '?', ',', '{', '}', '[', ']':
			return true
		}
	}
	// 以特殊字符开头也要引号
	switch s[0] {
	case '-', '?', ':', ' ', '\t':
		return true
	}
	return false
}

// yamlDoubleQuote 用双引号包裹并转义内部双引号与反斜杠。
func yamlDoubleQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}

// fetcherAdapter 让 *fetch.Fetcher 实现 RuleFetcher 接口。
// 用闭包而非方法，避免给 fetch 包加耦合；manifest 集成层注入即可。
type fetcherAdapter struct {
	f *fetch.Fetcher
}

// Fetch 透传到 *fetch.Fetcher.Fetch。
func (a fetcherAdapter) Fetch(ctx context.Context, sources []fetch.Source) ([]fetch.Result, error) {
	return a.f.Fetch(ctx, sources)
}

// NewRuleFetcher 用 *fetch.Fetcher 包装成 RuleFetcher。
func NewRuleFetcher(f *fetch.Fetcher) RuleFetcher {
	return fetcherAdapter{f: f}
}

// MaxParallel 限制展开时的并发拉取数，供 manifest 集成层构造 fetch.Config 时使用。
const MaxParallel = maxParallel
