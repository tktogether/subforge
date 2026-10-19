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

// --- INI 集成测试 ---

func TestUpdate_INI_Source_TextFormat(t *testing.T) {
	// httptest server serves an INI with [custom] + ruleset= pointing to a .list
	listSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DOMAIN-SUFFIX,google.com,PROXY\nDOMAIN-SUFFIX,youtube.com,PROXY\n"))
	}))
	defer listSrv.Close()

	iniSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[custom]\nruleset=Google," + listSrv.URL + "\n"))
	}))
	defer iniSrv.Close()

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "rules")
	m := Manifest{
		Sources: []Source{{Name: "ini1", URL: iniSrv.URL, Type: "clash", InputFormat: "ini"}},
		Output:  Output{Format: "text", Path: outDir},
	}
	m.applyDefaults()

	res, err := m.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	// text format → .txt extension
	if !strings.HasSuffix(res[0].Path, ".txt") {
		t.Fatalf("path = %q, want .txt suffix", res[0].Path)
	}
	b, _ := os.ReadFile(res[0].Path)
	content := string(b)
	if !strings.Contains(content, "DOMAIN-SUFFIX,google.com,PROXY") {
		t.Fatalf("content missing google rule: %s", content)
	}
	if !strings.Contains(content, "DOMAIN-SUFFIX,youtube.com,PROXY") {
		t.Fatalf("content missing youtube rule: %s", content)
	}
}

func TestUpdate_INI_Source_ClashMetaFormat(t *testing.T) {
	listSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DOMAIN-SUFFIX,google.com,PROXY\n"))
	}))
	defer listSrv.Close()

	iniSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[custom]\nruleset=Google," + listSrv.URL + "\n"))
	}))
	defer iniSrv.Close()

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "rules")
	m := Manifest{
		Sources: []Source{{Name: "ini1", URL: iniSrv.URL, Type: "clash", InputFormat: "ini"}},
		Output:  Output{Format: "clash-meta", Path: outDir},
	}
	m.applyDefaults()

	res, err := m.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	// clash-meta → .yaml extension
	if !strings.HasSuffix(res[0].Path, ".yaml") {
		t.Fatalf("path = %q, want .yaml suffix", res[0].Path)
	}
	b, _ := os.ReadFile(res[0].Path)
	content := string(b)
	if !strings.Contains(content, "rules:") {
		t.Fatalf("content missing rules: key: %s", content)
	}
	if !strings.Contains(content, "DOMAIN-SUFFIX,google.com,PROXY") {
		t.Fatalf("content missing rule: %s", content)
	}
}

func TestUpdate_AutoSniff_INI(t *testing.T) {
	// auto (empty input_format) + content starts with [custom] → should expand
	listSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DOMAIN-SUFFIX,google.com,PROXY\n"))
	}))
	defer listSrv.Close()

	iniSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[custom]\nruleset=Google," + listSrv.URL + "\n"))
	}))
	defer iniSrv.Close()

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "rules")
	m := Manifest{
		Sources: []Source{{Name: "ini1", URL: iniSrv.URL, Type: "clash"}}, // no InputFormat → auto
		Output:  Output{Format: "text", Path: outDir},
	}
	m.applyDefaults()

	res, err := m.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	b, _ := os.ReadFile(res[0].Path)
	if !strings.Contains(string(b), "DOMAIN-SUFFIX,google.com,PROXY") {
		t.Fatalf("auto sniff should expand ini: %s", b)
	}
}

func TestUpdate_AutoSniff_NotINI(t *testing.T) {
	// auto + content is plain YAML → should NOT expand, write raw
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
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	b, _ := os.ReadFile(res[0].Path)
	// Should be raw content, not expanded
	if !strings.Contains(string(b), "DOMAIN,example.com,PROXY") {
		t.Fatalf("non-ini source should write raw content: %s", b)
	}
}

func TestUpdate_NonINI_ExplicitFormat(t *testing.T) {
	// input_format explicitly set to something else (e.g. "yaml") → no expansion
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("rules:\n  - DOMAIN,example.com,PROXY\n"))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "rules")
	m := Manifest{
		Sources: []Source{{Name: "src1", URL: srv.URL + "/a.yaml", Type: "clash", InputFormat: "yaml"}},
		Output:  Output{Format: "clash-meta", Path: outDir},
	}
	m.applyDefaults()

	res, err := m.Update(context.Background())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
	b, _ := os.ReadFile(res[0].Path)
	if !strings.Contains(string(b), "DOMAIN,example.com,PROXY") {
		t.Fatalf("explicit yaml format should not expand: %s", b)
	}
}

func TestExtForFormat_Text(t *testing.T) {
	if extForFormat("text") != "txt" {
		t.Fatalf("extForFormat(text) = %q, want txt", extForFormat("text"))
	}
	if extForFormat("txt") != "txt" {
		t.Fatalf("extForFormat(txt) = %q, want txt", extForFormat("txt"))
	}
	if extForFormat("plain") != "txt" {
		t.Fatalf("extForFormat(plain) = %q, want txt", extForFormat("plain"))
	}
}

func TestShouldExpandINI(t *testing.T) {
	iniData := []byte("[custom]\nruleset=A,https://a.list\n")
	yamlData := []byte("rules:\n  - DOMAIN,example.com,PROXY\n")

	if !shouldExpandINI("ini", yamlData) {
		t.Fatal("ini format should always expand")
	}
	if !shouldExpandINI("auto", iniData) {
		t.Fatal("auto + ini content should expand")
	}
	if shouldExpandINI("auto", yamlData) {
		t.Fatal("auto + yaml content should not expand")
	}
	if !shouldExpandINI("", iniData) {
		t.Fatal("empty (default auto) + ini content should expand")
	}
	if shouldExpandINI("yaml", iniData) {
		t.Fatal("explicit yaml should not expand")
	}
}
