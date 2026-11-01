package subforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ruleSourceHandler 返回一段可被 parse.Sniff 识别的规则文本。
func ruleSourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("rules:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n"))
	}
}

// writeRuleManifest 在临时目录写一个最小 manifest.yaml 并把它配成当前 manifest 路径。
func writeRuleManifest(t *testing.T, m *Manager, url, format, outDir string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	content := "sources:\n" +
		"  - name: a\n" +
		"    url: " + url + "\n" +
		"output:\n" +
		"  format: " + format + "\n" +
		"  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := m.SetRuleManifestPath(context.Background(), path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	return path
}

// seedVersions 在输出目录造两个版本文件。
func seedVersions(t *testing.T, dir, name, ext string) []string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	var names []string
	for _, v := range []string{"20240101-000000", "20240102-000000"} {
		n := name + "-" + v + "." + ext
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("写版本文件: %v", err)
		}
		names = append(names, n)
	}
	return names
}

// TestGetSetRuleManifestPath 覆盖未配置返回空串与 Set/Get 往返。
func TestGetSetRuleManifestPath(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	got, err := m.GetRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("GetRuleManifestPath: %v", err)
	}
	if got != "" {
		t.Fatalf("未配置应返回空串, got %q", got)
	}
	want := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, want); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	got2, err := m.GetRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("GetRuleManifestPath 二次: %v", err)
	}
	if got2 != want {
		t.Fatalf("Get = %q, want %q", got2, want)
	}
}

// TestLoadRuleManifestUnconfigured 覆盖未配置 manifest 路径时返回 ErrRuleManifestMissing（不再报「未配置」）。
func TestLoadRuleManifestUnconfigured(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, err := m.LoadRuleManifest(ctx)
	if err == nil {
		t.Fatal("未配置时应报错")
	}
	if !errors.Is(err, ErrRuleManifestMissing) {
		t.Fatalf("错误应 Is ErrRuleManifestMissing, got %q", err.Error())
	}
}

// TestLoadRuleManifestMissingFile 覆盖配置了不存在的文件时报错。
func TestLoadRuleManifestMissingFile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.SetRuleManifestPath(ctx, filepath.Join(t.TempDir(), "nope.yaml")); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	_, err := m.LoadRuleManifest(ctx)
	if err == nil {
		t.Fatal("文件不存在时应报错")
	}
	if !errors.Is(err, ErrRuleManifestMissing) {
		t.Fatalf("错误应 Is ErrRuleManifestMissing, got %q", err.Error())
	}
}

// TestLoadRuleManifestOK 覆盖正常解析最小 manifest。
func TestLoadRuleManifestOK(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		t.Fatalf("LoadRuleManifest: %v", err)
	}
	if len(mf.Sources) != 1 {
		t.Fatalf("Sources 长度 = %d, want 1", len(mf.Sources))
	}
	if mf.Sources[0].SourceName() != "a" {
		t.Errorf("SourceName = %q, want a", mf.Sources[0].SourceName())
	}
}

// TestListRuleSourcesEmpty 覆盖无版本时 Versions 为空、Latest 为空串（输出目录不存在也不中断）。
// TestRuleUpdateUsesOutboundClientFactory 控制台注入的出网工厂要接到规则源更新上。
func TestRuleUpdateUsesOutboundClientFactory(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	if _, err := m.SetRuleSourceOutbound(ctx, "a", "direct"); err != nil {
		t.Fatalf("SetRuleSourceOutbound: %v", err)
	}

	var mu sync.Mutex
	var keys []string
	m.SetOutboundClientFor(func(_ context.Context, outbound string) (*http.Client, error) {
		mu.Lock()
		keys = append(keys, outbound)
		mu.Unlock()
		return http.DefaultClient, nil
	})
	if _, err := m.UpdateRuleSourceOne(ctx, "a"); err != nil {
		t.Fatalf("UpdateRuleSourceOne: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keys) == 0 || keys[0] != "direct" {
		t.Fatalf("出网工厂入参 = %v, want [direct ...]", keys)
	}
}

// TestSetRuleSourceOutbound 规则源出网节点写入 manifest 并在状态里返回。
func TestSetRuleSourceOutbound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	n, err := m.AddSelfNode(ctx, "socks5://user:pass@127.0.0.1:1080", "节点A", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}

	if _, err := m.SetRuleSourceOutbound(ctx, "a", fmt.Sprint(n.ID)); err != nil {
		t.Fatalf("SetRuleSourceOutbound: %v", err)
	}
	statuses, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	if len(statuses) != 1 || statuses[0].OutboundNode != fmt.Sprint(n.ID) {
		t.Fatalf("status = %+v", statuses)
	}

	if _, err := m.SetRuleSourceOutbound(ctx, "a", "direct"); err != nil {
		t.Fatalf("direct: %v", err)
	}
	if _, err := m.SetRuleSourceOutbound(ctx, "a", "9999"); err == nil {
		t.Fatal("不存在的节点应报错")
	}
	if _, err := m.SetRuleSourceOutbound(ctx, "nope", "direct"); err == nil {
		t.Fatal("不存在的规则源应报错")
	}
}

