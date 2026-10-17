package render

import "strings"

// LinkTarget 描述 subconverter 那批"只输出分享链接"的 target 语义。
// Renderer 是归一后的渲染器名（list=裸链接清单，mixed=整体 base64 包裹），
// Types 是隐含的协议收窄，用内核规范化后的类型名，空表示不收窄。
type LinkTarget struct {
	Renderer string
	Types    []string
}

// LinkTargets 是 target 名到链接形态语义的映射，唯一的真源。
// convert 靠它做协议收窄，registry 靠它判断 target 是否合法——拆成两份
// 就必然出现"HTTP 能转、CLI 报不支持格式"的分裂。
// 取值来自 subconverter v0.9.0 实机输出（见 docs/compatibility.md）：
// ss / ssd 同时含 ss 与 ssr 节点，v2ray 只含 vmess。
var LinkTargets = map[string]LinkTarget{
	"list":         {Renderer: "list"},
	"sub":          {Renderer: "list"},
	"subscription": {Renderer: "list"},
	"mixed":        {Renderer: "mixed"},
	"ss":           {Renderer: "mixed", Types: []string{"ss", "ssr"}},
	"ssd":          {Renderer: "mixed", Types: []string{"ss", "ssr"}},
	"shadowsocks":  {Renderer: "mixed", Types: []string{"ss", "ssr"}},
	"ssr":          {Renderer: "mixed", Types: []string{"ssr"}},
	"shadowsocksr": {Renderer: "mixed", Types: []string{"ssr"}},
	"trojan":       {Renderer: "mixed", Types: []string{"trojan"}},
	"v2ray":        {Renderer: "mixed", Types: []string{"vmess"}},
}

// NormalizeTarget 归一化 target 名；不在表里时 ok=false，调用方按未支持的格式处理。
// ok=false 时返回的 LinkTarget 是零值，不要使用。
func NormalizeTarget(target string) (LinkTarget, bool) {
	lt, ok := LinkTargets[strings.ToLower(strings.TrimSpace(target))]
	return lt, ok
}

// IsMixedTarget 判断 target 是否走整体 base64 包裹的混合订阅形态。
func IsMixedTarget(target string) bool {
	lt, ok := NormalizeTarget(target)
	return ok && lt.Renderer == "mixed"
}

// ClashYAMLTargets 是产出 Clash YAML 配置的 target 写法。
// 与 ClashJSONTargets 分开成两张表，因为输出编码不同、扩展方式也不同：
// 新增一种序列化格式不该去改这一张名单。
var ClashYAMLTargets = map[string]bool{
	"":           true, // 空 target 走默认
	"clash":      true,
	"clash.meta": true,
	"clashmeta":  true,
	"mihomo":     true,
	"yaml":       true,
	// clashr 是 subconverter 的 ClashR 目标，也是 sub-web 客户端列表里的一项。
	// ClashR 早已停止维护，且它的字段名（Proxy/Rule 大写）是 Clash 大写形态的历史遗留；
	// 这里按现代 Clash/meta 的小写形态产出——ClashR 客户端接受小写，而大写形态
	// mihomo 会拒绝加载。取舍是"能用"优先于"逐字节复刻一个死掉的客户端"。
	"clashr": true,
}

// ClashJSONTargets 是产出 Clash JSON 配置的 target 写法。
// 构树逻辑与 YAML 完全共用（同一份配置，只换编码步骤），
// 因此这里只登记写法，不复制渲染逻辑。
var ClashJSONTargets = map[string]bool{
	"clash.json":      true,
	"clash.meta.json": true,
	"clashmeta.json":  true,
	"clash-json":      true,
	"clashmeta-json":  true,
	"mihomo.json":     true,
}

// SingboxTargets 是产出 sing-box JSON 配置的 target 写法。
// 与 ClashJSONTargets 分表：两者的构树逻辑与出站 schema 完全不同，
// 只是输出编码同为 JSON。
var SingboxTargets = map[string]bool{
	"sing-box": true,
	"singbox":  true,
}

// IsSingboxTarget 判断 target 是否产出 sing-box JSON 配置。
func IsSingboxTarget(target string) bool {
	_, ok := SingboxTargets[strings.ToLower(strings.TrimSpace(target))]
	return ok
}

// IsClashYAMLTarget 判断 target 是否产出 Clash YAML 配置。
func IsClashYAMLTarget(target string) bool {
	_, ok := ClashYAMLTargets[strings.ToLower(strings.TrimSpace(target))]
	return ok
}

