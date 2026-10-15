package mellow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

func testNodes() model.NodeList {
	return model.NodeList{
		model.NewNodeFromParts("vmess-node", "vmess", "vmess.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "alterId": 0, "cipher": "auto",
			"tls": true, "servername": "vmess.example.com", "network": "ws",
			"ws-opts": map[string]any{
				"path": "/ws", "headers": map[string]any{"Host": "vmess.example.com"},
			},
		}),
		model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}),
	}
}

func testRequest() render.Request {
	return render.Request{
		Nodes: testNodes(),
		Groups: []render.Group{
			{Name: "pick", Type: "select", Proxies: []string{"vmess-node", "ss-node"}},
			{Name: "auto", Type: "url-test", Proxies: []string{"pick"},
				Options: render.UrlTestOptions{Interval: 300, Tolerance: 50, Timeout: 6000}},
			{Name: "fb", Type: "fallback", Proxies: []string{"vmess-node", "ss-node"}},
		},
		Rules: []string{
			"DOMAIN-SUFFIX,google.com,pick",
			"GEOIP,CN,DIRECT",
			"IP-CIDR,10.0.0.0/8,pick",
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

// TestNodeLine 节点行形态：名字, 协议, 分享链接。
// 凭据必须完整——参考产物这里是空的（见包注释的 C8），本实现有意偏离。
func TestNodeLine(t *testing.T) {
	body := renderTest(t, testRequest())

	for _, want := range []string{
		"vmess-node, vmess, vmess://",
		"ss-node, ss, ss://YWVzLTI1Ni1nY206cGFzc3dvcmQxMjM@ss.example.com:8388",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("产物缺少 %q", want)
		}
	}
	// ss 链接里的 method:password 必须被 base64 编码，不能出现裸冒号。
	// 这正是参考产物的缺陷形态（ss://Og@…，Og 是 base64 的 ":"，即两段皆空）。
	if strings.Contains(string(body), "ss://Og@") {
		t.Error("ss 节点凭据为空，形态与参考产物的缺陷一致")
	}
}

// TestNodeURIcarriesCredentials 各协议的凭据字段都得在链接里，一条漏了就是一条坏节点。
func TestNodeURIcarriesCredentials(t *testing.T) {
	cases := []struct {
		name, typ string
		raw       map[string]any
	}{
		{"vless-node", "vless", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "tls": true,
			"servername": "www.microsoft.com", "client-fingerprint": "chrome",
		}},
		{"trojan-node", "trojan", map[string]any{
			"password": "password123", "sni": "trojan.example.com",
		}},
		{"hy2-node", "hysteria2", map[string]any{
			"password": "password123", "sni": "hy2.example.com",
		}},
		{"tuic-node", "tuic", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "password": "password123",
		}},
		{"anytls-node", "anytls", map[string]any{
			"password": "password123", "sni": "anytls.example.com",
		}},
	}
	nodes := model.NodeList{}
	for _, c := range cases {
		raw := map[string]any{"udp": true}
		for k, v := range c.raw {
			raw[k] = v
		}
		nodes = append(nodes, model.NewNodeFromParts(c.name, c.typ, "host.example.com", "443", raw))
	}
	r := New(DefaultOptions())
	body, err := r.Render(render.Request{Nodes: nodes})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := r.Warnings(); len(got) != 0 {
		t.Fatalf("全部协议都应可导出，却有告警: %v", got)
	}
	for _, c := range cases {
		// 这几种协议的分享链接 scheme 与协议名同名（vless://、trojan://…），
		// 所以整行前缀可以精确断言。
		want := c.name + ", " + c.typ + ", " + c.typ + "://"
		if !strings.Contains(string(body), want) {
			t.Errorf("缺少 %s 节点行前缀 %q", c.typ, want)
		}
	}
}

