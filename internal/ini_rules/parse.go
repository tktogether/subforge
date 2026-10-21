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
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/parse"
)

// RemoteRuleset 是 [custom] 段里一行 ruleset= 引用。
//
// subconverter 的 ruleset= 一行可以声明两种完全不同的东西：
//   - 内联规则集：`<组名>,[]GEOSITE,<取值>[,<标志>]`，直接用内核自带的
//     GEOSITE / GEOIP 数据集，或 `[]FINAL` 兜底；不产生网络请求。
//   - 远程规则集：`<组名>,clash-domain:<URL>[,<interval>]`，把外部清单拉下来展开。
//
// 两类共用本结构，用 Kind 区分：Kind=="" 表示老式裸 URL 引用（按远程处理），
// "GEOSITE" / "GEOIP" / "FINAL" 是内联形态，"domain" / "classic" 是远程形态。
type RemoteRuleset struct {
	// Group 是 ruleset= 行的"组名"部分（逗号前），供调用方分组使用。
	Group string
	// URL 是远程规则集的引用地址（Kind=="" 或 "domain"/"classic" 时有值），
	// 指向一个 .list / .txt / .yaml / .mrs 规则集。
	URL string
	// Kind 见类型注释。Kind=="" 是"只有 URL 没有形态声明"的老写法，按 domain 处理。
	Kind string
	// Body 是内联规则集的取值（如 []GEOSITE,private 的 private）；
	// []FINAL 没有取值，Body 为空。仅 Kind 为内联形态时有意义。
	Body string
	// Flag 是内联规则集的附加标志位（如 no-resolve），来自第四段。
	Flag string
	// Interval 是远程规则集声明的缓存秒数（subconverter 的 interval），
	// 由模板作者声明；是否用于缓存策略由调用方决定。内联形态恒为 0。
	Interval int
}

// IsInline 报告这条引用是内联规则集（不产生网络请求）。
func (r RemoteRuleset) IsInline() bool {
	switch r.Kind {
	case "GEOSITE", "GEOIP", "FINAL":
		return true
	default:
		return false
	}
}

