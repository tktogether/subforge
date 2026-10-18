package render_test

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/render"
)

func TestCond_Render_MarshalJSONDocEmptyMapping(t *testing.T) {
	// render.go:224
	node, err := render.MappingNode()
	if err != nil {
		t.Fatal(err)
	}
	out, err := render.MarshalJSONDoc(node)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "{}\n" {
		t.Fatalf("got %s", out)
	}
}

func TestCond_Render_JsonScalarTagEmpty(t *testing.T) {
	// render.go:279
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "", Value: "hello"}
	out, err := render.MarshalJSONDoc(node)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "\"hello\"\n" {
		t.Fatalf("got %s", out)
	}
}

func TestCond_Render_JsonScalarBoolTrue(t *testing.T) {
	// render.go:286
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
	out, err := render.MarshalJSONDoc(node)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "true\n" {
		t.Fatalf("got %s", out)
	}
}

func TestCond_Render_JsonScalarBoolFalse(t *testing.T) {
	// render.go:289
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "nope"}
	out, err := render.MarshalJSONDoc(node)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "false\n" {
		t.Fatalf("got %s", out)
	}
}

func TestCond_Render_JsonScalarIntOk(t *testing.T) {
	// render.go:293
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "42"}
	out, err := render.MarshalJSONDoc(node)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "42\n" {
		t.Fatalf("got %s", out)
	}
}

func TestCond_Render_JsonScalarFloatOk(t *testing.T) {
	// render.go:297
	node := yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: "3.14"}
	out, err := render.MarshalJSONDoc(node)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "3.14\n" {
		t.Fatalf("got %s", out)
	}
}

// marshalErrYAML triggers yaml.Marshal error (not panic).
type marshalErrYAML struct{}

func (marshalErrYAML) MarshalYAML() (any, error) { return nil, fmt.Errorf("boom") }

func TestCond_Render_MappingNodeKeyErr(t *testing.T) {
	_, err := render.MappingNode(marshalErrYAML{}, "v")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCond_Render_MappingNodeValErr(t *testing.T) {
	_, err := render.MappingNode("k", marshalErrYAML{})
	if err == nil {
		t.Fatal("expected error")
	}
}
