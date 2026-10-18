package rulebot

import (
	"context"
	"strings"
	"sync"
)

// Loop 把「日志摄入 → 异常检测 → 建议入队 → 已确认建议落库」串成闭环。
type Loop struct {
	detector *Detector
	queue    *PendingQueue

	mu   sync.Mutex
	seen map[string]struct{} // 去重键：异常类型 + 来源 + 原始行
}

// NewLoop 绑定一个检测器与一个待确认队列。
func NewLoop(d *Detector, q *PendingQueue) *Loop {
	return &Loop{detector: d, queue: q, seen: make(map[string]struct{})}
}

// Ingest 摄入一批日志行，返回新入队的建议条数（已见过的重复异常不重复入队）。
// 空行/无异常行跳过；同 (异常类型, 原始日志行, 来源) 只入队一次。
func (l *Loop) Ingest(source string, lines []string) int {
	added := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		_, sug := l.detector.Analyze(line, source)
		if sug == nil {
			continue
		}
		key := string(sug.Anomaly.Type) + "\x00" + source + "\x00" + line
		l.mu.Lock()
		if _, dup := l.seen[key]; dup {
			l.mu.Unlock()
			continue
		}
		l.seen[key] = struct{}{}
		l.mu.Unlock()
		l.queue.Enqueue(sug)
		added++
	}
	return added
}

// ApplyConfirmed 把已确认的建议落到规则库：收集非空 Rule，按来源 source 调 apply 一次；
// 成功后 ClearConfirmed，返回实际写入的规则条数。
// apply 为 nil 或没有非空规则时返回 (0, nil)，不报错。
// apply 返回 error 时不保证清空（保留 confirmed），原样返回 error 与已收集条数。
func (l *Loop) ApplyConfirmed(ctx context.Context, source string, apply func(ctx context.Context, source string, rules []string) error) (int, error) {
	confirmed := l.queue.Confirmed()
	var rules []string
	for _, s := range confirmed {
		if strings.TrimSpace(s.Rule) == "" {
			continue
		}
		rules = append(rules, s.Rule)
	}
	if apply == nil || len(rules) == 0 {
		return 0, nil
	}
	if err := apply(ctx, source, rules); err != nil {
		return len(rules), err
	}
	l.queue.ClearConfirmed()
	return len(rules), nil
}

// Stats 透传 detector 的异常统计。
func (l *Loop) Stats() map[string]int {
	return l.detector.Stats()
}
