package lineform

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// 期望值取自 subconverter v0.9.0 实机输出（tools/scv/，GPL-3.0 仅作参照，
// 不进发行物）；字段顺序、引号约定、节点名放置位置都是客户端解析的硬约束。

func linef(t *testing.T, format Format, n model.Node) (string, bool) {
	t.Helper()
	out, ok := format.Node(n)
	if !ok {
		t.Fatalf("节点 %q（%s）预期可在 %s 表达，实际被跳过", n.Name, n.Type, format.Name)
	}
	if out == "" {
		t.Fatalf("节点 %q 渲染出空行", n.Name)
	}
	return out, ok
}

func nodes(name, typ, server, port string, extra map[string]any) model.Node {
	return model.NewNodeFromParts(name, typ, server, port, extra)
}

func TestSurgeNode(t *testing.T) {
	f := SurgeFormat()
	cases := []struct {
		n    model.Node
		want string
	}{
		{nodes("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), "ss-node = ss, ss.example.com, 8388, encrypt-method=aes-256-gcm, password=password123"},
		{nodes("ss-spaces", "ss", "1.2.3.4", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "a b,c",
		}), `ss-spaces = ss, 1.2.3.4, 8388, encrypt-method=aes-256-gcm, password="a b,c"`},
		{nodes("http-node", "http", "1.2.3.4", "1080", map[string]any{
			"username": "user", "password": "pw",
		}), "http-node = http, 1.2.3.4, 1080, username=user, password=pw"},
		{nodes("socks-node", "socks5", "1.2.3.4", "1080", nil),
			"socks-node = socks5, 1.2.3.4, 1080"},
	}
	for _, c := range cases {
		got, _ := linef(t, f, c.n)
		if got != c.want {
			t.Errorf("Surge\n  实际: %s\n  期望: %s", got, c.want)
		}
	}

	// Surge 内核原生只支持 ss / socks / http；vless / hysteria2 / tuic 等
	// 必须跳过并告警，硬塞出来的行客户端会直接报错。
	for _, n := range []model.Node{
		nodes("vless-node", "vless", "1.2.3.4", "443", map[string]any{
			"udp": true, "tls": true, "uuid": "uuid-x",
			"servername": "1.2.3.4", "network": "tcp",
		}),
		nodes("hy2-node", "hysteria2", "1.2.3.4", "443", map[string]any{
			"udp": true, "password": "pw", "sni": "1.2.3.4",
		}),
		nodes("tuic-node", "tuic", "1.2.3.4", "443", map[string]any{
			"udp": true, "uuid": "u", "password": "p", "sni": "1.2.3.4",
		}),
	} {
		if _, ok := f.Node(n); ok {
			t.Errorf("节点 %q（%s）不应能在 Surge 表达", n.Name, n.Type)
		}
	}
}

func TestQuanNode(t *testing.T) {
	f := QuanFormat()
	cases := []struct {
		n    model.Node
		want string
	}{
		{nodes("vmess-node", "vmess", "v.example.com", "443", map[string]any{
			"udp": true, "tls": true, "cipher": "auto",
			"uuid": "u-1234", "servername": "v.example.com",
			"network": "ws",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "v.example.com"}},
		}), `vmess-node = vmess, v.example.com, 443, chacha20-ietf-poly1305, "u-1234", group=V2RayProvider, over-tls=true, tls-host=v.example.com, obfs=ws, obfs-path="/ws", obfs-header="Host: v.example.com"`},
		{nodes("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), `ss-node = shadowsocks, ss.example.com, 8388, aes-256-gcm, "password123", group=SSProvider`},
		{nodes("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password123",
			"protocol": "origin", "protocol-param": "",
			"obfs": "plain", "obfs-param": "",
		}), `ssr-node = shadowsocksr, ssr.example.com, 1234, aes-256-cfb, "password123", group=SSRProvider, protocol=origin, obfs=plain`},
	}
	for _, c := range cases {
		got, _ := linef(t, f, c.n)
		if got != c.want {
			t.Errorf("QX classic\n  实际: %s\n  期望: %s", got, c.want)
		}
	}
}

