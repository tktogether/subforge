package model

import (
	"reflect"
	"testing"
)

func TestMapString(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		want     string
		nonempty bool
	}{
		{name: "字符串", m: map[string]any{"port": "443"}, key: "port", want: "443"},
		{name: "int", m: map[string]any{"port": 443}, key: "port", want: "443"},
		{name: "float 整数", m: map[string]any{"port": 8080.0}, key: "port", want: "8080"},
		{name: "float 小数", m: map[string]any{"ratio": 0.5}, key: "ratio", want: "0.5"},
		{name: "缺失", m: map[string]any{}, key: "port"},
		{name: "nil", m: map[string]any{"port": nil}, key: "port"},
		{name: "布尔", m: map[string]any{"skip-cert-verify": true}, key: "skip-cert-verify", want: "true"},
		{name: "嵌套 map", m: map[string]any{"reality-opts": map[string]any{"public-key": "x"}}, key: "reality-opts", nonempty: true},
		{name: "嵌套 slice", m: map[string]any{"ip": []string{"1.1.1.1"}}, key: "ip", nonempty: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MapString(tc.m, tc.key)
			if tc.nonempty {
				// 复合值：fmt.Sprint 的呈现形式不作断言，只要求非空（不 panic）
				if got == "" {
					t.Fatalf("MapString(%v,%q) 不应为空", tc.m, tc.key)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("MapString(%v,%q) = %q, want %q", tc.m, tc.key, got, tc.want)
			}
		})
	}
}

func TestNodeFromRaw(t *testing.T) {
	n := NodeFromRaw(map[string]any{
		"name":   "vmess-node",
		"type":   "vmess",
		"server": "vmess.example.com",
		"port":   "443",
		"uuid":   "11111111-2222-3333-4444-555555555555",
	}, "airportA")
	if n.Name != "vmess-node" || n.Type != "vmess" ||
		n.Server != "vmess.example.com" || n.Port != "443" || n.Source != "airportA" {
		t.Fatalf("派生字段错误: %+v", n)
	}
	if got, err := n.PortInt(); got != 443 || err != nil {
		t.Fatalf("PortInt = %d, err = %v", got, err)
	}
	// Raw 是唯一事实源：改 Raw 后重派生即可反映新值
	n.Raw["name"] = "renamed"
	if got := NodeFromRaw(n.Raw, n.Source).Name; got != "renamed" {
		t.Fatalf("重派生 Name = %q", got)
	}
}

