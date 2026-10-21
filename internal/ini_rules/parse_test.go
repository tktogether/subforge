package ini_rules

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"os"

	"github.com/tktogether/subforge/internal/fetch"
)

// --- Parse 测试 ---

func TestParse_Custom_RulesetRefs(t *testing.T) {
	data := []byte(`[custom]
ruleset=苹果服务,https://example.com/apple.list
ruleset=Google,https://example.com/google.list
custom_proxy_group=` + "`🍎 苹果服务` select  不搞你说啥`" + `
`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Form != FormCustom {
		t.Fatalf("Form = %v, want FormCustom", doc.Form)
	}
	if len(doc.RulesetRefs) != 2 {
		t.Fatalf("refs = %d, want 2", len(doc.RulesetRefs))
	}
	if doc.RulesetRefs[0].Group != "苹果服务" || doc.RulesetRefs[0].URL != "https://example.com/apple.list" {
		t.Fatalf("refs[0] = %+v", doc.RulesetRefs[0])
	}
	if doc.RulesetRefs[1].Group != "Google" || doc.RulesetRefs[1].URL != "https://example.com/google.list" {
		t.Fatalf("refs[1] = %+v", doc.RulesetRefs[1])
	}
}

func TestParse_Custom_IgnoresCustomProxyGroup(t *testing.T) {
	data := []byte(`[custom]
ruleset=A,https://a.list
custom_proxy_group=` + "`🚀 节点组` select  🚀香港01,🚀香港02,DIRECT`" + `
ruleset=B,https://b.list
`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.RulesetRefs) != 2 {
		t.Fatalf("refs = %d, want 2 (custom_proxy_group 应被忽略)", len(doc.RulesetRefs))
	}
}

func TestParse_Rule_PlainRules(t *testing.T) {
	data := []byte(`[Rule]
DOMAIN-SUFFIX,google.com,PROXY
DOMAIN-SUFFIX,youtube.com,PROXY
IP-CIDR,10.0.0.0/8,DIRECT
`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Form != FormRule {
		t.Fatalf("Form = %v, want FormRule", doc.Form)
	}
	if len(doc.PlainRules) != 3 {
		t.Fatalf("PlainRules = %d, want 3", len(doc.PlainRules))
	}
	want := []string{"DOMAIN-SUFFIX,google.com,PROXY", "DOMAIN-SUFFIX,youtube.com,PROXY", "IP-CIDR,10.0.0.0/8,DIRECT"}
	for i, w := range want {
		if doc.PlainRules[i] != w {
			t.Errorf("PlainRules[%d] = %q, want %q", i, doc.PlainRules[i], w)
		}
	}
}

func TestParse_CommentsAndBlankLines(t *testing.T) {
	data := []byte(`# 顶部注释
; 分号注释行

[custom]
# 段内注释
ruleset=A,https://a.list

ruleset=B,https://b.list
`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.RulesetRefs) != 2 {
		t.Fatalf("refs = %d, want 2", len(doc.RulesetRefs))
	}
}

func TestParse_RulesetMissingURL_Error(t *testing.T) {
	data := []byte(`[custom]
ruleset=A,
`)
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !strings.Contains(err.Error(), "缺 URL") {
		t.Fatalf("error should mention 缺 URL, got: %v", err)
	}
}

func TestParse_RulesetMissingURL_LineNo(t *testing.T) {
	data := []byte(`[custom]
# 注释行占位
ruleset=B,
`)
	_, err := Parse(data)
	if err == nil {
		t.Fatal("expected error")
	}
	// ruleset=B, 在第 3 行
	if !strings.Contains(err.Error(), "3") {
		t.Fatalf("error should mention line 3, got: %v", err)
	}
}

func TestParse_NoCustomNoRule_FormUnknown(t *testing.T) {
	data := []byte(`[common]
mode=global
[Group]
select=🚀 香港
`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse should not error for non-ini source: %v", err)
	}
	if doc.Form != FormUnknown {
		t.Fatalf("Form = %v, want FormUnknown", doc.Form)
	}
	if len(doc.RulesetRefs) != 0 || len(doc.PlainRules) != 0 {
		t.Fatal("FormUnknown should have empty refs and rules")
	}
}

func TestParse_RulesetNoComma_OnlyURL(t *testing.T) {
	data := []byte(`[custom]
ruleset=https://example.com/x.list
`)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.RulesetRefs) != 1 {
		t.Fatalf("refs = %d, want 1", len(doc.RulesetRefs))
	}
	if doc.RulesetRefs[0].Group != "" || doc.RulesetRefs[0].URL != "https://example.com/x.list" {
		t.Fatalf("refs[0] = %+v", doc.RulesetRefs[0])
	}
}

func TestParse_RealWorldTemplateRefCount_OK(t *testing.T) {
	// 上游真实模板的 ruleset= 条数会随分组增删漂移：Aethersailor/Custom_OpenClash_Rules
	// 的 Custom_Clash_Full.ini 在 2026-10-21 曾达到 71 条，旧上限 64 直接让
	// 「更新规则源」失败。这里钉住「正常模板必须能过」，并用 maxRefs 本身验证边界。
	const realWorldRefs = 71
	var sb strings.Builder
	sb.WriteString("[custom]\n")
	for i := 0; i < realWorldRefs; i++ {
		sb.WriteString("ruleset=G" + strconv.Itoa(i) + ",https://example.com/" + strconv.Itoa(i) + ".list\n")
	}
	doc, err := Parse([]byte(sb.String()))
	if err != nil {
		t.Fatalf("Parse(%d refs) should succeed: %v", realWorldRefs, err)
	}
	if got := len(doc.RulesetRefs); got != realWorldRefs {
		t.Fatalf("refs = %d, want %d", got, realWorldRefs)
	}

	var atCap strings.Builder
	atCap.WriteString("[custom]\n")
	for i := 0; i < maxRefs; i++ {
		atCap.WriteString("ruleset=G,https://example.com/x.list\n")
	}
	if _, err := Parse([]byte(atCap.String())); err != nil {
		t.Fatalf("Parse(%d refs, at cap) should succeed: %v", maxRefs, err)
	}
}

func TestParse_TooManyRefs_Error(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[custom]\n")
	for i := 0; i < maxRefs+1; i++ {
		sb.WriteString("ruleset=G,https://x.list\n")
	}
	_, err := Parse([]byte(sb.String()))
	if err == nil {
		t.Fatal("expected error for too many refs")
	}
}

// --- IsINI 嗅探 ---

func TestIsINI_Custom(t *testing.T) {
	if !IsINI([]byte("[custom]\nruleset=A,https://a.list\n")) {
		t.Fatal("should detect ini")
	}
}

func TestIsINI_RulesetTopLevel(t *testing.T) {
	if !IsINI([]byte("ruleset=A,https://a.list\n")) {
		t.Fatal("should detect ini")
	}
}

func TestIsINI_RuleSection(t *testing.T) {
	if !IsINI([]byte("[Rule]\nDOMAIN,google.com,PROXY\n")) {
		t.Fatal("should detect ini")
	}
}

func TestIsINI_NotINI(t *testing.T) {
	if IsINI([]byte("rules:\n  - DOMAIN,google.com,PROXY\n")) {
		t.Fatal("should not detect ini")
	}
}

func TestIsINI_CommentSkipped(t *testing.T) {
	if !IsINI([]byte("# comment\n\n[custom]\nruleset=A,https://a.list\n")) {
		t.Fatal("should skip comments and detect ini")
	}
}

// --- Expand 测试 ---

func TestExpand_RuleForm_Dedup(t *testing.T) {
	doc := Doc{Form: FormRule, PlainRules: []string{"A", "B", "A", "C", "B"}}
	got, err := Expand(context.Background(), doc, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{"A", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpand_UnknownForm_ReturnsNil(t *testing.T) {
	doc := Doc{Form: FormUnknown}
	got, err := Expand(context.Background(), doc, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestExpand_Custom_MergeDedup(t *testing.T) {
	// 两个 httptest server，list 含重复规则
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DOMAIN-SUFFIX,a.com,PROXY\nDOMAIN-SUFFIX,b.com,PROXY\n"))
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DOMAIN-SUFFIX,b.com,PROXY\nDOMAIN-SUFFIX,c.com,PROXY\n"))
	}))
	defer srv2.Close()

	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "g1", URL: srv1.URL},
		{Group: "g2", URL: srv2.URL},
	}}
	fetcher := fetch.New(fetch.WithConfig(fetch.DefaultConfig()))
	got, err := Expand(context.Background(), doc, fetcher)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// 合并顺序：g1 先（a,b），g2 后（b 落另一个组所以不是重复行, c）→ a,b,b,c。
	// 去重按整行：同一条域名落到不同组是两条合法规则，不能合并。
	// 出站名必须是引用声明的组（g1/g2），而不是清单原文里的 PROXY：
	// 规则源展开后是最终产物，清单自带的出站名会把规则指向一个并不存在的策略组。
	want := []string{"DOMAIN-SUFFIX,a.com,g1", "DOMAIN-SUFFIX,b.com,g1", "DOMAIN-SUFFIX,b.com,g2", "DOMAIN-SUFFIX,c.com,g2"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExpand_Custom_AllFail(t *testing.T) {
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{{Group: "g1", URL: "http://127.0.0.1:1/nope"}}}
	fetcher := fetch.New(fetch.WithConfig(fetch.DefaultConfig()))
	_, err := Expand(context.Background(), doc, fetcher)
	if err == nil {
		t.Fatal("expected error for all-fail refs")
	}
}

func TestExpand_Custom_PartialFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DOMAIN-SUFFIX,a.com,PROXY\n"))
	}))
	defer srv.Close()
	doc := Doc{Form: FormCustom, RulesetRefs: []RemoteRuleset{
		{Group: "good", URL: srv.URL},
		{Group: "bad", URL: "http://127.0.0.1:1/nope"},
	}}
	fetcher := fetch.New(fetch.WithConfig(fetch.DefaultConfig()))
	got, err := Expand(context.Background(), doc, fetcher)
	if err != nil {
		t.Fatalf("partial fail should not error: %v", err)
	}
	if len(got) != 1 || got[0] != "DOMAIN-SUFFIX,a.com,good" {
		t.Fatalf("got %v", got)
	}
}