func TestListRuleSourcesEmpty(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	statuses, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len = %d, want 1", len(statuses))
	}
	if len(statuses[0].Versions) != 0 {
		t.Fatalf("Versions = %v, want 空", statuses[0].Versions)
	}
	if statuses[0].Versions == nil {
		t.Errorf("Versions 应为非 nil 空切片")
	}
	if statuses[0].Latest != "" {
		t.Fatalf("Latest = %q, want 空串", statuses[0].Latest)
	}
	if statuses[0].Name != "a" || statuses[0].Format != "text" {
		t.Errorf("status = %+v", statuses[0])
	}
}

// TestListRuleSourcesWithVersions 覆盖有版本时 Latest == 最新版本（升序列表末尾）。
// 版本名是 YYYYMMDD-HHMMSS，升序即从旧到新，所以「最新」必须是最后一个而不是第一个。
func TestListRuleSourcesWithVersions(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	seedVersions(t, outDir, "a", "txt")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	statuses, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len = %d, want 1", len(statuses))
	}
	if len(statuses[0].Versions) != 2 {
		t.Fatalf("Versions = %v, want 2 个", statuses[0].Versions)
	}
	if got, want := statuses[0].Latest, statuses[0].Versions[len(statuses[0].Versions)-1]; got != want {
		t.Fatalf("Latest = %q, want 最新版本 %q", got, want)
	}
	for _, v := range statuses[0].Versions {
		if !strings.HasSuffix(v, ".txt") {
			t.Errorf("版本文件名 %q 应以 .txt 结尾", v)
		}
	}
}

// TestUpdateRuleSources 覆盖一次全量更新：输出目录出现文件且 Err 为 nil。
func TestUpdateRuleSources(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	results, err := m.UpdateRuleSources(ctx)
	if err != nil {
		t.Fatalf("UpdateRuleSources: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results 长度 = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("UpdateResult.Err = %v", results[0].Err)
	}
	if _, err := os.Stat(results[0].Path); err != nil {
		t.Fatalf("输出文件不存在: %v", err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	versionFiles := 0
	for _, e := range entries {
		// 输出目录里现在还有一个 .current-versions.json 状态文件，只数版本文件。
		if e.Name() == CurrentVersionsFile {
			continue
		}
		versionFiles++
	}
	if versionFiles != 1 {
		t.Fatalf("输出目录版本文件数 = %d, want 1", versionFiles)
	}
	if got := latestOf(t, m, results[0].Source); got != filepath.Base(results[0].Path) {
		t.Fatalf("更新后当前版本 = %q, want %q", got, filepath.Base(results[0].Path))
	}
}

// TestRollbackRuleSourceUnknown 覆盖 name 不存在时报错且错误含该 name。
func TestRollbackRuleSourceUnknown(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	writeRuleManifest(t, m, srv.URL, "text", filepath.Join(t.TempDir(), "out"))
	_, err := m.RollbackRuleSource(ctx, "ghost")
	if err == nil {
		t.Fatal("未知源应报错")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("错误文本 = %q, 应含 ghost", err.Error())
	}
}

// TestDefaultDataDirFromDSN 覆盖从 DSN 推导数据根目录的 5 种情形。
func TestDefaultDataDirFromDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"memory", ":memory:", ""},
		{"abs path", filepath.Join("/tmp", "a", "b.db"), filepath.Join("/tmp", "a")},
		{"bare file", "a.db", ""},
		{"query dsn", "file:x.db?cache=shared", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := defaultDataDirFromDSN(c.dsn); got != c.want {
			t.Errorf("%s: defaultDataDirFromDSN(%q) = %q, want %q", c.name, c.dsn, got, c.want)
		}
	}
}

// TestManagerDataDir 覆盖 NewManagerWithDataDir / NewManager 的 dataDir 语义。
func TestManagerDataDir(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), dir)
	if err != nil {
		t.Fatalf("NewManagerWithDataDir: %v", err)
	}
	defer func() { _ = m.Close() }()
	if m.DataDir() != dir {
		t.Fatalf("DataDir = %q, want %q", m.DataDir(), dir)
	}
	m2, err := NewManager(":memory:")
	if err != nil {
		t.Fatalf("NewManager(:memory:): %v", err)
	}
	defer func() { _ = m2.Close() }()
	if m2.DataDir() != "" {
		t.Fatalf(":memory: 的 DataDir 应为空串, got %q", m2.DataDir())
	}
}

