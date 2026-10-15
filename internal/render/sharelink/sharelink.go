// Package sharelink 把节点渲染成分享链接（vmess:// / vless:// / trojan:// …）。
//
// 与 parse.DecodeSubscription 互为逆操作，但内核 outbound 适配器没有 Marshal 方法
// （见 docs/adr/0001），序列化只能自己实现。链接格式以内核 common/convert 的解析器为准——
// 那既是本服务的事实源，也是产物能否被真实客户端识别的最严格约束。
//
// 约定（对齐 render 包）：只读 Node.Raw，不在渲染时改写节点；输出确定性（同一节点
// 永远得到同一条链接），否则 golden 测试无法工作。
package sharelink

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// ErrUnsupportedProtocol 表示该协议类型没有对应的分享链接形态。
var ErrUnsupportedProtocol = errors.New("协议不支持分享链接序列化")

// Output 是一次清单渲染的结果。
type Output struct {
	// Body 分享链接清单（每行一条，无尾随空行）。
	Body []byte
	// Lines 成功序列化的条数。
	Lines int
	// Skipped 无法序列化的节点名（协议不支持），供调用方作为警告上报。
	Skipped []string
}

// Supported 返回可序列化的协议类型（按字典序，供文档与错误信息使用）。
func Supported() []string {
	out := []string{
		"anytls", "hysteria", "hysteria2", "http", "https", "shadowsocks",
		"shadowsocksr", "socks5", "ss", "ssr", "trojan", "tuic", "vless", "vmess",
	}
	sort.Strings(out)
	return out
}

// Render 把一个节点渲染成单条分享链接。
func Render(n model.Node) (string, error) {
	raw := n.Raw
	switch strings.ToLower(strings.TrimSpace(n.Type)) {
	case "vmess":
		return vmess(raw)
	case "vless":
		return vless(raw)
	case "trojan":
		return trojan(raw)
	case "ss", "shadowsocks":
		return shadowsocks(raw)
	case "ssr", "shadowsocksr":
		return ssr(raw)
	case "hysteria":
		return hysteria(raw)
	case "hysteria2":
		return hysteria2(raw)
	case "tuic":
		return tuic(raw)
	case "anytls":
		return anytls(raw)
	case "socks5":
		return socks(raw)
	case "http", "https":
		return httpLink(raw)
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedProtocol, n.Type)
	}
}

// RenderAll 把节点列表渲染成分享链接清单。
//
// 协议不支持的节点不中断整批导出：跳过并记名，让调用方决定要不要告警——
// 静默丢节点最危险，但一条异常节点也不该让整份订阅导不出来。
func RenderAll(nodes model.NodeList) (Output, error) {
	lines := make([]string, 0, nodes.Len())
	skipped := []string{}
	for _, n := range nodes {
		link, err := Render(n)
		if err != nil {
			if errors.Is(err, ErrUnsupportedProtocol) {
				skipped = append(skipped, labelOf(n))
				continue
			}
			return Output{}, err
		}
		lines = append(lines, link)
	}
	out := Output{Lines: len(lines), Skipped: skipped}
	if len(lines) > 0 {
		out.Body = []byte(strings.Join(lines, "\n"))
	}
	return out, nil
}

// labelOf 给跳过的节点一个可读标识，无名字时带上协议类型便于排查。
func labelOf(n model.Node) string {
	if n.Name != "" {
		return n.Name
	}
	return "<unnamed " + n.Type + ">"
}

// --- render.Renderer 接入 ---

// Options 控制清单的包裹形态。
type Options struct {
	// Base64 为 true 时把整份清单再包一层 base64（subconverter 的 mixed / ss / ssr /
	// trojan / v2ray 都是这个形态）。包裹前每条链接后补一个换行，与 subconverter 对齐。
	Base64 bool
}

// Renderer 把节点清单渲染成分享链接清单，与 clash.meta 共用 Engine 的
// filter / rename / sort 路径。实例由 RendererFor 按请求创建，
// 所以可以用字段带回本次渲染的降级信息。
type Renderer struct {
	target   string
	base64   bool
	warnings []string
}

// New 按目标名与选项构造渲染器。
func New(target string, opts Options) *Renderer {
	return &Renderer{target: target, base64: opts.Base64}
}

func (r *Renderer) Name() string { return r.target }

// Render 输出纯链接清单，不加注释头——产物可能被包进 base64 订阅，多一行就废了。
func (r *Renderer) Render(req render.Request) ([]byte, error) {
	r.warnings = nil
	out, err := RenderAll(req.Nodes)
	r.warnings = out.Skipped
	if err != nil {
		return nil, err
	}
	if !r.base64 || len(out.Body) == 0 {
		return out.Body, nil
	}
	payload := make([]byte, len(out.Body)+1)
	copy(payload, out.Body)
	payload[len(out.Body)] = '\n'
	return []byte(base64.StdEncoding.EncodeToString(payload)), nil
}

