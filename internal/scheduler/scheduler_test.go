package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakeJob 是测试用的可控任务。
type fakeJob struct {
	name     string
	spec     string
	calls    int32
	errOnNth int32 // 第 N 次调用返回错误（1-based）
	err      error
}

func (f *fakeJob) Name() string { return f.name }
func (f *fakeJob) Spec() string { return f.spec }
func (f *fakeJob) Run(_ context.Context) error {
	n := atomic.AddInt32(&f.calls, 1)
	if f.errOnNth > 0 && n <= f.errOnNth {
		return f.err
	}
	return nil
}

func TestScheduler_AddAndRemoveJob(t *testing.T) {
	s := New()
	job := &fakeJob{name: "test", spec: "@every 1s"}
	if err := s.AddJob(job); err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	// 重复注册应报错
	if err := s.AddJob(job); err == nil {
		t.Fatal("重复注册应报错")
	}
	if err := s.RemoveJob("test"); err != nil {
		t.Fatalf("RemoveJob: %v", err)
	}
	if err := s.RemoveJob("test"); err == nil {
		t.Fatal("移除不存在的任务应报错")
	}
}

func TestScheduler_RetryAndAlert(t *testing.T) {
	s := New()
	logCh := &LogAlertChannel{}
	s.AddAlertChannel(logCh)

	// errOnNth=3：前 3 次都失败 → 3 次重试后触发告警
	job := &fakeJob{
		name:     "fail3",
		spec:     "@every 1s",
		errOnNth: 3,
		err:      errors.New("boom"),
	}
	// 直接调用 execute 测试重试逻辑
	s.execute(context.Background(), job)

	if got := atomic.LoadInt32(&job.calls); got != 3 {
		t.Fatalf("重试次数 = %d, want 3", got)
	}
	logs := logCh.Logs()
	if len(logs) != 1 {
		t.Fatalf("告警数 = %d, want 1", len(logs))
	}
}

func TestScheduler_RetrySucceedsOnSecondAttempt(t *testing.T) {
	s := New()
	logCh := &LogAlertChannel{}
	s.AddAlertChannel(logCh)

	// errOnNth=1：第 1 次失败，第 2 次成功 → 不告警
	job := &fakeJob{
		name:     "fail1",
		spec:     "@every 1s",
		errOnNth: 1,
		err:      errors.New("transient"),
	}
	s.execute(context.Background(), job)

	if got := atomic.LoadInt32(&job.calls); got != 2 {
		t.Fatalf("调用次数 = %d, want 2", got)
	}
	if len(logCh.Logs()) != 0 {
		t.Fatalf("成功重试不应告警, got %d alerts", len(logCh.Logs()))
	}
	results := s.Results()
	if len(results) != 1 {
		t.Fatalf("结果数 = %d, want 1", len(results))
	}
	if !results[0].Success {
		t.Errorf("最终结果应成功")
	}
}

func TestScheduler_ResultRetention(t *testing.T) {
	s := New()
	for i := 0; i < 110; i++ {
		s.recordResult(JobResult{JobName: "x", Success: true})
	}
	if len(s.Results()) != 100 {
		t.Fatalf("保留结果数 = %d, want 100", len(s.Results()))
	}
}

func TestScheduler_InvalidSpec(t *testing.T) {
	s := New()
	job := &fakeJob{name: "bad", spec: "not-a-cron-spec"}
	if err := s.AddJob(job); err == nil {
		t.Fatal("非法 cron 表达式应报错")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New()
	job := &fakeJob{name: "ok", spec: "@every 1s"}
	if err := s.AddJob(job); err != nil {
		t.Fatal(err)
	}
	s.Start()
	time.Sleep(2500 * time.Millisecond)
	s.Stop()
	if got := atomic.LoadInt32(&job.calls); got < 1 {
		t.Fatalf("应至少执行 1 次, got %d", got)
	}
}
