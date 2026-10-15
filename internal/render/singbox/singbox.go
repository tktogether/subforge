// Package singbox 渲染 sing-box JSON 配置。
//
// 节点凭据同样只来自 Node.Raw（docs/adr/0001），但两套内核的出站 schema
// 完全不同：Clash 用平铺键加自定键名，sing-box 用 type/tag 打头的出站加
// tls / transport 两个子对象。所以这里必须逐协议映射，不能像 clashmeta
// 那样直接透传 raw map。
//
// sing-box 侧的出站 schema 按 v1.10+（type / tag 取代旧的 server / port
// 平铺写法）；JSON 编码复用 render.MarshalJSONDoc，避免 encoding/json 按
// 字典序排键把 golden 弄乱。
package singbox

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/ruleset"
	"gopkg.in/yaml.v3"
)

// Options 是 sing-box 渲染器的输出配置。
type Options struct {
	// LogLevel 日志级别：silent / error / warning / info / debug / trace。
	LogLevel string
	// Timestamp 日志带时间戳。
	Timestamp bool
	// MixedPort mixed inbound 监听端口；0 表示不写该 inbound。
	MixedPort int
	// AllowLan 监听 0.0.0.0 而不是 127.0.0.1。
	AllowLan bool
	// Tun 是否启用 TUN inbound（全系统接管）。
	Tun bool
	// TunAddress TUN 的 IPv4 地址与掩码。
	TunAddress string
	// TunStack TUN 协议栈：system / gvisor / mixed。
	TunStack string
	// DNS DNS 段配置。
	DNS DNSOptions
}

// DNSOptions 是最小可用的 DNS 配置。
//
// 服务器顺序刻意与 subconverter 实机产物一致（dns_proxy → dns_direct →
// dns_fakeip → dns_resolver → block）：fakeip 要能工作，必须有一个不依赖
// fakeip 的 resolver 先就位，顺序乱了解析会自我依赖。
type DNSOptions struct {
	// Servers 域名服务器，顺序即解析依赖顺序。
	Servers []DNSServer
	// DefaultDNSTag 任何出站都能用的解析器。
	DefaultDNSTag string
	// FinalDNSTag 所有规则都不命中时用的解析器（dns.final）。
	FinalDNSTag string
	// ResolverDNSTag 解析 DNS 服务器自身地址用的解析器。
	ResolverDNSTag string
	// ProxyDNSTag 走代理解析用的解析器。
	ProxyDNSTag string
	// BlockTag 丢弃解析用的解析器。
	BlockTag string
	// FakeIPRange fakeip 地址池。
	FakeIPRange string
}

// DNSServer 是 dns.servers 里的一项。
type DNSServer struct {
	// Tag 服务标识，被 dns.rules / dns.final 引用。
	Tag string
	// Address 服务器地址（tls:// / h3:// / fakeip / rcode://success）。
	Address string
	// Resolver 该服务器自身地址用哪个解析器解析。
	Resolver string
	// Detour 该服务器走哪个出站；空表示默认出站。
	Detour string
}

// DefaultOptions 给出保守可用默认值。
func DefaultOptions() Options {
	return Options{
		LogLevel:   "info",
		Timestamp:  true,
		MixedPort:  2080,
		AllowLan:   true,
		Tun:        true,
		TunAddress: "172.19.0.1/30",
		TunStack:   "mixed",
		DNS: DNSOptions{
			Servers: []DNSServer{
				{Tag: "dns_proxy", Address: "tls://1.1.1.1", Resolver: "dns_resolver"},
				{Tag: "dns_direct", Address: "h3://dns.alidns.com/dns-query", Detour: "DIRECT"},
				{Tag: "dns_fakeip", Address: "fakeip"},
				{Tag: "dns_resolver", Address: "223.5.5.5", Detour: "DIRECT"},
				{Tag: "block", Address: "rcode://success"},
			},
			DefaultDNSTag:  "dns_resolver",
			FinalDNSTag:    "dns_direct",
			ResolverDNSTag: "dns_resolver",
			ProxyDNSTag:    "dns_proxy",
			BlockTag:       "block",
			FakeIPRange:    "198.18.0.0/15",
		},
	}
}

// New 返回给定选项的渲染器。
func New(opts Options) *Renderer { return &Renderer{opts: opts} }

// Renderer 渲染 sing-box JSON。
type Renderer struct {
	opts  Options
	warns []string
}

// Name 目标名。
func (r *Renderer) Name() string { return "sing-box" }

