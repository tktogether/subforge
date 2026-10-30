package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/subforge"
)

// TestAirportRuleSourceUpdateAndGuard 规则源页的更新必须走机场规则提取，
// 不能把机场完整 YAML（含节点配置）写进规则产物；机场源也不能单独删除。
func TestAirportRuleSourceUpdateAndGuard(t *testing.T) {
	ts, _, mgr := profileServer(t)
	up := rulesUpstream(t)
	id := addImportAirport(t, mgr, "src-up", up.URL)

	st, body := postAPIJSON(t, ts, fmt.Sprintf("/api/airports/%d/import-rules", id), map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("首次导入状态 = %d；body=%s", st, body)
	}
	var imported struct {
		RuleSourceName string `json:"rule_source_name"`
	}
	mustDecode(t, body, &imported)
	if imported.RuleSourceName == "" {
		t.Fatalf("未生成机场规则源：%s", body)
	}

	// 单源更新：走机场订阅重新提取，返回版本。
	st, body = postAPIJSON(t, ts, "/api/rule-sources/update-one", map[string]any{"name": imported.RuleSourceName})
	if st != http.StatusOK {
		t.Fatalf("单源更新状态 = %d；body=%s", st, body)
	}
	var updated struct {
		Results []struct {
			Source  string `json:"source"`
			Version string `json:"version"`
			Err     string `json:"err"`
		} `json:"results"`
	}
	mustDecode(t, body, &updated)
	if len(updated.Results) != 1 || updated.Results[0].Err != "" || updated.Results[0].Version == "" {
		t.Fatalf("单源更新结果不对：%+v body=%s", updated, body)
	}

	// 产物是规则 YAML，不应包含机场节点配置。
	sources, err := mgr.ListRuleSources(t.Context())
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	var latestPath string
	for _, src := range sources {
		if src.Name == imported.RuleSourceName {
			latestPath = filepath.Join(src.Dir, src.Latest)
		}
	}
	if latestPath == "" {
		t.Fatalf("找不到机场规则源产物：%+v", sources)
	}
	b, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("读取规则源产物: %v", err)
	}
	if strings.Contains(string(b), "proxies:") {
		t.Fatalf("规则源产物混入了机场节点配置：%s", string(b))
	}
	if !strings.Contains(string(b), "rules:") {
		t.Fatalf("规则源产物缺少 rules：%s", string(b))
	}

	// 全部更新：机场源走机场提取，其它源走远程清单更新。
	st, body = postAPIJSON(t, ts, "/api/rule-sources/update", map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("全部更新状态 = %d；body=%s", st, body)
	}
	var all struct {
		Results []struct {
			Source string `json:"source"`
			Err    string `json:"err"`
		} `json:"results"`
	}
	mustDecode(t, body, &all)
	if len(all.Results) != 1 || all.Results[0].Source != imported.RuleSourceName || all.Results[0].Err != "" {
		t.Fatalf("全部更新结果不对：%+v body=%s", all, body)
	}

	// 机场源由机场维护，不能单独删除。
	st, body = postAPIJSON(t, ts, "/api/rule-sources/remove", map[string]any{"name": imported.RuleSourceName})
	if st != http.StatusBadRequest || !strings.Contains(body, "机场规则源") {
		t.Fatalf("删除机场规则源状态 = %d body=%s，应 400 且说明由机场维护", st, body)
	}
}

func TestAirportRuleSourceNaming(t *testing.T) {
	name := subforge.AirportRuleSourceName("a:b/c", 7)
	if name != "airport-7-a-b-c" {
		t.Fatalf("AirportRuleSourceName = %q", name)
	}
	if got := subforge.AirportRuleSourceLabel(name); got != "a-b-c" {
		t.Fatalf("AirportRuleSourceLabel = %q", got)
	}
	id, ok := subforge.AirportIDFromRuleSourceName(name)
	if !ok || id != 7 {
		t.Fatalf("AirportIDFromRuleSourceName = %d/%v", id, ok)
	}
	if name := subforge.AirportRuleSourceName("  ", 9); name != "airport-9" {
		t.Fatalf("空机场名 = %q，应为 airport-9", name)
	}
	if _, ok := subforge.AirportIDFromRuleSourceName("remote-source"); ok {
		t.Fatal("远程源不应被识别成机场源")
	}
	if _, ok := subforge.AirportIDFromRuleSourceName("airport-x-y"); ok {
		t.Fatal("非法 id 不应被识别")
	}
	// 防止文件名过长。
	long := strings.Repeat("长", 120)
	got := subforge.AirportRuleSourceName(long, 1)
	if len([]rune(got)) > 100 {
		t.Fatalf("规则源名过长: %d", len([]rune(got)))
	}
}
