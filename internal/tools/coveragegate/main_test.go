package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const profileAllGreen = "mode: atomic\n" +
	"github.com/tktogether/subforge/internal/a/a.go:1.1,2.2 4 1\n" +
	"github.com/tktogether/subforge/internal/a/a.go:3.1,4.2 6 2\n" +
	"github.com/tktogether/subforge/internal/b/b.go:1.1,3.2 10 1\n"

const profileHalf = "mode: count\n" +
	"github.com/tktogether/subforge/internal/a/a.go:1.1,2.2 1 1\n" +
	"github.com/tktogether/subforge/internal/a/a.go:3.1,4.2 1 0\n"

func writeProfile(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatalf("写入临时 profile 失败: %v", err)
	}
	return p
}

func TestParseProfile(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantMode   string
		wantBlocks int
		wantFile   string
		wantErr    string
	}{
		{name: "atomic 正常", in: "mode: atomic\nx.go:1.1,2.2 3 1\n", wantMode: "atomic", wantBlocks: 1, wantFile: "x.go"},
		{name: "set 正常", in: "mode: set\nx.go:1.1,2.2 3 0\n", wantMode: "set", wantBlocks: 1},
		{name: "count 正常", in: "mode: count\nx.go:1.1,2.2 3 5\n", wantMode: "count", wantBlocks: 1},
		{name: "空行被忽略", in: "mode: set\n\nx.go:1.1,2.2 1 1\n\n\n", wantMode: "set", wantBlocks: 1},
		{name: "windows 路径", in: "mode: set\nE:\\Projects\\subforge\\internal\\a\\a.go:12.3,14.5 7 2\n", wantMode: "set", wantBlocks: 1, wantFile: "E:\\Projects\\subforge\\internal\\a\\a.go"},
		{name: "空输入", in: "", wantErr: "profile 为空"},
		{name: "仅换行", in: "\n", wantErr: "首行非法"},
		{name: "缺 mode 前缀", in: "atomic\nx.go:1.1,2.2 1 1\n", wantErr: "首行非法"},
		{name: "非法 mode", in: "mode: bogus\nx.go:1.1,2.2 1 1\n", wantErr: "非法 coverage mode"},
		{name: "mode 为空", in: "mode:\n", wantErr: "非法 coverage mode"},
		{name: "列数不足", in: "mode: set\nx.go:1.1,2.2 1\n", wantErr: "期望 3 个字段"},
		{name: "列数过多", in: "mode: set\nx.go:1.1,2.2 1 1 extra\n", wantErr: "期望 3 个字段"},
		{name: "numStmt 非法", in: "mode: set\nx.go:1.1,2.2 x 1\n", wantErr: "numStmt"},
		{name: "numStmt 为负", in: "mode: set\nx.go:1.1,2.2 -1 1\n", wantErr: "numStmt"},
		{name: "count 非法", in: "mode: set\nx.go:1.1,2.2 1 y\n", wantErr: "count"},
		{name: "count 为负", in: "mode: set\nx.go:1.1,2.2 1 -2\n", wantErr: "count"},
		{name: "缺位置分隔符", in: "mode: set\nx.go 1 1\n", wantErr: "位置"},
		{name: "末尾冒号", in: "mode: set\nx.go: 1 1\n", wantErr: "位置"},
		{name: "区间缺逗号", in: "mode: set\nx.go:1.1 1 1\n", wantErr: "位置区间"},
		{name: "区间点多", in: "mode: set\nx.go:1.1,2.2.3 1 1\n", wantErr: "位置"},
		{name: "位置缺小数点", in: "mode: set\nx.go:1,2 1 1\n", wantErr: "位置"},
		{name: "行号为 0", in: "mode: set\nx.go:0.1,2.2 1 1\n", wantErr: "行号"},
		{name: "列号非数字", in: "mode: set\nx.go:1.x,2.2 1 1\n", wantErr: "列号"},
		{name: "列号为 0", in: "mode: set\nx.go:1.0,2.2 1 1\n", wantErr: "列号"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseProfile(strings.NewReader(tc.in))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("期望错误包含 %q，实际 nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("期望错误包含 %q，实际 %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("非预期错误: %v", err)
			}
			if p.Mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", p.Mode, tc.wantMode)
			}
			if len(p.Blocks) != tc.wantBlocks {
				t.Fatalf("blocks = %d, want %d", len(p.Blocks), tc.wantBlocks)
			}
			if tc.wantFile != "" && p.Blocks[0].File != tc.wantFile {
				t.Errorf("file = %q, want %q", p.Blocks[0].File, tc.wantFile)
			}
			if tc.name == "atomic 正常" {
				want := Block{File: "x.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, NumStmt: 3, Count: 1}
				if p.Blocks[0] != want {
					t.Errorf("block = %+v, want %+v", p.Blocks[0], want)
				}
			}
		})
	}
}

