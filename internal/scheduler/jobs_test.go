package scheduler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/subforge"
)

func newTestManagerT(t *testing.T) *subforge.Manager {
	t.Helper()
	m, err := subforge.NewManager(filepath.Join(t.TempDir(), "subforge.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// TestSourceRefreshJob 后台刷新：无缓存时拉取并落缓存，新鲜时跳过，过期后重拉，0=关闭。
func TestSourceRefreshJob(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManagerT(t)
	if err := mgr.SetSettings(ctx, map[string]string{subforge.SourceRefreshKey: "30"}); err != nil {
		t.Fatalf("设置刷新间隔: %v", err)
	}
	body := "proxies:\n  - name: a\n    type: ss\n    server: 1.2.3.4\n    port: 8388\n    cipher: aes-256-gcm\n    password: x\n"
	var hits int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(body))
	}))
	defer up.Close()
	a, err := mgr.AddAirport(ctx, "a", up.URL+"/sub.yaml", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}

	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "test", Timeout: 5 * time.Second, Parallel: 2,
		Retries: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	_ = NewSourceRefreshJob(mgr, fetcher, nil) // nil logger 走默认分支
	job := NewSourceRefreshJob(mgr, fetcher, log.New(io.Discard, "", 0))
	if job.Name() != "source-refresh" || job.Spec() != DefaultSourceRefreshCron {
		t.Fatalf("name/spec = %q/%q", job.Name(), job.Spec())
	}
	job.WithSpec("*/2 * * * *")
	if job.Spec() != "*/2 * * * *" {
		t.Fatalf("WithSpec 未生效: %q", job.Spec())
	}
	job.WithSpec(DefaultSourceRefreshCron)

	// 上一轮未结束时本轮直接跳过。
	job.running.Store(true)
	if err := job.Run(ctx); err != nil {
		t.Fatalf("重叠运行应跳过: %v", err)
	}
	job.running.Store(false)

	if err := job.Run(ctx); err != nil {
		t.Fatalf("首轮刷新: %v", err)
	}
	if atomic.LoadInt32(&hits) == 0 {
		t.Fatal("无缓存时应拉取上游")
	}
	content, _, ok, err := mgr.GetSourceCache(ctx, a.SubscriptionURL)
	if err != nil || !ok || len(content) == 0 {
		t.Fatalf("应写入缓存: ok=%v err=%v", ok, err)
	}

	before := atomic.LoadInt32(&hits)
	if err := job.Run(ctx); err != nil {
		t.Fatalf("新鲜缓存轮: %v", err)
	}
	if atomic.LoadInt32(&hits) != before {
		t.Fatal("缓存新鲜时不应拉取上游")
	}

	if _, err := mgr.DB().Raw().ExecContext(ctx,
		"UPDATE source_cache SET fetched_at = ? WHERE url = ?",
		time.Now().Add(-time.Hour).Unix(), a.SubscriptionURL); err != nil {
		t.Fatalf("老化缓存: %v", err)
	}
	if err := job.Run(ctx); err != nil {
		t.Fatalf("过期刷新: %v", err)
	}
	if atomic.LoadInt32(&hits) <= before {
		t.Fatal("缓存过期后应重新拉取")
	}

	if err := mgr.SetSettings(ctx, map[string]string{subforge.SourceRefreshKey: "0"}); err != nil {
		t.Fatalf("关闭缓存: %v", err)
	}
	before = atomic.LoadInt32(&hits)
	if err := job.Run(ctx); err != nil {
		t.Fatalf("关闭后运行: %v", err)
	}
	if atomic.LoadInt32(&hits) != before {
		t.Fatal("间隔 0 时不应后台刷新")
	}

	// 配置值损坏时任务应报错（设置页写入会被拦，这里直接改库模拟脏数据）。
	if _, err := mgr.DB().Raw().ExecContext(ctx,
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		subforge.SourceRefreshKey, "abc"); err != nil {
		t.Fatalf("写入脏配置: %v", err)
	}
	if err := job.Run(ctx); err == nil {
		t.Fatal("非法间隔应返回错误")
	}
}

