package parse

import (
	"testing"
)

// parse_cond_test.go — 条件覆盖补充

func TestMRSBehaviorStringDefault(t *testing.T) {
	// mrs.go:55 未覆盖：switch default
	b := MRSBehavior(99)
	if b.String() == "" {
		t.Fatalf("String 不应为空")
	}
}

func TestDumpDomainSetNil(t *testing.T) {
	// mrs.go:157 未覆盖：ds==nil
	if got := dumpDomainSet(nil); got != nil {
		t.Fatalf("nil 应返回 nil, got %v", got)
	}
}

func TestFormatStringDefault(t *testing.T) {
	// parse.go:62 未覆盖：switch default
	f := Format(99)
	if f.String() != "unknown" {
		t.Fatalf("got %q", f.String())
	}
}

func TestValidateInvalid(t *testing.T) {
	// parse.go:44 未覆盖：ParseProxy err
	err := Validate(map[string]any{"type": "bogus"})
	if err == nil {
		t.Fatalf("应返回错误")
	}
}

func TestNameOfNoName(t *testing.T) {
	// parse.go:161 未覆盖：name 非 string 分支
	got := NameOf(map[string]any{"name": 123})
	if got != "123" {
		t.Fatalf("got %q", got)
	}
}

func TestPreviewLong(t *testing.T) {
	// parse.go:195 未覆盖：len(b)>64
	b := make([]byte, 100)
	for i := range b {
		b[i] = 'a'
	}
	got := preview(b)
	if len(got) != 64 {
		t.Fatalf("got len %d", len(got))
	}
}

func TestTryBase64BlobValid(t *testing.T) {
	// parse.go:177 未覆盖：DecodeString err==nil
	out, ok := tryBase64Blob([]byte("aGVsbG8gd29ybGQhISEhISE="))
	if !ok || string(out) != "hello world!!!!!!" {
		t.Fatalf("got %v %v", ok, string(out))
	}
}
