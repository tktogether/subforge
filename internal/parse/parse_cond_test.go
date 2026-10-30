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

func TestUsableNodeNamesSkipsPlaceholders(t *testing.T) {
	nodes := []map[string]any{
		{"name": "real-1", "server": "example.com"},
		{"name": "real-2", "server": " 203.0.113.1 "},
		{"name": "info-1", "server": "127.0.0.1"},
		{"name": "info-2", "server": "localhost"},
		{"name": "info-3", "server": "::1"},
		{"name": "info-4", "server": "[::1]"},
		{"name": "info-5", "server": "0.0.0.0"},
		{"name": "info-6", "server": ""},
		{"name": "info-7"},
		{"name": "info-8", "server": 123},
	}
	got := UsableNodeNames(nodes)
	want := []string{"real-1", "real-2"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("UsableNodeNames = %v, want %v", got, want)
	}
	if got := UsableNodeNames(nil); len(got) != 0 {
		t.Fatalf("nil 应返回空切片, got %v", got)
	}
	allPlaceholder := []map[string]any{
		{"name": "info-1", "server": "127.0.0.1"},
		{"name": "info-2", "server": "localhost"},
	}
	if got := UsableNodeNames(allPlaceholder); len(got) != 2 || got[0] != "info-1" || got[1] != "info-2" {
		t.Fatalf("全部是占位节点时应原样返回, got %v", got)
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
