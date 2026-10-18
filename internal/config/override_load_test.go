package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const iniSample = `[override.group:AI]
type = select
proxy = HK
proxy = JP
strategy = url-test
test = http://www.gstatic.com/generate_204
lazy = true
[override.rules]
prepend = DOMAIN-SUFFIX,openai.com,AI
append = DOMAIN-SUFFIX,claude.ai,AI
[override.proxy:HK]
server = new-hk.example.com
port = 443
`

const yamlSample = `groups:
  - name: AI
    type: select
    proxies: [HK, JP]
    strategy: url-test
    test: http://www.gstatic.com/generate_204
    lazy: true
rules:
  prepend:
    - DOMAIN-SUFFIX,openai.com,AI
  append:
    - DOMAIN-SUFFIX,claude.ai,AI
proxies:
  - name: HK
    server: new-hk.example.com
    port: "443"
`

func TestLoadOverrideFile_INI(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "override.ini")
	if err := os.WriteFile(p, []byte(iniSample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOverrideFile(p)
	if err != nil {
		t.Fatalf("LoadOverrideFile ini: %v", err)
	}
	want, err := ParseOverrideINI([]byte(iniSample))
	if err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].Name != want.Groups[0].Name || got.Groups[0].Type != want.Groups[0].Type {
		t.Fatalf("ini branch mismatch: got=%+v want=%+v", got, want)
	}
	if len(got.Rules.Prepend) != len(want.Rules.Prepend) {
		t.Fatalf("prepend len mismatch")
	}
}

func TestLoadOverrideFile_YAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "override.yaml")
	if err := os.WriteFile(p, []byte(yamlSample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOverrideFile(p)
	if err != nil {
		t.Fatalf("LoadOverrideFile yaml: %v", err)
	}
	want, err := ParseOverrideYAML([]byte(yamlSample))
	if err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].Name != want.Groups[0].Name || len(got.Proxies) != len(want.Proxies) {
		t.Fatalf("yaml branch mismatch: got=%+v want=%+v", got, want)
	}
}

func TestLoadOverrideFile_UpperExt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.INI")
	if err := os.WriteFile(p, []byte(iniSample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOverrideFile(p)
	if err != nil {
		t.Fatalf("x.INI: %v", err)
	}
	if len(got.Groups) == 0 || got.Groups[0].Name != "AI" {
		t.Fatalf("x.INI did not use INI branch: %+v", got)
	}
}

func TestLoadOverrideFile_NoExt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "override")
	if err := os.WriteFile(p, []byte(yamlSample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOverrideFile(p)
	if err != nil {
		t.Fatalf("no ext: %v", err)
	}
	if len(got.Groups) == 0 || got.Groups[0].Name != "AI" {
		t.Fatalf("no ext did not use YAML branch: %+v", got)
	}
}

func TestLoadOverrideFile_NotExist(t *testing.T) {
	_, err := LoadOverrideFile(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("missing file should error")
	}
	if !strings.Contains(err.Error(), "读取 override 文件") {
		t.Fatalf("error missing phrase: %v", err)
	}
}
