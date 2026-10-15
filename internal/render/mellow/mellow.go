// Package mellow 渲染 Mellow（iOS）配置形态。
//
// Mellow 的配置是文本段落：节点写成 `名字, 协议, 分享链接` 三列一行，
// 代理组写成 `名字, 成员列表, 模式, 选项`，规则沿用 Clash 语法。
// 节点凭据不需要单独展开——直接复用分享链接，
// 因此 URI 的正确性只有一处实现（sharelink），mellow 不会漂出去。
//
// 语法依据 subconverter v0.9.0 实机输出（tools/scv/goldens2/mellow.out，
// GPL-3.0 仅作参照）。参考产物本身有缺陷：12 个节点只导出了 vmess 与 ss 两个，
// 而且这两条的凭据都是空的（`vmess1://@host:port?network=&tls=`、`ss://Og@…`，
// 其中 Og 是 base64 的 ":"，即 method 与 password 全空）。本实现保留段落与行形态，
// 但把凭据补齐——产物是可用的，这是有意偏离，见 docs/compatibility.md 的 C8。
package mellow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/sharelink"
)

// BuiltinEndpoints 是 Mellow 自带的三个内置端点，必须在 [Endpoint] 段最前面。
// 行形态与参考输出逐字一致：名字, 协议, URI, 附加选项。
var BuiltinEndpoints = []string{
	"DIRECT, builtin, freedom, domainStrategy=UseIP",
	"REJECT, builtin, blackhole",
	"Dns-Out, builtin, dns",
}

// Options 控制渲染选项。
type Options struct {
	// DomainStrategy 域名解析策略：UseIP / UseIPv4Only / UseIPv6Only /
	// IPIfNonMatch / IPIfMatch / UseIPv4AndIPv6Only。
	DomainStrategy string
	// LogLevel 日志级别：silent / error / warning / info / verbose。
	LogLevel string
	// DNSClientIP 本机 DNS 客户端 IP，用于 hijack 场景的本机回环。
	DNSClientIP string
	// DNS 服务器列表，写入 [DnsServer] 段；空则不写该段。
	DNS []string
	// ManagedConfigURL 非空时在产物首行写 #!MANAGED-CONFIG，供 Mellow 自动更新。
	ManagedConfigURL string
	// UpdateInterval 自动更新间隔秒数。
	UpdateInterval int
}

// DefaultOptions 给一份保守可用的默认值。
func DefaultOptions() Options {
	return Options{
		DomainStrategy: "IPIfNonMatch",
		LogLevel:       "warning",
		DNSClientIP:    "114.114.114.114",
	}
}

// Renderer 渲染 Mellow 配置。
type Renderer struct {
	opts  Options
	warns []string
}

// New 构造渲染器，空字段用默认值补齐。
func New(opts Options) *Renderer {
	d := DefaultOptions()
	if opts.DomainStrategy == "" {
		opts.DomainStrategy = d.DomainStrategy
	}
	if opts.LogLevel == "" {
		opts.LogLevel = d.LogLevel
	}
	if opts.DNSClientIP == "" {
		opts.DNSClientIP = d.DNSClientIP
	}
	return &Renderer{opts: opts}
}

// Name 返回目标名。
func (r *Renderer) Name() string { return "mellow" }

