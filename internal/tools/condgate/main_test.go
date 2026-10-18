package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ===== A. 纯函数直测 =====

func TestGatePass(t *testing.T) {
	cases := []struct {
		pct, min float64
		want     bool
	}{
		{100, 100, true},
		{79.999999999, 80, true},
		{79, 80, false},
		{0, 0, true},
	}
	for _, c := range cases {
		if got := GatePass(c.pct, c.min); got != c.want {
			t.Fatalf("GatePass(%v,%v)=%v want %v", c.pct, c.min, got, c.want)
		}
	}
}

func TestPercentOf(t *testing.T) {
	if got := percentOf(0, 0); got != 0 {
		t.Fatalf("除零应返回 0, got %v", got)
	}
	if got := percentOf(1, 2); got != 50 {
		t.Fatalf("percentOf(1,2)=%v want 50", got)
	}
	if got := percentOf(3, 4); got != 75 {
		t.Fatalf("percentOf(3,4)=%v want 75", got)
	}
}

func TestPackageOf(t *testing.T) {
	if got := packageOf("github.com/a/b/c.go"); got != "github.com/a/b" {
		t.Fatalf("got %q", got)
	}
	if got := packageOf(`github.com\\a\\b\\c.go`); got != "github.com/a/b" {
		t.Fatalf("反斜杠 got %q", got)
	}
}

func TestMapPath(t *testing.T) {
	mod := "github.com/x/y"
	root := "r"
	got := mapPath(mod+"/internal/foo.go", mod, root)
	want := filepath.Join(root, "internal", "foo.go")
	if got != want {
		t.Fatalf("prefix got %q want %q", got, want)
	}
	got = mapPath("other/pkg/file.go", mod, root)
	want = filepath.Join(root, filepath.FromSlash("other/pkg/file.go"))
	if got != want {
		t.Fatalf("non-prefix got %q want %q", got, want)
	}
}

func TestReadModule(t *testing.T) {
	dir := t.TempDir()
	gomod := filepath.Join(dir, "go.mod")
	os.WriteFile(gomod, []byte("module example.com/m\n\ngo 1.26\n"), 0644)
	got, err := readModule(dir)
	if err != nil || got != "example.com/m" {
		t.Fatalf("readModule 正常: got %q err=%v", got, err)
	}
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "go.mod"), []byte("go 1.26\n"), 0644)
	_, err = readModule(dir2)
	if err == nil || err.Error() != "go.mod 缺少 module 行" {
		t.Fatalf("缺 module 行 err=%v", err)
	}
	dir3 := t.TempDir()
	_, err = readModule(dir3)
	if err == nil {
		t.Fatal("go.mod 不存在应返回错误")
	}
}

func TestUniqueFiles(t *testing.T) {
	p := &Profile{Blocks: []Block{
		{File: "b.go"}, {File: "a.go"}, {File: "b.go"}, {File: "c.go"},
	}}
	got := uniqueFiles(p)
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
	got2 := uniqueFiles(&Profile{})
	if len(got2) != 0 {
		t.Fatalf("空 Profile got %v", got2)
	}
}

