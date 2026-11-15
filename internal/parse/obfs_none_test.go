// obfs=none 归一化测试：hysteria / hysteria2 分享链接里的 `obfs=none` 表示
// 不做混淆（URI 规范语义），但 mihomo 内核转换器会原样拷贝该参数，
// 内核 outbound 构造时又要求非空 obfs 必须配套 obfs-password，导致 400。
// 解析边界必须把 none 剥掉，其余 obfs 值原样保留。
package parse_test

import (
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/render/sharelink"
)

// userHy2Link 是用户实测被 400 拒收的原始链接（obfs=none，无 obfs-password）。
const userHy2Link = "hysteria2://8a43d222-8f68-451b-9e4d-1a3988737fd7@45.125.32.103:24795/?sni=www.apple.com&insecure=1&pinSHA256=0e7199a08ca115856955ff9dd6aa4e4c64f88e3de72f8c420152fc8c7dddb9a5&alpn=h3&obfs=none#YOO_HK-hy2"

// assertNoObfs 断言节点 map 里没有 obfs / obfs-password 残留。
func assertNoObfs(t *testing.T, node map[string]any) {
	t.Helper()
	for _, key := range []string{"obfs", "obfs-password"} {
		if v, ok := node[key]; ok {
			t.Errorf("节点不应残留 %q 键，实际 = %v（map: %v）", key, v, node)
		}
	}
}

func TestConvertSubscription_ObfsNone(t *testing.T) {
	cases := []struct {
		desc     string
		link     string
		wantObfs bool // true = obfs 应原样保留（非 none 值）
	}{
		{desc: "用户实测 obfs=none 链接", link: userHy2Link, wantObfs: false},
		{desc: "obfs 大写 NONE 同样归一", link: "hysteria2://pass@198.51.100.10:443/?sni=example.com&obfs=NONE#hy2-NONE", wantObfs: false},
		{desc: "obfs=salamander 原样保留", link: "hysteria2://pass@198.51.100.10:443/?sni=example.com&obfs=salamander&obfs-password=pw123#hy2-sal", wantObfs: true},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			nodes, err := parse.ConvertSubscription([]byte(c.link))
			if err != nil {
				t.Fatalf("ConvertSubscription: %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("节点数 = %d，应为 1", len(nodes))
			}
			if err := parse.Validate(nodes[0]); err != nil {
				t.Fatalf("内核校验失败: %v", err)
			}
			if c.wantObfs {
				if nodes[0]["obfs"] == "" {
					t.Errorf("obfs 应原样保留，实际为空: %v", nodes[0])
				}
				return
			}
			assertNoObfs(t, nodes[0])
		})
	}
}

func TestConvertSubscription_ObfsNonePreservesName(t *testing.T) {
	nodes, err := parse.ConvertSubscription([]byte(userHy2Link))
	if err != nil {
		t.Fatalf("ConvertSubscription: %v", err)
	}
	if got := parse.NameOf(nodes[0]); got != "YOO_HK-hy2" {
		t.Fatalf("fragment 名字应保留，实际 = %q", got)
	}
}

func TestDecodeSource_LinksObfsNone(t *testing.T) {
	// 拉取链路（fetch → DecodeSource）必须与 validateSelfNode 用的 ConvertSubscription 行为一致。
	d, err := parse.DecodeSource([]byte(userHy2Link))
	if err != nil {
		t.Fatalf("DecodeSource: %v", err)
	}
	if len(d.Nodes) != 1 {
		t.Fatalf("节点数 = %d，应为 1", len(d.Nodes))
	}
	assertNoObfs(t, d.Nodes[0])
	if err := parse.Validate(d.Nodes[0]); err != nil {
		t.Fatalf("内核校验失败: %v", err)
	}
}

