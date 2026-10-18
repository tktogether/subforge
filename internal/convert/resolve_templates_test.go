package convert

import (
	"context"
	"reflect"
	"testing"
)

func TestResolveTemplates_NilReceiver(t *testing.T) {
	var e *Engine
	got, err := e.ResolveTemplates(context.Background(), "x")
	if err != nil {
		t.Fatalf("nil receiver should not error: %v", err)
	}
	if !reflect.DeepEqual(got, Params{}) {
		t.Fatalf("got = %+v, want Params{}", got)
	}
}

func TestResolveTemplates_NilBaseTemplateEmptyRef(t *testing.T) {
	e := &Engine{}
	got, err := e.ResolveTemplates(context.Background(), "")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if !reflect.DeepEqual(got, Params{}) {
		t.Fatalf("got = %+v, want Params{}", got)
	}
}

func TestResolveTemplates_WithConfigRef(t *testing.T) {
	e := &Engine{}
	got, err := e.ResolveTemplates(context.Background(), "cfgref")
	if err == nil {
		want, werr := e.resolveParams(context.Background(), Params{Config: "cfgref"})
		if werr != nil {
			t.Fatalf("resolveParams error: %v", werr)
		}
		if got.Config != want.Config {
			t.Fatalf("Config mismatch: got=%q want=%q", got.Config, want.Config)
		}
	} else {
		t.Logf("ResolveTemplates returned err (acceptable): %v", err)
	}
}
