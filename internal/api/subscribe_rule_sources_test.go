// 规则源产物 → 订阅：方案绑定（rule_sources）的 API 往返、归一化，
// 以及订阅组装时把产物内联进规则列表（并整层停用模板规则）。
//
// 契约同源：.tmp/ux-e2e/fix/PLAN-WAVE7.md §1.2/§1.3、TASK-W7-B.md §1-§4。
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subforge"
)

// --- 规则源夹具 ---

// ruleSourceFixture 写一份单源 manifest（yaml 输出）并把 rules.manifest_path /
// rules.output_dir 指向它，返回产物目录。
// 不写任何版本文件——「有源但没产物」本身就是一种要覆盖的状态。
func ruleSourceFixture(t *testing.T, mgr *subforge.Manager, name, format string) string {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("建产物目录: %v", err)
	}
	mf := filepath.Join(dir, "manifest.yaml")
	content := "sources:\n" +
		"  - name: " + name + "\n" +
		"    url: https://example.com/" + name + ".list\n" +
		"output:\n" +
		"  format: " + format + "\n" +
		"  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(mf, []byte(content), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := mgr.SetSettings(t.Context(), map[string]string{
		subforge.RuleManifestPathKey: mf,
		subforge.RulesOutputDirKey:   outDir,
	}); err != nil {
		t.Fatalf("写规则源配置: %v", err)
	}
	return outDir
}

// ruleSourceProduct 落一个版本产物。不写「当前版本」指针：pickCurrentVersion 会
// 回退到升序列表的最后一个（= 最新版），这正是「刚更新完」的常见状态。
func ruleSourceProduct(t *testing.T, outDir, name, content string) {
	t.Helper()
	path := filepath.Join(outDir, name+"-20240101-000000.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写产物 %s: %v", path, err)
	}
}

// ruleSourceSubServer 起方案订阅服务：fixture 机场 + INI 模板 + 一份 yaml 规则源产物。
// 返回服务器、方案（已绑定 src1）、令牌名。
func ruleSourceSubServer(t *testing.T) (*httptest.Server, string, *subforge.Manager) {
	t.Helper()
	ts, _, mgr := profileServer(t)
	if _, err := mgr.AddAirport(t.Context(), "fx", fixtureURL(t), "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	tpl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(profileTemplateINI))
	}))
	t.Cleanup(tpl.Close)

	outDir := ruleSourceFixture(t, mgr, "src1", "yaml")
	ruleSourceProduct(t, outDir, "src1",
		"rules:\n"+
			"  - \"DOMAIN-SUFFIX,a.example,DIRECT\"\n"+
			"  - \"DOMAIN-SUFFIX,b.example,REJECT\"\n"+
			"  - \"MATCH,DIRECT\"\n")
	return ts, tpl.URL + "/tpl.ini", mgr
}

// --- 方案 API ---

// seedRuleSources 在一次 manifest 里登记多个源，并为每个源落一版产物。
// 方案保存会校验“源存在且有产物”，所以归一化/上限测试也必须先把源准备好。
func seedRuleSources(t *testing.T, mgr *subforge.Manager, names []string) {
	t.Helper()
	dir := t.TempDir()
	outDir := filepath.Join(dir, "rules")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("建产物目录: %v", err)
	}
	var b strings.Builder
	b.WriteString("sources:\n")
	for _, name := range names {
		b.WriteString("  - name: " + name + "\n")
		b.WriteString("    url: https://example.com/" + name + ".list\n")
	}
	b.WriteString("output:\n  format: yaml\n  path: " + filepath.ToSlash(outDir) + "\n")
	mf := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(mf, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写 manifest: %v", err)
	}
	if err := mgr.SetSettings(t.Context(), map[string]string{
		subforge.RuleManifestPathKey: mf,
		subforge.RulesOutputDirKey:   outDir,
	}); err != nil {
		t.Fatalf("写规则源配置: %v", err)
	}
	for _, name := range names {
		ruleSourceProduct(t, outDir, name, "rules:\n  - \"MATCH,DIRECT\"\n")
	}
}

