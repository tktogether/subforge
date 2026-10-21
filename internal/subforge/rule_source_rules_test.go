package subforge

// internal/subforge/rulesources.go 的 RuleSourceRules 补测：
// 契约要求「永不返回 error，失败一律降级 warning」，所以每个失败分支都要有断言。
// warning 文案逐字比对（前端直接展示给用户，改文案必须同步改测试）。
//
// 规则源产物格式：manifest output.format 决定文件扩展名
// （clash-meta/json/sing-box → yaml、mrs → mrs、text/txt/plain → txt），
// 版本文件名形如 <source>-YYYYMMDD-HHMMSS.<ext>。

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeMultiSourceManifest 写一个含多个 source 的最小 manifest.yaml（全部同一格式与输出目录）。
func writeMultiSourceManifest(t *testing.T, m *Manager, outDir, format string, names ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("sources:\n")
	for _, n := range names {
		b.WriteString("  - name: " + n + "\n    url: https://example.test/" + n + "\n")
	}
	b.WriteString("output:\n  format: " + format + "\n  path: " + filepath.ToSlash(outDir) + "\n")
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := m.SetRuleManifestPath(context.Background(), path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
}

// seedProduct 在输出目录写一个指定名的版本产物文件。
func seedProduct(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("写产物 %s: %v", name, err)
	}
	return full
}

func equalStrings(t *testing.T, got, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %#v，期望 %#v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s = %#v，期望 %#v", label, got, want)
		}
	}
}

// TestRuleSourceRulesYAML 覆盖 yaml 产物：当前版本指针生效、引号被 yaml 解掉、
// 空行跳过、FINAL/MATCH 兜底被丢弃并计入 warning。
func TestRuleSourceRulesYAML(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "out")
	writeMultiSourceManifest(t, m, outDir, "clash-meta", "a")

	oldName := "a-20240101-000000.yaml"
	newName := "a-20240102-000000.yaml"
	seedProduct(t, outDir, oldName, "rules:\n"+
		"  - \"DOMAIN-SUFFIX,old.example,DIRECT\"\n"+
		"  - \"MATCH,🎯 兜底\"\n"+
		"  - DOMAIN,keep.example,PROXY\n")
	seedProduct(t, outDir, newName, "rules:\n  - \"DOMAIN-SUFFIX,new.example,DIRECT\"\n")
	// 指针指向旧版：必须读旧版产物，而不是「最新」版本。
	writeCurrentVersions(t, outDir, map[string]string{"a": oldName})

	rules, warnings := m.RuleSourceRules(ctx, []string{"a"})
	equalStrings(t, rules, []string{"DOMAIN-SUFFIX,old.example,DIRECT", "DOMAIN,keep.example,PROXY"}, "rules")
	equalStrings(t, warnings, []string{"规则源 a 含 1 条 FINAL/MATCH 兜底规则，已忽略"}, "warnings")

	// 指针缺失/损坏时回退最新版本：新版本的规则进订阅。
	writeCurrentVersions(t, outDir, map[string]string{"a": "a-20990101-000000.yaml"})
	rules, warnings = m.RuleSourceRules(ctx, []string{"a"})
	equalStrings(t, rules, []string{"DOMAIN-SUFFIX,new.example,DIRECT"}, "rules(回退最新)")
	equalStrings(t, warnings, nil, "warnings(回退最新)")
}

// TestRuleSourceRulesText 覆盖 text 产物：按行切、TrimSpace、空行与 # 注释跳过。
func TestRuleSourceRulesText(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "out")
	writeMultiSourceManifest(t, m, outDir, "text", "a")

	seedProduct(t, outDir, "a-20240101-000000.txt",
		"# 注释行\n\nDOMAIN-SUFFIX,example.com,DIRECT\nFINAL,DIRECT\nMATCH,PROXY\n  DOMAIN,trim.example,PROXY  \n")

	rules, warnings := m.RuleSourceRules(ctx, []string{"a"})
	equalStrings(t, rules,
		[]string{"DOMAIN-SUFFIX,example.com,DIRECT", "DOMAIN,trim.example,PROXY"}, "rules")
	equalStrings(t, warnings, []string{"规则源 a 含 2 条 FINAL/MATCH 兜底规则，已忽略"}, "warnings")
}

