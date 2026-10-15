package parse

import (
	"testing"
)

func TestSniff(t *testing.T) {
	t.Run("明文链接", func(t *testing.T) {
		res, err := Sniff(readFixture(t, "mixed-subscription.txt"))
		if err != nil {
			t.Fatalf("Sniff: %v", err)
		}
		if res.Format != FormatLinks || res.Encoded {
			t.Fatalf("format=%v encoded=%v", res.Format, res.Encoded)
		}
	})
	t.Run("整体Base64", func(t *testing.T) {
		res, err := Sniff(readFixture(t, "mixed-subscription.b64"))
		if err != nil {
			t.Fatalf("Sniff: %v", err)
		}
		if res.Format != FormatLinks || !res.Encoded {
			t.Fatalf("format=%v encoded=%v", res.Format, res.Encoded)
		}
		if len(res.Decoded) == 0 {
			t.Error("Decoded 不应为空")
		}
	})
	t.Run("ClashYAML", func(t *testing.T) {
		res, err := Sniff([]byte("mixed-port: 7890\nproxies:\n  - name: n1\n    type: ss\n"))
		if err != nil {
			t.Fatalf("Sniff: %v", err)
		}
		if res.Format != FormatClashYAML || res.Encoded {
			t.Fatalf("format=%v encoded=%v", res.Format, res.Encoded)
		}
	})
	t.Run("空内容", func(t *testing.T) {
		if _, err := Sniff([]byte("   \n\n")); err == nil {
			t.Error("空白内容应报错")
		}
	})
	t.Run("垃圾内容", func(t *testing.T) {
		if _, err := Sniff([]byte("hello world\nthis is not a subscription")); err == nil {
			t.Error("垃圾内容应报错")
		}
	})
}

func TestDecodeSubscription_AllForms(t *testing.T) {
	want, err := ConvertSubscription(readFixture(t, "mixed-subscription.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []string{"mixed-subscription.txt", "mixed-subscription.b64"} {
		t.Run(fixture, func(t *testing.T) {
			got, err := DecodeSubscription(readFixture(t, fixture))
			if err != nil {
				t.Fatalf("DecodeSubscription: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("节点数 = %d, want %d", len(got), len(want))
			}
			for i := range got {
				if NameOf(got[i]) != NameOf(want[i]) {
					t.Errorf("第 %d 个节点名不同: %q vs %q", i, NameOf(got[i]), NameOf(want[i]))
				}
			}
		})
	}
}

func TestDecodeSubscription_ClashYAML(t *testing.T) {
	yaml := []byte("mixed-port: 7890\nproxies:\n" +
		"  - name: ss-one\n    type: ss\n    server: a.example.com\n    port: \"8388\"\n    cipher: aes-256-gcm\n    password: p1\n" +
		"  - name: vmess-one\n    type: vmess\n    server: b.example.com\n    port: \"443\"\n    uuid: 11111111-2222-3333-4444-555555555555\n")
	nodes, err := DecodeSubscription(yaml)
	if err != nil {
		t.Fatalf("DecodeSubscription: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("节点数 = %d, want 2", len(nodes))
	}
	if NameOf(nodes[0]) != "ss-one" || NameOf(nodes[1]) != "vmess-one" {
		t.Fatalf("节点名: %q %q", NameOf(nodes[0]), NameOf(nodes[1]))
	}
	if nodes[0]["type"] != "ss" || nodes[1]["server"] != "b.example.com" {
		t.Fatalf("字段未保留: %v / %v", nodes[0], nodes[1])
	}
}

func TestDecodeSubscription_Invalid(t *testing.T) {
	for _, input := range [][]byte{
		[]byte(""),
		[]byte("not a subscription at all"),
		[]byte("proxies:\n  - broken: [unclosed"),
	} {
		if _, err := DecodeSubscription(input); err == nil {
			t.Errorf("输入 %q 应报错", input)
		}
	}
}
