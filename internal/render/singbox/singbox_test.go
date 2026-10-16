package singbox

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// testNodes 覆盖 sing-box 有实现的协议全集，用来逐条核对字段映射。
// raw map 的键名沿用 mihomo 归一化后的形态（docs/adr/0001）。
// ssr 节点刻意留在夹具里：sing-box 1.12 起移除 shadowsocksr 出站，
// 它必须被跳过并进告警，而不是映射到一个不存在的类型。
func testNodes() model.NodeList {
	return model.NodeList{
		model.NewNodeFromParts("vmess-node", "vmess", "vmess.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "alterId": 0, "cipher": "auto",
			"tls": true, "servername": "vmess.example.com", "network": "ws", "udp": true,
			"ws-opts": map[string]any{
				"path": "/ws", "headers": map[string]any{"Host": "vmess.example.com"},
			},
		}),
		model.NewNodeFromParts("vless-reality", "vless", "vless.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "flow": "xtls-rprx-vision",
			"tls": true, "servername": "vless.example.com", "network": "tcp", "udp": true,
			"client-fingerprint": "chrome",
			"reality-opts": map[string]any{
				"public-key": "9GWwAr72DJJ45eXBbzDnD3K+wQ4tglpMiy4s8V6W2Sg=", "short-id": "0123456789abcdef",
			},
		}),
		model.NewNodeFromParts("trojan-node", "trojan", "trojan.example.com", "443", map[string]any{
			"password": "password123", "sni": "trojan.example.com",
			"skip-cert-verify": true, "network": "tcp", "udp": true,
		}),
		// up 给数字、down 给字符串两种形态都可能出现，都要落成整数。
		model.NewNodeFromParts("hy2-node", "hysteria2", "hy2.example.com", "443", map[string]any{
			"password": "password123", "sni": "hy2.example.com", "up": 10, "down": "20",
		}),
		// 故意不带 up/down：机场链接省略速度是常态，渲染器得兜底，不能产出
		// 加载失败的配置。
		model.NewNodeFromParts("hy-node", "hysteria", "hy.example.com", "443", map[string]any{
			"password": "password123", "sni": "hy.example.com", "alpn": []any{"h3"},
		}),
		model.NewNodeFromParts("hy-speeds-node", "hysteria", "hys.example.com", "443", map[string]any{
			"password": "password123", "sni": "hys.example.com", "alpn": []any{"h3"},
			"up": 100, "down": 200,
		}),
		model.NewNodeFromParts("tuic-node", "tuic", "tuic.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "password": "password123",
			"sni": "tuic.example.com", "alpn": []any{"h3"}, "congestion-controller": "bbr",
		}),
		model.NewNodeFromParts("anytls-node", "anytls", "anytls.example.com", "443", map[string]any{
			"username": "user1", "password": "password123", "sni": "anytls.example.com",
		}),
		model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123", "udp": true,
		}),
		model.NewNodeFromParts("ssr-node", "ssr", "ssr.example.com", "8388", map[string]any{
			"cipher": "chacha20", "password": "password123",
			"protocol": "auth_sha1_v2", "obfs": "tls1.2_ticket_auth", "udp": true,
		}),
		model.NewNodeFromParts("socks-node", "socks5", "socks.example.com", "1080", map[string]any{
			"username": "user1", "password": "pass1",
		}),
		model.NewNodeFromParts("http-node", "http", "http.example.com", "8080", map[string]any{
			"username": "user1", "password": "pass1",
		}),
		// https 在 sing-box 没有独立出站类型，落成 http + tls。
		model.NewNodeFromParts("https-node", "https", "https.example.com", "8443", map[string]any{
			"username": "user1", "password": "pass1", "sni": "https.example.com", "tls": true,
		}),
	}
}

func testGroups() []render.Group {
	return []render.Group{
		{Name: "pick", Type: "select", Proxies: []string{"vmess-node", "ss-node", "nope"}},
		{Name: "auto", Type: "url-test", Proxies: []string{"vmess-node", "ss-node"},
			Options: render.UrlTestOptions{Interval: 300, Tolerance: 50, Timeout: 6000}},
		{Name: "fb", Type: "fallback", Proxies: []string{"vmess-node", "ss-node"},
			Options: render.UrlTestOptions{Interval: 180}},
		{Name: "empty", Type: "select", Proxies: []string{"nope"}},
	}
}

