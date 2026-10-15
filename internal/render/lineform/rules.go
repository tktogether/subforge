package lineform

import (
	"fmt"
	"strconv"
	"strings"
)

// 规则语法映射。
//
// 大部分规则类型在 Clash 与 Surge/QX/Loon/Surfboard 之间是 1:1 的，直接透传。
// 这一点是逐个 target 实机输出核实过的：subconverter v0.9.0 的 Surge / Quantumult(X) /
// Loon 产物里都原样出现 `GEOIP,CN,组名` 与 `FINAL,组名`，所以 GEOIP 不能当 Clash 专属丢弃。
// 真正的差异只有两处：
//   - GEOSITE 是内核专属，非 Clash 格式不认识，必须丢弃。
//   - 规则集的写法每个格式都不同，而且 Loon 要落到独立的 [Remote Rule] 段。
// 丢规则不能静默，render 层的告警会带出被丢的规则。

// passthroughRules 是各格式都认的规则类型。
var passthroughRules = map[string]bool{
	"DOMAIN-SUFFIX": true, "DOMAIN": true, "DOMAIN-KEYWORD": true,
	"IP-CIDR": true, "IP-CIDR6": true, "IP-CIDRNOLOG": true, "IP-CIDR6NOLOG": true,
	"DST-PORT": true, "SRC-PORT": true, "GEOIP": true,
	"FINAL": true, "MATCH": true,
}

// NewRuleFormatter 构造规则渲染器。
// ruleset 为 nil 表示该格式不支持远程规则集；remote 表示规则集行要落到独立的远程规则段；
// noResolve 表示 IP-CIDR 类规则要追加 no-resolve（Quantumult classic 的约定）。
func NewRuleFormatter(ruleset func(url, group string, interval int) string, remote, noResolve bool, interval int) func(string) RuleOut {
	if interval <= 0 {
		interval = 86400
	}
	return func(rule string) RuleOut {
		parts := strings.Split(rule, ",")
		if len(parts) < 2 {
			return RuleOut{}
		}
		typ := strings.ToUpper(strings.TrimSpace(parts[0]))
		target := strings.TrimSpace(parts[1])

		switch typ {
		case "RULE-SET":
			if ruleset == nil {
				return RuleOut{}
			}
			iv := intervalFromRule(parts)
			if iv <= 0 {
				iv = interval
			}
			line := ruleset(target, ruleGroup(parts), iv)
			// remote 决定落点：Loon 的规则集属于远程规则段；Surge / QX classic /
			// Surfboard 把 RULE-SET 直接写在本地规则段里。
			if remote {
				return RuleOut{Remote: line}
			}
			return RuleOut{Local: line}
		case "GEOSITE":
			return RuleOut{}
		default:
			if !passthroughRules[typ] {
				return RuleOut{}
			}
			if typ == "FINAL" || typ == "MATCH" {
				return RuleOut{Local: typ + "," + target}
			}
			out := RuleOut{Local: rule}
			if noResolve && (typ == "IP-CIDR" || typ == "IP-CIDR6") {
				out.Local = rule + ",no-resolve"
			}
			// 其余按原样透传，保留 NOLOG 后缀等细节。
			return out
		}
	}
}

// ruleGroup 取规则行最后的分组名。
func ruleGroup(parts []string) string {
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

// intervalFromRule 从规则行的 update-interval= 片段里取出间隔。
func intervalFromRule(parts []string) int {
	for _, p := range parts {
		if iv, ok := strings.CutPrefix(strings.TrimSpace(p), "update-interval="); ok {
			if n, err := strconv.Atoi(iv); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// simpleRulesetRule 是 "RULE-SET,url,group,update-interval=N" 的写法，
// Surge / Quantumult classic / Surfboard 都用这一形。
func simpleRulesetRule(url, group string, interval int) string {
	return fmt.Sprintf("RULE-SET,%s,%s,update-interval=%d", url, group, interval)
}

// qxRulesetRule 是 Quantumult X 的写法：rule-list= 前缀式。
func qxRulesetRule(url, group string, interval int) string {
	return fmt.Sprintf("rule-list=%s,group=%s,update-interval=%d", url, group, interval)
}

// loonRulesetRule 是 Loon 的写法：裸 "url,group"，落在 [Remote Rule] 段。
func loonRulesetRule(url, group string, interval int) string {
	return fmt.Sprintf("%s,%s", url, group)
}