func TestQuanxNode(t *testing.T) {
	f := QuanxFormat()
	// QX 用 "类型 = 主机:端口, 字段..."，节点名放 tag。
	cases := []struct {
		n    model.Node
		want string
	}{
		{nodes("vmess-node", "vmess", "v.example.com", "443", map[string]any{
			"udp": true, "tls": true, "cipher": "chacha20-ietf-poly1305",
			"uuid": "u-1234", "network": "ws",
			"ws-opts": map[string]any{"path": "/ws"},
		}), `vmess = v.example.com:443, method=chacha20-ietf-poly1305, password=u-1234, obfs=wss, obfs-host=v.example.com, obfs-uri=/ws, tag=vmess-node`},
		{nodes("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), "shadowsocks = ss.example.com:8388, method=aes-256-gcm, password=password123, tag=ss-node"},
		{nodes("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password123",
		}), "shadowsocks = ssr.example.com:1234, method=aes-256-cfb, password=password123, ssr-protocol=origin, obfs=plain, tag=ssr-node"},
		{nodes("trojan-node", "trojan", "tr.example.com", "443", map[string]any{
			"udp": true, "tls": true, "password": "p-1234",
			"servername": "tr.example.com", "skip-cert-verify": true,
		}), "trojan = tr.example.com:443, password=p-1234, over-tls=true, tls-verification=false, tls-host=tr.example.com, tag=trojan-node"},
	}
	for _, c := range cases {
		got, _ := linef(t, f, c.n)
		if got != c.want {
			t.Errorf("QX\n  实际: %s\n  期望: %s", got, c.want)
		}
	}
}

func TestLoonNode(t *testing.T) {
	f := LoonFormat()
	cases := []struct {
		n    model.Node
		want string
	}{
		{nodes("vmess-node", "vmess", "v.example.com", "443", map[string]any{
			"udp": true, "tls": true, "cipher": "chacha20-ietf-poly1305",
			"uuid": "u-1234", "servername": "v.example.com", "network": "ws",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "v.example.com"}},
		}), `vmess-node = vmess,v.example.com,443,chacha20-ietf-poly1305,"u-1234",over-tls=true,tls-name=v.example.com,transport=ws,path=/ws,host=v.example.com`},
		{nodes("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), `ss-node = Shadowsocks,ss.example.com,8388,aes-256-gcm,"password123"`},
		{nodes("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password123",
			"protocol": "origin", "obfs": "plain",
		}), `ssr-node = ShadowsocksR,ssr.example.com,1234,aes-256-cfb,"password123",protocol=origin,protocol-param=,obfs=plain,obfs-param=`},
		{nodes("trojan-node", "trojan", "tr.example.com", "443", map[string]any{
			"udp": true, "tls": true, "password": "p-1234",
			"servername": "tr.example.com", "skip-cert-verify": true,
		}), `trojan-node = trojan,tr.example.com,443,"p-1234",tls-name=tr.example.com,skip-cert-verify=true`},
	}
	for _, c := range cases {
		got, _ := linef(t, f, c.n)
		if got != c.want {
			t.Errorf("Loon\n  实际: %s\n  期望: %s", got, c.want)
		}
	}
}