func testRequest() render.Request {
	return render.Request{
		Nodes:  testNodes(),
		Groups: testGroups(),
		Rules: []string{
			"DOMAIN-SUFFIX,google.com,pick",
			"DOMAIN-SUFFIX,gstatic.com,pick",
			"DOMAIN,ai.openai.com,DIRECT",
			"DOMAIN-KEYWORD,openai,pick",
			"IP-CIDR,10.0.0.0/8,DIRECT",
			"IP-CIDR6,::1/128,DIRECT",
			"GEOIP,CN,DIRECT",
			"GEOSITE,geolocation-!cn,pick",
			"MATCH,pick",
		},
		Profile: render.Profile{Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z"},
	}
}

func renderTest(t *testing.T, req render.Request) []byte {
	t.Helper()
	body, err := New(DefaultOptions()).Render(req)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return body
}

// outboundsOf 取产物里的 outbounds 段。
func outboundsOf(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	outs := make([]map[string]any, 0)
	for _, o := range doc["outbounds"].([]any) {
		outs = append(outs, o.(map[string]any))
	}
	return outs
}

// TestRender_ValidJSON 产物必须能被标准 JSON 解码——sing-box 的配置文件
// 是严格 JSON，多了注释头就整份加载失败。
func TestRender_ValidJSON(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatalf("产物不是合法 JSON: %v", err)
	}
}

// TestTopLevelKeys 顶层段落顺序与内容按参考产物固定。
func TestTopLevelKeys(t *testing.T) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	want := []string{"log", "dns", "ntp", "inbounds", "outbounds", "route", "experimental"}
	for _, k := range want {
		if _, ok := doc[k]; !ok {
			t.Errorf("缺少顶层段落 %q", k)
		}
	}
}

// TestBuiltinOutbounds 只有两个内置出站：direct 与 block。
// 1.12 起 dns 出站被移除，DNS 解析改成 dns 段自己驱动。
func TestBuiltinOutbounds(t *testing.T) {
	outs := outboundsOf(t, renderTest(t, testRequest()))
	if len(outs) < 2 {
		t.Fatalf("出站过少: %v", outs)
	}
	for i, w := range []struct {
		typ, tag string
	}{
		{"direct", "DIRECT"},
		{"block", "REJECT"},
	} {
		if outs[i]["type"] != w.typ || outs[i]["tag"] != w.tag {
			t.Errorf("内置出站[%d] = %v，期望 type=%s tag=%s", i, outs[i], w.typ, w.tag)
		}
	}
}

// TestNoRemovedOutboundTypes 1.12 移除的出站类型不能以任何形式出现。
func TestNoRemovedOutboundTypes(t *testing.T) {
	outs := outboundsOf(t, renderTest(t, testRequest()))
	banned := map[string]bool{"dns": true, "shadowsocksr": true, "fallback": true}
	for _, o := range outs {
		if banned[o["type"].(string)] {
			t.Errorf("出现了 sing-box 1.12 已移除的出站类型 %v: %v", o["type"], o)
		}
	}
}

// TestNoUdpFieldAnywhere 1.12 起 udp 字段从所有出站删除——sing-box 自己管
// 双栈，配置里写 udp 会被直接判为非法配置。
func TestNoUdpFieldAnywhere(t *testing.T) {
	for _, o := range outboundsOf(t, renderTest(t, testRequest())) {
		if _, has := o["udp"]; has {
			t.Errorf("出站 %v 仍含 udp 字段: %v", o["tag"], o)
		}
	}
}

