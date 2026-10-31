package subforge

// 机场规则源：命名、版本写入、provider 读取、方案解绑与远程源更新边界。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/model"
)

func findRuleSourceStatus(t *testing.T, sources []RuleSourceStatus, name string) RuleSourceStatus {
	t.Helper()
	for _, s := range sources {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("规则源 %q 不存在: %+v", name, sources)
	return RuleSourceStatus{}
}

func TestAirportRuleSourceNaming(t *testing.T) {
	cases := []struct {
		name string
		id   int64
		want string
	}{
		{"守候-SNTP", 2, "airport-2-守候-SNTP"},
		{"A:B/C", 7, "airport-7-A-B-C"},
		{"  ", 9, "airport-9"},
		{"a..", 3, "airport-3-a"},
	}
	for _, c := range cases {
		if got := AirportRuleSourceName(c.name, c.id); got != c.want {
			t.Errorf("AirportRuleSourceName(%q, %d) = %q, want %q", c.name, c.id, got, c.want)
		}
	}
	long := strings.Repeat("港", 81)
	part := strings.TrimPrefix(AirportRuleSourceName(long, 4), "airport-4-")
	if len([]rune(part)) != 80 {
		t.Errorf("超长机场名应截到 80 个字符，got %d", len([]rune(part)))
	}
	if got := AirportRuleSourceName("a\x01b", 5); got != "airport-5-a-b" {
		t.Errorf("控制字符应替换为 -: %q", got)
	}

	labels := map[string]string{
		"airport-2-守候-SNTP": "守候-SNTP",
		"airport-2":         "2",
		"airport-x-a":       "x-a",
		"custom":            "custom",
		"":                  "",
	}
	for in, want := range labels {
		if got := AirportRuleSourceLabel(in); got != want {
			t.Errorf("AirportRuleSourceLabel(%q) = %q, want %q", in, got, want)
		}
	}

	ids := []struct {
		name string
		id   int64
		ok   bool
	}{
		{"airport-2-a", 2, true},
		{"airport-2", 0, false},
		{"airport--a", 0, false},
		{"airport-0-a", 0, false},
		{"airport-abc-a", 0, false},
		{"custom", 0, false},
	}
	for _, c := range ids {
		id, ok := AirportIDFromRuleSourceName(c.name)
		if id != c.id || ok != c.ok {
			t.Errorf("AirportIDFromRuleSourceName(%q) = (%d, %v), want (%d, %v)", c.name, id, ok, c.id, c.ok)
		}
	}
}

// TestSaveAirportRuleSource 覆盖版本写入、改名复用、当前指针与展示名跟随机场名。
func TestSaveAirportRuleSource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "rules")
	writeRuleManifest(t, m, "https://example.test/remote", "clash-meta", outDir)

	airport, err := m.AddAirport(ctx, "守候:SNTP", "https://up.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	rules := []string{"DOMAIN-SUFFIX,a.com,DIRECT", "RULE-SET,p1,机场组"}
	providers := []model.RuleProvider{{Name: "p1", Raw: map[string]any{
		"type": "http", "url": "https://rules.example/p1.yaml", "interval": 3600,
	}}}

	res, err := m.SaveAirportRuleSource(ctx, airport.Name, airport.ID, airport.SubscriptionURL, rules, providers)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}
	wantName := AirportRuleSourceName(airport.Name, airport.ID)
	if res.Name != wantName || res.Label != "守候-SNTP" || res.Rules != 2 || res.Providers != 1 {
		t.Fatalf("结果不符: %+v", res)
	}
	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, want := range []string{"DOMAIN-SUFFIX,a.com,DIRECT", "rule-providers:", "p1:", "https://rules.example/p1.yaml"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("产物缺少 %q:\n%s", want, body)
		}
	}
	sources, err := m.ListRuleSources(ctx)
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	st := findRuleSourceStatus(t, sources, res.Name)
	if st.Origin != "airport" || st.Label != "守候:SNTP" || st.Latest != filepath.Base(res.Path) || st.RuleCount != 2 {
		t.Fatalf("来源状态不符: %+v", st)
	}

	// 改名后仍复用同一内部源，展示名跟随机场当前名。
	if _, err := m.UpdateAirport(ctx, airport.ID, "守候-新名", airport.SubscriptionURL, "", true); err != nil {
		t.Fatalf("UpdateAirport: %v", err)
	}
	res2, err := m.SaveAirportRuleSource(ctx, "守候-新名", airport.ID, "https://up.example/sub2", []string{"DOMAIN,b.com,DIRECT"}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource(rename): %v", err)
	}
	if res2.Name != res.Name {
		t.Fatalf("改名后源名应复用 %q，got %q", res.Name, res2.Name)
	}
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		t.Fatalf("LoadRuleManifest: %v", err)
	}
	var airportSources int
	for _, src := range mf.Sources {
		if src.SourceName() == res.Name {
			airportSources++
			if src.URL != "https://up.example/sub2" {
				t.Fatalf("清单里的机场 URL 未更新: %+v", src)
			}
		}
	}
	if airportSources != 1 {
		t.Fatalf("同一机场应只有 1 个规则源，got %d", airportSources)
	}
	sources, _ = m.ListRuleSources(ctx)
	st = findRuleSourceStatus(t, sources, res.Name)
	if st.Label != "守候-新名" {
		t.Fatalf("展示名应跟随机场名: %+v", st)
	}

	// 机场删除后源作为历史快照保留，并标记 airport_missing（展示名回退到内部名）。
	if err := m.DeleteAirport(ctx, airport.ID); err != nil {
		t.Fatalf("DeleteAirport: %v", err)
	}
	sources, _ = m.ListRuleSources(ctx)
	st = findRuleSourceStatus(t, sources, res.Name)
	if !st.AirportMissing || st.Label != "守候-SNTP" {
		t.Fatalf("机场删除后的状态不符: %+v", st)
	}
}

