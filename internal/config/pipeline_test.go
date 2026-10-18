package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/transform"
)

func TestPipelineSpec_Parse(t *testing.T) {
	sem := mustSem(t, `[pipeline:clean]
step = filter.regex include=^HK|^JP exclude=1
step = op.rename pattern=" - .*$" repl=
step = op.dedupe
on_error = warn
`)
	if len(sem.Pipelines) != 1 {
		t.Fatalf("应解析出 1 条命名管线，实际 %d 条", len(sem.Pipelines))
	}
	p := sem.Pipelines[0]
	if p.Name != "clean" {
		t.Errorf("Name = %q，want %q", p.Name, "clean")
	}
	if p.OnError != "warn" {
		t.Errorf("OnError = %q，want %q", p.OnError, "warn")
	}
	if len(p.Steps) != 3 {
		t.Fatalf("Steps 数量 = %d，want 3", len(p.Steps))
	}
	if p.Steps[0].Kind != "filter.regex" {
		t.Errorf("第 1 步 Kind = %q，want %q", p.Steps[0].Kind, "filter.regex")
	}
	if got := p.Steps[0].Args["include"]; got != "^HK|^JP" {
		t.Errorf("第 1 步 include = %q，want %q", got, "^HK|^JP")
	}
	// 双引号包裹的参数里的空格必须原样保留。
	if got := p.Steps[1].Args["pattern"]; got != " - .*$" {
		t.Errorf("第 2 步 pattern = %q，want %q", got, " - .*$")
	}
	if got := p.Steps[1].Args["repl"]; got != "" {
		t.Errorf("第 2 步 repl = %q，want 空", got)
	}
	if p.Steps[2].Kind != "op.dedupe" {
		t.Errorf("第 3 步 Kind = %q，want %q", p.Steps[2].Kind, "op.dedupe")
	}
}

// 段落名大小写不敏感，但管线名本身保留原样。
func TestPipelineSpec_SectionCaseInsensitive(t *testing.T) {
	sem := mustSem(t, `[Pipeline:Clean]
step = op.dedupe
`)
	if len(sem.Pipelines) != 1 || sem.Pipelines[0].Name != "Clean" {
		t.Fatalf("段落名大小写不敏感、管线名保留原样：%v", sem.Pipelines)
	}
}

func TestPipelineSpec_Compile(t *testing.T) {
	sem := mustSem(t, `[pipeline:clean]
step = filter.regex include=^HK
step = op.dedupe
on_error = warn
`)
	plan, err := sem.Pipelines[0].Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if plan.Name != "clean" {
		t.Errorf("Plan.Name = %q，want %q", plan.Name, "clean")
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("Plan.Steps 数量 = %d，want 2", len(plan.Steps))
	}
	// 段落级 on_error 作为默认策略落到每一步。
	if plan.Steps[0].OnError != transform.OnErrorWarn {
		t.Errorf("第 1 步 OnError = %q，want %q", plan.Steps[0].OnError, transform.OnErrorWarn)
	}
	if plan.Steps[0].Label != "filter.regex" {
		t.Errorf("第 1 步 Label = %q，want %q", plan.Steps[0].Label, "filter.regex")
	}
}

// 单步的 on_error=... 覆盖段落级默认。
func TestPipelineSpec_StepOverridesOnError(t *testing.T) {
	sem := mustSem(t, `[pipeline:clean]
step = filter.regex include=^HK on_error=abort
step = op.dedupe
on_error = warn
`)
	plan, err := sem.Pipelines[0].Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if plan.Steps[0].OnError != transform.OnErrorAbort {
		t.Errorf("第 1 步 OnError = %q，want %q（应被单步覆盖）", plan.Steps[0].OnError, transform.OnErrorAbort)
	}
	if plan.Steps[1].OnError != transform.OnErrorWarn {
		t.Errorf("第 2 步 OnError = %q，want %q（应沿用段落级）", plan.Steps[1].OnError, transform.OnErrorWarn)
	}
	// on_error 不该被当成普通参数误传给 Stage。
	if _, ok := sem.Pipelines[0].Steps[0].Args["on_error"]; ok {
		t.Errorf("on_error 不应留在 Args 里：%v", sem.Pipelines[0].Steps[0].Args)
	}
}

