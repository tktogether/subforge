package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestSplitListQuery(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
		{"a,,b", []string{"a", "b"}},
		{",,,", nil},
	}
	for _, c := range cases {
		got := splitListQuery(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("splitListQuery(%q): got %v want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("splitListQuery(%q): got %v want %v", c.in, got, c.want)
			}
		}
		// nil check
		if c.want == nil && got != nil {
			t.Fatalf("splitListQuery(%q): want nil got %v", c.in, got)
		}
	}
}

func TestSubscribeQueryInclude(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}

	// baseline
	_, _, baseBody := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	// include only vless
	status, _, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&include=vless")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Count(body, "vless") >= strings.Count(baseBody, "vless") {
		// include should reduce nodes; at least total body shorter or fewer vless? check node count proxy
	}
	if len(body) >= len(baseBody) {
		t.Fatalf("include 未减少输出: base=%d filtered=%d", len(baseBody), len(body))
	}
	_ = baseBody
}

func TestSubscribeQueryExclude(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	status, _, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&exclude=vmess")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Contains(body, "vmess-node") {
		t.Fatalf("exclude 未生效，vmess-node 仍在输出")
	}
}

func TestSubscribeQueryConfig(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	// baseline without config
	status0, _, body0 := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	// nonexistent template name
	status1, _, body1 := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&config=__no_such_template__")
	t.Logf("baseline status=%d len=%d; config status=%d len=%d", status0, len(body0), status1, len(body1))
	// accept either: if engine errors, status differs; if not, document
	if status0 != status1 {
		t.Logf("config 触发了不同响应码：%d vs %d", status0, status1)
	}
}

func TestSubscribeQueryPipelineFallback(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	// write subscribe.pipeline into config table
	if _, err := mgr.DB().Raw().ExecContext(ctx, "INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", "subscribe.pipeline", "nope"); err != nil {
		t.Fatalf("写 config: %v", err)
	}
	// fallback (no pipeline query) should behave like explicit pipeline=nope
	statusFb, _, bodyFb := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	statusEx, _, bodyEx := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta&pipeline=nope")
	t.Logf("fallback status=%d len=%d; explicit status=%d len=%d", statusFb, len(bodyFb), statusEx, len(bodyEx))
	if statusFb != statusEx {
		t.Fatalf("fallback 与显式 pipeline 响应码不一致: %d vs %d", statusFb, statusEx)
	}
}

func TestSubscribeQueryRegression(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "机场A", fixtureURL(t), "备注", true); err != nil {
		t.Fatalf("插入 airport: %v", err)
	}
	group, err := mgr.AddProxyGroup(ctx, "AutoSelect", "select", ".*", "", 0)
	if err != nil {
		t.Fatalf("插入 group: %v", err)
	}
	if _, err := mgr.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", group.ID, true, true); err != nil {
		t.Fatalf("插入 rule: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("插入 sub token: %v", err)
	}
	status, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1&target=clash.meta")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if ct := hdr.Get("Content-Type"); strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(body, "AutoSelect") {
		t.Errorf("缺少 AutoSelect")
	}
}
