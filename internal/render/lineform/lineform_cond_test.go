package lineform

import (
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

func TestQIfEmpty(t *testing.T) {
	if got := qIf(""); got != "" {
		t.Fatalf("got %q", got)
	}
}
func TestQAlwaysEmpty(t *testing.T) {
	if got := qAlways(""); got != "" {
		t.Fatalf("got %q", got)
	}
}
func TestIsTrueCases(t *testing.T) {
	cases := []struct {
		v    any
		want bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"1", true},
		{"false", false},
		{42, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isTrue(c.v); got != c.want {
			t.Fatalf("isTrue(%v)=%v want %v", c.v, got, c.want)
		}
	}
}
func TestNewRuleFormatterIntervalDefault(t *testing.T) {
	f := NewRuleFormatter(nil, false, false, 0)
	out := f("DOMAIN-SUFFIX,example.com,Proxy")
	if out.Local == "" {
		t.Fatalf("expected local rule")
	}
}
func TestRuleGroupShort(t *testing.T) {
	if got := ruleGroup([]string{"a"}); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := ruleGroup([]string{"a", "Proxy"}); got != "Proxy" {
		t.Fatalf("got %q", got)
	}
}
func TestIntervalFromRuleParse(t *testing.T) {
	if got := intervalFromRule([]string{"update-interval=abc"}); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := intervalFromRule([]string{"update-interval=10"}); got != 10 {
		t.Fatalf("got %d", got)
	}
}
func TestGeneralSectionNameFalse(t *testing.T) {
	f := Format{HasSections: false}
	if got := f.GeneralSectionName(); got != "" {
		t.Fatalf("got %q", got)
	}
}
func TestLabelUnnamed(t *testing.T) {
	n := model.Node{Type: "ss"}
	if got := label(n); got != "<unnamed ss>" {
		t.Fatalf("got %q", got)
	}
	if got := label(model.Node{Name: "foo"}); got != "foo" {
		t.Fatalf("got %q", got)
	}
}
func TestManagedConfigDefaultInterval(t *testing.T) {
	r := New(SurgeFormat(), Options{ManagedConfigURL: "http://x", UpdateInterval: 0})
	_, err := r.Render(render.Request{Groups: []render.Group{}, Rules: []string{}})
	if err != nil {
		t.Fatalf("err %v", err)
	}
}
func TestSkipGroupLineWarn(t *testing.T) {
	r := New(SurgeFormat(), Options{})
	req := render.Request{Groups: []render.Group{{Name: "g", Type: "wechat", Proxies: []string{"nope"}}}, Rules: []string{}}
	_, err := r.Render(req)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(r.Warnings()) == 0 {
		t.Fatalf("expected warning")
	}
}
func TestRemoteRuleUnsupportedWarn(t *testing.T) {
	r := New(SurgeFormat(), Options{})
	req := render.Request{Groups: []render.Group{}, Rules: []string{"RULE-SET,http://x,Proxy"}}
	_, err := r.Render(req)
	if err != nil {
		t.Fatalf("err %v", err)
	}
}
func TestNodeVariants(t *testing.T) {
	n := nodes("n", "socks5", "1.1.1.1", "443", map[string]any{"username": "u", "password": "p"})
	s, ok := surgeNode(n)
	if !ok || s == "" {
		t.Fatalf("surge socks5: %q %v", s, ok)
	}
	n2 := nodes("n2", "http", "1.1.1.1", "443", map[string]any{"tls": true, "username": "u", "password": "p"})
	s2, ok2 := surgeNode(n2)
	if !ok2 || s2 == "" {
		t.Fatalf("surge http: %q %v", s2, ok2)
	}
	n3 := nodes("n3", "vmess", "1.1.1.1", "443", map[string]any{"cipher": "auto", "uuid": "u", "tls": true, "servername": "sni", "network": "ws"})
	s3, _ := quanNode(n3)
	if s3 == "" {
		t.Fatalf("quan vmess empty")
	}
	n4 := nodes("n4", "vmess", "1.1.1.1", "443", map[string]any{"cipher": "auto", "uuid": "u", "network": "ws"})
	s4, _ := quanxNode(n4)
	if s4 == "" {
		t.Fatalf("quanx vmess empty")
	}
	n5 := nodes("n5", "vmess", "1.1.1.1", "443", map[string]any{"cipher": "auto", "uuid": "u", "servername": "sni"})
	s5, _ := loonNode(n5)
	if s5 == "" {
		t.Fatalf("loon vmess empty")
	}
	n6 := nodes("n6", "vmess", "1.1.1.1", "443", map[string]any{"cipher": "auto", "uuid": "u", "tls": true})
	s6, _ := surfboardNode(n6)
	if s6 == "" {
		t.Fatalf("surfboard vmess empty")
	}
	n7 := nodes("n7", "socks5", "1.1.1.1", "443", map[string]any{"username": "u", "password": "p"})
	s7, _ := surfboardNode(n7)
	if s7 == "" {
		t.Fatalf("surfboard socks5 empty")
	}
	n8 := nodes("n8", "http", "1.1.1.1", "443", map[string]any{"tls": true, "username": "u", "password": "p"})
	s8, _ := surfboardNode(n8)
	if s8 == "" {
		t.Fatalf("surfboard http empty")
	}
}
func TestBuildGroupLineSelectEmpty(t *testing.T) {
	st := groupStyle{selectType: ""}
	g := render.Group{Name: "g", Type: "select", Proxies: []string{"a"}}
	out := buildGroupLine("g", g, st, map[string]bool{"a": true})
	if out != "" {
		t.Fatalf("got %q", out)
	}
}
func TestBuildGroupLineUrlTestEmpty(t *testing.T) {
	st := groupStyle{urlTestType: ""}
	g := render.Group{Name: "g", Type: "url-test", Proxies: []string{"a"}}
	out := buildGroupLine("g", g, st, map[string]bool{"a": true})
	if out != "" {
		t.Fatalf("got %q", out)
	}
}
func TestBuildGroupLineFallbackTest(t *testing.T) {
	st := groupStyle{fallback: "fallback", urlKey: "url", intervalKey: "interval"}
	g := render.Group{Name: "g", Type: "fallback", Proxies: []string{"a"}, Test: "http://t", Options: render.UrlTestOptions{Interval: 5}}
	out := buildGroupLine("g", g, st, map[string]bool{"a": true})
	if out == "" {
		t.Fatalf("empty")
	}
}
