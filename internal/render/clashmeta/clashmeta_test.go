package clashmeta

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/render"
)

func testNodes() model.NodeList {
	return model.NodeList{
		model.NewNodeFromParts("HK-香港-01", "vless", "hk1.example.com", "443", map[string]any{
			"udp":                true,
			"tls":                true,
			"servername":         "hk1.example.com",
			"client-fingerprint": "chrome",
			"ws-opts":            map[string]any{"path": "/ws", "headers": map[string]any{"Host": "hk1.example.com"}},
		}),
		model.NewNodeFromParts("JP-日本-01", "vmess", "jp1.example.com", "8443", map[string]any{
			"udp":     true,
			"tls":     true,
			"network": "tcp",
			"cipher":  "auto",
			"uuid":    "11111111-2222-3333-4444-555555555555",
		}),
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

// renderJSON 与 renderTest 对称，产出 JSON 形态。
func renderJSON(t *testing.T, req render.Request) []byte {
	t.Helper()
	opts := DefaultOptions()
	opts.JSON = true
	body, err := New(opts).Render(req)
	if err != nil {
		t.Fatalf("Render(JSON): %v", err)
	}
	return body
}

// TestRender_Golden 是配置产物的字节级金测（SUBFORGE_UPDATE_GOLDEN=1 可刷新）。
func TestRender_Golden(t *testing.T) {
	got := renderTest(t, render.Request{
		Nodes:   testNodes(),
		Profile: render.Profile{Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z"},
	})

	path := filepath.Join("..", "..", "..", "testdata", "golden", "clash-meta-default.yaml")
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

// TestRender_Golden_JSON 是 JSON 输出的字节级金测。
// JSON 与 YAML 构的是同一棵树，所以金测用同一个 Request。
func TestRender_Golden_JSON(t *testing.T) {
	got := renderJSON(t, render.Request{
		Nodes:   testNodes(),
		Profile: render.Profile{Name: "subforge-test", Version: "0.1.0", GeneratedAt: "2026-10-14T00:00:00Z"},
	})

	path := filepath.Join("..", "..", "..", "testdata", "golden", "clash-meta-default.json")
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

// TestRender_JSONRoundTrip 用标准库把 JSON 产物解析回来，确认字段与类型都在。
// 类型漂移是 JSON 编码最容易犯的错：YAML 里 "443" 是字符串，
// 编码成 443 之后 mihomo 就当数字读，端口字段直接失效。
func TestRender_JSONRoundTrip(t *testing.T) {
	body := renderJSON(t, render.Request{Nodes: testNodes()})
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("产物不是合法 JSON: %v", err)
	}
	proxies, ok := doc["proxies"].([]any)
	if !ok {
		t.Fatalf("proxies 不是数组: %T", doc["proxies"])
	}
	if len(proxies) != 2 {
		t.Fatalf("proxies 数量 = %d, want 2", len(proxies))
	}
	for i, p := range proxies {
		m, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("proxies[%d] 不是对象: %T", i, p)
			continue
		}
		// 端口必须是字符串——mihomo 按 map[string]any 读，数字与字符串都收，
		// 但改成数字就与 YAML 产物不一致，两种格式的描述就不等价了。
		port, isStr := m["port"].(string)
		if !isStr || port != "443" && port != "8443" {
			t.Errorf("proxies[%d].port 应为字符串 443/8443，实际 %T %v", i, m["port"], m["port"])
		}
		for _, f := range []string{"name", "type", "server"} {
			if _, ok := m[f].(string); !ok {
				t.Errorf("proxies[%d].%s 不是字符串: %T", i, f, m[f])
			}
		}
	}
	if _, ok := doc["rules"].([]any); !ok {
		t.Errorf("rules 不是数组: %T", doc["rules"])
	}
	if _, ok := doc["proxy-groups"].([]any); !ok {
		t.Errorf("proxy-groups 不是数组: %T", doc["proxy-groups"])
	}
}

// TestRender_JSONDeterministic JSON 输出同样要字节稳定，否则金测抖动。
func TestRender_JSONDeterministic(t *testing.T) {
	req := render.Request{Nodes: testNodes()}
	a := renderJSON(t, req)
	b := renderJSON(t, req)
	if !bytes.Equal(a, b) {
		t.Fatal("JSON 输出两次渲染不一致（golden 无法工作）")
	}
}

// TestRender_JSONEmptyLists 空列表必须是 [] 而不是 null，客户端对 null 的处理各不相同。
func TestRender_JSONEmptyLists(t *testing.T) {
	body := renderJSON(t, render.Request{Nodes: model.NodeList{}})
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("产物不是合法 JSON: %v", err)
	}
	if _, ok := doc["proxies"].([]any); !ok {
		t.Fatalf("空 proxies 应为 []，实际 %T: %s", doc["proxies"], body)
	}
	if bytes.Contains(body, []byte("null")) {
		t.Errorf("JSON 产物不应出现 null：%s", body)
	}
}

// TestRender_RoundTrip 验证产物可被重新解析，且节点名/类型/服务器/端口与输入一致。
func TestRender_RoundTrip(t *testing.T) {
	body := renderTest(t, render.Request{Nodes: testNodes()})
	raws, err := model.ParseClashProxies(body)
	if err != nil {
		t.Fatalf("产物无法被解析回 proxies: %v", err)
	}
	in := testNodes()
	if len(raws) != len(in) {
		t.Fatalf("节点数 = %d, want %d", len(raws), len(in))
	}
	for i, raw := range raws {
		n := model.NodeFromRaw(raw, "")
		if n.Name != in[i].Name || n.Type != in[i].Type || n.Server != in[i].Server || n.Port != in[i].Port {
			t.Errorf("第 %d 个节点不一致: %+v vs %+v", i, n, in[i])
		}
	}
}

// TestRender_ProxyFieldsPreserved 验证扩展字段（ws-opts、reality-opts 一类嵌套结构）不丢失。
func TestRender_ProxyFieldsPreserved(t *testing.T) {
	body := renderTest(t, render.Request{Nodes: testNodes()})
	text := string(body)
	for _, key := range []string{
		"servername", "client-fingerprint", "ws-opts", "path:", "Host:", "cipher", "uuid",
	} {
		if !strings.Contains(text, key) {
			t.Errorf("产物缺少字段 %q", key)
		}
	}
}

func TestRender_NoNodes(t *testing.T) {
	body := renderTest(t, render.Request{Nodes: model.NodeList{}})
	text := string(body)
	for _, want := range []string{"proxies: []", "MATCH,DIRECT"} {
		if !strings.Contains(text, want) {
			t.Errorf("空节点产物应含 %q，实际:\n%s", want, text)
		}
	}
	// 没有节点时不应生成引用不存在组的规则
	if strings.Contains(text, "漏网之鱼") {
		t.Error("无节点时不应引用自动生成的组")
	}
}

// 用户启用任意一条规则后，规则集合不再为空；此时仍必须补一条兜底 MATCH，
// 否则未匹配流量会落到内核默认 DIRECT。见订阅审计 SF-01。
func TestRender_AppendsCatchAllWhenRulesLackMatch(t *testing.T) {
	body := renderTest(t, render.Request{
		Nodes:  testNodes(),
		Groups: []render.Group{{Name: "pick-1", Type: "select", Proxies: []string{"."}}},
		Rules:  []string{"DOMAIN-SUFFIX,example.test,DIRECT"},
	})
	text := string(body)
	if !strings.Contains(text, "  - MATCH,pick-1") {
		t.Fatalf("缺少兜底 MATCH,pick-1，实际:\n%s", text)
	}
	if got := strings.Count(text, "MATCH,"); got != 1 {
		t.Fatalf("MATCH 条数 = %d，期望 1:\n%s", got, text)
	}
	if !strings.HasSuffix(strings.TrimSpace(text), "MATCH,pick-1") {
		t.Fatalf("兜底 MATCH 必须在最后，实际:\n%s", text)
	}
}

func TestRender_DoesNotDuplicateExistingCatchAll(t *testing.T) {
	body := renderTest(t, render.Request{
		Nodes:  testNodes(),
		Groups: []render.Group{{Name: "pick-1", Type: "select", Proxies: []string{"."}}},
		Rules:  []string{"DOMAIN-SUFFIX,example.test,DIRECT", "MATCH,pick-1"},
	})
	text := string(body)
	if got := strings.Count(text, "MATCH,"); got != 1 {
		t.Fatalf("MATCH 条数 = %d，期望 1:\n%s", got, text)
	}
}

func TestRender_CatchAllPrefersLoneFishGroup(t *testing.T) {
	body := renderTest(t, render.Request{
		Nodes: testNodes(),
		Groups: []render.Group{
			{Name: "pick-1", Type: "select", Proxies: []string{"."}},
			{Name: "🐟 漏网之鱼", Type: "select", Proxies: []string{"pick-1"}},
		},
		Rules: []string{"DOMAIN-SUFFIX,example.test,pick-1"},
	})
	text := string(body)
	if !strings.Contains(text, "MATCH,🐟 漏网之鱼") {
		t.Fatalf("兜底应优先选择漏网之鱼，实际:\n%s", text)
	}
}

func TestRender_CatchAllWithOnlyUrlTestGroup(t *testing.T) {
	body := renderTest(t, render.Request{
		Nodes:  testNodes(),
		Groups: []render.Group{{Name: "auto", Type: "url-test", Proxies: []string{"."}}},
		Rules:  []string{"DOMAIN-SUFFIX,example.test,DIRECT"},
	})
	text := string(body)
	if !strings.Contains(text, "  - MATCH,auto") {
		t.Fatalf("没有 select 组时应退到第一个组，实际:\n%s", text)
	}
}

func TestRender_CustomGroupsAndRules(t *testing.T) {
	body := renderTest(t, render.Request{
		Nodes: testNodes(),
		Groups: []render.Group{
			{Name: "pick-1", Type: "select", Proxies: []string{"HK-香港-01", "JP-日本-01"}},
			{
				Name: "auto", Type: "url-test",
				Proxies: []string{"pick-1"}, Lazy: true,
				Options: render.UrlTestOptions{Interval: 600, Tolerance: 80, Timeout: 10000},
			},
		},
		Rules: []string{"GEOIP,私网,DIRECT", "MATCH,pick-1"},
	})
	text := string(body)
	for _, want := range []string{
		"name: pick-1", "type: select", "type: url-test",
		"interval: 600", "tolerance: 80", "timeout: 10000", "lazy: true",
		"GEOIP,私网,DIRECT", "MATCH,pick-1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("产物缺少 %q", want)
		}
	}
	// 用户显式给组时不应再自动加组
	if strings.Contains(text, "节点选择") {
		t.Error("自定义组时不应再自动生成默认组")
	}
}

// TestRender_RejectsUnsupportedGroupType 是"产物必须能被 mihomo 加载"这一验收条件的
// 渲染期护栏：mihomo 没有 reject/direct/pass 这类分组类型，写进去配置直接加载失败。
func TestRender_RejectsUnsupportedGroupType(t *testing.T) {
	for _, bad := range []string{"reject", "REJECT", "direct", "pass", "relay", "dns-fallback"} {
		_, err := New(DefaultOptions()).Render(render.Request{
			Nodes:  testNodes(),
			Groups: []render.Group{{Name: "bad", Type: bad, Proxies: []string{"HK-香港-01"}}},
		})
		if err == nil {
			t.Errorf("分组类型 %q 应被拒绝", bad)
			continue
		}
		if !strings.Contains(err.Error(), bad) {
			t.Errorf("错误信息应指出非法类型 %q，实际: %v", bad, err)
		}
	}
}

func TestRender_Deterministic(t *testing.T) {
	req := render.Request{Nodes: testNodes(), Profile: render.Profile{Name: "x"}}
	a := renderTest(t, req)
	b := renderTest(t, req)
	if !bytes.Equal(a, b) {
		t.Fatal("相同输入两次渲染结果不一致（golden 无法工作）")
	}
}

// TestRender_Providers provider 定义原样透传，组的 use 一并输出。
func TestRender_Providers(t *testing.T) {
	req := render.Request{
		Nodes: testNodes(),
		Groups: []render.Group{
			{Name: "provider 组", Type: "select", Use: []string{"airport"}},
		},
		Providers: []model.Provider{
			{Name: "airport", Raw: map[string]any{
				"type": "http",
				"url":  "https://example.com/sub",
				"health-check": map[string]any{
					"enable":   true,
					"url":      "https://www.gstatic.com/generate_204",
					"interval": 300,
				},
			}},
		},
	}
	body := renderTest(t, req)
	text := string(body)
	for _, want := range []string{
		"proxy-providers:", "airport:", "type: http",
		"url: https://example.com/sub", "health-check:", "use:", "- airport",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("产物缺少 %q\n%s", want, text)
		}
	}
	// 无 provider 时不应出现该键（保持旧产物字节级不变）。
	plain := string(renderTest(t, render.Request{Nodes: testNodes()}))
	if strings.Contains(plain, "proxy-providers") {
		t.Error("无 provider 时不应输出 proxy-providers")
	}
}

