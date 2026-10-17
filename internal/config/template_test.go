package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustSem(t *testing.T, body string) Semantics {
	t.Helper()
	tpl, err := LoadBody([]byte(body), "test.ini")
	if err != nil {
		t.Fatalf("LoadBody: %v", err)
	}
	sem, err := tpl.Semantics()
	if err != nil {
		t.Fatalf("Semantics: %v", err)
	}
	return sem
}

func assertParam(t *testing.T, sem Semantics, key, want string) {
	t.Helper()
	got := sem.Params[key]
	if len(got) != 1 || got[0] != want {
		t.Errorf("%s = %v，want [%q]", key, got, want)
	}
}

func TestSemantics_CommonPassthrough(t *testing.T) {
	sem := mustSem(t, `[common]
target = clash.meta
include = .*HK.*
!exclude = - 02$
type = vless,vmess
sort = name
udp = true
tls13 = 1
name = 测试配置
custom_flag = kept
!remote = 1
node_pref = v1
emoji_type = jp
node-order = 1
`)

	for _, key := range []string{"target", "include", "exclude", "type", "sort", "udp", "tls13", "name", "custom_flag"} {
		if len(sem.Params[key]) == 0 {
			t.Errorf("%s 应被透传，实际 Params = %v", key, sem.Params)
		}
	}
	assertParam(t, sem, "target", "clash.meta")
	assertParam(t, sem, "exclude", "- 02$")
	assertParam(t, sem, "type", "vless,vmess")

	// 模板控制键不能变成请求参数。
	for _, key := range []string{"remote", "node_pref", "emoji_type", "node-order"} {
		if _, ok := sem.Params[key]; ok {
			t.Errorf("%s 是模板控制键，不应出现在 Params 里", key)
		}
	}
}

// TestSemantics_CommonMergeSameKey 同一键的普通写法与 ! 写法叠加为取值列表。
func TestSemantics_CommonMergeSameKey(t *testing.T) {
	sem := mustSem(t, "[common]\ninclude = .*HK.*\n!include = .*JP.*\n")
	if got := sem.Params["include"]; len(got) != 2 || got[0] != ".*HK.*" || got[1] != ".*JP.*" {
		t.Errorf("include = %v", got)
	}
}

func TestSemantics_NodePref(t *testing.T) {
	sem := mustSem(t, `[node_pref]
vmess = [V2Ray]
vless = [VLESS]
TROJAN = [Trojan]
include = .*HK.*;.*JP.*
exclude = - 02$
sort = name
remove = badnode
`)

	prefixes := sem.NodePref.Prefixes
	if prefixes["vmess"] != "[V2Ray]" || prefixes["vless"] != "[VLESS]" {
		t.Errorf("前缀表 = %v", prefixes)
	}
	// 协议类型大小写归一。
	if prefixes["trojan"] != "[Trojan]" {
		t.Errorf("类型键应小写化: %v", prefixes)
	}
	// 控制键不能被当成协议类型。
	for _, k := range []string{"sort", "remove", "include", "exclude"} {
		if _, ok := prefixes[k]; ok {
			t.Errorf("控制键 %s 不应成为前缀项", k)
		}
	}
	if len(sem.NodePref.Include) != 2 || len(sem.NodePref.Exclude) != 1 {
		t.Errorf("include/exclude = %v / %v", sem.NodePref.Include, sem.NodePref.Exclude)
	}
}

// TestSemantics_NodePrefKeepsTrailingSpace 前缀尾部的空格是分隔符，必须原样保留。
//
// 丢掉它会把 "🚀HK-香港-01" 拼成 "🚀HK..." 的样子，是最常踩的隐形坑。
func TestSemantics_NodePrefKeepsTrailingSpace(t *testing.T) {
	sem := mustSem(t, "[node_pref]\nvless = [VLESS] \nvmess = [V2Ray]\n")
	if got := sem.NodePref.Prefixes["vless"]; got != "[VLESS] " {
		t.Errorf("vless 前缀 = %q，尾部空格应保留", got)
	}
	if got := sem.NodePref.Prefixes["vmess"]; got != "[V2Ray]" {
		t.Errorf("vmess 前缀 = %q，不该凭空补空格", got)
	}
}