// --- FormatRulesText / FormatRulesYAML ---

func TestFormatRulesText(t *testing.T) {
	got := string(FormatRulesText([]string{"A", "B", "C"}))
	want := "A\nB\nC\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatRulesYAML(t *testing.T) {
	got := string(FormatRulesYAML([]string{"DOMAIN-SUFFIX,google.com,PROXY", "IP-CIDR,10.0.0.0/8,DIRECT"}))
	want := "rules:\n  - \"DOMAIN-SUFFIX,google.com,PROXY\"\n  - \"IP-CIDR,10.0.0.0/8,DIRECT\"\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatRulesYAML_Quoting(t *testing.T) {
	// 含冒号的规则应加引号
	got := string(FormatRulesYAML([]string{"DOMAIN-KEYWORD,google:"}))
	if !strings.Contains(got, "\"DOMAIN-KEYWORD,google:\"") {
		t.Fatalf("got %q, want quoted", got)
	}
}

// --- 验证 fetch.Fetcher 实现 RuleFetcher 接口 ---

func TestNewRuleFetcher_Compile(t *testing.T) {
	f := fetch.New(fetch.WithConfig(fetch.DefaultConfig()))
	_ = NewRuleFetcher(f) // 编译期验证接口实现
}

// --- 使用 sample 文件 ---

