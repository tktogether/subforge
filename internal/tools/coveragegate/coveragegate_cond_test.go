package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// coveragegate_cond_test.go — 条件覆盖补充

func TestRunBadMode(t *testing.T) {
	// main.go:165 未覆盖：switch default（非法 mode）— 通过 profile 触发
	p := writeProfile(t, "mode: bogus\nx.go:1.1,2.2 1 1\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("bad mode 应返回 2, got %d", code)
	}
}

func TestRunMinFuncParseErr(t *testing.T) {
	// main.go:528 未覆盖：-min-func 非法
	p := writeProfile(t, profileAllGreen)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80", "-min-func", "abc"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("min-func parse err 应返回 2, got %d", code)
	}
}

func TestRunMinFuncNaN(t *testing.T) {
	// main.go:528 未覆盖：NaN
	p := writeProfile(t, profileAllGreen)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80", "-min-func", "NaN"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("min-func NaN 应返回 2, got %d", code)
	}
}

func TestRunMinFuncRangeLow(t *testing.T) {
	// main.go:532 未覆盖：v<0
	p := writeProfile(t, profileAllGreen)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80", "-min-func", "-1"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("min-func range low 应返回 2, got %d", code)
	}
}

func TestRunMinFuncRangeHigh(t *testing.T) {
	// main.go:532 未覆盖：v>100
	p := writeProfile(t, profileAllGreen)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80", "-min-func", "101"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("min-func range high 应返回 2, got %d", code)
	}
}

func TestRunMinFuncDetectModuleErr(t *testing.T) {
	// main.go:543 未覆盖：detectModule 失败（无 go.mod，未指定 -module）
	dir := t.TempDir()
	p := writeProfile(t, "mode: set\nx.go:1.1,2.2 1 1\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80", "-min-func", "80", "-root", dir}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("detectModule err 应返回 2, got %d", code)
	}
}

func TestRunMinFuncComputeErr(t *testing.T) {
	// main.go:568 未覆盖：ComputeFuncCoverage 失败（profile 指向不存在源码）
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	p := writeProfile(t, "mode: set\nexample.com/m/missing.go:1.1,2.2 1 1\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", p, "-min", "80", "-min-func", "80", "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("ComputeFuncCoverage err 应返回 2, got %d", code)
	}
}

func TestRunMinFuncFail(t *testing.T) {
	// main.go:574 未覆盖：funcCode != 0（函数覆盖率低于门禁）
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\nfunc G() int { return 2 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	// only cover F, not G -> func coverage < 100
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:3.1,3.2 1 1\nexample.com/m/internal/a.go:4.1,4.2 1 0\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", prof, "-min", "0", "-min-func", "100", "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("min-func fail 应返回 1, got %d: %s", code, stderr.String())
	}
}

func TestRunMinFuncPassText(t *testing.T) {
	// main.go:473,475,466,468 未覆盖：hasMinFunc 文本输出含函数覆盖段
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:3.1,3.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", prof, "-min", "0", "-min-func", "80", "-root", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("min-func pass 应返回 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("函数覆盖")) {
		t.Fatalf("输出应含 函数覆盖")
	}
}

func TestRunMinFuncJson(t *testing.T) {
	// main.go:589 未覆盖：json 路径 hasMinFunc
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:3.1,3.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", prof, "-min", "0", "-min-func", "80", "-root", dir, "-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("json 应返回 0, got %d: %s", code, stderr.String())
	}
}

func TestRunTopFuncDefault(t *testing.T) {
	// main.go:463 未覆盖：nf<=0 默认
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:3.1,3.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", prof, "-min", "0", "-min-func", "80", "-root", dir, "-module", "example.com/m", "-top-func", "0"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("top-func 0 应返回 0, got %d: %s", code, stderr.String())
	}
}

func TestRunTopFuncTooBig(t *testing.T) {
	// main.go:463 未覆盖：nf>len(pkgs)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:3.1,3.2 1 1\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", prof, "-min", "0", "-min-func", "80", "-root", dir, "-module", "example.com/m", "-top-func", "999"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("top-func 999 应返回 0, got %d: %s", code, stderr.String())
	}
}

func TestRunUncoveredFunc(t *testing.T) {
	// main.go:445,452,454 未覆盖：有未覆盖函数时的文本输出
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\nfunc F() int { return 1 }\nfunc G() int { return 2 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:3.1,3.2 1 1\nexample.com/m/internal/a.go:4.1,4.2 1 0\n"), 0o644)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-profile", prof, "-min", "0", "-min-func", "0", "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("uncovered func 应返回 0, got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("未覆盖的函数")) {
		t.Fatalf("输出应含 未覆盖的函数")
	}
}

func TestComputeFuncNonFuncDecl(t *testing.T) {
	// main.go:322 未覆盖：非 *ast.FuncDecl 声明（type/var decl）
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644)
	srcDir := filepath.Join(dir, "internal")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.go"), []byte("package internal\n\ntype Foo int\nvar X = 1\nfunc F() int { return 1 }\n"), 0o644)
	prof := filepath.Join(dir, "c.out")
	os.WriteFile(prof, []byte("mode: set\nexample.com/m/internal/a.go:5.1,5.2 1 1\n"), 0o644)
	fr, err := ComputeFuncCoverage(mustParseProfile(t, prof), dir, "example.com/m")
	if err != nil {
		t.Fatalf("ComputeFuncCoverage err: %v", err)
	}
	if fr.Total.Functions != 1 {
		t.Fatalf("应只有 1 个函数, got %d", fr.Total.Functions)
	}
}

func mustParseProfile(t *testing.T, p string) *Profile {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	prof, err := ParseProfile(f)
	if err != nil {
		t.Fatal(err)
	}
	return prof
}