func TestSurfboardNode(t *testing.T) {
	f := SurfboardFormat()
	// Surfboard 把 vmess 的 uuid 放 username=，与其余格式的 password= 相反。
	cases := []struct {
		n    model.Node
		want string
	}{
		{nodes("vmess-node", "vmess", "v.example.com", "443", map[string]any{
			"udp": true, "tls": true, "cipher": "auto", "uuid": "u-1234",
			"servername": "v.example.com", "network": "ws",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "v.example.com"}},
		}), `vmess-node = vmess, v.example.com, 443, username=u-1234, tls=true, vmess-aead=true, ws=true, ws-path=/ws, ws-headers=Host:v.example.com, sni=v.example.com`},
		{nodes("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), "ss-node = ss, ss.example.com, 8388, encrypt-method=aes-256-gcm, password=password123"},
		{nodes("trojan-node", "trojan", "tr.example.com", "443", map[string]any{
			"udp": true, "tls": true, "password": "p-1234",
			"servername": "tr.example.com", "skip-cert-verify": true,
		}), "trojan-node = trojan, tr.example.com, 443, password=p-1234, sni=tr.example.com, skip-cert-verify=true"},
	}
	for _, c := range cases {
		got, _ := linef(t, f, c.n)
		if got != c.want {
			t.Errorf("Surfboard\n  实际: %s\n  期望: %s", got, c.want)
		}
	}

	// Surfboard 不支持 SSR（与 subconverter 一致）。
	if _, ok := f.Node(nodes("ssr-node", "ssr", "1.2.3.4", "1234", map[string]any{
		"cipher": "aes-256-cfb", "password": "p",
	})); ok {
		t.Error("ssr 节点不应能在 Surfboard 表达")
	}
}

func allNodes() model.NodeList {
	return model.NodeList{
		nodes("vmess-ws", "vmess", "v.example.com", "443", map[string]any{
			"udp": true, "tls": true, "cipher": "auto", "uuid": "u-1234",
			"servername": "v.example.com", "network": "ws",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "v.example.com"}},
		}),
		nodes("ss-aes", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}),
		nodes("ssr-plain", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password123",
			"protocol": "origin", "obfs": "plain",
		}),
		nodes("trojan-tls", "trojan", "tr.example.com", "443", map[string]any{
			"udp": true, "tls": true, "password": "p-1234",
			"servername": "tr.example.com", "skip-cert-verify": true,
		}),
		nodes("vless-reality", "vless", "vless.example.com", "443", map[string]any{
			"udp": true, "tls": true, "uuid": "u-vless", "network": "tcp",
			"servername": "vless.example.com", "client-fingerprint": "chrome",
			"reality-opts": map[string]any{
				"public-key": "pk1", "short-id": "sid", "fingerprint": "chrome",
			},
		}),
		nodes("hy2-udp", "hysteria2", "hy.example.com", "443", map[string]any{
			"udp": true, "password": "p-hy2", "sni": "hy.example.com",
			"skip-cert-verify": true,
		}),
	}
}

