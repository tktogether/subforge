// Package clashmeta 渲染 Clash Meta / mihomo 兼容配置。
//
// mihomo 的 outbound 适配器没有 Marshal 方法（见 docs/adr/0001），
// 因此 proxies 直接透传 Node.Raw 的全部键值，不经过任何自研结构体中转——
// 这样订阅里的任何扩展字段（reality-opts、ws-opts、obfs…）都不会丢失。
package clashmeta

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/render"
)

// Options 是 clash.meta 渲染器的输出配置。
type Options struct {
	// JSON 输出 JSON 而非 YAML。构树逻辑完全共用，只换编码步骤——
	// 两种格式描述的是同一份配置，不该有各自的内容实现。
	JSON bool
	// MixedPort HTTP+TCP+UDP 混合监听端口。
	MixedPort int
	// AllowLan 是否允许局域网连接。
	AllowLan bool
	// Mode 运行模式：rule / global / direct。
	Mode string
	// ExternalController RESTful API 监听地址。
	ExternalController string
	// InterfaceName 绑定网卡（可为空）。
	InterfaceName string
	// DNS DNS 段配置。
	DNS DNSOptions
}

// DNSOptions 是最小可用的 DNS 配置（fake-ip 模式）。
type DNSOptions struct {
	// Enable 启用 DNS 监听。
	Enable bool
	// Listen 监听地址。
	Listen string
	// EnhancedMode fake-ip / redir-host / hijack-domain。
	EnhancedMode string
	// FakeIPRange fake-ip 地址池。
	FakeIPRange string
	// FakeIPFilter 不走 fake-ip 的域名/网段。
	FakeIPFilter []string
	// DefaultNameserver 兜底 DNS（用于解析 DNS 服务器自身地址）。
	DefaultNameserver []string
	// Nameserver 主 DNS。
	Nameserver []string
	// Fallback 回退 DNS。
	Fallback []string
	// FallbackFilterGeoIP 按 IP 归属地过滤回退结果。
	FallbackFilterGeoIP bool
	// FallbackFilterGeoIPCode 期望的归属地代码。
	FallbackFilterGeoIPCode string
}

// DefaultOptions 给出保守可用默认值。
func DefaultOptions() Options {
	return Options{
		MixedPort:          7890,
		AllowLan:           false,
		Mode:               "rule",
		ExternalController: "127.0.0.1:9090",
		DNS: DNSOptions{
			Enable:                  true,
			Listen:                  "0.0.0.0:1053",
			EnhancedMode:            "fake-ip",
			FakeIPRange:             "198.18.0.0/16",
			FakeIPFilter:            []string{"+.lan", "lanserver.test", "*.local", "*.localdomain"},
			DefaultNameserver:       []string{"223.5.5.5", "1.1.1.1"},
			Nameserver:              []string{"https://doh.pub/dns-query", "https://dns.cloudflare.com/dns-query"},
			Fallback:                []string{"tls://8.8.8.8:853"},
			FallbackFilterGeoIP:     true,
			FallbackFilterGeoIPCode: "CN",
		},
	}
}

// Renderer 渲染 clash.meta 配置。
type Renderer struct {
	opts Options
}

// New 构造渲染器；空配置字段用默认值补齐。
func New(opts Options) *Renderer {
	return &Renderer{opts: fillDefaults(opts)}
}

func fillDefaults(o Options) Options {
	d := DefaultOptions()
	if o.MixedPort == 0 {
		o.MixedPort = d.MixedPort
	}
	if o.Mode == "" {
		o.Mode = d.Mode
	}
	if o.ExternalController == "" {
		o.ExternalController = d.ExternalController
	}
	if o.DNS.Listen == "" {
		o.DNS.Listen = d.DNS.Listen
	}
	if o.DNS.EnhancedMode == "" {
		o.DNS.EnhancedMode = d.DNS.EnhancedMode
	}
	if o.DNS.FakeIPRange == "" {
		o.DNS.FakeIPRange = d.DNS.FakeIPRange
	}
	if len(o.DNS.Nameserver) == 0 {
		o.DNS = d.DNS
	}
	if o.DNS.FallbackFilterGeoIPCode == "" {
		o.DNS.FallbackFilterGeoIPCode = d.DNS.FallbackFilterGeoIPCode
	}
	return o
}