// Warnings 本次渲染被丢弃的节点与规则；未渲染前为空。
func (r *Renderer) Warnings() []string { return r.warns }

// Render 产出 sing-box JSON 配置。
func (r *Renderer) Render(req render.Request) ([]byte, error) {
	r.warns = nil

	// 出站与组都按"有效 tag"记账：组里引用了不存在的节点时，客户端整份配置
	// 会加载失败，比不生成更糟。
	valid := map[string]bool{}
	var outs []*yaml.Node
	addOut := func(typ, tag string) error {
		n, err := render.MappingNode("type", typ, "tag", tag)
		if err != nil {
			return fmt.Errorf("构内置出站 %s: %w", tag, err)
		}
		outs = append(outs, &n)
		valid[tag] = true
		return nil
	}
	for _, b := range []struct {
		typ, tag string
	}{
		{"direct", "DIRECT"},
		{"block", "REJECT"},
		{"dns", "dns-out"},
	} {
		if err := addOut(b.typ, b.tag); err != nil {
			return nil, err
		}
	}

	for _, n := range req.Nodes {
		node, err := r.outbound(n)
		if err != nil {
			r.warns = append(r.warns, err.Error())
			continue
		}
		if node == nil {
			continue
		}
		outs = append(outs, node)
		valid[n.Name] = true
	}

	groups, err := r.groupNodes(req.Groups, valid)
	if err != nil {
		return nil, err
	}
	outs = append(outs, groups.nodes...)
	for _, g := range groups.defined {
		valid[g.name] = true
	}

	// route.final 指向第一个手动选择组；没有组就直连。
	finalTag := "DIRECT"
	for _, g := range groups.defined {
		if g.selector {
			finalTag = g.name
			break
		}
	}

	rulesNode, err := r.routeRules(req.Rules, finalTag)
	if err != nil {
		return nil, err
	}

	logNode, err := render.MappingNode(
		"disabled", false,
		"level", r.opts.LogLevel,
		"timestamp", r.opts.Timestamp,
	)
	if err != nil {
		return nil, fmt.Errorf("构 log 段: %w", err)
	}

	dnsNode, err := r.dnsNode()
	if err != nil {
		return nil, fmt.Errorf("构 dns 段: %w", err)
	}

	ntpNode, err := render.MappingNode(
		"enabled", true,
		"server", "time.apple.com",
		"server_port", 123,
		"interval", "30m",
		"detour", "DIRECT",
	)
	if err != nil {
		return nil, fmt.Errorf("构 ntp 段: %w", err)
	}

	inboundsNode, err := r.inboundsNode()
	if err != nil {
		return nil, fmt.Errorf("构 inbounds 段: %w", err)
	}

	routeNode, err := render.MappingNode(
		"rules", rulesNode,
		"auto_detect_interface", true,
		"final", finalTag,
	)
	if err != nil {
		return nil, fmt.Errorf("构 route 段: %w", err)
	}

	cacheNode, err := render.MappingNode("enabled", true, "store_fakeip", true)
	if err != nil {
		return nil, fmt.Errorf("构 experimental.cache_file: %w", err)
	}
	apiNode, err := render.MappingNode(
		"external_controller", "127.0.0.1:9090",
		"external_ui", "dashboard",
	)
	if err != nil {
		return nil, fmt.Errorf("构 experimental.clash_api: %w", err)
	}
	expNode, err := render.MappingNode("cache_file", &cacheNode, "clash_api", &apiNode)
	if err != nil {
		return nil, fmt.Errorf("构 experimental 段: %w", err)
	}

	root, err := render.MappingNode(
		"log", &logNode,
		"dns", &dnsNode,
		"ntp", &ntpNode,
		"inbounds", &inboundsNode,
		"outbounds", render.SequenceNode(outs...),
		"route", &routeNode,
		"experimental", &expNode,
	)
	if err != nil {
		return nil, fmt.Errorf("构配置树: %w", err)
	}

	// JSON 不支持注释，生成时间一类元信息无处可放，只留在 HTTP 响应头里。
	return render.MarshalJSONDoc(root)
}