// TestSourceRefreshJobSkipsAndFailures 停用/空地址跳过，失败源不写缓存且不拖垮任务。
func TestSourceRefreshJobSkipsAndFailures(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManagerT(t)
	if err := mgr.SetSettings(ctx, map[string]string{subforge.SourceRefreshKey: "30"}); err != nil {
		t.Fatalf("设置刷新间隔: %v", err)
	}
	body := `proxies:
  - name: a
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: x
`
	var badHits int32
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer okSrv.Close()
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&badHits, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer badSrv.Close()

	if _, err := mgr.AddAirport(ctx, "ok", okSrv.URL, "", true); err != nil {
		t.Fatalf("AddAirport ok: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "disabled", okSrv.URL+"/d", "", false); err != nil {
		t.Fatalf("AddAirport disabled: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "empty", "", "", true); err != nil {
		t.Fatalf("AddAirport empty: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "bad", badSrv.URL, "", true); err != nil {
		t.Fatalf("AddAirport bad: %v", err)
	}

	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "test", Timeout: 5 * time.Second, Parallel: 2,
		Retries: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	job := NewSourceRefreshJob(mgr, fetcher, log.New(io.Discard, "", 0))
	if err := job.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if atomic.LoadInt32(&badHits) == 0 {
		t.Fatal("失败源应被尝试刷新")
	}
	if _, _, ok, _ := mgr.GetSourceCache(ctx, badSrv.URL); ok {
		t.Fatal("失败源不应写缓存")
	}
	if _, _, ok, _ := mgr.GetSourceCache(ctx, okSrv.URL); !ok {
		t.Fatal("健康源应写缓存")
	}
	if _, _, ok, _ := mgr.GetSourceCache(ctx, okSrv.URL+"/d"); ok {
		t.Fatal("停用机场不应刷新")
	}
}

func TestFuncJobMethods(t *testing.T) {
	called := false
	job := NewFuncJob("my-job", "*/5 * * * *", func(ctx context.Context) error {
		called = true
		return nil
	})
	if job.Name() != "my-job" {
		t.Fatalf("Name() = %q", job.Name())
	}
	if job.Spec() != "*/5 * * * *" {
		t.Fatalf("Spec() = %q", job.Spec())
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	if !called {
		t.Fatal("Run() 未调用函数")
	}
}

func TestFuncJobNilRunReturnsNil(t *testing.T) {
	job := NewFuncJob("nil-run", "* * * * *", nil)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("nil run 应返回 nil, got %v", err)
	}
}

func TestFuncJobRunPropagatesError(t *testing.T) {
	want := fmt.Errorf("boom")
	job := NewFuncJob("err-run", "* * * * *", func(ctx context.Context) error { return want })
	if err := job.Run(context.Background()); err != want {
		t.Fatalf("Run() err = %v, want %v", err, want)
	}
}

func TestRuleSourceUpdateJobNameSpec(t *testing.T) {
	m := newTestManagerT(t)
	job := NewRuleSourceUpdateJob(m, nil)
	if job.Name() != "rule-sources" {
		t.Fatalf("Name() = %q", job.Name())
	}
	if job.Spec() != DefaultRuleSourceCron {
		t.Fatalf("Spec() = %q", job.Spec())
	}
	job2 := NewRuleSourceUpdateJob(m, nil).WithSpec("1 2 3 4 5")
	if job2.Spec() != "1 2 3 4 5" {
		t.Fatalf("WithSpec Spec() = %q", job2.Spec())
	}
}

func TestRuleSourceUpdateJobUnconfiguredReturnsNil(t *testing.T) {
	m := newTestManagerT(t)
	job := NewRuleSourceUpdateJob(m, nil)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("未配置 manifest 应返回 nil, got %v", err)
	}
}

func TestRuleSourceUpdateJobConfiguredWorks(t *testing.T) {
	ctx := context.Background()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("rules:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n"))
	}))
	t.Cleanup(up.Close)

	m := newTestManagerT(t)
	outDir := filepath.Join(t.TempDir(), "out")
	manifestPath := filepath.Join(t.TempDir(), "manifest.yaml")
	content := "sources:\n  - name: a\n    url: " + up.URL + "\noutput:\n  format: clash\n  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := m.SetRuleManifestPath(ctx, manifestPath); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}

	job := NewRuleSourceUpdateJob(m, nil)
	if err := job.Run(ctx); err != nil {
		t.Fatalf("Run() err = %v", err)
	}
}

func TestSubscheckJobNameSpec(t *testing.T) {
	m := newTestManagerT(t)
	job := NewSubscheckJob(m, nil, nil)
	if job.Name() != "subscheck" {
		t.Fatalf("Name() = %q", job.Name())
	}
	if job.Spec() != DefaultSubscheckCron {
		t.Fatalf("Spec() = %q", job.Spec())
	}
	job2 := NewSubscheckJob(m, nil, nil).WithSpec("*/10 * * * *").WithClient(http.DefaultClient)
	if job2.client == nil {
		t.Fatal("WithClient 未保存客户端")
	}
	if job2.Spec() != "*/10 * * * *" {
		t.Fatalf("WithSpec Spec() = %q", job2.Spec())
	}
}

func TestSubscheckJobEmptyAirportsReturnsNil(t *testing.T) {
	m := newTestManagerT(t)
	job := NewSubscheckJob(m, nil, nil)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("空机场应返回 nil, got %v", err)
	}
}

func TestSubscheckJobWithAirportsWorks(t *testing.T) {
	ctx := context.Background()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	t.Cleanup(up.Close)

	m := newTestManagerT(t)
	airport, err := m.AddAirport(ctx, "a1", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if airport == nil {
		t.Fatal("AddAirport 返回 nil")
	}

	job := NewSubscheckJob(m, nil, nil)
	if err := job.Run(ctx); err != nil {
		t.Fatalf("Run() err = %v", err)
	}
}

func TestCronSpecFromConfigMissingKeyUsesDefault(t *testing.T) {
	ctx := context.Background()
	m := newTestManagerT(t)
	got, err := CronSpecFromConfig(ctx, m, "no.such.key", "DEFAULT")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "DEFAULT" {
		t.Fatalf("got %q, want DEFAULT", got)
	}
}

func TestCronSpecFromConfigPresentEmptyReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	m := newTestManagerT(t)
	if _, err := m.DB().Raw().ExecContext(ctx, "INSERT INTO config (key, value) VALUES (?, ?)", "empty.key", ""); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := CronSpecFromConfig(ctx, m, "empty.key", "DEFAULT")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestCronSpecFromConfigPresentValueReturnsValue(t *testing.T) {
	ctx := context.Background()
	m := newTestManagerT(t)
	if _, err := m.DB().Raw().ExecContext(ctx, "INSERT INTO config (key, value) VALUES (?, ?)", "val.key", "*/5 * * * *"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := CronSpecFromConfig(ctx, m, "val.key", "DEFAULT")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "*/5 * * * *" {
		t.Fatalf("got %q", got)
	}
}