// Name 返回目标名；JSON 输出带上 .json 后缀，X-Convert-Format 与落盘名都从它派生。
func (r *Renderer) Name() string {
	if r.opts.JSON {
		return "clash.meta.json"
	}
	return "clash.meta"
}

// Render 产出 YAML（含注释头）或 JSON 文本。
func (r *Renderer) Render(req render.Request) ([]byte, error) {
	groups := req.Groups
	if len(groups) == 0 {
		groups = defaultGroups(req.Nodes.Names())
	}
	rules := req.Rules
	if len(rules) == 0 {
		rules = defaultRules(groups)
	}

	entries := make([]any, 0, 20)
	add := func(k string, v any) {
		entries = append(entries, k, v)
	}
	add("mixed-port", r.opts.MixedPort)
	add("allow-lan", r.opts.AllowLan)
	if r.opts.InterfaceName != "" {
		add("interface-name", r.opts.InterfaceName)
	}
	add("mode", r.opts.Mode)
	add("log-level", "info")
	add("ipv6", false)
	add("unify-ip", true)
	add("tcp-concurrent", true)
	add("external-controller", r.opts.ExternalController)
	dns, err := r.dnsNode()
	if err != nil {
		return nil, err
	}
	add("dns", dns)

	proxies, err := r.proxiesNode(req.Nodes.Raws())
	if err != nil {
		return nil, err
	}
	add("proxies", proxies)
	groupsNode, err := r.groupsNode(groups)
	if err != nil {
		return nil, err
	}
	add("proxy-groups", groupsNode)
	add("rules", render.StringSequence(rules))

	root, err := render.MappingNode(entries...)
	if err != nil {
		return nil, err
	}

	if r.opts.JSON {
		// JSON 不支持注释，配置头无处可放——元信息只留在 HTTP 响应头里。
		body, err := render.MarshalJSONDoc(root)
		if err != nil {
			return nil, fmt.Errorf("编码 JSON: %w", err)
		}
		return body, nil
	}

	body, err := render.MarshalDoc(root)
	if err != nil {
		return nil, fmt.Errorf("编码 YAML: %w", err)
	}
	return []byte(render.CommentBlock(headerLines(req.Profile)) + string(body)), nil
}

func (r *Renderer) dnsNode() (yaml.Node, error) {
	d := r.opts.DNS
	entries := make([]any, 0, 16)
	add := func(k string, v any) { entries = append(entries, k, v) }
	add("enable", d.Enable)
	add("listen", d.Listen)
	if d.EnhancedMode != "" {
		add("enhanced-mode", d.EnhancedMode)
	}
	if d.FakeIPRange != "" {
		add("fake-ip-range", d.FakeIPRange)
	}
	if len(d.FakeIPFilter) > 0 {
		add("fake-ip-filter", render.StringSequence(d.FakeIPFilter))
	}
	if len(d.DefaultNameserver) > 0 {
		add("default-nameserver", render.StringSequence(d.DefaultNameserver))
	}
	if len(d.Nameserver) > 0 {
		add("nameserver", render.StringSequence(d.Nameserver))
	}
	if len(d.Fallback) > 0 {
		add("fallback", render.StringSequence(d.Fallback))
	}
	if d.FallbackFilterGeoIP {
		filter, err := render.MappingNode("geoip", true, "geoip-code", d.FallbackFilterGeoIPCode)
		if err != nil {
			return yaml.Node{}, err
		}
		add("fallback-filter", filter)
	}
	return render.MappingNode(entries...)
}

