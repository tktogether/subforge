package transform

import (
	"strings"
	"testing"
)

func TestParseOnError(t *testing.T) {
	cases := []struct {
		in      string
		want    OnError
		wantErr bool
	}{
		{"", OnErrorAbort, false},
		{"abort", OnErrorAbort, false},
		{"ABORT", OnErrorAbort, false},
		{" skip ", OnErrorSkip, false},
		{"warn", OnErrorWarn, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ParseOnError(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseOnError(%q) 应报错", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseOnError(%q) 意外错误: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseOnError(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// abort 策略下错误要能定位到管线名、步序号与步标签。
func TestPlan_AbortStopsPipeline(t *testing.T) {
	p := Plan{Name: "main", Steps: []Step{
		{Stage: failingStage{}, OnError: OnErrorAbort, Label: "坏步骤"},
		{Stage: Dedupe{}, OnError: OnErrorAbort},
	}}
	_, err := p.Run(sampleNodes())
	if err == nil {
		t.Fatal("abort 策略应返回错误")
	}
	for _, want := range []string{"main", "第 1 步", "坏步骤"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息缺少 %q: %v", want, err)
		}
	}
}

// skip 静默丢弃该步结果（保持进入该步时的列表），后续步骤照常执行。
func TestPlan_SkipKeepsGoing(t *testing.T) {
	p := Plan{Name: "soft", Steps: []Step{
		{Stage: failingStage{}, OnError: OnErrorSkip},
		{Stage: Dedupe{}, OnError: OnErrorAbort},
	}}
	res, err := p.Run(sampleNodes())
	if err != nil {
		t.Fatalf("skip 不应中止: %v", err)
	}
	if len(res.Nodes) != 6 {
		t.Fatalf("节点数 = %d, want 6（7 个入参去重后）", len(res.Nodes))
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("skip 不应产生告警: %v", res.Warnings)
	}
}

// warn 与 skip 行为一致，但失败要记入 Warnings。
func TestPlan_WarnRecordsWarning(t *testing.T) {
	p := Plan{Name: "soft", Steps: []Step{
		{Stage: failingStage{}, OnError: OnErrorWarn, Label: "可选规则"},
		{Stage: Dedupe{}, OnError: OnErrorAbort},
	}}
	res, err := p.Run(sampleNodes())
	if err != nil {
		t.Fatalf("warn 不应中止: %v", err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("应有 1 条告警: %v", res.Warnings)
	}
	if !strings.Contains(res.Warnings[0], "可选规则") {
		t.Errorf("告警应含步标签: %q", res.Warnings[0])
	}
	if len(res.Nodes) != 6 {
		t.Fatalf("节点数 = %d, want 6", len(res.Nodes))
	}
}

// 零值 OnError（未显式设置）等价 abort。
func TestPlan_ZeroOnErrorIsAbort(t *testing.T) {
	p := Plan{Name: "x", Steps: []Step{{Stage: failingStage{}}}}
	if _, err := p.Run(sampleNodes()); err == nil {
		t.Fatal("零值 OnError 应按 abort 处理")
	}
}

// nil Stage 的步骤原样透传，不报错也不产生告警。
func TestPlan_NilStageSkipped(t *testing.T) {
	in := sampleNodes()
	p := Plan{Name: "x", Steps: []Step{{OnError: OnErrorAbort}}}
	res, err := p.Run(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != len(in) {
		t.Fatalf("nil 步应原样透传: %d vs %d", len(res.Nodes), len(in))
	}
}

func TestPlanFrom_WrapsPipeline(t *testing.T) {
	p := PlanFrom("convert", Pipeline{Dedupe{}})
	if p.Name != "convert" || len(p.Steps) != 1 {
		t.Fatalf("PlanFrom 结构不对: %+v", p)
	}
	if p.Steps[0].OnError != OnErrorAbort {
		t.Fatalf("PlanFrom 默认应为 abort: %q", p.Steps[0].OnError)
	}
	res, err := p.Run(sampleNodes())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 6 {
		t.Fatalf("节点数 = %d, want 6", len(res.Nodes))
	}
}