// Warnings 返回本次渲染里无法序列化的节点名，满足 render.WarningReporter。
func (r *Renderer) Warnings() []string { return r.warnings }

// --- 各协议的序列化 ---

// vmess 走 V2RayN 的 base64(JSON) 形态。
func vmess(raw map[string]any) (string, error) {
	ws := mapVal(raw, "ws-opts")
	p := map[string]string{
		"v":    "2",
		"ps":   strVal(raw, "name"),
		"add":  strVal(raw, "server"),
		"port": strVal(raw, "port"),
		"id":   strVal(raw, "uuid"),
		"aid":  orDefault(strVal(raw, "alterId"), "0"),
		"scy":  orDefault(strVal(raw, "cipher"), "auto"),
		"net":  orDefault(strVal(raw, "network"), "tcp"),
		"type": "none",
	}
	if ws != nil {
		setIf(p, "path", strVal(ws, "path"))
	}
	// 内核只读 "sni"（converter.go 的 vmess 分支），V2RayN / Clash Nyanpasu 等客户端读 "sn"，
	// 两个都写才能保证这份产物两边都认。
	sn := strVal(raw, "servername")
	if sn != "" {
		p["sn"] = sn
		p["sni"] = sn
	}
	host := ""
	if ws != nil {
		host = strVal(mapVal(ws, "headers"), "Host")
	}
	if host == "" {
		host = sn
	}
	setIf(p, "host", host)
	if boolVal(raw, "tls") {
		p["tls"] = "tls"
	}
	// map[string]string 经 json.Marshal 按键字典序输出，确定性有保证。
	buf, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("vmess 序列化: %w", err)
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(buf), nil
}

// vless 走 Xray 的 vless://uuid@host:port?...#name 形态。
func vless(raw map[string]any) (string, error) {
	q := url.Values{}
	setParam(q, "encryption", orDefault(strVal(raw, "encryption"), "none"))
	setParam(q, "fp", strVal(raw, "client-fingerprint"))
	setParam(q, "type", strVal(raw, "network"))
	setParam(q, "flow", strVal(raw, "flow"))
	setParam(q, "alpn", joinStrs(strSlice(raw["alpn"])))
	setParam(q, "sni", strVal(raw, "servername"))

	// security 与 reality 参数绑定：内核看到 security=reality 才写 reality-opts。
	if reality := mapVal(raw, "reality-opts"); reality != nil {
		q.Set("security", "reality")
		setParam(q, "pbk", strVal(reality, "public-key"))
		setParam(q, "sid", strVal(reality, "short-id"))
	} else if boolVal(raw, "tls") {
		q.Set("security", "tls")
	}
	if boolVal(raw, "packet-addr") {
		q.Set("packetEncoding", "packet")
	}
	hp := transportHostPath(raw)
	if hp.host != "" {
		q.Set("host", hp.host)
	}
	setParam(q, "path", hp.path)
	setParam(q, "serviceName", strVal(mapVal(raw, "grpc-opts"), "grpc-service-name"))

	return build("vless", strVal(raw, "uuid"), "", strVal(raw, "server"), strVal(raw, "port"), q, strVal(raw, "name")), nil
}

// trojan 走 trojan://password@host:port?...#name 形态。
func trojan(raw map[string]any) (string, error) {
	q := url.Values{}
	q.Set("allowInsecure", boolStr(boolVal(raw, "skip-cert-verify")))
	setParam(q, "sni", strVal(raw, "sni"))
	setParam(q, "type", strVal(raw, "network"))
	setParam(q, "fp", strVal(raw, "client-fingerprint"))
	setParam(q, "pcs", strVal(raw, "fingerprint"))
	setParam(q, "alpn", joinStrs(strSlice(raw["alpn"])))
	hp := transportHostPath(raw)
	if hp.host != "" {
		q.Set("host", hp.host)
	}
	setParam(q, "path", hp.path)
	setParam(q, "serviceName", strVal(mapVal(raw, "grpc-opts"), "grpc-service-name"))

	return build("trojan", "", strVal(raw, "password"), strVal(raw, "server"), strVal(raw, "port"), q, strVal(raw, "name")), nil
}

// shadowsocks 走 ss://BASE64(method:password)@host:port#name 形态。
// payload 整段塞进 userinfo 位置；base64url 字符集都是保留外的安全字符，不会被二次转义。
func shadowsocks(raw map[string]any) (string, error) {
	cipher := orDefault(strVal(raw, "cipher"), "aes-256-gcm")
	payload := base64.RawURLEncoding.EncodeToString([]byte(cipher + ":" + strVal(raw, "password")))
	return build("ss", payload, "", strVal(raw, "server"), strVal(raw, "port"), nil, strVal(raw, "name")), nil
}

