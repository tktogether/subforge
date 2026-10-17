package sharelink

import (
	"encoding/base64"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/render"
)

// derivedKeys 与 parse 包 roundtrip_test.go 一致：内核解析时补的默认值。
var derivedKeys = map[string]bool{
	"udp":  true,
	"xudp": true,
}

func normalize(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if derivedKeys[k] {
			continue
		}
		out[k] = v
	}
	if ws, ok := out["ws-opts"].(map[string]any); ok {
		if h, ok := ws["headers"].(map[string]any); ok {
			cp := map[string]any(h)
			delete(cp, "User-Agent")
			ws = map[string]any(ws)
			ws["headers"] = cp
			out["ws-opts"] = ws
		}
	}
	return out
}

func mustParse(t *testing.T, buf []byte) []map[string]any {
	t.Helper()
	nodes, err := parse.DecodeSubscription(buf)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return nodes
}

func ssNode() model.Node {
	return model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
		"cipher": "aes-256-gcm", "password": "password123",
	})
}

// assertSubset 递归断言 got 包含 want 的所有键且值相等（got 可有额外键）。
func assertSubset(t *testing.T, got, want map[string]any) {
	t.Helper()
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Errorf("roundtrip 缺字段 %q", k)
			continue
		}
		if wmap, ok := wv.(map[string]any); ok {
			if gmap, ok := gv.(map[string]any); ok {
				assertSubset(t, gmap, wmap)
				continue
			}
			t.Errorf("字段 %q 类型不匹配: got=%T want=map", k, gv)
			continue
		}
		if !reflect.DeepEqual(gv, wv) {
			t.Errorf("字段 %q: got=%v want=%v", k, gv, wv)
		}
	}
}

// --- 生成：每个协议的黄金链接 ---

