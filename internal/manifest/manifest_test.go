package manifest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest_Valid(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "m.yaml")
	content := `sources:
  - name: "ACL4SSR"
    url: "https://example.com/rules.yaml"
    type: "clash"
    update_interval: "24h"
  - url: "https://example.com/other.yaml"
output:
  format: "clash-meta"
  path: "./rules"
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(m.Sources))
	}
	if m.Sources[0].SourceName() != "ACL4SSR" {
		t.Errorf("name[0] = %q, want ACL4SSR", m.Sources[0].SourceName())
	}
	if m.Sources[1].SourceName() != "other" {
		t.Errorf("name[1] = %q, want other", m.Sources[1].SourceName())
	}
	if m.Output.Format != "clash-meta" {
		t.Errorf("format = %q", m.Output.Format)
	}
}

func TestLoadManifest_EmptySources(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "m.yaml")
	if err := os.WriteFile(p, []byte("output:\n  path: ./r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(p)
	if err == nil {
		t.Fatal("expected error for empty sources")
	}
}

func TestLoadManifest_MissingURL(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "m.yaml")
	if err := os.WriteFile(p, []byte("sources:\n  - name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(p)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

// TestUpdate_PassesSourceOutbound 规则源自带的出网选择要传给 ClientFor。
func TestUpdate_PassesSourceOutbound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("rules:\n  - DOMAIN,example.com,PROXY\n"))
	}))
	defer srv.Close()

	m := Manifest{
		Sources: []Source{{Name: "src1", URL: srv.URL + "/a.yaml", Type: "clash", OutboundNode: "direct"}},
		Output:  Output{Format: "clash-meta", Path: filepath.Join(t.TempDir(), "rules")},
	}
	m.applyDefaults()

	var keys []string
	_, err := m.Update(context.Background(), WithClientFor(func(_ context.Context, outbound string) (*http.Client, error) {
		keys = append(keys, outbound)
		return http.DefaultClient, nil
	}))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("ClientFor 未被调用")
	}
	for _, k := range keys {
		if k != "direct" {
			t.Fatalf("出网选择 = %q, want direct（全部 %v）", k, keys)
		}
	}
}

func TestUpdate_WritesVersionedFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("rules:\n  - DOMAIN,example.com,PROXY\n"))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "rules")
	m := Manifest{
		Sources: []Source{{Name: "src1", URL: srv.URL + "/a.yaml", Type: "clash"}},
		Output:  Output{Format: "clash-meta", Path: outDir},
	}
	m.applyDefaults()

	res, err := m.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("results = %d, want 1", len(res))
	}
	if res[0].Err != nil {
		t.Fatalf("result err: %v", res[0].Err)
	}
	if res[0].Path == "" {
		t.Fatal("result path empty")
	}
	if _, err := os.Stat(res[0].Path); err != nil {
		t.Fatalf("file not written: %v", err)
	}
	b, _ := os.ReadFile(res[0].Path)
	if !strings.Contains(string(b), "DOMAIN,example.com") {
		t.Fatalf("content mismatch: %s", b)
	}
}

func TestPruneVersions_Retention(t *testing.T) {
	tmp := t.TempDir()
	// 非版本文件（时间戳不合规）不应参与剪枝。
	for i := 0; i < 7; i++ {
		fname := "mysrc-2026010" + string(rune('0'+i)) + ".yaml"
		if err := os.WriteFile(filepath.Join(tmp, fname), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 7 个合规版本文件，时间戳按顺序递增。
	for i := 0; i < 7; i++ {
		fname := fmt.Sprintf("mysrc-20260101-00000%d.000000000.yaml", i)
		if err := os.WriteFile(filepath.Join(tmp, fname), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruneVersions(tmp, "mysrc", "yaml", 3)
	versions, _ := ListVersions(tmp, "mysrc", "yaml")
	if len(versions) != 3 {
		t.Fatalf("versions = %d, want 3: %v", len(versions), versions)
	}
}

// TestListVersionsPrefixCollision 源名互为前缀时只返回自己的版本文件。
func TestListVersionsPrefixCollision(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{
		"base-20260101-000000.000000000.yaml",
		"base-extra-20260101-000000.000000000.yaml",
	} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ListVersions(tmp, "base", "yaml")
	if err != nil {
		t.Fatalf("ListVersions(base): %v", err)
	}
	if len(got) != 1 || got[0] != "base-20260101-000000.000000000.yaml" {
		t.Errorf("base 版本 = %v，不应包含 base-extra", got)
	}
	got, err = ListVersions(tmp, "base-extra", "yaml")
	if err != nil {
		t.Fatalf("ListVersions(base-extra): %v", err)
	}
	if len(got) != 1 || got[0] != "base-extra-20260101-000000.000000000.yaml" {
		t.Errorf("base-extra 版本 = %v", got)
	}
}

func TestSaveProductKeepsOwnVersionWithPrefixCollision(t *testing.T) {
	tmp := t.TempDir()
	for i := 1; i <= 6; i++ {
		fname := fmt.Sprintf("base-extra-2026010%d-000000.000000000.yaml", i)
		if err := os.WriteFile(filepath.Join(tmp, fname), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := SaveProduct(tmp, "base", "yaml", []byte("new"), 2)
	if err != nil {
		t.Fatalf("SaveProduct: %v", err)
	}
	if _, err := os.Stat(res.Path); err != nil {
		t.Fatalf("刚写入的版本被剪枝删除: %v", err)
	}
	extra, err := ListVersions(tmp, "base-extra", "yaml")
	if err != nil {
		t.Fatalf("ListVersions(base-extra): %v", err)
	}
	if len(extra) != 6 {
		t.Errorf("另一源的版本被误剪枝: %d", len(extra))
	}
}

func TestRollback_InsufficientVersions(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "mysrc-20260101-000000.000000000.yaml"), []byte("x"), 0o644)
	_, err := Rollback(tmp, "mysrc", "yaml")
	if err == nil {
		t.Fatal("expected error for insufficient versions")
	}
}

func TestRollback_ReturnsPrevious(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "mysrc-20260101-000000.000000000.yaml"), []byte("v1"), 0o644)
	os.WriteFile(filepath.Join(tmp, "mysrc-20260102-000000.000000000.yaml"), []byte("v2"), 0o644)
	p, err := Rollback(tmp, "mysrc", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, "mysrc-20260101-000000.000000000.yaml") {
		t.Fatalf("rollback = %s, want previous version", p)
	}
}

func TestLoadManifest_RelativeOutputPathResolved(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n  - name: a\n    url: https://example.com/a.yaml\noutput:\n  format: clash-meta\n  path: ./rules\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	want := filepath.Join(dir, "rules")
	if m.Output.Path != want {
		t.Fatalf("Output.Path = %q, want %q", m.Output.Path, want)
	}
	if !filepath.IsAbs(m.Output.Path) {
		t.Fatalf("Output.Path 应为绝对路径, got %q", m.Output.Path)
	}
}

func TestLoadManifest_AbsoluteOutputPathKept(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "abs-out")
	p := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n  - name: a\n    url: https://example.com/a.yaml\noutput:\n  format: clash-meta\n  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Output.Path != filepath.ToSlash(outDir) {
		t.Fatalf("Output.Path = %q, want %q", m.Output.Path, filepath.ToSlash(outDir))
	}
}

func TestLoadManifest_RelativeOutputPathEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("rules:\n  - DOMAIN,example.com,PROXY\n"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n  - name: rel\n    url: " + srv.URL + "/a.yaml\noutput:\n  format: clash-meta\n  path: ./rules\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	res, err := m.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	wantDir := filepath.Join(dir, "rules")
	if filepath.Dir(res[0].Path) != wantDir {
		t.Fatalf("输出目录 = %q, want %q", filepath.Dir(res[0].Path), wantDir)
	}
	entries, err := os.ReadDir(wantDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("版本文件数 = %d, want 1", len(entries))
	}
}

// TestListVersionsAcceptsLegacyShortStamp Update 历史产物不带纳秒段，
// 版本归属判断必须同时兼容两种命名格式。
func TestListVersionsAcceptsLegacyShortStamp(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{
		"mysrc-20240101-000000.yaml",
		"mysrc-extra-20240102-000000.yaml",
		"mysrc-20240101-000001.000000000.yaml",
	} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ListVersions(tmp, "mysrc", "yaml")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(got) != 2 || got[0] != "mysrc-20240101-000000.yaml" || got[1] != "mysrc-20240101-000001.000000000.yaml" {
		t.Errorf("版本列表 = %v", got)
	}
}

// TestPruneVersionsShortStamp 短时间戳（Update 历史产物）同样参与保留策略，
// 且不误删名字互为前缀的其它源。
func TestPruneVersionsShortStamp(t *testing.T) {
	tmp := t.TempDir()
	for i := 1; i <= 6; i++ {
		name := fmt.Sprintf("a-2024010%d-000000.yaml", i)
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "a-extra-20240101-000000.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	pruneVersions(tmp, "a", "yaml", 2)
	got, err := ListVersions(tmp, "a", "yaml")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("保留数 = %d，期望 2: %v", len(got), got)
	}
	if _, err := os.Stat(filepath.Join(tmp, "a-extra-20240101-000000.yaml")); err != nil {
		t.Errorf("另一源版本被误删: %v", err)
	}
}
