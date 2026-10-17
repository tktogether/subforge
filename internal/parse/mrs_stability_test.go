package parse

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeMRS_StableAcrossCalls 同一份字节反复解码必须得到完全一样的结果。
//
// 这条守门针对 succinct trie 的遍历：一旦解码带上不确定性（map 迭代、rank/select
// 缓存复用等），产物规则数会随进程抖动，而且只在线上偶发、极难复现。
func TestDecodeMRS_StableAcrossCalls(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", "Custom_Direct_Domain.mrs"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := DecodeMRS(buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		got, err := DecodeMRS(buf)
		if err != nil {
			t.Fatalf("第 %d 次解码: %v", i+2, err)
		}
		if len(got.Rules) != len(first.Rules) {
			t.Fatalf("第 %d 次条数 = %d，want %d", i+2, len(got.Rules), len(first.Rules))
		}
		for j := range got.Rules {
			if got.Rules[j] != first.Rules[j] {
				t.Fatalf("第 %d 次第 %d 条 = %q，want %q", i+2, j+1, got.Rules[j], first.Rules[j])
			}
		}
	}
}