func TestGateDecision(t *testing.T) {
	cases := []struct {
		name    string
		covered int
		total   int
		min     float64
		want    int
	}{
		{name: "全绿", covered: 10, total: 10, min: 100, want: 0},
		{name: "刚好等于门禁", covered: 5, total: 10, min: 50, want: 0},
		{name: "刚好等于小数门禁", covered: 1, total: 8, min: 12.5, want: 0},
		{name: "低于门禁", covered: 4, total: 10, min: 40.0001, want: 1},
		{name: "零语句门禁为零", covered: 0, total: 0, min: 0, want: 0},
		{name: "零语句门禁为正", covered: 0, total: 0, min: 0.1, want: 1},
		{name: "零覆盖门禁为零", covered: 0, total: 10, min: 0, want: 0},
		{name: "三分之一精确比较", covered: 1, total: 3, min: 100.0 / 3.0, want: 0},
		{name: "三分之一 ULP 舍入仍达标", covered: 1, total: 3, min: PercentOf(1, 3) + 1e-12, want: 0},
		{name: "超出容差即不达标", covered: 1, total: 3, min: PercentOf(1, 3) + 1e-6, want: 1},
		{name: "三分之一略高门禁", covered: 1, total: 3, min: 33.34, want: 1},
		{name: "全零覆盖", covered: 0, total: 7, min: 1, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GateDecision(tc.covered, tc.total, tc.min); got != tc.want {
				t.Fatalf("GateDecision(%d, %d, %v) = %d, want %d", tc.covered, tc.total, tc.min, got, tc.want)
			}
		})
	}
}

func TestPercentOf(t *testing.T) {
	if got := PercentOf(0, 0); got != 0 {
		t.Errorf("PercentOf(0,0) = %v, want 0", got)
	}
	if got := PercentOf(1, 4); got != 25 {
		t.Errorf("PercentOf(1,4) = %v, want 25", got)
	}
	if got := (Stats{Covered: 3, Statements: 6}).Percent(); got != 50 {
		t.Errorf("Stats.Percent() = %v, want 50", got)
	}
}

func TestPackageOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a.go", "."},
		{"github.com/tktogether/subforge/internal/a/a.go", "github.com/tktogether/subforge/internal/a"},
		{"E:\\Projects\\subforge\\internal\\a\\a.go", "E:/Projects/subforge/internal/a"},
		{"/abs/path/a.go", "/abs/path"},
	}
	for _, tc := range cases {
		if got := packageOf(tc.in); got != tc.want {
			t.Errorf("packageOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAggregate(t *testing.T) {
	prof, err := ParseProfile(strings.NewReader("mode: set\n" +
		"p/x/a.go:1.1,2.2 2 1\n" +
		"p/x/b.go:3.1,4.2 2 0\n" +
		"p/y/a.go:1.1,2.2 2 1\n" +
		"p/y/b.go:3.1,4.2 2 0\n" +
		"p/z/a.go:1.1,2.2 10 10\n"))
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	res := Aggregate(prof)
	if res.Mode != "set" {
		t.Errorf("mode = %q, want set", res.Mode)
	}
	if res.Total.Statements != 18 || res.Total.Covered != 14 {
		t.Errorf("total = %+v, want {Statements:18 Covered:14}", res.Total)
	}
	wantOrder := []string{"p/x", "p/y", "p/z"}
	if len(res.Packages) != len(wantOrder) {
		t.Fatalf("packages = %d, want %d", len(res.Packages), len(wantOrder))
	}
	for i, want := range wantOrder {
		if res.Packages[i].Package != want {
			t.Errorf("packages[%d] = %q, want %q", i, res.Packages[i].Package, want)
		}
	}
	if res.Packages[0].Covered != 2 || res.Packages[0].Statements != 4 {
		t.Errorf("p/x stats = %+v, want {2 4}", res.Packages[0].Stats)
	}
	if res.Packages[2].Percent() != 100 {
		t.Errorf("p/z percent = %v, want 100", res.Packages[2].Percent())
	}
}

func TestVerdictLine(t *testing.T) {
	if got := VerdictLine(85.25, 80); got != "覆盖率 85.2% ≥ 门禁 80%" {
		t.Errorf("pass line = %q", got)
	}
	if got := VerdictLine(79.95, 80.5); got != "覆盖率 80.0% 低于门禁 80.5%" {
		t.Errorf("fail line = %q", got)
	}
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("boom") }

func TestRun(t *testing.T) {
	green := writeProfile(t, profileAllGreen)
	half := writeProfile(t, profileHalf)
	empty := writeProfile(t, "mode: set\n")
	blank := writeProfile(t, "")
	badMode := writeProfile(t, "mode: bogus\nx.go:1.1,2.2 1 1\n")
	shortCols := writeProfile(t, "mode: set\nx.go:1.1,2.2\n")
	missing := filepath.Join(t.TempDir(), "not-exist.out")

	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  []string
		wantErr  []string
	}{
		{
			name: "全绿通过", args: []string{"-profile", green, "-min", "80"}, wantCode: 0,
			wantOut: []string{"覆盖率 100.0% ≥ 门禁 80%"},
		},
		{
			name: "刚好等于门禁", args: []string{"-profile", half, "-min", "50"}, wantCode: 0,
			wantOut: []string{"覆盖率 50.0% ≥ 门禁 50%"},
		},
		{
			name: "低于门禁", args: []string{"-profile", half, "-min", "80"}, wantCode: 1,
			wantOut: []string{"覆盖率 50.0% 低于门禁 80%"},
		},
		{
			name: "小数门禁刚好相等", args: []string{"-profile", half, "-min", "50.0"}, wantCode: 0,
			wantOut: []string{"≥ 门禁 50%"},
		},
		{
			name: "空 profile 零语句", args: []string{"-profile", empty, "-min", "80"}, wantCode: 1,
			wantOut: []string{"覆盖率 0.0% 低于门禁 80%"},
		},
		{
			name: "零语句且门禁为零", args: []string{"-profile", empty, "-min", "0"}, wantCode: 0,
			wantOut: []string{"覆盖率 0.0% ≥ 门禁 0%"},
		},
		{
			name: "top 限制输出", args: []string{"-profile", green, "-min", "0", "-top", "1"}, wantCode: 0,
			wantOut: []string{"覆盖率最低的 1 个包", "总计: 覆盖率 100.0%"},
		},
		{
			name: "top 零表示全部", args: []string{"-profile", green, "-min", "0", "-top", "0"}, wantCode: 0,
			wantOut: []string{"覆盖率最低的 2 个包"},
		},
		{
			name: "默认 top 超过包数", args: []string{"-profile", green, "-min", "0"}, wantCode: 0,
			wantOut: []string{"覆盖率最低的 2 个包"},
		},
		{
			name: "缺少 profile 参数", args: []string{"-min", "80"}, wantCode: 2,
			wantErr: []string{"缺少必填参数 -profile"},
		},
		{
			name: "缺少 min 参数", args: []string{"-profile", green}, wantCode: 2,
			wantErr: []string{"缺少必填参数 -min"},
		},
		{
			name: "min 非数字", args: []string{"-profile", green, "-min", "abc"}, wantCode: 2,
			wantErr: []string{"不是合法的小数百分比"},
		},
		{
			name: "min 为 NaN", args: []string{"-profile", green, "-min", "NaN"}, wantCode: 2,
			wantErr: []string{"不是合法的小数百分比"},
		},
		{
			name: "min 越界", args: []string{"-profile", green, "-min", "101"}, wantCode: 2,
			wantErr: []string{"超出合法范围"},
		},
		{
			name: "min 为负", args: []string{"-profile", green, "-min", "-1"}, wantCode: 2,
			wantErr: []string{"超出合法范围"},
		},
		{
			name: "文件不存在", args: []string{"-profile", missing, "-min", "80"}, wantCode: 2,
			wantErr: []string{"无法打开 profile"},
		},
		{
			name: "profile 为空文件", args: []string{"-profile", blank, "-min", "80"}, wantCode: 2,
			wantErr: []string{"profile 为空"},
		},
		{
			name: "mode 非法", args: []string{"-profile", badMode, "-min", "80"}, wantCode: 2,
			wantErr: []string{"非法 coverage mode"},
		},
		{
			name: "列数不足", args: []string{"-profile", shortCols, "-min", "80"}, wantCode: 2,
			wantErr: []string{"格式非法"},
		},
		{
			name: "未知 flag", args: []string{"-bogus"}, wantCode: 2,
			wantErr: []string{"flag provided but not defined"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := run(tc.args, &stdout, &stderr)
			if got != tc.wantCode {
				t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", got, tc.wantCode, stdout.String(), stderr.String())
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout 缺少 %q\n实际:\n%s", want, stdout.String())
				}
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr 缺少 %q\n实际:\n%s", want, stderr.String())
				}
			}
		})
	}
}