// TestProfilesRuleSourcesRoundTrip 覆盖 rule_sources 的 CRUD 语义：
// 未绑定输出 []（不是 null）、POST 归一化（trim/丢空/去重保序/上限 20）、
// PUT 只带 rule_sources 时层与命名都不动。
func TestProfilesRuleSourcesRoundTrip(t *testing.T) {
	ts, _, mgr := profileServer(t)
	allNames := []string{"a", "b", "x", "y"}
	names := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		names = append(names, fmt.Sprintf("s%02d", i))
	}
	allNames = append(allNames, names...)
	seedRuleSources(t, mgr, allNames)

	// 1) 未绑定：rule_sources 必须是空数组。
	p := mustProfile(t, ts, "方案 RS")
	if p.RuleSources == nil || len(p.RuleSources) != 0 {
		t.Fatalf("未绑定方案 rule_sources = %#v，应为空数组", p.RuleSources)
	}
	st, _, body := uiGet(t, ts, "/api/profiles")
	if st != http.StatusOK || !strings.Contains(body, "\"rule_sources\":[]") {
		t.Fatalf("列表响应缺 rule_sources:[]（状态=%d）：%s", st, body)
	}

	// 2) POST 归一化：trim、去重保序、丢空串。
	st, body = postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name": "RS 去重", "rule_sources": []string{" a ", "b", "a", ""},
	})
	if st != http.StatusCreated {
		t.Fatalf("POST 状态 = %d；body=%s", st, body)
	}
	var created struct {
		Profile profileJSON `json:"profile"`
	}
	mustDecode(t, body, &created)
	if got := strings.Join(created.Profile.RuleSources, ","); got != "a,b" {
		t.Fatalf("POST 归一化结果 = %q，期望 \"a,b\"", got)
	}

	// 3) 上限 20：第 21 个起丢弃，且保持原有顺序。
	st, body = postAPIJSON(t, ts, "/api/profiles", map[string]any{"name": "RS 上限", "rule_sources": names})
	if st != http.StatusCreated {
		t.Fatalf("POST 上限用例状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &created)
	if len(created.Profile.RuleSources) != 20 {
		t.Fatalf("绑定源数量 = %d，期望 20", len(created.Profile.RuleSources))
	}
	if created.Profile.RuleSources[0] != "s00" || created.Profile.RuleSources[19] != "s19" {
		t.Fatalf("上限截断应保序：%v", created.Profile.RuleSources)
	}

	// 4) PUT 只带 rule_sources：绑定被改写，但层与命名不动。
	p2 := mustProfilePayload(t, ts, map[string]any{
		"name": "RS 层保护", "rule_layers": []string{"custom", "airport"}, "group_layers": []string{"custom"},
	})
	st, body = doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p2.ID), map[string]any{
		"name": p2.Name, "rule_sources": []string{"x", "y"},
	})
	if st != http.StatusOK {
		t.Fatalf("PUT 状态 = %d；body=%s", st, body)
	}
	var updated struct {
		Profile profileJSON `json:"profile"`
	}
	mustDecode(t, body, &updated)
	if got := strings.Join(updated.Profile.RuleSources, ","); got != "x,y" {
		t.Fatalf("PUT 绑定 = %q，期望 \"x,y\"", got)
	}
	if got := strings.Join(updated.Profile.RuleLayers, ","); got != "custom,airport" {
		t.Fatalf("只带 rule_sources 的 PUT 改写了规则层：%q", got)
	}
	if got := strings.Join(updated.Profile.GroupLayers, ","); got != "custom" {
		t.Fatalf("只带 rule_sources 的 PUT 改写了分组层：%q", got)
	}

	// 5) 空数组 = 清空绑定；层仍然不动。
	st, body = doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/profiles/%d", p2.ID), map[string]any{
		"name": p2.Name, "rule_sources": []string{},
	})
	if st != http.StatusOK {
		t.Fatalf("清空绑定 PUT 状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &updated)
	if updated.Profile.RuleSources == nil || len(updated.Profile.RuleSources) != 0 {
		t.Fatalf("清空后 rule_sources = %#v，应为空数组", updated.Profile.RuleSources)
	}
	if got := strings.Join(updated.Profile.RuleLayers, ","); got != "custom,airport" {
		t.Fatalf("清空绑定改写了规则层：%q", got)
	}
	if got := strings.Join(updated.Profile.GroupLayers, ","); got != "custom" {
		t.Fatalf("清空绑定改写了分组层：%q", got)
	}

	// 6) 列表接口也要输出空数组，而不是 null。
	st, _, body = uiGet(t, ts, "/api/profiles")
	if st != http.StatusOK || strings.Contains(body, "\"rule_sources\":null") {
		t.Fatalf("列表响应出现 rule_sources:null（状态=%d）：%s", st, body)
	}
}

// TestProfilesRuleSourcesUnknownID 只带 rule_sources 改不存在的方案：404 + 与层一致的文案。
func TestProfilesRuleSourcesUnknownID(t *testing.T) {
	ts, _, _ := profileServer(t)

	st, body := doJSON(t, ts, http.MethodPut, "/api/profiles/99999", map[string]any{
		"name": "x", "rule_sources": []string{"a"},
	})
	if st != http.StatusNotFound {
		t.Fatalf("PUT 未知方案状态 = %d，应为 404；body=%s", st, body)
	}
	if !strings.Contains(body, "订阅方案 99999 不存在") {
		t.Fatalf("404 文案不对：%s", body)
	}
}

// TestNormalizeRuleSources 直接覆盖归一化函数的边界：空输入、纯空白、
// 去重保序、以及 20 条上限截断。
func TestNormalizeRuleSources(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"空输入", nil, nil},
		{"空数组", []string{}, nil},
		{"纯空白", []string{"  ", "\t", ""}, nil},
		{"trim 去重保序", []string{" b ", "a", "b", "a "}, []string{"b", "a"}},
		{"上限 20", func() []string {
			out := make([]string, 30)
			for i := range out {
				out[i] = fmt.Sprintf("s%02d", i)
			}
			return out
		}(), func() []string {
			out := make([]string, 20)
			for i := range out {
				out[i] = fmt.Sprintf("s%02d", i)
			}
			return out
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeRuleSources(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("结果 = %v，期望 %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("结果 = %v，期望 %v", got, tc.want)
				}
			}
		})
	}
}

