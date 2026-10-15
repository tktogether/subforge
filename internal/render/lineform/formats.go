package lineform

import (
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// groupStyle 描述各格式的代理组词汇差异。
type groupStyle struct {
	selectType  string // select
	urlTestType string // url-test / url-latency-test / url-latency-benchmark
	fallback    string // 空表示不支持
	direct      string
	reject      string
	intervalKey string // interval / check-interval
	urlKey      string // url / check-url
}

// reservedNames 是各格式都认可的伪节点，组成员引用它们无需过滤。
var reservedNames = map[string]bool{
	"DIRECT": true, "REJECT": true, "DNS": true, "PASS": true, "LOCAL": true,
	"DRESS": true, "NO-CDN": true, "DIRECT-INTERNET": true, "FINAL": true,
}

// buildGroupLine 把 Clash 代理组渲染成目标格式的一行。
// 成员里引用了被跳过节点的名字会被过滤掉——产出的配置不能含悬空引用。
func buildGroupLine(name string, g render.Group, st groupStyle, valid map[string]bool) string {
	members := make([]string, 0, len(g.Proxies))
	for _, m := range g.Proxies {
		if valid[m] {
			members = append(members, m)
		}
	}
	joined := strings.Join(members, ",")

	switch strings.ToLower(g.Type) {
	case "select", "":
		if st.selectType == "" {
			return ""
		}
		return eq(name, st.selectType+","+joined)
	case "url-test":
		if st.urlTestType == "" {
			return ""
		}
		body := st.urlTestType + "," + joined
		if g.Test != "" {
			body += "," + st.urlKey + "=" + g.Test
		}
		if iv := intStr(g.Options.Interval); iv != "" {
			body += "," + st.intervalKey + "=" + iv
		}
		if t := intStr(g.Options.Tolerance); t != "" {
			body += ",tolerance=" + t
		}
		if to := intStr(g.Options.Timeout); to != "" {
			body += ",timeout=" + to
		}
		return eq(name, body)
	case "fallback":
		if st.fallback == "" {
			return ""
		}
		body := st.fallback + "," + joined
		if g.Test != "" {
			body += "," + st.urlKey + "=" + g.Test
		}
		if iv := intStr(g.Options.Interval); iv != "" {
			body += "," + st.intervalKey + "=" + iv
		}
		return eq(name, body)
	case "direct", "pass", "local":
		return eq(name, st.direct)
	case "reject":
		return eq(name, st.reject)
	default:
		// relay / balanced / sniffer 无法在这些格式里表达。
		return ""
	}
}

func eq(name, body string) string { return name + " = " + body }

func intStr(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

// --- Surge ---

// SurgeFormat 是 Surge 配置格式。
//
// Surge 内核原生只支持 ss / socks / http；ssr 需要外挂 ssr-local，vmess 与 vless
// 完全不支持。这里与 subconverter v0.9.0 的取舍一致：不支持的协议跳过并告警，
// 不尝试硬塞——硬塞出来的行客户端会直接报错。
func SurgeFormat() Format {
	style := groupStyle{
		selectType: "select", urlTestType: "url-test", fallback: "fallback",
		direct: "direct", reject: "REJECT", intervalKey: "interval", urlKey: "url",
	}
	return Format{
		Name:          "surge",
		CommentPrefix: "#",
		HasSections:   true,
		General: []string{
			"loglevel = notify",
			"bypass-system = true",
			"skip-proxy = 127.0.0.1,192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,100.64.0.0/10,localhost,*.local,e.crashlytics.com,captive.apple.com,::ffff:0:0:0:0/1,::ffff:128:0:0:0/1",
			"bypass-tun = 192.168.0.0/16,10.0.0.0/8,172.16.0.0/12",
		},
		ProxyIntro:   []string{"DIRECT = direct"},
		ProxySection: "Proxy",
		GroupSection: "Proxy Group",
		RuleSection:  "Rule",
		Node:         surgeNode,
		Rule:         NewRuleFormatter(simpleRulesetRule, false, false, 86400),
		Group: func(g render.Group, valid map[string]bool) string {
			return buildGroupLine(g.Name, g, style, valid)
		},
	}
}

func surgeNode(n model.Node) (string, bool) {
	p := nodeSpec(n)
	switch p.Type {
	case "ss":
		return eq(p.Name, fmt.Sprintf("ss, %s, %s, encrypt-method=%s, password=%s",
			p.Server, p.Port, p.Cipher, qIf(p.Pwd))), true
	case "socks5", "socks4":
		body := fmt.Sprintf("socks5, %s, %s", p.Server, p.Port)
		if p.User != "" {
			body += ", username=" + qIf(p.User)
		}
		if p.Pwd != "" {
			body += ", password=" + qIf(p.Pwd)
		}
		return eq(p.Name, body), true
	case "http":
		scheme := "http"
		if p.TLS {
			scheme = "https"
		}
		body := fmt.Sprintf("%s, %s, %s", scheme, p.Server, p.Port)
		if p.User != "" {
			body += ", username=" + qIf(p.User)
		}
		if p.Pwd != "" {
			body += ", password=" + qIf(p.Pwd)
		}
		return eq(p.Name, body), true
	}
	return "", false
}

// --- Quantumult（QX classic，target=quan） ---

func QuanFormat() Format {
	style := groupStyle{
		selectType: "select", urlTestType: "url-latency-test",
		direct: "direct", reject: "reject", intervalKey: "check-interval", urlKey: "check-url",
	}
	return Format{
		Name:          "quan",
		CommentPrefix: "#",
		HasSections:   true,
		ProxySection:  "SERVER",
		GroupSection:  "POLICY",
		RuleSection:   "TCP",
		Node:          quanNode,
		Group: func(g render.Group, valid map[string]bool) string {
			return buildGroupLine(g.Name, g, style, valid)
		},
		Rule: NewRuleFormatter(simpleRulesetRule, false, true, 86400),
	}
}

func quanNode(n model.Node) (string, bool) {
	p := nodeSpec(n)
	switch p.Type {
	case "vmess":
		parts := []string{"vmess", p.Server, p.Port, vmessCipher(p.Cipher), qAlways(p.Pwd),
			"group=V2RayProvider", fmt.Sprintf("over-tls=%t", p.TLS)}
		if p.Servername != "" {
			parts = append(parts, "tls-host="+p.Servername)
		}
		if p.Network == "ws" {
			parts = append(parts, "obfs=ws")
			if p.Path != "" {
				parts = append(parts, "obfs-path="+qAlways(p.Path))
			}
			if p.Host != "" {
				parts = append(parts, `obfs-header="Host: `+p.Host+`"`)
			}
		}
		return eq(p.Name, strings.Join(parts, ", ")), true
	case "ss":
		return eq(p.Name, strings.Join([]string{
			"shadowsocks", p.Server, p.Port, p.Cipher, qAlways(p.Pwd), "group=SSProvider",
		}, ", ")), true
	case "ssr":
		return eq(p.Name, strings.Join([]string{
			"shadowsocksr", p.Server, p.Port, p.Cipher, qAlways(p.Pwd), "group=SSRProvider",
			"protocol=" + firstNonEmpty(p.Protocol, "origin"),
			"obfs=" + firstNonEmpty(p.Obfs, "plain"),
		}, ", ")), true
	}
	return "", false
}

// --- Quantumult X（target=quanx） ---

func QuanxFormat() Format {
	style := groupStyle{
		selectType: "static", urlTestType: "url-latency-benchmark",
		direct: "direct", reject: "reject", intervalKey: "check-interval", urlKey: "check-url",
	}
	return Format{
		Name:              "quanx",
		CommentPrefix:     "//!",
		HasSections:       true,
		ProxySection:      "server_local",
		GroupSection:      "policy",
		RuleSection:       "filter_local",
		RemoteRuleSection: "filter_remote",
		Node:              quanxNode,
		Group: func(g render.Group, valid map[string]bool) string {
			return buildGroupLine(g.Name, g, style, valid)
		},
		Rule: NewRuleFormatter(qxRulesetRule, true, false, 86400),
	}
}

// quanxNode 用 Quantumult X 的 "类型 = 主机:端口, ...tag=名字" 形态，节点名放 tag。
func quanxNode(n model.Node) (string, bool) {
	p := nodeSpec(n)
	peer := p.Server + ":" + p.Port
	switch p.Type {
	case "vmess":
		parts := []string{peer, "method=" + vmessCipher(p.Cipher), "password=" + p.Pwd}
		if p.Network == "ws" {
			parts = append(parts, "obfs=wss")
			if p.Server != "" {
				parts = append(parts, "obfs-host="+p.Server)
			}
			if p.Path != "" {
				parts = append(parts, "obfs-uri="+p.Path)
			}
		} else if p.TLS {
			parts = append(parts, "over-tls=true")
		}
		if p.Servername != "" && p.Network != "ws" {
			parts = append(parts, "tls-host="+p.Servername)
		}
		parts = append(parts, "tag="+p.Name)
		return eq("vmess", strings.Join(parts, ", ")), true
	case "ss":
		return eq("shadowsocks", strings.Join([]string{
			peer, "method=" + p.Cipher, "password=" + p.Pwd, "tag=" + p.Name,
		}, ", ")), true
	case "ssr":
		return eq("shadowsocks", strings.Join([]string{
			peer, "method=" + p.Cipher, "password=" + p.Pwd,
			"ssr-protocol=" + firstNonEmpty(p.Protocol, "origin"),
			"obfs=" + firstNonEmpty(p.Obfs, "plain"),
			"tag=" + p.Name,
		}, ", ")), true
	case "trojan":
		parts := []string{peer, "password=" + p.Pwd}
		if p.TLS {
			parts = append(parts, "over-tls=true", "tls-verification=false")
		}
		if p.Servername != "" {
			parts = append(parts, "tls-host="+p.Servername)
		}
		parts = append(parts, "tag="+p.Name)
		return eq("trojan", strings.Join(parts, ", ")), true
	}
	return "", false
}

// --- Loon ---

func LoonFormat() Format {
	style := groupStyle{
		selectType: "select", urlTestType: "url-test", fallback: "fallback",
		direct: "direct", reject: "reject", intervalKey: "interval", urlKey: "url",
	}
	return Format{
		Name:              "loon",
		CommentPrefix:     "#",
		HasSections:       true,
		ProxySection:      "Proxy",
		GroupSection:      "Proxy Group",
		RuleSection:       "Rule",
		RemoteRuleSection: "Remote Rule",
		Node:              loonNode,
		Group: func(g render.Group, valid map[string]bool) string {
			return buildGroupLine(g.Name, g, style, valid)
		},
		Rule: NewRuleFormatter(loonRulesetRule, true, false, 86400),
	}
}

func loonNode(n model.Node) (string, bool) {
	p := nodeSpec(n)
	switch p.Type {
	case "vmess":
		parts := []string{"vmess", p.Server, p.Port, vmessCipher(p.Cipher), qAlways(p.Pwd),
			fmt.Sprintf("over-tls=%t", p.TLS)}
		if p.Servername != "" {
			parts = append(parts, "tls-name="+p.Servername)
		}
		if p.Network == "ws" {
			parts = append(parts, "transport=ws", "path="+p.Path)
			if p.Host != "" {
				parts = append(parts, "host="+p.Host)
			}
		}
		return eq(p.Name, strings.Join(parts, ",")), true
	case "ss":
		return eq(p.Name, strings.Join([]string{
			"Shadowsocks", p.Server, p.Port, p.Cipher, qAlways(p.Pwd),
		}, ",")), true
	case "ssr":
		return eq(p.Name, strings.Join([]string{
			"ShadowsocksR", p.Server, p.Port, p.Cipher, qAlways(p.Pwd),
			"protocol=" + firstNonEmpty(p.Protocol, "origin"),
			"protocol-param=" + p.ProtocolParam,
			"obfs=" + firstNonEmpty(p.Obfs, "plain"),
			"obfs-param=" + p.ObfsParam,
		}, ",")), true
	case "trojan":
		parts := []string{"trojan", p.Server, p.Port, qAlways(p.Pwd)}
		if p.Servername != "" {
			parts = append(parts, "tls-name="+p.Servername)
		}
		if p.SkipVerify {
			parts = append(parts, "skip-cert-verify=true")
		}
		return eq(p.Name, strings.Join(parts, ",")), true
	}
	return "", false
}

// --- Surfboard ---

func SurfboardFormat() Format {
	style := groupStyle{
		selectType: "select", urlTestType: "url-test",
		direct: "direct", reject: "reject", intervalKey: "interval", urlKey: "url",
	}
	return Format{
		Name:          "surfboard",
		CommentPrefix: "#",
		HasSections:   true,
		ProxySection:  "Proxy",
		GroupSection:  "Proxy Group",
		RuleSection:   "Rule",
		Node:          surfboardNode,
		Group: func(g render.Group, valid map[string]bool) string {
			return buildGroupLine(g.Name, g, style, valid)
		},
		Rule: NewRuleFormatter(simpleRulesetRule, false, true, 86400),
	}
}

func surfboardNode(n model.Node) (string, bool) {
	p := nodeSpec(n)
	switch p.Type {
	case "vmess":
		parts := []string{"vmess", p.Server, p.Port, "username=" + p.Pwd}
		if p.TLS {
			parts = append(parts, "tls=true", "vmess-aead=true")
		}
		if p.Network == "ws" {
			parts = append(parts, "ws=true", "ws-path="+p.Path)
			if p.Host != "" {
				parts = append(parts, "ws-headers=Host:"+p.Host)
			}
		}
		if p.Servername != "" {
			parts = append(parts, "sni="+p.Servername)
		}
		return eq(p.Name, strings.Join(parts, ", ")), true
	case "ss":
		return eq(p.Name, strings.Join([]string{
			"ss", p.Server, p.Port,
			"encrypt-method=" + p.Cipher, "password=" + qIf(p.Pwd),
		}, ", ")), true
	case "trojan":
		parts := []string{"trojan", p.Server, p.Port, "password=" + p.Pwd}
		if p.Servername != "" {
			parts = append(parts, "sni="+p.Servername)
		}
		if p.SkipVerify {
			parts = append(parts, "skip-cert-verify=true")
		}
		return eq(p.Name, strings.Join(parts, ", ")), true
	case "socks5", "socks4":
		parts := []string{"socks5", p.Server, p.Port}
		if p.User != "" {
			parts = append(parts, "username="+p.User)
		}
		if p.Pwd != "" {
			parts = append(parts, "password="+p.Pwd)
		}
		return eq(p.Name, strings.Join(parts, ", ")), true
	case "http":
		scheme := "http"
		if p.TLS {
			scheme = "https"
		}
		parts := []string{scheme, p.Server, p.Port}
		if p.User != "" {
			parts = append(parts, "username="+p.User)
		}
		if p.Pwd != "" {
			parts = append(parts, "password="+p.Pwd)
		}
		return eq(p.Name, strings.Join(parts, ", ")), true
	}
	return "", false
}
