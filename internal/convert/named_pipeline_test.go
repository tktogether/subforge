package convert

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/config"
)

// namedPipelineTemplate 是一份带命名管线的 INI 模板：
//   - [common] pipeline=clean 让模板自己默认选中 clean 管线；
//   - clean 先用 regex 过滤掉美国节点，再把 " - <后缀>" 去掉。
//
// 两步都是"看得见"的：过滤改节点数，改名改产物文本。只用其中一步的话，
// 默认装配（BuildPipeline）也能凑出同样结果，测试就分辨不出命名管线到底有没有生效。
const namedPipelineTemplate = `[common]
target = clash.meta
pipeline = clean

[pipeline:clean]
step = filter.regex include="^HK|^JP"
step = op.rename pattern=" - .*$" repl=
on_error = abort
`

// onlyUSTemplate 与上面同构，但默认选中另一条管线，用来验证请求参数能改选。
const onlyUSTemplate = `[common]
target = clash.meta
pipeline = clean

[pipeline:clean]
step = filter.regex include="^HK|^JP"

[pipeline:onlyUS]
step = filter.regex include="^US"
step = op.rename pattern=" - .*$" repl=
`

// withTemplate 给 Engine 挂上模板（等价于 CLI 的 -config / serve 的配置文件）。
func withTemplate(t *testing.T, e *Engine, body string) *Engine {
	t.Helper()
	tpl, err := config.LoadBody([]byte(body), "test.ini")
	if err != nil {
		t.Fatalf("加载模板: %v", err)
	}
	e.BaseTemplate = func() *config.Template { return tpl }
	return e
}

// TestEngine_NamedPipelineFromTemplate 模板 [common] pipeline=<名> 必须真的选中命名管线：
// 节点被 regex 过滤（US 掉队），名字里的后缀被 rename 去掉。
func TestEngine_NamedPipelineFromTemplate(t *testing.T) {
	srv := subServer(t,
		ssLink("10.10.0.1", 8388, "HK-01 - 香港"),
		ssLink("10.10.0.2", 8388, "JP-01 - 日本"),
		ssLink("10.10.0.3", 8388, "US-01 - 美国"),
	)
	e := withTemplate(t, newEngine(t), namedPipelineTemplate)

	res, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.RawNodes != 3 {
		t.Fatalf("RawNodes = %d, want 3（管线前）", res.RawNodes)
	}
	if res.Nodes != 2 {
		t.Fatalf("Nodes = %d, want 2（US 应被 filter.regex 过滤掉）", res.Nodes)
	}
	body := string(res.Body)
	if !strings.Contains(body, "HK-01") || !strings.Contains(body, "JP-01") {
		t.Error("产物应含 HK-01 / JP-01")
	}
	if strings.Contains(body, "US-01") {
		t.Error("US-01 应被命名管线的 filter 过滤掉")
	}
	if strings.Contains(body, " - 香港") || strings.Contains(body, " - 日本") {
		t.Error("命名管线的 op.rename 未生效：区域后缀仍在")
	}
}