// TestSemantics_CommonValueTrimmed [common] 的值是普通参数，尾部空白不保留。
func TestSemantics_CommonValueTrimmed(t *testing.T) {
	sem := mustSem(t, "[common]\nsort = name   \ninclude = ^HK-\t\n")
	if got := sem.Params["sort"]; len(got) != 1 || got[0] != "name" {
		t.Errorf("sort = %q，应去掉尾部空白", got)
	}
	if got := sem.Params["include"]; len(got) != 1 || got[0] != "^HK-" {
		t.Errorf("include = %q，应去掉尾部空白", got)
	}
}

// TestSemantics_RenameSection 段落写法是"一行一条规则"：键是正则，值是替换。
func TestSemantics_RenameSection(t *testing.T) {
	sem := mustSem(t, "[rename]\n港 = Hong Kong\n01 = 一\n")
	if got := sem.Params["rename"]; len(got) != 2 || got[0] != "港,Hong Kong" || got[1] != "01,一" {
		t.Errorf("rename = %v", got)
	}
}

// TestSemantics_RenameSectionMultiPerLine 一行带分号时按多规则列表处理。
func TestSemantics_RenameSectionMultiPerLine(t *testing.T) {
	sem := mustSem(t, "[rename]\ndefault = .*港.*,港;01,一\n")
	if got := sem.Params["rename"]; len(got) != 2 || got[0] != ".*港.*,港" || got[1] != "01,一" {
		t.Errorf("rename = %v", got)
	}
}

// TestSemantics_RenameInline 直接在 [common] 里写规则。
func TestSemantics_RenameInline(t *testing.T) {
	sem := mustSem(t, "[common]\n!rename = .*港.*,港;01,一\n")
	if got := sem.Params["rename"]; len(got) != 2 {
		t.Errorf("rename = %v", got)
	}
}

// TestSemantics_RenameCombined 同一段模板里两种写法叠加：段落规则在前，行规则在后。
func TestSemantics_RenameCombined(t *testing.T) {
	sem := mustSem(t, "[common]\n!rename = 01,一\n[rename]\n港 = Hong Kong\n")
	if got := sem.Params["rename"]; len(got) != 2 || got[0] != "港,Hong Kong" || got[1] != "01,一" {
		t.Errorf("rename = %v", got)
	}
}

// TestSemantics_EmojiSection emoji 规则与 rename 同构。
func TestSemantics_EmojiSection(t *testing.T) {
	sem := mustSem(t, "[emoji]\ndefault = ^HK,🇭🇰;^JP,🇯🇵\n")
	if got := sem.Params["emoji"]; len(got) != 2 || got[0] != "^HK,🇭🇰" || got[1] != "^JP,🇯🇵" {
		t.Errorf("emoji = %v", got)
	}

	sem = mustSem(t, "[emoji]\n^HK = 🇭🇰\n")
	if got := sem.Params["emoji"]; len(got) != 1 || got[0] != "^HK,🇭🇰" {
		t.Errorf("emoji = %v", got)
	}
}

func TestSemantics_GroupsMultiple(t *testing.T) {
	sem := mustSem(t, `[clash_proxy_group]
name = 自动选择
type = url-test
interval = 300
tolerance = 50
timeout = 5000
url = https://www.gstatic.com/generate_204
strategy = global
lazy = true
nodes = .*HK.*;DIRECT
---
name = 手动选择
type = select
proxies = 自动选择;DIRECT;REJECT
---
type = fallback
`)

	if len(sem.Groups) != 2 {
		t.Fatalf("代理组数 = %d，应为 2（第三个块没有名字应被跳过）: %+v", len(sem.Groups), sem.Groups)
	}
	auto := sem.Groups[0]
	if auto.Name != "自动选择" || auto.Type != "url-test" || auto.Strategy != "global" || !auto.Lazy {
		t.Errorf("第一个组 = %+v", auto)
	}
	if auto.Test != "https://www.gstatic.com/generate_204" {
		t.Errorf("测活地址 = %q", auto.Test)
	}
	if auto.Options.Interval != 300 || auto.Options.Tolerance != 50 || auto.Options.Timeout != 5000 {
		t.Errorf("测活参数 = %+v", auto.Options)
	}
	if len(auto.Proxies) != 2 || auto.Proxies[0] != ".*HK.*" || auto.Proxies[1] != "DIRECT" {
		t.Errorf("成员 = %v", auto.Proxies)
	}

	manual := sem.Groups[1]
	if manual.Name != "手动选择" || manual.Type != "select" {
		t.Errorf("第二个组 = %+v", manual)
	}
	if len(manual.Proxies) != 3 {
		t.Errorf("成员 = %v", manual.Proxies)
	}
}

