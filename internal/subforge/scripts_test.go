package subforge

import (
	"context"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/store"
)

func TestScriptResolver_DefaultDisabled(t *testing.T) {
	m := newTestManager(t)
	r := m.ScriptResolver()
	if r.ScriptsEnabled() {
		t.Fatal("默认应为 false")
	}
}

func TestScriptResolver_SetConfigEnables(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	if err := m.setConfig(ctx, ScriptEnabledKey, "1"); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	r := m.ScriptResolver()
	if !r.ScriptsEnabled() {
		t.Fatal("写入 1 后应为 true")
	}
}

func TestScriptResolver_TruthyValues(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"  ON  ", true},
		{"True", true},
		{"yes", true},
		{"0", false},
		{"", false},
		{"false", false},
	}
	for _, c := range cases {
		m := newTestManager(t)
		ctx := context.Background()
		if err := m.setConfig(ctx, ScriptEnabledKey, c.val); err != nil {
			t.Fatalf("setConfig %q: %v", c.val, err)
		}
		got := m.ScriptResolver().ScriptsEnabled()
		if got != c.want {
			t.Errorf("val=%q: got %v, want %v", c.val, got, c.want)
		}
	}
}

func TestScriptResolver_ResolveHit(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	_, err := m.DB().CreateScript(ctx, store.Script{Name: "foo", Code: "return node;", Enabled: true})
	if err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	code, err := m.ScriptResolver().ResolveScript("foo")
	if err != nil {
		t.Fatalf("ResolveScript: %v", err)
	}
	if code != "return node;" {
		t.Errorf("code = %q, want %q", code, "return node;")
	}
}

func TestScriptResolver_ResolveNotFound(t *testing.T) {
	m := newTestManager(t)
	_, err := m.ScriptResolver().ResolveScript("nope")
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("错误应含脚本名: %v", err)
	}
}

func TestScriptResolver_ResolveDisabledScript(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	_, err := m.DB().CreateScript(ctx, store.Script{Name: "disabled", Code: "return node;", Enabled: false})
	if err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	_, err = m.ScriptResolver().ResolveScript("disabled")
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "已停用") {
		t.Errorf("错误应含 已停用: %v", err)
	}
}