func TestSharelinkRender_Golden(t *testing.T) {
	cases := []struct {
		name string
		node model.Node
		gold string
	}{
		{"ss", model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQxMjM@ss.example.com:8388#ss-node"},
		{"ssr", model.NewNodeFromParts("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password123", "protocol": "origin", "obfs": "plain",
		}), "ssr://c3NyLmV4YW1wbGUuY29tOjEyMzQ6b3JpZ2luOmFlcy0yNTYtY2ZiOnBsYWluOmNHRnpjM2R2Y21ReE1qTS8_cmVtYXJrcz1jM055TFc1dlpHVQ"},
		{"vmess", model.NewNodeFromParts("vmess-node", "vmess", "vmess.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "alterId": "0", "cipher": "auto",
			"network": "ws", "tls": true, "servername": "vmess.example.com",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "vmess.example.com"}},
		}), "vmess://eyJhZGQiOiJ2bWVzcy5leGFtcGxlLmNvbSIsImFpZCI6IjAiLCJob3N0Ijoidm1lc3MuZXhhbXBsZS5jb20iLCJpZCI6IjExMTExMTExLTIyMjItMzMzMy00NDQ0LTU1NTU1NTU1NTU1NSIsIm5ldCI6IndzIiwicGF0aCI6Ii93cyIsInBvcnQiOiI0NDMiLCJwcyI6InZtZXNzLW5vZGUiLCJzY3kiOiJhdXRvIiwic24iOiJ2bWVzcy5leGFtcGxlLmNvbSIsInNuaSI6InZtZXNzLmV4YW1wbGUuY29tIiwidGxzIjoidGxzIiwidHlwZSI6Im5vbmUiLCJ2IjoiMiJ9"},
		{"vless-reality", model.NewNodeFromParts("vless-reality", "vless", "vless.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "flow": "xtls-rprx-vision",
			"servername": "www.microsoft.com", "client-fingerprint": "chrome",
			"reality-opts": map[string]any{"public-key": "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY", "short-id": "0123456789abcdef"},
		}), "vless://11111111-2222-3333-4444-555555555555@vless.example.com:443?encryption=none&flow=xtls-rprx-vision&fp=chrome&pbk=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY&security=reality&sid=0123456789abcdef&sni=www.microsoft.com&type=tcp#vless-reality"},
		{"vless-ws", model.NewNodeFromParts("vless-ws", "vless", "vless-ws.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "tls": true,
			"servername": "vless-ws.example.com",
			"ws-opts":    map[string]any{"path": "/ws", "headers": map[string]any{"Host": "vless-ws.example.com"}},
		}), "vless://11111111-2222-3333-4444-555555555555@vless-ws.example.com:443?encryption=none&host=vless-ws.example.com&path=%2Fws&security=tls&sni=vless-ws.example.com&type=ws#vless-ws"},
		{"trojan", model.NewNodeFromParts("trojan-node", "trojan", "trojan.example.com", "443", map[string]any{
			"password": "password123", "sni": "trojan.example.com", "skip-cert-verify": true, "network": "tcp",
		}), "trojan://password123@trojan.example.com:443?allowInsecure=1&sni=trojan.example.com&type=tcp#trojan-node"},
		{"hysteria2", model.NewNodeFromParts("hy2-node", "hysteria2", "hy2.example.com", "443", map[string]any{
			"password": "password123", "sni": "hy2.example.com", "skip-cert-verify": true,
		}), "hysteria2://password123@hy2.example.com:443?insecure=1&sni=hy2.example.com#hy2-node"},
		{"hysteria", model.NewNodeFromParts("hy1-node", "hysteria", "hy1.example.com", "443", map[string]any{
			"auth_str": "password123", "sni": "hy1.example.com", "skip-cert-verify": true,
			"protocol": "udp", "up": "100", "down": "200",
		}), "hysteria://hy1.example.com:443?auth=password123&down=200&insecure=1&peer=hy1.example.com&protocol=udp&up=100#hy1-node"},
		{"tuic", model.NewNodeFromParts("tuic-node", "tuic", "tuic.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "password": "password123",
			"sni": "tuic.example.com", "congestion-controller": "bbr", "udp-relay-mode": "native",
		}), "tuic://11111111-2222-3333-4444-555555555555:password123@tuic.example.com:443?congestion_control=bbr&sni=tuic.example.com&udp_relay_mode=native#tuic-node"},
		{"anytls", model.NewNodeFromParts("anytls-node", "anytls", "anytls.example.com", "443", map[string]any{
			"password": "password123", "sni": "anytls.example.com", "skip-cert-verify": true,
		}), "anytls://password123@anytls.example.com:443?insecure=1&sni=anytls.example.com#anytls-node"},
		{"socks5", model.NewNodeFromParts("socks-node", "socks5", "socks.example.com", "1080", map[string]any{
			"username": "user.name", "password": "pass.word",
		}), "socks5://user.name:pass.word@socks.example.com:1080#socks-node"},
		{"http", model.NewNodeFromParts("http-node", "http", "http.example.com", "8080", map[string]any{
			"username": "user.name", "password": "pass.word",
		}), "http://user.name:pass.word@http.example.com:8080#http-node"},
		{"https", model.NewNodeFromParts("https-node", "https", "https.example.com", "8443", map[string]any{
			"username": "user.name", "password": "pass.word",
		}), "https://user.name:pass.word@https.example.com:8443#https-node"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Render(c.node)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got != c.gold {
				t.Errorf("链接不匹配\n got: %s\nwant: %s", got, c.gold)
			}
		})
	}
}

// --- 往返：关键字段不丢 ---

