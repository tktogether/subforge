package subforge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/manifest"
	"github.com/tktogether/subforge/internal/subcfg"
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
	fg := subcfg.DefaultFinalGroupConfig()
	fg.Enabled = true
	fg.Overrides = map[string][]string{
		"rule:base":  {"出口"},
		"group:self": {"自建出口"},
		"rule:other": {"其它"},
	}
	if err := m.SetProfileFinalGroup(ctx, p.ID, fg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	// 第二个方案没有任何覆盖配置：改名循环要能跳过它（ok=false 分支）。
	if _, err := m.AddProfile(ctx, "方案二", "", "", "", true, 0); err != nil {
		t.Fatalf("AddProfile(方案二): %v", err)
	}

	if _, err := m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "renamed-20260101-000000.000000000.yaml")); err != nil {
		t.Fatalf("new version missing: %v", err)
	}
	profiles, _ := m.ListProfiles(ctx)
	var got Profile
	for _, item := range profiles {
		if item.ID == p.ID {
			got = item
		}
	}
	if len(got.RuleSources) != 1 || got.RuleSources[0] != "renamed" {
		t.Fatalf("profile binding not renamed: %+v", profiles)
	}
	overrides := got.FinalGroup.Overrides
	if got := overrides["rule:renamed"]; len(got) != 1 || got[0] != "出口" {
		t.Fatalf("最终组覆盖键未迁移: %+v", overrides)
	}
	if _, ok := overrides["rule:base"]; ok {
		t.Fatalf("旧覆盖键仍存在: %+v", overrides)
	}
	if got := overrides["group:self"]; len(got) != 1 || got[0] != "自建出口" {
		t.Fatalf("来源覆盖键被误改: %+v", overrides)
	}
	if got := overrides["rule:other"]; len(got) != 1 || got[0] != "其它" {
		t.Fatalf("其它规则源覆盖键被误改: %+v", overrides)
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