// TestOutboundFields 逐协议核对字段映射。字段名错了配置能加载但节点连不上，
// 只能靠这份断言表兜住。
func TestOutboundFields(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	byTag := map[string]map[string]any{}
	for _, o := range doc["outbounds"].([]any) {
		m := o.(map[string]any)
		byTag[m["tag"].(string)] = m
	}

	cases := []struct {
		tag     string
		typ     string
		fields  map[string]any
		sub     string // 需要额外核对的子对象
		subWant map[string]any
	}{
		{
			tag: "vmess-node", typ: "vmess",
			fields: map[string]any{
				"server": "vmess.example.com", "server_port": float64(443),
				"uuid": "11111111-2222-3333-4444-555555555555", "alter_id": float64(0),
				"security": "auto",
			},
			sub: "transport", subWant: map[string]any{"type": "ws", "path": "/ws"},
		},
		{
			tag: "vless-reality", typ: "vless",
			fields: map[string]any{
				"server": "vless.example.com", "server_port": float64(443),
				"flow": "xtls-rprx-vision",
			},
			sub: "tls", subWant: map[string]any{"enabled": true, "server_name": "vless.example.com"},
		},
		{
			tag: "trojan-node", typ: "trojan",
			fields: map[string]any{"password": "password123", "server_port": float64(443)},
			sub:    "tls", subWant: map[string]any{
				"enabled": true, "server_name": "trojan.example.com", "insecure": true,
			},
		},
		{
			tag: "hy2-node", typ: "hysteria2",
			// up_mbps 必须是整数，不能落成字符串。
			fields: map[string]any{
				"password": "password123", "up_mbps": float64(10), "down_mbps": float64(20),
			},
			sub: "tls", subWant: map[string]any{"enabled": true, "server_name": "hy2.example.com"},
		},
		{
			// hysteria（v1）出站的凭据字段叫 auth_str，不叫 password。
			// 速度上限强制要求，节点没给就按 defaultBandwidthMbps 兜底。
			tag: "hy-node", typ: "hysteria",
			fields: map[string]any{
				"auth_str": "password123",
				"up_mbps":  float64(defaultBandwidthMbps), "down_mbps": float64(defaultBandwidthMbps),
			},
			sub: "tls", subWant: map[string]any{"enabled": true, "server_name": "hy.example.com"},
		},
		{
			tag: "hy-speeds-node", typ: "hysteria",
			fields: map[string]any{
				"auth_str": "password123", "up_mbps": float64(100), "down_mbps": float64(200),
			},
			sub: "tls", subWant: map[string]any{"enabled": true, "server_name": "hys.example.com"},
		},
		{
			tag: "tuic-node", typ: "tuic",
			fields: map[string]any{"congestion_control": "bbr"},
			sub:    "tls", subWant: map[string]any{"enabled": true},
		},
		{
			// anytls 出站只有 password 一个凭据字段。
			tag: "anytls-node", typ: "anytls",
			fields: map[string]any{"password": "password123", "server_port": float64(443)},
			sub:    "tls", subWant: map[string]any{"enabled": true, "server_name": "anytls.example.com"},
		},
		{
			tag: "ss-node", typ: "shadowsocks",
			fields: map[string]any{"method": "aes-256-gcm", "password": "password123"},
		},
		{
			tag: "socks-node", typ: "socks",
			fields: map[string]any{"username": "user1", "password": "pass1", "server_port": float64(1080)},
		},
		{
			tag: "http-node", typ: "http",
			fields: map[string]any{"username": "user1", "password": "pass1", "server_port": float64(8080)},
		},
		{
			// https 映射成 http 出站 + tls 子对象。
			tag: "https-node", typ: "http",
			fields: map[string]any{"username": "user1", "password": "pass1", "server_port": float64(8443)},
			sub:    "tls", subWant: map[string]any{"enabled": true, "server_name": "https.example.com"},
		},
	}
	for _, c := range cases {
		got, ok := byTag[c.tag]
		if !ok {
			t.Errorf("缺少出站 %s", c.tag)
			continue
		}
		if got["type"] != c.typ {
			t.Errorf("%s type = %v，期望 %s", c.tag, got["type"], c.typ)
		}
		for k, v := range c.fields {
			if got[k] != v {
				t.Errorf("%s.%s = %v (%T)，期望 %v", c.tag, k, got[k], got[k], v)
			}
		}
		if _, has := got["username"]; has && c.tag == "anytls-node" {
			t.Error("anytls 不应有 username 字段，凭据要回落到 password")
		}
		if c.sub != "" {
			sub, ok := got[c.sub].(map[string]any)
			if !ok {
				t.Errorf("%s 缺少 %s 子对象: %v", c.tag, c.sub, got)
				continue
			}
			for k, v := range c.subWant {
				if sub[k] != v {
					t.Errorf("%s.%s.%s = %v (%T)，期望 %v", c.tag, c.sub, k, sub[k], sub[k], v)
				}
			}
		}
	}
}

// TestVlessRealityNesting reality 与 utls 在 1.12 起都归到 tls 子对象里，
// 顶层不再接受 reality 字段；utls 也从字符串变成 {enabled, fingerprint}。
func TestVlessRealityNesting(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	for _, o := range doc["outbounds"].([]any) {
		m := o.(map[string]any)
		if m["tag"] != "vless-reality" {
			continue
		}
		if _, has := m["reality"]; has {
			t.Error("reality 不应是出站顶层字段，1.12 起它在 tls 子对象里")
		}
		tls, ok := m["tls"].(map[string]any)
		if !ok {
			t.Fatal("vless-reality 缺少 tls 子对象")
		}
		// utls 在 1.12 起从字符串变成 {enabled, fingerprint} 对象。
		u, ok := tls["utls"].(map[string]any)
		if !ok {
			t.Errorf("tls.utls = %v，期望 {enabled, fingerprint} 对象而不是字符串", tls["utls"])
		} else if u["fingerprint"] != "chrome" || u["enabled"] != true {
			t.Errorf("tls.utls = %v", tls["utls"])
		}
		real, ok := tls["reality"].(map[string]any)
		if !ok {
			t.Fatal("tls.reality 缺失")
		}
		wantReal := map[string]any{
			// 夹具里是 Clash 生态的标准 base64，产物必须是 URL 安全无填充写法。
			"enabled":    true,
			"public_key": "9GWwAr72DJJ45eXBbzDnD3K-wQ4tglpMiy4s8V6W2Sg",
			"short_id":   "0123456789abcdef",
		}
		for k, v := range wantReal {
			if real[k] != v {
				t.Errorf("tls.reality.%s = %v，期望 %v", k, real[k], v)
			}
		}
		if _, has := real["spider_x"]; has {
			t.Error("spider_x 在 1.12 起被移除，不应输出")
		}
	}
}