func TestBlocksForFile(t *testing.T) {
	p := &Profile{Blocks: []Block{
		{File: "a.go", StartLine: 1}, {File: "b.go", StartLine: 2}, {File: "a.go", StartLine: 3},
	}}
	got := blocksForFile(p, "a.go")
	if len(got) != 2 || got[0].StartLine != 1 || got[1].StartLine != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestFirstBlockInRange(t *testing.T) {
	blocks := []Block{
		{StartLine: 5, StartCol: 10}, {StartLine: 3, StartCol: 20}, {StartLine: 3, StartCol: 5},
	}
	b := firstBlockInRange(blocks, 1, 10)
	if b == nil || b.StartLine != 3 || b.StartCol != 5 {
		t.Fatalf("got %+v", b)
	}
	if firstBlockInRange(blocks, 100, 200) != nil {
		t.Fatal("应返回 nil")
	}
}

func TestFindNextBlockAfter(t *testing.T) {
	blocks := []Block{
		{StartLine: 5}, {StartLine: 8}, {StartLine: 12},
	}
	b := findNextBlockAfter(blocks, 6, 0)
	if b == nil || b.StartLine != 8 {
		t.Fatalf("got %+v", b)
	}
	b = findNextBlockAfter(blocks, 6, 10)
	if b == nil || b.StartLine != 8 {
		t.Fatalf("funcEnd 限制 got %+v", b)
	}
	if findNextBlockAfter(blocks, 6, 7) != nil {
		t.Fatalf("funcEnd=7 应排除 line 8")
	}
	if findNextBlockAfter(blocks, 20, 0) != nil {
		t.Fatal("应返回 nil")
	}
}

func TestParsePos(t *testing.T) {
	l, c, err := parsePos("12.34")
	if err != nil || l != 12 || c != 34 {
		t.Fatalf("got %d,%d err=%v", l, c, err)
	}
	if _, _, err := parsePos("0.1"); err == nil {
		t.Fatal("行号 0 应报错")
	}
	if _, _, err := parsePos("12.0"); err == nil {
		t.Fatal("列号 0 应报错")
	}
	if _, _, err := parsePos("abc.1"); err == nil {
		t.Fatal("非数字行号应报错")
	}
	if _, _, err := parsePos("12"); err == nil {
		t.Fatal("无点应报错")
	}
}

func TestParseLine(t *testing.T) {
	b, err := parseLine("github.com/x/y/file.go:12.34,15.2 3 1")
	if err != nil {
		t.Fatalf("parseLine 合法: %v", err)
	}
	if b.File != "github.com/x/y/file.go" || b.StartLine != 12 || b.StartCol != 34 || b.EndLine != 15 || b.EndCol != 2 || b.NumStmt != 3 || b.Count != 1 {
		t.Fatalf("Block 不符: %+v", b)
	}
	if _, err := parseLine("only one"); err == nil {
		t.Fatal("字段数不对应报错")
	}
	if _, err := parseLine("noColonHere 3 1"); err == nil {
		t.Fatal("位置非法应报错")
	}
	if _, err := parseLine("f.go:1.1,2.1 x 1"); err == nil {
		t.Fatal("NumStmt 非数字应报错")
	}
	if _, err := parseLine("f.go:1.1,2.1 3 x"); err == nil {
		t.Fatal("Count 非数字应报错")
	}
}

func TestParseProfile(t *testing.T) {
	_, err := ParseProfile(strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "profile 为空") {
		t.Fatalf("空输入 err=%v", err)
	}
	_, err = ParseProfile(strings.NewReader("garbage\n"))
	if err == nil || !strings.Contains(err.Error(), "profile 首行非法") {
		t.Fatalf("首行非法 err=%v", err)
	}
	_, err = ParseProfile(strings.NewReader("mode: bogus\n"))
	if err == nil {
		t.Fatal("非法 mode 应报错")
	}
	for _, m := range []string{"set", "count", "atomic"} {
		in := "mode: " + m + "\nf.go:1.1,2.1 1 1\n"
		p, err := ParseProfile(strings.NewReader(in))
		if err != nil || p.Mode != m || len(p.Blocks) != 1 {
			t.Fatalf("mode %s: err=%v p=%+v", m, err, p)
		}
	}
	p, err := ParseProfile(strings.NewReader("mode: atomic\r\n\r\nf.go:1.1,2.1 1 1\r\n"))
	if err != nil || p.Mode != "atomic" || len(p.Blocks) != 1 {
		t.Fatalf("CRLF/空行: err=%v p=%+v", err, p)
	}
}

// ===== B. AST 分析（visitNode 及 6 个 handle*）=====

func parseSource(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return fset, f
}

func makeProfile(blocks []Block) string {
	var sb strings.Builder
	sb.WriteString("mode: count\n")
	for _, b := range blocks {
		sb.WriteString("test.go:")
		sb.WriteString(itoa(b.StartLine))
		sb.WriteString(".1,")
		sb.WriteString(itoa(b.EndLine))
		sb.WriteString(".2 1 ")
		sb.WriteString(itoa(b.Count))
		sb.WriteString("\n")
	}
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func analyze(t *testing.T, src string, blocks []Block) ([]Judgment, int) {
	t.Helper()
	prof, err := ParseProfile(strings.NewReader(makeProfile(blocks)))
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	fset, f := parseSource(t, src)
	js, sk := analyzeFile("test.go", f, fset, prof.Blocks)
	return js, sk
}

func TestVisitNodeNil(t *testing.T) {
	fset := token.NewFileSet()
	var js []Judgment
	sk := 0
	visitNode(nil, fset, nil, "test.go", &js, &sk, 0)
	if len(js) != 0 || sk != 0 {
		t.Fatalf("nil node 应无输出, js=%v sk=%d", js, sk)
	}
}

func TestAnalyzeIf(t *testing.T) {
	src := `package main
	func f(x int) {
		if x > 0 {
			_ = 1
		} else {
			_ = 2
		}
	}`
	js, _ := analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 5, EndLine: 6, Count: 1}})
	if len(js) != 1 || !js[0].Covered || js[0].Kind != "if" {
		t.Fatalf("if else covered: %+v", js)
	}
	js, _ = analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 5, EndLine: 6, Count: 0}})
	if len(js) != 1 || js[0].Covered || js[0].Detail != "只走过真值" {
		t.Fatalf("只真值: %+v", js)
	}
	src2 := `package main
	func f(x int) {
		if x > 0 {
			_ = 1
		}
		_ = 2
	}`
	js, _ = analyze(t, src2, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 6, EndLine: 6, Count: 1}})
	if len(js) != 1 || !js[0].Covered || js[0].Kind != "if" {
		t.Fatalf("if no-else covered: %+v", js)
	}
	src3 := `package main
	func f(x int) {
		if x > 0 {
			_ = 1
		}
	}`
	_, sk := analyze(t, src3, []Block{{StartLine: 3, EndLine: 4, Count: 1}})
	if sk != 1 {
		t.Fatalf("无后续块应 skipped=1, got %d", sk)
	}
	src4 := `package main
	func f(a, b bool) {
		if a && b {
			_ = 1
		}
		_ = 2
	}`
	js, _ = analyze(t, src4, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 6, EndLine: 6, Count: 1}})
	if len(js) != 1 {
		t.Fatalf("a&&b 应 1 个判定, got %d", len(js))
	}
}

