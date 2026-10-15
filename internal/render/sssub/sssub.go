// Package sssub 把节点渲染成 Shadowsocks-android 的节点配置形态。
//
// SS-android 吃的是「JSON 数组」而不是分享链接：每个节点是一组
// server / server_port / method / password / plugin 字段，
// 再带上一套应用级默认值（路由、DNS、代理应用白名单）。
// 这些应用级字段与具体节点无关，所以写成默认值，而不是每个节点重复一遍。
//
// 语法依据是 subconverter v0.9.0 的实机输出（tools/scv/goldens2/sssub.out，
// GPL-3.0 仅作参照）。JSON 字段顺序按参考输出原样固定：SS-android 反序列化时
// 不挑顺序，但 golden 测试要字节级稳定，所以不能交给 encoding/json 的 map 排序。
package sssub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// ProxyApps 是 SS-android 的「代理应用」白名单（走代理的 App 包名）。
// 取自 subconverter 的默认值；空列表即全部走代理。
var ProxyApps = []string{
	"com.eg.android.AlipayGphone",
	"com.wudaokou.hippo",
	"com.zhihu.android",
}

// Options 控制渲染选项。
type Options struct {
	// Base64 为 true 时输出 base64 包裹（SS-android 订阅的入口格式）。
	Base64 bool
	// Route 路由预设名，如 "bypass-lan-china"。
	Route string
	// RemoteDNS 远程 DNS 地址。
	RemoteDNS string
	// Plugin SSR 插件名（如 obfs-simple）。空表示不使用插件。
	Plugin string
	// PluginOpts SSR 插件参数（如 "mode=tcp;host=example.com"）。
	PluginOpts string
}

// DefaultOptions 给一份可直接使用的默认值。
func DefaultOptions() Options {
	return Options{Route: "bypass-lan-china", RemoteDNS: "dns.google"}
}

// Renderer 是 SS-android 配置形态的渲染器。
type Renderer struct {
	opts  Options
	warns []string
}

// New 构造渲染器。
func New(opts Options) *Renderer { return &Renderer{opts: opts} }

func (r *Renderer) Name() string { return "ss-sub" }

// Render 产出节点配置。每个节点是一组固定顺序的字段，
// 应用级字段取 Options 的默认值，不逐节点重复节点专属信息。
func (r *Renderer) Render(req render.Request) ([]byte, error) {
	r.warns = nil

	nodes := make([][]pair, 0, req.Nodes.Len())
	for _, n := range req.Nodes {
		node, ok := nodeFields(r.opts, n)
		if !ok {
			r.warns = append(r.warns, fmt.Sprintf("节点 %q（%s）不支持 SS-android 形态，已跳过",
				nodeLabel(n), n.Type))
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有可导出的节点")
	}

	body := marshalNodes(nodes)
	if r.opts.Base64 {
		body = []byte(base64.StdEncoding.EncodeToString(body))
	}
	return body, nil
}

// Warnings 返回最近一次 Render 中被跳过的节点。
func (r *Renderer) Warnings() []string { return r.warns }

// pair 是一个 "键":值 片段。
// kind 决定值怎么编码：字符串要加引号并转义，布尔与数字直接写，对象则原样嵌入。
// 不用 interface{} 是为了不引入 encoding/json 的 map 排序——那种排序会让 golden 不稳。
type pair struct {
	key   string
	kind  int
	value string
}

const (
	kindStr = iota // 字符串，编码成 "…"
	kindRaw        // 数字或布尔，原样写出
	kindObj        // 对象字面量，原样写出
)

func str(k, v string) pair    { return pair{k, kindStr, v} }
func rawVal(k, v string) pair { return pair{k, kindRaw, v} }
func obj(k, v string) pair    { return pair{k, kindObj, v} }

// strVal 把一个字符串编码成 JSON 字符串字面量（带引号，正确处理反斜杠与换行）。
func strVal(v string) string {
	buf, err := json.Marshal(v)
	if err != nil {
		return `""`
	}
	return string(buf)
}

// nodeFields 把一个节点渲染成字段列表。ok=false 表示该协议无法表达。
// SS-android 只支持 SS 与 SSR：其余协议没有对应的 server/method 语义。
func nodeFields(opts Options, n model.Node) ([]pair, bool) {
	raw := n.Raw
	switch strings.ToLower(strings.TrimSpace(n.Type)) {
	case "ss", "shadowsocks":
		return append(commonFields(opts),
			str("remarks", n.Name),
			str("server", n.Server),
			rawVal("server_port", intField(n)),
			str("method", mapString(raw, "cipher")),
			str("password", mapString(raw, "password")),
			str("plugin", ""),
			str("plugin_opts", ""),
		), true
	case "ssr", "shadowsocksr":
		return append(commonFields(opts),
			str("remarks", n.Name),
			str("server", n.Server),
			rawVal("server_port", intField(n)),
			str("method", mapString(raw, "cipher")),
			str("password", mapString(raw, "password")),
			str("plugin", opts.Plugin),
			str("plugin_opts", opts.PluginOpts),
		), true
	}
	return nil, false
}

// commonFields 是两种协议共用的应用级字段。
// 它们与具体节点无关，取值一律来自 Options 默认值。
func commonFields(opts Options) []pair {
	return []pair{
		str("route", opts.Route),
		str("remote_dns", opts.RemoteDNS),
		rawVal("ipv6", "false"),
		rawVal("metered", "false"),
		obj("proxy_apps", `{"enabled":false,"bypass":true,"android_list":`+jsonListVal(ProxyApps)+`}`),
		rawVal("udpdns", "false"),
	}
}

// marshalNodes 把字段列表编成 JSON 数组。
// 手工编码而不是 encoding/json.Marshal：map 的键会被排序，产物就不确定了。
func marshalNodes(nodes [][]pair) []byte {
	var b strings.Builder
	b.WriteByte('[')
	for i, node := range nodes {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('{')
		for j, p := range node {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`"`)
			b.WriteString(p.key)
			b.WriteString(`":`)
			switch p.kind {
			case kindRaw, kindObj:
				b.WriteString(p.value)
			default:
				b.WriteString(strVal(p.value))
			}
		}
		b.WriteByte('}')
	}
	b.WriteByte(']')
	return []byte(b.String())
}

// jsonListVal 把字符串列表编成 JSON 数组字面量。
func jsonListVal(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, it := range items {
		buf, err := json.Marshal(it)
		if err != nil {
			quoted = append(quoted, `""`)
			continue
		}
		quoted = append(quoted, string(buf))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// intField 把端口转成 JSON 数字（不带引号）。
// 端口缺失时不写这个字段是错的，SS-android 会拒绝无端口的节点。
func intField(n model.Node) string {
	return strings.TrimSpace(n.Port)
}

// mapString 从 raw map 取一个字符串字段。
func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return model.MapString(m, key)
}

func nodeLabel(n model.Node) string {
	if n.Name != "" {
		return n.Name
	}
	return "<unnamed " + n.Type + ">"
}
