package convert

import (
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/transform"
)

func TestMergeParams_LaterWins(t *testing.T) {
	got := MergeParams(
		map[string][]string{"include": {"A"}, "sort": {"name"}},
		map[string][]string{"include": {"B"}},
		map[string][]string{"rename": {"x,y"}},
	)
	if !sameStrings(got["include"], []string{"B"}) {
		t.Errorf("include = %v，后者应整个覆盖前者", got["include"])
	}
	if !sameStrings(got["sort"], []string{"name"}) {
		t.Errorf("未覆盖的键应保留: %v", got["sort"])
	}
	if !sameStrings(got["rename"], []string{"x,y"}) {
		t.Errorf("新增键应加入: %v", got["rename"])
	}
}

// TestMergeParams_NilSafe nil 来源被跳过。
func TestMergeParams_NilSafe(t *testing.T) {
	got := MergeParams(nil, map[string][]string{"a": {"1"}})
	if !sameStrings(got["a"], []string{"1"}) {
		t.Errorf("a = %v", got["a"])
	}
}

func TestApplyTemplate_QueryBeatsTemplate(t *testing.T) {
	sem := config.Semantics{
		Params:   map[string][]string{"include": {"模板HK"}, "sort": {"name"}, "rename": {"港,Hong Kong"}},
		NodePref: transform.NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}},
		Groups:   []render.Group{{Name: "模板组", Type: "select"}},
	}
	base, err := ParseMulti(map[string][]string{"include": {"请求JP"}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := ApplyTemplate(sem, base)
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(p.Include, []string{"请求JP"}) {
		t.Errorf("Include = %v，请求参数应覆盖模板", p.Include)
	}
	if p.Sort != "name" {
		t.Errorf("Sort = %q，模板值应保留", p.Sort)
	}
	if len(p.Rename) != 1 || p.Rename[0].Pattern != "港" || p.Rename[0].Replacement != "Hong Kong" {
		t.Errorf("Rename = %v", p.Rename)
	}
	if p.NodePref.Prefixes["vless"] != "[VLESS] " {
		t.Errorf("NodePref = %v", p.NodePref)
	}
	if len(p.Groups) != 1 || p.Groups[0].Name != "模板组" {
		t.Errorf("Groups = %v", p.Groups)
	}
}

// TestApplyTemplate_RequestStructFieldsWin 请求侧自带的结构化字段优先。
func TestApplyTemplate_RequestStructFieldsWin(t *testing.T) {
	sem := config.Semantics{
		Params:   map[string][]string{},
		NodePref: transform.NodePrefSpec{Prefixes: map[string]string{"vmess": "[V2Ray] "}},
		Groups:   []render.Group{{Name: "模板组"}},
	}
	base := Params{
		NodePref: transform.NodePrefSpec{Prefixes: map[string]string{"vless": "[VLESS] "}},
		Groups:   []render.Group{{Name: "请求组"}},
		Headers:  map[string]string{"X-Token": "abc"},
		Files:    []string{"local.txt"},
		Raw:      map[string][]string{},
	}
	p, err := ApplyTemplate(sem, base)
	if err != nil {
		t.Fatal(err)
	}
	if p.NodePref.Prefixes["vless"] != "[VLESS] " || p.NodePref.Prefixes["vmess"] != "" {
		t.Errorf("NodePref = %v", p.NodePref)
	}
	// v2 起分组是合并而非覆盖：请求侧（DB 分组）在前，模板分组补在后面。
	if len(p.Groups) != 2 || p.Groups[0].Name != "请求组" || p.Groups[1].Name != "模板组" {
		t.Errorf("Groups = %v，期望 [请求组 模板组]", p.Groups)
	}
	if p.Headers["X-Token"] != "abc" || len(p.Files) != 1 {
		t.Errorf("Headers/Files 未带过来: %v %v", p.Headers, p.Files)
	}
}

// TestApplyTemplate_EmptyTemplate 空模板等于原样返回。
func TestApplyTemplate_EmptyTemplate(t *testing.T) {
	base, err := ParseMulti(map[string][]string{"target": {"clash.meta"}, "sort": {"name"}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := ApplyTemplate(config.Semantics{}, base)
	if err != nil {
		t.Fatal(err)
	}
	if p.Target != "clash.meta" || p.Sort != "name" {
		t.Errorf("参数被改了: %q %q", p.Target, p.Sort)
	}
	if !p.NodePref.Empty() || len(p.Groups) != 0 || len(p.Rules) != 0 {
		t.Errorf("空模板不应注入结构化字段: %+v", p)
	}
}

// TestApplyTemplate_BadRegex 合并后仍然要校验正则。
func TestApplyTemplate_BadRegex(t *testing.T) {
	sem := config.Semantics{Params: map[string][]string{"include": {"("}}}
	if _, err := ApplyTemplate(sem, Params{Raw: map[string][]string{}}); err == nil {
		t.Error("模板里的非法正则应报错")
	}
}

func TestResolveGroups_ExpandsRegex(t *testing.T) {
	names := []string{"HK-香港-01", "HK-香港-02", "JP-日本-01"}
	groups := ResolveGroups([]render.Group{
		{Name: "香港组", Proxies: []string{`^HK-香港-0`, "DIRECT"}},
		{Name: "全量", Proxies: []string{`.`, `^HK-香港-01$`}},
	}, names)

	want := [][]string{
		{"HK-香港-01", "HK-香港-02", "DIRECT"},
		{"HK-香港-01", "HK-香港-02", "JP-日本-01"},
	}
	if len(groups) != len(want) {
		t.Fatalf("组数 = %d", len(groups))
	}
	for i, g := range groups {
		if !sameStrings(g.Proxies, want[i]) {
			t.Errorf("组 %d 成员 = %v, want %v", i, g.Proxies, want[i])
		}
	}
}

// TestResolveGroups_LiteralMembers 组名与内置出站按字面量保留。
func TestResolveGroups_LiteralMembers(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "自动选择", Proxies: []string{".*"}},
		{Name: "上游", Proxies: []string{"自动选择", "REJECT", "DIRECT"}},
	}, []string{"HK-香港-01"})
	var up render.Group
	for _, g := range groups {
		if g.Name == "上游" {
			up = g
		}
	}
	if !sameStrings(up.Proxies, []string{"自动选择", "REJECT", "DIRECT"}) {
		t.Errorf("成员 = %v", up.Proxies)
	}
}