func TestParse_SampleFile(t *testing.T) {
	data, err := os.ReadFile("testdata/acl4ssr_sample.ini")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Form != FormCustom {
		t.Fatalf("Form = %v, want FormCustom", doc.Form)
	}
	if len(doc.RulesetRefs) != 2 {
		t.Fatalf("refs = %d, want 2", len(doc.RulesetRefs))
	}
}

// --- 验证容错：; 在值中间不是注释 ---

func TestParse_SemicolonInValue_NotComment(t *testing.T) {
	// [Rule] 一行规则里若含分号，不应被截断
	data := []byte("[Rule]\nDOMAIN-KEYWORD,foo;bar,PROXY\n")
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.PlainRules) != 1 {
		t.Fatalf("PlainRules = %d, want 1", len(doc.PlainRules))
	}
	if doc.PlainRules[0] != "DOMAIN-KEYWORD,foo;bar,PROXY" {
		t.Fatalf("PlainRules[0] = %q", doc.PlainRules[0])
	}
}

// --- 验证 URL 含 query 也能采集 ---

func TestParse_URLWithQuery(t *testing.T) {
	data := []byte("[custom]\nruleset=A,https://example.com/x.list?v=1&t=2\n")
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.RulesetRefs) != 1 {
		t.Fatalf("refs = %d, want 1", len(doc.RulesetRefs))
	}
	if doc.RulesetRefs[0].URL != "https://example.com/x.list?v=1&t=2" {
		t.Fatalf("URL = %q", doc.RulesetRefs[0].URL)
	}
}

// --- 占位，确保 import errors 不报未使用 ---

var _ = errors.Is