func TestClauseBlocks(t *testing.T) {
	clauses := []*ast.CaseClause{}
	w, n := clauseBlocks(clauses, &token.FileSet{}, []Block{})
	if w != 0 || n != 0 {
		t.Fatalf("空 clauses got %d/%d", w, n)
	}
	src := `package main
	func f(x int) {
		switch x {
		case 1:
			_ = 1
		case 2:
			_ = 2
		default:
			_ = 3
		}
	}`
	fset, f2 := parseSource(t, src)
	ccs := []*ast.CaseClause{}
	for _, s := range f2.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.SwitchStmt).Body.List {
		ccs = append(ccs, s.(*ast.CaseClause))
	}
	w, n = clauseBlocks(ccs, fset, []Block{{StartLine: 5, EndLine: 5, Count: 1}})
	if n != 3 {
		t.Fatalf("nonEmpty=%d want 3", n)
	}
	if w != 1 {
		t.Fatalf("walked=%d want 1", w)
	}
}

func TestHandleFor(t *testing.T) {
	src := `package main
	func f() {
		for i := 0; i < 3; i++ {
			_ = i
		}
		_ = 1
	}`
	js, sk := analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 6, EndLine: 6, Count: 1}})
	if len(js) != 1 || !js[0].Covered || js[0].Kind != "for" {
		t.Fatalf("for covered: %+v", js)
	}
	js, sk = analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 6, EndLine: 6, Count: 0}})
	if len(js) != 1 || js[0].Covered || js[0].Detail != "只走过循环体" {
		t.Fatalf("只循环体: %+v", js)
	}
	js, _ = analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 0}, {StartLine: 6, EndLine: 6, Count: 1}})
	if len(js) != 1 || js[0].Covered || js[0].Detail != "只走过循环后" {
		t.Fatalf("只循环后: %+v", js)
	}
	js, sk = analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 1}})
	if sk != 1 {
		t.Fatalf("无 falseBlock 应 skipped=1, got %d", sk)
	}
}