// ssr 走 ssr://BASE64URL(host:port:protocol:cipher:obfs:BASE64(password)/?remarks=BASE64(name)) 形态。
//
// 内核解析器要求 payload 里同时用 `/?` 分隔查询段、再把 password 与 remarks 各自 base64 一次，
// 这是实测确认过的唯一可解析形式（写成 `/remarks=名称` 会被内核拒为 format invalid）。
func ssr(raw map[string]any) (string, error) {
	base := strings.Join([]string{
		strVal(raw, "server"),
		strVal(raw, "port"),
		orDefault(strVal(raw, "protocol"), "origin"),
		orDefault(strVal(raw, "cipher"), "aes-256-cfb"),
		orDefault(strVal(raw, "obfs"), "plain"),
		urlSafeB64(strVal(raw, "password")),
	}, ":")
	payload := base + "/?remarks=" + urlSafeB64(strVal(raw, "name"))
	return "ssr://" + base64.RawURLEncoding.EncodeToString([]byte(payload)), nil
}

// hysteria 走 hysteria://host:port?protocol=&auth=&peer=&insecure=&up=&down=&alpn=#name 形态。
// v1 没有 userinfo 段，口令走 auth 查询参数。
func hysteria(raw map[string]any) (string, error) {
	q := url.Values{}
	setParam(q, "protocol", strVal(raw, "protocol"))
	setParam(q, "auth", strVal(raw, "auth_str"))
	setParam(q, "peer", strVal(raw, "sni"))
	setParam(q, "insecure", boolStr(boolVal(raw, "skip-cert-verify")))
	setParam(q, "obfs", strVal(raw, "obfs"))
	setParam(q, "up", strVal(raw, "up"))
	setParam(q, "down", strVal(raw, "down"))
	setParam(q, "alpn", joinStrs(strSlice(raw["alpn"])))
	return build("hysteria", "", "", strVal(raw, "server"), strVal(raw, "port"), q, strVal(raw, "name")), nil
}

// hysteria2 走 hysteria2://password@host:port?sni=&insecure=&obfs=&alpn=#name 形态。
func hysteria2(raw map[string]any) (string, error) {
	q := url.Values{}
	setParam(q, "sni", strVal(raw, "sni"))
	setParam(q, "insecure", boolStr(boolVal(raw, "skip-cert-verify")))
	setParam(q, "obfs", strVal(raw, "obfs"))
	setParam(q, "obfs-password", strVal(raw, "obfs-password"))
	setParam(q, "up", strVal(raw, "up"))
	setParam(q, "down", strVal(raw, "down"))
	setParam(q, "alpn", joinStrs(strSlice(raw["alpn"])))
	setParam(q, "pinSHA256", strVal(raw, "fingerprint"))
	return build("hysteria2", "", strVal(raw, "password"), strVal(raw, "server"), strVal(raw, "port"), q, strVal(raw, "name")), nil
}

// tuic 走 tuic://uuid:password@host:port?congestion_control=&alpn=&sni=&udp_relay_mode=#name 形态。
func tuic(raw map[string]any) (string, error) {
	q := url.Values{}
	setParam(q, "congestion_control", strVal(raw, "congestion-controller"))
	setParam(q, "alpn", joinStrs(strSlice(raw["alpn"])))
	setParam(q, "sni", strVal(raw, "sni"))
	setParam(q, "udp_relay_mode", strVal(raw, "udp-relay-mode"))
	if v := strVal(raw, "token"); v != "" {
		q.Set("token", v)
	}
	return build("tuic", strVal(raw, "uuid"), strVal(raw, "password"), strVal(raw, "server"), strVal(raw, "port"), q, strVal(raw, "name")), nil
}

// anytls 走 anytls://password@host:port?sni=&insecure=&hpkp=#name 形态。
func anytls(raw map[string]any) (string, error) {
	q := url.Values{}
	setParam(q, "sni", strVal(raw, "sni"))
	setParam(q, "insecure", boolStr(boolVal(raw, "skip-cert-verify")))
	setParam(q, "hpkp", strVal(raw, "fingerprint"))
	return build("anytls", "", strVal(raw, "password"), strVal(raw, "server"), strVal(raw, "port"), q, strVal(raw, "name")), nil
}

// socks 走 socks5://[user:pass@]host:port#name 形态。
// 内核 socks / socks5 / socks5h 三种 scheme 都归一为 type=socks5，这里用 socks5 这个最常见的写法。
//
// 注意内核的 http/socks 分支会先对 userinfo 做 base64 解码再按冒号拆，解码失败才回落到明文。
// 因此纯 base64 字面量的用户名（如恰好 4 字符的 abcd）会被内核误读——这是内核解析器的缺陷，
// 明文 userinfo 仍是各客户端的通用约定，这里沿用。
func socks(raw map[string]any) (string, error) {
	return build("socks5", strVal(raw, "username"), strVal(raw, "password"), strVal(raw, "server"), strVal(raw, "port"), nil, strVal(raw, "name")), nil
}

