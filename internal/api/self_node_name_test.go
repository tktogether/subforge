package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestApplySelfNodeName(t *testing.T) {
	cases := []struct {
		desc string
		link string
		name string
		want string
	}{
		{"URI 无 fragment 时追加", "socks5://u:p@example.com:1080", "SOCKS5-测试", "socks5://u:p@example.com:1080#SOCKS5-%E6%B5%8B%E8%AF%95"},
		{"URI 已有 fragment 时替换", "vless://id@example.com:443?security=tls#old", "VLESS-测试", "vless://id@example.com:443?security=tls#VLESS-%E6%B5%8B%E8%AF%95"},
		{"名称里的空格编码", "ss://x@example.com:1", "A B", "ss://x@example.com:1#A%20B"},
		{"空名称保留原链接", "trojan://p@example.com:443#orig", "  ", "trojan://p@example.com:443#orig"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := applySelfNodeName(c.link, c.name)
			if got != c.want {
				t.Fatalf("applySelfNodeName(%q) = %q，应为 %q", c.link, got, c.want)
			}
		})
	}
}

func TestApplySelfNodeNameVMess(t *testing.T) {
	raw := `{"v":"2","ps":"","add":"example.com","port":443,"id":"11111111-2222-3333-4444-555555555555","aid":0,"net":"tcp","type":"none"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(raw))
	got := applySelfNodeName(link, "VMESS-测试")
	if !strings.HasPrefix(got, "vmess://") {
		t.Fatalf("结果不是 vmess 链接: %q", got)
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "vmess://"))
	if err != nil {
		t.Fatalf("Base64 解码: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("JSON 解码: %v", err)
	}
	if obj["ps"] != "VMESS-测试" {
		t.Fatalf("ps = %v，应为 VMESS-测试", obj["ps"])
	}
	if obj["add"] != "example.com" || obj["port"] != float64(443) {
		t.Fatalf("其它字段被改动: %+v", obj)
	}
}

func TestSelfNodeNameAppearsInSubscription(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := t.Context()
	vmessRaw := `{"v":"2","ps":"","add":"198.51.100.10","port":443,"id":"11111111-2222-3333-4444-555555555555","aid":0,"net":"tcp","type":"none"}`
	vmessLink := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessRaw))
	if _, err := mgr.AddSelfNode(ctx, "socks5://u:p@198.51.100.11:1080", "SOCKS5-测试", true, 1); err != nil {
		t.Fatalf("AddSelfNode socks5: %v", err)
	}
	if _, err := mgr.AddSelfNode(ctx, vmessLink, "VMESS-测试", true, 2); err != nil {
		t.Fatalf("AddSelfNode vmess: %v", err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/tokens", `{"token":"t-selfname","name":"n"}`)
	if status != http.StatusCreated {
		t.Fatalf("创建令牌状态码 = %d，应为 201；body=%s", status, body)
	}
	st, sub := subscribe(t, ts, "t-selfname")
	if st != http.StatusOK {
		t.Fatalf("订阅状态码 = %d，应为 200；body=%s", st, sub)
	}
	for _, want := range []string{"SOCKS5-测试", "VMESS-测试"} {
		if !strings.Contains(sub, want) {
			t.Fatalf("订阅里缺少自建节点名称 %q\n%s", want, sub)
		}
	}
	if strings.Contains(sub, `name: ""`) {
		t.Fatalf("订阅里仍有空名称节点\n%s", sub)
	}
}