// TestSemantics_GroupBadValues 写错的取值不应破坏解析。
func TestSemantics_GroupBadValues(t *testing.T) {
	sem := mustSem(t, `[clash_proxy_group]
name = 组
type = select
interval = abc
tolerance = -5
strategy = unknown
lazy = no
`)

	if len(sem.Groups) != 1 {
		t.Fatalf("组数 = %d", len(sem.Groups))
	}
	g := sem.Groups[0]
	if g.Options.Interval != 0 || g.Options.Tolerance != 0 || g.Strategy != "" || g.Lazy {
		t.Errorf("非法取值应被忽略: %+v", g)
	}
}

func TestSemantics_RulesetSection(t *testing.T) {
	sem := mustSem(t, `[ruleset]
MyGeoSite = 2
MyGeoIP = 1

[ruleset]
`)

	if got := sem.Params["ruleset"]; len(got) != 2 || got[0] != "MyGeoSite:2" || got[1] != "MyGeoIP:1" {
		t.Errorf("ruleset = %v", got)
	}
}

// TestSemantics_RulesParam [common] 里的规则行按分号拆开，走规则流而不是普通参数。
// 留在 Params 里的话会与规则流重复收集一遍，产物出现重复规则行。
func TestSemantics_RulesParam(t *testing.T) {
	sem := mustSem(t, "[common]\nrules = GEOIP,CN,DIRECT;MATCH,自动选择\n")
	if got := sem.RuleLines(); len(got) != 2 || got[0] != "GEOIP,CN,DIRECT" || got[1] != "MATCH,自动选择" {
		t.Errorf("规则行 = %v", got)
	}
	if _, ok := sem.Params["rules"]; ok {
		t.Errorf("规则行不应同时出现在 Params：%v", sem.Params)
	}
}