// outbound 把一个 Clash 形态的节点映射成 sing-box 出站。
// 返回 (nil, nil) 表示该协议 sing-box 没有对应实现。
func (r *Renderer) outbound(n model.Node) (*yaml.Node, error) {
	typ := singboxType[strings.ToLower(strings.TrimSpace(n.Type))]
	if typ == "" {
		// 该协议 sing-box 没有对应出站实现，跳过并进告警。
		return nil, fmt.Errorf("sing-box 不支持协议 %q，已跳过节点 %s", n.Type, n.Name)
	}
	raw := n.Raw

	entries := make([]any, 0, 14)
	add := func(k string, v any) { entries = append(entries, k, v) }
	add("type", typ)
	add("tag", n.Name)
	add("server", rawStr(raw, "server"))
	add("server_port", rawInt(raw, "port", 0))

	switch typ {
	case "shadowsocks":
		add("method", rawStr(raw, "cipher"))
		add("password", rawStr(raw, "password"))
	case "shadowsocksr":
		add("method", rawStr(raw, "cipher"))
		add("password", rawStr(raw, "password"))
		add("protocol", rawStrOr(raw, "origin", "protocol"))
		add("protocol_param", "")
		add("obfs", rawStrOr(raw, "plain", "obfs"))
		add("obfs_param", "")
	case "vmess":
		add("uuid", rawStr(raw, "uuid"))
		add("alter_id", rawInt(raw, "alterId", 0))
		// Clash 的 vmess 加密写 cipher；sing-box 叫 security，取值域相同。
		add("security", rawStrOr(raw, "auto", "encryption", "cipher"))
	case "vless":
		add("uuid", rawStr(raw, "uuid"))
		if f := rawStr(raw, "flow"); f != "" {
			add("flow", f)
		}
	case "trojan":
		add("password", rawStr(raw, "password"))
	case "hysteria2":
		add("password", rawStr(raw, "password"))
		add("up", rawStr(raw, "up"))
		add("down", rawStr(raw, "down"))
	case "hysteria":
		add("password", rawStr(raw, "password"))
		add("udp", true)
		add("congestion_controller", rawStrOr(raw, "bbr", "congestion-controller"))
		add("alpn", rawStrList(raw, "alpn"))
	case "tuic":
		add("uuid", rawStr(raw, "uuid"))
		add("password", rawStr(raw, "password"))
		add("udp", true)
		add("congestion_controller", rawStrOr(raw, "bbr", "congestion-controller"))
		add("alpn", rawStrList(raw, "alpn"))
	case "anytls":
		// Clash 的 anytls 把用户名写在 username，缺省时回落到 password。
		add("username", rawStrOr(raw, rawStr(raw, "password"), "username"))
		add("password", rawStr(raw, "password"))
		add("udp", true)
	case "socks", "http":
		if u := rawStr(raw, "username"); u != "" {
			add("username", u)
		}
		if p := rawStr(raw, "password"); p != "" {
			add("password", p)
		}
	}

	// Clash 只对部分协议显式写 udp；hysteria2 在 sing-box 侧必须自己声明
	// udp，否则 UDP 流量会被当成 TCP 转发。
	if typ == "hysteria2" {
		add("udp", true)
	} else if rawBool(raw, "udp") {
		add("udp", true)
	}

	if transport, ok := r.transportNode(raw); ok {
		add("transport", &transport)
	}
	if rp := r.realityNode(raw); rp != nil {
		add("reality", rp)
	}
	if tls, ok := r.tlsNode(raw); ok {
		add("tls", &tls)
	}

	node, err := render.MappingNode(entries...)
	if err != nil {
		return nil, fmt.Errorf("构出站 %s: %w", n.Name, err)
	}
	return &node, nil
}

// tlsNode 组装 tls 子对象。server_name 优先取 sni；Clash 的 vmess / vless
// 传统上写 servername，其余协议写 sni。
//
// 部分协议（hysteria2 / trojan / anytls）在 Clash 侧没有 tls 布尔位，
// TLS 是协议固有属性，因此 sni 非空即视为启用。
func (r *Renderer) tlsNode(raw map[string]any) (yaml.Node, bool) {
	sni := rawStr(raw, "sni")
	if sni == "" {
		sni = rawStr(raw, "servername")
	}
	enabled := rawBool(raw, "tls")
	if !enabled {
		if _, ok := raw["reality-opts"]; ok {
			enabled = true
		} else {
			enabled = sni != ""
		}
	}
	if !enabled && sni == "" {
		return yaml.Node{}, false
	}
	node, err := render.MappingNode("enabled", enabled)
	if err != nil {
		return node, false
	}
	if sni != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "server_name"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: sni},
		)
	}
	if rawBool(raw, "skip-cert-verify") {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "insecure"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: "true"},
		)
	}
	if fp := rawStr(raw, "client-fingerprint"); fp != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "utls"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fp},
		)
	} else if fp := rawStr(raw, "fingerprint"); fp != "" {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "utls"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: fp},
		)
	}
	return node, true
}