// TestBuiltinEndpointsFirst 三个内置端点必须在 [Endpoint] 段最前面，顺序固定。
func TestBuiltinEndpointsFirst(t *testing.T) {
	body := string(renderTest(t, testRequest()))
	idx := strings.Index(body, "[Endpoint]")
	if idx < 0 {
		t.Fatal("缺 [Endpoint] 段")
	}
	rest := body[idx:]
	if !strings.HasPrefix(rest, "[Endpoint]\nDIRECT, builtin, freedom, domainStrategy=UseIP\nREJECT, builtin, blackhole\nDns-Out, builtin, dns\n") {
		t.Fatalf("内置端点顺序或内容不对:\n%s", rest[:200])
	}
}

// TestGroupLine 代理组模式映射：select→static、url-test→latency、fallback→fallback。
func TestGroupLine(t *testing.T) {
	valid := map[string]bool{
		"DIRECT": true, "REJECT": true, "Dns-Out": true,
		"vmess-node": true, "ss-node": true, "pick": true,
	}
	cases := []struct {
		g    render.Group
		want string
	}{
		{render.Group{Name: "pick", Type: "select", Proxies: []string{"vmess-node", "ss-node"}},
			"pick, vmess-node:ss-node, static"},
		{render.Group{Name: "auto", Type: "url-test", Proxies: []string{"pick"},
			Options: render.UrlTestOptions{Interval: 300, Tolerance: 50, Timeout: 6000}},
			"auto, pick, latency, interval=300, timeout=6"},
		{render.Group{Name: "fb", Type: "fallback", Proxies: []string{"vmess-node", "ss-node"}},
			"fb, vmess-node:ss-node, fallback"},
	}
	for _, c := range cases {
		if got := groupLine(c.g, valid); got != c.want {
			t.Errorf("groupLine(%q) = %q，应为 %q", c.g.Name, got, c.want)
		}
	}
}

// TestGroupLineTimeoutUnit 超时单位是秒。Clash 侧是毫秒，不换算会等到一小时才超时。
func TestGroupLineTimeoutUnit(t *testing.T) {
	valid := map[string]bool{"n": true}
	if got := groupLine(render.Group{
		Name: "auto", Type: "url-test", Proxies: []string{"n"},
		Options: render.UrlTestOptions{Interval: 600, Timeout: 10000},
	}, valid); !strings.HasSuffix(got, "interval=600, timeout=10") {
		t.Errorf("10000ms 应转成 timeout=10，实际: %q", got)
	}
	// 亚秒超时向下取整到 1 秒：0 秒会被客户端当成不超时。
	if got := groupLine(render.Group{
		Name: "auto", Type: "url-test", Proxies: []string{"n"},
		Options: render.UrlTestOptions{Interval: 600, Timeout: 400},
	}, valid); !strings.HasSuffix(got, "timeout=1") {
		t.Errorf("400ms 应取整成 timeout=1，实际: %q", got)
	}
}

// TestGroupLineFiltersInvalidMembers 成员引用了被跳过/不存在的节点时必须过滤，
// 不能产出悬空引用——那样客户端整个组直接失效。
func TestGroupLineFiltersInvalidMembers(t *testing.T) {
	valid := map[string]bool{"n": true}
	if got := groupLine(render.Group{
		Name: "g", Type: "select", Proxies: []string{"n", "missing"},
	}, valid); got != "g, n, static" {
		t.Errorf("应过滤不存在的成员，实际: %q", got)
	}
	// 全部成员都无效时整组跳过，返回空串。
	if got := groupLine(render.Group{
		Name: "g", Type: "select", Proxies: []string{"missing"},
	}, valid); got != "" {
		t.Errorf("无有效成员时应返回空串，实际: %q", got)
	}
}