// Doc 是 Parse 的输出。两种形态互斥：
// Form==FormCustom 时 RulesetRefs 有值；Form==FormRule 时 PlainRules 有值。
type Doc struct {
	Form        Form
	RulesetRefs []RemoteRuleset
	PlainRules  []string
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
	//
	// 真实模板会随上游增删分组而漂移：Aethersailor/Custom_OpenClash_Rules 的
	// cfg/Custom_Clash_Full.ini 在 2026-10-21 就一度有 71 条 ruleset=（此前上限 64
	// 直接让「更新规则源」失败）。上限只用于挡住异常膨胀的输入，不能卡住正常模板，
	// 故放宽到 256；单条引用的体积另由 maxListBytes 限制。
	maxRefs = 256
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

// parseRuleset 解析 ruleset= 的值，两种形态都收：
//
//	<组名>,[]GEOSITE,<取值>[,<标志>]        内联规则集（不联网）
//	<组名>,[]GEOIP,<取值>[,<标志>]          内联规则集（不联网）
//	<组名>,[]FINAL                          内联兜底（不联网）
//	<组名>,clash-domain:<URL>[,<interval>]  远程规则集
//	<组名>,clash-classic:<URL>[,<interval>] 远程规则集
//	<组名>,<URL>                            远程规则集（老式裸 URL，形态未知）
//	<URL>                                   远程规则集（组名为空）
//
// 形态判定只看第二段：`[]` 前缀是内核自带数据集（subconverter 的内联写法），
// 冒号前缀是远程规则集（clash- 只是目标客户端标记，不是内容形态）。
// 与 internal/config/custom.go 的 splitRulesetSpec 保持同一套语义——那边服务
// 转换侧，本包服务规则源侧，两边都不能 import 对方（manifest→ini_rules 已存在，
// 再反向引会成环），所以这里独立实现，改动时要同步核对。
func parseRuleset(val string, lineNo int) (RemoteRuleset, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行缺值（组名,规则集）", lineNo)
	}
	parts := strings.SplitN(val, ",", 4)
	group := strings.TrimSpace(parts[0])
	var spec string
	if len(parts) == 1 {
		// 只有一段：组名缺省，整段是规则集声明（老式 ruleset=<URL> 写法）。
		// 不能直接当 URL——`ruleset=clash-domain:https://...` 也是单段写法。
		spec, group = group, ""
	} else {
		spec = strings.TrimSpace(parts[1])
	}
	if spec == "" {
		return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行缺 URL（逗号后为空）", lineNo)
	}
	// 内联形态：[]GEOSITE / []GEOIP / []FINAL，取值与标志位在第三、第四段。
	if inline := strings.TrimPrefix(spec, "[]"); inline != spec {
		kind := strings.ToUpper(strings.TrimSpace(inline))
		if kind == "" {
			return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行内联规则集缺形态名（[] 后为空）", lineNo)
		}
		// 内联形态没法从内容里找命中组，组名是必填的（远程形态可以只写 URL）。
		if group == "" {
			return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行内联规则集缺组名（写法：组名,[]%s,...）", lineNo, kind)
		}
		rr := RemoteRuleset{Group: group, Kind: kind}
		if len(parts) >= 3 {
			rr.Body = strings.TrimSpace(parts[2])
		}
		if len(parts) >= 4 {
			rr.Flag = strings.TrimSpace(parts[3])
		}
		switch kind {
		case "GEOSITE", "GEOIP":
			if rr.Body == "" {
				return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行内联 %s 缺取值", lineNo, kind)
			}
		case "FINAL":
			// FINAL 没有取值，第三段是标志位而非取值（正常写法下为空）。
		default:
			return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行内联规则集形态 %q 不支持（支持 GEOSITE / GEOIP / FINAL）", lineNo, kind)
		}
		return rr, nil
	}
	// 组名位置写成了 []形态：这行漏了组名（内联形态必须有命中组），
	// 不拦的话第二段会被当成裸 URL 去拉取，报出来的错误跟真实原因无关。
	if strings.HasPrefix(group, "[]") {
		return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行内联规则集缺组名（[] 开头的是内联形态，写法：组名,[]%s,...）", lineNo, strings.TrimPrefix(group, "[]"))
	}
	// 远程形态：冒号前缀声明内容形态；没有形态前缀则是老式裸 URL（形态按 domain 处理）。
	//
	// 判定必须排掉 URL 自带的 scheme 冒号：`https://x.list` 里的第一个冒号属于
	// scheme 而不是形态分隔符，按冒号硬拆会把 kind 认成 "https"、URL 认成 "//x.list"。
	// 判据是冒号后紧跟 `//`——那是 scheme 的写法；形态前缀后面跟的是 payload。
	if i := strings.Index(spec, ":"); i > 0 && !strings.HasPrefix(spec[i+1:], "//") {
		kind := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(spec[:i])), "clash-")
		url := strings.TrimSpace(spec[i+1:])
		if url == "" {
			return RemoteRuleset{}, fmt.Errorf("ruleset 第 %d 行远程规则集缺 URL（%s: 后为空）", lineNo, kind)
		}
		rr := RemoteRuleset{Group: group, URL: url, Kind: kind}
		if len(parts) >= 3 {
			rr.Interval = parseInterval(parts[2])
		}
		return rr, nil
	}
	// 裸 URL：老写法，形态未知（按 domain 展开），第三段仍是 interval。
	rr := RemoteRuleset{Group: group, URL: spec}
	if len(parts) >= 3 {
		rr.Interval = parseInterval(parts[2])
	}
	return rr, nil
}

