package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGobcoStart(t *testing.T) {
	f, l, c, ok := parseGobcoStart("a/b.go:10:5")
	if !ok || f != "a/b.go" || l != 10 || c != 5 {
		t.Fatalf("valid: got %q %d %d %v", f, l, c, ok)
	}
	if _, _, _, ok := parseGobcoStart("bad"); ok {
		t.Fatal("invalid should be false")
	}
	if _, _, _, ok := parseGobcoStart("a.go:0:1"); ok {
		t.Fatal("non-positive line should be false")
	}
}

func TestParsePositiveInt(t *testing.T) {
	if n, err := parsePositiveInt("42"); err != nil || n != 42 {
		t.Fatalf("got %d %v", n, err)
	}
	if _, err := parsePositiveInt(""); err == nil {
		t.Fatal("empty should fail")
	}
	if _, err := parsePositiveInt("0"); err == nil {
		t.Fatal("zero should fail")
	}
	if _, err := parsePositiveInt("1a"); err == nil {
		t.Fatal("non-digit should fail")
	}
}

func TestRunGobcoEngineListPackagesError(t *testing.T) {
	orig := listPackages
	defer func() { listPackages = orig }()
	listPackages = func(patterns []string) ([]string, error) {
		return nil, errFake
	}
	var stdout, stderr bytes.Buffer
	code := runGobcoEngine([]string{"./..."}, 0, 5, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("got %d", code)
	}
}

func TestRunGobcoEnginePackageFailure(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	listPackages = func(patterns []string) ([]string, error) { return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return nil, errFake }
	var stdout, stderr bytes.Buffer
	code := runGobcoEngine([]string{"./..."}, 0, 5, true, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "全部 1 个包的 gobco 都失败了") {
		t.Fatalf("missing all-failed: %s", stderr.String())
	}
}

func TestRunGobcoEngineSuccess(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	listPackages = func(patterns []string) ([]string, error) { return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) {
		return []GobcoCondition{{Start: "f.go:1:2", Code: "if x", TrueCount: 1, FalseCount: 0}}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runGobcoEngine([]string{"./..."}, 80, 5, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("min 80 should fail, got %d", code)
	}
	var stdout2, stderr2 bytes.Buffer
	code2 := runGobcoEngine([]string{"./..."}, 0, 5, true, &stdout2, &stderr2)
	if code2 != 0 {
		t.Fatalf("min 0 should pass, got %d", code2)
	}
}

func TestParseGobcoStartNoSecondColon(t *testing.T) {
	if _, _, _, ok := parseGobcoStart("a.go:5"); ok {
		t.Fatal("missing second colon should be false")
	}
}

func TestRunGobcoEngineTextAndZeroTotal(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	listPackages = func(patterns []string) ([]string, error) { return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return nil, nil }
	var stdout, stderr bytes.Buffer
	code := runGobcoEngine([]string{"./..."}, 80, 5, false, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("zero total min positive should fail, got %d", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("条件覆盖")) {
		t.Fatalf("text output missing")
	}
	// success text path
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) {
		return []GobcoCondition{{Start: "f.go:1:2", Code: "if x", TrueCount: 1, FalseCount: 1}}, nil
	}
	var stdout2, stderr2 bytes.Buffer
	code2 := runGobcoEngine([]string{"./..."}, 0, 5, false, &stdout2, &stderr2)
	if code2 != 0 {
		t.Fatalf("got %d: %s", code2, stderr2.String())
	}
	// package failure text path: all failed -> exit 2
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return nil, errFake }
	var stdout3, stderr3 bytes.Buffer
	code3 := runGobcoEngine([]string{"./..."}, 0, 5, false, &stdout3, &stderr3)
	if code3 != 2 {
		t.Fatalf("got %d: %s", code3, stderr3.String())
	}
	if !strings.Contains(stderr3.String(), "全部 1 个包的 gobco 都失败了") {
		t.Fatalf("missing all-failed: %s", stderr3.String())
	}
}

func TestParseGobcoStartEdge(t *testing.T) {
	if _, _, _, ok := parseGobcoStart(""); ok {
		t.Fatal("empty should be false")
	}
	if _, _, _, ok := parseGobcoStart(":1:2"); ok {
		t.Fatal("empty file should be false")
	}
	if _, _, _, ok := parseGobcoStart("a.go:1:"); ok {
		t.Fatal("empty col should be false")
	}
}