// TestRealityPublicKey Clash 生态的标准 base64 公钥必须转成 sing-box 要的
// URL 安全无填充写法，否则实机判 "illegal base64 data"。
func TestRealityPublicKey(t *testing.T) {
	for _, c := range []struct {
		in, want string
		bad      bool
	}{
		{"9GWwAr72DJJ45eXBbzDnD3K+wQ4tglpMiy4s8V6W2Sg=", "9GWwAr72DJJ45eXBbzDnD3K-wQ4tglpMiy4s8V6W2Sg", false},
		// 已经是 URL 安全无填充写法时原样返回。
		{"9GWwAr72DJJ45eXBbzDnD3K-wQ4tglpMiy4s8V6W2Sg", "9GWwAr72DJJ45eXBbzDnD3K-wQ4tglpMiy4s8V6W2Sg", false},
		{"", "", false},
		{"not-base64-at-all++", "not-base64-at-all++", true},
	} {
		got, bad := realityPublicKey(c.in)
		if got != c.want || bad != c.bad {
			t.Errorf("realityPublicKey(%q) = (%q, %v)，期望 (%q, %v)", c.in, got, bad, c.want, c.bad)
		}
	}
}

// TestRealityPublicKeyBadWarned 解不出来的公钥要告警，不能静默产出加载失败的配置。
func TestRealityPublicKeyBadWarned(t *testing.T) {
	req := render.Request{
		Nodes: model.NodeList{
			model.NewNodeFromParts("bad-key", "vless", "h.example.com", "443", map[string]any{
				"uuid": "11111111-2222-3333-4444-555555555555", "tls": true,
				"reality-opts": map[string]any{"public-key": "definitely not base64 +++"},
			}),
		},
		Profile: render.Profile{Name: "t"},
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "bad-key") {
		t.Errorf("非法公钥应逐节点告警: %q", warns)
	}
}

// TestTransportWsHeaders ws 传输的 Host 必须落在 headers 子对象里，
// 而不是平铺成 host 字段——平铺写法 sing-box 读不到。
func TestTransportWsHeaders(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	for _, o := range doc["outbounds"].([]any) {
		m := o.(map[string]any)
		if m["tag"] != "vmess-node" {
			continue
		}
		tr, ok := m["transport"].(map[string]any)
		if !ok {
			t.Fatal("vmess-node 缺少 transport")
		}
		h, ok := tr["headers"].(map[string]any)
		if !ok || h["Host"] != "vmess.example.com" {
			t.Errorf("transport.headers = %v，期望 Host=vmess.example.com", tr["headers"])
		}
		if _, has := tr["host"]; has {
			t.Error("ws 传输不应有平铺的 host 字段")
		}
	}
}

// TestGroups 组映射成 selector / urltest；fallback 退化成 selector。
func TestGroups(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	byTag := map[string]map[string]any{}
	for _, o := range doc["outbounds"].([]any) {
		m := o.(map[string]any)
		byTag[m["tag"].(string)] = m
	}

	if m := byTag["pick"]; m == nil || m["type"] != "selector" {
		t.Errorf("pick = %v，期望 selector", byTag["pick"])
	} else {
		// 成员被过滤掉无效 tag（nope），不能留下指向不存在出站的引用。
		for _, p := range m["outbounds"].([]any) {
			if p == "nope" {
				t.Error("pick 含未导出的成员 nope")
			}
		}
	}
	if m := byTag["auto"]; m == nil || m["type"] != "urltest" {
		t.Errorf("auto = %v，期望 urltest", byTag["auto"])
	} else {
		if m["interval"] != "300s" {
			t.Errorf("auto.interval = %v，期望 300s（Clash 的秒要转成时长字面量）", m["interval"])
		}
		if m["tolerance"] != float64(50) {
			t.Errorf("auto.tolerance = %v，期望 50", m["tolerance"])
		}
		if _, has := m["timeout"]; has {
			t.Error("sing-box 的 urltest 无 timeout 字段，不应输出")
		}
	}
	// fallback 出站类型在 1.12 起被移除，只能退化成 selector——不能静默丢掉。
	if m := byTag["fb"]; m == nil || m["type"] != "selector" {
		t.Errorf("fb = %v，期望退化成 selector", byTag["fb"])
	}
	if _, has := byTag["empty"]; has {
		t.Error("无有效成员的组不应输出——客户端会因引用不存在而整份加载失败")
	}
}