// transportNode 把 Clash 的 network / ws-opts / grpc-opts / h2-opts 映射到
// sing-box 的 transport 子对象。tcp 不带 path 与 header，不产出子对象。
func (r *Renderer) transportNode(raw map[string]any) (yaml.Node, bool) {
	var node yaml.Node
	switch strings.ToLower(rawStr(raw, "network")) {
	case "ws":
		opts, _ := raw["ws-opts"].(map[string]any)
		var err error
		node, err = render.MappingNode("type", "ws", "path", rawStrOr(opts, "", "path"))
		if err != nil {
			return node, false
		}
		if host := wsHeaderHost(opts); host != "" {
			h, err := render.MappingNode("Host", host)
			if err != nil {
				return node, false
			}
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "headers"},
				&h,
			)
		}
		return node, true
	case "grpc":
		opts, _ := raw["grpc-opts"].(map[string]any)
		var err error
		node, err = render.MappingNode("type", "grpc", "service_name", rawStrOr(opts, "", "path"))
		if err != nil {
			return node, false
		}
		if rawBool(opts, "multiMode") {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "multi_mode"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: "true"},
			)
		}
		return node, true
	case "h2", "httpupgrade":
		opts, _ := raw["h2-opts"].(map[string]any)
		var err error
		node, err = render.MappingNode("type", "httpupgrade", "path", rawStrOr(opts, "", "path"))
		if err != nil {
			return node, false
		}
		host := rawStrOr(opts, "", "host")
		if host == "" {
			host = rawStr(raw, "h2Host")
		}
		if host != "" {
			h, err := render.MappingNode("Host", host)
			if err != nil {
				return node, false
			}
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "headers"},
				&h,
			)
		}
		return node, true
	}
	return node, false
}

// realityNode 组装 vless-reality 的 reality 子对象；无配置时返回 nil。
func (r *Renderer) realityNode(raw map[string]any) *yaml.Node {
	opts, ok := raw["reality-opts"].(map[string]any)
	if !ok {
		return nil
	}
	entries := []any{
		"public_key", rawStr(opts, "public-key"),
		"short_id", rawStr(opts, "short-id"),
	}
	if list, ok := opts["spider-x"].([]any); ok && len(list) > 0 {
		entries = append(entries, "spider_x", render.StringSequence(anyToStrings(list)))
	}
	n, err := render.MappingNode(entries...)
	if err != nil {
		return nil
	}
	return &n
}

// groupsResult 是组映射结果：已构建的出站节点与已定义的组。
type groupsResult struct {
	nodes   []*yaml.Node
	defined []definedGroup
}

type definedGroup struct {
	name     string
	selector bool
}

// groupNodes 把 Clash 组映射成 sing-box 出站。
// sing-box 的 urltest 支持 tolerance 但不支持 timeout，fallback 只支持
// interval——不支持的参数静默丢弃（见 docs/compatibility.md）。
func (r *Renderer) groupNodes(groups []render.Group, valid map[string]bool) (groupsResult, error) {
	res := groupsResult{}
	for _, g := range groups {
		var members []string
		for _, m := range g.Proxies {
			if valid[m] {
				members = append(members, m)
			}
		}
		if len(members) == 0 {
			continue
		}
		seq := render.StringSequence(members)
		var n yaml.Node
		var err error
		switch strings.ToLower(g.Type) {
		case "select", "relay", "direct", "pass":
			// sing-box 没有 relay 出站；relay 组退化成 selector，成员顺序仍在。
			n, err = render.MappingNode("type", "selector", "tag", g.Name, "outbounds", seq)
		case "url-test":
			test := g.Test
			if test == "" {
				test = "http://www.gstatic.com/generate_204"
			}
			n, err = render.MappingNode(
				"type", "urltest",
				"tag", g.Name,
				"outbounds", seq,
				"url", test,
				"tolerance", g.Options.Tolerance,
				"interval", duration(g.Options.Interval),
			)
		case "fallback":
			n, err = render.MappingNode(
				"type", "fallback",
				"tag", g.Name,
				"outbounds", seq,
				"interval", duration(g.Options.Interval),
			)
		default:
			r.warns = append(r.warns,
				fmt.Sprintf("sing-box 无对应出站类型 %q，已跳过组 %s", g.Type, g.Name))
			continue
		}
		if err != nil {
			return res, fmt.Errorf("构组 %s: %w", g.Name, err)
		}
		isSelector := strings.ToLower(g.Type) == "select" || strings.ToLower(g.Type) == "relay"
		res.nodes = append(res.nodes, &n)
		res.defined = append(res.defined, definedGroup{name: g.Name, selector: isSelector})
	}
	return res, nil
}

