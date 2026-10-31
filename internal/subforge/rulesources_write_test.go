package subforge

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/manifest"
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
