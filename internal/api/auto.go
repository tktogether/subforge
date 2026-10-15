// 按 User-Agent 推断目标格式。
//
// 客户端的订阅地址里通常不带 target 参数，格式由 UA 决定——subconverter 就是这个约定。
// 这里只负责"识别出客户端想要什么"，是否支持由渲染器注册表裁决。
package api

import "strings"

const defaultTarget = "clash.meta"

// uaRule 是一条 UA 关键词到目标格式的映射。
type uaRule struct {
	keyword string
	target  string
}

// uaRules 按"越具体越靠前"排列，第一条命中即返回。
// 顺序有讲究：Clash Verge 也含 "Clash"，但两者目标相同，所以真正需要区分的是
// ClashX Meta（meta）与 ClashX（原版 Clash），前者必须排在前面。
var uaRules = []uaRule{
	{"ClashX Meta", "clash.meta"},
	{"Mihomo Party", "mihomo-party"},
	{"mihomo", "clash.meta"},
	{"Clash", "clash.meta"},
	{"Surge", "surge"},
	{"Loon", "loon"},
	{"Stash", "stash"},
	{"Quantumult", "quantumult"},
	{"Shadowrocket", "quantumult"},
	{"sing-box", "sing-box"},
	{"SFI", "sing-box"},
	{"Nekoray", "v2ray"},
	{"hiddify", "v2ray"},
	{"v2rayN", "v2ray"},
	{"QwertyX", "qwertyx"},
	{"FoXray", "foxray"},
}

// guessTarget 从 User-Agent 推断目标格式，推不出来时返回 clash.meta。
func guessTarget(userAgent string) string {
	ua := strings.ToLower(userAgent)
	if ua == "" {
		return defaultTarget
	}
	for _, rule := range uaRules {
		if strings.Contains(ua, strings.ToLower(rule.keyword)) {
			return rule.target
		}
	}
	return defaultTarget
}
