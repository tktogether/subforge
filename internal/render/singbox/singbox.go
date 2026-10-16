// Package singbox 渲染 sing-box JSON 配置。
//
// 节点凭据只来自 Node.Raw（docs/adr/0001）。两套内核的出站 schema 完全不同：
// Clash 用平铺键加自定键名，sing-box 用 type / tag 打头的出站，TLS 与传输各
// 是一个子对象。所以这里必须逐协议映射，不能像 clashmeta 那样透传 raw map。
//
// 目标 schema 按 sing-box 1.14 对齐。1.12 起有整片破坏性变更，早期版本能加载
// 的字段在 1.14 已非法：
//
//   - dns.servers 每项是 type 判别式的对象（tls/https/h3/udp/tcp/fakeip/...），
//     旧的 address 字段与 dns.fakeip / dns.independent_cache 一并移除；
//     dns.default / dns.resolver / dns.proxy / dns.block 也移除了，兜底解析器
//     改写成一条匹配所有域名的 dns.rules 条目。
//   - dns 与 route 规则不再有内置 geoip / geosite 匹配，只能引用 route.rule_set
//     指向的外部规则集（第三方规则数据，见设计书 R3，本实现不打包）。
//   - 出站不再有 udp 布尔位（UDP 是所有出站的固有属性）。
//   - shadowsocksr 出站被移除；fallback 出站被移除；dns 出站被移除。
//   - tls.utls 从字符串变成 {enabled, fingerprint} 对象；reality 从出站顶层
//     移进 tls.reality；alpn 从出站顶层移进 tls.alpn；spider-x 被移除。
//   - inbound 的 mixed 用 listen / listen_port；tun 用 address，inet4_address /
//     stack / sniff 三个字段被移除。
//   - hysteria（v1）出站的凭据字段叫 auth_str 不叫 password；tuic 的
//     congestion-controller 改叫 congestion_control。
//   - DNS 服务器若以主机名接入（非 IP），必须声明 domain_resolver，1.12 起
//     弃用、1.14 起直接判配置非法。
//
// 规则动作是平铺字段，不是嵌套对象：route 规则写 outbound，DNS 规则写
// action + server。官方 schema.json 这里有个生成器造成的坑（RuleAction 经
// allOf 引用，oneOf 不向父级传播注解，外层 unevaluatedProperties:false 反而
// 把 action 拒了），按 schema 字面写 {action:{action:"route",outbound:...}}
// 会被 sing-box 1.14 判非法。以 sing-box check 实测为准。
//
// 每处与内核实际行为相关的取舍都记在 docs/compatibility.md 的 B1~B7。
// JSON 编码复用 render.MarshalJSONDoc，避免 encoding/json 按字典序排键把
// golden 弄乱。
package singbox

import (
	"encoding/base64"
	"fmt"
	"net"
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
	// DomainResolver 出站拨号主机名用的 DNS 服务器 tag，落成 route.default_domain_resolver。
	// 必须是能直连解析的服务器：拿代理自己的 DNS 去解析代理地址会绕回来。
	// 1.12 起弃用隐式默认值、1.14 起缺了就判配置非法。
	DomainResolver string
	// DNS DNS 段配置。
	DNS DNSOptions
}

// DNSOptions 是最小可用的 DNS 配置。
type DNSOptions struct {
	// Servers 域名服务器。支持两种写法：Address 是 tls:// / h3:// / udp:// /
	// rcode://success 这类旧式 URI（按 1.12 之前的地址写法解析），Type 是
	// 显式类型（tls / https / h3 / udp / tcp / fakeip）。Type 优先。
	Servers []DNSServer
	// DefaultDNSTag 兜底解析器，落成一条匹配所有域名的 route 规则——1.12 起
	// dns.default 字段本身被移除了，语义只能由规则表达。
	DefaultDNSTag string
	// FinalDNSTag 所有规则都不命中时用的解析器（dns.final）。
	FinalDNSTag string
}