// parseInterval 解析远程规则集声明的缓存秒数，两种写法都收：
// 裸数字（`...,28800`）与 subconverter 的 `interval=28800`。非法值按 0（未声明）处理，
// 不报错——缓存秒数是可选项，写错不该让整份规则源更新失败。
func parseInterval(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if i := strings.Index(s, "="); i >= 0 {
		s = strings.TrimSpace(s[i+1:])
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
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

// expandCustom 展开各 ruleset= 引用，按 ini 顺序合并去重。
//
// 内联引用（[]GEOSITE / []GEOIP / []FINAL）就地展开，不联网；远程引用（clash-domain: /
// clash-classic: / 裸 URL）并发拉取后按内容形态展开。两条路径的结果都按 ini 里的
// 声明顺序插回规则流——mihomo 按顺序匹配，把 []FINAL 展开的 MATCH 挪到中间会让它
// 后面的规则全部变成死规则。
func expandCustom(ctx context.Context, doc Doc, fetcher RuleFetcher) ([]string, error) {
	if len(doc.RulesetRefs) == 0 {
		return nil, nil
	}
	if len(doc.RulesetRefs) > maxRefs {
		return nil, fmt.Errorf("ini 规则引用数 %d 超过上限 %d", len(doc.RulesetRefs), maxRefs)
	}

	// 第一遍：内联引用就地展开；远程引用登记待拉取。
	perRef := make([][]string, len(doc.RulesetRefs))
	remoteIdx := make([]int, 0, len(doc.RulesetRefs))
	for i, rr := range doc.RulesetRefs {
		if rr.IsInline() {
			lines, err := inlineRuleset(rr)
			if err != nil {
				return nil, fmt.Errorf("展开内联规则集 #%d %s: %w", i, rr.Group, err)
			}
			perRef[i] = lines
			continue
		}
		remoteIdx = append(remoteIdx, i)
	}
	if len(remoteIdx) == 0 {
		return mergeRules(perRef), nil
	}

	// 第二遍：只对远程引用发请求，Name=Group（空则用 URL 末段），顺序与 ini 一致。
	srcs := make([]fetch.Source, len(remoteIdx))
	for k, i := range remoteIdx {
		rr := doc.RulesetRefs[i]
		name := rr.Group
		if name == "" {
			name = rr.URL
		}
		srcs[k] = fetch.Source{Name: name, URL: rr.URL}
	}
	results, err := fetcher.Fetch(ctx, srcs)
	if err != nil && len(results) == 0 {
		return nil, err
	}

	// fetch.Fetcher 会对内容做 Sniff（订阅形态识别），规则集会被判成"无法识别订阅
	// 内容形态"。那不是真正的拉取失败——只要有 Content 就继续用（与 manifest.Update
	// 对 res.Content 的使用语义一致）。单文件体积上限按 maxListBytes 另行校验。
	failed := make([]string, 0, len(remoteIdx))
	for k, i := range remoteIdx {
		rr := doc.RulesetRefs[i]
		name := srcs[k].Name
		var res fetch.Result
		if k < len(results) {
			res = results[k]
		}
		if len(res.Content) == 0 {
			failed = append(failed, fmt.Sprintf("#%d %s: %v", i, name, res.Err))
			continue
		}
		if len(res.Content) > maxListBytes {
			failed = append(failed, fmt.Sprintf("#%d %s: 单文件 %d 字节超过上限 %d", i, name, len(res.Content), maxListBytes))
			continue
		}
		lines, err := expandRemoteRuleset(rr, res.Content)
		if err != nil {
			failed = append(failed, fmt.Sprintf("#%d %s: %v", i, name, err))
			continue
		}
		perRef[i] = lines
	}
	if len(failed) == len(remoteIdx) {
		return nil, fmt.Errorf("全部 %d 个 ruleset 拉取失败（首个: %s）", len(remoteIdx), failed[0])
	}
	return mergeRules(perRef), nil
}

// mergeRules 按引用顺序拼接各规则集展开结果，按整行去重并保持首现顺序。
func mergeRules(perRef [][]string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, lines := range perRef {
		for _, rule := range lines {
			if seen[rule] {
				continue
			}
			seen[rule] = true
			out = append(out, rule)
		}
	}
	return out
}

// inlineRuleset 把一条内联规则集展开成 mihomo 规则行。
//
//	[]GEOSITE,<取值>[,<标志>] → GEOSITE,<取值>,<组名>[,<标志>]
//	[]GEOIP,<取值>[,<标志>]   → GEOIP,<取值>,<组名>[,<标志>]
//	[]FINAL                   → MATCH,<组名>
//
// FINAL 是 Clash Premium 的叫法，mihomo 只认 MATCH（见 docs/wiki/渲染器与输出格式.md）。
// GEOIP 规则集不查域名，没写 no-resolve 时默认补上——否则 mihomo 会把整个数据集
// 当成要做 DNS 解析的目标，规则既慢又不生效（与 internal/config/custom.go 同口径）。
func inlineRuleset(rr RemoteRuleset) ([]string, error) {
	flag := rr.Flag
	switch rr.Kind {
	case "FINAL":
		return []string{ruleLine("MATCH", "", rr.Group, flag)}, nil
	case "GEOSITE":
		return []string{ruleLine("GEOSITE", rr.Body, rr.Group, flag)}, nil
	case "GEOIP":
		if flag == "" {
			flag = "no-resolve"
		}
		return []string{ruleLine("GEOIP", rr.Body, rr.Group, flag)}, nil
	default:
		return nil, fmt.Errorf("不支持的内联规则集形态 %q", rr.Kind)
	}
}

// ruleLine 拼一条 mihomo 规则行：形态,取值,命中组[,附加]。
// MATCH / FINAL 的第二段本身就是出站名，没有取值，传空即可。
func ruleLine(kind, value, group, flag string) string {
	parts := []string{kind}
	if value != "" {
		parts = append(parts, value)
	}
	if group != "" {
		parts = append(parts, group)
	}
	if flag != "" {
		parts = append(parts, flag)
	}
	return strings.Join(parts, ",")
}

// expandRemoteRuleset 按内容形态展开一份远程规则集。
//
// 形态优先看文件内容：MRS 是 zstd 二进制，先按二进制分流，绝不能当文本去拆行
// （二进制里混着换行字节，拆出来是一堆乱码规则行，产物看起来还完全正常）。
// 其余按引用声明的 kind 走：domain 是域名清单，classic 是 Clash 经典规则清单，
// 空 kind 是"只有 URL 没有形态声明"的老写法，按 domain 处理。
func expandRemoteRuleset(rr RemoteRuleset, body []byte) ([]string, error) {
	// 没声明命中组（老式 ruleset=<URL>）：没有组名可补，清单原样保留——补空组会生成
	// "DOMAIN,x," 这种末尾空段的非法规则行，丢行又比原样透传更糟。
	if rr.Group == "" {
		return rawRuleLines(body), nil
	}
	if parse.IsMRS(body) {
		set, err := parse.DecodeMRS(body)
		if err != nil {
			return nil, fmt.Errorf("MRS 规则集解码失败: %w", err)
		}
		// 展开形态以文件声明的行为为准，不信引用里写的 clash-domain / clash-classic：
		// 声明与内容不符时按内容展开仍然是有效规则，照声明硬拆才会产出永不命中的规则。
		return expandMRSBinary(set, rr.Group), nil
	}
	if reason := rulesetProblem(body); reason != "" {
		return nil, fmt.Errorf("规则集内容异常（%s）", reason)
	}
	switch rr.Kind {
	case "", "domain":
		return expandDomainList(body, rr.Group), nil
	case "classic":
		return expandClassicList(body, rr.Group), nil
	default:
		return nil, fmt.Errorf("规则集类型 %q 不支持", rr.Kind)
	}
}

// expandDomainList 展开域名清单（subconverter 的 clash-domain 文本形态），
// 同时兼容 ACL4SSR 一系的"整行规则"清单。
//
// 两种内容都收：
//   - 裸域名：`example.com` → DOMAIN,example.com,<组名>；前缀 `.` 表示后缀匹配，
//     `+.` 是内核 domain 集合导出的等价写法，两者都展开成 DOMAIN-SUFFIX。
//   - 整行规则：`DOMAIN-SUFFIX,example.com` / `IP-CIDR,10.0.0.0/8,no-resolve`
//     → 丢掉原行的出站名，把组名补到取值之后（no-resolve 这类标志位保留在末位）。
//
// 裸域名不会带逗号，所以"有逗号就是整行规则"的判定足够用；声明头（mrs-ver:）
// 与注释跳过。
func expandDomainList(body []byte, group string) []string {
	var out []string
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "mrs-ver") {
			continue
		}
		if strings.Contains(line, ",") {
			if rule, ok := rewriteRuleLine(line, group); ok {
				out = append(out, rule)
			}
			continue
		}
		// 裸域名常见的尾巴是 ";备注" 或空白分隔的备注，只取第一段。
		if i := strings.IndexAny(line, "; \t"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case strings.HasPrefix(line, "+."):
			if d := line[2:]; d != "" {
				out = append(out, "DOMAIN-SUFFIX,"+d+","+group)
			}
		case strings.HasPrefix(line, "."):
			if d := line[1:]; d != "" {
				out = append(out, "DOMAIN-SUFFIX,"+d+","+group)
			}
		case line != "":
			out = append(out, "DOMAIN,"+line+","+group)
		}
	}
	return out
}

