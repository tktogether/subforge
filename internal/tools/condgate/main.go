// Command condgate is a conditional-coverage gate (approximate).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const percentEpsilon = 1e-9

// textUncoveredLimit 限制**文本报告**里列出的未覆盖判定条数；
// JSON 输出（-json）始终给完整清单，供补测定位使用。
const textUncoveredLimit = 50

type Block struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NumStmt   int
	Count     int
}

type Profile struct {
	Mode   string
	Blocks []Block
}

type Judgment struct {
	File    string
	Line    int
	Col     int
	Kind    string
	Detail  string
	Covered bool
	Package string
}

type uncoveredJSON struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type packageStatsJSON struct {
	Package string  `json:"package"`
	Covered int     `json:"covered"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
}

type reportJSON struct {
	Engine    string             `json:"engine"`
	Mode      string             `json:"mode"`
	Min       float64            `json:"min"`
	Percent   float64            `json:"percent"`
	Passed    bool               `json:"passed"`
	ExitCode  int                `json:"exit_code"`
	Covered   int                `json:"covered"`
	Total     int                `json:"total"`
	Skipped   int                `json:"skipped"`
	Packages  []packageStatsJSON `json:"packages"`
	Uncovered []uncoveredJSON    `json:"uncovered"`
}

func run() int {
	return runArgs(os.Args[1:], os.Stdout, os.Stderr)
}

// runArgs 是 run 的可测试内核：参数、stdout/stderr 全部显式注入。
func runArgs(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("condgate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	engine := fs.String("engine", "profile", "引擎")
	profile := fs.String("profile", "", "profile 路径")
	minStr := fs.String("min", "", "门禁百分比")
	top := fs.Int("top", 20, "top N")
	jsonOut := fs.Bool("json", false, "JSON")
	root := fs.String("root", ".", "源码根")
	module := fs.String("module", "", "模块前缀")
	packages := fs.String("packages", "", "gobco 用")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "condgate -engine=profile -profile coverage.out -min 80 [-top 20] [-json] [-root .] [-module P]")
		fmt.Fprintln(stderr, "condgate -engine=gobco  -packages ./internal/... -min 80 [-top 20] [-json]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "本工具 profile 引擎是判定覆盖的近似：if a && b 整体算 1 个判定，")
		fmt.Fprintln(stderr, "与 gobco 的真条件覆盖不可直接比较（condgate 分数结构性 >= gobco 分数）。")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *engine != "profile" && *engine != "gobco" {
		fmt.Fprintln(stderr, "非法 engine:", *engine)
		return 2
	}
	if *minStr == "" {
		fmt.Fprintln(stderr, "缺少 -min")
		return 2
	}
	minVal, err := strconv.ParseFloat(*minStr, 64)
	if err != nil {
		fmt.Fprintln(stderr, "非法 -min:", err)
		return 2
	}
	if *engine == "gobco" {
		patterns := []string{}
		if strings.TrimSpace(*packages) != "" {
			for _, p := range strings.Fields(*packages) {
				patterns = append(patterns, p)
			}
		}
		patterns = append(patterns, fs.Args()...)
		if len(patterns) == 0 {
			fmt.Fprintln(stderr, "gobco 引擎需要 -packages 或位置参数")
			return 2
		}
		return runGobcoEngine(patterns, minVal, *top, *jsonOut, stdout, stderr)
	}
	if *profile == "" {
		fmt.Fprintln(stderr, "缺少 -profile")
		return 2
	}

	f, err := os.Open(*profile)
	if err != nil {
		fmt.Fprintln(stderr, "读取 profile 失败:", err)
		return 2
	}
	defer f.Close()
	prof, err := ParseProfile(f)
	if err != nil {
		fmt.Fprintln(stderr, "profile 解析失败:", err)
		return 2
	}

	mod := *module
	if mod == "" {
		m, err := readModule(*root)
		if err != nil {
			fmt.Fprintln(stderr, "读取 go.mod 失败:", err)
			return 2
		}
		mod = m
	}

	files := uniqueFiles(prof)
	var all []Judgment
	skipped := 0
	for _, pf := range files {
		diskPath := mapPath(pf, mod, *root)
		src, err := os.ReadFile(diskPath)
		if err != nil {
			fmt.Fprintln(stderr, "读取源码失败:", diskPath, err)
			return 2
		}
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, diskPath, src, 0)
		if err != nil {
			fmt.Fprintln(stderr, "解析源码失败:", diskPath, err)
			return 2
		}
		fileBlocks := blocksForFile(prof, pf)
		js, sk := analyzeFile(pf, astFile, fset, fileBlocks)
		all = append(all, js...)
		skipped += sk
	}

	covered := 0
	total := 0
	for _, j := range all {
		total++
		if j.Covered {
			covered++
		}
	}

	byPkg := make(map[string]*struct{ covered, total int })
	for _, j := range all {
		p := j.Package
		if byPkg[p] == nil {
			byPkg[p] = &struct{ covered, total int }{}
		}
		byPkg[p].total++
		if j.Covered {
			byPkg[p].covered++
		}
	}
	type pkgAgg struct {
		name    string
		covered int
		total   int
	}
	var pkgs []pkgAgg
	for name, v := range byPkg {
		pkgs = append(pkgs, pkgAgg{name: name, covered: v.covered, total: v.total})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		pi := percentOf(pkgs[i].covered, pkgs[i].total)
		pj := percentOf(pkgs[j].covered, pkgs[j].total)
		if pi != pj {
			return pi < pj
		}
		return pkgs[i].name < pkgs[j].name
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

	var uncovered []Judgment
	for _, j := range all {
		if !j.Covered {
			uncovered = append(uncovered, j)
		}
	}
	sort.Slice(uncovered, func(i, j int) bool {
		if uncovered[i].File != uncovered[j].File {
			return uncovered[i].File < uncovered[j].File
		}
		if uncovered[i].Line != uncovered[j].Line {
			return uncovered[i].Line < uncovered[j].Line
		}
		return uncovered[i].Col < uncovered[j].Col
	})

	if *jsonOut {
		pkgsJSON := make([]packageStatsJSON, 0, len(pkgs))
		for _, p := range pkgs {
			pkgsJSON = append(pkgsJSON, packageStatsJSON{
				Package: p.name,
				Covered: p.covered,
				Total:   p.total,
				Percent: percentOf(p.covered, p.total),
			})
		}
		uncJSON := make([]uncoveredJSON, 0, len(uncovered))
		for _, j := range uncovered {
			uncJSON = append(uncJSON, uncoveredJSON{
				File:   j.File,
				Line:   j.Line,
				Col:    j.Col,
				Kind:   j.Kind,
				Detail: j.Detail,
			})
		}
		r := reportJSON{
			Engine:    *engine,
			Mode:      prof.Mode,
			Min:       minVal,
			Percent:   pct,
			Passed:    passed,
			ExitCode:  exitCode,
			Covered:   covered,
			Total:     total,
			Skipped:   skipped,
			Packages:  pkgsJSON,
			Uncovered: uncJSON,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return exitCode
	}

	fmt.Fprintf(stdout, "condgate: engine=profile mode=%s 文件数=%d 包数=%d\n", prof.Mode, len(files), len(pkgs))
	fmt.Fprintf(stdout, "条件覆盖: %d/%d = %.1f%%\n", covered, total, pct)
	fmt.Fprintf(stdout, "未覆盖的判定（最多列 %d 条，JSON 输出为完整清单）:\n", textUncoveredLimit)
	shown := uncovered
	if len(shown) > textUncoveredLimit {
		shown = shown[:textUncoveredLimit]
	}
	for _, j := range shown {
		fmt.Fprintf(stdout, "  %s:%d:%d: %s 语句 %s\n", j.File, j.Line, j.Col, j.Kind, j.Detail)
	}
	if rest := len(uncovered) - len(shown); rest > 0 {
		fmt.Fprintf(stdout, "  …还有 %d 条未列出（用 -json 取完整清单）\n", rest)
	}
	fmt.Fprintf(stdout, "跳过无法判定的判定: %d 条（无 else 且其后无块）\n", skipped)
	n := *top
	if n <= 0 || n > len(pkgs) {
		n = len(pkgs)
	}
	fmt.Fprintf(stdout, "未覆盖判定最多的 %d 个包:\n", n)
	for i := 0; i < n; i++ {
		p := pkgs[i]
		fmt.Fprintf(stdout, "    %.1f%%  %s            (%d/%d 判定)\n", percentOf(p.covered, p.total), p.name, p.covered, p.total)
	}
	if passed {
		fmt.Fprintf(stdout, "条件覆盖率 %.1f%% ≥ 门禁 %.0f%%\n", pct, minVal)
	} else {
		fmt.Fprintf(stdout, "条件覆盖率 %.1f%% 低于门禁 %.0f%%\n", pct, minVal)
	}
	return exitCode
}

func GatePass(pct, minPercent float64) bool {
	return pct+percentEpsilon >= minPercent
}

func percentOf(covered, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}

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
		return nil, fmt.Errorf("profile 首行非法")
	}
	mode := strings.TrimSpace(strings.TrimPrefix(modeLine, "mode:"))
	switch mode {
	case "set", "count", "atomic":
	default:
		return nil, fmt.Errorf("非法 coverage mode %q", mode)
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
		return b, fmt.Errorf("期望 3 个字段, 实际 %d", len(fields))
	}
	loc, numStmtStr, countStr := fields[0], fields[1], fields[2]
	numStmt, err := strconv.Atoi(numStmtStr)
	if err != nil || numStmt < 0 {
		return b, fmt.Errorf("numStmt %q 非法", numStmtStr)
	}
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 0 {
		return b, fmt.Errorf("count %q 非法", countStr)
	}
	colon := strings.LastIndex(loc, ":")
	if colon <= 0 || colon == len(loc)-1 {
		return b, fmt.Errorf("位置 %q 非法", loc)
	}
	file := loc[:colon]
	span := loc[colon+1:]
	parts := strings.Split(span, ",")
	if len(parts) != 2 {
		return b, fmt.Errorf("区间 %q 非法", span)
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
		return 0, 0, fmt.Errorf("位置 %q 非法", s)
	}
	line, err := strconv.Atoi(parts[0])
	if err != nil || line < 1 {
		return 0, 0, fmt.Errorf("行号 %q 非法", parts[0])
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil || col < 1 {
		return 0, 0, fmt.Errorf("列号 %q 非法", parts[1])
	}
	return line, col, nil
}

func readModule(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}
	return "", errors.New("go.mod 缺少 module 行")
}

func mapPath(pf, mod, root string) string {
	if strings.HasPrefix(pf, mod+"/") {
		return filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(pf, mod+"/")))
	}
	return filepath.Join(root, filepath.FromSlash(pf))
}

func uniqueFiles(p *Profile) []string {
	seen := map[string]bool{}
	var out []string
	for _, b := range p.Blocks {
		if !seen[b.File] {
			seen[b.File] = true
			out = append(out, b.File)
		}
	}
	sort.Strings(out)
	return out
}

func blocksForFile(p *Profile, file string) []Block {
	var out []Block
	for _, b := range p.Blocks {
		if b.File == file {
			out = append(out, b)
		}
	}
	return out
}

func firstBlockInRange(blocks []Block, startLine, endLine int) *Block {
	var best *Block
	for i := range blocks {
		b := blocks[i]
		if b.StartLine < startLine || b.StartLine > endLine {
			continue
		}
		if best == nil || b.StartLine < best.StartLine || (b.StartLine == best.StartLine && b.StartCol < best.StartCol) {
			bc := b
			best = &bc
		}
	}
	return best
}

func findNextBlockAfter(blocks []Block, afterLine, funcEndLine int) *Block {
	var best *Block
	for _, b := range blocks {
		if b.StartLine <= afterLine {
			continue
		}
		if funcEndLine > 0 && b.StartLine > funcEndLine {
			continue
		}
		if best == nil || b.StartLine < best.StartLine || (b.StartLine == best.StartLine && b.StartCol < best.StartCol) {
			bc := b
			best = &bc
		}
	}
	return best
}

func analyzeFile(pf string, f *ast.File, fset *token.FileSet, blocks []Block) ([]Judgment, int) {
	var judgments []Judgment
	skipped := 0
	visitNode(f, fset, blocks, pf, &judgments, &skipped, 0)
	pkg := packageOf(pf)
	for i := range judgments {
		judgments[i].Package = pkg
		judgments[i].File = pf
	}
	return judgments, skipped
}

func visitNode(node ast.Node, fset *token.FileSet, blocks []Block, pf string, judgments *[]Judgment, skipped *int, funcEnd int) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.File:
		for _, d := range n.Decls {
			visitNode(d, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.FuncDecl:
		fe := funcEnd
		if n.Body != nil {
			fe = fset.Position(n.End()).Line
		}
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, fe)
		}
	case *ast.FuncLit:
		fe := funcEnd
		if n.Body != nil {
			fe = fset.Position(n.End()).Line
		}
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, fe)
		}
	case *ast.BlockStmt:
		for _, s := range n.List {
			visitNode(s, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.IfStmt:
		handleIf(pf, n, fset, blocks, judgments, skipped, funcEnd)
		if n.Init != nil {
			visitNode(n.Init, fset, blocks, pf, judgments, skipped, funcEnd)
		}
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, funcEnd)
		}
		if n.Else != nil {
			visitNode(n.Else, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.ForStmt:
		handleFor(pf, n, fset, blocks, judgments, skipped, funcEnd)
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, funcEnd)
		}
		if n.Init != nil {
			visitNode(n.Init, fset, blocks, pf, judgments, skipped, funcEnd)
		}
		if n.Post != nil {
			visitNode(n.Post, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.RangeStmt:
		handleRange(pf, n, fset, blocks, judgments, skipped, funcEnd)
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.SwitchStmt:
		handleSwitch(pf, n, fset, blocks, judgments, skipped)
		if n.Init != nil {
			visitNode(n.Init, fset, blocks, pf, judgments, skipped, funcEnd)
		}
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.TypeSwitchStmt:
		handleTypeSwitch(pf, n, fset, blocks, judgments, skipped)
		if n.Init != nil {
			visitNode(n.Init, fset, blocks, pf, judgments, skipped, funcEnd)
		}
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.SelectStmt:
		handleSelect(pf, n, fset, blocks, judgments, skipped)
		if n.Body != nil {
			visitNode(n.Body, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.CaseClause:
		for _, s := range n.Body {
			visitNode(s, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	case *ast.CommClause:
		for _, s := range n.Body {
			visitNode(s, fset, blocks, pf, judgments, skipped, funcEnd)
		}
	}
}

func handleIf(pf string, stmt *ast.IfStmt, fset *token.FileSet, blocks []Block, judgments *[]Judgment, skipped *int, funcEnd int) {
	thenBlock := firstBlockInRange(blocks, fset.Position(stmt.Body.Pos()).Line, fset.Position(stmt.Body.End()).Line)
	ifEndLine := fset.Position(stmt.End()).Line
	if stmt.Else != nil {
		var elseBlock *Block
		if eb, ok := stmt.Else.(*ast.BlockStmt); ok {
			elseBlock = firstBlockInRange(blocks, fset.Position(eb.Pos()).Line, fset.Position(eb.End()).Line)
		} else if ei, ok := stmt.Else.(*ast.IfStmt); ok {
			elseBlock = firstBlockInRange(blocks, fset.Position(ei.Pos()).Line, fset.Position(ei.End()).Line)
		}
		covered := thenBlock != nil && thenBlock.Count > 0 && elseBlock != nil && elseBlock.Count > 0
		detail := ""
		if !covered {
			tb := thenBlock != nil && thenBlock.Count > 0
			eb := elseBlock != nil && elseBlock.Count > 0
			if tb && !eb {
				detail = "只走过真值"
			} else if !tb && eb {
				detail = "只走过假值"
			} else {
				detail = "两臂均未走到"
			}
		}
		*judgments = append(*judgments, Judgment{
			Line:    fset.Position(stmt.Pos()).Line,
			Col:     fset.Position(stmt.Pos()).Column,
			Kind:    "if",
			Detail:  detail,
			Covered: covered,
		})
	} else {
		falseBlock := findNextBlockAfter(blocks, ifEndLine, funcEnd)
		if falseBlock == nil {
			*skipped++
			return
		}
		covered := thenBlock != nil && thenBlock.Count > 0 && falseBlock.Count > 0
		detail := ""
		if !covered {
			tb := thenBlock != nil && thenBlock.Count > 0
			fb := falseBlock.Count > 0
			if tb && !fb {
				detail = "只走过真值"
			} else if !tb && fb {
				detail = "只走过假值"
			} else {
				detail = "两臂均未走到"
			}
		}
		*judgments = append(*judgments, Judgment{
			Line:    fset.Position(stmt.Pos()).Line,
			Col:     fset.Position(stmt.Pos()).Column,
			Kind:    "if",
			Detail:  detail,
			Covered: covered,
		})
	}
}

func handleFor(pf string, stmt *ast.ForStmt, fset *token.FileSet, blocks []Block, judgments *[]Judgment, skipped *int, funcEnd int) {
	bodyBlock := firstBlockInRange(blocks, fset.Position(stmt.Body.Pos()).Line, fset.Position(stmt.Body.End()).Line)
	afterLine := fset.Position(stmt.End()).Line
	falseBlock := findNextBlockAfter(blocks, afterLine, funcEnd)
	if falseBlock == nil {
		*skipped++
		return
	}
	covered := bodyBlock != nil && bodyBlock.Count > 0 && falseBlock.Count > 0
	detail := ""
	if !covered {
		bb := bodyBlock != nil && bodyBlock.Count > 0
		fb := falseBlock.Count > 0
		if bb && !fb {
			detail = "只走过循环体"
		} else if !bb && fb {
			detail = "只走过循环后"
		} else {
			detail = "两臂均未走到"
		}
	}
	*judgments = append(*judgments, Judgment{
		Line:    fset.Position(stmt.Pos()).Line,
		Col:     fset.Position(stmt.Pos()).Column,
		Kind:    "for",
		Detail:  detail,
		Covered: covered,
	})
}

func handleRange(pf string, stmt *ast.RangeStmt, fset *token.FileSet, blocks []Block, judgments *[]Judgment, skipped *int, funcEnd int) {
	bodyBlock := firstBlockInRange(blocks, fset.Position(stmt.Body.Pos()).Line, fset.Position(stmt.Body.End()).Line)
	afterLine := fset.Position(stmt.End()).Line
	falseBlock := findNextBlockAfter(blocks, afterLine, funcEnd)
	if falseBlock == nil {
		*skipped++
		return
	}
	covered := bodyBlock != nil && bodyBlock.Count > 0 && falseBlock.Count > 0
	detail := ""
	if !covered {
		bb := bodyBlock != nil && bodyBlock.Count > 0
		fb := falseBlock.Count > 0
		if bb && !fb {
			detail = "只走过循环体"
		} else if !bb && fb {
			detail = "只走过循环后"
		} else {
			detail = "两臂均未走到"
		}
	}
	*judgments = append(*judgments, Judgment{
		Line:    fset.Position(stmt.Pos()).Line,
		Col:     fset.Position(stmt.Pos()).Column,
		Kind:    "range",
		Detail:  detail,
		Covered: covered,
	})
}

func clauseBlocks(clauses []*ast.CaseClause, fset *token.FileSet, blocks []Block) (int, int) {
	nonEmpty := 0
	walked := 0
	for _, cc := range clauses {
		if len(cc.Body) == 0 {
			continue
		}
		nonEmpty++
		start := fset.Position(cc.Pos()).Line
		end := fset.Position(cc.Body[len(cc.Body)-1].End()).Line
		b := firstBlockInRange(blocks, start, end)
		if b != nil && b.Count > 0 {
			walked++
		}
	}
	return walked, nonEmpty
}

func handleSwitch(pf string, stmt *ast.SwitchStmt, fset *token.FileSet, blocks []Block, judgments *[]Judgment, skipped *int) {
	clauses := []*ast.CaseClause{}
	if stmt.Body != nil {
		for _, s := range stmt.Body.List {
			if cc, ok := s.(*ast.CaseClause); ok {
				clauses = append(clauses, cc)
			}
		}
	}
	walked, nonEmpty := clauseBlocks(clauses, fset, blocks)
	if nonEmpty == 0 {
		*skipped++
		return
	}
	covered := walked == nonEmpty
	detail := ""
	if !covered {
		detail = fmt.Sprintf("case %d/%d 臂未走到", nonEmpty-walked, nonEmpty)
	}
	*judgments = append(*judgments, Judgment{
		Line:    fset.Position(stmt.Pos()).Line,
		Col:     fset.Position(stmt.Pos()).Column,
		Kind:    "switch",
		Detail:  detail,
		Covered: covered,
	})
}

func handleTypeSwitch(pf string, stmt *ast.TypeSwitchStmt, fset *token.FileSet, blocks []Block, judgments *[]Judgment, skipped *int) {
	clauses := []*ast.CaseClause{}
	if stmt.Body != nil {
		for _, s := range stmt.Body.List {
			if cc, ok := s.(*ast.CaseClause); ok {
				clauses = append(clauses, cc)
			}
		}
	}
	walked, nonEmpty := clauseBlocks(clauses, fset, blocks)
	if nonEmpty == 0 {
		*skipped++
		return
	}
	covered := walked == nonEmpty
	detail := ""
	if !covered {
		detail = fmt.Sprintf("case %d/%d 臂未走到", nonEmpty-walked, nonEmpty)
	}
	*judgments = append(*judgments, Judgment{
		Line:    fset.Position(stmt.Pos()).Line,
		Col:     fset.Position(stmt.Pos()).Column,
		Kind:    "switch",
		Detail:  detail,
		Covered: covered,
	})
}

func handleSelect(pf string, stmt *ast.SelectStmt, fset *token.FileSet, blocks []Block, judgments *[]Judgment, skipped *int) {
	clauses := []*ast.CommClause{}
	if stmt.Body != nil {
		for _, s := range stmt.Body.List {
			if cc, ok := s.(*ast.CommClause); ok {
				clauses = append(clauses, cc)
			}
		}
	}
	nonEmpty := 0
	walked := 0
	for _, cc := range clauses {
		if len(cc.Body) == 0 {
			continue
		}
		nonEmpty++
		start := fset.Position(cc.Body[0].Pos()).Line
		end := fset.Position(cc.Body[len(cc.Body)-1].End()).Line
		b := firstBlockInRange(blocks, start, end)
		if b != nil && b.Count > 0 {
			walked++
		}
	}
	if nonEmpty == 0 {
		*skipped++
		return
	}
	covered := walked == nonEmpty
	detail := ""
	if !covered {
		detail = fmt.Sprintf("case %d/%d 臂未走到", nonEmpty-walked, nonEmpty)
	}
	*judgments = append(*judgments, Judgment{
		Line:    fset.Position(stmt.Pos()).Line,
		Col:     fset.Position(stmt.Pos()).Column,
		Kind:    "select",
		Detail:  detail,
		Covered: covered,
	})
}

func packageOf(file string) string {
	return path.Dir(strings.ReplaceAll(file, "\\", "/"))
}

func main() {
	os.Exit(run())
}