func TestRunJSON(t *testing.T) {
	green := writeProfile(t, profileAllGreen)
	half := writeProfile(t, profileHalf)

	t.Run("通过", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if got := run([]string{"-profile", green, "-min", "80", "-json"}, &stdout, &stderr); got != 0 {
			t.Fatalf("exit = %d, want 0 (stderr=%s)", got, stderr.String())
		}
		var rep Report
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatalf("JSON 解析失败: %v\n%s", err, stdout.String())
		}
		if !rep.Passed || rep.ExitCode != 0 || rep.Percent != 100 {
			t.Errorf("report = %+v", rep)
		}
		if rep.Mode != "atomic" || rep.Min != 80 {
			t.Errorf("mode/min = %q/%v", rep.Mode, rep.Min)
		}
		if rep.Total.Covered != 20 || rep.Total.Statements != 20 {
			t.Errorf("total = %+v", rep.Total)
		}
		if len(rep.Packages) != 2 {
			t.Errorf("packages = %d, want 2", len(rep.Packages))
		}
	})

	t.Run("未通过", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if got := run([]string{"-profile", half, "-min", "90", "-json"}, &stdout, &stderr); got != 1 {
			t.Fatalf("exit = %d, want 1", got)
		}
		var rep Report
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatalf("JSON 解析失败: %v", err)
		}
		if rep.Passed || rep.ExitCode != 1 || rep.Percent != 50 {
			t.Errorf("report = %+v", rep)
		}
	})

	t.Run("JSON 写入失败", func(t *testing.T) {
		var stderr bytes.Buffer
		if got := run([]string{"-profile", green, "-min", "0", "-json"}, errWriter{}, &stderr); got != 2 {
			t.Fatalf("exit = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "输出 JSON 失败") {
			t.Errorf("stderr = %q", stderr.String())
		}
	})
}

func TestRunUsageOnMissingArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("exit = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "用法: coveragegate") {
		t.Errorf("stderr 缺少用法说明: %q", stderr.String())
	}
}

func TestFuncStatsPercentCoveragegate(t *testing.T) {
	if got := (FuncStats{Covered: 1, Functions: 2}).Percent(); got != 50 {
		t.Errorf("Percent() = %v, want 50", got)
	}
	if got := (FuncStats{Covered: 0, Functions: 0}).Percent(); got != 0 {
		t.Errorf("Percent() = %v, want 0", got)
	}
	if got := (FuncStats{Covered: 3, Functions: 3}).Percent(); got != 100 {
		t.Errorf("Percent() = %v, want 100", got)
	}
}

func TestDetectModuleCoveragegate(t *testing.T) {
	t.Run("跳过注释取 module", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("// 注释\nmodule example.com/m\n\ngo 1.26\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := detectModule(dir)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if m != "example.com/m" {
			t.Errorf("module = %q, want example.com/m", m)
		}
	})
	t.Run("无 go.mod", func(t *testing.T) {
		dir := t.TempDir()
		_, err := detectModule(dir)
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("无 module 行", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("go 1.26\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := detectModule(dir)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "go.mod 中未找到 module 行" {
			t.Errorf("err = %q", err.Error())
		}
	})
}

func TestComputeFuncCoverageNormal(t *testing.T) {
	tmp := t.TempDir()
	// a.go
	aSrc := "package m\n\nfunc CoveredA() int {\n\treturn 1\n}\n\nfunc NotCoveredA() int {\n\treturn 2\n}\n\nfunc CoveredB() int {\n\treturn 3\n}\n"
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "a.go"), []byte(aSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bSrc := "package m\n\nfunc NeverHasBlocks() int {\n\treturn 0\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "b.go"), []byte(bSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cSrc := "package sub\n\nfunc CoveredC() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "sub", "c.go"), []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/m\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prof := &Profile{Mode: "atomic", Blocks: []Block{
		{File: "example.com/m/a.go", StartLine: 4, EndLine: 4, NumStmt: 1, Count: 1},
		{File: "example.com/m/a.go", StartLine: 8, EndLine: 8, NumStmt: 1, Count: 0},
		{File: "example.com/m/a.go", StartLine: 12, EndLine: 12, NumStmt: 1, Count: 2},
		{File: "example.com/m/a.go", StartLine: 20, EndLine: 20, NumStmt: 1, Count: 1},
		{File: "example.com/m/b.go", StartLine: 10, EndLine: 10, NumStmt: 1, Count: 1},
		{File: "example.com/m/sub/c.go", StartLine: 4, EndLine: 4, NumStmt: 1, Count: 1},
	}}

	res, err := ComputeFuncCoverage(prof, tmp, "example.com/m")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Total.Functions != 4 {
		t.Errorf("Functions = %d, want 4", res.Total.Functions)
	}
	if res.Total.Covered != 3 {
		t.Errorf("Covered = %d, want 3", res.Total.Covered)
	}
	if len(res.Uncovered) != 1 {
		t.Fatalf("Uncovered len = %d, want 1", len(res.Uncovered))
	}
	if res.Uncovered[0].Func != "NotCoveredA" {
		t.Errorf("Uncovered[0].Func = %q", res.Uncovered[0].Func)
	}
	if res.Uncovered[0].File != "example.com/m/a.go" {
		t.Errorf("Uncovered[0].File = %q", res.Uncovered[0].File)
	}
	// NotCoveredA func keyword at line 7
	if res.Uncovered[0].Line != 7 {
		t.Errorf("Uncovered[0].Line = %d, want 7", res.Uncovered[0].Line)
	}

	if len(res.Packages) != 2 {
		t.Fatalf("Packages len = %d, want 2", len(res.Packages))
	}
	pkgMap := map[string]FuncStats{}
	for _, p := range res.Packages {
		pkgMap[p.Package] = p.FuncStats
	}
	if v, ok := pkgMap[packageOf("example.com/m/a.go")]; !ok || v.Functions != 3 || v.Covered != 2 {
		t.Errorf("a.go pkg stats = %+v", v)
	}
	if v, ok := pkgMap[packageOf("example.com/m/sub/c.go")]; !ok || v.Functions != 1 || v.Covered != 1 {
		t.Errorf("c.go pkg stats = %+v", v)
	}
}