// TestSaveAirportRuleSourceTextFormat 文本产物不能带 rule-providers。
func TestSaveAirportRuleSourceTextFormat(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "rules")
	writeRuleManifest(t, m, "https://example.test/remote", "text", outDir)
	airport, err := m.AddAirport(ctx, "文本机场", "https://up.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	_, err = m.SaveAirportRuleSource(ctx, airport.Name, airport.ID, airport.SubscriptionURL,
		[]string{"DOMAIN,a.com,DIRECT"}, []model.RuleProvider{{Name: "p1", Raw: map[string]any{"type": "http"}}})
	if err == nil || !strings.Contains(err.Error(), "无法保存 rule-providers") {
		t.Fatalf("文本格式带 provider 应报错, got %v", err)
	}
	res, err := m.SaveAirportRuleSource(ctx, airport.Name, airport.ID, airport.SubscriptionURL,
		[]string{"DOMAIN,a.com,DIRECT", "# 注释"}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource(text): %v", err)
	}
	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(body), "rule-providers") || !strings.Contains(string(body), "DOMAIN,a.com,DIRECT") {
		t.Fatalf("文本产物内容不符:\n%s", body)
	}
	if res.Providers != 0 {
		t.Fatalf("Providers = %d, want 0", res.Providers)
	}
}

