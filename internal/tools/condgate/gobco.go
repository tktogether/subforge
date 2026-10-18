// Command condgate: gobco engine.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// GobcoCondition 对应 gobco -stats 输出的单条判定记录。
type GobcoCondition struct {
	Start      string `json:"Start"`
	Code       string `json:"Code"`
	TrueCount  int    `json:"TrueCount"`
	FalseCount int    `json:"FalseCount"`
}

// listPackages 把包模式展开为包目录（gobco 只接受目录，不接受 import path）；可被测试替换。
var listPackages = func(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("没有包模式")
	}
	args := append([]string{"list", "-f", "{{.Dir}}"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = "."
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("go list 失败: %w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("go list 失败: %w", err)
	}
	seen := map[string]bool{}
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "/vendor/") {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		pkgs = append(pkgs, line)
	}
	sort.Strings(pkgs)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("没有匹配的包")
	}
	return pkgs, nil
}

// prepareGobcoTemp 在 tmpDir 下建独立临时根，返回 stats 文件路径与清理函数。
// gobco 会在 TMPDIR 里留下大量临时文件，无论成功失败都必须清掉，否则磁盘会被写满。
func prepareGobcoTemp(tmpDir string) (string, func(), error) {
	root, err := os.MkdirTemp(tmpDir, "condgate-gobco-")
	if err != nil {
		return "", nil, fmt.Errorf("创建 gobco 临时根失败: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	return filepath.Join(root, "gobco-stats.json"), cleanup, nil
}

// gobcoPackageDir 把 go list 给出的绝对目录转成相对目录传给 gobco。
// 实测：buildinfo 绝对 61.25s vs 相对 52.55s；subscheck 绝对 56.0s vs 相对 76.5s
// （subscheck 相对那次在 Lead 全量 gobco 并发跑时测得，受 CPU 争用影响）。
// 选相对目录：gobco 按进程 cwd 解析，省一次绝对路径解析，且 Lead 实测曾差 4 倍。
func gobcoPackageDir(pkg string) string {
	if !filepath.IsAbs(pkg) {
		return pkg
	}
	cwd, err := os.Getwd()
	if err != nil {
		return pkg
	}
	rel, err := filepath.Rel(cwd, pkg)
	if err != nil || strings.HasPrefix(rel, "..") {
		return pkg
	}
	return rel
}

// gobcoRun 调用 gobco 并读回 stats；可被测试替换。
var gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) {
	statsPath, cleanup, err := prepareGobcoTemp(tmpDir)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	cmd := exec.Command("gobco", "-stats", statsPath, gobcoPackageDir(pkg))
	root := filepath.Dir(statsPath)
	cmd.Env = append(os.Environ(), "TMPDIR="+root, "TMP="+root, "TEMP="+root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("gobco 执行失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(statsPath)
	if err != nil {
		return nil, fmt.Errorf("读取 gobco stats 失败: %w", err)
	}
	var conds []GobcoCondition
	if err := json.Unmarshal(data, &conds); err != nil {
		return nil, fmt.Errorf("解析 gobco stats 失败: %w", err)
	}
	return conds, nil
}

// parseGobcoStart 解析 "file:line:col"。
func parseGobcoStart(s string) (file string, line, col int, ok bool) {
	last := strings.LastIndex(s, ":")
	if last <= 0 || last == len(s)-1 {
		return "", 0, 0, false
	}
	colStr := s[last+1:]
	prev := strings.LastIndex(s[:last], ":")
	if prev <= 0 {
		return "", 0, 0, false
	}
	lineStr := s[prev+1 : last]
	file = s[:prev]
	l, err := parsePositiveInt(lineStr)
	if err != nil {
		return "", 0, 0, false
	}
	c, err := parsePositiveInt(colStr)
	if err != nil {
		return "", 0, 0, false
	}
	return file, l, c, true
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("空")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("非数字")
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 {
		return 0, fmt.Errorf("非正")
	}
	return n, nil
}

// runGobcoEngine 是 gobco 引擎的可测试内核（默认临时根）。
func runGobcoEngine(patterns []string, minVal float64, top int, jsonOut bool, stdout, stderr io.Writer) int {
	return runGobcoEngineWithTmpDir(patterns, minVal, top, jsonOut, os.TempDir(), stdout, stderr)
}

// runGobcoEngineWithTmpDir 是 gobco 引擎的可测试内核（显式临时根）。
func runGobcoEngineWithTmpDir(patterns []string, minVal float64, top int, jsonOut bool, tmpDir string, stdout, stderr io.Writer) int {
	pkgs, err := listPackages(patterns)
	if err != nil {
		fmt.Fprintln(stderr, "展开包失败:", err)
		return 2
	}

	type pkgAgg struct {
		name    string
		covered int
		total   int
	}
	var pkgsOut []pkgAgg
	var allUncovered []uncoveredJSON
	covered := 0
	total := 0
	skipped := 0

	for _, pkg := range pkgs {
		conds, err := gobcoRun(pkg, tmpDir)
		if err != nil {
			fmt.Fprintln(stderr, "包失败", pkg, ":", err)
			skipped++
			continue
		}
		pCovered := 0
		pTotal := 0
		for _, c := range conds {
			pTotal += 2
			total += 2
			if c.TrueCount > 0 {
				pCovered++
				covered++
			}
			if c.FalseCount > 0 {
				pCovered++
				covered++
			}
			if c.TrueCount == 0 || c.FalseCount == 0 {
				file, line, col, ok := parseGobcoStart(c.Start)
				if !ok {
					file = c.Start
				}
				allUncovered = append(allUncovered, uncoveredJSON{
					File:   file,
					Line:   line,
					Col:    col,
					Kind:   "cond",
					Detail: c.Code,
				})
			}
		}
		pkgsOut = append(pkgsOut, pkgAgg{name: pkg, covered: pCovered, total: pTotal})
	}

	if total == 0 && skipped > 0 {
		fmt.Fprintf(stderr, "全部 %d 个包的 gobco 都失败了\n", skipped)
		return 2
	}

	sort.Slice(pkgsOut, func(i, j int) bool {
		pi := percentOf(pkgsOut[i].covered, pkgsOut[i].total)
		pj := percentOf(pkgsOut[j].covered, pkgsOut[j].total)
		if pi != pj {
			return pi < pj
		}
		return pkgsOut[i].name < pkgsOut[j].name
	})

	sort.Slice(allUncovered, func(i, j int) bool {
		if allUncovered[i].File != allUncovered[j].File {
			return allUncovered[i].File < allUncovered[j].File
		}
		if allUncovered[i].Line != allUncovered[j].Line {
			return allUncovered[i].Line < allUncovered[j].Line
		}
		return allUncovered[i].Col < allUncovered[j].Col
	})

	pct := percentOf(covered, total)
	passed := pct+percentEpsilon >= minVal
	exitCode := 0
	if !passed {
		exitCode = 1
	}
	if total == 0 && minVal > 0 {
		exitCode = 1
	}

	if jsonOut {
		pkgsJSON := make([]packageStatsJSON, 0, len(pkgsOut))
		for _, p := range pkgsOut {
			pkgsJSON = append(pkgsJSON, packageStatsJSON{
				Package: p.name,
				Covered: p.covered,
				Total:   p.total,
				Percent: percentOf(p.covered, p.total),
			})
		}
		r := reportJSON{
			Engine:    "gobco",
			Mode:      "gobco",
			Min:       minVal,
			Percent:   pct,
			Passed:    passed,
			ExitCode:  exitCode,
			Covered:   covered,
			Total:     total,
			Skipped:   skipped,
			Packages:  pkgsJSON,
			Uncovered: allUncovered,
		}
		if r.Packages == nil {
			r.Packages = []packageStatsJSON{}
		}
		if r.Uncovered == nil {
			r.Uncovered = []uncoveredJSON{}
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return exitCode
	}

	fmt.Fprintf(stdout, "condgate: engine=gobco 包数=%d\n", len(pkgsOut))
	fmt.Fprintf(stdout, "条件覆盖: %d/%d = %.1f%%\n", covered, total, pct)
	shown := allUncovered
	if len(shown) > textUncoveredLimit {
		shown = shown[:textUncoveredLimit]
	}
	for _, j := range shown {
		fmt.Fprintf(stdout, "  %s:%d:%d: cond %s\n", j.File, j.Line, j.Col, j.Detail)
	}
	if rest := len(allUncovered) - len(shown); rest > 0 {
		fmt.Fprintf(stdout, "  …还有 %d 条未列出（用 -json 取完整清单）\n", rest)
	}
	n := top
	if n <= 0 || n > len(pkgsOut) {
		n = len(pkgsOut)
	}
	fmt.Fprintf(stdout, "未覆盖判定最多的 %d 个包:\n", n)
	for i := 0; i < n; i++ {
		p := pkgsOut[i]
		fmt.Fprintf(stdout, "    %.1f%%  %s            (%d/%d 判定)\n", percentOf(p.covered, p.total), p.name, p.covered, p.total)
	}
	if passed {
		fmt.Fprintf(stdout, "条件覆盖率 %.1f%% ≥ 门禁 %.0f%%\n", pct, minVal)
	} else {
		fmt.Fprintf(stdout, "条件覆盖率 %.1f%% 低于门禁 %.0f%%\n", pct, minVal)
	}
	return exitCode
}
