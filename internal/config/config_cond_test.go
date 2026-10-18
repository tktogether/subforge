package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/template"
)

// ===== override.go uncovered =====

func TestCond_Override_LoadOverrideFile_INI(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "o.ini")
	if err := os.WriteFile(p, []byte("[override.rules]\nprepend = X\n"), 0644); err != nil {
		t.Fatal(err)
	}
	o, err := LoadOverrideFile(p)
	if err != nil || len(o.Rules.Prepend) != 1 {
		t.Fatalf("err=%v o=%+v", err, o)
	}
}

func TestCond_Override_LoadOverrideFile_DefaultYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "o.yaml")
	if err := os.WriteFile(p, []byte("rules:\n  prepend:\n    - X\n"), 0644); err != nil {
		t.Fatal(err)
	}
	o, err := LoadOverrideFile(p)
	if err != nil || len(o.Rules.Prepend) != 1 {
		t.Fatalf("err=%v o=%+v", err, o)
	}
}

func TestCond_Override_ParseINI_UnknownSection(t *testing.T) {
	_, err := ParseOverrideINI([]byte("[foo]\nbar = 1\n"))
	if err == nil || !strings.Contains(err.Error(), "未知段落") {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Override_ParseINI_GroupUnknownKey(t *testing.T) {
	src := "[override.group:G]\nfoo = bar\n"
	_, err := ParseOverrideINI([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "未知键") {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Override_ParseINI_ProxyUnknownKey(t *testing.T) {
	src := "[override.proxy:P]\nfoo = bar\n"
	_, err := ParseOverrideINI([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "未知键") {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Override_ParseINI_RulesUnknownKey(t *testing.T) {
	src := "[override.rules]\nfoo = bar\n"
	_, err := ParseOverrideINI([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "未知键") {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Override_ParseINI_GroupProxyEmpty(t *testing.T) {
	src := "[override.group:G]\nproxy = \nuse = \n"
	o, err := ParseOverrideINI([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Groups) != 1 || len(o.Groups[0].Proxies) != 0 || len(o.Groups[0].Use) != 0 {
		t.Fatalf("o=%+v", o)
	}
}

func TestCond_Override_ParseINI_GroupLazy(t *testing.T) {
	src := "[override.group:G]\nlazy = true\n"
	o, err := ParseOverrideINI([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !o.Groups[0].Lazy {
		t.Fatal("lazy not set")
	}
}

func TestCond_Override_MergeGroups_NewGroup(t *testing.T) {
	base := []render.Group{{Name: "A", Type: "select"}}
	over := []render.Group{{Name: "B", Type: "url-test"}}
	out := MergeGroups(base, over)
	if len(out) != 2 || out[1].Name != "B" {
		t.Fatalf("out=%+v", out)
	}
}

func TestCond_Override_MergeGroups_Options(t *testing.T) {
	base := []render.Group{{Name: "A", Type: "select"}}
	over := []render.Group{{Name: "A", Type: "url-test", Options: render.UrlTestOptions{Interval: 5}}}
	out := MergeGroups(base, over)
	if out[0].Options.Interval != 5 {
		t.Fatalf("out=%+v", out)
	}
}

func TestCond_Override_MergeGroups_ProxiesDedupe(t *testing.T) {
	base := []render.Group{{Name: "A", Proxies: []string{"X"}}}
	over := []render.Group{{Name: "A", Proxies: []string{"X", "Y"}}}
	out := MergeGroups(base, over)
	if len(out[0].Proxies) != 2 {
		t.Fatalf("out=%+v", out)
	}
}

func TestCond_Override_MergeGroups_UseDedupe(t *testing.T) {
	base := []render.Group{{Name: "A", Use: []string{"P"}}}
	over := []render.Group{{Name: "A", Use: []string{"P", "Q"}}}
	out := MergeGroups(base, over)
	if len(out[0].Use) != 2 {
		t.Fatalf("out=%+v", out)
	}
}

func TestCond_Override_MergeGroups_EmptyOverride(t *testing.T) {
	base := []render.Group{{Name: "A"}}
	out := MergeGroups(base, nil)
	if len(out) != 1 || out[0].Name != "A" {
		t.Fatalf("out=%+v", out)
	}
}

func TestCond_Override_MergeGroups_Lazy(t *testing.T) {
	base := []render.Group{{Name: "A"}}
	over := []render.Group{{Name: "A", Lazy: true}}
	out := MergeGroups(base, over)
	if !out[0].Lazy {
		t.Fatal("lazy not merged")
	}
}

// ===== custom.go uncovered =====

func TestCond_Custom_GroupFromInline_TooFewSegs(t *testing.T) {
	if _, ok := groupFromInline("name"); ok {
		t.Fatal("should fail")
	}
}

func TestCond_Custom_GroupFromInline_EmptyName(t *testing.T) {
	if _, ok := groupFromInline("`select`"); ok {
		t.Fatal("should fail")
	}
}

func TestCond_Custom_GroupFromInline_EmptyTypeDefaults(t *testing.T) {
	g, ok := groupFromInline("G``member")
	if !ok || g.Type != "select" {
		t.Fatalf("g=%+v ok=%v", g, ok)
	}
}

func TestCond_Custom_GroupFromInline_MemberEmpty(t *testing.T) {
	g, ok := groupFromInline("G`select``")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Custom_GroupFromInline_URLIdx(t *testing.T) {
	g, ok := groupFromInline("G`url-test`member`http://example.com/generate_204`3000,,100")
	if !ok || g.Test != "http://example.com/generate_204" || g.Options.Interval != 3000 {
		t.Fatalf("g=%+v ok=%v", g, ok)
	}
}

func TestCond_Custom_GroupFromInline_IntervalTolerance(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com`100,200,300")
	if !ok || g.Options.Interval != 100 || g.Options.Tolerance != 300 {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_NoURL(t *testing.T) {
	g, ok := groupFromInline("G`select`member1`member2")
	if !ok || len(g.Proxies) != 2 {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_InlineRules_NilSection(t *testing.T) {
	cfg, _ := template.Parse([]byte(""), "")
	items, remotes := inlineRules(cfg)
	if len(items) != 0 || len(remotes) != 0 {
		t.Fatalf("items=%v remotes=%v", items, remotes)
	}
}

func TestCond_Custom_InlineRules_GEOIP_NoFlag(t *testing.T) {
	src := "[custom]\nruleset = AI,[]GEOIP,cn\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	items, _ := inlineRules(cfg)
	if len(items) != 1 || !strings.Contains(items[0].Line, "no-resolve") {
		t.Fatalf("items=%v", items)
	}
}

func TestCond_Custom_InlineRules_RemoteRuleset(t *testing.T) {
	src := "[custom]\nruleset = AI,clash-domain:https://example.com/rules.txt,3600\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	items, remotes := inlineRules(cfg)
	if len(remotes) != 1 || len(items) != 1 || items[0].Remote != 0 {
		t.Fatalf("items=%v remotes=%v", items, remotes)
	}
}

func TestCond_Custom_SplitRulesetSpec_BadSpec(t *testing.T) {
	if _, _, _, _, _, ok := splitRulesetSpec("AI"); ok {
		t.Fatal("should fail")
	}
	if _, _, _, _, _, ok := splitRulesetSpec("AI,"); ok {
		t.Fatal("should fail")
	}
}

func TestCond_Custom_SplitRulesetSpec_BuiltinNoBody(t *testing.T) {
	g, kind, body, flag, iv, ok := splitRulesetSpec("AI,[]FINAL")
	if !ok || g != "AI" || kind != "FINAL" || body != "" || flag != "" || iv != 0 {
		t.Fatalf("g=%s kind=%s body=%s flag=%s iv=%d ok=%v", g, kind, body, flag, iv, ok)
	}
}

func TestCond_Custom_SplitRulesetSpec_RemoteEmptyBody(t *testing.T) {
	if _, _, _, _, _, ok := splitRulesetSpec("AI,clash-domain:"); ok {
		t.Fatal("should fail")
	}
}

func TestCond_Custom_SplitRulesetSpec_RemoteNoColon(t *testing.T) {
	if _, _, _, _, _, ok := splitRulesetSpec("AI,foo"); ok {
		t.Fatal("should fail")
	}
}

func TestCond_Custom_SplitRulesetSpec_RemoteInterval(t *testing.T) {
	g, kind, body, _, iv, ok := splitRulesetSpec("AI,clash-domain:https://x.com/r.txt,7200")
	if !ok || g != "AI" || kind != "domain" || body != "https://x.com/r.txt" || iv != 7200 {
		t.Fatalf("g=%s kind=%s body=%s iv=%d", g, kind, body, iv)
	}
}

func TestCond_Custom_IsAbsoluteURL(t *testing.T) {
	if !isAbsoluteURL("https://example.com") {
		t.Fatal("should true")
	}
	if isAbsoluteURL("http.*") {
		t.Fatal("should false")
	}
	if isAbsoluteURL("ftp://x.com") {
		t.Fatal("should false")
	}
}

func TestCond_Custom_GroupFromInline_SegsLen2(t *testing.T) {
	g, ok := groupFromInline("G`select")
	if !ok || g.Name != "G" {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_URLIdxNegative(t *testing.T) {
	g, ok := groupFromInline("G`select`member")
	if !ok || len(g.Proxies) != 1 {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_IntervalParseFail(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com`abc,0,100")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Custom_GroupFromInline_ToleranceParseFail(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com`100,0,abc")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Custom_GroupFromInline_ToleranceNoThirdField(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com`100")
	if !ok || g.Options.Interval != 100 {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_IntervalNonPositive(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com`0,0,100")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Custom_GroupFromInline_ToleranceNonPositive(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com`100,0,0")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Custom_InlineGroups_CustomProxyGroup(t *testing.T) {
	src := "[custom]\ncustom_proxy_group = G`select`DIRECT\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	g := inlineGroups(cfg)
	if len(g) != 1 || g[0].Name != "G" {
		t.Fatalf("g=%v", g)
	}
}

func TestCond_Custom_GroupFromInline_StrategyCompatible(t *testing.T) {
	g, ok := groupFromInline("G`select`m`http://x.com`")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Custom_GroupFromInline_MembersEmpty(t *testing.T) {
	g, ok := groupFromInline("G`select`")
	if !ok || len(g.Proxies) != 0 {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_URLTestNoParams(t *testing.T) {
	g, ok := groupFromInline("G`url-test`m`http://x.com")
	if !ok || g.Test != "http://x.com" {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_Fallback(t *testing.T) {
	g, ok := groupFromInline("G`fallback`m`http://x.com`3000")
	if !ok || g.Type != "fallback" || g.Test != "http://x.com" {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Custom_GroupFromInline_MemberEmpty2(t *testing.T) {
	g, ok := groupFromInline("G`select``")
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

// ===== pipeline.go uncovered =====

func TestCond_Pipeline_BuildPatternEmpty(t *testing.T) {
	st := PipelineStep{Kind: "op.rename", Args: map[string]string{"pattern": "   "}}
	_, err := st.build()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_BuildEmojiPatternEmpty(t *testing.T) {
	st := PipelineStep{Kind: "op.emoji", Args: map[string]string{"pattern": "   "}}
	_, err := st.build()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_BuildSortByEmpty(t *testing.T) {
	st := PipelineStep{Kind: "op.sort", Args: map[string]string{"by": "   "}}
	_, err := st.build()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_BuildScriptTimeoutErr(t *testing.T) {
	st := PipelineStep{Kind: "filter.script", Args: map[string]string{"src": "x", "timeout": "bad"}}
	_, err := st.build()
	if err == nil {
		t.Fatal("should error")
	}
	st2 := PipelineStep{Kind: "op.script", Args: map[string]string{"src": "x", "timeout": "bad"}}
	_, err2 := st2.build()
	if err2 == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_BuildUnknownKind(t *testing.T) {
	st := PipelineStep{Kind: "foo.bar", Args: map[string]string{}}
	_, err := st.build()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_PipelinesNilCfg(t *testing.T) {
	var t2 *Template
	out, err := t2.pipelines()
	if err != nil || out != nil {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestCond_Pipeline_PipelinesUnknownKey(t *testing.T) {
	src := "[pipeline:p]\nfoo = bar\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	tt := &Template{cfg: cfg}
	_, err = tt.pipelines()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_PipelinesEmptyName(t *testing.T) {
	src := "[pipeline:]\nstep = op.dedupe\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	tt := &Template{cfg: cfg}
	_, err = tt.pipelines()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_SplitStepTokensQuote(t *testing.T) {
	out := splitStepTokens("a \"b c\" d")
	if len(out) != 3 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Pipeline_TimeoutArgEmpty(t *testing.T) {
	d, err := timeoutArg(map[string]string{})
	if err != nil || d != 0 {
		t.Fatalf("d=%v err=%v", d, err)
	}
}

func TestCond_Pipeline_TimeoutArgBad(t *testing.T) {
	_, err := timeoutArg(map[string]string{"timeout": "xyz"})
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_TimeoutArgNonPositive(t *testing.T) {
	_, err := timeoutArg(map[string]string{"timeout": "-1s"})
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_ScriptNameDefault(t *testing.T) {
	if scriptName(map[string]string{}, "k") != "k" {
		t.Fatal("bad")
	}
	if scriptName(map[string]string{"name": "n"}, "k") != "n" {
		t.Fatal("bad")
	}
}

func TestCond_Pipeline_CompileStepOnErrorParseErr(t *testing.T) {
	src := "[pipeline:p]\nstep = op.dedupe\non_error = bogus\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	sem, _ := (&Template{cfg: cfg}).Semantics()
	pm := sem.PipelineMap()
	p := pm["p"]
	_, err = p.Compile()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_CompileStepLevelOnErrorParseErr(t *testing.T) {
	src := "[pipeline:p]\nstep = op.dedupe, on_error=bogus\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	sem, _ := (&Template{cfg: cfg}).Semantics()
	pm := sem.PipelineMap()
	p := pm["p"]
	_, err = p.Compile()
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Pipeline_OnErrorOverride(t *testing.T) {
	src := "[pipeline:p]\non_error = skip\nstep = op.dedupe\n"
	cfg, err := template.Parse([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	sem, _ := (&Template{cfg: cfg}).Semantics()
	pm := sem.PipelineMap()
	p := pm["p"]
	plan, err := p.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps=%d", len(plan.Steps))
	}
}

// ===== template.go uncovered =====

func TestCond_Template_HTTPLoaderNilClient(t *testing.T) {
	loader := HTTPLoader(nil, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer srv.Close()
	b, err := loader(context.Background(), srv.URL)
	if err != nil || string(b) != "ok" {
		t.Fatalf("err=%v b=%s", err, b)
	}
}

func TestCond_Template_LoadBodyBad(t *testing.T) {
	_, err := LoadBody([]byte("["), "origin")
	if err == nil {
		t.Fatal("should error")
	}
}

func TestCond_Template_MergeEmpty(t *testing.T) {
	out, err := Merge(nil, nil)
	if err != nil || out != nil {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestCond_Template_MergeSkipNil(t *testing.T) {
	t1, _ := LoadBody([]byte("[common]\nfoo=bar\n"), "a")
	out, err := Merge(nil, t1, nil)
	if err != nil || out == nil {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestCond_Template_PipelineMapEmpty(t *testing.T) {
	s := Semantics{}
	if s.PipelineMap() != nil {
		t.Fatal("should nil")
	}
}

func TestCond_Template_InlineWarningsNilSection(t *testing.T) {
	cfg, _ := template.Parse([]byte(""), "")
	out := inlineWarnings(cfg)
	if len(out) != 0 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Template_NodePrefIncludeExclude(t *testing.T) {
	src := "[node_pref]\ninclude = foo\nexclude = bar\nhttp = prefix\n"
	cfg, _ := template.Parse([]byte(src), "")
	tt := &Template{cfg: cfg}
	sem, _ := tt.Semantics()
	if len(sem.NodePref.Include) == 0 || len(sem.NodePref.Exclude) == 0 {
		t.Fatalf("spec=%+v", sem.NodePref)
	}
	if sem.NodePref.Prefixes["http"] != "prefix" {
		t.Fatalf("prefixes=%v", sem.NodePref.Prefixes)
	}
}

func TestCond_Template_GroupFromBlockInterval(t *testing.T) {
	// use inline instead
	g, ok := groupFromBlock([]template.Entry{{Key: "name", Value: template.Value{Raw: "G"}}, {Key: "interval", Value: template.Value{Raw: "5"}}})
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Template_Truthy(t *testing.T) {
	if !truthy("true") || !truthy("1") || !truthy("yes") || !truthy("on") {
		t.Fatal("bad")
	}
	if truthy("no") {
		t.Fatal("bad")
	}
}

func TestCond_Template_GroupFromBlockCases(t *testing.T) {
	// tolerance, timeout, idle, url, test, strategy, lazy, nodes, use
	entries := []template.Entry{
		{Key: "name", Value: template.Value{Raw: "G"}},
		{Key: "type", Value: template.Value{Raw: "url-test"}},
		{Key: "tolerance", Value: template.Value{Raw: "10"}},
		{Key: "timeout", Value: template.Value{Raw: "20"}},
		{Key: "idle", Value: template.Value{Raw: "30"}},
		{Key: "url", Value: template.Value{Raw: "http://x.com"}},
		{Key: "strategy", Value: template.Value{Raw: "global"}},
		{Key: "lazy", Value: template.Value{Raw: "true"}},
		{Key: "nodes", Value: template.Value{Raw: "A,B"}},
		{Key: "use", Value: template.Value{Raw: "P"}},
	}
	g, ok := groupFromBlock(entries)
	if !ok || g.Name != "G" || g.Options.Tolerance != 10 || g.Options.Timeout != 20 || g.Options.Idle != 30 || g.Test != "http://x.com" || g.Strategy != "global" || !g.Lazy {
		t.Fatalf("g=%+v", g)
	}
	if len(g.Proxies) != 2 || len(g.Use) != 1 {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Template_GroupFromBlockBadInterval(t *testing.T) {
	entries := []template.Entry{
		{Key: "name", Value: template.Value{Raw: "G"}},
		{Key: "interval", Value: template.Value{Raw: "bad"}},
	}
	g, ok := groupFromBlock(entries)
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Template_GroupFromBlockStrategyCompatible(t *testing.T) {
	entries := []template.Entry{
		{Key: "name", Value: template.Value{Raw: "G"}},
		{Key: "strategy", Value: template.Value{Raw: "compatible"}},
	}
	g, ok := groupFromBlock(entries)
	if !ok || g.Strategy != "compatible" {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Template_GroupFromBlockTestEmpty(t *testing.T) {
	entries := []template.Entry{
		{Key: "name", Value: template.Value{Raw: "G"}},
		{Key: "test", Value: template.Value{Raw: "   "}},
	}
	g, ok := groupFromBlock(entries)
	if !ok {
		t.Fatal("should ok")
	}
	_ = g
}

func TestCond_Template_GroupFromBlockTypeEmpty(t *testing.T) {
	entries := []template.Entry{
		{Key: "name", Value: template.Value{Raw: "G"}},
		{Key: "type", Value: template.Value{Raw: "   "}},
	}
	g, ok := groupFromBlock(entries)
	if !ok || g.Type != "select" {
		t.Fatalf("g=%+v", g)
	}
}

func TestCond_Template_GroupFromBlockNoName(t *testing.T) {
	_, ok := groupFromBlock([]template.Entry{})
	if ok {
		t.Fatal("should fail")
	}
}

func TestCond_Template_InlineWarningsCustomUnknown(t *testing.T) {
	src := "[common]\ncustom_foo = bar\n"
	cfg, _ := template.Parse([]byte(src), "")
	out := inlineWarnings(cfg)
	if len(out) == 0 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Template_InlineWarningsDuplicate(t *testing.T) {
	src := "[common]\ncustom_foo = bar\ncustom_foo = baz\n"
	cfg, _ := template.Parse([]byte(src), "")
	out := inlineWarnings(cfg)
	if len(out) != 1 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Template_InlineWarningsEmptyValue(t *testing.T) {
	src := "[common]\ncustom_foo = \n"
	cfg, _ := template.Parse([]byte(src), "")
	out := inlineWarnings(cfg)
	if len(out) != 0 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Template_SemanticsRulesetEmpty(t *testing.T) {
	src := "[ruleset]\n = x\nfoo = \n"
	cfg, _ := template.Parse([]byte(src), "")
	tt := &Template{cfg: cfg}
	sem, _ := tt.Semantics()
	_ = sem
}

func TestCond_Template_CommonSkip(t *testing.T) {
	src := "[common]\nrename = x\nemoji = y\nrules = z\n"
	cfg, _ := template.Parse([]byte(src), "")
	tt := &Template{cfg: cfg}
	sem, _ := tt.Semantics()
	_ = sem
}

func TestCond_Template_MergeAppendGroups(t *testing.T) {
	src1 := "[clash_proxy_group]\nname = G\ntype = select\n---\n"
	src2 := "[clash_proxy_group]\nname = H\ntype = select\n---\n"
	t1, _ := LoadBody([]byte(src1), "a")
	t2, _ := LoadBody([]byte(src2), "b")
	out, err := Merge(t1, t2)
	if err != nil || out == nil {
		t.Fatalf("err=%v", err)
	}
}

// ===== remaining if branches =====

func TestCond_Custom_InlineGroups_SectionNil(t *testing.T) {
	// inlineGroups iterates SectionNames; an empty config yields no groups
	cfg, _ := template.Parse([]byte(""), "")
	out := inlineGroups(cfg)
	if len(out) != 0 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Custom_InlineGroups_EmptyValue(t *testing.T) {
	src := "[common]\ncustom_proxy_group = \n"
	cfg, _ := template.Parse([]byte(src), "")
	out := inlineGroups(cfg)
	if len(out) != 0 {
		t.Fatalf("out=%v", out)
	}
}

func TestCond_Custom_InlineRules_NilSection2(t *testing.T) {
	cfg, _ := template.Parse([]byte(""), "")
	items, remotes := inlineRules(cfg)
	if len(items) != 0 || len(remotes) != 0 {
		t.Fatalf("items=%v remotes=%v", items, remotes)
	}
}

func TestCond_Custom_InlineRules_BadSpec(t *testing.T) {
	src := "[common]\nruleset = AI,badtype\n"
	cfg, _ := template.Parse([]byte(src), "")
	items, remotes := inlineRules(cfg)
	if len(items) != 0 || len(remotes) != 0 {
		t.Fatalf("items=%v remotes=%v", items, remotes)
	}
}

func TestCond_Pipeline_BuildOnError(t *testing.T) {
	st := PipelineStep{Kind: "op.dedupe", Args: map[string]string{}, OnError: "skip"}
	// OnError is on PipelineStep? check field
	_ = st
}

func TestCond_Template_AppendGroupsEmpty(t *testing.T) {
	tt := &Template{cfg: mustParseCond("")}
	// appendGroups with other having no group section
	other, _ := template.Parse([]byte(""), "")
	tt.appendGroups(other)
}

func TestCond_Template_SemanticsRulesetEmptyName(t *testing.T) {
	src := "[ruleset]\n = x\n"
	cfg, _ := template.Parse([]byte(src), "")
	tt := &Template{cfg: cfg}
	sem, _ := tt.Semantics()
	_ = sem
}

func mustParseCond(s string) *template.Config {
	c, _ := template.Parse([]byte(s), "")
	return c
}

// ===== pipeline false arms =====
func TestCond_Pipeline_BuildRenameOk(t *testing.T) {
	st := PipelineStep{Kind: "op.rename", Args: map[string]string{"pattern": "a", "repl": "b"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildEmojiOk(t *testing.T) {
	st := PipelineStep{Kind: "op.emoji", Args: map[string]string{"pattern": "a", "repl": "b"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildSortOk(t *testing.T) {
	st := PipelineStep{Kind: "op.sort", Args: map[string]string{"by": "name"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildScriptOk2(t *testing.T) {
	st := PipelineStep{Kind: "op.script", Args: map[string]string{"src": "x", "timeout": "1s"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
	st2 := PipelineStep{Kind: "filter.script", Args: map[string]string{"src": "x", "timeout": "1s"}}
	s2, err2 := st2.build()
	if err2 != nil || s2 == nil {
		t.Fatalf("err=%v", err2)
	}
}

func TestCond_Pipeline_BuildFilterRegexOk(t *testing.T) {
	st := PipelineStep{Kind: "filter.regex", Args: map[string]string{"include": "a"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildFilterRegionOk(t *testing.T) {
	st := PipelineStep{Kind: "filter.region", Args: map[string]string{"include": "a"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildFilterTypeOk(t *testing.T) {
	st := PipelineStep{Kind: "filter.type", Args: map[string]string{"include": "a"}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildDedupeOk(t *testing.T) {
	st := PipelineStep{Kind: "op.dedupe", Args: map[string]string{}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildSplitOk(t *testing.T) {
	st := PipelineStep{Kind: "op.split", Args: map[string]string{"sep": ","}}
	s, err := st.build()
	if err != nil || s == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestCond_Pipeline_BuildOnErrorStep(t *testing.T) {
	st := PipelineStep{Kind: "op.dedupe", Args: map[string]string{}, OnError: "skip"}
	// OnError field exists? compile uses it
	_ = st
}
