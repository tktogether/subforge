package transform

import (
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/model"
)

// OnError 是单步失败时的处置策略（对应设计书 M2 §1.1 的错误处理决策）。
type OnError string

const (
	// OnErrorAbort 中止整条管线并返回错误，是默认策略。
	// 配置写错（非法正则、脚本语法错误）应当在请求期就暴露，而不是悄悄产出错误产物。
	OnErrorAbort OnError = "abort"
	// OnErrorSkip 丢弃这一步的结果、静默继续——进入该步时的节点列表原样传给下一步。
	OnErrorSkip OnError = "skip"
	// OnErrorWarn 与 skip 相同，但把失败记入 Result.Warnings。
	// 个人场景里"某条可选规则坏掉但整体仍要出配置"很常见，warn 让它在日志里可见。
	OnErrorWarn OnError = "warn"
)

// ParseOnError 解析策略名；空串按默认 abort。
func ParseOnError(s string) (OnError, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", string(OnErrorAbort):
		return OnErrorAbort, nil
	case string(OnErrorSkip):
		return OnErrorSkip, nil
	case string(OnErrorWarn):
		return OnErrorWarn, nil
	default:
		return "", fmt.Errorf("未知 on_error 策略 %q（可选: abort/skip/warn）", s)
	}
}

// Step 是管线中带错误策略的一步。
type Step struct {
	// Stage 是实际的变换；为 nil 时整步跳过。
	Stage Stage
	// OnError 是失败处置策略；零值等价于 abort。
	OnError OnError
	// Label 用于错误与告警定位；为空时回落到 Stage 的类型名。
	Label string
}

// label 返回这一步的可读标识。
func (s Step) label() string {
	if s.Label != "" {
		return s.Label
	}
	if s.Stage == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", s.Stage)
}

// Plan 是一条带错误策略的命名管线。
// 与 Pipeline 的关系：Pipeline 是"任一步失败即中止"的裸链（转换主流程仍用它），
// Plan 在其上叠加 per-step 策略与告警收集，供声明式 [pipeline] 模板使用。
type Plan struct {
	Name  string
	Steps []Step
}

// PlanResult 是 Plan 的执行结果。
type PlanResult struct {
	// Nodes 是管线后的节点列表。
	Nodes model.NodeList
	// Warnings 是 skip/warn 策略下被跳过的步骤说明；abort 不会产生告警（直接报错）。
	Warnings []string
}

// Run 按各自策略依次执行每一步。
//
//   - abort：立即返回错误，错误带上管线名、步序号与步标签；
//   - skip/warn：丢弃该步结果（保持进入该步时的节点列表），继续执行；
//     warn 额外把失败写入 Warnings。
//
// Stage 约定为纯变换（不修改入参），因此"丢弃该步结果"只需不采纳其返回值。
func (p Plan) Run(ns model.NodeList) (PlanResult, error) {
	var warnings []string
	for i, st := range p.Steps {
		if st.Stage == nil {
			continue
		}
		out, err := st.Stage.Apply(ns)
		if err == nil {
			ns = out
			continue
		}
		switch st.OnError {
		case OnErrorSkip, OnErrorWarn:
			if st.OnError == OnErrorWarn {
				warnings = append(warnings, fmt.Sprintf(
					"管线 %q 第 %d 步（%s）失败，已跳过: %v", p.Name, i+1, st.label(), err))
			}
		default:
			return PlanResult{}, fmt.Errorf("管线 %q 第 %d 步（%s）: %w", p.Name, i+1, st.label(), err)
		}
	}
	return PlanResult{Nodes: ns, Warnings: warnings}, nil
}

// PlanFrom 把一条裸 Pipeline 包成默认策略（abort）的 Plan。
// 供转换主流程在需要统一告警出口时复用。
func PlanFrom(name string, p Pipeline) Plan {
	steps := make([]Step, len(p))
	for i, s := range p {
		steps[i] = Step{Stage: s, OnError: OnErrorAbort}
	}
	return Plan{Name: name, Steps: steps}
}