// nil 资料不得 panic（仅为提升分支覆盖率；实际调用 path 上 p 恒非 nil）。
func TestNormalizeProfileSourcesForJSON(t *testing.T) {
	normalizeProfileSourcesForJSON(nil) // 不应 panic
	var p subforge.Profile
	normalizeProfileSourcesForJSON(&p)
	if p.RuleSources == nil {
		t.Fatal("期望 nil→[]string{}，却还是 nil")
	}
	if len(p.RuleSources) != 0 || fmt.Sprintf("%T", p.RuleSources) != "[]string" {
		t.Fatalf("期望 []string{}，得到 %#v", p.RuleSources)
	}
}

// --- /subscribe ---

// TestSubscribeInjectsRuleSourceRules 方案绑定 src1 且产物有内容：
// 产物规则内联进订阅，模板规则整层停用；模板自带的 FINAL/MATCH 被丢弃，
// 但渲染器必须补一条兜底 MATCH，否则未匹配流量会静默直连。
func TestSubscribeInjectsRuleSourceRules(t *testing.T) {
	ts, tplURL, _ := ruleSourceSubServer(t)

	p := mustProfilePayload(t, ts, map[string]any{
		"name": "注入方案", "config": tplURL, "rule_sources": []string{"src1"},
	})
	mustToken(t, ts, "t-src", p.ID)

	st, body := subscribe(t, ts, "t-src")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d；body=%s", st, body)
	}
	for _, want := range []string{"DOMAIN-SUFFIX,a.example,DIRECT", "DOMAIN-SUFFIX,b.example,REJECT"} {
		if !strings.Contains(body, want) {
			t.Errorf("订阅正文缺少注入规则 %q；正文：\n%s", want, body)
		}
	}
	if strings.Contains(body, "tplonly.test") {
		t.Errorf("产物注入后模板规则应整层停用，但 tplonly.test 仍在；正文：\n%s", body)
	}
	if !strings.Contains(body, "MATCH,🚀 手动选择") {
		t.Errorf("产物注入后必须补兜底 MATCH,🚀 手动选择；正文：\n%s", body)
	}
	if strings.Count(body, "MATCH,") != 1 {
		t.Errorf("兜底 MATCH 不应重复；正文：\n%s", body)
	}
}

// TestSubscribeRuleSourceMissingKeepsTemplateRules 源不存在 → 注入 0 条：
// 订阅照常成功，模板规则与模板 FINAL 都保留（warning 只是提示，不阻断）。
// TestSubscribeRuleSourceMissingRejected 绑定不存在的规则源应在保存时 400，
// 而不是保存成功、订阅时静默回退模板规则。
func TestSubscribeRuleSourceMissingRejected(t *testing.T) {
	ts, _, _ := profileServer(t)
	st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name": "缺源方案", "rule_sources": []string{"nope"},
	})
	if st != http.StatusBadRequest || !strings.Contains(body, "不存在") {
		t.Fatalf("缺源应 400 且提示不存在；status=%d body=%s", st, body)
	}
}

// TestSubscribeRuleSourceEmptyRejected 源在 manifest 里但还没有产物：保存同样 400。
func TestSubscribeRuleSourceEmptyRejected(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ruleSourceFixture(t, mgr, "src1", "yaml") // 只登记源，不落产物
	st, body := postAPIJSON(t, ts, "/api/profiles", map[string]any{
		"name": "无产物方案", "rule_sources": []string{"src1"},
	})
	if st != http.StatusBadRequest || !strings.Contains(body, "还没有可用产物") {
		t.Fatalf("无产物应 400 且提示先更新；status=%d body=%s", st, body)
	}
}