func TestHandleRange(t *testing.T) {
	src := `package main
	func f(s []int) {
		for _, v := range s {
			_ = v
		}
		_ = 1
	}`
	js, _ := analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 1}, {StartLine: 6, EndLine: 6, Count: 1}})
	if len(js) != 1 || !js[0].Covered || js[0].Kind != "range" {
		t.Fatalf("range covered: %+v", js)
	}
	js, _ = analyze(t, src, []Block{{StartLine: 3, EndLine: 4, Count: 0}, {StartLine: 6, EndLine: 6, Count: 1}})
	if len(js) != 1 || js[0].Covered || js[0].Detail != "只走过循环后" {
		t.Fatalf("只循环后: %+v", js)
	}
}

func TestHandleSwitch(t *testing.T) {
	src := `package main
	func f(x int) {
		switch x {
		case 1:
			_ = 1
		case 2:
			_ = 2
		default:
			_ = 3
		}
	}`
	js, _ := analyze(t, src, []Block{{StartLine: 5, EndLine: 5, Count: 1}, {StartLine: 7, EndLine: 7, Count: 1}, {StartLine: 9, EndLine: 9, Count: 1}})
	if len(js) != 1 || !js[0].Covered || js[0].Kind != "switch" {
		t.Fatalf("switch all covered: %+v", js)
	}
	js, _ = analyze(t, src, []Block{{StartLine: 5, EndLine: 5, Count: 1}, {StartLine: 7, EndLine: 7, Count: 0}, {StartLine: 9, EndLine: 9, Count: 1}})
	if len(js) != 1 || js[0].Covered {
		t.Fatalf("switch partial: %+v", js)
	}
	// empty switch body (no cases with body) -> skipped
	src2 := `package main
	func f(x int) {
		switch x {
		}
	}`
	_, sk := analyze(t, src2, []Block{})
	if sk != 1 {
		t.Fatalf("empty switch 应 skipped=1, got %d", sk)
	}
}

func TestHandleTypeSwitch(t *testing.T) {
	src := `package main
	func f(x interface{}) {
		switch x.(type) {
		case int:
			_ = 1
		case string:
			_ = 2
		default:
			_ = 3
		}
	}`
	js, _ := analyze(t, src, []Block{{StartLine: 5, EndLine: 5, Count: 1}, {StartLine: 7, EndLine: 7, Count: 0}, {StartLine: 9, EndLine: 9, Count: 1}})
	if len(js) != 1 || js[0].Covered {
		t.Fatalf("type switch partial: %+v", js)
	}
	js, _ = analyze(t, src, []Block{{StartLine: 5, EndLine: 5, Count: 1}, {StartLine: 7, EndLine: 7, Count: 1}, {StartLine: 9, EndLine: 9, Count: 1}})
	if len(js) != 1 || !js[0].Covered {
		t.Fatalf("type switch all: %+v", js)
	}
}

func TestHandleSelect(t *testing.T) {
	src := `package main
	func f(ch chan int) {
		select {
		case <-ch:
			_ = 1
		default:
			_ = 2
		}
	}`
	js, _ := analyze(t, src, []Block{{StartLine: 5, EndLine: 5, Count: 1}, {StartLine: 7, EndLine: 7, Count: 1}})
	if len(js) != 1 || !js[0].Covered || js[0].Kind != "select" {
		t.Fatalf("select all: %+v", js)
	}
	js, _ = analyze(t, src, []Block{{StartLine: 5, EndLine: 5, Count: 1}, {StartLine: 7, EndLine: 7, Count: 0}})
	if len(js) != 1 || js[0].Covered {
		t.Fatalf("select partial: %+v", js)
	}
	src2 := `package main
	func f() {
		select {}
	}`
	_, sk := analyze(t, src2, []Block{})
	if sk != 1 {
		t.Fatalf("empty select 应 skipped=1, got %d", sk)
	}
}