// TestRouteRules 规则按字段合并成数组，动作落在 action 对象里。
func TestRouteRules(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	route := doc["route"].(map[string]any)
	rules := route["rules"].([]any)
	if len(rules) == 0 {
		t.Fatal("route.rules 为空")
	}
	for _, r := range rules {
		m := r.(map[string]any)
		seen := map[string]bool{}
		for k := range m {
			if seen[k] {
				t.Errorf("规则对象含重复字段 %s: %v", k, m)
			}
			seen[k] = true
		}
		// 动作是平铺的 outbound 字段。
		if _, has := m["outbound"]; !has {
			t.Errorf("规则缺少 outbound: %v", m)
		}
		// 不能包成 {action:{action:"route",outbound:...}}：sing-box 1.14 会
		// 直接报 "cannot unmarshal object into Go struct field ...action of type
		// string"。官方 schema.json 在这一点上是错的（见 compatibility.md B7）。
		if _, has := m["action"]; has {
			t.Errorf("规则不应有 action 字段: %v", m)
		}
	}

	// DOMAIN-SUFFIX 的两行应合并进同一个对象的数组。
	var suffix map[string]any
	for _, r := range rules {
		m := r.(map[string]any)
		if _, ok := m["domain_suffix"]; ok {
			suffix = m
			break
		}
	}
	if suffix == nil {
		t.Fatal("缺少 domain_suffix 规则")
	}
	if n := len(suffix["domain_suffix"].([]any)); n != 2 {
		t.Errorf("domain_suffix 元素数 = %d，期望 2（相邻同字段应合并）", n)
	}
	if suffix["outbound"] != "pick" {
		t.Errorf("domain_suffix 出站 = %v，期望 pick", suffix["outbound"])
	}

	// MATCH / FINAL 不落成规则行，改写 route.final。
	for _, r := range rules {
		m := r.(map[string]any)
		for k := range m {
			if k == "match" || k == "final" {
				t.Errorf("MATCH/FINAL 不应落成规则字段 %s", k)
			}
		}
	}
	if route["final"] != "pick" {
		t.Errorf("route.final = %v，期望 pick（来自 MATCH 的行动）", route["final"])
	}
	// 1.12 起隐式默认解析器被弃用、1.14 起缺了它配置直接非法。
	if route["default_domain_resolver"] != DefaultOptions().DomainResolver {
		t.Errorf("route.default_domain_resolver = %v，期望 %s",
			route["default_domain_resolver"], DefaultOptions().DomainResolver)
	}
}

// TestRouteRules_MergeBoundary 同字段但不同出站的相邻行必须拆开，
// 合并会丢出站归属。
func TestRouteRules_MergeBoundary(t *testing.T) {
	req := testRequest()
	req.Rules = []string{
		"DOMAIN-SUFFIX,a.com,pick",
		"DOMAIN-SUFFIX,b.com,DIRECT",
		"DOMAIN-SUFFIX,c.com,pick",
	}
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, req), &doc); err != nil {
		t.Fatal(err)
	}
	rules := doc["route"].(map[string]any)["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("规则对象数 = %d，期望 3（不同出站不能合并）\n%v", len(rules), rules)
	}
}

// TestRouteRules_DropIsReported 无对应字段的形态必须丢弃并进 Warnings，
// 不能静默丢弃——用户以为规则在，其实整条没了。
func TestRouteRules_DropIsReported(t *testing.T) {
	req := testRequest()
	req.Rules = []string{
		"DOMAIN-SUFFIX,google.com,pick",
		"SCRIPT,example.com,pick",
		"RULE-SET,proxies.yaml,pick",
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "SCRIPT") {
		t.Errorf("SCRIPT 应计入告警: %q", warns)
	}
	if !strings.Contains(warns, "RULE-SET") {
		t.Errorf("RULE-SET 应计入告警: %q", warns)
	}
}

