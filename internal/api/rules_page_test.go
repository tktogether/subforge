package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAPIRulesPaginationFiltersAndSources 覆盖 GET /api/rules 的服务端分页、
// enabled/q/source 过滤、include_raw 字段裁剪，以及轻量来源计数接口。
func TestAPIRulesPaginationFiltersAndSources(t *testing.T) {
	ts, mgr := uiSubServer(t)
	if _, err := mgr.ImportRulesToCustom(t.Context(), "", []string{"DOMAIN,raw.test,DIRECT"}, false); err != nil {
		t.Fatalf("ImportRulesToCustom: %v", err)
	}
	for i, param := range []string{"a.test", "b.test", "c.test"} {
		createRule(t, ts, "DOMAIN", param, 0, "", i%2 == 0)
	}
	createRule(t, ts, "DOMAIN-SUFFIX", "foo.example", 0, "", false)
	createRule(t, ts, "IP-CIDR", "10.0.0.0/8", 0, "", false)
	createRule(t, ts, "DOMAIN", "manual.test", 0, "manual", true)
	createRule(t, ts, "DOMAIN", "manual2.test", 0, "manual", true)

	var page struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}
	st, _, body := uiGet(t, ts, "/api/rules?limit=2&offset=1&include_raw=1")
	if st != http.StatusOK {
		t.Fatalf("分页状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("解析分页: %v", err)
	}
	if page.Total != 8 || page.Limit != 2 || page.Offset != 1 || len(page.Rules) != 2 {
		t.Fatalf("分页结果不对：%+v body=%s", page, body)
	}
	st, _, body = uiGet(t, ts, "/api/rules?q=raw.test&include_raw=1")
	if st != http.StatusOK {
		t.Fatalf("include_raw 状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil || page.Total != 1 {
		t.Fatalf("include_raw 结果不对：%+v body=%s", page, body)
	}
	if page.Rules[0].Raw == "" || page.Rules[0].OriginKey == "" {
		t.Fatalf("include_raw=1 应返回 raw/origin_key：%+v", page.Rules[0])
	}

	// 默认不返回 raw/origin_key，也不再为了下拉拉全量。
	st, _, body = uiGet(t, ts, "/api/rules?limit=100")
	if st != http.StatusOK {
		t.Fatalf("默认分页状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("解析默认分页: %v", err)
	}
	if page.Total != 8 || page.Rules[0].Raw != "" || page.Rules[0].OriginKey != "" {
		t.Fatalf("默认不应返回 raw/origin_key：%+v", page.Rules[0])
	}
	st, _, body = uiGet(t, ts, "/api/rules")
	if st != http.StatusOK {
		t.Fatalf("无 limit 状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("解析无 limit: %v", err)
	}
	if page.Total != 8 || len(page.Rules) != 8 {
		t.Fatalf("无 limit 应保持旧的全量行为：%+v", page)
	}

	// enabled/q/source 过滤。
	st, _, body = uiGet(t, ts, "/api/rules?enabled=0&limit=100")
	if st != http.StatusOK {
		t.Fatalf("enabled 过滤状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil || page.Total != 3 {
		t.Fatalf("enabled=0 总数 = %d，应 3；body=%s", page.Total, body)
	}
	st, _, body = uiGet(t, ts, "/api/rules?q=foo&limit=100")
	if st != http.StatusOK {
		t.Fatalf("q 过滤状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil || page.Total != 1 || page.Rules[0].RuleParam != "foo.example" {
		t.Fatalf("q 过滤结果不对：%+v body=%s", page, body)
	}
	st, _, body = uiGet(t, ts, "/api/rules?source=manual&limit=100")
	if st != http.StatusOK {
		t.Fatalf("source 过滤状态 = %d；body=%s", st, body)
	}
	page = struct {
		Rules  []ruleJSON `json:"rules"`
		Total  int        `json:"total"`
		Limit  int        `json:"limit"`
		Offset int        `json:"offset"`
	}{}
	if err := json.Unmarshal([]byte(body), &page); err != nil || page.Total != 2 {
		t.Fatalf("source 过滤结果不对：%+v body=%s", page, body)
	}

	// 轻量来源计数。
	var sources struct {
		Sources []struct {
			Source string `json:"source"`
			Count  int64  `json:"count"`
		} `json:"sources"`
	}
	st, _, body = uiGet(t, ts, "/api/rules/sources")
	if st != http.StatusOK {
		t.Fatalf("来源计数状态 = %d；body=%s", st, body)
	}
	if err := json.Unmarshal([]byte(body), &sources); err != nil {
		t.Fatalf("解析来源计数: %v", err)
	}
	counts := map[string]int64{}
	for _, s := range sources.Sources {
		counts[s.Source] = s.Count
	}
	if counts[""] != 6 || counts["manual"] != 2 {
		t.Fatalf("来源计数不对：%+v", counts)
	}

	// 参数校验。
	for _, q := range []string{"limit=0", "limit=501", "limit=x", "offset=-1", "enabled=bogus"} {
		if st, _, _ := uiGet(t, ts, "/api/rules?"+q); st != http.StatusBadRequest {
			t.Fatalf("%s 状态 = %d，应为 400", q, st)
		}
	}
}
