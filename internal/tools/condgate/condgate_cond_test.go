package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// condgate_cond_test.go — 条件覆盖补充（不改既有 main_test.go）

func TestRunArgsHelp(t *testing.T) {
	// main.go:106 未覆盖：flag.ErrHelp 分支
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help 应返回 0, got %d", code)
	}
}

func TestRunArgsBadEngine(t *testing.T) {
	// main.go:112 未覆盖：非法 engine
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "bogus", "-min", "80", "-profile", "x.out"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("bad engine 应返回 2, got %d", code)
	}
}

func TestRunArgsGobco(t *testing.T) {
	// gobco 引擎：缺少包模式应返回 2
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "gobco", "-min", "80"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("gobco 缺少包模式应返回 2, got %d", code)
	}
}

func TestRunArgsMissingMin(t *testing.T) {
	// main.go:120 未覆盖：缺少 -min
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-profile", "x.out"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("missing min 应返回 2, got %d", code)
	}
}

func TestRunArgsBadMin(t *testing.T) {
	// main.go:125 未覆盖：非法 -min
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "abc", "-profile", "x.out"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("bad min 应返回 2, got %d", code)
	}
}

func TestRunArgsMissingProfile(t *testing.T) {
	// main.go:129 未覆盖：缺少 -profile
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("missing profile 应返回 2, got %d", code)
	}
}

func TestRunArgsProfileOpenFail(t *testing.T) {
	// main.go:135 未覆盖：读取 profile 失败
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80", "-profile", filepath.Join(t.TempDir(), "nope.out")}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("profile open fail 应返回 2, got %d", code)
	}
}

func TestRunArgsProfileParseFail(t *testing.T) {
	// main.go:141 未覆盖：profile 解析失败
	p := filepath.Join(t.TempDir(), "bad.out")
	os.WriteFile(p, []byte("not a profile\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80", "-profile", p}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("profile parse fail 应返回 2, got %d", code)
	}
}

func TestRunArgsReadModuleFail(t *testing.T) {
	// main.go:149 未覆盖：读取 go.mod 失败（root 无 go.mod，module 未指定）
	dir := t.TempDir()
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\n"+dir+"/a.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80", "-profile", prof, "-root", dir}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("readModule fail 应返回 2, got %d", code)
	}
}

func TestRunArgsReadSourceFail(t *testing.T) {
	// main.go:162 未覆盖：读取源码失败
	dir := t.TempDir()
	// create go.mod
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/missing.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80", "-profile", prof, "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("read source fail 应返回 2, got %d", code)
	}
}

func TestRunArgsParseSourceFail(t *testing.T) {
	// main.go:168 未覆盖：解析源码失败
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "bad.go"), []byte("this is not go"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/bad.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80", "-profile", prof, "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("parse source fail 应返回 2, got %d", code)
	}
}

func TestRunArgsTopDefault(t *testing.T) {
	// main.go:296 未覆盖：n <= 0 分支 (top 默认 0 → len(pkgs))
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "0", "-profile", prof, "-root", dir, "-module", "example.com/m", "-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("top default 应返回 0, got %d: %s", code, stderr.String())
	}
}

func TestRunArgsTopTooBig(t *testing.T) {
	// main.go:296 未覆盖：n > len(pkgs) 分支
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "0", "-profile", prof, "-root", dir, "-module", "example.com/m", "-top", "999"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("top too big 应返回 0, got %d: %s", code, stderr.String())
	}
}

func TestRunArgsJsonOutput(t *testing.T) {
	// main.go:242 未覆盖：json 输出分支
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "0", "-profile", prof, "-root", dir, "-module", "example.com/m", "-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("json 应返回 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("percent")) {
		t.Fatalf("json 输出应含 percent")
	}
}

func TestRunArgsTotalZeroMinPositive(t *testing.T) {
	// main.go:222 未覆盖：total==0 && minVal>0
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "80", "-profile", prof, "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("total zero min positive 应返回 1, got %d", code)
	}
}

func TestRunArgsUnknownFlag(t *testing.T) {
	// main.go:106 未覆盖：fs.Parse 返回非 help 错误（假臂）
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("unknown flag 应返回 2, got %d", code)
	}
}

func TestRunArgsNoModuleValidMod(t *testing.T) {
	// main.go:149 未覆盖：readModule 成功（假臂，err==nil）
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:1.1,2.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-min", "0", "-profile", prof, "-root", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("no module valid mod 应返回 0, got %d: %s", code, stderr.String())
	}
}

func TestRunArgsGobcoCommaPackages(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	var got []string
	listPackages = func(patterns []string) ([]string, error) { got = patterns; return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return nil, nil }
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "gobco", "-packages", "./internal/a/, ./internal/b/", "-min", "0", "-json", "-tmpdir", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d: %s", code, stderr.String())
	}
	if len(got) != 2 || got[0] != "./internal/a/" || got[1] != "./internal/b/" {
		t.Fatalf("patterns got %v", got)
	}
}

func TestRunArgsGobcoPositionalAfterFlags(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	var got []string
	listPackages = func(patterns []string) ([]string, error) { got = patterns; return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return nil, nil }
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "gobco", "-min", "0", "-json", "-tmpdir", t.TempDir(), "./internal/x/", "./internal/y/"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d: %s", code, stderr.String())
	}
	if len(got) != 2 || got[0] != "./internal/x/" || got[1] != "./internal/y/" {
		t.Fatalf("patterns got %v", got)
	}
}
