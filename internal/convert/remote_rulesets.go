package convert

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/ruleset"
)

// expandRemoteRulesets 把模板引用的远程规则集拉取并就地展开成规则行。
//
// subconverter 的 ruleset=clash-domain:URL 语义是"把这份域名清单拉下来，每条命中时
// 路由到指定组"。mihomo 的 ruleset provider 只接受本地 file: 路径、不接受 http(s)，
// 所以规则集必须在转换时下载、展开成内联规则行——这也是产物体积会明显大于节点本身的原因。
//
// 展开后的行按模板声明的位置插回规则流：mihomo 按顺序匹配，模板作者把 MATCH 放最后
// 才保证前面的规则都能生效，把展开结果整体追加到末尾会让 MATCH 之后的规则变成死规则。
//
// 单个规则集失败只提示、不阻断转换：订阅本身还能转出一份可用配置，强过整体失败；
// 但必须提示，否则用户会以为规则集已经生效。
func (e *Engine) expandRemoteRulesets(ctx context.Context, p Params) (Params, []string) {
	if len(p.TemplateRules) == 0 {
		return p, nil
	}
	var warnings []string
	expanded := make(map[int][]string, len(p.TemplateRemoteRulesets))
	for i, rs := range p.TemplateRemoteRulesets {
		body, err := rulesetBody(ctx, e.RulesetLoader, rs.URL)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("规则集 %s 获取失败，该规则集未生效: %v", rs.URL, err))
			continue
		}
		if reason := rulesetProblem(body); reason != "" {
			warnings = append(warnings, fmt.Sprintf("规则集 %s 内容异常（%s），该规则集未生效", rs.URL, reason))
			continue
		}
		var out []string
		switch rs.Kind {
		case "domain":
			out = expandMRS(body, rs.Group)
		case "classic":
			out = expandClassic(body, rs.Group)
		default:
			warnings = append(warnings, fmt.Sprintf("规则集 %s 的类型 %q 不支持", rs.URL, rs.Kind))
			continue
		}
		if len(out) == 0 {
			warnings = append(warnings, fmt.Sprintf("规则集 %s 没有展开出任何规则", rs.URL))
			continue
		}
		expanded[i] = out
	}
	var rules []string
	for _, it := range p.TemplateRules {
		if it.Remote < 0 {
			rules = append(rules, it.Line)
			continue
		}
		rules = append(rules, expanded[it.Remote]...)
	}
	// 清空流，避免后续再看到"有引用但没展开"的中间状态。
	p.TemplateRules = nil
	p.TemplateRemoteRulesets = nil
	p.Rules = append(p.Rules, rules...)
	return p, warnings
}

// rulesetBody 取一份规则集的内容：带 "://" 的当 URL 拉，否则按本地路径读。
//
// 刻意不复用 fetch.Fetcher：那是为订阅内容做的，会把响应体当订阅去识别形态，
// 一份纯域名清单会被判成"无法识别订阅内容形态"而整体失败。
func rulesetBody(ctx context.Context, loader config.RemoteLoader, ref string) ([]byte, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("规则集地址为空")
	}
	if !strings.Contains(ref, "://") {
		buf, err := os.ReadFile(ref)
		if err != nil {
			return nil, fmt.Errorf("读取本地规则集 %s: %w", ref, err)
		}
		return buf, nil
	}
	if loader == nil {
		loader = config.HTTPLoader(http.DefaultClient, nil)
	}
	buf, err := loader(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("拉取规则集 %s: %w", ref, err)
	}
	return buf, nil
}

// expandMRS 展开 mrs 域名清单（subconverter 的 clash-domain 类型）。
//
// mrs 语法：一行一个域名，前缀 "." 表示域名后缀匹配，其余按精确域名匹配。
// 声明头（mrs-ver:）与注释按注释处理。
func expandMRS(body []byte, group string) []string {
	var out []string
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "mrs-ver") {
			continue
		}
		// 尾部备注用分号或空白分隔，只取第一段。
		if i := strings.IndexAny(line, "; \t"); i >= 0 {
			line = line[:i]
		}
		dom := strings.TrimSpace(line)
		if dom == "" {
			continue
		}
		if strings.HasPrefix(dom, ".") {
			if dom = strings.TrimPrefix(dom, "."); dom == "" {
				continue
			}
			out = append(out, fmt.Sprintf("DOMAIN-SUFFIX,%s,%s", dom, group))
			continue
		}
		out = append(out, fmt.Sprintf("DOMAIN,%s,%s", dom, group))
	}
	return out
}