// classicKinds 是经典规则清单里会出现的形态名。
//
// 冒号分隔写法要靠它区分"分隔符"与"取值里带的冒号"，所以这里必须有一份形态表；
// 但不复刻 internal/ruleset 的闭集——那里的表只覆盖渲染器自己会产出的形态，而清单
// 文件可以写 mihomo 支持的任意形态（DST-PORT / SRC-IP-CIDR 等，Aethersailor 的
// Custom_Direct_Classical_IP.yaml 里就有 DST-PORT、SRC-PORT 各一条）。
var classicKinds = map[string]bool{
	"DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
	"DOMAIN-REGEX": true, "IP-CIDR": true, "IP-CIDR6": true,
	"GEOIP": true, "GEOSITE": true, "FINAL": true, "MATCH": true,
	"SRC-IP-CIDR": true, "DST-PORT": true, "SRC-PORT": true,
	"PROCESS-NAME": true, "RULE-SET": true, "DOMAIN-SET": true,
}

// classicFlags 是规则行里合法出现的标志位（出站名之外的第三段）。
var classicFlags = map[string]bool{"no-resolve": true, "no-ip": true}

// rewriteRuleLine 把一行"整行规则"重写成命中指定组的规则行。
//
// 两种分隔写法都收：逗号是主流（Loyalsoldier / ACL4SSR），冒号是 blackmatrix7 一系。
// 冒号只有当冒号前的词是已知形态名时才当分隔符——DOMAIN-REGEX 的取值里带冒号并不罕见，
// 按冒号硬拆会把正则截断成两半。
//
// 原行自带的第三段是清单作者的出站名（DIRECT 之类），展开时必须丢掉换成规则集指定的组；
// 只保留 no-resolve 这类标志位。判断按标志位白名单而不是"非 DIRECT 就算标志位"：
// 规则里合法出现裸域名取值（"IP-CIDR,1.1.1.1/32,1.1.1.1"），后者会误伤。
func rewriteRuleLine(line, group string) (string, bool) {
	kind, value, flag, ok := splitRuleLine(line)
	if !ok {
		return "", false
	}
	// MATCH / FINAL 是清单级的收尾规则，它的第二段已经是出站名；
	// 再塞一个组会生成非法规则，跳过比生成错行更诚实。
	if kind == "MATCH" || kind == "FINAL" {
		return "", false
	}
	return ruleLine(kind, value, group, flag), true
}