func TestDecodeSource_ClashYAMLObfsNone(t *testing.T) {
	// 用户手写的 Clash YAML 也可能带 obfs: none，下游同样会被内核拒。
	yamlBuf := []byte(`proxies:
  - name: hy2-yaml-none
    type: hysteria2
    server: 45.125.32.103
    port: 24795
    password: 8a43d222-8f68-451b-9e4d-1a3988737fd7
    sni: www.apple.com
    obfs: none
  - name: hy2-yaml-salamander
    type: hysteria2
    server: 45.125.32.103
    port: 24795
    password: 8a43d222-8f68-451b-9e4d-1a3988737fd7
    obfs: salamander
    obfs-password: pw123
`)
	d, err := parse.DecodeSource(yamlBuf)
	if err != nil {
		t.Fatalf("DecodeSource: %v", err)
	}
	if len(d.Nodes) != 2 {
		t.Fatalf("节点数 = %d，应为 2", len(d.Nodes))
	}
	assertNoObfs(t, d.Nodes[0])
	if err := parse.Validate(d.Nodes[0]); err != nil {
		t.Fatalf("内核校验失败（obfs: none 节点）: %v", err)
	}
	// salamander 节点不受影响。
	if d.Nodes[1]["obfs"] != "salamander" || d.Nodes[1]["obfs-password"] != "pw123" {
		t.Errorf("salamander 节点的 obfs 字段应原样保留: %v", d.Nodes[1])
	}
	if err := parse.Validate(d.Nodes[1]); err != nil {
		t.Fatalf("内核校验失败（salamander 节点）: %v", err)
	}
}

// TestValidate_ObfsWithoutPasswordStillFails 是回归护栏：只修 none，
// 不放宽内核其它语义——salamander 缺密码仍然必须报错。
func TestValidate_ObfsWithoutPasswordStillFails(t *testing.T) {
	m := map[string]any{
		"name": "hy2-broken", "type": "hysteria2",
		"server": "198.51.100.10", "port": 443,
		"password": "pass", "obfs": "salamander",
	}
	err := parse.Validate(m)
	if err == nil {
		t.Fatal("obfs=salamander 且无密码应被内核拒绝，实际通过")
	}
	if !strings.Contains(err.Error(), "missing obfs password") {
		t.Fatalf("错误应来自内核 missing obfs password，实际: %v", err)
	}
}

// TestObfsNone_RendersClean 渲染器不得再输出 obfs 残迹：
// clashmeta 透传原始 map，singbox / sharelink 按字段组装——归一化后三者都应干净。
func TestObfsNone_RendersClean(t *testing.T) {
	nodes, err := parse.ConvertSubscription([]byte(userHy2Link))
	if err != nil {
		t.Fatalf("ConvertSubscription: %v", err)
	}
	list := model.NewNodeList(nodes, "t")

	t.Run("clashmeta", func(t *testing.T) {
		r, err := registry.Lookup("clash.meta", clashmeta.Options{})
		if err != nil {
			t.Fatalf("Lookup: %v", err)
		}
		body, err := r.Render(render.Request{Nodes: list})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(string(body), "obfs") {
			t.Errorf("clashmeta 产物不应含 obfs:\n%s", body)
		}
	})

	t.Run("singbox", func(t *testing.T) {
		r, err := registry.Lookup("sing-box", clashmeta.Options{})
		if err != nil {
			t.Fatalf("Lookup: %v", err)
		}
		body, err := r.Render(render.Request{Nodes: list})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if strings.Contains(string(body), "obfs") {
			t.Errorf("singbox 产物不应含 obfs:\n%s", body)
		}
	})

	t.Run("sharelink", func(t *testing.T) {
		out, err := sharelink.RenderAll(list)
		if err != nil {
			t.Fatalf("RenderAll: %v", err)
		}
		if len(out.Skipped) > 0 {
			t.Fatalf("节点不应被跳过: %v", out.Skipped)
		}
		if strings.Contains(string(out.Body), "obfs") {
			t.Errorf("sharelink 产物不应含 obfs:\n%s", out.Body)
		}
		// 产物还必须能被解析回来（订阅产物是给客户端吃的）。
		// 注意：内核转换器对 hysteria2 无条件写入 obfs = query.Get("obfs")，
		// 链接里没有该参数时也会带上空串键——空值对内核无害，这里只断言
		// 不会重新出现非空 obfs（即 none 不会在往返中被重新引入）。
		reread, err := parse.ConvertSubscription(out.Body)
		if err != nil {
			t.Fatalf("产物回读失败: %v", err)
		}
		for _, key := range []string{"obfs", "obfs-password"} {
			if v, _ := reread[0][key].(string); v != "" {
				t.Errorf("往返后 %q 应为空，实际 = %q", key, v)
			}
		}
		if err := parse.Validate(reread[0]); err != nil {
			t.Fatalf("往返产物内核校验失败: %v", err)
		}
	})
}
