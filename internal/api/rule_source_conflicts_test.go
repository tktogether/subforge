package api

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRuleSourceConflicts(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()
	a, err := mgr.SaveAirportRuleSource(ctx, "A", 1, "http://a", []string{
		"DOMAIN,a.test,DIRECT", "DOMAIN-SUFFIX,dup.test,DIRECT", "IP-CIDR,1.1.1.1/32,DIRECT",
	}, nil)
	if err != nil {
		t.Fatalf("Save A: %v", err)
	}
	b, err := mgr.SaveAirportRuleSource(ctx, "B", 2, "http://b", []string{
		"DOMAIN,a.test,REJECT", "DOMAIN-SUFFIX,dup.test,REJECT", "DOMAIN,only-b.test,REJECT",
	}, nil)
	if err != nil {
		t.Fatalf("Save B: %v", err)
	}
	st, _, body := uiGet(t, ts, "/api/rule-sources/conflicts?names="+url.QueryEscape(a.Name+","+b.Name))
	if st != http.StatusOK {
		t.Fatalf("状态 = %d；body=%s", st, body)
	}
	var resp struct {
		Total   int `json:"total"`
		Samples []struct {
			Type, Param, First, Second string
		} `json:"samples"`
	}
	mustDecode(t, body, &resp)
	if resp.Total != 2 || len(resp.Samples) != 2 {
		t.Fatalf("冲突结果不对：%+v", resp)
	}
	if resp.Samples[0].Param != "a.test" || resp.Samples[0].First != a.Name || resp.Samples[0].Second != b.Name {
		t.Fatalf("冲突示例不对：%+v", resp.Samples[0])
	}
	// 单源没有跨源冲突。
	st, _, body = uiGet(t, ts, "/api/rule-sources/conflicts?names="+url.QueryEscape(a.Name))
	if st != http.StatusOK {
		t.Fatalf("单源状态 = %d；body=%s", st, body)
	}
	var single struct{ Total int }
	mustDecode(t, body, &single)
	if single.Total != 0 {
		t.Fatalf("单源 total = %d，应为 0", single.Total)
	}
	if st, _, _ := uiPostJSON(t, ts, "/api/rule-sources/conflicts?names=a,b", ""); st != http.StatusMethodNotAllowed {
		t.Fatalf("POST 状态 = %d，应为 405", st)
	}
}