// TestRuleSourceRuleProviders 覆盖去重、未知源与清单损坏告警。
func TestRuleSourceRuleProviders(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if got, warns := m.RuleSourceRuleProviders(ctx, []string{"", "  "}); got != nil || warns != nil {
		t.Fatalf("空名单应返回 nil/nil, got %+v/%v", got, warns)
	}

	outDir := filepath.Join(t.TempDir(), "rules")
	writeRuleManifest(t, m, "https://example.test/remote", "clash-meta", outDir)
	airport, err := m.AddAirport(ctx, "守候", "https://up.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	res, err := m.SaveAirportRuleSource(ctx, airport.Name, airport.ID, airport.SubscriptionURL,
		[]string{"RULE-SET,p1,组"}, []model.RuleProvider{{Name: "p1", Raw: map[string]any{"type": "http", "url": "https://rules.example/p1.yaml"}}})
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}
	got, warns := m.RuleSourceRuleProviders(ctx, []string{res.Name, res.Name, "ghost"})
	if len(warns) != 0 {
		t.Fatalf("不应有告警: %v", warns)
	}
	if len(got) != 1 || got[0].Name != "p1" || got[0].Raw["url"] != "https://rules.example/p1.yaml" {
		t.Fatalf("provider 不符: %+v", got)
	}

	// 清单损坏：降级为告警而不是 panic/error。
	bad := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := os.WriteFile(bad, []byte("sources: [\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.SetRuleManifestPath(ctx, bad); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	got, warns = m.RuleSourceRuleProviders(ctx, []string{res.Name})
	if got != nil || len(warns) != 1 || !strings.Contains(warns[0], "读取规则源清单失败") {
		t.Fatalf("清单损坏应返回告警, got %+v/%v", got, warns)
	}
}

// TestUnbindAirportRuleSource 覆盖从方案移除绑定、无绑定与未知机场。
func TestUnbindAirportRuleSource(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	outDir := filepath.Join(t.TempDir(), "rules")
	writeRuleManifest(t, m, "https://example.test/remote", "clash-meta", outDir)
	airport, err := m.AddAirport(ctx, "守候", "https://up.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	res, err := m.SaveAirportRuleSource(ctx, airport.Name, airport.ID, airport.SubscriptionURL, []string{"DOMAIN,a.com,DIRECT"}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}
	p1, err := m.AddProfile(ctx, "方案一", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	p2, err := m.AddProfile(ctx, "方案二", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := m.SetProfileRuleSources(ctx, p1.ID, []string{"other", res.Name}); err != nil {
		t.Fatalf("SetProfileRuleSources(p1): %v", err)
	}
	if err := m.SetProfileRuleSources(ctx, p2.ID, []string{"other2"}); err != nil {
		t.Fatalf("SetProfileRuleSources(p2): %v", err)
	}

	name, err := m.UnbindAirportRuleSource(ctx, airport.ID)
	if err != nil {
		t.Fatalf("UnbindAirportRuleSource: %v", err)
	}
	if name != res.Name {
		t.Fatalf("返回源名 = %q, want %q", name, res.Name)
	}
	gotP1, _ := m.GetProfile(ctx, p1.ID)
	if len(gotP1.RuleSources) != 1 || gotP1.RuleSources[0] != "other" {
		t.Fatalf("方案一绑定未清理: %+v", gotP1.RuleSources)
	}
	gotP2, _ := m.GetProfile(ctx, p2.ID)
	if len(gotP2.RuleSources) != 1 || gotP2.RuleSources[0] != "other2" {
		t.Fatalf("无关方案不应被改: %+v", gotP2.RuleSources)
	}

	other, err := m.AddAirport(ctx, "无规则源机场", "https://up.example/other", "", true)
	if err != nil {
		t.Fatalf("AddAirport(other): %v", err)
	}
	name, err = m.UnbindAirportRuleSource(ctx, other.ID)
	if err != nil || name != "" {
		t.Fatalf("没有规则源的机场应返回空名, got %q/%v", name, err)
	}
}

// TestUpdateRemoteRuleSources 只更新远程源，机场源不在远程更新链路里。
func TestUpdateRemoteRuleSources(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("rules:\n  - DOMAIN-SUFFIX,remote.com,DIRECT\n"))
	}))
	defer srv.Close()
	outDir := filepath.Join(t.TempDir(), "rules")
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	content := "sources:\n" +
		"  - name: remote\n    url: " + srv.URL + "\n" +
		"  - name: airport-2-demo\n    url: https://invalid.example/never\n" +
		"output:\n  format: clash-meta\n  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	results, err := m.UpdateRemoteRuleSources(ctx)
	if err != nil {
		t.Fatalf("UpdateRemoteRuleSources: %v", err)
	}
	if len(results) != 1 || results[0].Source != "remote" || results[0].Err != nil {
		t.Fatalf("只应更新远程源: %+v", results)
	}
	if _, err := os.Stat(results[0].Path); err != nil {
		t.Fatalf("远程源产物不存在: %v", err)
	}

	// 只有机场源时不需要任何网络请求。
	onlyAirport := "sources:\n  - name: airport-2-demo\n    url: https://invalid.example/never\n" +
		"output:\n  format: clash-meta\n  path: " + filepath.ToSlash(outDir) + "\n"
	if err := os.WriteFile(path, []byte(onlyAirport), 0o644); err != nil {
		t.Fatalf("WriteFile(only airport): %v", err)
	}
	results, err = m.UpdateRemoteRuleSources(ctx)
	if err != nil {
		t.Fatalf("只有机场源时不应报错: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("只有机场源时应返回空结果: %+v", results)
	}
}
