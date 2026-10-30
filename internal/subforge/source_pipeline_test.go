package subforge

import (
	"fmt"
	"testing"
)

func TestManagerSourcePipeline(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	a, err := m.AddAirport(ctx, "机场P", "https://p.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if got := AirportSourceKey(a.ID); got != fmt.Sprintf("airport:%d", a.ID) {
		t.Fatalf("AirportSourceKey = %q", got)
	}
	if err := m.SetAirportPipeline(ctx, a.ID, "pre"); err != nil {
		t.Fatalf("SetAirportPipeline: %v", err)
	}
	got, err := m.GetAirport(ctx, a.ID)
	if err != nil || got.Pipeline != "pre" {
		t.Fatalf("GetAirport pipeline = %q err=%v", got.Pipeline, err)
	}
	if err := m.SetSelfSourcePipeline(ctx, "selfpre"); err != nil {
		t.Fatalf("SetSelfSourcePipeline: %v", err)
	}
	if v, err := m.GetSelfSourcePipeline(ctx); err != nil || v != "selfpre" {
		t.Fatalf("GetSelfSourcePipeline = %q err=%v", v, err)
	}
}