// TestRouteRules_GeoIPGeoSiteWarned geoip / geosite 在 1.12 起不再是内置匹配，
// 只能引用 route.rule_set 指向的第三方规则集文件；本实现不打包规则数据，
// 因此必须显式丢弃并告警，而不是产出客户端读不懂的规则行。
func TestRouteRules_GeoIPGeoSiteWarned(t *testing.T) {
	r := New(DefaultOptions())
	if _, err := r.Render(testRequest()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "GEOIP") {
		t.Errorf("GEOIP 应计入告警: %q", warns)
	}
	if !strings.Contains(warns, "GEOSITE") {
		t.Errorf("GEOSITE 应计入告警: %q", warns)
	}
	body, err := New(DefaultOptions()).Render(testRequest())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	for _, rule := range doc["route"].(map[string]any)["rules"].([]any) {
		m := rule.(map[string]any)
		for _, banned := range []string{"geoip", "geosite"} {
			if _, has := m[banned]; has {
				t.Errorf("规则含已移除的匹配字段 %s: %v", banned, m)
			}
		}
	}
}

// TestRouteRules_DanglingOutboundWarned 规则指向的出站被跳过时不能留下悬空引用，
// 否则整份配置加载失败。
func TestRouteRules_DanglingOutboundWarned(t *testing.T) {
	req := render.Request{
		Nodes:   testNodes(),
		Rules:   []string{"DOMAIN-SUFFIX,ssr-only.com,ssr-node", "MATCH,pick"},
		Profile: render.Profile{Name: "t"},
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "ssr-node") {
		t.Errorf("指向被跳转过站的规则应计入告警: %q", warns)
	}
	body, err := New(DefaultOptions()).Render(req)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	for _, rule := range doc["route"].(map[string]any)["rules"].([]any) {
		m := rule.(map[string]any)
		if m["outbound"] == "ssr-node" {
			t.Error("不应留下指向不存在出站的规则引用")
		}
	}
}

// TestUnsupportedProtocol_Warned 协议不受支持必须逐节点告警。
func TestUnsupportedProtocol_Warned(t *testing.T) {
	req := render.Request{
		Nodes: model.NodeList{
			model.NewNodeFromParts("mieru-node", "mieru", "host.example.com", "443", map[string]any{}),
		},
		Profile: render.Profile{Name: "t"},
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "mieru") {
		t.Errorf("未支持协议应逐节点告警: %q", warns)
	}
	for _, o := range outboundsOf(t, renderTest(t, req)) {
		if o["tag"] == "mieru-node" {
			t.Error("不支持协议的节点不应出现在 outbounds")
		}
	}
}

// TestSSRWarned ssr 节点被跳过必须告警——用户以为还在，其实整条没了。
func TestSSRWarned(t *testing.T) {
	r := New(DefaultOptions())
	if _, err := r.Render(render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "t"}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "ssr-node") {
		t.Errorf("ssr 节点应逐条告警: %q", warns)
	}
	for _, o := range outboundsOf(t, renderTest(t, testRequest())) {
		if o["tag"] == "ssr-node" {
			t.Error("ssr 节点不应出现在 outbounds")
		}
	}
}

// TestHysteriaMissingSpeedDefaulted hysteria（v1）出站强制要求 up_mbps 和
// down_mbps 同时存在，缺一即 missing upload/download speed，整份配置加载不了。
// 节点链接里省略速度是常态，所以必须兜底而不是丢节点。
func TestHysteriaMissingSpeedDefaulted(t *testing.T) {
	one := func(extra map[string]any) (map[string]any, string) {
		r := New(DefaultOptions())
		raw := map[string]any{
			"password": "password123", "sni": "h.example.com", "alpn": []any{"h3"},
		}
		for k, v := range extra {
			raw[k] = v
		}
		nd := model.NewNodeFromParts("hy", "hysteria", "h.example.com", "443", raw)
		if _, err := r.Render(render.Request{Nodes: []model.Node{nd}, Profile: render.Profile{Name: "t"}}); err != nil {
			t.Fatalf("Render: %v", err)
		}
		var ob map[string]any
		for _, o := range outboundsOf(t, renderTest(t, render.Request{Nodes: []model.Node{nd}, Profile: render.Profile{Name: "t"}})) {
			if o["tag"] == "hy" {
				ob = o
			}
		}
		if ob == nil {
			t.Fatalf("hysteria 节点丢了 outbounds: %v", outboundsOf(t, renderTest(t, render.Request{Nodes: []model.Node{nd}, Profile: render.Profile{Name: "t"}})))
		}
		return ob, strings.Join(r.Warnings(), "\n")
	}

	ob, warns := one(nil)
	if ob["up_mbps"] != float64(defaultBandwidthMbps) || ob["down_mbps"] != float64(defaultBandwidthMbps) {
		t.Errorf("两个速度都缺时应各自兜底 %d，实际 %v / %v", defaultBandwidthMbps, ob["up_mbps"], ob["down_mbps"])
	}
	if !strings.Contains(warns, "没有声明 up/down 速度") {
		t.Errorf("缺速度应有告警: %q", warns)
	}

	ob, warns = one(map[string]any{"up": 50})
	if ob["up_mbps"] != float64(50) {
		t.Errorf("up 应原样保留，实际 %v", ob["up_mbps"])
	}
	if ob["down_mbps"] != float64(defaultBandwidthMbps) {
		t.Errorf("down 缺失应单独兜底 %d，实际 %v", defaultBandwidthMbps, ob["down_mbps"])
	}
	if !strings.Contains(warns, "缺少 down 速度") {
		t.Errorf("单侧缺失应有告警: %q", warns)
	}

	ob, warns = one(map[string]any{"up": 50, "down": 60})
	if ob["up_mbps"] != float64(50) || ob["down_mbps"] != float64(float64(60)) {
		t.Errorf("两侧都给时应原样保留，实际 %v / %v", ob["up_mbps"], ob["down_mbps"])
	}
	if warns != "" {
		t.Errorf("两侧齐全不应告警: %q", warns)
	}
}