// TestResolveGroups_BadRegex 无法解释成正则的条目按字面量保留，不报错。
func TestResolveGroups_BadRegex(t *testing.T) {
	groups := ResolveGroups([]render.Group{{Name: "g", Proxies: []string{"("}}}, []string{"a"})
	if !sameStrings(groups[0].Proxies, []string{"("}) {
		t.Errorf("成员 = %v", groups[0].Proxies)
	}
}

// TestResolveGroups_ZeroMatchFallsBackToDirect 正则合法但没有任何节点命中时给直连兜底，
// 而不是把正则串写进配置——后者 mihomo 会当成引用了不存在的代理组而拒绝加载。
func TestResolveGroups_ZeroMatchFallsBackToDirect(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "日本组", Proxies: []string{`(JP|Japan)`}},
		{Name: "混合", Proxies: []string{`HK-香港`, `(JP|Japan)`, `(KR|Korea)`}},
	}, []string{"HK-香港-01"})
	if !sameStrings(groups[0].Proxies, []string{"DIRECT"}) {
		t.Errorf("全不命中成员 = %v", groups[0].Proxies)
	}
	if !sameStrings(groups[1].Proxies, []string{"HK-香港-01", "DIRECT"}) {
		t.Errorf("混合成员 = %v", groups[1].Proxies)
	}
}

// TestResolveGroups_EmptyMemberListFallsBackToDirect 零成员分组也要有成员：
// 渲染出 proxies: [] 会让 mihomo 拒绝加载整份配置（不是只坏这一个组）。
// subconverter 对零成员分组是直接丢弃；这里保留组名并兜底 DIRECT，
// 因为组名常被规则引用，丢组会让规则指向不存在的出站。
func TestResolveGroups_EmptyMemberListFallsBackToDirect(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "Empty", Type: "select"},
		{Name: "Good", Type: "select", Proxies: []string{".*"}},
	}, []string{"HK-01"})
	if !sameStrings(groups[0].Proxies, []string{"DIRECT"}) {
		t.Errorf("零成员组 = %v，want [DIRECT]", groups[0].Proxies)
	}
	if !sameStrings(groups[1].Proxies, []string{"HK-01"}) {
		t.Errorf("正常组 = %v，want [HK-01]", groups[1].Proxies)
	}
}