// TestRuleLine 规则映射与丢弃策略。
func TestRuleLine(t *testing.T) {
	cases := []struct {
		rule string
		want string
		err  bool
	}{
		{"DOMAIN-SUFFIX,google.com,pick", "DOMAIN-SUFFIX,google.com,pick", false},
		{"DOMAIN-KEYWORD,msn,pick", "DOMAIN-KEYWORD,msn,pick", false},
		{"DOMAIN,example.com,pick", "DOMAIN,example.com,pick", false},
		{"IP-CIDR,10.0.0.0/8,pick", "IP-CIDR,10.0.0.0/8,pick,no-resolve", false},
		{"IP-CIDR6,fc00::/7,pick", "IP-CIDR6,fc00::/7,pick,no-resolve", false},
		{"IP-CIDRNOLOG,10.0.0.0/8,pick", "IP-CIDRNOLOG,10.0.0.0/8,pick", false},
		{"GEOIP,CN,DIRECT", "GEOIP,CN,DIRECT", false},
		{"PROCESS-NAME,Safari,pick", "PROCESS-NAME,Safari,pick", false},
		{"URL-REGEX,(torrent),pick", "URL-REGEX,(torrent),pick", false},
		{"FINAL,pick", "FINAL,pick", false},
		// MATCH 是内核专属收尾语法，Mellow 只认 FINAL。
		{"MATCH,pick", "FINAL,pick", false},
		// 以下三类必须丢弃：GEOSITE 依赖内核 mmdb，RULE-SET 语法无参考可核实，
		// 未知类型不猜测。
		{"GEOSITE,geolocation-!cn,pick", "", true},
		{"RULE-SET,https://x.example.com/r.surge,rules", "", true},
		{"SCRIPT,https://x.example.com/s.js,pick", "", true},
		{"BADTYPE", "", true},
	}
	for _, c := range cases {
		got, err := ruleLine(c.rule)
		if c.err {
			if err == nil || got != "" {
				t.Errorf("规则 %q 应被丢弃，实际 got=%q err=%v", c.rule, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("规则 %q 不应报错: %v", c.rule, err)
			continue
		}
		if got != c.want {
			t.Errorf("ruleLine(%q) = %q，应为 %q", c.rule, got, c.want)
		}
	}
}

// TestRender_DropsDontGoSilent 丢弃必须变成告警，静默丢规则比丢节点更危险——
// 用户以为规则都生效了，实际流量全走兜底。
func TestRender_DropsDontGoSilent(t *testing.T) {
	req := testRequest()
	req.Rules = []string{
		"GEOIP,CN,DIRECT",
		"GEOSITE,geolocation-!cn,pick",
		"RULE-SET,https://x.example.com/r.surge,rules",
		"MATCH,pick",
	}
	r := New(DefaultOptions())
	if _, err := r.Render(req); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := r.Warnings()
	if len(warns) != 2 {
		t.Fatalf("应有 2 条丢弃告警，实际 %d: %v", len(warns), warns)
	}
	joined := strings.Join(warns, "\n")
	for _, frag := range []string{"GEOSITE", "RULE-SET"} {
		if !strings.Contains(joined, frag) {
			t.Errorf("告警应指出被丢的规则类型 %q: %s", frag, joined)
		}
	}
	body := string(renderTest(t, req))
	if strings.Contains(body, "GEOSITE") || strings.Contains(body, "RULE-SET") {
		t.Errorf("产物不应包含被丢弃的规则:\n%s", body)
	}
}

// TestRender_SkippedNodesWarned 无法表达的协议节点要跳过并记名。
func TestRender_SkippedNodesWarned(t *testing.T) {
	nodes := append(testNodes(),
		model.NewNodeFromParts("mieru-node", "mieru", "m.example.com", "443", map[string]any{}))
	r := New(DefaultOptions())
	if _, err := r.Render(render.Request{Nodes: nodes}); err != nil {
		t.Fatalf("Render: %v", err)
	}
	warns := r.Warnings()
	if len(warns) != 1 || !strings.Contains(warns[0], "mieru-node") {
		t.Fatalf("应只跳过 mieru 节点并记名，实际: %v", warns)
	}
}

// TestRender_ManagedConfig 自动更新首行。
func TestRender_ManagedConfig(t *testing.T) {
	opts := DefaultOptions()
	opts.ManagedConfigURL = "https://x.example.com/sub?target=mellow"
	opts.UpdateInterval = 7200
	body, err := New(opts).Render(testRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.HasPrefix(string(body), "#!MANAGED-CONFIG https://x.example.com/sub?target=mellow interval=7200 strict=false\n") {
		t.Errorf("首行不是 MANAGED-CONFIG:\n%s", body[:120])
	}
}

// TestRender_DefaultGroups 没有组配置时生成最小可用的兜底组，成员只引用真实节点。
func TestRender_DefaultGroups(t *testing.T) {
	req := render.Request{Nodes: testNodes()}
	body := string(renderTest(t, req))
	for _, want := range []string{
		"[EndpointGroup]",
		"节点选择, DIRECT:ss-node:vmess-node, static",
		"自动测速, ",
		"latency, interval=300, timeout=6",
		"[RoutingRule]",
		"GEOIP,CN,DIRECT",
		"FINAL,节点选择",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("产物缺少 %q", want)
		}
	}
}

// TestRender_DefaultGroupsNoNodes 无节点时不生成引用不存在成员的组。
func TestRender_DefaultGroupsNoNodes(t *testing.T) {
	body := string(renderTest(t, render.Request{Nodes: model.NodeList{}}))
	if strings.Contains(body, "[EndpointGroup]") {
		t.Errorf("无节点时不应生成代理组:\n%s", body)
	}
	// 收尾仍要兜底到内置 DIRECT，否则 Mellow 不知道流量去哪。
	if !strings.Contains(body, "FINAL,DIRECT") {
		t.Errorf("无节点时收尾规则应兜底 DIRECT:\n%s", body)
	}
}

// TestRender_NoDuplicateRuleTail 收尾规则只有一个，兜底不能重复。
func TestRender_NoDuplicateRuleTail(t *testing.T) {
	body := renderTest(t, testRequest())
	text := string(body)
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "FINAL,") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("FINAL 收尾规则应恰好一条，实际 %d", n)
	}
}