// TestRuleSourceRulesSkips 覆盖每个「跳过 + warning」分支（文案逐字）。
func TestRuleSourceRulesSkips(t *testing.T) {
	cases := []struct {
		name     string
		format   string
		product  string // 空 = 不写产物
		names    []string
		wantWarn []string
		wantRule []string
	}{
		{
			name:     "源不存在",
			format:   "clash-meta",
			product:  "rules:\n  - DOMAIN,a.example,DIRECT\n",
			names:    []string{"ghost"},
			wantWarn: []string{"规则源 ghost 不存在，已跳过"},
		},
		{
			name:     "源还没有产物",
			format:   "clash-meta",
			names:    []string{"a"},
			wantWarn: []string{"规则源 a 还没有产物（先在规则源页执行更新），已跳过"},
		},
		{
			name:     "产物格式不支持（mrs）",
			format:   "mrs",
			product:  "x",
			names:    []string{"a"},
			wantWarn: []string{"规则源 a 的产物格式 .mrs 不支持内联到订阅，已跳过（请把规则源输出格式设为 yaml 或 text）"},
		},
		{
			name:     "yaml 解析失败",
			format:   "clash-meta",
			product:  "rules:\n  - [unclosed\n",
			names:    []string{"a"},
			wantWarn: []string{"规则源 a 的产物里没有 rules 列表，已跳过"},
		},
		{
			name:     "yaml 没有 rules 列表",
			format:   "clash-meta",
			product:  "proxies:\n  - name: p\n",
			names:    []string{"a"},
			wantWarn: []string{"规则源 a 的产物里没有 rules 列表，已跳过"},
		},
		{
			name:     "yaml 的 rules 是空列表",
			format:   "clash-meta",
			product:  "rules: []\n",
			names:    []string{"a"},
			wantWarn: []string{"规则源 a 的产物里没有 rules 列表，已跳过"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			outDir := filepath.Join(t.TempDir(), "out")
			writeMultiSourceManifest(t, m, outDir, tc.format, "a")
			if tc.product != "" {
				seedProduct(t, outDir, "a-20240101-000000."+extForRuleSourceFormat(tc.format), tc.product)
			}
			rules, warnings := m.RuleSourceRules(context.Background(), tc.names)
			equalStrings(t, warnings, tc.wantWarn, "warnings")
			if len(rules) != len(tc.wantRule) {
				t.Fatalf("rules = %#v，期望 %#v", rules, tc.wantRule)
			}
		})
	}
}

// TestRuleSourceRulesDedupeAndOrder 覆盖去重保序、空白丢弃，以及跨源拼接顺序。
func TestRuleSourceRulesDedupeAndOrder(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "out")
	writeMultiSourceManifest(t, m, outDir, "clash-meta", "a", "b")
	seedProduct(t, outDir, "a-20240101-000000.yaml",
		"rules:\n  - \"DOMAIN-SUFFIX,a1.example,DIRECT\"\n  - \"DOMAIN,a2.example,PROXY\"\n")
	seedProduct(t, outDir, "b-20240101-000000.yaml",
		"rules:\n  - \"DOMAIN-SUFFIX,b1.example,DIRECT\"\n")

	rules, warnings := m.RuleSourceRules(ctx, []string{"b", " a ", "b", ""})
	equalStrings(t, rules, []string{
		"DOMAIN-SUFFIX,b1.example,DIRECT",
		"DOMAIN-SUFFIX,a1.example,DIRECT",
		"DOMAIN,a2.example,PROXY",
	}, "rules")
	equalStrings(t, warnings, nil, "warnings")

	// 全空输入：直接返回 (nil, nil)，不读清单。
	rules, warnings = m.RuleSourceRules(ctx, nil)
	if rules != nil || warnings != nil {
		t.Fatalf("空输入应返回 (nil, nil)，实际 (%#v, %#v)", rules, warnings)
	}
	rules, warnings = m.RuleSourceRules(ctx, []string{"", "   "})
	if rules != nil || warnings != nil {
		t.Fatalf("全空白输入应返回 (nil, nil)，实际 (%#v, %#v)", rules, warnings)
	}
}

