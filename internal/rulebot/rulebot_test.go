package rulebot

import (
	"testing"
)

func TestDetector_AnalyzeNodeUnreachable(t *testing.T) {
	d := NewDetector()
	a, sug := d.Analyze("proxy HK-01 is unreachable: connection refused", "airport1")
	if a == nil {
		t.Fatal("应检测到节点不可达异常")
	}
	if a.Type != AnomalyNodeUnreachable {
		t.Errorf("类型 = %v, want %v", a.Type, AnomalyNodeUnreachable)
	}
	if sug == nil {
		t.Fatal("应生成建议")
	}
	if sug.Anomaly.Type != AnomalyNodeUnreachable {
		t.Errorf("建议中的异常类型不匹配")
	}
}

func TestDetector_AnalyzeRuleConflict(t *testing.T) {
	d := NewDetector()
	a, _ := d.Analyze("duplicate rule detected: DOMAIN-SUFFIX,google.com", "template")
	if a == nil || a.Type != AnomalyRuleConflict {
		t.Fatal("应检测到规则冲突")
	}
}

func TestDetector_AnalyzeNoMatch(t *testing.T) {
	d := NewDetector()
	a, sug := d.Analyze("everything is fine, no issues", "source")
	if a != nil || sug != nil {
		t.Fatal("正常日志不应产生异常")
	}
}

func TestDetector_Stats(t *testing.T) {
	d := NewDetector()
	d.Analyze("node unreachable", "s1")
	d.Analyze("parse error here", "s2")
	d.Analyze("node timeout again", "s3")
	stats := d.Stats()
	if stats[string(AnomalyNodeUnreachable)] != 2 {
		t.Errorf("node_unreachable 计数 = %d, want 2", stats[string(AnomalyNodeUnreachable)])
	}
	if stats[string(AnomalyParseError)] != 1 {
		t.Errorf("parse_error 计数 = %d, want 1", stats[string(AnomalyParseError)])
	}
}

func TestDetector_AddPattern(t *testing.T) {
	d := NewDetector()
	err := d.AddPattern(AnomalyUnknown, "(?i)custom-error", func(match []string, source string) Suggestion {
		return Suggestion{Action: "append", Reason: "custom"}
	})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := d.Analyze("custom-error detected", "src")
	if a == nil || a.Type != AnomalyUnknown {
		t.Fatal("自定义模式应匹配")
	}
}

func TestPendingQueue_EnqueueConfirmReject(t *testing.T) {
	q := NewPendingQueue()
	q.Enqueue(&Suggestion{Reason: "s1"})
	q.Enqueue(&Suggestion{Reason: "s2"})
	q.Enqueue(&Suggestion{Reason: "s3"})
	if len(q.Pending()) != 3 {
		t.Fatalf("待确认数 = %d, want 3", len(q.Pending()))
	}
	// 确认第 1 条（索引 0）
	if err := q.Confirm(0); err != nil {
		t.Fatal(err)
	}
	if len(q.Pending()) != 2 {
		t.Fatalf("确认后待确认数 = %d, want 2", len(q.Pending()))
	}
	if len(q.Confirmed()) != 1 {
		t.Fatalf("已确认数 = %d, want 1", len(q.Confirmed()))
	}
	// 拒绝第 1 条（索引 0，现在是 s2）
	if err := q.Reject(0); err != nil {
		t.Fatal(err)
	}
	if len(q.Pending()) != 1 {
		t.Fatalf("拒绝后待确认数 = %d, want 1", len(q.Pending()))
	}
}

func TestPendingQueue_ConfirmOutOfRange(t *testing.T) {
	q := NewPendingQueue()
	if err := q.Confirm(0); err == nil {
		t.Fatal("空队列确认应报错")
	}
	if err := q.Reject(5); err == nil {
		t.Fatal("越界拒绝应报错")
	}
}

func TestPendingQueue_ClearConfirmed(t *testing.T) {
	q := NewPendingQueue()
	q.Enqueue(&Suggestion{Reason: "s1"})
	q.Confirm(0)
	if len(q.Confirmed()) != 1 {
		t.Fatal("应有 1 条已确认")
	}
	q.ClearConfirmed()
	if len(q.Confirmed()) != 0 {
		t.Fatal("清空后不应有已确认")
	}
}