// DNSServer 是 dns.servers 里的一项。
type DNSServer struct {
	// Tag 服务标识，被 dns.rules / dns.final 引用。
	Tag string
	// Type 服务器类型；空表示从 Address 推。
	Type string
	// Address 旧式地址写法：tls://host:port、h3://host:port/path、udp://ip:port、
	// udp:ip:port、host:port、ip、fakeip、rcode://success。
	Address string
	// Server 显式写法的服务器地址。
	Server string
	// Port 显式写法的端口；0 表示用协议默认值。
	Port int
	// Path https / h3 的路径。
	Path string
	// Resolver 解析该服务器自身地址用的解析器（domain_resolver）。
	Resolver string
	// Detour 该服务器走哪个出站。
	Detour string
	// ServerName TLS 的 SNI；空表示用 Server。
	ServerName string
	// Insecure 跳过证书校验。
	Insecure bool
	// FakeIPRange type 为 fakeip 时的地址池。
	FakeIPRange string
}

// DefaultOptions 给出保守可用默认值。
//
// 兜底解析器必须指向 dns_proxy 而不是 dns_resolver：dns_resolver 只有一个
// 用途，就是替 dns_proxy 解析 1.1.1.1 自己（否则整条链自我依赖）。兜底如果
// 指到它，所有域名都会绕过代理做直连解析，fakeip 也就永远不生效——境外的域名
// 拿到真实 IP，走不走代理就看运气了。代价是 CN 流量也会先绕代理解析一次，
// 这是 R3（不打包 geosite 分类）之下的必然取舍。
func DefaultOptions() Options {
	return Options{
		LogLevel:       "info",
		Timestamp:      true,
		MixedPort:      2080,
		AllowLan:       true,
		Tun:            true,
		TunAddress:     "172.19.0.1/30",
		DomainResolver: "dns_resolver",
		DNS: DNSOptions{
			Servers: []DNSServer{
				{Tag: "dns_proxy", Address: "tls://1.1.1.1", Resolver: "dns_resolver"},
				// dns.alidns.com 是主机名，必须声明用哪个 DNS 解析自己，否则
				// 1.14 起整份配置被判非法（deprecation 变 fatal）。
				{Tag: "dns_direct", Address: "h3://dns.alidns.com/dns-query",
					Detour: "DIRECT", Resolver: "dns_resolver"},
				{Tag: "dns_fakeip", Address: "fakeip", FakeIPRange: "198.18.0.0/15"},
				{Tag: "dns_resolver", Address: "223.5.5.5", Detour: "DIRECT"},
			},
			DefaultDNSTag: "dns_proxy",
			FinalDNSTag:   "dns_direct",
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

	rulesNode, err := r.routeRules(req.Rules, finalTag, valid)
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

	r.checkDNSRefs()

	routeEntries := []any{"rules", rulesNode, "auto_detect_interface", true}
	if r.opts.DomainResolver != "" {
		// 出站拨号主机名用什么 DNS 解析：1.12 起弃用隐式默认值、1.14 起缺了就
		// 判配置非法。必须是能直连解析的服务器——拿代理自己的 DNS 去解析代理
		// 地址会绕回来。
		routeEntries = append(routeEntries, "default_domain_resolver", r.opts.DomainResolver)
	}
	routeEntries = append(routeEntries, "final", finalTag)
	routeNode, err := render.MappingNode(routeEntries...)
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

	entries := make([]any, 0, 12)
	add := func(k string, v any) { entries = append(entries, k, v) }
	add("type", typ)
	add("tag", n.Name)
	add("server", rawStr(raw, "server"))
	add("server_port", rawInt(raw, "port", 0))

	switch typ {
	case "shadowsocks":
		add("method", rawStr(raw, "cipher"))
		add("password", rawStr(raw, "password"))
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
		if p := rawStr(raw, "password"); p != "" {
			add("password", p)
		}
		// hysteria2 的速度上限是可选字段，节点没写就不补。
		r.addBandwidth(add, n.Name, raw, false)
	case "hysteria":
		// hysteria（v1）出站的凭据字段叫 auth_str，不叫 password——写 password
		// 会被判为未知字段，整份配置加载失败。
		if p := rawStr(raw, "password"); p != "" {
			add("auth_str", p)
		}
		// 速度上限是必填，缺了就兜底并告警。
		r.addBandwidth(add, n.Name, raw, true)
		// 旧 congestion-controller 字段对 hysteria 不是合法字段，丢弃。
	case "tuic":
		add("uuid", rawStr(raw, "uuid"))
		if p := rawStr(raw, "password"); p != "" {
			add("password", p)
		}
		// Clash 叫 congestion-controller，sing-box 叫 congestion_control。
		add("congestion_control", rawStrOr(raw, "bbr", "congestion-controller"))
	case "anytls":
		// anytls 出站只有 password 一个凭据字段；Clash 有时把凭据写进
		// username，回落到 username 以免丢凭据。
		pw := rawStr(raw, "password")
		if pw == "" {
			pw = rawStr(raw, "username")
		}
		if pw != "" {
			add("password", pw)
		}
	case "socks", "http":
		if u := rawStr(raw, "username"); u != "" {
			add("username", u)
		}
		if p := rawStr(raw, "password"); p != "" {
			add("password", p)
		}
	}

	if transport, ok := r.transportNode(raw); ok {
		add("transport", transport)
	}
	if tls := r.tlsNode(n.Name, raw, typ, strings.EqualFold(n.Type, "https")); tls != nil {
		add("tls", tls)
	}

	node, err := render.MappingNode(entries...)
	if err != nil {
		return nil, fmt.Errorf("构出站 %s: %w", n.Name, err)
	}
	return &node, nil
}

// tlsNode 组装 tls 子对象，无内容时返回 nil。
//
// server_name 优先取 sni；Clash 的 vmess / vless 传统上写 servername，其余
// 协议写 sni。tls / reality 与 alpn 在 1.12 起都归到 tls 子对象里，reality
// 不再是出站的顶层字段。utls 也从字符串变成 {enabled, fingerprint} 对象。
//
// 部分协议（hysteria2 / trojan / anytls / https）在 Clash 侧没有 tls 布尔位，
// TLS 是协议固有属性，因此 sni 非空即视为启用。
func (r *Renderer) tlsNode(tag string, raw map[string]any, typ string, isHTTPS bool) *yaml.Node {
	sni := rawStr(raw, "sni")
	if sni == "" {
		sni = rawStr(raw, "servername")
	}
	opts, hasReality := raw["reality-opts"].(map[string]any)
	enabled := rawBool(raw, "tls") || hasReality || sni != "" || tlsInherent(typ, isHTTPS)
	if !enabled {
		return nil
	}

	entries := make([]any, 0, 8)
	add := func(k string, v any) { entries = append(entries, k, v) }
	add("enabled", true)
	if sni != "" {
		add("server_name", sni)
	}
	if rawBool(raw, "skip-cert-verify") || rawBool(raw, "skip-cert-verify-fallback") {
		add("insecure", true)
	}
	if fp := rawStr(raw, "client-fingerprint"); fp != "" {
		u, err := render.MappingNode("enabled", true, "fingerprint", fp)
		if err == nil {
			add("utls", &u)
		}
	} else if fp := rawStr(raw, "fingerprint"); fp != "" {
		u, err := render.MappingNode("enabled", true, "fingerprint", fp)
		if err == nil {
			add("utls", &u)
		}
	}
	if alpn := rawStrList(raw, "alpn"); len(alpn) > 0 {
		add("alpn", render.StringSequence(alpn))
	}
	if hasReality {
		// OutboundRealityOptions 只有 enabled / public_key / short_id，
		// 旧 spider-x 在 1.12 起被移除，只能丢弃。
		pub, bad := realityPublicKey(rawStr(opts, "public-key"))
		if bad {
			r.warns = append(r.warns, fmt.Sprintf(
				"节点 %s 的 reality public-key %q 不是合法 base64，已原样输出，sing-box 会判配置非法", tag, pub))
		}
		entries = append(entries,
			"reality",
			&yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "enabled"},
				{Kind: yaml.ScalarNode, Value: "true"},
				{Kind: yaml.ScalarNode, Value: "public_key"},
				{Kind: yaml.ScalarNode, Value: pub},
				{Kind: yaml.ScalarNode, Value: "short_id"},
				{Kind: yaml.ScalarNode, Value: rawStr(opts, "short-id")},
			}},
		)
	}

	node, err := render.MappingNode(entries...)
	if err != nil {
		return nil
	}
	return &node
}