// routeRules 把 Clash 风格规则行映射成 sing-box 的 route.rules。
// 同一字段、同一出站的相邻行合并进同一个对象的数组——sing-box 的 rule 是
// "一个对象可带多个数组"，逐行拆开会产出一串重复字段。但不同字段不能合并：
// domain 类与 geoip 的匹配阶段不同，合并会改变命中顺序。
func (r *Renderer) routeRules(rules []string, finalTag string) (*yaml.Node, error) {
	items := make([]*yaml.Node, 0, len(rules))
	var openEntries []any
	openField, openTag := "", ""
	commit := func() {
		if len(openEntries) == 0 {
			return
		}
		n, err := render.MappingNode(openEntries...)
		if err != nil {
			r.warns = append(r.warns, fmt.Sprintf("构规则对象失败，已丢弃: %s", openTag))
			openEntries = nil
			return
		}
		items = append(items, &n)
		openEntries = nil
	}
	for _, line := range rules {
		rule, err := ruleset.ParseLine(line)
		if err != nil {
			r.warns = append(r.warns, fmt.Sprintf("无法解析的规则行，已丢弃: %s", line))
			continue
		}
		// MATCH / FINAL 不落成规则行：sing-box 用 route.final 表达兜底出站。
		if rule.Kind == ruleset.Match || rule.Kind == ruleset.Final {
			if rule.Action != "" {
				finalTag = rule.Action
			}
			continue
		}
		field, ok := ruleField[rule.Kind]
		if !ok {
			r.warns = append(r.warns,
				fmt.Sprintf("sing-box 无对应字段 %q，已丢弃规则: %s", rule.Kind, line))
			continue
		}
		tag := rule.Action
		if tag == "" {
			tag = finalTag
		}
		if len(openEntries) == 0 {
			openEntries = []any{field, []string{rule.Value}, "outbound", tag}
			openField, openTag = field, tag
		} else if field != openField || tag != openTag {
			commit()
			openEntries = []any{field, []string{rule.Value}, "outbound", tag}
			openField, openTag = field, tag
		} else {
			values := openEntries[1].([]string)
			openEntries[1] = append(values, rule.Value)
		}
	}
	commit()
	node := render.SequenceNode(items...)
	return &node, nil
}

// dnsNode 构造 dns 段。
func (r *Renderer) dnsNode() (yaml.Node, error) {
	d := r.opts.DNS
	var servers []*yaml.Node
	for _, s := range d.Servers {
		n, err := render.MappingNode("tag", s.Tag, "address", s.Address)
		if err != nil {
			return n, err
		}
		if s.Resolver != "" {
			n.Content = append(n.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "address_resolver"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: s.Resolver},
			)
		}
		if s.Detour != "" {
			n.Content = append(n.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "detour"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: s.Detour},
			)
		}
		servers = append(servers, &n)
	}

	var dnsRules []*yaml.Node
	addRule := func(entries ...any) {
		n, err := render.MappingNode(entries...)
		if err != nil {
			return
		}
		dnsRules = append(dnsRules, &n)
	}
	addRule("outbound", render.StringSequence([]string{"any"}), "server", d.DefaultDNSTag)
	addRule("geosite", render.StringSequence([]string{"category-ads-all"}),
		"server", d.BlockTag, "disable_cache", true)
	addRule("geosite", render.StringSequence([]string{"geolocation-!cn"}),
		"query_type", render.StringSequence([]string{"A", "AAAA"}), "server", "dns_fakeip")
	addRule("geosite", render.StringSequence([]string{"geolocation-!cn"}), "server", d.ProxyDNSTag)

	fakeip, err := render.MappingNode("enabled", true, "inet4_range", d.FakeIPRange)
	if err != nil {
		return yaml.Node{}, err
	}

	root, err := render.MappingNode(
		"servers", render.SequenceNode(servers...),
		"rules", render.SequenceNode(dnsRules...),
		"final", d.FinalDNSTag,
		"independent_cache", true,
		"fakeip", &fakeip,
	)
	return root, err
}