// TestResolveGroups_LookaroundTolerated 环视断言是 PCRE 写法，剥掉断言后仍要能编译匹配；
// 剥掉的代价是丢掉"白俄不算俄"这类护栏，匹配范围只会放宽。
func TestResolveGroups_LookaroundTolerated(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "俄组", Proxies: []string{`(?<!白)俄|(?<![\d\.])0?\.\d+(?![0-9])x`}},
	}, []string{"俄-01", "俄-02 0.20x", "白俄-01"})
	want := []string{"俄-01", "俄-02 0.20x", "白俄-01"}
	if !sameStrings(groups[0].Proxies, want) {
		t.Errorf("成员 = %v, want %v", groups[0].Proxies, want)
	}
}

// TestResolveGroups_NegatedMatch 整段取反写法翻译成"不匹配排除项"。
func TestResolveGroups_NegatedMatch(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "其他", Proxies: []string{`^(?!.*(HK|US)).*`}},
	}, []string{"HK-香港-01", "US-01", "SG-01", "JP-01"})
	if !sameStrings(groups[0].Proxies, []string{"SG-01", "JP-01"}) {
		t.Errorf("成员 = %v", groups[0].Proxies)
	}
}

// TestResolveGroups_NegatedMatchOuterWrap 外层裸分组包裹的取反写法同样识别。
func TestResolveGroups_NegatedMatchOuterWrap(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "其他", Proxies: []string{`(^(?!.*(HK)).*)`}},
	}, []string{"HK-01", "SG-01"})
	if !sameStrings(groups[0].Proxies, []string{"SG-01"}) {
		t.Errorf("成员 = %v", groups[0].Proxies)
	}
}

// TestResolveGroups_NegatedMatchZeroResult 取反后也可能一个都不剩，同样走直连兜底。
func TestResolveGroups_NegatedMatchZeroResult(t *testing.T) {
	groups := ResolveGroups([]render.Group{
		{Name: "其他", Proxies: []string{`^(?!.*X).*`}},
	}, []string{"X-01", "X-02"})
	if !sameStrings(groups[0].Proxies, []string{"DIRECT"}) {
		t.Errorf("成员 = %v", groups[0].Proxies)
	}
}

func TestResolveGroups_Empty(t *testing.T) {
	if got := ResolveGroups(nil, []string{"a"}); got != nil {
		t.Errorf("nil 输入应返回 nil，实际 %v", got)
	}
}

// TestApplyTemplate_MergeGroups 分组按层合并：高优先层在前，同名保留高优先层。
func TestApplyTemplate_MergeGroups(t *testing.T) {
	sem := config.Semantics{
		Params: map[string][]string{},
		Groups: []render.Group{
			{Name: "模板独有", Type: "select"},
			{Name: "同名", Type: "url-test"},
		},
	}
	base := Params{
		Raw: map[string][]string{},
		Groups: []render.Group{
			{Name: "自定义组", Type: "select"},
			{Name: "同名", Type: "select"},
			{Name: "", Type: "select"},
		},
	}
	p, err := ApplyTemplate(sem, base)
	if err != nil {
		t.Fatal(err)
	}
	// 空名分组被丢掉；同名只留 base 的定义（type select）。
	want := []string{"自定义组", "同名", "模板独有"}
	got := make([]string, 0, len(p.Groups))
	for _, g := range p.Groups {
		got = append(got, g.Name)
	}
	if !sameStrings(got, want) {
		t.Fatalf("分组顺序 = %v，期望 %v", got, want)
	}
	if p.Groups[1].Type != "select" {
		t.Fatalf("同名分组 type=%q，应保留高优先层的 select", p.Groups[1].Type)
	}
}