func TestRunGobcoEngineInvalidStartAndTop(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	listPackages = func(patterns []string) ([]string, error) { return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) {
		return []GobcoCondition{{Start: "badstart", Code: "if x", TrueCount: 0, FalseCount: 1}}, nil
	}
	var stdout, stderr bytes.Buffer
	code := runGobcoEngine([]string{"./..."}, 0, 0, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d: %s", code, stderr.String())
	}
	// top too big
	var stdout2, stderr2 bytes.Buffer
	code2 := runGobcoEngine([]string{"./..."}, 0, 999, true, &stdout2, &stderr2)
	if code2 != 0 {
		t.Fatalf("got %d: %s", code2, stderr2.String())
	}
}

func TestParseGobcoStartBadCol(t *testing.T) {
	if _, _, _, ok := parseGobcoStart("a.go:1:0"); ok {
		t.Fatal("non-positive col should be false")
	}
	if _, _, _, ok := parseGobcoStart("a.go:1:abc"); ok {
		t.Fatal("non-numeric col should be false")
	}
}

func TestParseGobcoStartLastLEZero(t *testing.T) {
	if _, _, _, ok := parseGobcoStart(":abc"); ok {
		t.Fatal("colon at start should be false")
	}
}

func TestRunGobcoEngineManyUncovered(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	listPackages = func(patterns []string) ([]string, error) { return []string{"pkg/a"}, nil }
	conds := make([]GobcoCondition, 0, 51)
	for i := 0; i < 51; i++ {
		conds = append(conds, GobcoCondition{Start: "f.go:1:2", Code: "if x", TrueCount: 0, FalseCount: 1})
	}
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return conds, nil }
	var stdout, stderr bytes.Buffer
	code := runGobcoEngine([]string{"./..."}, 0, 5, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d: %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("还有")) {
		t.Fatalf("rest message missing")
	}
}

func TestPrepareGobcoTemp(t *testing.T) {
	tmp := t.TempDir()
	stats, cleanup, err := prepareGobcoTemp(tmp)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(stats)
	if !strings.HasPrefix(root, tmp) {
		t.Fatalf("root not under tmp: %s", root)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("root not created: %v", err)
	}
	if filepath.Base(stats) != "gobco-stats.json" {
		t.Fatalf("bad stats name: %s", stats)
	}
	cleanup()
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove root: %v", err)
	}
}

func TestListPackagesDir(t *testing.T) {
	pkgs, err := listPackages([]string{"."})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("got %d pkgs: %v", len(pkgs), pkgs)
	}
	if !filepath.IsAbs(pkgs[0]) {
		t.Fatalf("not abs: %s", pkgs[0])
	}
	if strings.Contains(pkgs[0], "/vendor/") {
		t.Fatalf("vendor leaked: %s", pkgs[0])
	}
}

func TestRunGobcoEngineWithTmpDirAllFailed(t *testing.T) {
	origList := listPackages
	origRun := gobcoRun
	defer func() { listPackages = origList; gobcoRun = origRun }()
	listPackages = func(patterns []string) ([]string, error) { return []string{"pkg/a"}, nil }
	gobcoRun = func(pkg, tmpDir string) ([]GobcoCondition, error) { return nil, errFake }
	var stdout, stderr bytes.Buffer
	code := runGobcoEngineWithTmpDir([]string{"./..."}, 80, 5, false, t.TempDir(), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("got %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "全部 1 个包的 gobco 都失败了") {
		t.Fatalf("missing all-failed message: %s", stderr.String())
	}
}

func TestGobcoPackageDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	under := gobcoPackageDir(filepath.Join(cwd, "internal", "subscheck"))
	if filepath.IsAbs(under) || strings.HasPrefix(under, "..") {
		t.Fatalf("under cwd should be relative: %s", under)
	}
	outside := gobcoPackageDir("C:\\other\\pkg")
	if !filepath.IsAbs(outside) {
		t.Fatalf("outside cwd should stay absolute: %s", outside)
	}
	if got := gobcoPackageDir("internal/subscheck"); got != "internal/subscheck" {
		t.Fatalf("already relative changed: %s", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

var errFake = errString("boom")
