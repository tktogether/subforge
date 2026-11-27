package subforge

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/manifest"
	"github.com/tktogether/subforge/internal/subcfg"
)

// writeBareManifest 给 m 一个只含 srcs 的最小 manifest（绕开 URL，专注增删语义）。
func writeBareManifest(t *testing.T, m *Manager, srcs ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	content := "sources:\n"
	for _, s := range srcs {
		content += "  - " + s + "\n"
	}
	content += "output:\n  format: text\n  path: " + filepath.ToSlash(filepath.Join(t.TempDir(), "out")) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := m.SetRuleManifestPath(context.Background(), path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	return path
}

// TestAddRuleSource 覆盖：缺清单自动创建、空 URL 报错、重名报错、name 缺省推导、正常追加写回。
func TestAddRuleSource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	p0, created0, err := m.AddRuleSource(ctx, manifest.Source{URL: "u"})
	if err != nil || !created0 {
		t.Fatalf("缺清单应自动创建: path=%q created=%v err=%v", p0, created0, err)
	}
	if _, statErr := os.Stat(p0); statErr != nil {
		t.Fatalf("自动创建的清单不存在: %v", statErr)
	}
	path := writeBareManifest(t, m, "name: a\n    url: u-a")
	if _, _, err := m.AddRuleSource(ctx, manifest.Source{URL: ""}); err == nil || !strings.Contains(err.Error(), "url 不能为空") {
		t.Fatalf("空 URL 应报错: %v", err)
	}
	if _, _, err := m.AddRuleSource(ctx, manifest.Source{Name: "a", URL: "u-dup"}); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("重名应报错: %v", err)
	}
	p, created, err := m.AddRuleSource(ctx, manifest.Source{URL: "https://example.com/neo.ini"})
	if err != nil || !created {
		t.Fatalf("AddRuleSource: %v created=%v", err, created)
	}
	if p != path {
		t.Fatalf("返回清单路径 = %s, want %s", p, path)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "name: neo") {
		t.Fatalf("缺省 name 未从 URL 推导: %s", raw)
	}
}

// TestRemoveRuleSource 覆盖：缺清单、不存在、删空、正常删除写回。
func TestRemoveRuleSource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.RemoveRuleSource(ctx, "a"); err == nil || !strings.Contains(err.Error(), "清单不存在") {
		t.Fatalf("缺清单应报错: %v", err)
	}
	writeBareManifest(t, m, "name: a\n    url: u-a", "name: b\n    url: u-b")
	if _, err := m.RemoveRuleSource(ctx, "ghost"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("未知源应报错: %v", err)
	}
	if _, err := m.RemoveRuleSource(ctx, "a"); err != nil {
		t.Fatalf("RemoveRuleSource: %v", err)
	}
	if _, err := m.RemoveRuleSource(ctx, "b"); err == nil || !strings.Contains(err.Error(), "至少保留") {
		t.Fatalf("删空应报错: %v", err)
	}
}

// TestUpdateRuleSourceOne 覆盖：正常单源更新与未知源报错（stub 服务器喂内容）。
func TestUpdateRuleSourceOne(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	if _, err := m.UpdateRuleSourceOne(ctx, "ghost"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("未知源应报错: %v", err)
	}
	results, err := m.UpdateRuleSourceOne(ctx, "a")
	if err != nil {
		t.Fatalf("UpdateRuleSourceOne: %v", err)
	}
	if len(results) != 1 || results[0].Source != "a" {
		t.Fatalf("results = %+v, want 单源 a", results)
	}
	if results[0].Err != nil {
		t.Fatalf("UpdateResult.Err = %v", results[0].Err)
	}
	if _, err := os.Stat(results[0].Path); err != nil {
		t.Fatalf("输出文件不存在: %v", err)
	}
}

// TestRemoveRuleSourceClearsFinalGroupOverride 删除规则源时同步清理方案里
// 指向它的最终组覆盖键；其它源的键保持不变。
func TestRemoveRuleSourceClearsFinalGroupOverride(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	writeBareManifest(t, m, "name: a\n    url: u-a", "name: b\n    url: u-b")
	p, err := m.AddProfile(ctx, "方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Overrides = map[string][]string{"rule:a": {"出口"}, "rule:b": {"其它"}}
	if err := m.SetProfileFinalGroup(ctx, p.ID, cfg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	if _, err := m.RemoveRuleSource(ctx, "a"); err != nil {
		t.Fatalf("RemoveRuleSource: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if _, ok := got.FinalGroup.Overrides["rule:a"]; ok {
		t.Errorf("被删源的覆盖键未清理: %v", got.FinalGroup.Overrides)
	}
	if _, ok := got.FinalGroup.Overrides["rule:b"]; !ok {
		t.Errorf("其它源的覆盖键被误删: %v", got.FinalGroup.Overrides)
	}
}

// TestRemoveRuleSourceRestoresManifestOnBindingFailure 覆盖清理失败时清单恢复。
func TestRemoveRuleSourceRestoresManifestOnBindingFailure(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	path := writeBareManifest(t, m, "name: a\n    url: u-a", "name: b\n    url: u-b")
	p, err := m.AddProfile(ctx, "方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Overrides = map[string][]string{"rule:a": {"出口"}}
	if err := m.SetProfileFinalGroup(ctx, p.ID, cfg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	if _, err := m.DB().Raw().ExecContext(ctx,
		`CREATE TRIGGER block_profile_update BEFORE UPDATE ON profiles BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
		t.Fatalf("创建触发器: %v", err)
	}
	_, err = m.RemoveRuleSource(ctx, "a")
	if err == nil || !errors.Is(err, ErrRuleSourceRenameInternal) {
		t.Fatalf("清理失败应返回内部错误，实际 %v", err)
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		t.Fatalf("LoadManifestRaw: %v", err)
	}
	if len(mf.Sources) != 2 {
		t.Errorf("清单未恢复，源数 = %d", len(mf.Sources))
	}
}

// TestRemoveRuleSourceRejectsPaddedBinding 空白绑定在运行期仍有效，删除保护必须同口径。
func TestRemoveRuleSourceRejectsPaddedBinding(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	writeBareManifest(t, m, "name: a\n    url: u-a", "name: b\n    url: u-b")
	p, err := m.AddProfile(ctx, "方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := m.SetProfileRuleSources(ctx, p.ID, []string{" a "}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}
	if _, err := m.RemoveRuleSource(ctx, "a"); err == nil || !strings.Contains(err.Error(), "正被方案") {
		t.Fatalf("空白绑定应阻止删除，实际 %v", err)
	}
}
