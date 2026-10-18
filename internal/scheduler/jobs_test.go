package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
	job2 := NewSubscheckJob(m, nil, nil).WithSpec("*/10 * * * *")
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