// transportNode 把 Clash 的 network / ws-opts / grpc-opts / h2-opts 映射到
// sing-box 的 transport 子对象。tcp 不带 path 与 header，不产出子对象。
func (r *Renderer) transportNode(raw map[string]any) (*yaml.Node, bool) {
	switch strings.ToLower(rawStr(raw, "network")) {
	case "ws":
		opts, _ := raw["ws-opts"].(map[string]any)
		node, err := render.MappingNode("type", "ws", "path", rawStrOr(opts, "", "path"))
		if err != nil {
			return nil, false
		}
		// ws 传输没有 host 字段，Host 必须放进 headers 子对象。
		if h := headerHostNode(opts); h != nil {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "headers"},
				h,
			)
		}
		return &node, true
	case "grpc":
		opts, _ := raw["grpc-opts"].(map[string]any)
		node, err := render.MappingNode("type", "grpc", "service_name", rawStrOr(opts, "", "path"))
		if err != nil {
			return nil, false
		}
		// Clash 的 multiMode 在 sing-box 侧叫 permit_without_stream。
		if rawBool(opts, "multiMode") {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "permit_without_stream"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: "true"},
			)
		}
		return &node, true
	case "h2", "httpupgrade":
		opts, _ := raw["h2-opts"].(map[string]any)
		node, err := render.MappingNode("type", "httpupgrade", "path", rawStrOr(opts, "", "path"))
		if err != nil {
			return nil, false
		}
		host := rawStrOr(opts, "", "host")
		if host == "" {
			host = rawStr(raw, "h2Host")
		}
		if host != "" {
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "host"},
				&yaml.Node{Kind: yaml.ScalarNode, Value: host},
			)
		}
		return &node, true
	}
	return nil, false
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
//
// urltest 支持 tolerance 但不支持 timeout；fallback 出站类型在 1.12 起被移除，
// 只能退化成 selector——成员顺序与可达性保留，但失去了"逐个试探直到可用"的
// 语义，降级记入告警而不是静默丢弃（见 docs/compatibility.md）。
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
			r.warns = append(r.warns,
				fmt.Sprintf("sing-box 1.12 起已移除 fallback 出站，组 %s 退化为 selector（失去逐个试探语义）", g.Name))
			n, err = render.MappingNode("type", "selector", "tag", g.Name, "outbounds", seq)
		default:
			r.warns = append(r.warns,
				fmt.Sprintf("sing-box 无对应出站类型 %q，已跳过组 %s", g.Type, g.Name))
			continue
		}
		if err != nil {
			return res, fmt.Errorf("构组 %s: %w", g.Name, err)
		}
		// fallback 退化成 selector，在 route.final 选取"第一个手动选择组"时
		// 视为选择组，避免出现一个无人可用的兜底。
		isSelector := strings.ToLower(g.Type) == "select" || strings.ToLower(g.Type) == "relay" ||
			strings.ToLower(g.Type) == "fallback"
		res.nodes = append(res.nodes, &n)
		res.defined = append(res.defined, definedGroup{name: g.Name, selector: isSelector})
	}
	return res, nil
}

