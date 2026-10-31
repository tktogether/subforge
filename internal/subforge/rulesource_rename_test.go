package subforge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/manifest"
)

func TestManagerRenameRuleSource(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	mf := &manifest.Manifest{Sources: []manifest.Source{{Name: "base", URL: "http://example.com/base.yaml"}}, Output: manifest.Output{Format: "clash-meta", Path: filepath.Join(dir, "rules")}}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	out := filepath.Join(dir, "rules")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	version := "base-20260101-000000.000000000.yaml"
	if err := os.WriteFile(filepath.Join(out, version), []byte("rules: []\n"), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, ".current-versions.json"), []byte(`{"base":"`+version+`"}`), 0o644); err != nil {
		t.Fatalf("write pointer: %v", err)
	}
	p, err := m.AddProfile(ctx, "方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := m.SetProfileRuleSources(ctx, p.ID, []string{"base"}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}

	if _, err := m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "renamed-20260101-000000.000000000.yaml")); err != nil {
		t.Fatalf("new version missing: %v", err)
	}
	profiles, _ := m.ListProfiles(ctx)
	if len(profiles) != 1 || profiles[0].RuleSources[0] != "renamed" {
		t.Fatalf("profile binding not renamed: %+v", profiles)
	}
	cur, _ := os.ReadFile(filepath.Join(out, ".current-versions.json"))
	if !strings.Contains(string(cur), "renamed") {
		t.Fatalf("pointer not renamed: %s", cur)
	}
}

func TestManagerRenameRuleSourceRejectsAirport(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	mf := &manifest.Manifest{Sources: []manifest.Source{{Name: "airport-1-hk", URL: "http://example.com/a.yaml"}}, Output: manifest.Output{Format: "clash-meta", Path: "./rules"}}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "airport-1-hk", "renamed"); err == nil || !strings.Contains(err.Error(), "机场") {
		t.Fatalf("机场规则源改名应被拒绝: %v", err)
	}
}
