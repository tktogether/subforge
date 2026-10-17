// Package rulebot 实现 M3.4 Rule-Bot 闭环：日志驱动的规则反馈。
//
// 流程：转换日志/告警 → 异常检测（正则+阈值）→ 生成规则建议 → 确认后提交 override。
package rulebot

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// AnomalyType 是异常类型分类。
type AnomalyType string

const (
	AnomalyNodeUnreachable AnomalyType = "node_unreachable" // 节点不可达
	AnomalyRuleConflict    AnomalyType = "rule_conflict"    // 规则冲突
	AnomalyFormatError     AnomalyType = "format_error"     // 格式错误
	AnomalyParseError      AnomalyType = "parse_error"      // 解析错误
	AnomalyUnknown         AnomalyType = "unknown"          // 未分类
)

// Anomaly 是检测到的异常。
type Anomaly struct {
	Type      AnomalyType
	Message   string
	Source    string // 来源（订阅名/URL/文件名）
	Timestamp time.Time
	Raw       string // 原始日志行
}

// Suggestion 是基于异常生成的规则建议。
type Suggestion struct {
	Anomaly   Anomaly
	Action    string // prepend / append / replace
	Rule      string // 建议的规则行，如 "DOMAIN-SUFFIX,example.com,DIRECT"
	Reason    string // 生成理由
	AutoApply bool   // 是否自动应用（通常 false，需人工确认）
}

// Detector 异常检测器。
type Detector struct {
	mu       sync.Mutex
	patterns []detectPattern
	stats    map[string]int // 按类型统计
}

type detectPattern struct {
	anomalyType AnomalyType
	re          *regexp.Regexp
	extract     func(match []string, source string) Suggestion
}

// NewDetector 创建检测器，内置常见异常模式。
func NewDetector() *Detector {
	d := &Detector{stats: make(map[string]int)}
	d.registerDefaults()
	return d
}

// registerDefaults 注册默认的异常检测模式。
func (d *Detector) registerDefaults() {
	// 节点不可达：包含 "unreachable" / "timeout" / "connection refused"
	d.patterns = append(d.patterns, detectPattern{
		anomalyType: AnomalyNodeUnreachable,
		re:          regexp.MustCompile(`(?i)(unreachable|timeout|connection refused|no route to host)`),
		extract: func(match []string, source string) Suggestion {
			return Suggestion{
				Action:    "append",
				Rule:      "",
				Reason:    fmt.Sprintf("节点不可达: %s", match[0]),
				AutoApply: false,
			}
		},
	})
	// 规则冲突：包含 "conflict" / "duplicate"
	d.patterns = append(d.patterns, detectPattern{
		anomalyType: AnomalyRuleConflict,
		re:          regexp.MustCompile(`(?i)(conflict|duplicate rule|already exists)`),
		extract: func(match []string, source string) Suggestion {
			return Suggestion{
				Action:    "replace",
				Rule:      "",
				Reason:    fmt.Sprintf("规则冲突: %s", match[0]),
				AutoApply: false,
			}
		},
	})
	// 格式错误：包含 "invalid" / "malformed" / "bad format"
	d.patterns = append(d.patterns, detectPattern{
		anomalyType: AnomalyFormatError,
		re:          regexp.MustCompile(`(?i)(invalid|malformed|bad format|unrecognized)`),
		extract: func(match []string, source string) Suggestion {
			return Suggestion{
				Action:    "prepend",
				Rule:      "",
				Reason:    fmt.Sprintf("格式错误: %s", match[0]),
				AutoApply: false,
			}
		},
	})
	// 解析错误：包含 "parse error" / "syntax"
	d.patterns = append(d.patterns, detectPattern{
		anomalyType: AnomalyParseError,
		re:          regexp.MustCompile(`(?i)(parse error|syntax error|unexpected token)`),
		extract: func(match []string, source string) Suggestion {
			return Suggestion{
				Action:    "prepend",
				Rule:      "",
				Reason:    fmt.Sprintf("解析错误: %s", match[0]),
				AutoApply: false,
			}
		},
	})
}

// AddPattern 注册自定义检测模式。
func (d *Detector) AddPattern(anomalyType AnomalyType, pattern string, extract func(match []string, source string) Suggestion) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("编译正则 %q: %w", pattern, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.patterns = append(d.patterns, detectPattern{
		anomalyType: anomalyType,
		re:          re,
		extract:     extract,
	})
	return nil
}

// Analyze 分析一条日志，返回检测到的异常与建议。
// 无异常时返回 nil, nil。
func (d *Detector) Analyze(logLine, source string) (*Anomaly, *Suggestion) {
	d.mu.Lock()
	patterns := make([]detectPattern, len(d.patterns))
	copy(patterns, d.patterns)
	d.mu.Unlock()

	for _, p := range patterns {
		if match := p.re.FindStringSubmatch(logLine); match != nil {
			a := &Anomaly{
				Type:      p.anomalyType,
				Message:   strings.TrimSpace(match[0]),
				Source:    source,
				Timestamp: time.Now(),
				Raw:       logLine,
			}
			sug := p.extract(match, source)
			sug.Anomaly = *a
			d.mu.Lock()
			d.stats[string(p.anomalyType)]++
			d.mu.Unlock()
			return a, &sug
		}
	}
	return nil, nil
}

// Stats 返回按类型统计的异常计数。
func (d *Detector) Stats() map[string]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]int, len(d.stats))
	for k, v := range d.stats {
		out[k] = v
	}
	return out
}

// PendingQueue 是待确认的建议队列。
type PendingQueue struct {
	mu        sync.Mutex
	items     []*Suggestion
	confirmed []*Suggestion
}

func NewPendingQueue() *PendingQueue {
	return &PendingQueue{}
}

// Enqueue 将建议加入待确认队列。
func (q *PendingQueue) Enqueue(s *Suggestion) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, s)
}

// Pending 返回待确认的建议列表。
func (q *PendingQueue) Pending() []*Suggestion {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Suggestion, len(q.items))
	copy(out, q.items)
	return out
}

// Confirm 确认一条建议（移到 confirmed 列表，供后续自动提交）。
func (q *PendingQueue) Confirm(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index < 0 || index >= len(q.items) {
		return fmt.Errorf("索引 %d 超出范围（共 %d 条）", index, len(q.items))
	}
	q.confirmed = append(q.confirmed, q.items[index])
	q.items = append(q.items[:index], q.items[index+1:]...)
	return nil
}

// Reject 拒绝一条建议（直接丢弃）。
func (q *PendingQueue) Reject(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index < 0 || index >= len(q.items) {
		return fmt.Errorf("索引 %d 超出范围（共 %d 条）", index, len(q.items))
	}
	q.items = append(q.items[:index], q.items[index+1:]...)
	return nil
}

// Confirmed 返回已确认的建议列表（供自动提交 override）。
func (q *PendingQueue) Confirmed() []*Suggestion {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Suggestion, len(q.confirmed))
	copy(out, q.confirmed)
	return out
}

// ClearConfirmed 清空已确认列表。
func (q *PendingQueue) ClearConfirmed() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.confirmed = nil
}
