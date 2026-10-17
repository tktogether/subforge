package config

import (
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/render"
)

func TestOverride_ParseYAML(t *testing.T) {
	src := `
groups:
  - name: AI
    type: select
    proxies: [HK, JP]
    strategy: url-test
    test: http://www.gstatic.com/generate_204
    lazy: true
rules:
  prepend:
    - DOMAIN-SUFFIX,openai.com,AI
  append:
    - DOMAIN-SUFFIX,claude.ai,AI
proxies:
  - name: HK
    server: new-hk.example.com
    port: "443"
`
	o, err := ParseOverrideYAML([]byte(src))
	if err != nil {
		t.Fatalf("ParseOverrideYAML: %v", err)
	}
	if len(o.Groups) != 1 || o.Groups[0].Name != "AI" {
		t.Fatalf("groups 解析错误: %+v", o.Groups)
	}
	g := o.Groups[0]
	if g.Type != "select" || g.Strategy != "url-test" || !g.Lazy {
		t.Errorf("group 字段解析错误: %+v", g)
	}
	if len(g.Proxies) != 2 || g.Proxies[0] != "HK" || g.Proxies[1] != "JP" {
		t.Errorf("group.proxies 解析错误: %v", g.Proxies)
	}
	if len(o.Rules.Prepend) != 1 || o.Rules.Prepend[0] != "DOMAIN-SUFFIX,openai.com,AI" {
		t.Errorf("rules.prepend 解析错误: %v", o.Rules.Prepend)
	}
	if len(o.Rules.Append) != 1 || o.Rules.Append[0] != "DOMAIN-SUFFIX,claude.ai,AI" {
		t.Errorf("rules.append 解析错误: %v", o.Rules.Append)
	}
	if len(o.Proxies) != 1 || o.Proxies[0].Server != "new-hk.example.com" || o.Proxies[0].Port != "443" {
		t.Errorf("proxies 解析错误: %+v", o.Proxies)
	}
}

func TestOverride_ParseYAML_BadYAML(t *testing.T) {
	if _, err := ParseOverrideYAML([]byte("groups: [")); err == nil {
		t.Fatal("非法 YAML 应报错")
	}
}

func TestOverride_ParseINI(t *testing.T) {
	src := `
[override.group:AI]
type = select
proxy = HK
proxy = JP
strategy = url-test
lazy = 1

[override.rules]
prepend = DOMAIN-SUFFIX,openai.com,AI
append = DOMAIN-SUFFIX,claude.ai,AI

[override.proxy:HK]
server = new-hk.example.com
port = 443
`
	o, err := ParseOverrideINI([]byte(src))
	if err != nil {
		t.Fatalf("ParseOverrideINI: %v", err)
	}
	if len(o.Groups) != 1 || o.Groups[0].Name != "AI" {
		t.Fatalf("groups 解析错误: %+v", o.Groups)
	}
	g := o.Groups[0]
	if g.Type != "select" || g.Strategy != "url-test" || !g.Lazy {
		t.Errorf("group 字段解析错误: %+v", g)
	}
	if len(g.Proxies) != 2 || g.Proxies[0] != "HK" || g.Proxies[1] != "JP" {
		t.Errorf("proxy 多值解析错误: %v", g.Proxies)
	}
	if len(o.Rules.Prepend) != 1 || o.Rules.Prepend[0] != "DOMAIN-SUFFIX,openai.com,AI" {
		t.Errorf("prepend 解析错误: %v", o.Rules.Prepend)
	}
	if len(o.Rules.Append) != 1 || o.Rules.Append[0] != "DOMAIN-SUFFIX,claude.ai,AI" {
		t.Errorf("append 解析错误: %v", o.Rules.Append)
	}
	if len(o.Proxies) != 1 || o.Proxies[0].Server != "new-hk.example.com" || o.Proxies[0].Port != "443" {
		t.Errorf("proxy 节点覆盖解析错误: %+v", o.Proxies)
	}
}

func TestOverride_ParseINI_Errors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"未知段落", "[foo]\nx = 1", "未知段落"},
		{"规则未知键", "[override.rules]\nfoo = bar", "未知键"},
		{"组未知键", "[override.group:A]\nfoo = bar", "未知键"},
		{"节点覆盖未知键", "[override.proxy:A]\nfoo = bar", "未知键"},
		{"空组名", "[override.group:]\ntype = select", "缺少 name"},
		{"空节点名", "[override.proxy:]\nserver = x", "缺少 name"},
		{"非法正则", "[override.proxy:(']\nserver = x", "非法"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOverrideINI([]byte(tc.src))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("期望含 %q 的错误，实际: %v", tc.want, err)
			}
		})
	}
}

