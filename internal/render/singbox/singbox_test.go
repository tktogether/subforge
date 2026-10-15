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
				"public-key": "abcPUBLICKEY", "short-id": "0123456789abcdef",
			},
		}),
		model.NewNodeFromParts("trojan-node", "trojan", "trojan.example.com", "443", map[string]any{
			"password": "password123", "sni": "trojan.example.com",
			"skip-cert-verify": true, "network": "tcp", "udp": true,
		}),
		model.NewNodeFromParts("hy2-node", "hysteria2", "hy2.example.com", "443", map[string]any{
			"password": "password123", "sni": "hy2.example.com",
		}),
		model.NewNodeFromParts("hy-node", "hysteria", "hy.example.com", "443", map[string]any{
			"password": "password123", "sni": "hy.example.com", "alpn": []any{"h3"},
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

// TestBuiltinOutbounds 三个内置出站必须在前，tag 名与参考产物一致。
func TestBuiltinOutbounds(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	outs := doc["outbounds"].([]any)
	if len(outs) < 3 {
		t.Fatalf("出站过少: %v", outs)
	}
	want := []struct {
		typ, tag string
	}{
		{"direct", "DIRECT"},
		{"block", "REJECT"},
		{"dns", "dns-out"},
	}
	for i, w := range want {
		got := outs[i].(map[string]any)
		if got["type"] != w.typ || got["tag"] != w.tag {
			t.Errorf("内置出站[%d] = %v，期望 type=%s tag=%s", i, got, w.typ, w.tag)
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
				"security": "auto", "udp": true,
			},
			sub: "transport", subWant: map[string]any{"type": "ws", "path": "/ws"},
		},
		{
			tag: "vless-reality", typ: "vless",
			fields: map[string]any{
				"server": "vless.example.com", "server_port": float64(443),
				"flow": "xtls-rprx-vision", "udp": true,
			},
			sub: "reality", subWant: map[string]any{
				"public_key": "abcPUBLICKEY", "short_id": "0123456789abcdef",
			},
		},
		{
			tag: "trojan-node", typ: "trojan",
			fields: map[string]any{
				"password": "password123", "server_port": float64(443), "udp": true,
			},
			sub: "tls", subWant: map[string]any{
				"enabled": true, "server_name": "trojan.example.com", "insecure": true,
			},
		},
		{
			tag: "hy2-node", typ: "hysteria2",
			fields: map[string]any{"password": "password123", "udp": true},
			sub:    "tls", subWant: map[string]any{"enabled": true, "server_name": "hy2.example.com"},
		},
		{
			tag: "hy-node", typ: "hysteria",
			fields: map[string]any{"password": "password123", "udp": true},
			sub:    "tls", subWant: map[string]any{"enabled": true},
		},
		{
			tag: "tuic-node", typ: "tuic",
			fields: map[string]any{"udp": true, "congestion_controller": "bbr"},
			sub:    "tls", subWant: map[string]any{"enabled": true},
		},
		{
			tag: "anytls-node", typ: "anytls",
			fields: map[string]any{"username": "user1", "password": "password123", "udp": true},
		},
		{
			tag: "ss-node", typ: "shadowsocks",
			fields: map[string]any{"method": "aes-256-gcm", "password": "password123", "udp": true},
		},
		{
			tag: "ssr-node", typ: "shadowsocksr",
			fields: map[string]any{
				"method": "chacha20", "password": "password123",
				"protocol": "auth_sha1_v2", "obfs": "tls1.2_ticket_auth", "udp": true,
			},
		},
		{
			tag: "socks-node", typ: "socks",
			fields: map[string]any{"username": "user1", "password": "pass1", "server_port": float64(1080)},
		},
		{
			tag: "http-node", typ: "http",
			fields: map[string]any{"username": "user1", "password": "pass1", "server_port": float64(8080)},
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
			t.Error("transport 不应有平铺的 host 字段")
		}
	}
}

// TestGroups 组映射成 selector / urltest / fallback 三种出站。
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
	if m := byTag["fb"]; m == nil || m["type"] != "fallback" {
		t.Errorf("fb = %v，期望 fallback", byTag["fb"])
	} else if m["interval"] != "180s" {
		t.Errorf("fb.interval = %v，期望 180s", m["interval"])
	}
	if _, has := byTag["empty"]; has {
		t.Error("无有效成员的组不应输出——客户端会因引用不存在而整份加载失败")
	}
}

// TestRouteRules 规则按字段合并成数组，不能产出重复字段。
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
		if _, ok := m["outbound"]; !ok {
			t.Errorf("规则对象缺少 outbound: %v", m)
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
	var doc map[string]any
	body := renderTest(t, req)
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	for _, o := range doc["outbounds"].([]any) {
		if o.(map[string]any)["tag"] == "mieru-node" {
			t.Error("不支持协议的节点不应出现在 outbounds")
		}
	}
}

// TestInbounds 端口与 TUN 开关都要生效。
func TestInbounds(t *testing.T) {
	r := New(Options{
		MixedPort: 7890, AllowLan: false, Tun: true,
		TunAddress: "172.19.0.1/30", TunStack: "gvisor",
		DNS: DefaultOptions().DNS,
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
	if tun["stack"] != "gvisor" {
		t.Errorf("stack = %v", tun["stack"])
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

// TestDns DNS 段的服务器顺序、规则与 fakeip 都要在。
func TestDns(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(renderTest(t, testRequest()), &doc); err != nil {
		t.Fatal(err)
	}
	dns := doc["dns"].(map[string]any)
	servers := dns["servers"].([]any)
	if len(servers) != 5 {
		t.Fatalf("servers = %v", servers)
	}
	wantTags := []string{"dns_proxy", "dns_direct", "dns_fakeip", "dns_resolver", "block"}
	for i, want := range wantTags {
		if servers[i].(map[string]any)["tag"] != want {
			t.Errorf("servers[%d].tag = %v，期望 %s", i, servers[i].(map[string]any)["tag"], want)
		}
	}
	if dns["final"] != "dns_direct" {
		t.Errorf("dns.final = %v", dns["final"])
	}
	fakeip := dns["fakeip"].(map[string]any)
	if fakeip["inet4_range"] != "198.18.0.0/15" {
		t.Errorf("fakeip.inet4_range = %v", fakeip["inet4_range"])
	}
	if len(dns["rules"].([]any)) != 4 {
		t.Errorf("dns.rules = %v", dns["rules"])
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
