package parse

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("读取 fixture: %v", err)
	}
	return buf
}

// TestMixedSubscription_ParseAndValidate 是 I1 spike 的核心验证：
// 混合订阅（12 种协议链接）→ ConvertsV2Ray → 逐节点 ParseProxy 校验。
func TestMixedSubscription_ParseAndValidate(t *testing.T) {
	for _, fixture := range []string{"mixed-subscription.txt", "mixed-subscription.b64"} {
		t.Run(fixture, func(t *testing.T) {
			nodes, err := ConvertSubscription(readFixture(t, fixture))
			if err != nil {
				t.Fatalf("ConvertSubscription: %v", err)
			}
			if len(nodes) < 12 {
				t.Fatalf("节点数 = %d，期望 >= 12（fixture 有 12 条链接）", len(nodes))
			}

			types := make(map[string]int)
			for i, n := range nodes {
				name, _ := n["name"].(string)
				typ, _ := n["type"].(string)
				types[typ]++
				t.Logf("[%02d] type=%-9s name=%-14s keys=%v", i, typ, name, sortedKeys(n))

				if name == "" {
					t.Errorf("[%d] 缺 name: %v", i, n)
				}
				if typ == "" {
					t.Errorf("[%d] 缺 type: %v", i, n)
				}
				if _, ok := n["server"]; !ok {
					t.Errorf("[%d] (%s) 缺 server: %v", i, typ, n)
				}
				if _, ok := n["port"]; !ok {
					t.Errorf("[%d] (%s) 缺 port: %v", i, typ, n)
				}
				if err := Validate(n); err != nil {
					t.Errorf("内核校验失败: %v", err)
				}
			}
			t.Logf("协议分布: %v", types)
		})
	}
}

// TestValidateDoesNotMutateMapping 验证 ParseProxy 不会修改输入 map
// （事实源不可被校验过程污染，否则渲染结果会与解析产物不一致）。
func TestValidateDoesNotMutateMapping(t *testing.T) {
	nodes, err := ConvertSubscription(readFixture(t, "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("ConvertSubscription: %v", err)
	}
	for i, n := range nodes {
		before, err := json.Marshal(n)
		if err != nil {
			t.Fatalf("[%d] marshal: %v", i, err)
		}
		if err := Validate(n); err != nil {
			t.Errorf("[%d] Validate: %v", i, err)
		}
		after, err := json.Marshal(n)
		if err != nil {
			t.Fatalf("[%d] marshal: %v", i, err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("[%d] Validate 修改了输入 map:\n before=%s\n after =%s", i, before, after)
		}
	}
}

// TestConvertEmptyAndGarbage 验证异常输入的行为（不 panic、可辨认的错误）。
func TestConvertEmptyAndGarbage(t *testing.T) {
	if _, err := ConvertSubscription([]byte("")); err == nil {
		t.Error("空输入应该报错")
	}
	if _, err := ConvertSubscription([]byte("this is not a subscription\nneither is this")); err == nil {
		t.Error("垃圾输入应该报错")
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 直接冒泡太丑；这里只是日志用，简单插入排序即可
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// BenchmarkConvert1000 仅转换 1000 节点订阅（对应设计书 §8 的转换吞吐预算分摊）。
func BenchmarkConvert1000(b *testing.B) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		b.Fatal(err)
	}
	buf := bytes.Repeat(fixture, 84) // 12 * 84 = 1008 个节点
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes, err := ConvertSubscription(buf)
		if err != nil {
			b.Fatal(err)
		}
		if len(nodes) < 1000 {
			b.Fatalf("节点数 = %d", len(nodes))
		}
	}
}

// BenchmarkConvertAndValidate1000 转换 + 逐节点内核校验（I1 阶段的全链路成本上限）。
func BenchmarkConvertAndValidate1000(b *testing.B) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		b.Fatal(err)
	}
	buf := bytes.Repeat(fixture, 84)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes, err := ConvertSubscription(buf)
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range nodes {
			if err := Validate(n); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// TestDecodeSource_RuleProviders Clash YAML 来源同时解出节点与 rule-providers。
func TestDecodeSource_RuleProviders(t *testing.T) {
	yaml := []byte("rule-providers:\n  myrule:\n    type: http\n    behavior: domain\n    url: https://example.com/rule.yaml\nproxies:\n  - name: n1\n    type: ss\n    server: a.example.com\n    port: \"8388\"\n    cipher: aes-256-gcm\n    password: p1\n")
	d, err := DecodeSource(yaml)
	if err != nil {
		t.Fatalf("DecodeSource: %v", err)
	}
	if len(d.Nodes) != 1 {
		t.Fatalf("节点数 = %d, want 1", len(d.Nodes))
	}
	if len(d.RuleProviders) != 1 || d.RuleProviders[0].Name != "myrule" {
		t.Fatalf("ruleprovider = %+v", d.RuleProviders)
	}
	if d.RuleProviders[0].Raw["url"] != "https://example.com/rule.yaml" {
		t.Errorf("ruleprovider url 未保留: %v", d.RuleProviders[0].Raw["url"])
	}
	if len(d.Warnings) != 0 {
		t.Errorf("Warnings = %v, want empty", d.Warnings)
	}
}

// TestDecodeSource_LinksNoRuleProviders 分享链接来源没有 rule-provider 概念。
func TestDecodeSource_LinksNoRuleProviders(t *testing.T) {
	d, err := DecodeSource(readFixture(t, "mixed-subscription.txt"))
	if err != nil {
		t.Fatalf("DecodeSource: %v", err)
	}
	if len(d.Nodes) == 0 {
		t.Fatal("应解析出节点")
	}
	if d.RuleProviders != nil {
		t.Errorf("链接来源不应有 rule-provider: %v", d.RuleProviders)
	}
	if d.Warnings != nil {
		t.Errorf("链接来源 Warnings 应为 nil: %v", d.Warnings)
	}
}
