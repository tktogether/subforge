package store

import (
	"testing"

	"github.com/tktogether/subforge/internal/subcfg"
)

func TestProfileFinalGroupRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "统一组", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Enabled = true
	cfg.Mode = subcfg.FinalGroupModeCustom
	cfg.Name = "🌐 统一出口"
	cfg.IncludeDirect = false
	cfg.KeepSourceTerminals = true
	cfg.Overrides = map[string][]string{"group:airport:A": {"选择节点"}}
	if err := d.SetProfileFinalGroup(ctx, p.ID, cfg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if !got.FinalGroup.Enabled || got.FinalGroup.Name != "🌐 统一出口" ||
		got.FinalGroup.IncludeDirect || !got.FinalGroup.KeepSourceTerminals {
		t.Fatalf("FinalGroup = %+v", got.FinalGroup)
	}
	if names := got.FinalGroup.Overrides["group:airport:A"]; len(names) != 1 || names[0] != "选择节点" {
		t.Fatalf("Overrides = %+v", got.FinalGroup.Overrides)
	}
}

func TestProfileFinalGroupDefaultsForOldRow(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "旧方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.FinalGroup.Enabled || got.FinalGroup.Mode != subcfg.FinalGroupModeAuto ||
		got.FinalGroup.Name != subcfg.DefaultFinalGroupName || !got.FinalGroup.IncludeDirect {
		t.Fatalf("默认 FinalGroup = %+v", got.FinalGroup)
	}
}