// splitRuleLine 拆一条规则行：形态、取值、附加标志（no-resolve 等）。
// 拿不到可用形态时返回 ok=false——调用方据此判断"这行不是规则"。
func splitRuleLine(line string) (kind, value, flag string, ok bool) {
	var fields []string
	if strings.Contains(line, ",") {
		fields = strings.Split(line, ",")
	} else if i := strings.Index(line, ":"); i >= 0 {
		if classicKinds[strings.ToUpper(strings.TrimSpace(line[:i]))] {
			fields = strings.SplitN(line, ":", 2)
		}
	}
	if len(fields) >= 2 {
		k := strings.ToUpper(strings.TrimSpace(fields[0]))
		v := strings.TrimSpace(fields[1])
		if k != "" && v != "" {
			return k, v, classicFlag(fields[2:]), true
		}
	}
	// 裸段：先按网段试，否则当域名。
	v := strings.TrimSpace(line)
	if v == "" {
		return "", "", "", false
	}
	if _, _, err := net.ParseCIDR(v); err == nil {
		k := "IP-CIDR"
		if strings.Contains(v, ":") {
			k = "IP-CIDR6"
		}
		return k, v, "", true
	}
	return "DOMAIN", v, "", true
}

// classicFlag 从规则行剩余字段里挑出要保留的标志位（如 no-resolve）。
func classicFlag(parts []string) string {
	var flags []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" && classicFlags[p] {
			flags = append(flags, p)
		}
	}
	return strings.Join(flags, ",")
}

