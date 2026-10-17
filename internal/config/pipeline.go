package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/transform"
)

// 命名管线模板（M2 §1.1 "链式复用"）：把常见的 filter/op 组合在 INI 里命名，
// 请求侧用 pipeline=<name> 引用，避免在 URL 里堆一长串参数。
//
// 段落语法：
//
//	[pipeline:clean]
//	step = filter.regex include=^HK|^JP
//	step = op.rename pattern=" - .*$" repl=
//	step = op.dedupe
//	on_error = warn
//
// step 的取值是 "<kind> [k=v]..."，值含空格时用双引号包裹。
// on_error 是段落级默认策略，单步可用该步内的 on_error=... 覆盖。
//
// 可用 kind：filter.regex / filter.region / filter.type / filter.script /
// op.rename / op.emoji / op.dedupe / op.split / op.sort / op.script。

const pipelineSectionPrefix = "pipeline:"

// PipelineSpec 是 [pipeline:<name>] 段落解释后的命名管线定义。
type PipelineSpec struct {
	// Name 是管线名（段落名里 "pipeline:" 之后的部分）。
	Name string
	// OnError 是段落级默认错误策略；空串等价 abort。
	OnError string
	// Steps 按声明顺序排列。
	Steps []PipelineStep
}

// PipelineStep 是命名管线里的一步。
type PipelineStep struct {
	// Kind 是步骤类型（见文件头注释）。
	Kind string
	// Args 是 k=v 参数，键已归一为小写。
	Args map[string]string
	// OnError 覆盖段落级策略；空串表示沿用段落级。
	OnError string
	// Line 是定义所在行号，用于错误定位。
	Line int
}

// Compile 把命名管线定义编译成可执行的 Plan。
// 正则、排序键、脚本都在这里编译一次，配置写错在构建期暴露，而不是等请求期。
func (s PipelineSpec) Compile() (transform.Plan, error) {
	def, err := transform.ParseOnError(s.OnError)
	if err != nil {
		return transform.Plan{}, fmt.Errorf("管线 %q: %w", s.Name, err)
	}
	plan := transform.Plan{Name: s.Name}
	for i, st := range s.Steps {
		stage, err := st.build()
		if err != nil {
			return transform.Plan{}, fmt.Errorf("管线 %q 第 %d 步（%s）: %w", s.Name, i+1, st.Kind, err)
		}
		policy := def
		if st.OnError != "" {
			p, err := transform.ParseOnError(st.OnError)
			if err != nil {
				return transform.Plan{}, fmt.Errorf("管线 %q 第 %d 步（%s）: %w", s.Name, i+1, st.Kind, err)
			}
			policy = p
		}
		plan.Steps = append(plan.Steps, transform.Step{Stage: stage, OnError: policy, Label: st.Kind})
	}
	return plan, nil
}

// build 把一步编译成 Stage。
func (st PipelineStep) build() (transform.Stage, error) {
	a := st.Args
	switch st.Kind {
	case "filter.regex":
		return asStage(transform.NewRegexFilter(listArg(a["include"]), listArg(a["exclude"])))
	case "filter.region":
		return asStage(transform.NewRegionFilter(listArg(a["include"]), listArg(a["exclude"])))
	case "filter.type":
		return asStage(transform.NewTypeFilter(listArg(a["include"]), listArg(a["exclude"])))
	case "filter.script":
		d, err := timeoutArg(a)
		if err != nil {
			return nil, err
		}
		return asStage(transform.NewScriptFilter(scriptName(a, st.Kind), a["src"], d))
	case "op.rename":
		if strings.TrimSpace(a["pattern"]) == "" {
			return nil, fmt.Errorf("缺少 pattern=")
		}
		return asStage(transform.NewRename([]transform.RegexPair{{Pattern: a["pattern"], Replacement: a["repl"]}}))
	case "op.emoji":
		if strings.TrimSpace(a["pattern"]) == "" {
			return nil, fmt.Errorf("缺少 pattern=")
		}
		return asStage(transform.NewEmoji([]transform.RegexPair{{Pattern: a["pattern"], Replacement: a["repl"]}}))
	case "op.dedupe":
		return transform.Dedupe{}, nil
	case "op.split":
		return transform.NewSplit(a["sep"]), nil
	case "op.sort":
		by := strings.ToLower(strings.TrimSpace(a["by"]))
		if by == "" {
			return nil, fmt.Errorf("缺少 by=")
		}
		return asStage(transform.NewSort(transform.SortBy(by)))
	case "op.script":
		d, err := timeoutArg(a)
		if err != nil {
			return nil, err
		}
		return asStage(transform.NewScriptOperate(scriptName(a, st.Kind), a["src"], d))
	default:
		return nil, fmt.Errorf("未知步骤类型 %q（可用: filter.regex/region/type/script、op.rename/emoji/dedupe/split/sort/script）", st.Kind)
	}
}