func TestOverride_Validate_Duplicates(t *testing.T) {
	// YAML 不合并同名对象，重复定义能在 validate 里检测。
	src := `
groups:
  - name: AI
  - name: AI
`
	if _, err := ParseOverrideYAML([]byte(src)); err == nil ||
		!strings.Contains(err.Error(), "重复定义") {
		t.Fatalf("重复组名应报错: %v", err)
	}
	src = `
proxies:
  - name: HK
    server: a
  - name: HK
    server: b
`
	if _, err := ParseOverrideYAML([]byte(src)); err == nil ||
		!strings.Contains(err.Error(), "重复定义") {
		t.Fatalf("重复节点覆盖名应报错: %v", err)
	}
}

func TestMergeGroups(t *testing.T) {
	base := []render.Group{
		{Name: "AI", Type: "select", Proxies: []string{"HK", "US"}, Strategy: "url-test"},
		{Name: "Proxy", Type: "select", Proxies: []string{"HK"}},
	}
	overrides := []render.Group{
		{Name: "AI", Type: "url-test", Proxies: []string{"HK", "JP", "US"}, Strategy: "fallback"},
		{Name: "New", Type: "select", Proxies: []string{"HK"}},
	}
	merged := MergeGroups(base, overrides)
	if len(merged) != 3 {
		t.Fatalf("合并后应有 3 组，实际 %d: %+v", len(merged), merged)
	}
	ai := merged[0]
	if ai.Name != "AI" || ai.Type != "url-test" || ai.Strategy != "fallback" {
		t.Errorf("标量应替换: %+v", ai)
	}
	// 数组追加去重：HK/US 已在 base，只新增 JP。
	want := []string{"HK", "US", "JP"}
	if strings.Join(ai.Proxies, ",") != strings.Join(want, ",") {
		t.Errorf("proxies 应去重追加为 %v，实际 %v", want, ai.Proxies)
	}
	// 未在 override 里的组保持原样。
	if merged[1].Name != "Proxy" || merged[1].Type != "select" || len(merged[1].Proxies) != 1 {
		t.Errorf("未覆盖的组不应变动: %+v", merged[1])
	}
	// override 里的新组追加到末尾。
	if merged[2].Name != "New" || merged[2].Type != "select" {
		t.Errorf("新组应追加: %+v", merged[2])
	}
	// base 不应被就地修改。
	if len(base[0].Proxies) != 2 {
		t.Errorf("MergeGroups 不应修改 base: %+v", base[0])
	}
}

func TestMergeGroups_EmptyOverride(t *testing.T) {
	base := []render.Group{{Name: "AI", Type: "select"}}
	merged := MergeGroups(base, nil)
	if len(merged) != 1 || merged[0].Name != "AI" {
		t.Fatalf("空 override 应返回 base 副本: %+v", merged)
	}
	if &merged[0] == &base[0] {
		t.Error("应返回副本而非原切片元素")
	}
}

func TestMergeRules(t *testing.T) {
	base := []string{"DOMAIN-SUFFIX,cn.com,Proxy", "GEOIP,CN,DIRECT"}
	got := MergeRules(base, []string{"DOMAIN-SUFFIX,openai.com,AI"}, []string{"MATCH,Proxy"})
	want := []string{
		"DOMAIN-SUFFIX,openai.com,AI",
		"DOMAIN-SUFFIX,cn.com,Proxy",
		"GEOIP,CN,DIRECT",
		"MATCH,Proxy",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("MergeRules 顺序错误: %v", got)
	}
}

func TestToGroups(t *testing.T) {
	o := Override{Groups: []GroupOverride{
		{Name: "AI", Type: "select", Proxies: []string{"HK"}, Strategy: "url-test", Lazy: true},
	}}
	gs := o.ToGroups()
	if len(gs) != 1 {
		t.Fatalf("应有 1 组，实际 %d", len(gs))
	}
	g := gs[0]
	if g.Name != "AI" || g.Type != "select" || g.Strategy != "url-test" || !g.Lazy {
		t.Errorf("ToGroups 字段丢失: %+v", g)
	}
	if len(g.Proxies) != 1 || g.Proxies[0] != "HK" {
		t.Errorf("ToGroups proxies 丢失: %+v", g.Proxies)
	}
}