// TestEngine_NamedPipelineSelectedByParam 请求参数 pipeline=<名> 覆盖模板默认值，
// 与 HTTP 入口的 pipeline= 走同一条解析路径。
func TestEngine_NamedPipelineSelectedByParam(t *testing.T) {
	srv := subServer(t,
		ssLink("10.11.0.1", 8388, "HK-01 - 香港"),
		ssLink("10.11.0.2", 8388, "US-01 - 美国"),
	)
	e := withTemplate(t, newEngine(t), onlyUSTemplate)

	res, err := e.Run(context.Background(), Params{
		Target:   "clash.meta",
		URLs:     []string{srv.URL},
		Pipeline: "onlyUS",
		Raw:      map[string][]string{"pipeline": {"onlyUS"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Nodes != 1 {
		t.Fatalf("Nodes = %d, want 1（onlyUS 只留美国）", res.Nodes)
	}
	out := string(res.Body)
	if !strings.Contains(out, "US-01") || strings.Contains(out, "HK-01") {
		t.Errorf("请求侧 pipeline 未生效:\n%s", firstLines(out, 40))
	}
}

// TestEngine_NamedPipelineUnknown 模板里没有这条管线时报错要指明名字，
// 而不是静默退回默认装配——静默退回会让"管线写错"变成"产物悄悄不对"。
func TestEngine_NamedPipelineUnknown(t *testing.T) {
	srv := subServer(t, ssLink("10.12.0.1", 8388, "HK-01"))
	e := withTemplate(t, newEngine(t), namedPipelineTemplate)

	_, err := e.Run(context.Background(), Params{
		Target:   "clash.meta",
		URLs:     []string{srv.URL},
		Pipeline: "nope",
		Raw:      map[string][]string{"pipeline": {"nope"}},
	})
	if err == nil {
		t.Fatal("未找到命名管线时应报错")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("错误信息应带上管线名: %v", err)
	}
}

// TestEngine_NamedPipelineFromExtra 控制台建的管线（ExtraPipelines）不依赖模板也要能用。
//
// 这条路径以前是断的：界面在「管线」页列得出名字，转换引擎却只认模板段落，
// 于是用户选中界面上的管线就会拿到 502「未找到命名管线」。
// 这里刻意一份模板都不挂，覆盖 resolveParams 里 len(chains)==0 的提前返回分支。
func TestEngine_NamedPipelineFromExtra(t *testing.T) {
	srv := subServer(t,
		ssLink("10.14.0.1", 8388, "HK-01 - 香港"),
		ssLink("10.14.0.2", 8388, "US-01 - 美国"),
	)
	e := newEngine(t)
	e.ExtraPipelines = func(context.Context) (map[string]config.PipelineSpec, error) {
		return map[string]config.PipelineSpec{
			"uiOnlyHK": {
				Name: "uiOnlyHK",
				Steps: []config.PipelineStep{
					{Kind: "filter.regex", Args: map[string]string{"include": "^HK"}},
					{Kind: "op.rename", Args: map[string]string{"pattern": " - .*$", "repl": ""}},
				},
			},
		}, nil
	}

	res, err := e.Run(context.Background(), Params{
		Target:   "clash.meta",
		URLs:     []string{srv.URL},
		Pipeline: "uiOnlyHK",
		Raw:      map[string][]string{"pipeline": {"uiOnlyHK"}},
	})
	if err != nil {
		t.Fatalf("控制台管线应能被 pipeline= 选中: %v", err)
	}
	if res.Nodes != 1 {
		t.Fatalf("Nodes = %d, want 1（uiOnlyHK 只留香港）", res.Nodes)
	}
	out := string(res.Body)
	if !strings.Contains(out, "HK-01") || strings.Contains(out, "US-01") {
		t.Errorf("控制台管线未生效:\n%s", firstLines(out, 40))
	}
	if strings.Contains(out, " - 香港") {
		t.Error("控制台管线的 op.rename 未生效：区域后缀仍在")
	}
}

// TestEngine_ExtraPipelineOverridesTemplate 同名时以控制台定义为准。
// 口径必须与 GET /api/pipelines 一致（那里 DB 行排在模板行前面并覆盖同名项），
// 否则界面显示的和实际跑起来的会是两条不同的管线。
func TestEngine_ExtraPipelineOverridesTemplate(t *testing.T) {
	srv := subServer(t,
		ssLink("10.15.0.1", 8388, "HK-01"),
		ssLink("10.15.0.2", 8388, "US-01"),
	)
	// 模板里的 clean 只留香港；控制台里的同名管线只留美国。
	e := withTemplate(t, newEngine(t), onlyUSTemplate)
	e.ExtraPipelines = func(context.Context) (map[string]config.PipelineSpec, error) {
		return map[string]config.PipelineSpec{
			"clean": {
				Name:  "clean",
				Steps: []config.PipelineStep{{Kind: "filter.regex", Args: map[string]string{"include": "^US"}}},
			},
		}, nil
	}

	res, err := e.Run(context.Background(), Params{
		Target:   "clash.meta",
		URLs:     []string{srv.URL},
		Pipeline: "clean",
		Raw:      map[string][]string{"pipeline": {"clean"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := string(res.Body)
	if !strings.Contains(out, "US-01") || strings.Contains(out, "HK-01") {
		t.Errorf("同名时应以控制台定义为准:\n%s", firstLines(out, 40))
	}
}

// TestEngine_ExtraPipelinesError 控制台管线加载失败要报错，不能静默退回默认装配。
func TestEngine_ExtraPipelinesError(t *testing.T) {
	srv := subServer(t, ssLink("10.16.0.1", 8388, "HK-01"))
	e := newEngine(t)
	e.ExtraPipelines = func(context.Context) (map[string]config.PipelineSpec, error) {
		return nil, errors.New("数据库打不开")
	}

	_, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err == nil {
		t.Fatal("控制台管线加载失败时应报错")
	}
	if !strings.Contains(err.Error(), "数据库打不开") {
		t.Errorf("错误信息应保留底层原因: %v", err)
	}
}

// TestEngine_OverrideDSLApplies 覆盖 DSL（M2 §1.2）端到端：组合并、规则 prepend/append、
// 节点级 server 覆盖三层都要落到产物上，且命中不到的 proxy 覆盖要有告警。
func TestEngine_OverrideDSLApplies(t *testing.T) {
	srv := subServer(t,
		ssLink("10.13.0.1", 8388, "HK-01"),
		ssLink("10.13.0.2", 8388, "JP-01"),
	)
	e := newEngine(t)

	ov, err := config.ParseOverrideYAML([]byte(`groups:
  - name: 手动选择
    type: select
    proxies: [HK-01]
rules:
  prepend:
    - DOMAIN-SUFFIX,openai.com,手动选择
  append:
    - MATCH,手动选择
proxies:
  - name: "^HK-"
    server: hk-new.example.com
  - name: "^NOPE-"
    server: unused.example.com
`))
	if err != nil {
		t.Fatalf("解析 override: %v", err)
	}

	res, err := e.Run(context.Background(), Params{
		Target:   "clash.meta",
		URLs:     []string{srv.URL},
		Override: ov,
		Raw:      map[string][]string{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := string(res.Body)
	for _, want := range []string{"hk-new.example.com", "手动选择", "DOMAIN-SUFFIX,openai.com,手动选择"} {
		if !strings.Contains(out, want) {
			t.Errorf("产物缺少 %q:\n%s", want, firstLines(out, 60))
		}
	}
	if !strings.Contains(out, "MATCH,手动选择") {
		t.Error("override.rules.append 未生效")
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "NOPE-") {
		t.Errorf("命中不到的 proxy 覆盖应有 1 条告警: %v", res.Warnings)
	}
}

// firstLines 截取产物前 n 行，失败信息里只放有用的部分。
func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
