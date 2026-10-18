package clashmeta

import "testing"

func TestRendererName_ClashMeta(t *testing.T) {
	rJSON := &Renderer{opts: Options{JSON: true}}
	if got := rJSON.Name(); got != "clash.meta.json" {
		t.Errorf("JSON Name() = %q, want %q", got, "clash.meta.json")
	}
	rYAML := &Renderer{opts: Options{JSON: false}}
	if got := rYAML.Name(); got != "clash.meta" {
		t.Errorf("non-JSON Name() = %q, want %q", got, "clash.meta")
	}
	if got := (&Renderer{}).Name(); got != "clash.meta" {
		t.Errorf("zero Renderer Name() = %q, want %q", got, "clash.meta")
	}
}
