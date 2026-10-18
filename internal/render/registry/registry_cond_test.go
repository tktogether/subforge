package registry

import (
	"testing"

	"github.com/tktogether/subforge/internal/render/clashmeta"
)

func TestRegistryLookupAllTargets(t *testing.T) {
	cases := []string{"clash", "clash-json", "list", "sub", "mixed", "surge", "singbox", "ss-sub", "mellow", "nope"}
	for _, tg := range cases {
		r, err := Lookup(tg, clashmeta.Options{})
		if tg == "nope" {
			if err == nil {
				t.Fatalf("expected err for %s", tg)
			}
			continue
		}
		if err != nil || r == nil {
			t.Fatalf("Lookup(%s): %v %v", tg, r, err)
		}
	}
}
func TestRegistryAliasRendererCases(t *testing.T) {
	for _, tname := range []string{"ss-sub", "ss-sub-b64", "mellow", "nope"} {
		r, err := aliasRenderer(tname)
		if tname == "nope" {
			if err == nil {
				t.Fatalf("expected err")
			}
			return
		}
		if err != nil || r == nil {
			t.Fatalf("aliasRenderer(%s): %v %v", tname, r, err)
		}
	}
}
func TestRegistryLineFormFormatCases(t *testing.T) {
	for _, name := range []string{"surge", "quan", "quanx", "loon", "surfboard", "clash", "nope"} {
		_ = lineFormFormat(name)
	}
}