// TestRender_MihomoValid 用 mihomo -t 实机校验产物（外部二进制，环境变量指向才执行）。
// 本机无 Docker，故走 SUBFORGE_MIHOMO=<path> 触发；CI 里同理可选开启。
func TestRender_MihomoValid(t *testing.T) {
	bin := os.Getenv("SUBFORGE_MIHOMO")
	if bin == "" {
		t.Skip("未设置 SUBFORGE_MIHOMO，跳过 mihomo -t 实机校验")
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("找不到 mihomo: %v", err)
	}

	// 走真实订阅 fixture 的全链路：parse -> render -> mihomo，混合协议一并校验。
	buf, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Skipf("读取 fixture 失败: %v", err)
	}
	raws, err := parse.DecodeSubscription(buf)
	if err != nil {
		t.Fatalf("解析 fixture: %v", err)
	}
	req := render.Request{
		Nodes:   model.NewNodeList(raws, "fixture"),
		Profile: render.Profile{Name: "subforge-mihomo-test", Version: "test", GeneratedAt: "2026-10-14T00:00:00Z"},
	}

	// YAML 与 JSON 两种形态都必须能被内核加载——两者描述的是一份配置，
	// 只验一种会漏掉 JSON 编码的类型漂移问题。
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"yaml", renderTest(t, req)},
		{"json", renderJSON(t, req)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext := ".yaml"
			if tc.name == "json" {
				ext = ".json"
			}
			cfg := filepath.Join(t.TempDir(), "subforge"+ext)
			if err := os.WriteFile(cfg, tc.body, 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := filepath.Abs(cfg)
			if err != nil {
				t.Fatal(err)
			}
			// 必须用绝对路径的 -f 指名配置：mihomo 默认找 config.yaml，找不到时会先生成初始
			// 模板再校验那个模板——我们自己的坏配置也会"通过"，等于白测。
			out, err := exec.Command(bin, "-t", "-f", cfg).CombinedOutput()
			if err != nil {
				t.Fatalf("mihomo -t 校验失败:\n%s", out)
			}
			if !strings.Contains(string(out), "test is successful") {
				t.Fatalf("mihomo 未确认通过:\n%s", out)
			}
			t.Logf("mihomo -t 通过: %s", strings.TrimSpace(string(out)))
		})
	}
}