func TestSharelinkRoundTrip_KeyFields(t *testing.T) {
	cases := []struct {
		name string
		node model.Node
		want map[string]any
	}{
		{"ss", model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}), map[string]any{
			"type": "ss", "name": "ss-node", "server": "ss.example.com", "port": "8388",
			"cipher": "aes-256-gcm", "password": "password123",
		}},
		{"ssr", model.NewNodeFromParts("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password123", "protocol": "origin", "obfs": "plain",
		}), map[string]any{
			"type": "ssr", "name": "ssr-node", "server": "ssr.example.com", "port": "1234",
			"cipher": "aes-256-cfb", "password": "password123", "protocol": "origin", "obfs": "plain",
		}},
		{"vmess", model.NewNodeFromParts("vmess-node", "vmess", "vmess.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "alterId": "0", "cipher": "auto",
			"network": "ws", "tls": true, "servername": "vmess.example.com",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "vmess.example.com"}},
		}), map[string]any{
			"type": "vmess", "name": "vmess-node", "server": "vmess.example.com", "port": "443",
			"uuid": "11111111-2222-3333-4444-555555555555", "alterId": "0", "cipher": "auto",
			"network": "ws", "tls": true, "servername": "vmess.example.com",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "vmess.example.com"}},
		}},
		{"vless-reality", model.NewNodeFromParts("vless-reality", "vless", "vless.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "flow": "xtls-rprx-vision",
			"servername": "www.microsoft.com", "client-fingerprint": "chrome",
			"reality-opts": map[string]any{"public-key": "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY", "short-id": "0123456789abcdef"},
		}), map[string]any{
			"type": "vless", "name": "vless-reality", "server": "vless.example.com", "port": "443",
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp", "flow": "xtls-rprx-vision",
			"servername": "www.microsoft.com", "client-fingerprint": "chrome", "tls": true,
			"reality-opts": map[string]any{"public-key": "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY", "short-id": "0123456789abcdef"},
		}},
		{"vless-ws", model.NewNodeFromParts("vless-ws", "vless", "vless-ws.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "tls": true,
			"servername": "vless-ws.example.com",
			"ws-opts":    map[string]any{"path": "/ws", "headers": map[string]any{"Host": "vless-ws.example.com"}},
		}), map[string]any{
			"type": "vless", "name": "vless-ws", "server": "vless-ws.example.com", "port": "443",
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "ws", "tls": true,
			"servername": "vless-ws.example.com",
			"ws-opts":    map[string]any{"path": "/ws", "headers": map[string]any{"Host": "vless-ws.example.com"}},
		}},
		{"trojan", model.NewNodeFromParts("trojan-node", "trojan", "trojan.example.com", "443", map[string]any{
			"password": "password123", "sni": "trojan.example.com", "skip-cert-verify": true, "network": "tcp",
		}), map[string]any{
			"type": "trojan", "name": "trojan-node", "server": "trojan.example.com", "port": "443",
			"password": "password123", "sni": "trojan.example.com", "skip-cert-verify": true, "network": "tcp",
		}},
		{"hysteria2", model.NewNodeFromParts("hy2-node", "hysteria2", "hy2.example.com", "443", map[string]any{
			"password": "password123", "sni": "hy2.example.com", "skip-cert-verify": true,
		}), map[string]any{
			"type": "hysteria2", "name": "hy2-node", "server": "hy2.example.com", "port": "443",
			"password": "password123", "sni": "hy2.example.com", "skip-cert-verify": true,
		}},
		{"hysteria", model.NewNodeFromParts("hy1-node", "hysteria", "hy1.example.com", "443", map[string]any{
			"auth_str": "password123", "sni": "hy1.example.com", "skip-cert-verify": true,
			"protocol": "udp", "up": "100", "down": "200",
		}), map[string]any{
			"type": "hysteria", "name": "hy1-node", "server": "hy1.example.com", "port": "443",
			"auth_str": "password123", "sni": "hy1.example.com", "skip-cert-verify": true,
			"protocol": "udp", "up": "100", "down": "200",
		}},
		{"tuic", model.NewNodeFromParts("tuic-node", "tuic", "tuic.example.com", "443", map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "password": "password123",
			"sni": "tuic.example.com", "congestion-controller": "bbr", "udp-relay-mode": "native",
		}), map[string]any{
			"type": "tuic", "name": "tuic-node", "server": "tuic.example.com", "port": "443",
			"uuid": "11111111-2222-3333-4444-555555555555", "password": "password123",
			"sni": "tuic.example.com", "congestion-controller": "bbr", "udp-relay-mode": "native",
		}},
		{"anytls", model.NewNodeFromParts("anytls-node", "anytls", "anytls.example.com", "443", map[string]any{
			"password": "password123", "sni": "anytls.example.com", "skip-cert-verify": true,
		}), map[string]any{
			"type": "anytls", "name": "anytls-node", "server": "anytls.example.com", "port": "443",
			"password": "password123", "sni": "anytls.example.com", "skip-cert-verify": true,
		}},
		{"socks5", model.NewNodeFromParts("socks-node", "socks5", "socks.example.com", "1080", map[string]any{
			"username": "user.name", "password": "pass.word",
		}), map[string]any{
			"type": "socks5", "name": "socks-node", "server": "socks.example.com", "port": "1080",
			"username": "user.name", "password": "pass.word",
		}},
		{"http", model.NewNodeFromParts("http-node", "http", "http.example.com", "8080", map[string]any{
			"username": "user.name", "password": "pass.word",
		}), map[string]any{
			"type": "http", "name": "http-node", "server": "http.example.com", "port": "8080",
			"username": "user.name", "password": "pass.word", "skip-cert-verify": true,
		}},
		{"https", model.NewNodeFromParts("https-node", "https", "https.example.com", "8443", map[string]any{
			"username": "user.name", "password": "pass.word",
		}), map[string]any{
			"type": "http", "name": "https-node", "server": "https.example.com", "port": "8443",
			"username": "user.name", "password": "pass.word", "skip-cert-verify": true, "tls": true,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			link, err := Render(c.node)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			reread, err := parse.DecodeSubscription([]byte(link))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if len(reread) != 1 {
				t.Fatalf("期望 1 个节点，实际 %d", len(reread))
			}
			assertSubset(t, reread[0], c.want)
		})
	}
}

