package sssub

import (
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

func TestStrValBasic(t *testing.T) {
	if got := strVal("a"); got != `"a"` {
		t.Fatalf("got %s", got)
	}
}
func TestMapStringNil(t *testing.T) {
	if got := mapString(nil, "k"); got != "" {
		t.Fatalf("got %q", got)
	}
}
func TestNodeLabelWithName(t *testing.T) {
	n := model.Node{Name: "foo", Type: "ss"}
	if got := nodeLabel(n); got != "foo" {
		t.Fatalf("got %q", got)
	}
	n2 := model.Node{Type: "ss"}
	if got := nodeLabel(n2); got != "<unnamed ss>" {
		t.Fatalf("got %q", got)
	}
}
func TestSSRNodeFields(t *testing.T) {
	n := model.NewNodeFromParts("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
		"cipher": "aes-256-gcm", "password": "password123",
	})
	fields, ok := nodeFields(DefaultOptions(), n)
	if !ok || len(fields) == 0 {
		t.Fatalf("ssr nodeFields failed: %v %d", ok, len(fields))
	}
}
func TestJsonListVal(t *testing.T) {
	if got := jsonListVal([]string{"a", "b"}); got != `["a","b"]` {
		t.Fatalf("got %s", got)
	}
}
func TestSSSubRenderSSR(t *testing.T) {
	r := New(DefaultOptions())
	req := render.Request{Nodes: model.NodeList{
		model.NewNodeFromParts("ssr-node", "ssr", "ssr.example.com", "1234", map[string]any{
			"cipher": "aes-256-gcm", "password": "password123",
		}),
	}}
	out, err := r.Render(req)
	if err != nil || len(out) == 0 {
		t.Fatalf("render err %v len %d", err, len(out))
	}
}
