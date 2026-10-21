package convert

import (
	"context"
	"strings"
	"testing"
)

// scriptPipelineTemplate 用 goja 脚本步骤过滤 + 改写节点（M2 §1.1 的脚本沙箱）。
// 过滤用 filter.script，改名用 op.script：两者是脚本能力的两条入口。
const scriptPipelineTemplate = `[common]
target = clash.meta
pipeline = scripted

[pipeline:scripted]
step = filter.script src="node.type === 'ss' && node.server.indexOf('hk') >= 0"
step = op.script src="node.name = '[ss] ' + node.name; node.raw.udp = true"
step = op.dedupe
on_error = abort
`

// TestEngine_ScriptSteps 脚本步骤端到端：只有 hk 的 ss 节点留下，名字被脚本改写，
// 且脚本对 node.raw 的写入真的落到产物上（raw 是唯一事实源）。
func TestEngine_ScriptSteps(t *testing.T) {
	srv := subServer(t,
		ssLink("hk1.example.com", 8388, "HK-01"),
		ssLink("jp1.example.com", 8388, "JP-01"),
		ssLink("hk2.example.com", 8388, "HK-02"),
	)
	e := withTemplate(t, newEngine(t), scriptPipelineTemplate)

	res, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.RawNodes != 3 {
		t.Fatalf("RawNodes = %d, want 3", res.RawNodes)
	}
	if res.Nodes != 2 {
		t.Fatalf("Nodes = %d, want 2（脚本只保留 hk 的 ss 节点）", res.Nodes)
	}
	out := string(res.Body)
	if strings.Contains(out, "JP-01") {
		t.Error("JP-01 应被 filter.script 过滤掉")
	}
	if !strings.Contains(out, "[ss] HK-01") || !strings.Contains(out, "[ss] HK-02") {
		t.Errorf("op.script 改名未生效:\n%s", firstLines(out, 40))
	}
}

const scriptRefPipelineTemplate = `[common]
target = clash.meta
pipeline = hk-only

[pipeline:hk-only]
step = filter.script ref=e2e-hk-filter
on_error = abort
`

// TestEngine_ScriptRefFromResolver 端到端：Engine 注入 ScriptResolver 后，
// 命名管线里的 ref= 能取到脚本源码并真的过滤掉非 HK 节点。
// 修复前 Engine 没有 resolver 字段，这一步必定报「脚本步骤需要脚本库支持」。
func TestEngine_ScriptRefFromResolver(t *testing.T) {
	srv := subServer(t,
		ssLink("hk1.example.com", 8388, "HK-01"),
		ssLink("jp1.example.com", 8388, "JP-01"),
		ssLink("us1.example.com", 8388, "US-01"),
	)
	e := withTemplate(t, newEngine(t), scriptRefPipelineTemplate)
	e.ScriptResolver = &fakeScriptResolver{enabled: true, scripts: map[string]string{
		"e2e-hk-filter": "node.name.indexOf('HK') >= 0",
	}}

	res, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.RawNodes != 3 {
		t.Fatalf("RawNodes = %d, want 3", res.RawNodes)
	}
	if res.Nodes != 1 {
		t.Fatalf("Nodes = %d, want 1（ref 脚本只保留 HK）", res.Nodes)
	}
	out := string(res.Body)
	t.Logf("探针 Engine 端到端: rawNodes=%d nodes=%d 产物含HK-01=%v 含JP-01=%v",
		res.RawNodes, res.Nodes, strings.Contains(out, "HK-01"), strings.Contains(out, "JP-01"))
	if !strings.Contains(out, "HK-01") {
		t.Errorf("产物应含 HK-01:\n%s", firstLines(out, 40))
	}
	if strings.Contains(out, "JP-01") || strings.Contains(out, "US-01") {
		t.Errorf("JP-01/US-01 应被 ref 脚本过滤掉:\n%s", firstLines(out, 40))
	}
}

// TestEngine_ScriptRefWithoutResolver 反向对照：不注入 resolver 时同一条管线
// 必须报「脚本步骤需要脚本库支持」——锁定修复前 502 的报错口径。
func TestEngine_ScriptRefWithoutResolver(t *testing.T) {
	srv := subServer(t, ssLink("hk1.example.com", 8388, "HK-01"))
	e := withTemplate(t, newEngine(t), scriptRefPipelineTemplate)

	_, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err == nil {
		t.Fatal("没有 resolver 时 ref= 应报错")
	}
	if !strings.Contains(err.Error(), "脚本步骤需要脚本库支持") {
		t.Errorf("错误信息应含 脚本步骤需要脚本库支持: %v", err)
	}
}

// TestEngine_ScriptSyntaxErrorSurfaces 脚本语法错误在配置期（编译管线）暴露，
// 且错误信息要能定位到管线名与步序号——写错脚本不该静默产出未改写的产物。
func TestEngine_ScriptSyntaxErrorSurfaces(t *testing.T) {
	srv := subServer(t, ssLink("hk1.example.com", 8388, "HK-01"))
	bad := strings.Replace(scriptPipelineTemplate,
		`src="node.name = '[ss] ' + node.name; node.raw.udp = true"`,
		`src="this is not js("`, 1)
	e := withTemplate(t, newEngine(t), bad)

	_, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srv.URL},
		Raw:    map[string][]string{},
	})
	if err == nil {
		t.Fatal("脚本语法错误应报错")
	}
	for _, want := range []string{"scripted", "op.script"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息缺少 %q: %v", want, err)
		}
	}
}
