package clashmeta

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/render"
)

// marshalErr 触发 yaml.Marshal 返回错误（而非 panic）。
type marshalErr struct{}

func (marshalErr) MarshalYAML() (any, error) { return nil, fmt.Errorf("boom") }

func TestClashmetaCond_MixedPortZero(t *testing.T) {
	r := New(Options{})
	if r.opts.MixedPort == 0 {
		t.Fatal("MixedPort not defaulted")
	}
	_ = renderTest(t, render.Request{Nodes: testNodes()})
}

func TestClashmetaCond_InterfaceName(t *testing.T) {
	opts := DefaultOptions()
	opts.InterfaceName = "eth0"
	r := New(opts)
	body, err := r.Render(render.Request{Nodes: testNodes()})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(body), "interface-name") {
		t.Fatal("interface-name missing")
	}
}

func TestClashmetaCond_ProxiesRawErr(t *testing.T) {
	r := New(DefaultOptions())
	_, err := r.proxiesNode([]map[string]any{{"k": marshalErr{}}})
	if err == nil {
		t.Fatal("expected error")
	}
	badNodes := model.NodeList{model.NewNodeFromParts("X", "vless", "x.com", "443", map[string]any{"k": marshalErr{}})}
	_, err = r.Render(render.Request{Nodes: badNodes})
	if err == nil {
		t.Fatal("expected Render error")
	}
}

func TestClashmetaCond_ProvidersRawErr(t *testing.T) {
	r := New(DefaultOptions())
	_, err := r.providersNode([]model.Provider{{Name: "p", Raw: map[string]any{"k": marshalErr{}}}})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = r.Render(render.Request{Nodes: testNodes(), Providers: []model.Provider{{Name: "p", Raw: map[string]any{"k": marshalErr{}}}}})
	if err == nil {
		t.Fatal("expected Render error")
	}
}

func TestClashmetaCond_GroupTypeEmpty(t *testing.T) {
	r := New(DefaultOptions())
	g := render.Group{Name: "g", Type: "", Proxies: []string{"DIRECT"}}
	_, err := r.groupNode(g)
	if err != nil {
		t.Fatalf("groupNode: %v", err)
	}
	body, err := r.Render(render.Request{Nodes: testNodes(), Groups: []render.Group{g}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	_ = body
}
