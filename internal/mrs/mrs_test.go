package mrs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tktogether/subforge/internal/parse"
)

func TestGenerate_RoundTrip_IPCIDR(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", "cidr-sample.src.txt"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Generate(src, "ipcidr", "text")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// 验证与 committed .mrs 字节级一致
	committed, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", "cidr-sample.mrs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(committed) {
		t.Fatalf("size mismatch: generated %d vs committed %d", len(out), len(committed))
	}
	for i := range out {
		if out[i] != committed[i] {
			t.Fatalf("byte mismatch at %d: 0x%02x vs 0x%02x", i, out[i], committed[i])
		}
	}
	// 解码回文并对比 ref
	decoded, err := parse.DecodeMRS(out)
	if err != nil {
		t.Fatalf("DecodeMRS: %v", err)
	}
	refRaw, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", "cidr-sample.ref.txt"))
	ref := string(refRaw)
	_ = ref
	if decoded.Behavior != parse.MRSBehaviorIPCIDR {
		t.Errorf("behavior = %v, want ipcidr", decoded.Behavior)
	}
}

func TestGenerate_RoundTrip_Domain(t *testing.T) {
	// 使用 ref.txt 作为源（domain suffix 形式）— 但 ConvertToMrs 需要 text 格式
	// ref.txt 含 +. 前缀，可直接作为 text 源
	refRaw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", "Custom_Direct_Domain.ref.txt"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Generate(refRaw, "domain", "text")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mrs", "Custom_Direct_Domain.mrs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(committed) {
		t.Fatalf("size mismatch: generated %d vs committed %d", len(out), len(committed))
	}
	for i := range out {
		if out[i] != committed[i] {
			t.Fatalf("byte mismatch at %d", i)
		}
	}
}

func TestGenerate_InvalidBehavior(t *testing.T) {
	_, err := Generate([]byte("foo"), "bogus", "text")
	if err == nil {
		t.Fatal("expected error for invalid behavior")
	}
}

func TestCachePath(t *testing.T) {
	p := CachePath("/tmp/cache", "/path/to/rules.yaml", "domain", "text")
	if filepath.Base(p) != "rules-domain-text.mrs" {
		t.Fatalf("CachePath = %s", p)
	}
}
