// Package scheduler 实现 M3.3 定时任务：基于 robfig/cron 的内嵌调度器，
// 定时执行 manifest 更新、订阅测活等任务，失败时触发告警。
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Job 是一个可被调度器执行的任务。
type Job interface {
	// Name 返回任务名（日志与告警定位用）。
	Name() string
	// Spec 返回 cron 表达式（5 字段：分 时 日 月 周）。
	Spec() string
	// Run 执行任务；返回错误时触发告警。
	Run(ctx context.Context) error
}

// AlertChannel 是告警通道（Telegram / DingTalk / SMTP 的统一抽象）。
type AlertChannel interface {
	// Send 发送告警消息。
	Send(ctx context.Context, msg string) error
	// Name 返回通道名。
	Name() string
}

// Scheduler 是内嵌 cron 调度器。
type Scheduler struct {
	cron    *cron.Cron
	jobs    map[string]cron.EntryID
	alerts  []AlertChannel
	mu      sync.Mutex
	results []JobResult
}

// JobResult 是最近一次任务执行结果。
type JobResult struct {
	JobName   string
	Success   bool
	Err       string
	StartedAt time.Time
	Duration  time.Duration
}

// New 创建调度器。
func New() *Scheduler {
	return &Scheduler{
		cron: cron.New(cron.WithLocation(time.Local)),
		jobs: make(map[string]cron.EntryID),
	}
}

// AddAlertChannel 注册告警通道。
func (s *Scheduler) AddAlertChannel(ch AlertChannel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, ch)
}

// AddJob 注册一个定时任务。
func (s *Scheduler) AddJob(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.jobs[job.Name()]; exists {
		return fmt.Errorf("任务 %q 已注册", job.Name())
	}
	entryID, err := s.cron.AddFunc(job.Spec(), func() {
		s.execute(context.Background(), job)
	})
	if err != nil {
		return fmt.Errorf("注册任务 %q (spec=%q): %w", job.Name(), job.Spec(), err)
	}
	s.jobs[job.Name()] = entryID
	return nil
}

// RemoveJob 移除一个定时任务。
func (s *Scheduler) RemoveJob(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.jobs[name]
	if !ok {
		return fmt.Errorf("任务 %q 未注册", name)
	}
	s.cron.Remove(id)
	delete(s.jobs, name)
	return nil
}

// Start 启动调度器。
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop 停止调度器并等待运行中的任务完成。
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// Results 返回最近的任务执行结果（线程安全副本）。
func (s *Scheduler) Results() []JobResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]JobResult, len(s.results))
	copy(out, s.results)
	return out
}

// execute 执行单个任务并处理告警。
// 失败重试 3 次（指数退避），全部失败后触发告警。
func (s *Scheduler) execute(ctx context.Context, job Job) {
	var lastErr error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		start := time.Now()
		err := job.Run(ctx)
		dur := time.Since(start)
		result := JobResult{
			JobName:   job.Name(),
			StartedAt: start,
			Duration:  dur,
		}
		if err == nil {
			result.Success = true
			s.recordResult(result)
			return
		}
		lastErr = err
		result.Err = err.Error()
		// 最后一次不再退避
		if attempt < maxRetries-1 {
			backoff := time.Duration(1<<attempt) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				s.recordResult(result)
				return
			}
		}
		// 记录最后一次失败
		if attempt == maxRetries-1 {
			s.recordResult(result)
		}
	}
	// 全部失败，触发告警（同步发送，便于测试断言）
	s.alertSync(ctx, job.Name(), lastErr)
}

// recordResult 记录执行结果（保留最近 100 条）。
func (s *Scheduler) recordResult(r JobResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, r)
	if len(s.results) > 100 {
		s.results = s.results[len(s.results)-100:]
	}
}

// alertSync 同步向所有告警通道发送告警。
func (s *Scheduler) alertSync(ctx context.Context, jobName string, err error) {
	s.mu.Lock()
	channels := make([]AlertChannel, len(s.alerts))
	copy(channels, s.alerts)
	s.mu.Unlock()
	msg := fmt.Sprintf("[SubForge 告警] 任务 %q 执行失败（重试 3 次后仍失败）: %v\n时间: %s",
		jobName, err, time.Now().Format(time.RFC3339))
	for _, ch := range channels {
		_ = ch.Send(ctx, msg)
	}
}

// LogAlertChannel 是把告警写到日志的通道（测试用）。
type LogAlertChannel struct {
	mu   sync.Mutex
	logs []string
}

func (l *LogAlertChannel) Name() string { return "log" }

func (l *LogAlertChannel) Send(_ context.Context, msg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, msg)
	return nil
}

// Logs 返回已记录的告警（测试用）。
func (l *LogAlertChannel) Logs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.logs))
	copy(out, l.logs)
	return out
}