// TestApplyTemplate_DisableTemplateGroups 关掉模板层分组：只剩 DB 分组，并留一条可见告警。
func TestApplyTemplate_DisableTemplateGroups(t *testing.T) {
	sem := config.Semantics{
		Params: map[string][]string{},
		Groups: []render.Group{{Name: "模板组"}},
	}
	base := Params{
		Raw:                   map[string][]string{},
		Groups:                []render.Group{{Name: "自定义组"}},
		DisableTemplateGroups: true,
	}
	p, err := ApplyTemplate(sem, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Groups) != 1 || p.Groups[0].Name != "自定义组" {
		t.Fatalf("Groups = %v，期望只剩自定义组", p.Groups)
	}
	if !containsWarning(p.TemplateWarnings, "模板分组已跳过") {
		t.Fatalf("TemplateWarnings = %v，期望含「模板分组已跳过」", p.TemplateWarnings)
	}
}

// TestApplyTemplate_DisableTemplateRules 关掉模板层规则：规则流被清空，不再展开成 MATCH 之类。
func TestApplyTemplate_DisableTemplateRules(t *testing.T) {
	sem := config.Semantics{
		Params: map[string][]string{},
		RuleItems: []config.RuleItem{
			{Remote: -1, Line: "GEOIP,CN,DIRECT"},
		},
		RemoteRulesets: []config.RemoteRuleset{{URL: "https://example.com/x.list", Kind: "classical", Group: "G"}},
	}
	base := Params{
		Raw:                  map[string][]string{},
		Rules:                []string{"DOMAIN-SUFFIX,db.test,DIRECT"},
		DisableTemplateRules: true,
	}
	p, err := ApplyTemplate(sem, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.TemplateRules) != 0 || len(p.TemplateRemoteRulesets) != 0 {
		t.Fatalf("模板规则流未清空: %d %d", len(p.TemplateRules), len(p.TemplateRemoteRulesets))
	}
	if len(p.Rules) != 1 || p.Rules[0] != "DOMAIN-SUFFIX,db.test,DIRECT" {
		t.Fatalf("DB 规则不应被影响: %v", p.Rules)
	}
	if !containsWarning(p.TemplateWarnings, "模板规则已跳过") {
		t.Fatalf("TemplateWarnings = %v，期望含「模板规则已跳过」", p.TemplateWarnings)
	}
}

// TestApplyTemplateKeepsBaseTemplateWarnings 调用方已攒的告警不能被模板层覆盖掉：
// 订阅侧会先塞「已用规则源产物替换模板规则：注入 N 条」这类提示，
// 直接赋值成 sem.Warnings 会让用户在控制台看不到产物没生效的原因。
func TestApplyTemplateKeepsBaseTemplateWarnings(t *testing.T) {
	sem := config.Semantics{
		Params: map[string][]string{},
	}
	base := Params{
		Raw:              map[string][]string{},
		TemplateWarnings: []string{"已用规则源产物替换模板规则：注入 2 条"},
	}
	p, err := ApplyTemplate(sem, base)
	if err != nil {
		t.Fatal(err)
	}
	if !containsWarning(p.TemplateWarnings, "已用规则源产物替换模板规则：注入 2 条") {
		t.Fatalf("TemplateWarnings = %v，期望保留调用方已攒的告警", p.TemplateWarnings)
	}
	if len(p.TemplateWarnings) != 1 {
		t.Fatalf("TemplateWarnings = %v，期望只有调用方那一条（模板本次没有告警）", p.TemplateWarnings)
	}

	// 模板层规则被停用时，告警文案要说明「被规则源产物替换」这条真实路径，
	// 而不只是「方案未启用 template 层」。
	sem2 := config.Semantics{Params: map[string][]string{}}
	base2 := Params{
		Raw:                  map[string][]string{},
		DisableTemplateRules: true,
		TemplateWarnings:     []string{"caller-warn"},
	}
	p2, err := ApplyTemplate(sem2, base2)
	if err != nil {
		t.Fatal(err)
	}
	if !containsWarning(p2.TemplateWarnings, "caller-warn") {
		t.Fatalf("TemplateWarnings = %v，期望保留 caller-warn", p2.TemplateWarnings)
	}
	if !containsWarning(p2.TemplateWarnings, "模板规则已跳过") {
		t.Fatalf("TemplateWarnings = %v，期望含「模板规则已跳过」", p2.TemplateWarnings)
	}
}

// containsWarning 判断告警里是否有包含子串的一条。
func containsWarning(warns []string, sub string) bool {
	for _, w := range warns {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