// --- 往返：fixture 全量对拍（强） ---

func TestSharelinkRoundTrip_Fixture(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/fixtures/mixed-subscription.txt")
	if err != nil {
		t.Fatal(err)
	}
	nodes := model.NewNodeList(mustParse(t, raw), "fixture")
	out, err := RenderAll(nodes)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if len(out.Skipped) > 0 {
		t.Fatalf("有节点未能序列化: %v", out.Skipped)
	}
	if out.Lines != nodes.Len() {
		t.Fatalf("行数 %d，期望 %d", out.Lines, nodes.Len())
	}
	reread := mustParse(t, out.Body)
	if len(reread) != len(nodes) {
		t.Fatalf("往返后节点数 %d，期望 %d", len(reread), len(nodes))
	}
	for i := range nodes {
		got := normalize(reread[i])
		want := normalize(nodes[i].Raw)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%d] %s 往返不等价", i, nodes[i].Name)
		}
	}
}

// --- base64 混合形态往返 ---

func TestSharelinkMixedRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/fixtures/mixed-subscription.txt")
	if err != nil {
		t.Fatal(err)
	}
	nodes := model.NewNodeList(mustParse(t, raw), "fixture")
	r := New("mixed", Options{Base64: true})
	body, err := r.Render(render.Request{Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Warnings(); len(got) != 0 {
		t.Fatalf("有节点未能序列化: %v", got)
	}
	reread := mustParse(t, body)
	if len(reread) != len(nodes) {
		t.Fatalf("往返后节点数 %d，期望 %d", len(reread), len(nodes))
	}
	for i := range nodes {
		got := normalize(reread[i])
		want := normalize(nodes[i].Raw)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%d] %s 往返不等价", i, nodes[i].Name)
		}
	}
}

// --- Base64 开关两种形态 ---

func TestSharelinkBase64Toggle(t *testing.T) {
	nodes := model.NodeList{ssNode()}
	plainR := New("mixed", Options{Base64: false})
	plain, err := plainR.Render(render.Request{Nodes: nodes})
	if err != nil {
		t.Fatalf("plain Render: %v", err)
	}
	if !strings.Contains(string(plain), "ss://") {
		t.Fatalf("裸形态应含 ss:// 链接: %s", plain)
	}
	b64R := New("mixed", Options{Base64: true})
	enc, err := b64R.Render(render.Request{Nodes: nodes})
	if err != nil {
		t.Fatalf("b64 Render: %v", err)
	}
	dec, err := base64.StdEncoding.DecodeString(string(enc))
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if string(dec) != string(plain)+"\n" {
		t.Errorf("解码后应为裸链接 + 尾换行\n got: %q\nwant: %q", dec, string(plain)+"\n")
	}
	// 两种形态都能被解析器读回来。
	if _, err := parse.DecodeSubscription(plain); err != nil {
		t.Errorf("裸形态解析失败: %v", err)
	}
	if _, err := parse.DecodeSubscription(enc); err != nil {
		t.Errorf("base64 形态解析失败: %v", err)
	}
	// base64 形态不应含裸链接。
	if strings.Contains(string(enc), "ss://") {
		t.Errorf("base64 形态不应含裸链接")
	}
}

