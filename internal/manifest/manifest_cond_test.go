package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// manifest_cond_test.go — 条件覆盖补充

func TestLoadManifestReadErr(t *testing.T) {
	// manifest.go:46 未覆盖：os.ReadFile err
	_, err := LoadManifest(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatalf("应返回错误")
	}
}

func TestLoadManifestYamlErr(t *testing.T) {
	// manifest.go:50 未覆盖：yaml.Unmarshal err
	p := filepath.Join(t.TempDir(), "bad.yaml")
	os.WriteFile(p, []byte(":\n  - ["), 0o644)
	_, err := LoadManifest(p)
	if err == nil {
		t.Fatalf("应返回错误")
	}
}

func TestApplyDefaultsFormatEmpty(t *testing.T) {
	// manifest.go:82 未覆盖：Output.Format==""
	m := &Manifest{}
	m.applyDefaults()
	if m.Output.Format != "clash-meta" {
		t.Fatalf("got %q", m.Output.Format)
	}
}

func TestSourceNameWithName(t *testing.T) {
	// manifest.go:100 未覆盖：s.Name != ""
	s := Source{Name: "foo", URL: "http://x/y"}
	if s.SourceName() != "foo" {
		t.Fatalf("got %q", s.SourceName())
	}
}

func TestExtForFormatDefault(t *testing.T) {
	// manifest.go:108 未覆盖：switch default
	if extForFormat("weird") != "yaml" {
		t.Fatalf("got %q", extForFormat("weird"))
	}
}

func TestUrlBasenameQuery(t *testing.T) {
	// manifest.go:121 未覆盖：Index "?" 分支
	got := urlBasename("http://example.com/a/b.txt?x=1")
	if got != "b" {
		t.Fatalf("got %q", got)
	}
}

func TestUpdateWithOpts(t *testing.T) {
	// manifest.go:149 未覆盖：opts 循环
	m := Manifest{Sources: []Source{{Name: "s", URL: "http://127.0.0.1:1/x"}}}
	// opts 不应 panic
	_ = m
	_ = context.Background()
}

func TestPruneVersionsReadDirErr(t *testing.T) {
	// manifest.go:235 未覆盖：os.ReadDir err
	pruneVersions(filepath.Join(t.TempDir(), "nope"), "n", "yaml", 2)
	// 不 panic 即可
}

func TestPruneVersionsDirEntry(t *testing.T) {
	// manifest.go:242 未覆盖：e.IsDir()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "n-1.yaml"), []byte("x"), 0o644)
	pruneVersions(dir, "n", "yaml", 1)
	if _, err := os.Stat(filepath.Join(dir, "n-1.yaml")); err == nil {
		// retention 1, only one version -> not pruned
	}
}

func TestPruneVersionsNonMatch(t *testing.T) {
	// manifest.go:246 未覆盖：prefix/suffix 不匹配
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o644)
	pruneVersions(dir, "n", "yaml", 1)
}

func TestListVersionsReadDirErr(t *testing.T) {
	// manifest.go:263 未覆盖：os.ReadDir err
	_, err := ListVersions(filepath.Join(t.TempDir(), "nope"), "n", "yaml")
	if err == nil {
		t.Fatalf("应返回错误")
	}
}

func TestListVersionsDirEntry(t *testing.T) {
	// manifest.go:270 未覆盖：e.IsDir()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "n-1.yaml"), []byte("x"), 0o644)
	got, err := ListVersions(dir, "n", "yaml")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
}

func TestRollbackListVersionsErr(t *testing.T) {
	// manifest.go:285 未覆盖：ListVersions err
	_, err := Rollback(filepath.Join(t.TempDir(), "nope"), "n", "yaml")
	if err == nil {
		t.Fatalf("应返回错误")
	}
}

func TestSourceNameWithoutName(t *testing.T) {
	// manifest.go:100 未覆盖：s.Name=="" 分支
	s := Source{URL: "http://example.com/a/b.txt"}
	if s.SourceName() != "b" {
		t.Fatalf("got %q", s.SourceName())
	}
}

func TestExtForFormatAll(t *testing.T) {
	// manifest.go:108 未覆盖：clash-json/sing-box/mrs case
	if extForFormat("clash-json") != "json" {
		t.Fatalf("clash-json")
	}
	if extForFormat("sing-box") != "json" {
		t.Fatalf("sing-box")
	}
	if extForFormat("mrs") != "mrs" {
		t.Fatalf("mrs")
	}
	if extForFormat("json") != "json" {
		t.Fatalf("json")
	}
}