// TestRender_SectionOrder 段落顺序与参考输出一致，Mellow 解析器对顺序有要求。
func TestRender_SectionOrder(t *testing.T) {
	body := string(renderTest(t, testRequest()))
	want := []string{"[Endpoint]", "[Routing]", "[Dns]", "[Log]", "[EndpointGroup]", "[RoutingRule]"}
	pos := -1
	for _, s := range want {
		idx := strings.Index(body, s)
		if idx < 0 {
			t.Fatalf("缺段落 %s", s)
		}
		if idx <= pos {
			t.Errorf("段落 %s 顺序错乱（应在 %d 之后，实际 %d）", s, pos, idx)
		}
		pos = idx
	}
}

// TestRender_DNServers DNS 服务器列表写入 [DnsServer] 段；未配置时不写该段。
func TestRender_DNServers(t *testing.T) {
	opts := DefaultOptions()
	opts.DNS = []string{"223.5.5.5", "8.8.8.8"}
	body, err := New(opts).Render(testRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"[DnsServer]", "223.5.5.5", "8.8.8.8"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("产物缺少 %q", want)
		}
	}

	opts = DefaultOptions()
	opts.DNS = nil
	body, err = New(opts).Render(testRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(string(body), "[DnsServer]") {
		t.Error("未配置 DNS 时不应写 [DnsServer] 段")
	}
}

// TestRender_Deterministic 输出必须字节稳定，否则 golden 无法工作。
func TestRender_Deterministic(t *testing.T) {
	req := testRequest()
	a := renderTest(t, req)
	b := renderTest(t, req)
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
		{"min", render.Request{Nodes: testNodes(), Profile: testRequest().Profile}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTest(t, tc.req)
			path := filepath.Join("..", "..", "..", "testdata", "golden", "mellow", tc.name+".txt")
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

// TestName 目标名固定，X-Convert-Format 与落盘名都从它派生。
func TestName(t *testing.T) {
	if got := New(DefaultOptions()).Name(); got != "mellow" {
		t.Errorf("Name = %q，应为 mellow", got)
	}
}
