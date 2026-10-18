// Command coveragegate 是一个自包含的 Go 覆盖率门禁工具。
//
// 它直接解析 go test -coverprofile 生成的 coverage profile 文本（不 shell out 到
// go tool cover），按包（目录）聚合语句覆盖率，并在总覆盖率低于门禁时以退出码 1 失败。
//
// 用法：
//
//	go run ./internal/tools/coveragegate -profile coverage.out -min 80
//
// 退出码：
//
//	0  总覆盖率 >= 门禁
//	1  总覆盖率 <  门禁
//	2  参数缺失/文件不可读/profile 格式非法
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Block 是 coverage profile 中的一行。
type Block struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NumStmt   int
	Count     int
}

// Profile 是一个解析完成的 coverage profile。
type Profile struct {
	Mode   string
	Blocks []Block
}

// Stats 是语句计数。
type Stats struct {
	Statements int `json:"statements"`
	Covered    int `json:"covered"`
}

// Percent 返回覆盖率百分比；语句数为 0 时返回 0。
func (s Stats) Percent() float64 {
	return PercentOf(s.Covered, s.Statements)
}

// PercentOf 返回 covered/total*100；total 为 0 时返回 0。
func PercentOf(covered, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}

// PackageStats 是单个包（目录）的聚合结果。
type PackageStats struct {
	Package string `json:"package"`
	Stats
}

// Result 是聚合后的报告。
type Result struct {
	Mode     string         `json:"mode"`
	Total    Stats          `json:"total"`
	Packages []PackageStats `json:"packages"`
}

// Report 是 -json 的机器可读输出。
type Report struct {
	Mode     string         `json:"mode"`
	Min      float64        `json:"min"`
	Percent  float64        `json:"percent"`
	Passed   bool           `json:"passed"`
	ExitCode int            `json:"exit_code"`
	Total    Stats          `json:"total"`
	Packages []PackageStats `json:"packages"`
}

// percentEpsilon 是门禁比较的容差（单位：百分点）。
// 它只用于吸收浮点舍入误差——例如 1/3 实测为 33.33333333333333%，
// 而 100.0/3.0 为 33.333333333333336%，两者数学上相等。
// 1e-9 个百分点远小于「一条语句」所能造成的最小覆盖率变化（约 100/语句数），
// 因此不会放过任何真实的不达标。
const percentEpsilon = 1e-9

// GatePass 是纯函数形式的阈值比较：实测百分比是否达标（带浮点容差）。
func GatePass(pct, minPercent float64) bool {
	return pct+percentEpsilon >= minPercent
}

// GateDecision 是纯函数形式的门禁判定：输入语句计数与最低百分比，返回进程退出码。
// 0 表示达标，1 表示低于门禁。
func GateDecision(covered, total int, minPercent float64) int {
	if GatePass(PercentOf(covered, total), minPercent) {
		return 0
	}
	return 1
}

// ParseProfile 解析 Go coverage profile 文本。
// 首行为 "mode: set|count|atomic"，其后每行为
// <file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>。
func ParseProfile(r io.Reader) (*Profile, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("读取 profile 失败: %w", err)
		}
		return nil, errors.New("profile 为空: 缺少 mode 行")
	}
	modeLine := strings.TrimSpace(sc.Text())
	if !strings.HasPrefix(modeLine, "mode:") {
		return nil, fmt.Errorf("profile 首行非法，期望 \"mode: set|count|atomic\"，实际为 %q", modeLine)
	}
	mode := strings.TrimSpace(strings.TrimPrefix(modeLine, "mode:"))
	switch mode {
	case "set", "count", "atomic":
	default:
		return nil, fmt.Errorf("非法 coverage mode %q，期望 set、count 或 atomic", mode)
	}
	p := &Profile{Mode: mode}
	lineNo := 1
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		b, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("profile 第 %d 行非法: %w", lineNo, err)
		}
		p.Blocks = append(p.Blocks, b)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("读取 profile 失败: %w", err)
	}
	return p, nil
}

func parseLine(line string) (Block, error) {
	var b Block
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return b, fmt.Errorf("期望 3 个字段 (file:start,end numStmt count)，实际 %d 个: %q", len(fields), line)
	}
	loc, numStmtStr, countStr := fields[0], fields[1], fields[2]

	numStmt, err := strconv.Atoi(numStmtStr)
	if err != nil || numStmt < 0 {
		return b, fmt.Errorf("numStmt %q 非法，期望非负整数", numStmtStr)
	}
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 0 {
		return b, fmt.Errorf("count %q 非法，期望非负整数", countStr)
	}

	colon := strings.LastIndex(loc, ":")
	if colon <= 0 || colon == len(loc)-1 {
		return b, fmt.Errorf("位置 %q 非法，期望 file:startLine.startCol,endLine.endCol", loc)
	}
	file := loc[:colon]
	span := loc[colon+1:]
	parts := strings.Split(span, ",")
	if len(parts) != 2 {
		return b, fmt.Errorf("位置区间 %q 非法，期望 startLine.startCol,endLine.endCol", span)
	}
	sl, sc, err := parsePos(parts[0])
	if err != nil {
		return b, err
	}
	el, ec, err := parsePos(parts[1])
	if err != nil {
		return b, err
	}
	b = Block{File: file, StartLine: sl, StartCol: sc, EndLine: el, EndCol: ec, NumStmt: numStmt, Count: count}
	return b, nil
}

