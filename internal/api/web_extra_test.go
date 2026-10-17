package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestWebUI_StatsExtendedShape 验证扩展后的 /api/stats 契约。
func TestWebUI_StatsExtendedShape(t *testing.T) {
	ts := newTestServer(t, nil)
	body := `{"target":"clash.meta","text":"vmess://eyJ2IjoiMiIsInBzIjoidm1lc3Mtbm9kZSIsImFkZCI6InZtZXNzLmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiMTExMTExMTEtMjIyMi0zMzMzLTQ0NDQtNTU1NTU1NTU1NTU1IiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJ3cyIsInR5cGUiOiJub25lIiwiaG9zdCI6InZtZXNzLmV4YW1wbGUuY29tIiwicGF0aCI6Ii93cyIsInRscyI6InRscyIsInNuaSI6InZtZXNzLmV4YW1wbGUuY29tIn0="}`
	status, _, respBody := uiPostJSON(t, ts, "/api/convert", body)
	if status != http.StatusOK {
		t.Fatalf("convert 状态 = %d, body=%s", status, respBody)
	}
	status, _, respBody = uiGet(t, ts, "/api/stats")
	if status != http.StatusOK {
		t.Fatalf("stats 状态 = %d", status)
	}
	var v struct {
		History map[string]any `json:"history"`
		Logs    *int           `json:"logs"`
		Last    map[string]any `json:"last"`
	}
	if err := json.Unmarshal([]byte(respBody), &v); err != nil {
		t.Fatalf("stats 解析失败: %v\n%s", err, respBody)
	}
	for _, k := range []string{"total", "cachedHits", "urlTotal", "cacheHitRate"} {
		if _, ok := v.History[k]; !ok {
			t.Errorf("stats.history 缺少 %s: %s", k, respBody)
		}
	}
	if v.Last == nil {
		t.Fatalf("stats.last 为空: %s", respBody)
	}
	for _, k := range []string{"nodes", "rawNodes", "groups", "rules", "cached", "urlCount", "cacheHitRate"} {
		if _, ok := v.Last[k]; !ok {
			t.Errorf("stats.last 缺少 %s: %s", k, respBody)
		}
	}
}

// TestWebUI_ConvertText 验证 /api/convert 支持 text 字段。
func TestWebUI_ConvertText(t *testing.T) {
	ts := newTestServer(t, nil)
	text := "vmess://eyJ2IjoiMiIsInBzIjoidm1lc3Mtbm9kZSIsImFkZCI6InZtZXNzLmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiMTExMTExMTEtMjIyMi0zMzMzLTQ0NDQtNTU1NTU1NTU1NTU1IiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJ3cyIsInR5cGUiOiJub25lIiwiaG9zdCI6InZtZXNzLmV4YW1wbGUuY29tIiwicGF0aCI6Ii93cyIsInRscyI6InRscyIsInNuaSI6InZtZXNzLmV4YW1wbGUuY29tIn0="
	status, _, body := uiPostJSON(t, ts, "/api/convert", `{"target":"clash.meta","text":"`+text+`"}`)
	if status != http.StatusOK {
		t.Fatalf("text 转换状态 = %d, body=%s", status, body)
	}
	if !strings.Contains(body, "proxies:") {
		t.Errorf("text 转换产物缺少 proxies: %s", body)
	}
}

// TestWebUI_RulesetsIncludesRules 验证 /api/rulesets 包含 rules 数组。
func TestWebUI_RulesetsIncludesRules(t *testing.T) {
	ts := newTestServer(t, nil)
	status, _, body := uiGet(t, ts, "/api/rulesets")
	if status != http.StatusOK {
		t.Fatalf("rulesets 状态 = %d", status)
	}
	var v struct {
		Rulesets map[string]string `json:"rulesets"`
		Rules    []string          `json:"rules"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("rulesets 解析失败: %v\n%s", err, body)
	}
	if v.Rules == nil {
		t.Errorf("rulesets.rules 应为数组（非 null）: %s", body)
	}
}