// expandMRSBinary 把已解码的 MRS 规则集展开成规则行。
//
// 规则形态按**文件声明的行为**决定，而不是按引用写的 clash-domain/clash-classic：
// 文件里是什么就展开成什么，比照名字猜更不容易出错（猜错会产出匹配语义完全不同的规则）。
//
// 域名条目到 Clash 规则的映射（内核 domain 行为支持四种写法）：
//
//	+.d   → DOMAIN-SUFFIX,d      内核语义 = d 本身 + 所有子域，与 DOMAIN-SUFFIX 等价
//	.d    → DOMAIN-SUFFIX,d      内核语义 = 仅子域；这里略微放宽到含 d 本身
//	d     → DOMAIN,d             精确匹配
//	*.d   → DOMAIN-REGEX         单标签通配没有对应形态，用正则精确表达
//
// 与 internal/convert/remote_rulesets.go 的 expandMRSBinary 同语义（那边服务转换侧，
// 本包服务规则源侧，两边不能互相 import）。
func expandMRSBinary(set parse.MRSRuleSet, group string) []string {
	out := make([]string, 0, len(set.Rules))
	switch set.Behavior {
	case parse.MRSBehaviorDomain:
		for _, key := range set.Rules {
			switch {
			case strings.HasPrefix(key, "+."):
				if d := key[2:]; d != "" {
					out = append(out, "DOMAIN-SUFFIX,"+d+","+group)
				}
			case strings.HasPrefix(key, "*."):
				if d := key[2:]; d != "" {
					out = append(out, "DOMAIN-REGEX,^[^.]+\\."+regexp.QuoteMeta(d)+"$,"+group)
				}
			case strings.HasPrefix(key, "."):
				if d := key[1:]; d != "" {
					out = append(out, "DOMAIN-SUFFIX,"+d+","+group)
				}
			default:
				out = append(out, "DOMAIN,"+key+","+group)
			}
		}
	case parse.MRSBehaviorIPCIDR:
		for _, key := range set.Rules {
			kind := "IP-CIDR"
			if strings.Contains(key, ":") {
				kind = "IP-CIDR6"
			}
			out = append(out, kind+","+key+","+group)
		}
	}
	return out
}

// rulesetProblem 判断文本规则集内容是否可用于展开，返回原因（空表示可用）。
//
// mihomo 只接受文本规则。内容里混进控制字符时，yaml 编码会把它当成二进制输出成
// !!binary，产物变成一份内核加载不了的配置；比乱码更糟的是这种产物看起来完全正常。
// 宁可漏掉一份规则集并报错，也不能把乱码写成规则行。
func rulesetProblem(body []byte) string {
	for _, b := range body {
		switch b {
		case '\t', '\n', '\r':
		case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87:
			// 多字节序列的续字节，合法
		case 0x00, 0x7f, 0xff:
			return "含 NUL 或非文本字节"
		default:
			if b < 0x20 || (b >= 0xc0 && b < 0xc2) {
				return "含控制字符，疑似压缩或编码异常"
			}
		}
	}
	return ""
}

// expandClassicList 展开 Clash 经典规则清单（subconverter 的 clash-classic 类型）。
//
// 清单里的行本身就是 "TYPE,VALUE[,标志]"，把命中组插到取值之后即可：
// IP-CIDR,10.0.0.0/8 变成 IP-CIDR,10.0.0.0/8,组名，带 no-resolve 的补在末位。
// 形态校验走 splitRuleLine 的形态表：写错的形态会被挡掉，不产出无效规则行。
func expandClassicList(body []byte, group string) []string {
	var out []string
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// YAML 形态的清单把规则写在 payload: 下的序列里，"- " 是序列标记不是规则的一部分。
		for strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = strings.TrimSpace(line[2:])
		}
		// "payload:" 这类只有键没有值的行是清单结构，不是规则。
		if i := strings.Index(line, ":"); i >= 0 && strings.TrimSpace(line[i+1:]) == "" {
			continue
		}
		// 原行自带的第三段是清单作者的出站名（DIRECT 之类），展开时必须丢掉换成
		// 规则集指定的组；只保留 no-resolve 这类标志位（白名单判定，避免把裸域名
		// 取值误当标志位）。MATCH / FINAL 由 rewriteRuleLine 跳过——它的第二段已经
		// 是出站名，再塞一个组会生成非法规则。
		if rule, ok := rewriteRuleLine(line, group); ok {
			out = append(out, rule)
		}
	}
	return out
}

// rawRuleLines 原样保留清单里的规则行（去注释与空行）。
//
// 只用于没声明命中组的老式引用：没有组名可补时，清单作者写好的行是唯一可用的产物。
// 注释处理沿用 stripINIComment，与改动前 parseListRules 的行为一致。
func rawRuleLines(body []byte) []string {
	var out []string
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(stripINIComment(strings.TrimRight(raw, "\r")))
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