// TestRuleSourceRulesManifestLoadFailure 覆盖「清单读取失败」降级：
// 返回单条 warning（含错误原文），rules 为空，且不 panic。
func TestRuleSourceRulesManifestLoadFailure(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(path, []byte("sources: [\n  - name: a\n"), 0o644); err != nil {
		t.Fatalf("写坏 manifest: %v", err)
	}
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}

	rules, warnings := m.RuleSourceRules(ctx, []string{"a"})
	if len(rules) != 0 {
		t.Fatalf("清单读取失败时 rules 应为空，实际 %#v", rules)
	}
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "读取规则源清单失败：") {
		t.Fatalf("warnings = %#v，期望单条「读取规则源清单失败：…」", warnings)
	}
}

// TestRuleSourceRulesProductReadFailure 覆盖「产物文件读不出来」降级。
// 用一个指向不存在目标的符号链接占位：它在目录列举里存在（所以成为「当前版本」），
// 但 os.ReadFile 会失败。平台不允许创建符号链接时该子用例 skip（不伪造通过）。
func TestRuleSourceRulesProductReadFailure(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "out")
	writeMultiSourceManifest(t, m, outDir, "clash-meta", "a")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	broken := filepath.Join(outDir, "a-20240101-000000.yaml")
	if err := os.Symlink(filepath.Join(outDir, "no-such-target"), broken); err != nil {
		t.Skipf("当前平台不允许创建符号链接，跳过读失败分支：%v", err)
	}
	writeCurrentVersions(t, outDir, map[string]string{"a": "a-20240101-000000.yaml"})

	rules, warnings := m.RuleSourceRules(ctx, []string{"a"})
	if len(rules) != 0 {
		t.Fatalf("读失败时 rules 应为空，实际 %#v", rules)
	}
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "读取规则源 a 的产物失败：") ||
		!strings.HasSuffix(warnings[0], "，已跳过") {
		t.Fatalf("warnings = %#v，期望「读取规则源 a 的产物失败：…，已跳过」", warnings)
	}
}

// TestManagerSetProfileRuleSources 覆盖 Manager 包装层：与 store 契约一致，
// 空切片 = 未绑定，不存在的方案返回 sql.ErrNoRows（原样透出，不翻译）。
func TestManagerSetProfileRuleSources(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	p, err := m.AddProfile(ctx, "规则源方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.RuleSources != nil {
		t.Fatalf("新方案 RuleSources 应为 nil，实际 %v", got.RuleSources)
	}

	if err := m.SetProfileRuleSources(ctx, p.ID, []string{"Custom_Clash", "Custom_SingBox"}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}
	got, err = m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile(设置后): %v", err)
	}
	if !reflect.DeepEqual(got.RuleSources, []string{"Custom_Clash", "Custom_SingBox"}) {
		t.Fatalf("RuleSources = %#v", got.RuleSources)
	}

	if err := m.SetProfileRuleSources(ctx, p.ID, nil); err != nil {
		t.Fatalf("SetProfileRuleSources(nil): %v", err)
	}
	got, err = m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile(清空后): %v", err)
	}
	if got.RuleSources != nil {
		t.Fatalf("清空后 RuleSources 应为 nil，实际 %v", got.RuleSources)
	}

	if err := m.SetProfileRuleSources(ctx, 9999, []string{"a"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProfileRuleSources(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}