func TestPortInt(t *testing.T) {
	cases := []struct {
		port    string
		want    int
		wantErr bool
	}{
		{port: "443", want: 443},
		{port: " 1080 ", want: 1080},
		{port: "", wantErr: true},
		{port: "80:01", wantErr: true},
		{port: "abc", wantErr: true},
	}
	for _, tc := range cases {
		n := NodeFromRaw(map[string]any{"name": "x", "port": tc.port}, "")
		got, err := n.PortInt()
		if (err != nil) != tc.wantErr {
			t.Errorf("port=%q: err=%v, wantErr=%v", tc.port, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("port=%q: got=%d, want=%d", tc.port, got, tc.want)
		}
	}
}

func TestFingerprintAndDedupe(t *testing.T) {
	same := NewNodeFromParts("vmess-a", "vmess", "vmess.example.com", "443", map[string]any{"uuid": "u1"})
	renamed := NewNodeFromParts("vmess-a-改名", "vmess", "vmess.example.com", "443", map[string]any{"uuid": "u1"})
	different := NewNodeFromParts("vmess-b", "vmess", "vmess.example.com", "443", map[string]any{"uuid": "u2"})
	otherPort := NewNodeFromParts("vmess-c", "vmess", "vmess.example.com", "8443", map[string]any{"uuid": "u1"})

	if same.Fingerprint() != renamed.Fingerprint() {
		t.Error("同凭据但显示名不同 → 指纹应相同")
	}
	if same.Fingerprint() == different.Fingerprint() {
		t.Error("uuid 不同 → 指纹应不同")
	}
	if same.Fingerprint() == otherPort.Fingerprint() {
		t.Error("端口不同 → 指纹应不同")
	}

	ns := NodeList{same, renamed, different, otherPort}
	deduped := ns.Dedupe()
	if len(deduped) != 3 {
		t.Fatalf("Dedupe = %d, want 3", len(deduped))
	}
	if deduped[0].Name != "vmess-a" {
		t.Fatalf("Dedupe 应保留首次出现: %s", deduped[0].Name)
	}
}

func TestNodeListHelpers(t *testing.T) {
	ns := NewNodeList([]map[string]any{
		{"name": "a", "type": "vmess", "server": "s1", "port": "443"},
		{"name": "b", "type": "ss", "server": "s2", "port": "8388"},
		{"name": "c", "type": "vmess", "server": "s3", "port": "443"},
	}, "airportA")

	if ns.Len() != 3 || ns.IsEmpty() {
		t.Fatalf("Len/IsEmpty = %d/%v", ns.Len(), ns.IsEmpty())
	}
	if got := ns.Names(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Names = %v", got)
	}
	if got := ns.DistinctTypes(); !reflect.DeepEqual(got, []string{"ss", "vmess"}) {
		t.Fatalf("DistinctTypes = %v", got)
	}
	if got := ns.Filter(func(n Node) bool { return n.Type == "vmess" }); len(got) != 2 {
		t.Fatalf("Filter(vmess) = %d, want 2", len(got))
	}
	if got := ns.Filter(func(n Node) bool { return n.Server == "s2" }); len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("Filter(server=s2) = %v", got)
	}
	if got := ns.Raws(); got[1]["server"] != "s2" {
		t.Fatalf("Raws[1] = %v", got[1])
	}
	if !NodeList(nil).IsEmpty() {
		t.Error("nil NodeList 应为空")
	}
	if got := NodeList(nil).DistinctTypes(); len(got) != 0 {
		t.Fatalf("空 NodeList 的 DistinctTypes = %v", got)
	}
}

// TestParseClashProviders provider 定义原样提取，名字按字典序固定，扩展字段不丢。
func TestParseClashProviders(t *testing.T) {
	yaml := []byte(`mixed-port: 7890
proxy-providers:
  zeta:
    type: http
    url: https://z.example.com/sub
  alpha:
    type: http
    url: https://a.example.com/sub
    interval: 3600
    health-check:
      enable: true
      url: https://www.gstatic.com/generate_204
proxies:
  - name: n1
    type: ss
    server: a.example.com
    port: "8388"
`)
	got, err := ParseClashProviders(yaml)
	if err != nil {
		t.Fatalf("ParseClashProviders: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("provider 数 = %d, want 2", len(got))
	}
	if got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Fatalf("顺序应字典序: %q %q", got[0].Name, got[1].Name)
	}
	if got[0].Raw["url"] != "https://a.example.com/sub" {
		t.Errorf("url 未保留: %v", got[0].Raw["url"])
	}
	if got[0].Raw["interval"] != 3600 {
		t.Errorf("interval 未保留: %v (%T)", got[0].Raw["interval"], got[0].Raw["interval"])
	}
	if _, ok := got[0].Raw["health-check"].(map[string]any); !ok {
		t.Errorf("health-check 嵌套字段未保留: %T", got[0].Raw["health-check"])
	}
}

// TestParseClashProviders_None 没有 provider 段时返回 nil，不是错误。
func TestParseClashProviders_None(t *testing.T) {
	got, err := ParseClashProviders([]byte("proxies:\n  - name: n1\n    type: ss\n"))
	if err != nil {
		t.Fatalf("无 provider 不应报错: %v", err)
	}
	if got != nil {
		t.Fatalf("无 provider 应返回 nil, got %v", got)
	}
}