// --- RenderAll 行为 ---

func TestSharelinkRenderAll_SkipsUnsupported(t *testing.T) {
	weird := model.Node{
		Name: "w", Raw: map[string]any{"name": "w", "type": "wireguard", "server": "s", "port": "51820"}, Type: "wireguard",
	}
	ok := model.Node{
		Name: "ok", Raw: map[string]any{"name": "ok", "type": "socks5", "server": "s", "port": "1080"}, Type: "socks5",
	}
	out, err := RenderAll(model.NodeList{weird, ok})
	if err != nil {
		t.Fatalf("不支持的协议不应中断整批导出: %v", err)
	}
	if out.Lines != 1 {
		t.Errorf("期望 1 行，实际 %d", out.Lines)
	}
	if len(out.Skipped) != 1 || out.Skipped[0] != "w" {
		t.Errorf("Skipped = %v，应记录被跳过的节点名", out.Skipped)
	}
	if len(out.Body) == 0 {
		t.Errorf("Body 不应为空")
	}
	reread, err := parse.DecodeSubscription(out.Body)
	if err != nil {
		t.Fatalf("产物无法解析: %v", err)
	}
	if len(reread) != 1 {
		t.Errorf("往返节点数 %d，期望 1", len(reread))
	}
}

func TestSharelinkRenderAll_Empty(t *testing.T) {
	out, err := RenderAll(model.NodeList{})
	if err != nil {
		t.Fatalf("空输入不应报错: %v", err)
	}
	if out.Lines != 0 {
		t.Errorf("Lines = %d，期望 0", out.Lines)
	}
	if len(out.Body) != 0 {
		t.Errorf("空输入 Body 应为空: %q", out.Body)
	}
	if len(out.Skipped) != 0 {
		t.Errorf("Skipped 应为空: %v", out.Skipped)
	}
}

func TestSharelinkRender_Unsupported(t *testing.T) {
	weird := model.Node{
		Name: "w", Raw: map[string]any{"name": "w", "type": "wireguard", "server": "s", "port": "51820"}, Type: "wireguard",
	}
	_, err := Render(weird)
	if err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Errorf("应当报不支持，实际: %v", err)
	}
}

// --- Supported ---

func TestSharelinkSupported(t *testing.T) {
	got := Supported()
	if !sort.StringsAreSorted(got) {
		t.Errorf("Supported() 应排序: %v", got)
	}
	want := []string{"anytls", "hysteria", "hysteria2", "http", "https", "shadowsocks", "shadowsocksr", "socks5", "ss", "ssr", "trojan", "tuic", "vless", "vmess"}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("Supported() 长度 %d，期望 %d: %v", len(got), len(want), got)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Supported()[%d] = %q，期望 %q", i, got[i], want[i])
		}
	}
}

// --- Renderer ---