func TestPipelineSpec_CompileErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"未知步骤类型", "[pipeline:x]\nstep = filter.unknown\n"},
		{"缺 pattern", "[pipeline:x]\nstep = op.rename repl=a\n"},
		{"非法 timeout", "[pipeline:x]\nstep = filter.script src=n==1 timeout=nope\n"},
		{"非法正则", "[pipeline:x]\nstep = filter.regex include=[\n"},
		{"非法 on_error", "[pipeline:x]\nstep = op.dedupe\non_error = bogus\n"},
		{"脚本缺 src", "[pipeline:x]\nstep = filter.script timeout=1s\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sem := mustSem(t, c.body)
			if len(sem.Pipelines) == 0 {
				t.Fatalf("未解析出管线")
			}
			if _, err := sem.Pipelines[0].Compile(); err == nil {
				t.Fatalf("Compile 应报错")
			}
		})
	}
}

// 这些错误在解析段落时就暴露，走不到 Compile。
func TestPipelineSpec_SemanticsErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"缺管线名", "[pipeline:]\nstep = op.dedupe\n"},
		{"未知键", "[pipeline:x]\nfoo = bar\n"},
		{"空 step", "[pipeline:x]\nstep =\n"},
		{"参数缺等号", "[pipeline:x]\nstep = filter.regex include\n"},
		{"参数缺键名", "[pipeline:x]\nstep = op.rename =x\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tpl, err := LoadBody([]byte(c.body), "t.ini")
			if err != nil {
				t.Fatalf("LoadBody: %v", err)
			}
			if _, err := tpl.Semantics(); err == nil {
				t.Errorf("Semantics 应报错，body:\n%s", c.body)
			}
		})
	}
}

func TestSplitStepTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"filter.regex include=^HK|^JP", []string{"filter.regex", "include=^HK|^JP"}},
		{`op.rename pattern=" - .*$" repl=`, []string{"op.rename", `pattern= - .*$`, `repl=`}},
		{"  op.dedupe  ", []string{"op.dedupe"}},
		{"", nil},
	}
	for _, c := range cases {
		got := splitStepTokens(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitStepTokens(%q) = %v，want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitStepTokens(%q)[%d] = %q，want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// 同名 [pipeline:<name>] 段落由 INI 层合并，step 追加而不是覆盖。
func TestPipelineSpec_SameNameSectionsMerge(t *testing.T) {
	sem := mustSem(t, `[pipeline:clean]
step = op.dedupe

[pipeline:clean]
step = op.sort by=name
`)
	if len(sem.Pipelines) != 1 {
		t.Fatalf("同名段落应合并为 1 条，实际 %d", len(sem.Pipelines))
	}
	if len(sem.Pipelines[0].Steps) != 2 {
		t.Fatalf("step 应追加为 2 条，实际 %d", len(sem.Pipelines[0].Steps))
	}
	if sem.Pipelines[0].Steps[0].Kind != "op.dedupe" || sem.Pipelines[0].Steps[1].Kind != "op.sort" {
		t.Errorf("合并顺序：got %s,%s", sem.Pipelines[0].Steps[0].Kind, sem.Pipelines[0].Steps[1].Kind)
	}
}

func TestPipelineMap_Index(t *testing.T) {
	sem := mustSem(t, `[pipeline:clean]
step = op.dedupe

[pipeline:other]
step = op.dedupe
`)
	m := sem.PipelineMap()
	if len(m) != 2 {
		t.Fatalf("映射应含 2 条不同管线，实际 %d", len(m))
	}
	if m["clean"].Steps[0].Kind != "op.dedupe" {
		t.Errorf("clean 首步 = %q", m["clean"].Steps[0].Kind)
	}
	if _, ok := m["other"]; !ok {
		t.Errorf("other 应存在")
	}
}

// fakeResolver 是测试用的 ScriptResolver。
type fakeResolver struct {
	enabled bool
	scripts map[string]string
}

func (f *fakeResolver) ScriptsEnabled() bool { return f.enabled }

func (f *fakeResolver) ResolveScript(name string) (string, error) {
	if code, ok := f.scripts[name]; ok {
		return code, nil
	}
	return "", fmt.Errorf("脚本 %q 不存在", name)
}

func TestPipelineSpec_CompileWithResolver_RefHit(t *testing.T) {
	sem := mustSem(t, `[pipeline:p]
step = filter.script ref=foo timeout=1s
`)
	res := &fakeResolver{enabled: true, scripts: map[string]string{"foo": "true"}}
	plan, err := sem.Pipelines[0].CompileWithResolver(res)
	if err != nil {
		t.Fatalf("CompileWithResolver: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("Plan.Steps = %d，want 1", len(plan.Steps))
	}
	if _, ok := plan.Steps[0].Stage.(transform.ScriptFilter); !ok {
		t.Errorf("Stage 类型不是 ScriptFilter: %T", plan.Steps[0].Stage)
	}
}

func TestPipelineSpec_CompileWithResolver_RefNotFound(t *testing.T) {
	sem := mustSem(t, `[pipeline:p]
step = filter.script ref=nope timeout=1s
`)
	res := &fakeResolver{enabled: true, scripts: map[string]string{}}
	_, err := sem.Pipelines[0].CompileWithResolver(res)
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "解析脚本引用") {
		t.Errorf("错误应含 解析脚本引用: %v", err)
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("错误应含脚本名: %v", err)
	}
}

func TestPipelineSpec_CompileWithResolver_RefAndSrcConflict(t *testing.T) {
	sem := mustSem(t, `[pipeline:p]
step = filter.script ref=foo src=bar timeout=1s
`)
	res := &fakeResolver{enabled: true, scripts: map[string]string{"foo": "true"}}
	_, err := sem.Pipelines[0].CompileWithResolver(res)
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "不能同时给") {
		t.Errorf("错误应含 不能同时给: %v", err)
	}
}

func TestPipelineSpec_CompileWithResolver_NilResolverWithSrcCompat(t *testing.T) {
	sem := mustSem(t, "[pipeline:p]\nstep = filter.script src=x==1 timeout=1s\n")
	if _, err := sem.Pipelines[0].Compile(); err != nil {
		t.Fatalf("纯 src= 存量路径 Compile 应成功: %v", err)
	}
}

func TestPipelineSpec_CompileWithResolver_NilResolverWithRef(t *testing.T) {
	sem := mustSem(t, "[pipeline:p]\nstep = filter.script ref=foo timeout=1s\n")
	_, err := sem.Pipelines[0].Compile()
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "需要脚本库支持") {
		t.Errorf("错误应含 需要脚本库支持: %v", err)
	}
}

func TestPipelineSpec_CompileWithResolver_Disabled(t *testing.T) {
	sem := mustSem(t, `[pipeline:p]
step = filter.script src=x==1 timeout=1s
`)
	res := &fakeResolver{enabled: false, scripts: map[string]string{}}
	_, err := sem.Pipelines[0].CompileWithResolver(res)
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "脚本步骤已禁用（在「设置」里开启 script.enabled）") {
		t.Errorf("错误应逐字含禁用文案: %v", err)
	}
}

func TestPipelineSpec_CompileWithResolver_MissingSrcAndRef(t *testing.T) {
	sem := mustSem(t, `[pipeline:p]
step = filter.script timeout=1s
`)
	res := &fakeResolver{enabled: true, scripts: map[string]string{}}
	_, err := sem.Pipelines[0].CompileWithResolver(res)
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "缺少 src= 或 ref=") {
		t.Errorf("错误应含 缺少 src= 或 ref=: %v", err)
	}
}

func TestPipelineSpec_CompileNoScriptCompat(t *testing.T) {
	sem := mustSem(t, `[pipeline:clean]
step = filter.regex include=^HK
step = op.dedupe
`)
	if _, err := sem.Pipelines[0].Compile(); err != nil {
		t.Fatalf("无脚本管线 Compile 应成功: %v", err)
	}
}