// routeRules 把 Clash 风格规则行映射成 sing-box 的 route.rules。
//
// 动作是平铺的 outbound 字段，不是嵌套的 action 对象。官方 schema.json 这里
// 有个生成器造成的坑：RuleAction 被 allOf 进来，而 oneOf 不向父级传播注解，
// 于是 action 在外层 unevaluatedProperties:false 下反而被拒——按 schema 写
// {action:{action:"route",outbound:...}} 会被 sing-box 1.14 直接判非法
// （"cannot unmarshal object into Go struct field ...action of type string"）。
// 实测以 sing-box check 为准。
//
// 同一匹配字段、同一出站的相邻行合并进同一个对象的数组——sing-box 的 rule 是
// "一个对象可带多个数组"，逐行拆开会产出一串重复字段。但不同字段不能合并：
// domain 类与 ip 的匹配阶段不同，合并会改变命中顺序。
func (r *Renderer) routeRules(rules []string, finalTag string, valid map[string]bool) (*yaml.Node, error) {
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
			// geoip / geosite 在 sing-box 1.12 起不再是内置匹配，只能引用
			// route.rule_set 指向的规则集文件；那些是第三方数据（R3），
			// 本实现不打包，因此只能丢弃并说明。
			r.warns = append(r.warns, fmt.Sprintf(
				"sing-box 1.12 起 %q 不再内置匹配，需自行提供 route.rule_set，已丢弃规则: %s", rule.Kind, line))
			continue
		}
		tag := rule.Action
		if tag == "" {
			tag = finalTag
		}
		if !valid[tag] {
			r.warns = append(r.warns, fmt.Sprintf(
				"规则引用的出站 %q 不存在（节点可能被跳过），已丢弃规则: %s", tag, line))
			continue
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
		n, err := r.dnsServerNode(s)
		if err != nil {
			return yaml.Node{}, err
		}
		servers = append(servers, n)
	}

	var dnsRules []*yaml.Node
	addRule := func(entries ...any) {
		n, err := render.MappingNode(entries...)
		if err != nil {
			return
		}
		dnsRules = append(dnsRules, &n)
	}
	if d.DefaultDNSTag != "" {
		// 1.12 起 dns.default 字段被移除，兜底解析器只能写成一条匹配所有域名
		// 的规则。放在最前，更具体的规则（用户自己补的 rule_set）放后面覆盖。
		// action 是平铺字符串，不是对象——同 route 规则一样的 schema 生成器坑。
		addRule("domain", render.StringSequence([]string{"*"}),
			"action", "route", "server", d.DefaultDNSTag)
	}
	// 旧实现这里还有 geosite 驱动的三条规则（广告丢弃、境外 fakeip、境外走
	// proxy）。geosite 分类是第三方规则数据（设计书 R3），本实现不打包，
	// 也就无法产出这三条——不是遗忘，是有意的取舍，记在 compatibility.md。

	root, err := render.MappingNode(
		"servers", render.SequenceNode(servers...),
		"rules", render.SequenceNode(dnsRules...),
		"final", d.FinalDNSTag,
	)
	return root, err
}