// TestInbounds 端口与 TUN 开关都要生效；TUN 用 address，1.12 起不再有 stack。
func TestInbounds(t *testing.T) {
	r := New(Options{
		MixedPort: 7890, AllowLan: false, Tun: true,
		TunAddress: "172.19.0.1/30",
		DNS:        DefaultOptions().DNS,
	})
	body, err := r.Render(render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "t"}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	ins := doc["inbounds"].([]any)
	if len(ins) != 2 {
		t.Fatalf("inbounds = %v", ins)
	}
	mixed := ins[0].(map[string]any)
	if mixed["listen"] != "127.0.0.1" {
		t.Errorf("listen = %v，AllowLan=false 应监听 127.0.0.1", mixed["listen"])
	}
	if mixed["listen_port"] != float64(7890) {
		t.Errorf("listen_port = %v", mixed["listen_port"])
	}
	tun := ins[1].(map[string]any)
	if tun["address"] != "172.19.0.1/30" {
		t.Errorf("address = %v，期望 172.19.0.1/30", tun["address"])
	}
	// 1.12 起 inet4_address / stack / sniff 都从 tun inbound 删除。
	for _, banned := range []string{"inet4_address", "stack", "sniff"} {
		if _, has := tun[banned]; has {
			t.Errorf("tun 不应有 %s 字段: %v", banned, tun)
		}
	}

	r2 := New(Options{MixedPort: 0, Tun: false, DNS: DefaultOptions().DNS})
	body2, err := r2.Render(render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	var doc2 map[string]any
	if err := json.Unmarshal(body2, &doc2); err != nil {
		t.Fatal(err)
	}
	if n := len(doc2["inbounds"].([]any)); n != 0 {
		t.Errorf("端口与 TUN 都关闭时 inbounds = %v", doc2["inbounds"])
	}
}

// TestDns DNS 段的服务器列表、规则动作与 fakeip 池都要在。
func TestDns(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	dns := doc["dns"].(map[string]any)
	servers := dns["servers"].([]any)
	if len(servers) != 4 {
		t.Fatalf("servers = %v", servers)
	}
	wantTags := []string{"dns_proxy", "dns_direct", "dns_fakeip", "dns_resolver"}
	for i, want := range wantTags {
		if got := servers[i].(map[string]any)["tag"]; got != want {
			t.Errorf("servers[%d].tag = %v，期望 %s", i, got, want)
		}
	}
	if dns["final"] != "dns_direct" {
		t.Errorf("dns.final = %v", dns["final"])
	}
	// fakeip 地址池落在 fakeip 类型的服务器项上，不再是 dns 段的 fakeip 子对象。
	fakeip := servers[2].(map[string]any)
	if fakeip["type"] != "fakeip" {
		t.Errorf("servers[2].type = %v", fakeip["type"])
	}
	if fakeip["inet4_range"] != "198.18.0.0/15" {
		t.Errorf("fakeip.inet4_range = %v", fakeip["inet4_range"])
	}
	if n := len(dns["rules"].([]any)); n != 1 {
		t.Errorf("dns.rules = %v，期望 1 条兜底规则", dns["rules"])
	}
}

// TestDnsNoRemovedKeys dns 段不接受 1.12 已删除的字段。
func TestDnsNoRemovedKeys(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	dns := doc["dns"].(map[string]any)
	for _, banned := range []string{
		"fakeip", "independent_cache", "default", "resolver", "proxy", "block",
	} {
		if _, has := dns[banned]; has {
			t.Errorf("dns 不应有 %s 字段: %v", banned, dns)
		}
	}
	for _, s := range dns["servers"].([]any) {
		m := s.(map[string]any)
		if _, has := m["address"]; has {
			t.Errorf("dns server %v 用了已移除的 address 字段，应改用 server/server_port: %v",
				m["tag"], m)
		}
	}
}

// TestDnsDefaultByRule 1.12 起 dns.default 字段被移除，兜底解析器只能写成
// 一条匹配所有域名的规则；action 是平铺字符串。
func TestDnsDefaultByRule(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	dns := doc["dns"].(map[string]any)
	if dns["final"] != DefaultOptions().DNS.FinalDNSTag {
		t.Errorf("dns.final = %v", dns["final"])
	}
	rule := dns["rules"].([]any)[0].(map[string]any)
	if len(rule["domain"].([]any)) != 1 || rule["domain"].([]any)[0] != "*" {
		t.Errorf("兜底规则 = %v，期望 domain:*", rule)
	}
	if rule["action"] != "route" {
		t.Errorf("DNS 规则 action = %v，期望 route", rule["action"])
	}
	if rule["server"] != DefaultOptions().DNS.DefaultDNSTag {
		t.Errorf("DNS 规则 server = %v，期望 %s", rule["server"], DefaultOptions().DNS.DefaultDNSTag)
	}
}

// TestDnsRefWarned DNS 段引用了不存在的解析器必须告警——悬空引用会让客户端
// 整份配置加载失败，比不生成更糟。
func TestDnsRefWarned(t *testing.T) {
	opts := DefaultOptions()
	for i := range opts.DNS.Servers {
		opts.DNS.Servers[i].Resolver = "no-such-resolver"
	}
	opts.DNS.DefaultDNSTag = "no-such-resolver"
	opts.DNS.FinalDNSTag = "no-such-resolver"
	opts.DomainResolver = "no-such-resolver"
	r := New(opts)
	if _, err := r.Render(render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "t"}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	for _, want := range []string{
		"no-such-resolver", "domain_resolver", "dns.final", "default_domain_resolver",
	} {
		if !strings.Contains(warns, want) {
			t.Errorf("告警应提到 %q: %q", want, warns)
		}
	}
}

// TestDnsHostnameNeedsResolver 以主机名接入的 DNS 服务器若不声明 domain_resolver，
// sing-box 1.14 会判配置非法，必须告警。
func TestDnsHostnameNeedsResolver(t *testing.T) {
	opts := DefaultOptions()
	opts.DNS.Servers = []DNSServer{{Tag: "bad", Address: "https://dns.example.com/dns-query"}}
	r := New(opts)
	if _, err := r.Render(render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "t"}}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := strings.Join(r.Warnings(), "\n")
	if !strings.Contains(warns, "domain_resolver") {
		t.Errorf("主机名 DNS 服务器缺 domain_resolver 应告警: %q", warns)
	}
}