// httpLink 走 http://[user:pass@]host:port#name 形态。
//
// 内核对 http/socks 的解析不读任何查询参数：skip-cert-verify 恒为 true，tls 只看 scheme 是不是
// https，sni 直接丢弃。所以这里不输出 tls / insecure 参数（写了也是噪音），用 scheme 表达加密。
func httpLink(raw map[string]any) (string, error) {
	scheme := "http"
	if strings.EqualFold(strVal(raw, "type"), "https") || boolVal(raw, "tls") {
		scheme = "https"
	}
	return build(scheme, strVal(raw, "username"), strVal(raw, "password"), strVal(raw, "server"), strVal(raw, "port"), nil, strVal(raw, "name")), nil
}

// urlSafeB64 输出 base64url 且**不带 = 填充**。
// 内核的 decodeUrlSafe 用 base64.RawURLEncoding 解码，带填充会被判为非法而静默返回空串，
// 于是 ssr 的 password / remarks 会整段丢失。
func urlSafeB64(v string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(v))
}

// --- 通用构造与取值助手 ---

// build 拼一条分享链接：scheme://[userinfo@]host:port[?query][#fragment]。
func build(scheme, user, pass, host, port string, q url.Values, frag string) string {
	auth := net.JoinHostPort(host, port)
	if info := userinfoString(user, pass); info != "" {
		auth = info + "@" + auth
	}
	out := scheme + "://" + auth
	if len(q) > 0 {
		out += "?" + q.Encode()
	}
	if frag != "" {
		out += "#" + escapeFragment(frag)
	}
	return out
}

// userinfoString 输出 userinfo 段。只有一侧有值时必须用 url.User——
// url.UserPassword 会给空口令补一个冒号，而内核用 url.User.Password() 判断口令是否存在，
// 多出的冒号会让 ss / trojan / hysteria2 把口令当空值直接采纳，整条链接就废了。
func userinfoString(user, pass string) string {
	switch {
	case user != "" && pass != "":
		return url.UserPassword(user, pass).String()
	case user != "":
		return url.User(user).String()
	default:
		return url.User(pass).String()
	}
}

// escapeFragment 只转义 URL 结构字符，保留 UTF-8 多字节原样——
// 真实分享链接里的 emoji 与中文都是明文写的，转成 %E9 反而没人读得懂。
func escapeFragment(v string) string {
	const unsafe = " #?&=+%/\\\"'<>|^`{}[]"
	var b strings.Builder
	b.Grow(len(v))
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c < 0x80 && strings.ContainsRune(unsafe, rune(c)) {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func strVal(raw map[string]any, key string) string {
	return model.MapString(raw, key)
}

func boolVal(raw map[string]any, key string) bool { return isTrue(raw[key]) }

func isTrue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func setIf(m map[string]string, key, v string) {
	if v != "" {
		m[key] = v
	}
}

// setParam 只在非空时写查询参数，避免产物里出现一串空值噪音。
func setParam(q url.Values, key, v string) {
	if v != "" {
		q.Set(key, v)
	}
}

func mapVal(raw map[string]any, key string) map[string]any {
	if m, ok := raw[key].(map[string]any); ok {
		return m
	}
	return nil
}

// strSlice 把 map 里的列表值归一成字符串切片（内核可能给 []any 或 []string）。
func strSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if e != nil {
				out = append(out, fmt.Sprint(e))
			}
		}
		return out
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	default:
		return nil
	}
}

func joinStrs(v []string) string { return strings.Join(v, ",") }

// hostPath 承载传输层参数里的 host 与 path。
type hostPath struct {
	host string
	path string
}

// transportHostPath 从 ws-opts / http-opts / h2-opts 里取 host 与 path。
func transportHostPath(raw map[string]any) hostPath {
	if ws := mapVal(raw, "ws-opts"); ws != nil {
		return hostPath{
			path: strVal(ws, "path"),
			host: strVal(mapVal(ws, "headers"), "Host"),
		}
	}
	var out hostPath
	for _, key := range []string{"http-opts", "h2-opts"} {
		m := mapVal(raw, key)
		if m == nil {
			continue
		}
		if v := firstOf(strSlice(m["path"])); v != "" {
			out.path = v
		}
		if v := firstOf(strSlice(m["host"])); v != "" {
			out.host = v
		}
		if v := strVal(mapVal(m, "headers"), "Host"); v != "" {
			out.host = v
		}
	}
	return out
}

func firstOf(vals []string) string {
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}