// checkDNSRefs 校验 DNS 段引用的解析器 tag 都真的存在。
// 悬空引用会让客户端整份配置加载失败，所以必须告警而不是静默产出。
func (r *Renderer) checkDNSRefs() {
	d := r.opts.DNS
	tags := map[string]bool{}
	for _, s := range d.Servers {
		if s.Tag != "" {
			tags[s.Tag] = true
		}
	}
	missing := func(what, tag string) {
		if tag != "" && !tags[tag] {
			r.warns = append(r.warns, fmt.Sprintf(
				"%s 引用了不存在的 DNS 服务器 %q，sing-box 会判配置非法", what, tag))
		}
	}
	for _, s := range d.Servers {
		missing("DNS 服务器 ["+s.Tag+"] 的 domain_resolver", s.Resolver)
	}
	missing("兜底解析器", d.DefaultDNSTag)
	missing("dns.final", d.FinalDNSTag)
	missing("route.default_domain_resolver", r.opts.DomainResolver)
}

// dnsServerNode 按 1.12 的 type 判别式组装单个 dns.servers 项。
func (r *Renderer) dnsServerNode(s DNSServer) (*yaml.Node, error) {
	typ, server, port, path := s.Type, s.Server, s.Port, s.Path
	if typ == "" || (server == "" && path == "" && typ != "fakeip") {
		typ, server, port, path = parseDNSAddr(s.Address)
	}
	if server != "" {
		server, port = splitHostPort(server, port)
	}

	entries := make([]any, 0, 8)
	add := func(k string, v any) { entries = append(entries, k, v) }
	add("type", typ)
	if s.Tag != "" {
		add("tag", s.Tag)
	}

	if typ == "fakeip" {
		if s.FakeIPRange != "" {
			add("inet4_range", s.FakeIPRange)
		}
		if s.Resolver != "" {
			add("domain_resolver", s.Resolver)
		}
	} else {
		// 主机名服务器必须声明用哪个 DNS 解析自己：1.12 起弃用、1.14 起直接
		// 判配置非法。缺了就告警，不能静默产出加载失败的配置。
		if server != "" && s.Resolver == "" && net.ParseIP(server) == nil {
			r.warns = append(r.warns, fmt.Sprintf(
				"DNS 服务器 %q 的地址 %s 是主机名但未声明 domain_resolver，sing-box 1.14 会判配置非法", s.Tag, server))
		}
		if server != "" {
			add("server", server)
		}
		if port > 0 {
			add("server_port", port)
		}
		if path != "" {
			add("path", path)
		}
		if s.Resolver != "" {
			add("domain_resolver", s.Resolver)
		}
		if s.Detour != "" {
			add("detour", s.Detour)
		}
		if tls := dnsTLSNode(s); tls != nil {
			add("tls", tls)
		}
	}

	n, err := render.MappingNode(entries...)
	if err != nil {
		return nil, fmt.Errorf("构 DNS 服务器 %s: %w", s.Tag, err)
	}
	return &n, nil
}