// IsClashJSONTarget 判断 target 是否产出 Clash JSON 配置。
func IsClashJSONTarget(target string) bool {
	_, ok := ClashJSONTargets[strings.ToLower(strings.TrimSpace(target))]
	return ok
}

// TargetAliases 收录不属于"分享链接"与"一行一节点"两族、但存在多个写法的 target。
// SS-android 有两种入口格式：默认 `ss-sub` 输出裸 JSON（与 subconverter 一致），
// `ss-sub-b64` 输出 base64 包裹（客户端订阅入口用的那种）。两者共用一个渲染器，
// 靠别名选 Base64——渲染器名不变，X-Convert-Format 与文件名才保持一致。
// Mellow 只有单一输出形态，别名纯粹是写法差异。
var TargetAliases = map[string]string{
	"ss-sub":      "ss-sub",
	"ss-sub-json": "ss-sub",
	"ss-sub-b64":  "ss-sub",
	"ssjson":      "ss-sub",
	// sssub 是 subconverter 与 sub-web 用的写法（无连字符），落同一个渲染器；
	// 裸 JSON 形态，与 subconverter 的 sssub 目标一致。
	"sssub":      "ss-sub",
	"mellow":     "mellow",
	"mellow-ios": "mellow",
}

// LineFormTarget 描述"一行一节点"族的配置形态 target：Renderer 是归一后的
// 渲染器名（也就是 Format.Name），Aliases 是同一格式的其它写法。
// 与 LinkTargets 一样，convert 与 registry 都从这里读，避免两份名单漂移。
type LineFormTarget struct {
	Renderer string
	Aliases  []string
}

// LineFormTargets 是"一行一节点"族的 target 表，与 LinkTargets 同构。
// 别名按 subconverter 的常用写法收敛：surge2/3/4 与 suri 落到同一个 Surge 格式
// （它们只是客户端版本差异，不产生独立语法），quantumult/quantumultx 分别落到
// QX classic 与 QX。不支持的格式（surge6 的语法差异、sing-box 等）不列在这里，
// 让 Lookup 明确报"未支持"，而不是静默产出错格式。
var LineFormTargets = map[string]LineFormTarget{
	"surge":     {Renderer: "surge", Aliases: []string{"surge2", "surge3", "surge4", "suri"}},
	"quan":      {Renderer: "quan", Aliases: []string{"quantumult"}},
	"quanx":     {Renderer: "quanx", Aliases: []string{"quantumultx"}},
	"loon":      {Renderer: "loon"},
	"surfboard": {Renderer: "surfboard"},
}

// lineFormByName 把别名一并展开成 target -> 规范渲染器名。
// LineFormTargets 只按规范名做键，别名在各自的 Aliases 里；查表必须过这张展开表，
// 否则 `surge4` 会在"列出来可用"和"Lookup 报未支持"之间出现分裂。
var lineFormByName = func() map[string]string {
	m := make(map[string]string, len(LineFormTargets)+8)
	for target, lt := range LineFormTargets {
		m[target] = lt.Renderer
		for _, a := range lt.Aliases {
			m[a] = lt.Renderer
		}
	}
	return m
}()

// NormalizeLineFormTarget 归一化配置形态 target，返回规范 target 与其渲染器名。
// 别名归一到规范名：`surge4` 与 `suri` 都落到 `surge`。不在表里时 ok=false。
func NormalizeLineFormTarget(target string) (LineFormTarget, bool) {
	t := strings.ToLower(strings.TrimSpace(target))
	canonical, ok := lineFormByName[t]
	if !ok {
		return LineFormTarget{}, false
	}
	lt, ok := LineFormTargets[canonical]
	if !ok {
		return LineFormTarget{}, false
	}
	return LineFormTarget{Renderer: lt.Renderer, Aliases: lt.Aliases}, true
}

// LineFormRenderer 返回 target 归一后的规范渲染器名；不在表里时 ok=false。
func LineFormRenderer(target string) (string, bool) {
	renderer, ok := lineFormByName[strings.ToLower(strings.TrimSpace(target))]
	return renderer, ok
}

// IsLineFormTarget 判断 target 是否属"一行一节点"配置形态（Surge / QX / Loon / Surfboard）。
func IsLineFormTarget(target string) bool {
	_, ok := lineFormByName[strings.ToLower(strings.TrimSpace(target))]
	return ok
}

// IsTargetAlias 判断 target 是否属于 TargetAliases 收录的那一小批写法。
func IsTargetAlias(target string) bool {
	_, ok := TargetAliases[strings.ToLower(strings.TrimSpace(target))]
	return ok
}