func parsePos(s string) (int, int, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("位置 %q 非法，期望 line.col", s)
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil || line < 1 {
		return 0, 0, fmt.Errorf("行号 %q 非法，期望正整数", parts[0])
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil || col < 1 {
		return 0, 0, fmt.Errorf("列号 %q 非法，期望正整数", parts[1])
	}
	return line, col, nil
}

// packageOf 返回文件所属的包（目录），跨平台统一使用 / 分隔。
func packageOf(file string) string {
	return path.Dir(strings.ReplaceAll(file, "\\", "/"))
}

// Aggregate 按包（目录）聚合 profile，包列表按覆盖率升序（同覆盖率按名字升序）排序。
func Aggregate(p *Profile) *Result {
	res := &Result{Mode: p.Mode}
	byPkg := make(map[string]*Stats)
	for _, b := range p.Blocks {
		res.Total.Statements += b.NumStmt
		if b.Count > 0 {
			res.Total.Covered += b.NumStmt
		}
		key := packageOf(b.File)
		s := byPkg[key]
		if s == nil {
			s = &Stats{}
			byPkg[key] = s
		}
		s.Statements += b.NumStmt
		if b.Count > 0 {
			s.Covered += b.NumStmt
		}
	}
	names := make([]string, 0, len(byPkg))
	for n := range byPkg {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		a, c := byPkg[names[i]], byPkg[names[j]]
		if a.Covered*c.Statements != c.Covered*a.Statements {
			return a.Covered*c.Statements < c.Covered*a.Statements
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		res.Packages = append(res.Packages, PackageStats{Package: n, Stats: *byPkg[n]})
	}
	return res
}

func formatPercent(p float64) string {
	return strconv.FormatFloat(p, 'f', 1, 64)
}

func formatGate(m float64) string {
	return strconv.FormatFloat(m, 'f', -1, 64)
}

// VerdictLine 渲染人类可读的判定行。
func VerdictLine(pct, min float64) string {
	if GatePass(pct, min) {
		return fmt.Sprintf("覆盖率 %s%% ≥ 门禁 %s%%", formatPercent(pct), formatGate(min))
	}
	return fmt.Sprintf("覆盖率 %s%% 低于门禁 %s%%", formatPercent(pct), formatGate(min))
}

func formatText(res *Result, min float64, topN int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coveragegate: mode=%s 包数=%d\n", res.Mode, len(res.Packages))
	b.WriteString("按包（目录）聚合的语句覆盖率:\n")
	for _, p := range res.Packages {
		fmt.Fprintf(&b, "  %6s%%  %-60s (%d/%d 语句)\n", formatPercent(p.Percent()), p.Package, p.Covered, p.Statements)
	}
	n := topN
	if n <= 0 || n > len(res.Packages) {
		n = len(res.Packages)
	}
	if n > 0 {
		fmt.Fprintf(&b, "覆盖率最低的 %d 个包:\n", n)
		for i := 0; i < n; i++ {
			p := res.Packages[i]
			fmt.Fprintf(&b, "  %6s%%  %s\n", formatPercent(p.Percent()), p.Package)
		}
	}
	fmt.Fprintf(&b, "总计: 覆盖率 %s%% (%d/%d 语句)\n", formatPercent(res.Total.Percent()), res.Total.Covered, res.Total.Statements)
	b.WriteString(VerdictLine(res.Total.Percent(), min) + "\n")
	return b.String()
}

func run(args []string, stdout, stderr io.Writer) int {
	fset := flag.NewFlagSet("coveragegate", flag.ContinueOnError)
	fset.SetOutput(stderr)
	profilePath := fset.String("profile", "", "Go coverage profile 文件路径（必填）")
	minStr := fset.String("min", "", "覆盖率门禁百分比，支持小数（必填，如 80 或 80.5）")
	asJSON := fset.Bool("json", false, "以 JSON 输出机器可读报告")
	topN := fset.Int("top", 5, "打印覆盖率最低的 N 个包（<=0 表示全部）")
	fset.Usage = func() {
		fmt.Fprint(stderr, "用法: coveragegate -profile coverage.out -min 80 [-json] [-top 5]\n\n")
		fset.PrintDefaults()
	}
	if err := fset.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "错误: 缺少必填参数 -profile")
		fset.Usage()
		return 2
	}
	if strings.TrimSpace(*minStr) == "" {
		fmt.Fprintln(stderr, "错误: 缺少必填参数 -min")
		fset.Usage()
		return 2
	}
	minPercent, err := strconv.ParseFloat(strings.TrimSpace(*minStr), 64)
	if err != nil || math.IsNaN(minPercent) || math.IsInf(minPercent, 0) {
		fmt.Fprintf(stderr, "错误: -min %q 不是合法的小数百分比\n", *minStr)
		return 2
	}
	if minPercent < 0 || minPercent > 100 {
		fmt.Fprintf(stderr, "错误: -min %s 超出合法范围 [0,100]\n", formatGate(minPercent))
		return 2
	}

	f, err := os.Open(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "错误: 无法打开 profile %q: %v\n", *profilePath, err)
		return 2
	}
	defer f.Close()

	prof, err := ParseProfile(f)
	if err != nil {
		fmt.Fprintf(stderr, "错误: profile %q 格式非法: %v\n", *profilePath, err)
		return 2
	}
	res := Aggregate(prof)
	code := GateDecision(res.Total.Covered, res.Total.Statements, minPercent)

	if *asJSON {
		report := Report{
			Mode:     res.Mode,
			Min:      minPercent,
			Percent:  res.Total.Percent(),
			Passed:   code == 0,
			ExitCode: code,
			Total:    res.Total,
			Packages: res.Packages,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(&report); err != nil {
			fmt.Fprintf(stderr, "错误: 输出 JSON 失败: %v\n", err)
			return 2
		}
		return code
	}

	fmt.Fprint(stdout, formatText(res, minPercent, *topN))
	return code
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
