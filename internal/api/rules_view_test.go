package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestRulesViewGroupNames(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	g, err := mgr.AddProxyGroup(ctx, "机场A", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	// 一条映射得上的规则（rule_config = 分组 id）。
	if _, err := mgr.DB().Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, source, raw, origin_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"DOMAIN", "example.com", g.ID, true, 0, 1, "airport:机场A", "DOMAIN,example.com,机场A", "DOMAIN, example.com, 机场A"); err != nil {
		t.Fatalf("insert rule1: %v", err)
	}
	// 一条映射不上的规则（rule_config = 0）。
	if _, err := mgr.DB().Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, source, raw, origin_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"IP-CIDR", "1.1.1.1/32", 0, false, 0, 2, "manual", "IP-CIDR,1.1.1.1/32,DIRECT", "IP-CIDR, 1.1.1.1/32, DIRECT"); err != nil {
		t.Fatalf("insert rule2: %v", err)
	}

	status, _, body := uiGet(t, ts, "/api/rules")
	if status != http.StatusOK {
		t.Fatalf("GET /api/rules status = %d, body=%s", status, body)
	}

	var resp struct {
		Rules []struct {
			RuleConfig int64  `json:"rule_config"`
			GroupName  string `json:"group_name"`
			Source     string `json:"source"`
			Raw        string `json:"raw"`
			OriginKey  string `json:"origin_key"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, body)
	}
	if len(resp.Rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2, body=%s", len(resp.Rules), body)
	}
	if resp.Rules[0].GroupName != "机场A" {
		t.Errorf("rule0 group_name = %q, want %q", resp.Rules[0].GroupName, "机场A")
	}
	if resp.Rules[1].GroupName != "" {
		t.Errorf("rule1 group_name = %q, want empty", resp.Rules[1].GroupName)
	}
	for i, r := range resp.Rules {
		if r.Source == "" || r.Raw == "" || r.OriginKey == "" {
			t.Errorf("rule%d source/raw/origin_key empty: %+v", i, r)
		}
	}
}
