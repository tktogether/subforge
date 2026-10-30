package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestRuleSourceRulesSearchAndImport 覆盖规则源预览接口和「挑选到自定义」的冲突处理。
func TestRuleSourceRulesSearchAndImport(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()
	g, err := mgr.AddProxyGroup(ctx, "PROXY", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	src, err := mgr.SaveAirportRuleSource(ctx, "HK", 1, "http://example.com/sub", []string{
		"DOMAIN-SUFFIX,example.com,PROXY",
		"IP-CIDR,1.1.1.1/32,DIRECT,no-resolve",
		"RULE-SET,ads,REJECT",
	}, nil)
	if err != nil {
		t.Fatalf("SaveAirportRuleSource: %v", err)
	}

	// 预览：分页、搜索、参数校验。
	var page struct {
		Total int      `json:"total"`
		Rules []string `json:"rules"`
	}
	st, body := uiGetBody(t, ts, "/api/rule-sources/rules?name="+url.QueryEscape(src.Name))
	if st != http.StatusOK {
		t.Fatalf("预览状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &page)
	if page.Total != 3 || len(page.Rules) != 3 {
		t.Fatalf("预览规则数 = %d/%d，应为 3", page.Total, len(page.Rules))
	}
	st, body = uiGetBody(t, ts, "/api/rule-sources/rules?name="+url.QueryEscape(src.Name)+"&offset=1&limit=1")
	if st != http.StatusOK {
		t.Fatalf("分页状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &page)
	if page.Total != 3 || len(page.Rules) != 1 || page.Rules[0] != "IP-CIDR,1.1.1.1/32,DIRECT,no-resolve" {
		t.Fatalf("分页结果不对：%+v", page)
	}
	st, body = uiGetBody(t, ts, "/api/rule-sources/rules?name="+url.QueryEscape(src.Name)+"&q=EXAMPLE")
	if st != http.StatusOK {
		t.Fatalf("搜索状态 = %d；body=%s", st, body)
	}
	mustDecode(t, body, &page)
	if page.Total != 1 || page.Rules[0] != "DOMAIN-SUFFIX,example.com,PROXY" {
		t.Fatalf("搜索结果不对：%+v", page)
	}
	if st, _, _ := uiGet(t, ts, "/api/rule-sources/rules"); st != http.StatusBadRequest {
		t.Fatalf("缺少 name 状态 = %d，应为 400", st)
	}
	if st, _, _ := uiGet(t, ts, "/api/rule-sources/rules?name=nope"); st != http.StatusNotFound {
		t.Fatalf("不存在源状态 = %d，应为 404", st)
	}
	if st, _, _ := uiGet(t, ts, "/api/rule-sources/rules?name="+url.QueryEscape(src.Name)+"&limit=201"); st != http.StatusBadRequest {
		t.Fatalf("limit=201 状态 = %d，应为 400", st)
	}

	// 第一次导入：2 条普通规则进自定义，RULE-SET 依赖 provider 被跳过。
	importBody := map[string]any{
		"name":  src.Name,
		"rules": []string{"DOMAIN-SUFFIX,example.com,PROXY", "IP-CIDR,1.1.1.1/32,DIRECT,no-resolve", "RULE-SET,ads,REJECT"},
	}
	res, body := postPickImport(t, ts, importBody)
	if res.Added != 2 || res.Skipped != 1 || len(res.Warnings) == 0 {
		t.Fatalf("首次挑选结果不对：%+v body=%s", res, body)
	}
	custom := listRules(t, ts, "?source=")
	if len(custom) != 2 {
		t.Fatalf("自定义规则数 = %d，应为 2：%+v", len(custom), custom)
	}
	foundProxy := false
	for _, r := range custom {
		if r.RuleParam == "example.com" {
			foundProxy = r.RuleConfig == g.ID
		}
	}
	if !foundProxy {
		t.Fatalf("example.com 应指向 PROXY 分组：%+v", custom)
	}

	// 第二次相同规则：全部跳过（2 条重复 + 1 条 RULE-SET）。
	res, body = postPickImport(t, ts, importBody)
	if res.Added != 0 || res.Skipped != 3 {
		t.Fatalf("重复挑选应全部跳过：%+v body=%s", res, body)
	}

	// 冲突覆盖：同 type+param 但目标变化时，raw 也要跟着更新，否则渲染会继续用旧目标。
	res, body = postPickImport(t, ts, map[string]any{
		"name": src.Name, "rules": []string{"DOMAIN-SUFFIX,example.com,DIRECT"}, "conflict": "overwrite",
	})
	if res.Added != 0 || res.Overwrote != 1 || res.Skipped != 0 {
		t.Fatalf("覆盖挑选结果不对：%+v body=%s", res, body)
	}
	custom = listRules(t, ts, "?source=")
	foundDirect := false
	for _, r := range custom {
		if r.RuleParam == "example.com" {
			foundDirect = r.RuleConfig == 0 && strings.Contains(r.Raw, "example.com,DIRECT")
		}
	}
	if !foundDirect {
		t.Fatalf("覆盖后 raw/target 未更新：%+v", custom)
	}
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30001?allowInsecure=1#orig", "NODE-A", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	mustToken(t, ts, "t-pick", 0)
	st, sub := subscribe(t, ts, "t-pick")
	if st != http.StatusOK {
		t.Fatalf("订阅状态 = %d；body=%s", st, sub)
	}
	if !strings.Contains(sub, "DOMAIN-SUFFIX,example.com,DIRECT") || strings.Contains(sub, "DOMAIN-SUFFIX,example.com,PROXY") {
		t.Fatalf("覆盖后订阅仍使用旧目标：%s", sub)
	}

	// 参数校验。
	if st, _, _ := uiPostJSON(t, ts, "/api/rule-sources/import-rules", `{"name":"nope","rules":["DOMAIN,a.test,DIRECT"]}`); st != http.StatusNotFound {
		t.Fatalf("不存在源导入状态 = %d，应为 404", st)
	}
	if st, _, _ := uiPostJSON(t, ts, "/api/rule-sources/import-rules", `{"name":"`+src.Name+`","rules":[]}`); st != http.StatusBadRequest {
		t.Fatalf("空 rules 状态 = %d，应为 400", st)
	}
}

type pickImportResponse struct {
	Added     int      `json:"added"`
	Skipped   int      `json:"skipped"`
	Overwrote int      `json:"overwrote"`
	Warnings  []string `json:"warnings"`
}

func postPickImport(t *testing.T, ts *httptest.Server, body map[string]any) (pickImportResponse, string) {
	t.Helper()
	buf, _ := json.Marshal(body)
	st, _, raw := uiPostJSON(t, ts, "/api/rule-sources/import-rules", string(buf))
	if st != http.StatusOK {
		t.Fatalf("导入状态 = %d；body=%s", st, raw)
	}
	var out pickImportResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("解析导入响应: %v；body=%s", err, raw)
	}
	return out, raw
}

func uiGetBody(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()
	st, _, body := uiGet(t, ts, path)
	return st, body
}

// TestRemoveRuleSourceBlockedWhenBound 被方案绑定的规则源不能直接删除，
// 必须先解绑；否则订阅会静默丢规则。
func TestRemoveRuleSourceBlockedWhenBound(t *testing.T) {
	ts, _, _ := profileServer(t)
	up := rulesUpstream(t)
	if st, _, body := uiPostJSON(t, ts, "/api/rule-sources",
		`{"name":"remote-src","url":"`+up.URL+`"}`); st != http.StatusOK {
		t.Fatalf("新增规则源状态 = %d；body=%s", st, body)
	}
	if st, _, body := uiPostJSON(t, ts, "/api/rule-sources/update-one",
		`{"name":"remote-src"}`); st != http.StatusOK {
		t.Fatalf("更新规则源状态 = %d；body=%s", st, body)
	}
	if st, _, body := uiPostJSON(t, ts, "/api/rule-sources",
		`{"name":"keep-src","url":"`+up.URL+`"}`); st != http.StatusOK {
		t.Fatalf("新增保留源状态 = %d；body=%s", st, body)
	}
	p := mustProfilePayload(t, ts, map[string]any{
		"name": "绑定源方案", "rule_sources": []string{"remote-src"},
	})
	st, _, body := uiPostJSON(t, ts, "/api/rule-sources/remove", `{"name":"remote-src"}`)
	if st != http.StatusConflict || !strings.Contains(body, "正被方案") {
		t.Fatalf("绑定中的规则源删除状态 = %d body=%s，应 409", st, body)
	}
	if st, _, body := uiDoJSON(t, ts, "DELETE", fmt.Sprintf("/api/profiles/%d", p.ID), ""); st != http.StatusOK {
		t.Fatalf("删除方案状态 = %d；body=%s", st, body)
	}
	if st, _, body := uiPostJSON(t, ts, "/api/rule-sources/remove", `{"name":"remote-src"}`); st != http.StatusOK {
		t.Fatalf("解绑后删除规则源状态 = %d；body=%s", st, body)
	}
}