// TestSemantics_InlineGroupsAndRulesets subconverter 在 [custom] 里的内联写法
// （custom_proxy_group= / ruleset=）是公开模板的主流写法，必须真解析出来。
// 不解析的后果是"节点齐、规则空"的配置静默上线——这类模板分流完全失效。
func TestSemantics_InlineGroupsAndRulesets(t *testing.T) {
	// 反引号 DSL 写不进原样字符串（反引号本身是定界符），用普通字符串。
	body := "[custom]\n" +
		"custom_proxy_group=🚀 手动选择`select`[]♻️ 自动选择`.*\n" +
		"custom_proxy_group=🌍 自动选择`url-test`.*`http://www.gstatic.com/generate_204`300,,50\n" +
		"ruleset=🎯 全球直连,[]GEOSITE,private\n" +
		"ruleset=🎯 全球直连,[]GEOIP,cn,no-resolve\n" +
		"ruleset=🚀 手动选择,clash-domain:https://example.com/rule.mrs,28800\n" +
		"ruleset=🚀 手动选择,[]FINAL\n" +
		"enable_rule_generator=true\n"
	sem := mustSem(t, body)

	if len(sem.Groups) != 2 {
		t.Fatalf("组数 = %d，应为 2：%v", len(sem.Groups), sem.Groups)
	}
	if got := sem.Groups[0]; got.Name != "🚀 手动选择" || got.Type != "select" ||
		len(got.Proxies) != 2 || got.Proxies[0] != "♻️ 自动选择" || got.Proxies[1] != ".*" {
		t.Errorf("组 0 = %+v", got)
	}
	g1 := sem.Groups[1]
	if g1.Name != "🌍 自动选择" || g1.Type != "url-test" || g1.Proxies[0] != ".*" ||
		g1.Test != "http://www.gstatic.com/generate_204" ||
		g1.Options.Interval != 300 || g1.Options.Tolerance != 50 {
		t.Errorf("组 1 = %+v", g1)
	}

	// 规则流要保住声明顺序：FINAL 落在最后一项，前面的规则才不会被 MATCH 挡住。
	if len(sem.RuleItems) != 4 {
		t.Fatalf("规则流 = %d 项：%+v", len(sem.RuleItems), sem.RuleItems)
	}
	want := []struct {
		line   string
		remote bool
	}{
		{"GEOSITE,private,🎯 全球直连", false},
		{"GEOIP,cn,🎯 全球直连,no-resolve", false},
		{"", true},
		{"MATCH,🚀 手动选择", false},
	}
	for i, w := range want {
		it := sem.RuleItems[i]
		if it.Line != w.line {
			t.Errorf("第 %d 项 Line = %q，应为 %q", i, it.Line, w.line)
		}
		if (it.Remote >= 0) != w.remote {
			t.Errorf("第 %d 项的远程引用与预期不符: %+v", i, it)
		}
	}
	if got := sem.RemoteRulesets; len(got) != 1 ||
		got[0].Group != "🚀 手动选择" || got[0].URL != "https://example.com/rule.mrs" ||
		got[0].Kind != "domain" || got[0].Interval != 28800 {
		t.Errorf("远程规则集 = %+v", got)
	}

	// 内联写法已经被解析，不该再提示"未实现"；enable_rule_generator 是普通开关。
	if len(sem.Warnings) != 0 {
		t.Errorf("Warnings = %v，内联写法已支持，不应告警", sem.Warnings)
	}
	// 内联 ruleset= 不能被当成 getruleset 用的普通参数。
	if _, ok := sem.Params["ruleset"]; ok {
		t.Errorf("Params 里不应出现内联 ruleset：%v", sem.Params)
	}
}

// TestSemantics_InlineSyntaxWarnings_NoFalsePositives [ruleset] 段落里的 ruleset 键名是
// 正常用法（那里是段落名而不是键名），其他常见 [custom] 键也不该误报。
// 特别注意 custom_rules：名字像内联规则声明，实际是"是否启用自定义规则"的布尔开关，
// 公开模板里天天在用，误报一条就等于告警失去价值。
func TestSemantics_InlineSyntaxWarnings_NoFalsePositives(t *testing.T) {
	sem := mustSem(t, `[custom]
enable_rule_generator=true
overwrite_original_rules=true
custom_rules=false
udp=true

[ruleset]
MyGeoSite = 2
`)
	if len(sem.Warnings) != 0 {
		t.Errorf("Warnings = %v，常规模板不应产生告警", sem.Warnings)
	}
	if got := sem.Params["ruleset"]; len(got) != 1 || got[0] != "MyGeoSite:2" {
		t.Errorf("ruleset = %v", got)
	}
}

// TestSemantics_UnknownCustomInstruction_DuplicateKey 同一段落里同一个未实现的自定义指令
// 写多次只告警一次——模板里几十条同类指令不该刷屏成几十条提示。
func TestSemantics_UnknownCustomInstruction_DuplicateKey(t *testing.T) {
	sem := mustSem(t, `[custom]
custom_rewrite=.*foo,.*bar
custom_rewrite=.*baz,.*qux
custom_rewrite=.*zzz,.*yyy
`)
	if len(sem.Warnings) != 1 {
		t.Fatalf("Warnings = %v，应为 1 条", sem.Warnings)
	}
	if !strings.Contains(sem.Warnings[0], "custom_rewrite") {
		t.Errorf("告警未指出键名: %s", sem.Warnings[0])
	}
}