// TestDnsDefaultRoutesThroughProxy 兜底解析器必须指向 dns_proxy 而不是
// dns_resolver——dns_resolver 唯一用途是替 dns_proxy 解析 1.1.1.1 自己，
// 兜底指过去会让所有域名直连解析、fakeip 永不生效。
func TestDnsDefaultRoutesThroughProxy(t *testing.T) {
	if got := DefaultOptions().DNS.DefaultDNSTag; got != "dns_proxy" {
		t.Errorf("DefaultDNSTag = %q，期望 dns_proxy", got)
	}
}

// TestRender_Deterministic 输出必须字节稳定，否则 golden 无法工作。
func TestRender_Deterministic(t *testing.T) {
	a := renderTest(t, testRequest())
	b := renderTest(t, testRequest())
	if !bytes.Equal(a, b) {
		t.Fatalf("两次渲染不一致\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
}

// TestRender_Golden 字节级金测（SUBFORGE_UPDATE_GOLDEN=1 可刷新）。
func TestRender_Golden(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  render.Request
	}{
		{"default", testRequest()},
		{"min", render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "t"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTest(t, tc.req)
			path := filepath.Join("..", "..", "..", "testdata", "golden", "singbox", tc.name+".json")
			if os.Getenv("SUBFORGE_UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Skip("已刷新 golden")
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("读取 golden（先跑 SUBFORGE_UPDATE_GOLDEN=1 go test）: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("输出与 golden 不一致\n--- 实际 ---\n%s\n--- golden ---\n%s", got, want)
			}
		})
	}
}

// TestName 渲染器名决定 X-Convert-Format 与落盘名。
func TestName(t *testing.T) {
	if got := New(DefaultOptions()).Name(); got != "sing-box" {
		t.Errorf("Name = %q", got)
	}
}
