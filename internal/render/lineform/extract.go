package lineform

import (
	"strings"

	"github.com/tktogether/subforge/internal/model"
)

// spec 是节点配置的可读视图：把内核 raw map 里散落的字段抽出来，
// 让各格式的序列化函数只面对字符串，不各自重复类型断言。
type spec struct {
	Name       string
	Type       string
	Server     string
	Port       string
	Method     string // vmess 密码算法
	Cipher     string // ss / ssr 的 cipher
	User       string // socks5 / http 的用户名
	Pwd        string // 口令或 uuid
	TLS        bool
	Servername string
	FP         string // client-fingerprint
	Network    string // tcp / ws / grpc / httpupgrade / http
	Path       string // ws-opts.path 或 query path
	Host       string // ws-opts.headers.Host 或 query host
	SNI        string // 证书名：trojan/anytls/hy2 用 sni，vmess/vless 用 servername
	SkipVerify bool

	// SSR
	Protocol      string
	Obfs          string
	ProtocolParam string
	ObfsParam     string

	// VLESS Reality
	PubKey  string
	ShortID string
}

// nodeSpec 从节点的原始配置 map 抽出序列化所需字段。
func nodeSpec(n model.Node) spec {
	raw := n.Raw
	so := mapStr(raw, "ws-opts")
	ro := mapStr(raw, "reality-opts")

	sni := s(raw, "sni")
	sn := s(raw, "servername")
	if sn == "" {
		sn = sni
	}
	if sni == "" {
		sni = sn
	}

	// ws 的 Host 藏在 headers 子 map 里（{"headers":{"Host":...}}），
	// 直接取 so["Host"] 拿不到。大小写两个键名都试，跟随客户端习惯。
	headers := mapStr(so, "headers")
	host := s(headers, "Host")
	if host == "" {
		host = s(headers, "host")
	}
	path := s(so, "path")

	return spec{
		Name:       n.Name,
		Type:       n.Type,
		Server:     n.Server,
		Port:       n.Port,
		Method:     s(raw, "cipher"),
		Cipher:     s(raw, "cipher"),
		User:       s(raw, "username"),
		Pwd:        firstNonEmpty(s(raw, "uuid"), s(raw, "password")),
		TLS:        isTrue(raw["tls"]),
		Servername: sn,
		FP:         s(raw, "client-fingerprint"),
		Network:    s(raw, "network"),
		Path:       path,
		Host:       host,
		SNI:        sni,
		SkipVerify: isTrue(raw["skip-cert-verify"]),

		Protocol:      s(raw, "protocol"),
		Obfs:          s(raw, "obfs"),
		ProtocolParam: s(raw, "protocol-param"),
		ObfsParam:     s(raw, "obfs-param"),

		PubKey:  s(ro, "public-key"),
		ShortID: s(ro, "short-id"),
	}
}

// vmessCipher 把内核的 "auto" 落成客户端通用的实际密码算法。
// subconverter 同样做这一步；QX/Loon/Surfboard 不认 "auto"。
func vmessCipher(c string) string {
	if c == "" || c == "auto" {
		return "chacha20-ietf-poly1305"
	}
	return c
}

// netUpgrade 把内核的 network 名映射成客户端用的短名。
func netUpgrade(net string) string {
	if net == "http" {
		return "h2"
	}
	return net
}

// qIf 仅在值含分隔符或空白时加双引号。
func qIf(v string) string {
	if v == "" {
		return v
	}
	for _, c := range v {
		if c == ' ' || c == ',' || c == '"' || c == '\\' || c == '=' {
			return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
		}
	}
	return v
}

// qAlways 无条件加双引号。QX 与 Loon 对凭据字段一律加引号，跟随这个约定。
func qAlways(v string) string {
	if v == "" {
		return v
	}
	return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
}

// joinKV 把若干 "k=v" 片段用逗号连接，空片段被丢弃。
func joinKV(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

// kv 拼一个 "k=v" 片段；v 为空时返回空串（由 joinKV 丢弃）。
func kv(k, v string) string {
	if v == "" {
		return ""
	}
	return k + "=" + v
}

// --- raw map 读取助手 ---

func s(m map[string]any, key string) string {
	return model.MapString(m, key)
}

func mapStr(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if x, ok := m[key].(map[string]any); ok {
		return x
	}
	return nil
}

func isTrue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