// TestSemantics_UnknownCustomInstruction 遇到没实现的 custom_ 指令仍要提示：
// 已知写法静默解析，未知写法必须说出来，否则用户不知道模板的哪部分没生效。
func TestSemantics_UnknownCustomInstruction(t *testing.T) {
	sem := mustSem(t, "[custom]\ncustom_dns=1.1.1.1\n")
	if len(sem.Warnings) != 1 {
		t.Fatalf("Warnings = %v，应为 1 条", sem.Warnings)
	}
	if !strings.Contains(sem.Warnings[0], "custom_dns") ||
		!strings.Contains(sem.Warnings[0], "[custom]") {
		t.Errorf("告警内容不对: %s", sem.Warnings[0])
	}
}

// --- 覆盖链：Merge ---

func TestMerge_LocalThenRemote(t *testing.T) {
	local, err := LoadBody([]byte(`[common]
include = .*HK.*
sort = name
target = clash.meta

[clash_proxy_group]
name = 本地组
type = select
proxies = .*HK.*
`), "local.ini")
	if err != nil {
		t.Fatal(err)
	}
	remote, err := LoadBody([]byte(`[common]
include = .*JP.*
target = clash.meta

[clash_proxy_group]
name = 远程组
type = url-test
proxies = .*JP.*
`), "remote.ini")
	if err != nil {
		t.Fatal(err)
	}

	merged, err := Merge(local, remote)
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Origin(); got != "local.ini + remote.ini" {
		t.Errorf("来源 = %q", got)
	}

	sem, err := merged.Semantics()
	if err != nil {
		t.Fatal(err)
	}
	// 普通键：后者胜。
	assertParam(t, sem, "include", ".*JP.*")
	assertParam(t, sem, "sort", "name")
	// 代理组：按块追加，两个都保留。
	if len(sem.Groups) != 2 || sem.Groups[0].Name != "本地组" || sem.Groups[1].Name != "远程组" {
		t.Errorf("代理组 = %+v", sem.Groups)
	}
}

// TestMerge_DoesNotMutateInputs Merge 不能改动静默的输入模板。
func TestMerge_DoesNotMutateInputs(t *testing.T) {
	a, err := LoadBody([]byte("[common]\nk = a\n"), "a.ini")
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadBody([]byte("[common]\nk = b\n"), "b.ini")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Merge(a, b); err != nil {
		t.Fatal(err)
	}
	sem, err := a.Semantics()
	if err != nil {
		t.Fatal(err)
	}
	if got := sem.Params["k"]; len(got) != 1 || got[0] != "a" {
		t.Errorf("输入模板被改了: %v", got)
	}
}

// --- 加载入口 ---

func TestLoadSource_LocalPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tpl.ini")
	if err := os.WriteFile(path, []byte("[common]\ntarget = surge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tpl, err := LoadSource(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	sem, _ := tpl.Semantics()
	assertParam(t, sem, "target", "surge")
}

func TestLoadSource_Remote(t *testing.T) {
	loader := func(ctx context.Context, url string) ([]byte, error) {
		if url != "https://cfg.example.com/tpl.ini" {
			t.Errorf("意外的 URL: %q", url)
		}
		return []byte("[common]\ntarget = v2ray\n"), nil
	}
	tpl, err := LoadSource(context.Background(), "https://cfg.example.com/tpl.ini", loader)
	if err != nil {
		t.Fatal(err)
	}
	sem, _ := tpl.Semantics()
	assertParam(t, sem, "target", "v2ray")
}

// TestHTTPLoader 远程配置走普通 HTTP GET：不能用订阅 Fetcher，否则 INI 会被判成无法识别。
func TestHTTPLoader(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[common]\ntarget = v2ray\n"))
	}))
	t.Cleanup(srv.Close)

	loader := HTTPLoader(srv.Client(), map[string]string{"Authorization": "Bearer tk"})
	tpl, err := LoadSource(context.Background(), srv.URL+"/tpl.ini", loader)
	if err != nil {
		t.Fatalf("HTTPLoader: %v", err)
	}
	sem, _ := tpl.Semantics()
	assertParam(t, sem, "target", "v2ray")
	if authHeader != "Bearer tk" {
		t.Errorf("Authorization = %q，附加请求头应随请求发出", authHeader)
	}

	// 非 2xx 要报错，不能拿错误页当配置去解析。
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("no"))
	}))
	t.Cleanup(bad.Close)
	if _, err := LoadSource(context.Background(), bad.URL, HTTPLoader(bad.Client(), nil)); err == nil {
		t.Error("HTTP 403 应报错")
	}
}