// rulesetProblem 判断规则集内容是否可用于展开，返回原因（空表示可用）。
//
// mihomo 只接受文本规则。内容里混进控制字符时，yaml 编码会把它当成二进制输出成
// !!binary，产物变成一份内核加载不了的配置；比乱码更糟的是这种产物看起来完全正常。
// 宁可漏掉一份规则集并提示，也不能把乱码写成规则行。
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

// expandClassic 展开 Clash 经典规则清单（subconverter 的 clash-classic 类型）。
//
// 清单里的行本身就是 "TYPE,VALUE[,标志]"，把命中组插到取值之后即可：
// IP-CIDR,10.0.0.0/8 变成 IP-CIDR,10.0.0.0/8,组名，带 no-resolve 的补在末位。
func expandClassic(body []byte, group string) []string {
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
		kind, value, flag := splitClassicLine(line)
		if kind == "" || value == "" {
			continue
		}
		// MATCH / FINAL 是清单级的收尾规则，它的第二段已经是出站名；
		// 再塞一个组会生成非法规则，跳过比生成错行更诚实。
		if kind == string(ruleset.Match) || kind == string(ruleset.Final) {
			continue
		}
		out = append(out, fmt.Sprintf("%s,%s,%s", kind, value, group))
		if flag != "" {
			out[len(out)-1] += "," + flag
		}
	}
	return out
}

// classicKinds 是经典规则清单里会出现的形态名。
// 冒号分隔写法要靠它区分"分隔符"与"取值里带的冒号"，列在这里而不是复用
// ruleset 包的校验：那里的形态表是闭集，而清单文件可能写着 mihomo 之外的形态。
var classicKinds = map[string]bool{
	"DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
	"DOMAIN-REGEX": true, "IP-CIDR": true, "IP-CIDR6": true,
	"GEOIP": true, "GEOSITE": true, "FINAL": true, "MATCH": true,
	"SRC-IP-CIDR": true, "DST-PORT": true, "SRC-PORT": true,
	"PROCESS-NAME": true, "RULE-SET": true, "DOMAIN-SET": true,
}

// classicFlags 是规则行里合法出现的标志位。
var classicFlags = map[string]bool{"no-resolve": true, "no-ip": true}

// splitClassicLine 拆一条经典规则：形态、取值、附加标志（no-resolve 等）。
//
// 两种分隔写法都收：逗号是主流（Loyalsoldier / ACL4SSR），冒号是 blackmatrix7 一系。
// 冒号只有当冒号前的词是已知形态名时才当分隔符——DOMAIN-REGEX 的取值里带冒号并不罕见，
// 按冒号硬拆会把正则截断成两半。
//
// 原行自带的第三段是清单作者的出站名（DIRECT 之类），展开时必须丢掉换成规则集指定的组；
// 只保留 no-resolve 这类标志位。判断按标志位白名单而不是"非 DIRECT 就算标志位"：
// 规则里合法出现裸域名取值（"IP-CIDR,1.1.1.1/32,1.1.1.1"），后者会误伤。
func splitClassicLine(line string) (kind, value, flag string) {
	var fields []string
	if i := strings.Index(line, ","); i >= 0 {
		fields = strings.Split(line, ",")
	} else if i := strings.Index(line, ":"); i >= 0 {
		if classicKinds[strings.ToUpper(strings.TrimSpace(line[:i]))] {
			fields = strings.SplitN(line, ":", 2)
		}
	}
	if len(fields) >= 2 {
		kind = strings.ToUpper(strings.TrimSpace(fields[0]))
		value = strings.TrimSpace(fields[1])
		if kind != "" && value != "" {
			return kind, value, classicFlag(fields[2:])
		}
	}
	// 裸段：先按网段试，否则当域名。
	v := strings.TrimSpace(line)
	if v == "" {
		return "", "", ""
	}
	if _, _, err := net.ParseCIDR(v); err == nil {
		k := string(ruleset.IPCIDR)
		if strings.Contains(v, ":") {
			k = string(ruleset.IPCIDR6)
		}
		return k, v, ""
	}
	return string(ruleset.Domain), v, ""
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