// testRequestMin 只给节点，不给组与规则，用来验证渲染器的兜底产物。
func testRequestMin() render.Request {
	return render.Request{
		Nodes:   allNodes(),
		Profile: render.Profile{Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z"},
	}
}

func testRequest() render.Request {
	return render.Request{
		Nodes: allNodes(),
		Groups: []render.Group{
			{Name: "节点选择", Type: "select", Proxies: []string{"vmess-ws", "ss-aes", "ssr-plain", "trojan-tls"}},
			{Name: "自动测速", Type: "url-test", Proxies: []string{"ss-aes", "ssr-plain"},
				Test:    "https://www.google.com/generate_204",
				Options: render.UrlTestOptions{Interval: 300, Tolerance: 50}},
		},
		Rules: []string{
			"DOMAIN-SUFFIX,example.com,直连",
			"GEOIP,CN,直连",
			"RULE-SET,https://example.com/rules.txt,代理",
			"GEOIP,JP,直连",
			"FINAL,直连",
		},
		Profile: render.Profile{
			Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z",
		},
	}
}

func TestFormats_Golden(t *testing.T) {
	cases := []struct {
		name string
		req  render.Request
	}{
		{"default", testRequest()},
		// 不给组与规则时的兜底产物：真实订阅常不带这两项，缺了组用户就没法选节点。
		{"min", testRequestMin()},
	}
	for _, f := range []Format{SurgeFormat(), QuanFormat(), QuanxFormat(), LoonFormat(), SurfboardFormat()} {
		f := f
		for _, c := range cases {
			c := c
			t.Run(f.Name+"-"+c.name, func(t *testing.T) {
				r := New(f, Options{})
				got, err := r.Render(c.req)
				if err != nil {
					t.Fatalf("Render: %v", err)
				}

				path := filepath.Join("..", "..", "..", "testdata", "golden", "lineform",
					f.Name+"-"+c.name+".txt")
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
}

func TestFormats_Deterministic(t *testing.T) {
	for _, f := range []Format{SurgeFormat(), QuanFormat(), QuanxFormat(), LoonFormat(), SurfboardFormat()} {
		r := New(f, Options{})
		a, err := r.Render(testRequest())
		if err != nil {
			t.Fatalf("%s Render: %v", f.Name, err)
		}
		b, err := r.Render(testRequest())
		if err != nil {
			t.Fatalf("%s Render: %v", f.Name, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s 相同输入两次渲染结果不一致（golden 无法工作）", f.Name)
		}
	}
}

// TestFormats_SkipIsReported 降级不能静默：Node 返回 ok=false 的节点必须进 Warnings，
// 且产物里没有该节点行——否则用户会以为节点都在。
// 跳过名单不由测试写死，直接按各格式的 Node 函数判定，避免表驱动清单与实现漂移。
func TestFormats_SkipIsReported(t *testing.T) {
	for _, f := range []Format{SurgeFormat(), QuanFormat(), QuanxFormat(), LoonFormat(), SurfboardFormat()} {
		r := New(f, Options{})
		_, err := r.Render(testRequest())
		if err != nil {
			t.Fatalf("%s Render: %v", f.Name, err)
		}
		warns := strings.ToLower(strings.Join(r.Warnings(), "\n"))

		var kept, dropped []string
		for _, n := range testRequest().Nodes {
			if _, ok := f.Node(n); ok {
				kept = append(kept, n.Name)
			} else {
				dropped = append(dropped, n.Name)
			}
		}
		if len(dropped) == 0 {
			t.Errorf("%s 应当跳过 testRequest 里的节点（各格式协议支持面本就不一致）", f.Name)
		}
		for _, name := range dropped {
			if !strings.Contains(warns, strings.ToLower(name)) {
				t.Errorf("%s 跳过 %q 但没有告警：%v", f.Name, name, r.Warnings())
			}
		}
		for _, name := range kept {
			if strings.Contains(warns, strings.ToLower(name)) {
				t.Errorf("%s 把已渲染的节点 %q 报成跳过：%v", f.Name, name, r.Warnings())
			}
		}
	}
}

// TestFormats_ProtocolSupport 记录各格式的协议支持面，
// 防止"为了通过 golden 悄悄改支持面"。
func TestFormats_ProtocolSupport(t *testing.T) {
	type testCase struct {
		name, typ, cipher, password string
	}
	cases := []testCase{
		{"vmess", "vmess", "chacha20-ietf-poly1305", "u-1234"},
		{"ss", "ss", "aes-256-gcm", "p"},
		{"ssr", "ssr", "aes-256-cfb", "p"},
		{"trojan", "trojan", "", "p"},
	}
	for _, n := range cases {
		raw := map[string]any{
			"udp": true, "tls": true, "cipher": n.cipher,
			"password": n.password, "uuid": n.password,
			"servername": "1.2.3.4", "network": "tcp",
		}
		node := nodes(n.name+"-node", n.typ, "1.2.3.4", "443", raw)

		// 格式 -> 是否支持。留空表示该协议在此格式下必须被跳过。
		want := map[string]map[string]bool{
			"surge":     {"ss": true},
			"quan":      {"vmess": true, "ss": true, "ssr": true},
			"quanx":     {"vmess": true, "ss": true, "ssr": true, "trojan": true},
			"loon":      {"vmess": true, "ss": true, "ssr": true, "trojan": true},
			"surfboard": {"vmess": true, "ss": true, "trojan": true},
		}
		for _, f := range []Format{SurgeFormat(), QuanFormat(), QuanxFormat(), LoonFormat(), SurfboardFormat()} {
			_, ok := f.Node(node)
			if want[f.Name][n.typ] != ok {
				t.Errorf("%s 对 %s 的支持 = %t，与预期不一致", f.Name, n.typ, ok)
			}
		}
	}
}

// TestFormats_GroupReferencesFiltered 代理组成员引用了被跳过节点时，
// 产物里的组行必须只剩真实存在的名字——悬空引用会让客户端整个组失效。
func TestFormats_GroupReferencesFiltered(t *testing.T) {
	cases := []struct {
		name   string
		format Format
		want   string
	}{
		{"surge-select", SurgeFormat(), "节点选择"},
		{"surge-urltest", SurgeFormat(), "自动测速"},
		{"quanx-select", QuanxFormat(), "节点选择"},
		{"loon-select", LoonFormat(), "节点选择"},
		{"surfboard-select", SurfboardFormat(), "节点选择"},
	}
	for _, c := range cases {
		r := New(c.format, Options{})
		body, err := r.Render(testRequest())
		if err != nil {
			t.Fatalf("%s Render: %v", c.format.Name, err)
		}

		var got string
		for _, line := range strings.Split(string(body), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), c.want+" = "); ok {
				got = v
				break
			}
		}
		if got == "" {
			t.Fatalf("%s 产物里找不到代理组 %q", c.format.Name, c.want)
		}

		// 被跳过的节点名不该留在组里：悬空引用会让整个组失效。
		switch c.format.Name {
		case "surge", "surfboard":
			if strings.Contains(got, "ssr-plain") {
				t.Errorf("%s 组行仍引用被跳过的 ssr-plain: %q", c.format.Name, got)
			}
		case "quan":
			if strings.Contains(got, "trojan-tls") {
				t.Errorf("quan 组行仍引用被跳过的 trojan-tls: %q", got)
			}
		}
		if strings.Count(got, ",") < 1 {
			t.Errorf("%s 组 %q 的成员为空: %q", c.format.Name, c.want, got)
		}
	}
}

// TestFormats_RuleMapping 规则词汇差异必须落在各自的段落里，
// 且 GEOSITE 这种内核专属类型只丢弃不报错。
func TestFormats_RuleMapping(t *testing.T) {
	for _, f := range []Format{SurgeFormat(), QuanFormat(), QuanxFormat(), LoonFormat(), SurfboardFormat()} {
		out := f.Rule("RULE-SET,https://example.com/rules.txt,代理")
		switch f.Name {
		case "surge", "quan", "surfboard":
			// 这三个格式把 RULE-SET 直接写进本地规则段，没有独立远程规则段。
			want := "RULE-SET,https://example.com/rules.txt,代理,update-interval=86400"
			if out.Local != want {
				t.Errorf("%s 规则集 = %q，期望 %q", f.Name, out.Local, want)
			}
			if out.Remote != "" {
				t.Errorf("%s 不该有远程规则段: %q", f.Name, out.Remote)
			}
		case "quanx", "loon":
			// 这两个有独立远程规则段，规则集走 Remote。
			if out.Local != "" {
				t.Errorf("%s 不该有本地规则: %q", f.Name, out.Local)
			}
			if f.Name == "quanx" {
				if out.Remote != "rule-list=https://example.com/rules.txt,group=代理,update-interval=86400" {
					t.Errorf("quanx 规则集 = %q", out.Remote)
				}
			} else {
				if out.Remote != "https://example.com/rules.txt,代理" {
					t.Errorf("loon 规则集 = %q", out.Remote)
				}
			}
		}

		if out := f.Rule("GEOSITE,private,直连"); out.Local != "" || out.Remote != "" {
			t.Errorf("%s 不应表达 GEOSITE：%+v", f.Name, out)
		}
		if out := f.Rule("FINAL,直连"); out.Local != "FINAL,直连" {
			t.Errorf("%s FINAL = %+v", f.Name, out)
		}
	}

	// QX classic 给 IP-CIDR 类追加 no-resolve。
	q := QuanFormat()
	if out := q.Rule("IP-CIDR,8.8.8.8/32,直连"); out.Local != "IP-CIDR,8.8.8.8/32,直连,no-resolve" {
		t.Errorf("QX IP-CIDR = %q", out.Local)
	}
}

// TestGroupLine 代理组词汇与成员过滤是各格式最容易写错的地方，单独断言。
// 直接打 buildGroupLine：组词汇由 groupStyle 决定，不需要经过 Format 的闭包。
func TestGroupLine(t *testing.T) {
	valid := map[string]bool{
		"vmess-ws": true, "ss-aes": true, "节点选择": true, "DIRECT": true,
	}
	cases := []struct {
		style groupStyle
		g     render.Group
		want  string
	}{
		{groupStyle{selectType: "select", urlTestType: "url-test", fallback: "fallback",
			direct: "direct", reject: "REJECT", intervalKey: "interval", urlKey: "url"},
			render.Group{Name: "A", Type: "select", Proxies: []string{"vmess-ws", "ss-aes", "missing"}},
			"A = select,vmess-ws,ss-aes"},
		{groupStyle{selectType: "select", urlTestType: "url-latency-test",
			direct: "direct", reject: "reject", intervalKey: "check-interval", urlKey: "check-url"},
			render.Group{Name: "B", Type: "url-test", Proxies: []string{"ss-aes"},
				Test:    "https://www.google.com/generate_204",
				Options: render.UrlTestOptions{Interval: 300, Tolerance: 50}},
			"B = url-latency-test,ss-aes,check-url=https://www.google.com/generate_204,check-interval=300,tolerance=50"},
		{groupStyle{selectType: "static", urlTestType: "url-latency-benchmark",
			direct: "direct", reject: "reject", intervalKey: "check-interval", urlKey: "check-url"},
			render.Group{Name: "C", Type: "select", Proxies: []string{"vmess-ws", "ss-aes"}},
			"C = static,vmess-ws,ss-aes"},
		{groupStyle{selectType: "select", urlTestType: "url-test", fallback: "fallback",
			direct: "direct", reject: "reject", intervalKey: "interval", urlKey: "url"},
			render.Group{Name: "D", Type: "fallback", Proxies: []string{"ss-aes"}},
			"D = fallback,ss-aes"},
		// QX 没有 fallback 类型：返回空串，由 Render 计入告警。
		{groupStyle{selectType: "static", urlTestType: "url-latency-benchmark",
			direct: "direct", reject: "reject", intervalKey: "check-interval", urlKey: "check-url"},
			render.Group{Name: "F", Type: "fallback", Proxies: []string{"ss-aes"}}, ""},
		// relay 在任何格式里都无法表达。
		{groupStyle{selectType: "select", urlTestType: "url-test",
			direct: "direct", reject: "reject", intervalKey: "interval", urlKey: "url"},
			render.Group{Name: "G", Type: "relay", Proxies: []string{"ss-aes"}}, ""},
		// 成员引用了不存在的名字：过滤掉，不产出悬空引用。
		{groupStyle{selectType: "select", direct: "direct", reject: "reject"},
			render.Group{Name: "H", Type: "select", Proxies: []string{"missing"}},
			"H = select,"},
	}
	for i, c := range cases {
		got := buildGroupLine(c.g.Name, c.g, c.style, valid)
		if got != c.want {
			t.Errorf("case %d 组 %q = %q，期望 %q", i, c.g.Name, got, c.want)
		}
	}
}

// TestVmessCipherAuto vmess 的 cipher "auto" 在内核里合法，但客户端不认，
// 必须落成实际算法。
func TestVmessCipherAuto(t *testing.T) {
	if got := vmessCipher("auto"); got != "chacha20-ietf-poly1305" {
		t.Errorf("auto = %q", got)
	}
	if got := vmessCipher(""); got != "chacha20-ietf-poly1305" {
		t.Errorf("空 = %q", got)
	}
	if got := vmessCipher("aes-128-gcm"); got != "aes-128-gcm" {
		t.Errorf("aes-128-gcm = %q", got)
	}
}