func TestComputeFuncCoverageBarePaths(t *testing.T) {
	tmp := t.TempDir()
	aSrc := "package m\n\nfunc CoveredA() int {\n\treturn 1\n}\n\nfunc NotCoveredA() int {\n\treturn 2\n}\n\nfunc CoveredB() int {\n\treturn 3\n}\n"
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "a.go"), []byte(aSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bSrc := "package m\n\nfunc NeverHasBlocks() int {\n\treturn 0\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "b.go"), []byte(bSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cSrc := "package sub\n\nfunc CoveredC() int {\n\treturn 1\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "sub", "c.go"), []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	prof := &Profile{Mode: "atomic", Blocks: []Block{
		{File: "a.go", StartLine: 4, EndLine: 4, NumStmt: 1, Count: 1},
		{File: "a.go", StartLine: 8, EndLine: 8, NumStmt: 1, Count: 0},
		{File: "a.go", StartLine: 12, EndLine: 12, NumStmt: 1, Count: 2},
		{File: "a.go", StartLine: 20, EndLine: 20, NumStmt: 1, Count: 1},
		{File: "b.go", StartLine: 10, EndLine: 10, NumStmt: 1, Count: 1},
		{File: "sub/c.go", StartLine: 4, EndLine: 4, NumStmt: 1, Count: 1},
	}}

	res, err := ComputeFuncCoverage(prof, tmp, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Total.Functions != 4 {
		t.Errorf("Functions = %d, want 4", res.Total.Functions)
	}
	if res.Total.Covered != 3 {
		t.Errorf("Covered = %d, want 3", res.Total.Covered)
	}
}

func TestComputeFuncCoverageParseFail(t *testing.T) {
	tmp := t.TempDir()
	prof := &Profile{Mode: "atomic", Blocks: []Block{
		{File: "example.com/m/missing.go", StartLine: 1, EndLine: 1, NumStmt: 1, Count: 1},
	}}
	res, err := ComputeFuncCoverage(prof, tmp, "example.com/m")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "解析源文件") {
		t.Errorf("err = %q", err.Error())
	}
	if res != nil {
		t.Errorf("res = %+v, want nil", res)
	}
}

func TestComputeFuncCoverageExcludesMain(t *testing.T) {
	tmp := t.TempDir()
	mainSrc := "package main\n\nfunc main() {}\n\nfunc helper() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/m2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prof := &Profile{Mode: "atomic", Blocks: []Block{
		{File: "example.com/m2/main.go", StartLine: 3, EndLine: 3, NumStmt: 1, Count: 0},
		{File: "example.com/m2/main.go", StartLine: 5, EndLine: 5, NumStmt: 1, Count: 0},
	}}

	res, err := ComputeFuncCoverage(prof, tmp, "example.com/m2")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Total.Functions != 1 {
		t.Errorf("Functions = %d, want 1", res.Total.Functions)
	}
	if len(res.Uncovered) != 1 {
		t.Fatalf("Uncovered len = %d, want 1", len(res.Uncovered))
	}
	if res.Uncovered[0].Func != "helper" {
		t.Errorf("Uncovered[0].Func = %q, want helper", res.Uncovered[0].Func)
	}
}