// TestWarnings_RewrittenRuleTargets 覆盖 Warnings() 读取路径：规则目标指向产物里
// 不存在的分组时，渲染期会校正并记入 Warnings（降级不能静默）。
func TestWarnings_RewrittenRuleTargets(t *testing.T) {
	r := New(DefaultOptions())
	if got := r.Warnings(); len(got) != 0 {
		t.Fatalf("未渲染前 Warnings 应为空，实际 %v", got)
	}

	_, err := r.Render(render.Request{
		Nodes:  testNodes(),
		Groups: []render.Group{{Name: "pick-1", Type: "select", Proxies: []string{"HK-香港-01"}}},
		Rules:  []string{"DOMAIN-SUFFIX,no-such-target.test,不存在的组", "MATCH,pick-1"},
	})
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	got := r.Warnings()
	if len(got) == 0 {
		t.Fatal("规则目标被校正后 Warnings 不应为空")
	}
	if joined := strings.Join(got, "\n"); !strings.Contains(joined, "默认分组") {
		t.Errorf("Warnings 应说明落到了默认分组，实际: %v", got)
	}
}

// TestRender_RuleProviders 校验规则源产物的 rule-providers 渲染到顶层映射。
func TestRender_RuleProviders(t *testing.T) {
	body := renderTest(t, render.Request{
		Nodes: testNodes(),
		Rules: []string{"RULE-SET,ads,DIRECT", "MATCH,DIRECT"},
		RuleProviders: []model.RuleProvider{
			{Name: "ads", Raw: map[string]any{"type": "http", "url": "https://example.com/ads.yaml", "interval": 3600}},
		},
	})
	s := string(body)
	if !strings.Contains(s, "rule-providers:\n") {
		t.Fatalf("缺少 rule-providers: %s", s)
	}
	if !strings.Contains(s, "\n  ads:\n") || !strings.Contains(s, "url: https://example.com/ads.yaml") {
		t.Fatalf("provider 定义未渲染: %s", s)
	}
}