// proxiesNode 透传原始配置 map，键按字典序排列以保证 golden 稳定。
func (r *Renderer) proxiesNode(raws []map[string]any) (yaml.Node, error) {
	children := make([]*yaml.Node, 0, len(raws))
	for _, raw := range raws {
		keys := make([]string, 0, len(raw))
		for k := range raw {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		entries := make([]any, 0, len(keys)*2)
		for _, k := range keys {
			entries = append(entries, k, raw[k])
		}
		n, err := render.MappingNode(entries...)
		if err != nil {
			return yaml.Node{}, err
		}
		children = append(children, &n)
	}
	return render.SequenceNode(children...), nil
}

func (r *Renderer) groupsNode(groups []render.Group) (yaml.Node, error) {
	children := make([]*yaml.Node, 0, len(groups))
	for _, g := range groups {
		n, err := r.groupNode(g)
		if err != nil {
			return yaml.Node{}, err
		}
		children = append(children, &n)
	}
	return render.SequenceNode(children...), nil
}

// supportedTypes 是 mihomo 实际支持的 proxy-group 类型。
// 依据 mihomo v1.19.31 adapter/outboundgroup/parser.go 的类型 switch：
// 仅 select / url-test / fallback / load-balance 四类，relay 已被移除。
// mihomo 没有 reject / direct / pass 这类"特殊类型分组"——
// 要拒绝流量就用 select 分组套内置的 REJECT / REJECT-DROP 出站。
// 这里在渲染期就拦下，避免产出 mihomo 无法加载的配置。
var supportedTypes = map[string]bool{
	"select":       true,
	"url-test":     true,
	"fallback":     true,
	"load-balance": true,
}

func (r *Renderer) groupNode(g render.Group) (yaml.Node, error) {
	entries := make([]any, 0, 10)
	add := func(k string, v any) { entries = append(entries, k, v) }
	typ := g.Type
	if typ == "" {
		typ = "select"
	}
	if !supportedTypes[typ] {
		return yaml.Node{}, fmt.Errorf("proxy-group %q: mihomo 不支持的分组类型 %q（支持: select / url-test / fallback / load-balance）",
			g.Name, typ)
	}
	add("name", g.Name)
	add("type", typ)
	switch typ {
	case "url-test", "fallback":
		test := g.Test
		if test == "" {
			test = "https://www.gstatic.com/generate_204"
		}
		add("url", test)
		if g.Options.Interval > 0 {
			add("interval", g.Options.Interval)
		}
		if g.Options.Tolerance > 0 {
			add("tolerance", g.Options.Tolerance)
		}
		if g.Options.Timeout > 0 {
			add("timeout", g.Options.Timeout)
		}
		if g.Lazy {
			add("lazy", true)
		}
		if g.Strategy == "compatible" || g.Strategy == "global" {
			add("strategy", g.Strategy)
		}
	}
	add("proxies", render.StringSequence(g.Proxies))
	return render.MappingNode(entries...)
}

// defaultGroups 生成最小可用的四组结构（订阅转换默认产物）。
func defaultGroups(names []string) []render.Group {
	if len(names) == 0 {
		return nil
	}
	return []render.Group{
		{Name: "🚀 节点选择", Type: "select", Proxies: names},
		{
			Name:    "♻️ 自动选择",
			Type:    "url-test",
			Proxies: names,
			Options: render.UrlTestOptions{Interval: 300, Tolerance: 50, Timeout: 5000},
			Lazy:    true,
		},
		{Name: "🎯 全球直连", Type: "select", Proxies: []string{"DIRECT", "♻️ 自动选择"}},
		// mihomo 没有 reject 类型的分组，只能用 select 套内置 REJECT 出站。
		{Name: "🛑 拒绝访问", Type: "select", Proxies: []string{"REJECT", "REJECT-DROP"}},
		{Name: "🐟 漏网之鱼", Type: "select", Proxies: []string{"🚀 节点选择", "DIRECT"}},
	}
}

// defaultRules 生成最小可用的兜底规则。
//
// 注意用 MATCH 而不是 FINAL：mihomo v1.19.31 的 rules/parser.go 里没有 FINAL 这个
// 规则类型，写 FINAL 会报 "rules[i] error: format invalid"，兜底规则必须写 MATCH。
// 这一点经 `mihomo -t` 实机确认，见 scripts/validate-mihomo.sh 与 docs/compatibility.md。
func defaultRules(groups []render.Group) []string {
	if len(groups) == 0 {
		return []string{"GEOIP,CN,DIRECT", "MATCH,DIRECT"}
	}
	catchAll := "DIRECT"
	for _, g := range groups {
		if g.Type == "select" {
			catchAll = g.Name
			break
		}
	}
	return []string{"GEOIP,CN,DIRECT", "MATCH," + catchAll}
}

func headerLines(p render.Profile) []string {
	lines := []string{"此配置由 SubForge 自动生成"}
	if p.Name != "" {
		lines = append(lines, "配置名: "+p.Name)
	}
	if p.Version != "" {
		lines = append(lines, "SubForge "+p.Version)
	}
	if p.GeneratedAt != "" {
		lines = append(lines, "生成时间: "+p.GeneratedAt)
	}
	return lines
}
