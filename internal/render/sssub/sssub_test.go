package sssub

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// 期望值取自 subconverter v0.9.0 实机输出（tools/scv/goldens2/sssub.out，
// GPL-3.0 仅作参照）：字段名、字段顺序、应用级默认值都是客户端解析的硬约束。

func testNodes() model.NodeList {
	return model.NodeList{
		model.NewNodeFromParts("ss-node", "ss", "ss.example.com", "8388", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}),
		model.NewNodeFromParts("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-cfb", "password": "password1234",
			"protocol": "origin", "obfs": "plain",
		}),
		model.NewNodeFromParts("vmess-node", "vmess", "vmess.example.com", "443", map[string]any{
			"udp": true, "tls": true, "cipher": "auto", "uuid": "u-1234",
			"network": "tcp",
		}),
	}
}

func renderTest(t *testing.T, opts Options) []byte {
	t.Helper()
	body, err := New(opts).Render(renderRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return body
}

func renderRequest() render.Request {
	return render.Request{
		Nodes: testNodes(),
		Profile: render.Profile{
			Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z",
		},
	}
}

// TestRender_Golden 是产物的字节级金测（SUBFORGE_UPDATE_GOLDEN=1 可刷新）。
func TestRender_Golden(t *testing.T) {
	got := renderTest(t, DefaultOptions())

	path := filepath.Join("..", "..", "..", "testdata", "golden", "ss-sub.json")
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
}

// TestRender_Base64 base64 形态是 SS-android 的订阅入口格式，
// 内容必须与裸 JSON 完全一致，只是整体编码。
func TestRender_Base64(t *testing.T) {
	plain := renderTest(t, DefaultOptions())
	encoded := renderTest(t, Options{Base64: true, Route: DefaultOptions().Route, RemoteDNS: DefaultOptions().RemoteDNS})

	dec, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Errorf("base64 解码后的内容与裸 JSON 不一致")
	}
}

// TestRender_FieldOrder 字段顺序必须固定：SS-android 反序列化不挑顺序，
// 但 golden 要字节级稳定，所以顺序本身就是契约。
func TestRender_FieldOrder(t *testing.T) {
	body := string(renderTest(t, DefaultOptions()))
	wants := []string{
		`"route":"bypass-lan-china"`,
		`"remote_dns":"dns.google"`,
		`"ipv6":false`,
		`"metered":false`,
		`"proxy_apps":`,
		`"udpdns":false`,
		`"remarks":"ss-node"`,
		`"server":"ss.example.com"`,
		`"server_port":8388`,
		`"method":"aes-256-gcm"`,
		`"password":"password123"`,
		`"plugin":""`,
		`"plugin_opts":""`,
	}
	pos := 0
	for _, w := range wants {
		i := strings.Index(body[pos:], w)
		if i < 0 {
			t.Fatalf("找不到字段 %q", w)
		}
		pos += i + len(w)
	}
}

// TestRender_SkipUnsupported SS-android 只支持 SS 与 SSR；
// vmess 等协议必须跳过并告警，不能产出字段语义不对的节点。
func TestRender_SkipUnsupported(t *testing.T) {
	r := New(DefaultOptions())
	_, err := r.Render(renderRequest())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	found := false
	for _, w := range r.Warnings() {
		if strings.Contains(w, "vmess-node") {
			found = true
		}
	}
	if !found {
		t.Errorf("vmess 节点被跳过但没有告警：%v", r.Warnings())
	}
	body := renderTest(t, DefaultOptions())
	if strings.Contains(string(body), "vmess.example.com") {
		t.Error("产物里不应出现 vmess 节点")
	}
}

// TestRender_NoNodes 没有可导出的节点时报错，不产出空数组。
func TestRender_NoNodes(t *testing.T) {
	req := render.Request{
		Nodes: model.NodeList{
			model.NewNodeFromParts("vmess-node", "vmess", "vmess.example.com", "443",
				map[string]any{"uuid": "u", "network": "tcp"}),
		},
	}
	if _, err := New(DefaultOptions()).Render(req); err == nil {
		t.Error("全部节点都无法表达时应当报错")
	}
}

// TestRender_Deterministic 相同输入必须产出相同字节，否则 golden 无法工作。
func TestRender_Deterministic(t *testing.T) {
	a := renderTest(t, DefaultOptions())
	b := renderTest(t, DefaultOptions())
	if !bytes.Equal(a, b) {
		t.Error("相同输入两次渲染结果不一致")
	}
}

// TestRender_Name 渲染器名不随 base64 开关变化：X-Convert-Format 与文件名都从它派生。
func TestRender_Name(t *testing.T) {
	if got := New(DefaultOptions()).Name(); got != "ss-sub" {
		t.Errorf("Name() = %q", got)
	}
	if got := New(Options{Base64: true}).Name(); got != "ss-sub" {
		t.Errorf("Base64 形态的 Name() = %q，应为 ss-sub", got)
	}
}
