package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mrsFixture 读取 MRS 测试夹具。
func mrsFixture(t *testing.T, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", name))
	if err != nil {
		t.Fatalf("读取夹具 %s: %v", name, err)
	}
	return buf
}

// TestDecodeMRS_MatchesMihomoReference 是与内核自带命令的逐行对拍：
//
//	mihomo convert-ruleset domain mrs Custom_Direct_Domain.mrs ref.txt
//
// 夹具里的 .mrs 与 ref.txt 都是这条命令在 mihomo v1.19.31 上产出的，
// 因此本用例直接锁死"我们解出来的规则集 == 内核解出来的规则集"。
// 任何解码偏差（漏条目、排序不同、+. 合并逻辑不同）都会在这里暴露。
func TestDecodeMRS_MatchesMihomoReference(t *testing.T) {
	got, err := DecodeMRS(mrsFixture(t, "Custom_Direct_Domain.mrs"))
	if err != nil {
		t.Fatalf("DecodeMRS: %v", err)
	}
	if got.Behavior != MRSBehaviorDomain {
		t.Errorf("行为 = %v，want domain", got.Behavior)
	}
	refRaw := string(mrsFixture(t, "Custom_Direct_Domain.ref.txt"))
	ref := strings.Split(strings.ReplaceAll(refRaw, "\r\n", "\n"), "\n")
	// 去掉文件末尾换行产生的空串。
	for len(ref) > 0 && ref[len(ref)-1] == "" {
		ref = ref[:len(ref)-1]
	}
	if len(got.Rules) != len(ref) {
		t.Fatalf("规则条数 = %d，want %d（与内核输出不一致）", len(got.Rules), len(ref))
	}
	for i := range ref {
		if got.Rules[i] != ref[i] {
			t.Fatalf("第 %d 条 = %q，want %q", i+1, got.Rules[i], ref[i])
		}
	}
	if got.Count != 992 {
		t.Errorf("Count = %d，want 992（文件头记录值）", got.Count)
	}
}

// TestDecodeMRS_IPCIDRMatchesMihomoReference ipcidr 行为的逐行对拍。
// 夹具由 mihomo v1.19.31 生成：text → mrs → text，两侧都是内核产出。
func TestDecodeMRS_IPCIDRMatchesMihomoReference(t *testing.T) {
	got, err := DecodeMRS(mrsFixture(t, "cidr-sample.mrs"))
	if err != nil {
		t.Fatalf("DecodeMRS: %v", err)
	}
	if got.Behavior != MRSBehaviorIPCIDR {
		t.Fatalf("行为 = %v，want ipcidr", got.Behavior)
	}
	refRaw := strings.ReplaceAll(string(mrsFixture(t, "cidr-sample.ref.txt")), "\r\n", "\n")
	ref := strings.Split(strings.TrimRight(refRaw, "\n"), "\n")
	if len(got.Rules) != len(ref) {
		t.Fatalf("网段条数 = %d，want %d: %v", len(got.Rules), len(ref), got.Rules)
	}
	for i := range ref {
		if got.Rules[i] != ref[i] {
			t.Errorf("第 %d 条 = %q，want %q", i+1, got.Rules[i], ref[i])
		}
	}
}

// TestDecodeMRS_DomainMergesExactAndSuffix 精确条目与后缀条目成对时合并成 "+.<domain>"，
// 这是内核 DumpMrs 的既定行为；不合并会让产物规则行数翻倍。
func TestDecodeMRS_DomainMergesExactAndSuffix(t *testing.T) {
	// 真实夹具里 992 条全部是 "+." 形态，说明合并确实发生了。
	got, err := DecodeMRS(mrsFixture(t, "Custom_Direct_Domain.mrs"))
	if err != nil {
		t.Fatalf("DecodeMRS: %v", err)
	}
	for _, r := range got.Rules {
		if strings.HasPrefix(r, ".") {
			t.Errorf("残留未合并的后缀条目 %q", r)
		}
	}
	if len(got.Rules) == 0 {
		t.Fatal("规则集为空")
	}
}

// TestIsMRS 只认 zstd 帧魔数：文本规则集、空内容、截断内容都不能被误判。
func TestIsMRS(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		want bool
	}{
		{"真实 mrs", mrsFixture(t, "Custom_Direct_Domain.mrs"), true},
		{"zstd 魔数最短形态", []byte{0x28, 0xb5, 0x2f, 0xfd}, true},
		{"文本域名清单", []byte("+.example.com\nexample.org\n"), false},
		{"空", nil, false},
		{"截断到 3 字节", []byte{0x28, 0xb5, 0x2f}, false},
		{"yaml 规则集", []byte("payload:\n  - +.a.com\n"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsMRS(c.buf); got != c.want {
				t.Errorf("IsMRS = %v，want %v", got, c.want)
			}
		})
	}
}

// TestDecodeMRS_RejectsGarbage 非 MRS 内容与损坏内容必须报错，不能静默产出空规则集。
func TestDecodeMRS_RejectsGarbage(t *testing.T) {
	for _, c := range []struct {
		name string
		buf  []byte
	}{
		{"空", nil},
		{"纯文本", []byte("+.example.com\n")},
		{"zstd 魔数但内容损坏", []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x01, 0x02, 0x03}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := DecodeMRS(c.buf); err == nil {
				t.Error("应报错，实际成功")
			}
		})
	}
}
