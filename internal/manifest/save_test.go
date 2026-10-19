package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadManifestRawKeepsRaw 覆盖 LoadManifestRaw 不校验、不补默认、不改写路径的语义。
func TestLoadManifestRawKeepsRaw(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n" +
		"  - url: https://example.com/a.ini\n" +
		"output:\n" +
		"  format: text\n" +
		"  path: relative/out.txt\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifestRaw(path)
	if err != nil {
		t.Fatalf("LoadManifestRaw: %v", err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("sources 数 = %d, want 1", len(m.Sources))
	}
	if m.Sources[0].Name != "" {
		t.Fatalf("Raw 不应补默认 name，got %q", m.Sources[0].Name)
	}
	if filepath.IsAbs(m.Output.Path) {
		t.Fatalf("Raw 不应把 output.path 转绝对: %s", m.Output.Path)
	}
	if _, err := LoadManifestRaw(filepath.Join(dir, "ghost.yaml")); err == nil {
		t.Fatal("缺文件应报错")
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("sources: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifestRaw(filepath.Join(dir, "bad.yaml")); err == nil {
		t.Fatal("坏 YAML 应报错")
	}
}

// TestSaveManifestRoundTrip 覆盖写回、旧文件 .bak 备份与目录创建。
func TestSaveManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "manifest.yaml")
	m := &Manifest{Sources: []Source{{Name: "a", URL: "https://example.com/a.ini"}}, Output: Output{Format: "text", Path: "out/a.txt"}}
	if err := SaveManifest(path, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "https://example.com/a.ini") {
		t.Fatalf("写回内容缺 URL: %s", raw)
	}
	back, err := LoadManifestRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Sources) != 1 || back.Sources[0].Name != "a" {
		t.Fatalf("round-trip 不一致: %+v", back.Sources)
	}
	if err := SaveManifest(path, &Manifest{Sources: []Source{{Name: "b", URL: "https://example.com/b.ini"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("旧文件应备份为 .bak: %v", err)
	}
	if err := SaveManifest(path, nil); err == nil {
		t.Fatal("nil manifest 应报错")
	}
}

// TestDeriveSourceName 与 applyDefaults 的 urlBasename 规则一致。
func TestDeriveSourceName(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://example.com/acl.ini", "acl"},
		{"https://example.com/rules.txt", "rules"},
		{"https://example.com/", "example"},
	}
	for _, c := range cases {
		if got := DeriveSourceName(c.url); got != c.want {
			t.Errorf("DeriveSourceName(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// TestValidateForWrite 复用读取侧校验的三条硬规则。
func TestValidateForWrite(t *testing.T) {
	if err := (&Manifest{}).ValidateForWrite(); err == nil {
		t.Fatal("空 sources 应报错")
	}
	one := &Manifest{Sources: []Source{{Name: "a"}}}
	if err := one.ValidateForWrite(); err == nil {
		t.Fatal("URL 为空应报错")
	}
	dup := &Manifest{Sources: []Source{{Name: "a", URL: "u1"}, {Name: "a", URL: "u2"}}}
	if err := dup.ValidateForWrite(); err == nil {
		t.Fatal("重名应报错")
	}
	ok := &Manifest{Sources: []Source{{Name: "a", URL: "u"}}}
	if err := ok.ValidateForWrite(); err != nil {
		t.Fatalf("合法 manifest 不应报错: %v", err)
	}
}