// Render 产出 Mellow 配置文本。段落顺序与参考输出一致。
func (r *Renderer) Render(req render.Request) ([]byte, error) {
	r.warns = nil
	opts := r.opts

	var b strings.Builder
	section := func(name string, lines []string) {
		if len(lines) == 0 {
			return
		}
		b.WriteString("[" + name + "]\n")
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	if opts.ManagedConfigURL != "" {
		iv := opts.UpdateInterval
		if iv <= 0 {
			iv = 86400
		}
		b.WriteString(fmt.Sprintf("#!MANAGED-CONFIG %s interval=%d strict=false\n\n",
			opts.ManagedConfigURL, iv))
	}

	// 节点与内置端点。产物里真实存在的名字先收集起来，
	// 代理组要拿它过滤掉引用了被跳过节点的成员。
	endpoints := append([]string{}, BuiltinEndpoints...)
	valid := map[string]bool{"DIRECT": true, "REJECT": true, "Dns-Out": true}
	for _, g := range req.Groups {
		valid[g.Name] = true
	}
	for _, n := range req.Nodes {
		// 先查协议映射再编链接：sharelink 支持的协议 mellow 全都认，
		// 顺序放反了的话协议不支持的分支永远走不到。
		proto := mellowProtocol[strings.ToLower(strings.TrimSpace(n.Type))]
		if proto == "" {
			r.warns = append(r.warns, fmt.Sprintf("节点 %q（%s）在 Mellow 中没有对应协议，已跳过",
				label(n), n.Type))
			continue
		}
		link, err := sharelink.Render(n)
		if err != nil {
			r.warns = append(r.warns, fmt.Sprintf("节点 %q（%s）无法生成分享链接，已跳过：%v",
				label(n), n.Type, err))
			continue
		}
		endpoints = append(endpoints, fmt.Sprintf("%s, %s, %s", n.Name, proto, link))
		valid[n.Name] = true
	}

	// 默认组与规则在节点渲染之后注入：要引用的是真实存在的名字。
	if len(req.Groups) == 0 {
		req.Groups = defaultGroups(valid)
	}
	if len(req.Rules) == 0 {
		req.Rules = defaultRules(req.Groups)
	}

	groups := make([]string, 0, len(req.Groups))
	for _, g := range req.Groups {
		gline := groupLine(g, valid)
		if gline == "" {
			r.warns = append(r.warns, fmt.Sprintf("代理组 %q（%s）在 Mellow 中无法表达，已跳过",
				g.Name, g.Type))
			continue
		}
		groups = append(groups, gline)
	}

	rules := make([]string, 0, len(req.Rules))
	for _, rule := range req.Rules {
		out, err := ruleLine(rule)
		if err != nil {
			r.warns = append(r.warns, fmt.Sprintf("规则 %q 在 Mellow 中无法表达，已跳过：%v", rule, err))
			continue
		}
		if out != "" {
			rules = append(rules, out)
		}
	}

	section("Endpoint", endpoints)

	routing := []string{"domainStrategy=" + opts.DomainStrategy}
	section("Routing", routing)

	if opts.DNSClientIP != "" {
		section("Dns", []string{
			"clientIp=" + opts.DNSClientIP,
			"hijack=Dns-Out",
		})
	}
	section("DnsServer", opts.DNS)

	if opts.LogLevel != "" {
		section("Log", []string{"loglevel=" + opts.LogLevel})
	}
	section("EndpointGroup", groups)
	section("RoutingRule", rules)

	body := []byte(b.String())
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	return body, nil
}

// Warnings 返回最近一次 Render 中被跳过的节点、组与规则。
func (r *Renderer) Warnings() []string { return r.warns }

// mellowProtocol 把内核类型名映射成 Mellow 的协议标签。空串表示无法表达。
// 与 sharelink.Supported() 保持一致的覆盖面：能出分享链接的协议都能导出。
var mellowProtocol = map[string]string{
	"ss":           "ss",
	"shadowsocks":  "ss",
	"ssr":          "ssr",
	"shadowsocksr": "ssr",
	"vmess":        "vmess",
	"vless":        "vless",
	"trojan":       "trojan",
	"hysteria":     "hysteria",
	"hysteria2":    "hysteria2",
	"tuic":         "tuic",
	"anytls":       "anytls",
	"socks5":       "socks",
	"http":         "http",
	"https":        "http",
}

// groupLine 把一个代理组渲染成一行。
//
// 行形态：名字, 成员1:成员2, 模式, 选项。
// 模式只有三个：static（手动选择）、latency（测速择优）、fallback（故障回退）。
// timeout 单位是秒——Clash 侧是毫秒，得换算，不换算会等到一小时才超时。
func groupLine(g render.Group, valid map[string]bool) string {
	members := make([]string, 0, len(g.Proxies))
	for _, p := range g.Proxies {
		if valid[p] {
			members = append(members, p)
		}
	}
	if len(members) == 0 {
		return ""
	}

	typ := strings.ToLower(strings.TrimSpace(g.Type))
	if typ == "" {
		typ = "select"
	}
	switch typ {
	case "select":
		return fmt.Sprintf("%s, %s, static", g.Name, strings.Join(members, ":"))
	case "url-test":
		iv := g.Options.Interval
		if iv <= 0 {
			iv = 300
		}
		timeout := g.Options.Timeout
		if timeout <= 0 {
			timeout = 6000
		}
		// 毫秒转秒，四舍五入到整数；小于 1 秒按 1 秒算，0 秒会被当成不超时。
		secs := (timeout + 500) / 1000
		if secs < 1 {
			secs = 1
		}
		return fmt.Sprintf("%s, %s, latency, interval=%d, timeout=%d",
			g.Name, strings.Join(members, ":"), iv, secs)
	case "fallback":
		return fmt.Sprintf("%s, %s, fallback", g.Name, strings.Join(members, ":"))
	}
	return ""
}

// mellowRules 是 Mellow 认识的规则类型。
// GEOSITE 是内核专属的 mmdb 数据源，Mellow 没有这份数据，必须丢弃。
// RULE-SET 没有可核实的参考输出，同样丢弃而不是猜语法。
var mellowRules = map[string]bool{
	"DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
	"IP-CIDR": true, "IP-CIDR6": true,
	"IP-CIDRNOLOG": true, "IP-CIDR6NOLOG": true,
	"DST-PORT": true, "SRC-PORT": true,
	"GEOIP": true, "PROCESS-NAME": true, "URL-REGEX": true,
	"FINAL": true, "MATCH": true,
}

// ruleLine 把一条 Clash 规则渲染成 Mellow 的 RoutingRule 行。
//
// 返回空串表示该规则被主动丢弃（调用方计为告警）；
// 返回错误同样是丢弃，但错误信息能指出为什么。
func ruleLine(rule string) (string, error) {
	parts := strings.Split(rule, ",")
	if len(parts) < 2 {
		return "", fmt.Errorf("格式不完整")
	}
	typ := strings.ToUpper(strings.TrimSpace(parts[0]))

	switch typ {
	case "GEOSITE":
		return "", fmt.Errorf("GEOSITE 依赖内核 mmdb 数据，Mellow 没有对应数据源")
	case "RULE-SET":
		return "", fmt.Errorf("Mellow 的远程规则集语法未核实，不猜测输出")
	case "MATCH":
		// MATCH 是内核专属收尾语法，Mellow 只认 FINAL。
		return "FINAL," + strings.Join(parts[1:], ","), nil
	default:
		if !mellowRules[typ] {
			return "", fmt.Errorf("Mellow 不认识规则类型 %q", typ)
		}
		out := rule
		if typ == "IP-CIDR" || typ == "IP-CIDR6" {
			out += ",no-resolve"
		}
		return out, nil
	}
}

// defaultGroups 生成最小可用的两组结构（无组配置时的兜底）。
// 成员只能引用 valid 里的名字——valid 已经过滤掉了无法表达的节点。
// 组名刻意不用 emoji：这是本仓所有非 Clash 渲染器生成的默认组的一致约定。
func defaultGroups(valid map[string]bool) []render.Group {
	// 只看真实节点名：内置伪节点不算，否则无节点时也会生成一组只有 DIRECT 的
	// 空组——那种组在客户端里点不开，比不生成更糟。
	names := validNames(valid)
	if len(names) == 0 {
		return nil
	}
	members := make([]string, 0, len(names)+1)
	if valid["DIRECT"] {
		members = append(members, "DIRECT")
	}
	members = append(members, names...)
	return []render.Group{
		{Name: "节点选择", Type: "select", Proxies: members},
		{Name: "自动测速", Type: "url-test", Proxies: members,
			Test:    "https://www.google.com/generate_204",
			Options: render.UrlTestOptions{Interval: 300, Tolerance: 50, Timeout: 6000}},
	}
}

// defaultRules 生成最小可用的兜底规则。收尾写 FINAL 而不是 MATCH，
// 见 ruleLine 的说明。
func defaultRules(groups []render.Group) []string {
	catchAll := "DIRECT"
	for _, g := range groups {
		if strings.EqualFold(g.Type, "select") {
			catchAll = g.Name
			break
		}
	}
	return []string{"GEOIP,CN,DIRECT", "FINAL," + catchAll}
}

// validNames 返回 valid 里除内置伪节点外的名字，按字典序保证 golden 稳定。
var builtinNames = map[string]bool{"DIRECT": true, "REJECT": true, "Dns-Out": true}

func validNames(valid map[string]bool) []string {
	names := make([]string, 0, len(valid))
	for name := range valid {
		if !builtinNames[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func label(n model.Node) string {
	if n.Name != "" {
		return n.Name
	}
	return "<unnamed " + n.Type + ">"
}
