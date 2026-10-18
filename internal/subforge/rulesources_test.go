package subforge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ruleSourceHandler 返回一段可被 parse.Sniff 识别的规则文本。
func ruleSourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("rules:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n"))
	}
}

// writeRuleManifest 在临时目录写一个最小 manifest.yaml 并把它配成当前 manifest 路径。
func writeRuleManifest(t *testing.T, m *Manager, url, format, outDir string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	content := "sources:\n" +
		"  - name: a\n" +
		"    url: " + url + "\n" +
		"output:\n" +
		"  format: " + format + "\n" +
		"  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := m.SetRuleManifestPath(context.Background(), path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	return path
}

// seedVersions 在输出目录造两个版本文件。
func seedVersions(t *testing.T, dir, name, ext string) []string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	var names []string
	for _, v := range []string{"20240101-000000", "20240102-000000"} {
		n := name + "-" + v + "." + ext
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("写版本文件: %v", err)
		}
		names = append(names, n)
	}
	return names
}

// TestGetSetRuleManifestPath 覆盖未配置返回空串与 Set/Get 往返。
func TestGetSetRuleManifestPath(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	got, err := m.GetRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("GetRuleManifestPath: %v", err)
	}
	if got != "" {
		t.Fatalf("未配置应返回空串, got %q", got)
	}
	want := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, want); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	got2, err := m.GetRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("GetRuleManifestPath 二次: %v", err)
	}
	if got2 != want {
		t.Fatalf("Get = %q, want %q", got2, want)
	}
}

// TestLoadRuleManifestUnconfigured 覆盖未配置 manifest 路径时报错。
func TestLoadRuleManifestUnconfigured(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, err := m.LoadRuleManifest(ctx)
	if err == nil {
		t.Fatal("未配置时应报错")
	}
	if !strings.Contains(err.Error(), "未配置规则源清单路径") {
		t.Fatalf("错误文本 = %q", err.Error())
	}
}

// TestLoadRuleManifestMissingFile 覆盖配置了不存在的文件时报错。
func TestLoadRuleManifestMissingFile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.SetRuleManifestPath(ctx, filepath.Join(t.TempDir(), "nope.yaml")); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	_, err := m.LoadRuleManifest(ctx)
	if err == nil {
		t.Fatal("文件不存在时应报错")
	}
	if !strings.Contains(err.Error(), "读取规则源清单") {
		t.Fatalf("错误文本 = %q", err.Error())
	}
}

// TestLoadRuleManifestOK 覆盖正常解析最小 manifest。
func TestLoadRuleManifestOK(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		t.Fatalf("LoadRuleManifest: %v", err)
	}
	if len(mf.Sources) != 1 {
		t.Fatalf("Sources 长度 = %d, want 1", len(mf.Sources))
	}
	if mf.Sources[0].SourceName() != "a" {
		t.Errorf("SourceName = %q, want a", mf.Sources[0].SourceName())
	}
}

// TestListRuleSourcesEmpty 覆盖无版本时 Versions 为空、Latest 为空串（输出目录不存在也不中断）。
func TestListRuleSourcesEmpty(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	statuses, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len = %d, want 1", len(statuses))
	}
	if len(statuses[0].Versions) != 0 {
		t.Fatalf("Versions = %v, want 空", statuses[0].Versions)
	}
	if statuses[0].Versions == nil {
		t.Errorf("Versions 应为非 nil 空切片")
	}
	if statuses[0].Latest != "" {
		t.Fatalf("Latest = %q, want 空串", statuses[0].Latest)
	}
	if statuses[0].Name != "a" || statuses[0].Format != "text" {
		t.Errorf("status = %+v", statuses[0])
	}
}

// TestListRuleSourcesWithVersions 覆盖有版本时 Latest == Versions[0]。
func TestListRuleSourcesWithVersions(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	seedVersions(t, outDir, "a", "txt")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	statuses, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len = %d, want 1", len(statuses))
	}
	if len(statuses[0].Versions) != 2 {
		t.Fatalf("Versions = %v, want 2 个", statuses[0].Versions)
	}
	if statuses[0].Latest != statuses[0].Versions[0] {
		t.Fatalf("Latest = %q, want Versions[0] = %q", statuses[0].Latest, statuses[0].Versions[0])
	}
	for _, v := range statuses[0].Versions {
		if !strings.HasSuffix(v, ".txt") {
			t.Errorf("版本文件名 %q 应以 .txt 结尾", v)
		}
	}
}

// TestUpdateRuleSources 覆盖一次全量更新：输出目录出现文件且 Err 为 nil。
func TestUpdateRuleSources(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	results, err := m.UpdateRuleSources(ctx)
	if err != nil {
		t.Fatalf("UpdateRuleSources: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results 长度 = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("UpdateResult.Err = %v", results[0].Err)
	}
	if _, err := os.Stat(results[0].Path); err != nil {
		t.Fatalf("输出文件不存在: %v", err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("输出目录文件数 = %d, want 1", len(entries))
	}
}

// TestRollbackRuleSourceUnknown 覆盖 name 不存在时报错且错误含该 name。
func TestRollbackRuleSourceUnknown(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	_, err := m.RollbackRuleSource(ctx, "ghost")
	if err == nil {
		t.Fatal("未知源应报错")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("错误文本 = %q, 应含 ghost", err.Error())
	}
}

// TestRollbackRuleSourceOK 覆盖回滚成功且返回路径存在。
func TestRollbackRuleSourceOK(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	seedVersions(t, outDir, "a", "txt")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	path, err := m.RollbackRuleSource(ctx, "a")
	if err != nil {
		t.Fatalf("RollbackRuleSource: %v", err)
	}
	if path == "" {
		t.Fatal("回滚路径为空")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("回滚路径不存在: %v", err)
	}
	okExt := false
	for _, e := range []string{".txt", ".yaml", ".mrs"} {
		if strings.HasSuffix(path, e) {
			okExt = true
		}
	}
	if !okExt {
		t.Errorf("回滚路径 %q 扩展名不在 .txt/.yaml/.mrs 中", path)
	}
}