// pipelines 收集模板里全部 [pipeline:<name>] 段落，按段落出现顺序返回。
func (t *Template) pipelines() ([]PipelineSpec, error) {
	if t == nil || t.cfg == nil {
		return nil, nil
	}
	var out []PipelineSpec
	for _, name := range t.cfg.SectionNames() {
		if len(name) < len(pipelineSectionPrefix) ||
			!strings.EqualFold(name[:len(pipelineSectionPrefix)], pipelineSectionPrefix) {
			continue
		}
		pname := strings.TrimSpace(name[len(pipelineSectionPrefix):])
		if pname == "" {
			return nil, fmt.Errorf("段落 [%s] 缺少管线名（应为 [pipeline:<名称>]）", name)
		}
		spec := PipelineSpec{Name: pname}
		for _, e := range t.cfg.Section(name).Entries() {
			switch key := strings.ToLower(strings.TrimSpace(e.Key)); key {
			case "step":
				st, err := parseStep(e.Value.Raw, e.Value.Line)
				if err != nil {
					return nil, fmt.Errorf("[%s]: %w", name, err)
				}
				spec.Steps = append(spec.Steps, st)
			case "on_error":
				if v := strings.TrimSpace(e.Value.Raw); v != "" {
					spec.OnError = v
				}
			default:
				return nil, fmt.Errorf("[%s] 未知键 %q（pipeline 段落只认 step / on_error）", name, e.Key)
			}
		}
		out = append(out, spec)
	}
	return out, nil
}

// parseStep 解析一行 step 定义："<kind> [k=v]..."。
func parseStep(raw string, line int) (PipelineStep, error) {
	tokens := splitStepTokens(raw)
	if len(tokens) == 0 {
		return PipelineStep{}, fmt.Errorf("第 %d 行的 step 为空", line)
	}
	st := PipelineStep{Kind: strings.ToLower(tokens[0]), Args: map[string]string{}, Line: line}
	for _, tok := range tokens[1:] {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			return PipelineStep{}, fmt.Errorf("第 %d 行的 step 参数 %q 应为 key=value", line, tok)
		}
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" {
			return PipelineStep{}, fmt.Errorf("第 %d 行的 step 参数 %q 缺少键名", line, tok)
		}
		st.Args[k] = v
	}
	if st.OnError = st.Args["on_error"]; st.OnError != "" {
		delete(st.Args, "on_error")
	}
	return st, nil
}

// splitStepTokens 把一步定义切成 token：按空白切分，双引号内的空白保留。
// 引号本身被剥掉，这样正则里的空格（如 " - .*$"）能原样保留。
func splitStepTokens(s string) []string {
	var (
		out     []string
		b       strings.Builder
		inQuote bool
		started bool
	)
	flush := func() {
		if started {
			out = append(out, b.String())
			b.Reset()
			started = false
		}
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			started = true
		case (r == ' ' || r == '\t') && !inQuote:
			flush()
		default:
			b.WriteRune(r)
			started = true
		}
	}
	flush()
	return out
}

// asStage 把 (具体 Stage, error) 归一成 (Stage, error)。
// 各构造函数返回的是具体类型，直接 return 无法匹配 Stage 接口的返回签名。
func asStage(s transform.Stage, err error) (transform.Stage, error) {
	if err != nil {
		return nil, err
	}
	return s, nil
}

// listArg 把 "a,b;c" 形式的参数拆成列表（逗号与分号都当分隔符）。
func listArg(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// timeoutArg 解析脚本超时；缺省返回 0，交由 transform 用内置默认值。
func timeoutArg(a map[string]string) (time.Duration, error) {
	v := strings.TrimSpace(a["timeout"])
	if v == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("timeout %q 非法: %w", v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("timeout %q 必须为正", v)
	}
	return d, nil
}

// scriptName 取脚本名；缺省回落到步骤类型，仅用于错误信息定位。
func scriptName(a map[string]string, kind string) string {
	if n := strings.TrimSpace(a["name"]); n != "" {
		return n
	}
	return kind
}