func TestLoadSource_Errors(t *testing.T) {
	ctx := context.Background()
	if _, err := LoadSource(ctx, "", nil); err == nil {
		t.Error("空引用应报错")
	}
	if _, err := LoadSource(ctx, "https://x/tpl.ini", nil); err == nil {
		t.Error("远程配置但没给加载器应报错")
	}
	if _, err := LoadSource(ctx, "没有这样的文件.ini", nil); err == nil {
		t.Error("本地文件不存在应报错")
	}
	if _, err := LoadSource(ctx, "没有这样的文件.ini", func(context.Context, string) ([]byte, error) {
		return nil, nil
	}); err == nil {
		t.Error("不应把 loader 用于本地路径")
	}
}

// TestTemplate_NilSafe 零值与 nil 模板不崩。
func TestTemplate_NilSafe(t *testing.T) {
	var nilTpl *Template
	if got := nilTpl.Origin(); got != "" {
		t.Errorf("nil Origin = %q", got)
	}
	sem, err := nilTpl.Semantics()
	if err != nil {
		t.Errorf("nil Semantics 报错: %v", err)
	}
	if len(sem.Params) != 0 || len(sem.Groups) != 0 || !sem.NodePref.Empty() {
		t.Errorf("nil Semantics 应为空: %+v", sem)
	}

	merged, err := Merge(nil)
	if err != nil || merged != nil {
		t.Errorf("Merge(nil) = %+v, %v", merged, err)
	}
}

// TestSemantics_InlineGroupHTTPMember 成员正则以 "http" 开头时不能被误判成测活地址。
//
// 语法是 <组名>`<类型>`<成员>…[`<测活地址>`<参数>]，测活地址只可能是绝对 URL。
// 早期实现用宽松前缀匹配（strings.HasPrefix(seg, "http")），`http.*` 这种成员正则会
// 被当成测活地址，它以及它之后的 []DIRECT 整段被吞掉，组直接变零成员。
func TestSemantics_InlineGroupHTTPMember(t *testing.T) {
	sem := mustSem(t, "[custom]\n"+
		"custom_proxy_group=Test`select`http.*`[]DIRECT\n"+
		"custom_proxy_group=Auto`url-test`.*`https://www.gstatic.com/generate_204`300\n")
	if len(sem.Groups) != 2 {
		t.Fatalf("组数 = %d，want 2: %+v", len(sem.Groups), sem.Groups)
	}
	got := sem.Groups[0]
	if len(got.Proxies) != 2 || got.Proxies[0] != "http.*" || got.Proxies[1] != "DIRECT" {
		t.Errorf("成员 = %v，want [http.* DIRECT]（http.* 是成员正则，不是测活地址）", got.Proxies)
	}
	if got.Test != "" {
		t.Errorf("select 组不应有测活地址，得到 %q", got.Test)
	}
	// 真正的测活地址仍要被识别出来。
	auto := sem.Groups[1]
	if auto.Test != "https://www.gstatic.com/generate_204" {
		t.Errorf("测活地址 = %q", auto.Test)
	}
	if len(auto.Proxies) != 1 || auto.Proxies[0] != ".*" {
		t.Errorf("成员 = %v，want [.*]", auto.Proxies)
	}
	if auto.Options.Interval != 300 {
		t.Errorf("测活间隔 = %d，want 300", auto.Options.Interval)
	}
}

// TestIsAbsoluteURL 只认 http:// 与 https://。
func TestIsAbsoluteURL(t *testing.T) {
	cases := map[string]bool{
		"https://a/b": true,
		"http://a":    true,
		"HTTP://A":    true,
		" http://a ":  true,
		"http.*":      false,
		"http":        false,
		"https":       false,
		".*":          false,
		"":            false,
	}
	for in, want := range cases {
		if got := isAbsoluteURL(in); got != want {
			t.Errorf("isAbsoluteURL(%q) = %v，want %v", in, got, want)
		}
	}
}
