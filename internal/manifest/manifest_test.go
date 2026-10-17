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