func TestRunArgs(t *testing.T) {
	dir := t.TempDir()
	// write go.mod
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/m\n\ngo 1.26\n`), 0644)
	// write source
	srcDir := filepath.Join(dir, "pkg")
	os.MkdirAll(srcDir, 0755)
	src := `package pkg

	func F(x int) int {
		if x > 0 {
			return 1
		}
		return 0
	}`
	os.WriteFile(filepath.Join(srcDir, "f.go"), []byte(src), 0644)
	// write profile
	prof := "mode: count\nexample.com/m/pkg/f.go:4.1,5.1 1 1\nexample.com/m/pkg/f.go:7.1,7.1 1 0\n"
	profPath := filepath.Join(dir, "coverage.out")
	os.WriteFile(profPath, []byte(prof), 0644)
	var stdout, stderr bytes.Buffer
	code := runArgs([]string{"-engine", "profile", "-profile", profPath, "-min", "80", "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("期望失败 exit 1, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "") {
	}
	// covered case
	prof2 := "mode: count\nexample.com/m/pkg/f.go:4.1,5.1 1 1\nexample.com/m/pkg/f.go:7.1,7.1 1 1\n"
	os.WriteFile(profPath, []byte(prof2), 0644)
	stdout.Reset()
	stderr.Reset()
	code = runArgs([]string{"-engine", "profile", "-profile", profPath, "-min", "80", "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("期望通过 exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "≥") {
		t.Logf("stdout: %s", stdout.String())
	}
}

func TestRunArgsMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runArgs([]string{}, &stdout, &stderr); code != 2 {
		t.Fatalf("无 args 应 exit 2, got %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runArgs([]string{"-min", "80"}, &stdout, &stderr); code != 2 {
		t.Fatalf("缺 profile 应 exit 2, got %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runArgs([]string{"-profile", "x", "-min", "80", "-engine", "gobco"}, &stdout, &stderr); code != 2 {
		t.Fatalf("gobco 应 exit 2, got %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runArgs([]string{"-profile", "x", "-min", "bad"}, &stdout, &stderr); code != 2 {
		t.Fatalf("bad min 应 exit 2, got %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runArgs([]string{"-profile", "/nonexistent.out", "-min", "80"}, &stdout, &stderr); code != 2 {
		t.Fatalf("不存在 profile 应 exit 2, got %d", code)
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0644)
	srcDir := filepath.Join(dir, "pkg")
	os.MkdirAll(srcDir, 0755)
	src := "package pkg\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	os.WriteFile(filepath.Join(srcDir, "f.go"), []byte(src), 0644)
	prof := "mode: count\nexample.com/m/pkg/f.go:4.1,5.1 1 1\nexample.com/m/pkg/f.go:7.1,7.1 1 1\n"
	profPath := filepath.Join(dir, "coverage.out")
	os.WriteFile(profPath, []byte(prof), 0644)

	// redirect stdout/stderr to temp files
	outF, _ := os.CreateTemp("", "condgate-out-*.txt")
	errF, _ := os.CreateTemp("", "condgate-err-*.txt")
	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = outF
	os.Stderr = errF

	oldArgs := os.Args
	os.Args = []string{"condgate", "-engine", "profile", "-profile", profPath, "-min", "80", "-root", dir, "-module", "example.com/m"}
	code := run()
	os.Args = oldArgs
	os.Stdout = origStdout
	os.Stderr = origStderr

	outF.Close()
	errF.Close()

	if code != 0 {
		t.Fatalf("run() 期望通过 exit 0, got %d", code)
	}
}

func TestRunArgsJSONUncoveredIsComplete(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0644)
	srcDir := filepath.Join(dir, "pkg")
	os.MkdirAll(srcDir, 0755)

	const n = 70
	var src strings.Builder
	src.WriteString("package pkg\n\nfunc F(x int) {\n")
	blocks := []Block{}
	line := 4
	for i := 0; i < n; i++ {
		src.WriteString("    if x > " + itoa(i) + " {\n")
		blocks = append(blocks, Block{StartLine: line + 1, EndLine: line + 1, Count: 0})
		src.WriteString("        x++\n")
		src.WriteString("    }\n")
		blocks = append(blocks, Block{StartLine: line + 3, EndLine: line + 3, Count: 0})
		src.WriteString("    x++\n")
		line += 4
	}
	src.WriteString("}\n")

	os.WriteFile(filepath.Join(srcDir, "f.go"), []byte(src.String()), 0644)

	var prof strings.Builder
	prof.WriteString("mode: count\n")
	for _, b := range blocks {
		prof.WriteString("example.com/m/pkg/f.go:")
		prof.WriteString(itoa(b.StartLine))
		prof.WriteString(".1,")
		prof.WriteString(itoa(b.EndLine))
		prof.WriteString(".2 1 0\n")
	}
	profPath := filepath.Join(dir, "coverage.out")
	os.WriteFile(profPath, []byte(prof.String()), 0644)

	var stdout, stderr bytes.Buffer
	runArgs([]string{"-engine", "profile", "-profile", profPath, "-min", "0", "-root", dir, "-module", "example.com/m", "-json"}, &stdout, &stderr)
	var r reportJSON
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatalf("json unmarshal: %v; stdout=%s; stderr=%s", err, stdout.String(), stderr.String())
	}
	if len(r.Uncovered) <= textUncoveredLimit {
		t.Fatalf("JSON uncovered %d <= %d", len(r.Uncovered), textUncoveredLimit)
	}
	if len(r.Uncovered) != r.Total-r.Covered {
		t.Fatalf("invariant broken: uncovered=%d total-covered=%d", len(r.Uncovered), r.Total-r.Covered)
	}

	stdout.Reset()
	stderr.Reset()
	runArgs([]string{"-engine", "profile", "-profile", profPath, "-min", "0", "-root", dir, "-module", "example.com/m"}, &stdout, &stderr)
	out := stdout.String()
	cnt := 0
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "  ") && strings.Contains(ln, "语句") {
			cnt++
		}
	}
	if cnt != textUncoveredLimit {
		t.Fatalf("text 明细行数=%d want %d; out=%s", cnt, textUncoveredLimit, out)
	}
	if !strings.Contains(out, "还有 ") || !strings.Contains(out, "条未列出") {
		t.Fatalf("缺少 '还有 ... 条未列出' 提示; out=%s", out)
	}
}

// TestClauseBlocksMatchesMultiLineCaseBody 验证多行 case 体的 switch 臂匹配。
// Go cover 把 case 体块的 StartLine 标在 case 关键字行，而体首语句在下一行；
// clauseBlocks 必须用 case 子句自身的位置行作为下界，否则永远匹配不到。
func TestClauseBlocksMatchesMultiLineCaseBody(t *testing.T) {
	src := `package p

func f(name string) string {
	switch {
	case name == "a":
		return "A"
	case name == "b":
		return "B"
	default:
		return "C"
	}
}
`
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := astFile.Decls[0].(*ast.FuncDecl)
	ss := fd.Body.List[0].(*ast.SwitchStmt)

	// 合成 profile 块：每个 case 体块的 StartLine = case 关键字所在行，Count=1。
	var blocks []Block
	for _, s := range ss.Body.List {
		cc := s.(*ast.CaseClause)
		if len(cc.Body) == 0 {
			continue
		}
		line := fset.Position(cc.Pos()).Line
		blocks = append(blocks, Block{
			File:      "x.go",
			StartLine: line,
			StartCol:  1,
			EndLine:   fset.Position(cc.Body[len(cc.Body)-1].End()).Line,
			EndCol:    1,
			NumStmt:   1,
			Count:     1,
		})
	}
	clauses := []*ast.CaseClause{}
	for _, s := range ss.Body.List {
		clauses = append(clauses, s.(*ast.CaseClause))
	}
	walked, nonEmpty := clauseBlocks(clauses, fset, blocks)
	if walked != 3 || nonEmpty != 3 {
		t.Fatalf("clauseBlocks = (%d,%d), want (3,3)", walked, nonEmpty)
	}
}
