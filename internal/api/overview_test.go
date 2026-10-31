package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPIOverviewCounts 概览快照统计持久化资源，不依赖本次启动的转换历史。
func TestAPIOverviewCounts(t *testing.T) {
	srv, mgr := importTestServer(t)
	ctx := t.Context()
	if _, err := mgr.AddAirport(ctx, "机场A", "https://a.example/sub", "", true); err != nil {
		t.Fatalf("AddAirport A: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "机场B", "https://b.example/sub", "", false); err != nil {
		t.Fatalf("AddAirport B: %v", err)
	}
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30001?allowInsecure=1#n1", "自建1", true, 1); err != nil {
		t.Fatalf("AddSelfNode 1: %v", err)
	}
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30002?allowInsecure=1#n2", "自建2", false, 2); err != nil {
		t.Fatalf("AddSelfNode 2: %v", err)
	}
	if _, err := mgr.AddProfile(ctx, "方案A", "", "", "", true, 0); err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	if _, err := mgr.AddProfile(ctx, "方案B", "", "", "", false, 0); err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "tok-overview", "手机", "clash", "", "", "", ""); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}
	if _, err := mgr.AddProxyGroup(ctx, "节点选择", "select", "", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if _, err := mgr.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", 0, false, true); err != nil {
		t.Fatalf("AddRule enabled: %v", err)
	}
	if _, err := mgr.AddRule(ctx, "DOMAIN-SUFFIX", "off.example.com", 0, false, false); err != nil {
		t.Fatalf("AddRule disabled: %v", err)
	}

	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	status, _, body := uiGet(t, ts, "/api/overview")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Version string `json:"version"`
		Counts  struct {
			Airports         int `json:"airports"`
			AirportsEnabled  int `json:"airports_enabled"`
			SelfNodes        int `json:"self_nodes"`
			SelfNodesEnabled int `json:"self_nodes_enabled"`
			Profiles         int `json:"profiles"`
			ProfilesEnabled  int `json:"profiles_enabled"`
			Tokens           int `json:"tokens"`
			Groups           int `json:"groups"`
			Rules            int `json:"rules"`
			RulesEnabled     int `json:"rules_enabled"`
			RuleSources      int `json:"rule_sources"`
		} `json:"counts"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.Version != "test-version" {
		t.Fatalf("version=%q, want test-version", got.Version)
	}
	c := got.Counts
	if c.Airports != 2 || c.AirportsEnabled != 1 {
		t.Fatalf("airports=%d/%d, want 2/1", c.Airports, c.AirportsEnabled)
	}
	if c.SelfNodes != 2 || c.SelfNodesEnabled != 1 {
		t.Fatalf("self_nodes=%d/%d, want 2/1", c.SelfNodes, c.SelfNodesEnabled)
	}
	if c.Profiles != 2 || c.ProfilesEnabled != 1 {
		t.Fatalf("profiles=%d/%d, want 2/1", c.Profiles, c.ProfilesEnabled)
	}
	if c.Tokens != 1 {
		t.Fatalf("tokens=%d, want 1", c.Tokens)
	}
	if c.Groups != 1 {
		t.Fatalf("groups=%d, want 1", c.Groups)
	}
	if c.Rules != 2 || c.RulesEnabled != 1 {
		t.Fatalf("rules=%d/%d, want 2/1", c.Rules, c.RulesEnabled)
	}
	if c.RuleSources != 0 {
		t.Fatalf("rule_sources=%d, want 0", c.RuleSources)
	}
}