// dnsTLSNode 组装 DNS 服务器的 tls 子对象。tls / https / h3 / quic 类型自带
// TLS，缺省 SNI 用服务器地址。
func dnsTLSNode(s DNSServer) *yaml.Node {
	if s.Server == "" && s.ServerName == "" && !s.Insecure {
		return nil
	}
	sn := s.ServerName
	if sn == "" {
		sn = s.Server
	}
	entries := make([]any, 0, 4)
	add := func(k string, v any) { entries = append(entries, k, v) }
	add("enabled", true)
	if sn != "" {
		add("server_name", sn)
	}
	if s.Insecure {
		add("insecure", true)
	}
	n, err := render.MappingNode(entries...)
	if err != nil {
		return nil
	}
	return &n
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
			"address", r.opts.TunAddress,
			"auto_route", true,
			"strict_route", true,
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
// 只有 shadowsocks 有真差异：Clash 简写 ss。
// ssr 故意不在此列——sing-box 1.12 起移除了 shadowsocksr 出站，
// 这些节点会被跳过并进告警，而不是映射到一个不存在的类型。
// socks5 在 sing-box 侧写作 socks；https 没有独立出站，走 http + tls。
var singboxType = map[string]string{
	"ss":          "shadowsocks",
	"shadowsocks": "shadowsocks",
	"vmess":       "vmess",
	"vless":       "vless",
	"trojan":      "trojan",
	"hysteria":    "hysteria",
	"hysteria2":   "hysteria2",
	"tuic":        "tuic",
	"anytls":      "anytls",
	"socks5":      "socks",
	"socks":       "socks",
	"http":        "http",
	"https":       "http",
}

// ruleField 是规则形态到 sing-box 的 route rule 字段名。
// 未列入的形态没有对应字段，渲染时丢弃并计入告警：
//   - GeoIP / GeoSite 在 1.12 起不再是内置匹配，只能引用 route.rule_set
//     指向的规则集文件；那是第三方规则数据（设计书 R3），本实现不打包。
//   - RULE-SET / SCRIPT / POLICY 是 mihomo / Clash 特有，无对应实现。
//   - IPCIDR6 并入 ip_cidr：1.12 起 ip_cidr 同时接受 v4 与 v6 前缀，
//     单独的 ip_cidr6 字段已移除。
var ruleField = map[ruleset.Kind]string{
	ruleset.Domain:        "domain",
	ruleset.DomainSuffix:  "domain_suffix",
	ruleset.DomainKeyword: "domain_keyword",
	ruleset.DomainRegex:   "domain_regex",
	ruleset.IPCIDR:        "ip_cidr",
	ruleset.IPCIDR6:       "ip_cidr",
}

// realityPublicKey 把 reality 的 X25519 公钥转成 sing-box 要的写法。
//
// Clash / V2Ray 生态一律用标准 base64（+ 与 / 字母表、= 填充），sing-box 1.14
// 只接受 URL 安全变体且无填充（- 与 _ 字母表、无 =）。写错不会报错，只会判
// "illegal base64 data"，整份配置加载失败。实测 14 组随机公钥：标准写法全挂、
// URL 安全无填充全过。
//
// 返回值第二个是"没能转换、原样输出"——那意味着输入本身就解不出来，得告警。
func realityPublicKey(s string) (string, bool) {
	if s == "" || !strings.ContainsAny(s, "+/=") {
		// 空值，或者已经是 URL 安全无填充写法。
		return s, false
	}
	b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
	if err != nil || len(b) != 32 {
		return s, true
	}
	return base64.RawURLEncoding.EncodeToString(b), false
}

// tlsInherent 判定该出站的 TLS 是否协议固有——这些协议在 Clash 侧没有 tls
// 布尔位可看。https 单独判：它映射到 http 出站，但带 TLS 握手。
func tlsInherent(typ string, isHTTPS bool) bool {
	switch typ {
	case "trojan", "hysteria", "hysteria2", "anytls":
		return true
	}
	return isHTTPS
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

// rawStrList 把 raw 里的列表字段转成 []string；缺失时返回空列表。
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

// defaultBandwidthMbps 是节点链接里不带速度上限时的兜底值。hysteria（v1）出站
// 强制要求 up_mbps 和 down_mbps 同时存在——只给一个会报 missing download
// speed / missing upload speed，两个都不给同样判失败，整份配置加载不了。
// 机场链接里省略速度是常态，所以不能把这种节点整个丢掉：填一个足够大的上限
// （相当于不限速），并用告警说明这是渲染器补的而不是节点自带的。
// hysteria2 侧这两个字段是可选的，不补。
const defaultBandwidthMbps = 1000

// addBandwidth 写 hysteria 的速度上限。Clash 的 up / down 可能是数字也可能是
// 字符串，sing-box 1.12 起改叫 up_mbps / down_mbps。
//
// 必须落成整数：sing-box 的 HysteriaOutboundOptions.up_mbps 是 int，写成字符串
// 会直接判解码失败。官方 schema.json 把 hysteria 的 up/down 标成
// integer|string，跟着那个写字符串能过 schema 却过不了实机——以实机为准。
func (r *Renderer) addBandwidth(add func(string, any), tag string, raw map[string]any, required bool) {
	haveUp := false
	if v, ok := rawNum(raw, "up"); ok {
		add("up_mbps", v)
		haveUp = true
	}
	haveDown := false
	if v, ok := rawNum(raw, "down"); ok {
		add("down_mbps", v)
		haveDown = true
	}
	if !required {
		return
	}
	switch {
	case !haveUp && !haveDown:
		add("up_mbps", defaultBandwidthMbps)
		add("down_mbps", defaultBandwidthMbps)
		r.warns = append(r.warns, fmt.Sprintf(
			"节点 %q 没有声明 up/down 速度，hysteria 出站强制要求，已按 %d Mbps 兜底（相当于不限速）",
			tag, defaultBandwidthMbps))
	case !haveUp:
		add("up_mbps", defaultBandwidthMbps)
		r.warns = append(r.warns, fmt.Sprintf(
			"节点 %q 缺少 up 速度，hysteria 出站强制要求，已按 %d Mbps 兜底", tag, defaultBandwidthMbps))
	case !haveDown:
		add("down_mbps", defaultBandwidthMbps)
		r.warns = append(r.warns, fmt.Sprintf(
			"节点 %q 缺少 down 速度，hysteria 出站强制要求，已按 %d Mbps 兜底", tag, defaultBandwidthMbps))
	}
}

// rawNum 取出一个非负整数值，字符串形态也会解析。
// 解析不了就返回 false：宁可丢掉这个可选字段，也不产出加载失败的配置。
func rawNum(raw map[string]any, key string) (int, bool) {
	switch v := raw[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v >= 0 && v == float64(int(v)) {
			return int(v), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			return n, true
		}
	}
	return 0, false
}

// headerHostNode 取 ws-opts.headers.Host——sing-box 的 ws 传输用 headers
// 子对象，而不是 Clash 那种 host 平铺字段。
func headerHostNode(opts map[string]any) *yaml.Node {
	h, ok := opts["headers"].(map[string]any)
	if !ok {
		return nil
	}
	host := rawStrOr(h, "", "Host", "host")
	if host == "" {
		return nil
	}
	n, err := render.MappingNode("Host", host)
	if err != nil {
		return nil
	}
	return &n
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

// parseDNSAddr 解析旧式 DNS 服务器地址写法。
// 支持 tls:// / https:// / h3:// / quic:// / udp:// / tcp:// / host:port / ip /
// fakeip / rcode://success。
func parseDNSAddr(addr string) (typ, server string, port int, path string) {
	a := strings.TrimSpace(addr)
	if a == "" {
		return "udp", "", 0, ""
	}
	scheme, rest, hasScheme := strings.Cut(a, "://")
	if hasScheme {
		switch strings.ToLower(scheme) {
		case "tls":
			return "tls", rest, 443, ""
		case "https", "doh":
			parts := strings.SplitN(rest, "/", 2)
			p := parts[0]
			if len(parts) > 1 {
				p = "/" + parts[1]
			}
			return "https", p, 443, ""
		case "h3", "quic":
			parts := strings.SplitN(rest, "/", 2)
			p := parts[0]
			if len(parts) > 1 {
				p = "/" + parts[1]
			}
			return "h3", p, 443, ""
		case "udp":
			return "udp", rest, 53, ""
		case "tcp":
			return "tcp", rest, 53, ""
		case "fakeip":
			return "fakeip", rest, 0, ""
		case "rcode":
			return "fakeip", "rcode:" + rest, 0, ""
		}
	}
	if a == "fakeip" {
		return "fakeip", "", 0, ""
	}
	if strings.HasPrefix(a, "rcode:") {
		return "fakeip", a, 0, ""
	}
	return "udp", a, 53, ""
}

// splitHostPort 从 "host:port" 或 "ip" 里取出地址与端口。
func splitHostPort(server string, fallback int) (string, int) {
	if h, p, ok := strings.Cut(server, ":"); ok {
		if strings.Contains(h, ":") {
			return server, fallback // 裸 IPv6，无端口
		}
		if n, err := strconv.Atoi(p); err == nil {
			return h, n
		}
		return server, fallback
	}
	return server, fallback
}
