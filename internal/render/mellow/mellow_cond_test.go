package mellow

import (
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

func TestMellowNewDefaults(t *testing.T) {
	r := New(Options{})
	if r == nil {
		t.Fatal("nil")
	}
}
func TestMellowManagedConfig(t *testing.T) {
	r := New(Options{ManagedConfigURL: "http://x", UpdateInterval: 0})
	_, err := r.Render(render.Request{})
	if err != nil {
		t.Fatalf("err %v", err)
	}
}
func TestMellowSkipBadNode(t *testing.T) {
	r := New(DefaultOptions())
	req := render.Request{Nodes: model.NodeList{model.NewNodeFromParts("bad", "unknownproto", "1.1.1.1", "443", nil)}}
	_, err := r.Render(req)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(r.Warnings()) == 0 {
		t.Fatalf("expected warning")
	}
}
func TestMellowSkipGroup(t *testing.T) {
	r := New(DefaultOptions())
	req := render.Request{Groups: []render.Group{{Name: "g", Type: "select", Proxies: []string{"nope"}}}}
	_, err := r.Render(req)
	if err != nil {
		t.Fatalf("err %v", err)
	}
}
func TestMellowGroupEmptyType(t *testing.T) {
	g := render.Group{Name: "g", Type: "", Proxies: []string{"a"}}
	out := groupLine(g, map[string]bool{"a": true})
	if out == "" {
		t.Fatalf("empty")
	}
}
func TestMellowGroupUrlTest(t *testing.T) {
	g := render.Group{Name: "g", Type: "url-test", Proxies: []string{"a"}, Options: render.UrlTestOptions{Interval: 0, Timeout: 0}}
	out := groupLine(g, map[string]bool{"a": true})
	if out == "" {
		t.Fatalf("empty")
	}
}
func TestMellowRuleTypes(t *testing.T) {
	cases := []string{"GEOSITE,cn,DIRECT", "RULE-SET,http://x,Proxy", "MATCH,Proxy", "DOMAIN,example.com,DIRECT"}
	for _, c := range cases {
		_, err := ruleLine(c)
		_ = err
	}
}
func TestMellowLabelWithName(t *testing.T) {
	if got := label(model.Node{Name: "foo", Type: "ss"}); got != "foo" {
		t.Fatalf("got %q", got)
	}
	if got := label(model.Node{Type: "ss"}); got != "<unnamed ss>" {
		t.Fatalf("got %q", got)
	}
}