func TestSharelinkRenderer_NameAndWarnings(t *testing.T) {
	r := New("mixed", Options{Base64: false})
	if r.Name() != "mixed" {
		t.Errorf("Name() = %q，期望 mixed", r.Name())
	}
	body, err := r.Render(render.Request{Nodes: model.NodeList{ssNode()}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(body) == 0 {
		t.Errorf("Body 不应为空")
	}
	if w := r.Warnings(); len(w) != 0 {
		t.Errorf("Warnings 应为空: %v", w)
	}
	// 不支持的节点应被记录为警告，且不中断渲染。
	r2 := New("mixed", Options{Base64: false})
	body2, err := r2.Render(render.Request{Nodes: model.NodeList{
		model.Node{Name: "w", Raw: map[string]any{"name": "w", "type": "wireguard", "server": "s", "port": "1"}, Type: "wireguard"},
		ssNode(),
	}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(body2) == 0 {
		t.Errorf("Body 不应为空")
	}
	if len(r2.Warnings()) != 1 {
		t.Errorf("应记录 1 个警告: %v", r2.Warnings())
	}
}

// --- 未导出助手 ---

func TestUrlSafeB64(t *testing.T) {
	if got := urlSafeB64("abc"); strings.Contains(got, "=") {
		t.Errorf("不应含填充: %q", got)
	}
	if got := urlSafeB64("a+b/c"); got != base64.RawURLEncoding.EncodeToString([]byte("a+b/c")) {
		t.Errorf("编码不匹配: %q", got)
	}
}

func TestEscapeFragment(t *testing.T) {
	if got := escapeFragment("a b"); got != "a%20b" {
		t.Errorf("空格应转义: %q", got)
	}
	if got := escapeFragment("🚀节点"); got != "🚀节点" {
		t.Errorf("UTF-8 应保留: %q", got)
	}
	if got := escapeFragment("a/b&c=d+e"); got != "a%2Fb%26c%3Dd%2Be" {
		t.Errorf("结构字符应转义: %q", got)
	}
}

func TestBoolHelpers(t *testing.T) {
	if boolStr(true) != "1" || boolStr(false) != "0" {
		t.Errorf("boolStr 不正确")
	}
	if !isTrue(true) || !isTrue("true") || !isTrue("1") {
		t.Errorf("isTrue 应识别 true/1")
	}
	if isTrue("false") || isTrue(0) || isTrue(nil) {
		t.Errorf("isTrue 不应误判")
	}
}

func TestStrSliceHelpers(t *testing.T) {
	if got := strSlice([]any{"a", 2, nil}); !reflect.DeepEqual(got, []string{"a", "2"}) {
		t.Errorf("strSlice []any: %v", got)
	}
	if got := strSlice([]string{"x", "y"}); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("strSlice []string: %v", got)
	}
	if got := strSlice("z"); !reflect.DeepEqual(got, []string{"z"}) {
		t.Errorf("strSlice string: %v", got)
	}
	if got := strSlice(""); got != nil {
		t.Errorf("空字符串应返回 nil: %v", got)
	}
	if got := strSlice(nil); got != nil {
		t.Errorf("nil 应返回 nil: %v", got)
	}
	if joinStrs([]string{"a", "b"}) != "a,b" {
		t.Errorf("joinStrs 不正确")
	}
	if firstOf([]string{"x", "y"}) != "x" || firstOf(nil) != "" {
		t.Errorf("firstOf 不正确")
	}
}

func TestTransportHostPath(t *testing.T) {
	raw := map[string]any{
		"ws-opts": map[string]any{
			"path":    "/ws",
			"headers": map[string]any{"Host": "ws.example.com"},
		},
	}
	hp := transportHostPath(raw)
	if hp.path != "/ws" || hp.host != "ws.example.com" {
		t.Errorf("ws-opts 解析错误: %+v", hp)
	}
	raw2 := map[string]any{
		"http-opts": map[string]any{
			"path": []any{"/h"},
			"host": []any{"h.example.com"},
		},
	}
	hp2 := transportHostPath(raw2)
	if hp2.path != "/h" || hp2.host != "h.example.com" {
		t.Errorf("http-opts 解析错误: %+v", hp2)
	}
	raw3 := map[string]any{
		"ws-opts":   map[string]any{"path": "/ws"},
		"http-opts": map[string]any{"path": []any{"/h"}},
	}
	hp3 := transportHostPath(raw3)
	if hp3.path != "/ws" {
		t.Errorf("ws-opts 应优先: %+v", hp3)
	}
}

func TestUserinfoString(t *testing.T) {
	if got := userinfoString("u", "p"); got != "u:p" {
		t.Errorf("双侧: %q", got)
	}
	if got := userinfoString("u", ""); got != "u" {
		t.Errorf("仅用户: %q", got)
	}
	if got := userinfoString("", "p"); got != "p" {
		t.Errorf("仅口令: %q", got)
	}
	if got := userinfoString("", ""); got != "" {
		t.Errorf("空: %q", got)
	}
}
