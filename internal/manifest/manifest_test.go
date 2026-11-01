package manifest

import (
	"context"
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
	for i := 0; i < 7; i++ {
		// use timestamp-like names
		fname := "mysrc-2026010" + string(rune('0'+i)) + ".yaml"
		os.WriteFile(filepath.Join(tmp, fname), []byte("x"), 0o644)
	}
	// create 7 versioned files with sortable names
	for i := 0; i < 7; i++ {
		fname := "mysrc-2026010" + string(rune('0'+i)) + "0000.yaml"
		os.WriteFile(filepath.Join(tmp, fname), []byte("x"), 0o644)
	}
	pruneVersions(tmp, "mysrc", "yaml", 3)
	versions, _ := ListVersions(tmp, "mysrc", "yaml")
	if len(versions) != 3 {
		t.Fatalf("versions = %d, want 3: %v", len(versions), versions)
	}
}

func TestRollback_InsufficientVersions(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "mysrc-20260101.yaml"), []byte("x"), 0o644)
	_, err := Rollback(tmp, "mysrc", "yaml")
	if err == nil {
		t.Fatal("expected error for insufficient versions")
	}
}

func TestRollback_ReturnsPrevious(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "mysrc-20260101.yaml"), []byte("v1"), 0o644)
	os.WriteFile(filepath.Join(tmp, "mysrc-20260102.yaml"), []byte("v2"), 0o644)
	p, err := Rollback(tmp, "mysrc", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, "mysrc-20260101.yaml") {
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
