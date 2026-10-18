package rulebot

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// newTestLoop 构造一个带自定义格式错误模式的 Loop（该模式产出非空 Rule）。
func newTestLoop() *Loop {
	d := NewDetector()
	_ = d.AddPattern(AnomalyFormatError, "(?i)badformat", func(match []string, source string) Suggestion {
		return Suggestion{
			Action:    "prepend",
			Rule:      "DOMAIN-SUFFIX,bad.com,DIRECT",
			Reason:    "格式错误",
			AutoApply: false,
		}
	})
	q := NewPendingQueue()
	return NewLoop(d, q)
}

func TestLoop_Ingest_Normal(t *testing.T) {
	l := newTestLoop()
	n := l.Ingest("airport1", []string{"proxy badformat detected"})
	if n != 1 {
		t.Fatalf("Ingest = %d, want 1", n)
	}
	if got := len(l.queue.Pending()); got != 1 {
		t.Fatalf("Pending = %d, want 1", got)
	}
}

func TestLoop_Ingest_Dedup(t *testing.T) {
	l := newTestLoop()
	if n := l.Ingest("airport1", []string{"proxy badformat detected"}); n != 1 {
		t.Fatalf("first Ingest = %d, want 1", n)
	}
	if n := l.Ingest("airport1", []string{"proxy badformat detected"}); n != 0 {
		t.Fatalf("second Ingest = %d, want 0", n)
	}
	if got := len(l.queue.Pending()); got != 1 {
		t.Fatalf("Pending = %d, want 1", got)
	}
}

func TestLoop_Ingest_DifferentSourceSameLine(t *testing.T) {
	l := newTestLoop()
	if n := l.Ingest("airport1", []string{"proxy badformat detected"}); n != 1 {
		t.Fatalf("Ingest airport1 = %d, want 1", n)
	}
	if n := l.Ingest("airport2", []string{"proxy badformat detected"}); n != 1 {
		t.Fatalf("Ingest airport2 = %d, want 1", n)
	}
	if got := len(l.queue.Pending()); got != 2 {
		t.Fatalf("Pending = %d, want 2", got)
	}
}

func TestLoop_Ingest_NoAnomaly(t *testing.T) {
	l := newTestLoop()
	n := l.Ingest("airport1", []string{"all good, no issues", "", "   "})
	if n != 0 {
		t.Fatalf("Ingest = %d, want 0", n)
	}
	if got := len(l.queue.Pending()); got != 0 {
		t.Fatalf("Pending = %d, want 0", got)
	}
}
func TestLoop_ApplyConfirmed_Normal(t *testing.T) {
	l := newTestLoop()
	l.Ingest("airport1", []string{"proxy badformat detected"})
	if err := l.queue.Confirm(0); err != nil {
		t.Fatal(err)
	}
	var gotRules []string
	var gotSource string
	var calls int
	apply := func(ctx context.Context, source string, rules []string) error {
		calls++
		gotSource = source
		gotRules = append(gotRules, rules...)
		return nil
	}
	n, err := l.ApplyConfirmed(context.Background(), "airport1", apply)
	if err != nil {
		t.Fatalf("ApplyConfirmed: %v", err)
	}
	if n != 1 {
		t.Fatalf("n = %d, want 1", n)
	}
	if calls != 1 {
		t.Fatalf("apply calls = %d, want 1", calls)
	}
	if gotSource != "airport1" {
		t.Errorf("source = %q", gotSource)
	}
	if len(gotRules) != 1 || gotRules[0] != "DOMAIN-SUFFIX,bad.com,DIRECT" {
		t.Errorf("rules = %v", gotRules)
	}
	if got := len(l.queue.Confirmed()); got != 0 {
		t.Fatalf("Confirmed = %d, want 0", got)
	}
}

func TestLoop_ApplyConfirmed_ErrorKeepsConfirmed(t *testing.T) {
	l := newTestLoop()
	l.Ingest("airport1", []string{"proxy badformat detected"})
	if err := l.queue.Confirm(0); err != nil {
		t.Fatal(err)
	}
	apply := func(ctx context.Context, source string, rules []string) error {
		return errors.New("boom")
	}
	_, err := l.ApplyConfirmed(context.Background(), "airport1", apply)
	if err == nil {
		t.Fatal("apply 报错时 ApplyConfirmed 应返回 error")
	}
	if got := len(l.queue.Confirmed()); got != 1 {
		t.Fatalf("error 后 Confirmed = %d, want 1（保留以便重试）", got)
	}
}

func TestLoop_ApplyConfirmed_NilApply(t *testing.T) {
	l := newTestLoop()
	l.Ingest("airport1", []string{"proxy badformat detected"})
	if err := l.queue.Confirm(0); err != nil {
		t.Fatal(err)
	}
	n, err := l.ApplyConfirmed(context.Background(), "airport1", nil)
	if err != nil || n != 0 {
		t.Fatalf("nil apply = (%d, %v), want (0, nil)", n, err)
	}
}

func TestLoop_ApplyConfirmed_AllEmptyRule(t *testing.T) {
	l := newTestLoop()
	l.Ingest("airport1", []string{"duplicate rule detected"})
	if err := l.queue.Confirm(0); err != nil {
		t.Fatal(err)
	}
	called := false
	apply := func(ctx context.Context, source string, rules []string) error {
		called = true
		return nil
	}
	n, err := l.ApplyConfirmed(context.Background(), "airport1", apply)
	if err != nil || n != 0 {
		t.Fatalf("全空 Rule = (%d, %v), want (0, nil)", n, err)
	}
	if called {
		t.Fatal("apply 不应被调用")
	}
}

func TestLoop_Ingest_Concurrent(t *testing.T) {
	l := newTestLoop()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				l.Ingest("airport1", []string{"proxy badformat detected", "all good"})
			}
		}()
	}
	wg.Wait()
	if got := len(l.queue.Pending()); got != 1 {
		t.Fatalf("并发去重后 Pending = %d, want 1", got)
	}
	if n := l.Stats()["format_error"]; n != 8*50 {
		t.Errorf("format_error = %d, want %d", n, 8*50)
	}
}