// TestResolveRuleManifestPath 覆盖「配置优先 / 无配置有 dataDir / 无配置无 dataDir」三档。
func TestResolveRuleManifestPath(t *testing.T) {
	ctx := context.Background()

	t.Run("config wins", func(t *testing.T) {
		m := newTestManager(t)
		want := filepath.Join(t.TempDir(), "custom.yaml")
		if err := m.SetRuleManifestPath(ctx, want); err != nil {
			t.Fatalf("SetRuleManifestPath: %v", err)
		}
		got, err := m.ResolveRuleManifestPath(ctx)
		if err != nil {
			t.Fatalf("ResolveRuleManifestPath: %v", err)
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("dataDir default", func(t *testing.T) {
		m := newTestManager(t)
		got, err := m.ResolveRuleManifestPath(ctx)
		if err != nil {
			t.Fatalf("ResolveRuleManifestPath: %v", err)
		}
		want := filepath.Join(m.DataDir(), "manifest.yaml")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("cwd fallback", func(t *testing.T) {
		dir := t.TempDir()
		m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), "")
		if err != nil {
			t.Fatalf("NewManagerWithDataDir: %v", err)
		}
		defer func() { _ = m.Close() }()
		wd, werr := os.Getwd()
		if werr != nil {
			t.Fatalf("Getwd: %v", werr)
		}
		got, err := m.ResolveRuleManifestPath(ctx)
		if err != nil {
			t.Fatalf("ResolveRuleManifestPath: %v", err)
		}
		want := filepath.Join(wd, "manifest.yaml")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// TestDefaultRuleManifestPath 覆盖 DefaultRuleManifestPath 与配置无关且等于 <baseDir>/manifest.yaml。
func TestDefaultRuleManifestPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), dir)
	if err != nil {
		t.Fatalf("NewManagerWithDataDir: %v", err)
	}
	defer func() { _ = m.Close() }()

	want := filepath.Join(dir, "manifest.yaml")
	got, err := m.DefaultRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("DefaultRuleManifestPath: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// 配置了 rules.manifest_path 不应改变默认路径。
	custom := filepath.Join(t.TempDir(), "custom.yaml")
	if err := m.SetRuleManifestPath(ctx, custom); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	got2, err := m.DefaultRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("DefaultRuleManifestPath 二次: %v", err)
	}
	if got2 != want {
		t.Fatalf("配置后默认路径应不变, got %q, want %q", got2, want)
	}
	resolved, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("ResolveRuleManifestPath: %v", err)
	}
	if resolved != custom {
		t.Fatalf("ResolveRuleManifestPath 应返回配置值, got %q", resolved)
	}
}

// TestDefaultRuleManifestPathCwdFallback 覆盖 baseDir 为空时 DefaultRuleManifestPath 走 os.Getwd 分支。
func TestDefaultRuleManifestPathCwdFallback(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), "")
	if err != nil {
		t.Fatalf("NewManagerWithDataDir: %v", err)
	}
	defer func() { _ = m.Close() }()
	wd, werr := os.Getwd()
	if werr != nil {
		t.Fatalf("Getwd: %v", werr)
	}
	want := filepath.Join(wd, "manifest.yaml")
	got, err := m.DefaultRuleManifestPath(ctx)
	if err != nil {
		t.Fatalf("DefaultRuleManifestPath: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestResolveRulesOutputDir 覆盖「配置优先 / manifest output.path / 默认 dataDir/rules」。
func TestResolveRulesOutputDir(t *testing.T) {
	ctx := context.Background()

	t.Run("config wins", func(t *testing.T) {
		m := newTestManager(t)
		want := filepath.Join(t.TempDir(), "configured-out")
		if err := m.SetSettings(ctx, map[string]string{RulesOutputDirKey: want}); err != nil {
			t.Fatalf("SetSettings: %v", err)
		}
		got, err := m.ResolveRulesOutputDir(ctx)
		if err != nil {
			t.Fatalf("ResolveRulesOutputDir: %v", err)
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("manifest output path", func(t *testing.T) {
		m := newTestManager(t)
		outDir := filepath.Join(t.TempDir(), "from-manifest")
		writeRuleManifest(t, m, "http://example.com/a.yaml", "text", outDir)
		got, err := m.ResolveRulesOutputDir(ctx)
		if err != nil {
			t.Fatalf("ResolveRulesOutputDir: %v", err)
		}
		// writeRuleManifest 用 filepath.ToSlash 落盘，读回的 output.path 也是正斜杠形式。
		if got != filepath.ToSlash(outDir) {
			t.Fatalf("got %q, want %q", got, filepath.ToSlash(outDir))
		}
	})

	t.Run("default", func(t *testing.T) {
		m := newTestManager(t)
		got, err := m.ResolveRulesOutputDir(ctx)
		if err != nil {
			t.Fatalf("ResolveRulesOutputDir: %v", err)
		}
		want := filepath.Join(m.DataDir(), "rules")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// TestResolveRulesOutputDirRelativeManifest 覆盖 manifest output.path 为相对路径时解析到 manifest 所在目录下。
func TestResolveRulesOutputDirRelativeManifest(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n  - name: a\n    url: http://example.com/a.yaml\noutput:\n  format: text\n  path: ./rules\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	got, err := m.ResolveRulesOutputDir(ctx)
	if err != nil {
		t.Fatalf("ResolveRulesOutputDir: %v", err)
	}
	want := filepath.Join(dir, "rules")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestUpdateRuleSourcesRelativeManifest 覆盖 Update 把版本文件写到 manifest 相对目录下。
func TestUpdateRuleSourcesRelativeManifest(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n  - name: a\n    url: " + srv.URL + "\noutput:\n  format: text\n  path: ./rules\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	results, err := m.UpdateRuleSources(ctx)
	if err != nil {
		t.Fatalf("UpdateRuleSources: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	wantDir := filepath.Join(dir, "rules")
	if filepath.Dir(results[0].Path) != wantDir {
		t.Fatalf("输出目录 = %q, want %q", filepath.Dir(results[0].Path), wantDir)
	}
	if _, err := os.Stat(results[0].Path); err != nil {
		t.Fatalf("版本文件不存在: %v", err)
	}
}

// TestRuleManifestTemplate 覆盖模板非空且含两个源与 output。
func TestRuleManifestTemplate(t *testing.T) {
	tpl := RuleManifestTemplate()
	if strings.TrimSpace(tpl) == "" {
		t.Fatal("模板不应为空")
	}
	for _, want := range []string{"sources:", "name: reject", "name: private", "output:", "path: ./rules"} {
		if !strings.Contains(tpl, want) {
			t.Errorf("模板应含 %q", want)
		}
	}
}

// TestRuleManifestTemplateURLsAreDistinct 钉住两件事：每条公式源都是 https、
// 且两个源的地址不相同。
//
// 之前 reject 指向 meta-rules-dat 的 geoip/reject.yaml，那个路径在上游是 404，
// 于是"开箱即用"的默认清单每次更新都会留一条假失败。这里不做网络请求（单测不
// 该依赖外网），只保证两源不重复、都是 https，指向关系由文档与实测记录兜底。
func TestRuleManifestTemplateURLsAreDistinct(t *testing.T) {
	tpl := RuleManifestTemplate()
	urls := make([]string, 0, 2)
	for _, line := range strings.Split(tpl, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "url: ") {
			continue
		}
		u := strings.TrimPrefix(line, "url: ")
		if !strings.HasPrefix(u, "https://") {
			t.Errorf("规则源地址应使用 https: %q", u)
		}
		urls = append(urls, u)
	}
	if len(urls) != 2 {
		t.Fatalf("默认清单应有 2 个源，实得 %d 个: %v", len(urls), urls)
	}
	if urls[0] == urls[1] {
		t.Errorf("两个源的地址不应相同: %q", urls[0])
	}
}

// TestEnsureRuleManifestCreate 覆盖首次创建：created=true 且内容等于模板。
func TestEnsureRuleManifestCreate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), dir)
	if err != nil {
		t.Fatalf("NewManagerWithDataDir: %v", err)
	}
	defer func() { _ = m.Close() }()
	path, created, err := m.EnsureRuleManifest(ctx, false)
	if err != nil {
		t.Fatalf("EnsureRuleManifest: %v", err)
	}
	if !created {
		t.Fatal("首次应 created=true")
	}
	want := filepath.Join(dir, "manifest.yaml")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != RuleManifestTemplate() {
		t.Fatalf("文件内容不等于模板")
	}
}

// TestEnsureRuleManifestExisting 覆盖已存在且 !force：created=false 且内容不变。
func TestEnsureRuleManifestExisting(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), dir)
	if err != nil {
		t.Fatalf("NewManagerWithDataDir: %v", err)
	}
	defer func() { _ = m.Close() }()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("keep: me\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, created, err := m.EnsureRuleManifest(ctx, false)
	if err != nil {
		t.Fatalf("EnsureRuleManifest: %v", err)
	}
	if created {
		t.Fatal("已存在且 !force 应 created=false")
	}
	if got != path {
		t.Fatalf("path = %q, want %q", got, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != "keep: me\n" {
		t.Fatalf("内容不应被改动, got %q", string(b))
	}
}

// TestEnsureRuleManifestForce 覆盖 force：created=true 且旧内容备份到 .bak。
func TestEnsureRuleManifestForce(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m, err := NewManagerWithDataDir(filepath.Join(dir, "x.db"), dir)
	if err != nil {
		t.Fatalf("NewManagerWithDataDir: %v", err)
	}
	defer func() { _ = m.Close() }()
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	old := "old: content\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, created, err := m.EnsureRuleManifest(ctx, true)
	if err != nil {
		t.Fatalf("EnsureRuleManifest: %v", err)
	}
	if !created {
		t.Fatal("force 应 created=true")
	}
	if got != path {
		t.Fatalf("path = %q, want %q", got, path)
	}
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(cur) != RuleManifestTemplate() {
		t.Fatalf("force 后内容应等于模板")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("ReadFile .bak: %v", err)
	}
	if string(bak) != old {
		t.Fatalf(".bak 应等于旧内容, got %q", string(bak))
	}
}

// TestListRuleSourcesMissingManifest 覆盖 manifest 不存在时返回空切片 + nil error。
func TestListRuleSourcesMissingManifest(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	got, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("manifest 不存在不应报错, got %v", err)
	}
	if got == nil {
		t.Fatal("应返回非 nil 空切片")
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

// TestRollbackRuleSourceOK 覆盖回滚成功且返回路径存在。
func TestRollbackRuleSourceOK(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	seedVersions(t, outDir, "a", "txt")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	path, err := m.RollbackRuleSource(ctx, "a")
	if err != nil {
		t.Fatalf("RollbackRuleSource: %v", err)
	}
	if path == "" {
		t.Fatal("回滚路径为空")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("回滚路径不存在: %v", err)
	}
	okExt := false
	for _, e := range []string{".txt", ".yaml", ".mrs"} {
		if strings.HasSuffix(path, e) {
			okExt = true
		}
	}
	if !okExt {
		t.Errorf("回滚路径 %q 扩展名不在 .txt/.yaml/.mrs 中", path)
	}
}

// writeCurrentVersions 在输出目录写「当前版本」状态文件。
func writeCurrentVersions(t *testing.T, dir string, versions map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	b, err := json.Marshal(versions)
	if err != nil {
		t.Fatalf("Marshal 状态文件: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, CurrentVersionsFile), b, 0o644); err != nil {
		t.Fatalf("写状态文件: %v", err)
	}
}

// seedVersionFiles 按给定时间戳造版本文件，返回与时间戳同序（升序）的文件名。
func seedVersionFiles(t *testing.T, dir, name, ext string, stamps ...string) []string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	names := make([]string, 0, len(stamps))
	for _, v := range stamps {
		n := name + "-" + v + "." + ext
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("写版本文件: %v", err)
		}
		names = append(names, n)
	}
	return names
}

// latestOf 通过 ListRuleSources 读某源的「当前版本」，端到端断言 UI/API 看到的显示值。
func latestOf(t *testing.T, m *Manager, name string) string {
	t.Helper()
	statuses, err := m.ListRuleSources(context.Background())
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	for _, s := range statuses {
		if s.Name == name {
			return s.Latest
		}
	}
	t.Fatalf("规则源 %q 不在列表里", name)
	return ""
}

// TestListRuleSourcesCurrentVersionFromStateFile 覆盖状态文件指针优先于「最新」。
func TestListRuleSourcesCurrentVersionFromStateFile(t *testing.T) {
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	names := seedVersionFiles(t, outDir, "a", "txt", "20240101-000000", "20240102-000000", "20240103-000000")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	writeCurrentVersions(t, outDir, map[string]string{"a": names[0]})
	if got := latestOf(t, m, "a"); got != names[0] {
		t.Fatalf("Latest = %q, want 状态文件指针 %q", got, names[0])
	}
}

// TestListRuleSourcesCurrentVersionFallback 覆盖状态文件损坏或指向已删文件时回退最新版本。
func TestListRuleSourcesCurrentVersionFallback(t *testing.T) {
	cases := []struct {
		name  string
		write func(t *testing.T, dir string)
	}{
		{
			name: "内容损坏",
			write: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, CurrentVersionsFile), []byte("{ 不是合法 json"), 0o644); err != nil {
					t.Fatalf("写损坏状态文件: %v", err)
				}
			},
		},
		{
			name: "指向已删除版本",
			write: func(t *testing.T, dir string) {
				writeCurrentVersions(t, dir, map[string]string{"a": "a-20990101-000000.txt"})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			srv := httptest.NewServer(ruleSourceHandler())
			defer srv.Close()
			outDir := filepath.Join(t.TempDir(), "out")
			names := seedVersionFiles(t, outDir, "a", "txt", "20240101-000000", "20240102-000000")
			writeRuleManifest(t, m, srv.URL, "text", outDir)
			tc.write(t, outDir)
			if got := latestOf(t, m, "a"); got != names[len(names)-1] {
				t.Fatalf("Latest = %q, want 回退到最新 %q", got, names[len(names)-1])
			}
		})
	}
}

// TestRollbackRuleSourceMovesCurrentVersion 覆盖回滚真的改变当前版本、不删版本、用尽后报错。
func TestRollbackRuleSourceMovesCurrentVersion(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	names := seedVersionFiles(t, outDir, "a", "txt", "20240101-000000", "20240102-000000", "20240103-000000")
	writeRuleManifest(t, m, srv.URL, "text", outDir)

	// 初始没有指针，当前版本应是最新（升序最后一个）。
	if got := latestOf(t, m, "a"); got != names[2] {
		t.Fatalf("初始 Latest = %q, want %q", got, names[2])
	}
	// 第一次回滚：指针前移一个版本，并返回它的完整路径。
	path, err := m.RollbackRuleSource(ctx, "a")
	if err != nil {
		t.Fatalf("第一次回滚: %v", err)
	}
	if filepath.Base(path) != names[1] {
		t.Fatalf("回滚路径 = %q, want %q", path, names[1])
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("回滚路径不存在: %v", err)
	}
	if got := latestOf(t, m, "a"); got != names[1] {
		t.Fatalf("回滚后 Latest = %q, want %q（回滚必须真正改变当前版本）", got, names[1])
	}
	// 第二次回滚：再往前一个。
	path2, err := m.RollbackRuleSource(ctx, "a")
	if err != nil {
		t.Fatalf("第二次回滚: %v", err)
	}
	if filepath.Base(path2) != names[0] {
		t.Fatalf("第二次回滚路径 = %q, want %q", path2, names[0])
	}
	if got := latestOf(t, m, "a"); got != names[0] {
		t.Fatalf("第二次回滚后 Latest = %q, want %q", got, names[0])
	}
	// 已到最早版本，再回滚必须报「可回滚版本不足」，且状态不变。
	if _, err := m.RollbackRuleSource(ctx, "a"); err == nil {
		t.Fatal("已到最早版本，回滚应报错")
	} else if !errors.Is(err, ErrRollbackInsufficient) {
		t.Fatalf("错误 = %v, want ErrRollbackInsufficient", err)
	}
	if got := latestOf(t, m, "a"); got != names[0] {
		t.Fatalf("失败回滚后 Latest = %q, want 不变 %q", got, names[0])
	}
	// 回滚不得删除任何版本文件。
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txt") {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("回滚后版本数 = %d, want 3（回滚不得删版本）", count)
	}
}

// TestRollbackRuleSourceSingleVersion 覆盖只有 1 个版本时回滚报「可回滚版本不足」。
func TestRollbackRuleSourceSingleVersion(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	names := seedVersionFiles(t, outDir, "a", "txt", "20240101-000000")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	_, err := m.RollbackRuleSource(ctx, "a")
	if err == nil {
		t.Fatal("只有 1 个版本，回滚应报错")
	}
	if !errors.Is(err, ErrRollbackInsufficient) {
		t.Fatalf("错误 = %v, want ErrRollbackInsufficient", err)
	}
	if !strings.Contains(err.Error(), "可回滚版本不足") {
		t.Fatalf("错误消息 %q 未包含「可回滚版本不足」", err.Error())
	}
	if got := latestOf(t, m, "a"); got != names[0] {
		t.Fatalf("Latest = %q, want %q", got, names[0])
	}
	if _, err := os.Stat(filepath.Join(outDir, names[0])); err != nil {
		t.Fatalf("版本文件被删: %v", err)
	}
}

// TestUpdateRuleSourcesResetsCurrentVersion 覆盖成功更新会把当前版本重新指向新版本。
func TestUpdateRuleSourcesResetsCurrentVersion(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	names := seedVersionFiles(t, outDir, "a", "txt", "20240101-000000", "20240102-000000")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	// 模拟上一次回滚：指针停在旧版本上。
	writeCurrentVersions(t, outDir, map[string]string{"a": names[0]})
	results, err := m.UpdateRuleSources(ctx)
	if err != nil {
		t.Fatalf("UpdateRuleSources: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("更新结果 = %+v", results)
	}
	want := filepath.Base(results[0].Path)
	if got := latestOf(t, m, "a"); got != want {
		t.Fatalf("更新后 Latest = %q, want 新版本 %q（成功更新必须把当前版本指向新版本）", got, want)
	}
}

// TestUpdateRuleSourceOneResetsCurrentVersion 覆盖单源更新同样重设当前版本。
func TestUpdateRuleSourceOneResetsCurrentVersion(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(ruleSourceHandler())
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "out")
	names := seedVersionFiles(t, outDir, "a", "txt", "20240101-000000", "20240102-000000")
	writeRuleManifest(t, m, srv.URL, "text", outDir)
	writeCurrentVersions(t, outDir, map[string]string{"a": names[0]})
	results, err := m.UpdateRuleSourceOne(ctx, "a")
	if err != nil {
		t.Fatalf("UpdateRuleSourceOne: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("更新结果 = %+v", results)
	}
	want := filepath.Base(results[0].Path)
	if got := latestOf(t, m, "a"); got != want {
		t.Fatalf("单源更新后 Latest = %q, want 新版本 %q", got, want)
	}
}
