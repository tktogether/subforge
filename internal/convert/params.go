// Package convert 统一一次"转换请求"的语义：参数解析与管线组装。
//
// CLI 与 HTTP API 共用这一层，避免两处各写一套解析与装配逻辑。
// 参数名与取值约定对齐 subconverter / SCE，现有订阅脚本与客户端配置可原样使用。
package convert

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tktogether/subforge/internal/config"
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
	// DisableTemplateRules / DisableTemplateGroups 关掉模板里的规则流与代理分组。
	// 订阅方案没勾选 template 层时由调用方置位：此时模板只作为「参数与渲染目标」
	// 使用，不再贡献规则与分组——否则用户在方案里关掉的层会被模板静默补回来。
	DisableTemplateRules  bool
	DisableTemplateGroups bool
	// Filename 覆盖下载文件名（subconverter 的 filename 参数）。
	// sub-web 的"高级"模式会传它，用户改了这个字段却拿不到对应文件名会以为是后端 bug。
	// 只影响 Content-Disposition，不影响产物内容。
	Filename string
	// Headers 附加到上游订阅请求的请求头（token 等）。
	// 不走 query：凭证不该出现在 URL 里，由调用方按来源设置。
	Headers map[string]string
	// Ruleset 规则集键值（ruleset:<name>:<type> 形式，I6 完整实现）。
	Ruleset map[string]string
	// Pipeline 命名管线名（pipeline=<name>），引用模板里的 [pipeline:<name>] 定义。
	Pipeline string
	// Pipelines 模板提供的命名管线定义，按名索引。不来自 query，由模板带过来。
	Pipelines map[string]config.PipelineSpec
	// TemplateRules 模板给出的规则流（[common] rules= 与 [custom] 的内联规则），
	// 按模板声明顺序排列。Remote 非零的条目要由 Engine 拉取远程规则集后就地展开，
	// 展开完成前这个字段保留、Rules 不含它们。
	TemplateRules []config.RuleItem
	// TemplateRemoteRulesets 规则流引用的远程规则集，转换时拉取展开。
	TemplateRemoteRulesets []config.RemoteRuleset
	// TemplateWarnings 模板解释阶段的非阻断提示（如模板用了我们无法识别的
	// 分组/规则写法）。不影响本次转换是否成功，但必须让用户看到：否则一份
	// "节点齐、规则空"的配置会静默上线。
	TemplateWarnings []string
	// Override 用户本地覆盖层（M2 §1.2 分层覆盖 DSL）。由 CLI 加载文件后填入。
	// 在模板语义之后、管线之前应用：组合并到模板组，规则 prepend/append，
	// 节点级 server/port 改完再进管线——去重与排序看到的都是最终值。
	Override config.Override
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
	// url 的取值本身是 URL，`?` 是 URL 的一部分，不能当分隔符——
	// 按 `?` 拆会把 "?token=..." 丢掉，上游收不到 token 就 404，
	// 还多出一个 "token=..." 的假来源。多个来源用 `;` 或重复参数分隔。
	p.URLs = take(p.Raw, "url", ";")
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
	// sort 有两种写法，都必须吃：
	//   - subconverter / sub-web 用布尔：true=按备注排序，false=保持原序；
	//   - Sub-Store 语义用排序键：name/type/server/port/original。
	// 只认后者会让 sub-web 的"高级"模式整条请求 502——它固定会传 sort=false。
	// 布尔映射到排序键后交给 transform 统一校验，非法键仍在这里就报错。
	switch v := strings.ToLower(strings.TrimSpace(single(p.Raw, "sort"))); v {
	case "":
	case "1", "true", "yes", "on":
		p.Sort = string(transform.SortByName)
	case "0", "false", "no", "off":
		p.Sort = string(transform.SortByOriginal)
	default:
		p.Sort = v
	}
	p.List = isTrue(p.Raw, "list")
	p.UDP = isTrue(p.Raw, "udp")
	p.TFO = isTrue(p.Raw, "tfo")
	p.SCV = isTrue(p.Raw, "scv")
	p.TLS13 = isTrue(p.Raw, "tls13")
	p.Config = single(p.Raw, "config")
	p.Filename = strings.TrimSpace(single(p.Raw, "filename"))
	// pipeline=<name> 选中模板里的命名管线；未指定时走默认装配。
	p.Pipeline = strings.TrimSpace(single(p.Raw, "pipeline"))
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

// BuildPlan 把参数映射为可执行的 Plan（带 per-step 错误策略与告警收集）。
//
// 指定了 pipeline=<name> 时用模板里的命名管线；否则把 BuildPipeline 的裸链
// 包成默认策略（abort）的 Plan。两条路径都返回 Plan，Engine 只需一个执行入口。
//
// 本函数不带脚本库：命名管线里 ref=<脚本名> 的步骤会报「脚本步骤需要脚本库支持」。
// CLI 与其它只认模板的调用方依赖这个签名不变；控制台链路用 BuildPlanWithResolver。
func BuildPlan(p Params) (transform.Plan, error) {
	return BuildPlanWithResolver(p, nil)
}

// BuildPlanWithResolver 同 BuildPlan，但允许注入 ScriptResolver 以支持命名管线里
// ref=<脚本名> 的步骤。
//
// 为什么必须单独开一个入口而不是给 BuildPlan 加参数：脚本库只在控制台（serve）链路
// 里存在，CLI 与既有调用方没有它。写成新函数既让控制台把脚本注入进来，又不动
// BuildPlan 的签名——否则每个调用点都要跟着改，还会把「没脚本库」和「脚本被禁用」
// 两种语义混在一个 nil 里。
func BuildPlanWithResolver(p Params, res config.ScriptResolver) (transform.Plan, error) {
	if p.Pipeline != "" {
		spec, ok := p.Pipelines[p.Pipeline]
		if !ok {
			// 报错要把两条来源都点出来：控制台的「管线」页和模板 INI 是两处独立定义，
			// 只说模板会让只在界面上建过管线的用户以为功能坏了。
			return transform.Plan{}, fmt.Errorf(
				"未找到命名管线 %q：模板里没有 [pipeline:%s] 段落，控制台的「管线」页也没有启用的同名管线（GET /api/pipelines 可查现有名称）",
				p.Pipeline, p.Pipeline)
		}
		return spec.CompileWithResolver(res)
	}
	pipe, err := BuildPipeline(p)
	if err != nil {
		return transform.Plan{}, err
	}
	return transform.PlanFrom("default", pipe), nil
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