// inboundsNode 构造 inbounds 段。
func (r *Renderer) inboundsNode() (yaml.Node, error) {
	var items []*yaml.Node
	if r.opts.MixedPort > 0 {
		n, err := render.MappingNode(
			"type", "mixed",
			"tag", "mixed-in",
			"listen", mixedListen(r.opts.AllowLan),
			"listen_port", r.opts.MixedPort,
		)
		if err != nil {
			return yaml.Node{}, err
		}
		items = append(items, &n)
	}
	if r.opts.Tun {
		n, err := render.MappingNode(
			"type", "tun",
			"tag", "tun-in",
			"inet4_address", r.opts.TunAddress,
			"auto_route", true,
			"strict_route", true,
			"stack", r.opts.TunStack,
			"sniff", true,
		)
		if err != nil {
			return yaml.Node{}, err
		}
		items = append(items, &n)
	}
	node := render.SequenceNode(items...)
	return node, nil
}

// singboxType 是 Clash 类型名到 sing-box 出站 type 的映射。
// 只有 shadowsocks / shadowsocksr 有真差异：Clash 简写 ss / ssr。
// socks5 在 sing-box 侧写作 socks；http 的 https 变体走 https 出站。
var singboxType = map[string]string{
	"ss":           "shadowsocks",
	"shadowsocks":  "shadowsocks",
	"ssr":          "shadowsocksr",
	"shadowsocksr": "shadowsocksr",
	"vmess":        "vmess",
	"vless":        "vless",
	"trojan":       "trojan",
	"hysteria":     "hysteria",
	"hysteria2":    "hysteria2",
	"tuic":         "tuic",
	"anytls":       "anytls",
	"socks5":       "socks",
	"socks":        "socks",
	"http":         "http",
	"https":        "https",
}

// ruleField 是规则形态到 sing-box 的 route rule 字段名。
// 未列入的形态没有对应字段，渲染时丢弃并计入告警：
//   - GEOSITE 依赖 sing-box 自己的 geoip/geosite 数据，与 mihomo 的不通用，
//     不能直接搬字段名，否则客户端按不存在的分类匹配。
//   - RULE-SET / SCRIPT / POLICY 是 mihomo / Clash 特有，无对应实现。
var ruleField = map[ruleset.Kind]string{
	ruleset.Domain:        "domain",
	ruleset.DomainSuffix:  "domain_suffix",
	ruleset.DomainKeyword: "domain_keyword",
	ruleset.DomainRegex:   "domain_regex",
	ruleset.IPCIDR:        "ip_cidr",
	ruleset.IPCIDR6:       "ip_cidr6",
	ruleset.GeoIP:         "geoip",
	ruleset.GeoSite:       "geosite",
}

// --- raw map 取值助手 ---

func rawStr(raw map[string]any, key string) string {
	if s, ok := raw[key].(string); ok {
		return s
	}
	return ""
}

// rawStrOr 按顺序取第一个非空字符串字段值；全空时回落到 fallback。
func rawStrOr(raw map[string]any, fallback string, keys ...string) string {
	for _, k := range keys {
		if s, ok := raw[k].(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func rawBool(raw map[string]any, key string) bool {
	b, _ := raw[key].(bool)
	return b
}

func rawInt(raw map[string]any, key string, fallback int) int {
	switch v := raw[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// rawStrList 把 raw 里的列表字段转成 []string；缺失时返回空列表，
// sing-box 的 alpn 字段允许空数组。
func rawStrList(raw map[string]any, key string) []string {
	var out []string
	switch v := raw[key].(type) {
	case []string:
		out = v
	case []any:
		out = anyToStrings(v)
	}
	return out
}

// wsHeaderHost 取 ws-opts.headers.Host——sing-box 的 ws 传输用 headers
// 子对象，而不是 Clash 那种 host 平铺字段。
func wsHeaderHost(opts map[string]any) string {
	h, ok := opts["headers"].(map[string]any)
	if !ok {
		return ""
	}
	return rawStrOr(h, "", "Host", "host")
}

func anyToStrings(items []any) []string {
	out := make([]string, 0, len(items))
	for _, e := range items {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// duration 把秒数转成 sing-box 的时长字面量；非正数用 300 秒兜底，
// 0 会被客户端当成不启用。
func duration(seconds int) string {
	if seconds <= 0 {
		return "300s"
	}
	return fmt.Sprintf("%ds", seconds)
}

func mixedListen(allowLan bool) string {
	if allowLan {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}
