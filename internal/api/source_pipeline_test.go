package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSubscribe_PerSourcePipelines 机场 A/B 与自建节点先各自经过来源管线，
// 合并后再执行方案最终管线；三种来源互不串处理。
func TestSubscribe_PerSourcePipelines(t *testing.T) {
	ts, _, mgr := profileServer(t)
	ctx := t.Context()

	upA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: A-01, type: ss, server: 10.0.0.1, port: 8388, cipher: aes-256-gcm, password: pw}\n"))
	}))
	t.Cleanup(upA.Close)
	upB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: B-01, type: ss, server: 10.0.1.1, port: 8388, cipher: aes-256-gcm, password: pw}\n"))
	}))
	t.Cleanup(upB.Close)

	a, err := mgr.AddAirport(ctx, "机场A", upA.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport A: %v", err)
	}
	b, err := mgr.AddAirport(ctx, "机场B", upB.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport B: %v", err)
	}
	if _, err := mgr.AddSelfNode(ctx, "trojan://pass@127.0.0.1:30001?allowInsecure=1#SELF-01", "SELF-01", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}

	mk := func(name, prefix string) {
		t.Helper()
		steps := fmt.Sprintf(`[{"op":"op.rename","args":{"pattern":"^","repl":%q}}]`, prefix)
		if _, err := mgr.CreatePipeline(ctx, name, "", steps, true); err != nil {
			t.Fatalf("CreatePipeline %s: %v", name, err)
		}
	}
	mk("prefixA", "A-")
	mk("prefixB", "B-")
	mk("prefixSelf", "S-")
	mk("final", "F-")
	if err := mgr.SetAirportPipeline(ctx, a.ID, "prefixA"); err != nil {
		t.Fatalf("SetAirportPipeline A: %v", err)
	}
	if err := mgr.SetAirportPipeline(ctx, b.ID, "prefixB"); err != nil {
		t.Fatalf("SetAirportPipeline B: %v", err)
	}
	if err := mgr.SetSelfSourcePipeline(ctx, "prefixSelf"); err != nil {
		t.Fatalf("SetSelfSourcePipeline: %v", err)
	}

	p := mustProfilePayload(t, ts, map[string]any{
		"name": "来源处理", "airport_ids": []int64{a.ID, b.ID}, "pipeline": "final",
	})
	mustToken(t, ts, "t-source-pipeline", p.ID)
	st, body := subscribe(t, ts, "t-source-pipeline")
	if st != http.StatusOK {
		t.Fatalf("订阅状态=%d body=%s", st, body)
	}
	for _, want := range []string{"F-A-A-01", "F-B-B-01", "F-S-SELF-01"} {
		if !strings.Contains(body, want) {
			t.Fatalf("订阅缺少 %q：\n%s", want, body)
		}
	}
}

// TestAPISourcePipelineFields 机场新增/编辑可保存处理管线；自建节点集合管线可读写。
func TestAPISourcePipelineFields(t *testing.T) {
	ts, _, _ := profileServer(t)
	st, body := doJSON(t, ts, http.MethodPost, "/api/airports", map[string]any{
		"name": "机场P", "subscription_url": "https://p.example/sub", "enabled": true, "pipeline": "pre",
	})
	if st != http.StatusCreated || !strings.Contains(body, `"pipeline":"pre"`) {
		t.Fatalf("新增机场未保存处理管线: status=%d body=%s", st, body)
	}
	var created struct {
		Airport struct {
			ID       int64  `json:"id"`
			Pipeline string `json:"pipeline"`
		} `json:"airport"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("unmarshal airport: %v", err)
	}
	st, body = doJSON(t, ts, http.MethodPut, fmt.Sprintf("/api/airports/%d", created.Airport.ID), map[string]any{
		"name": "机场P", "subscription_url": "https://p.example/sub", "enabled": true, "pipeline": "post",
	})
	if st != http.StatusOK || !strings.Contains(body, `"pipeline":"post"`) {
		t.Fatalf("编辑机场未保存处理管线: status=%d body=%s", st, body)
	}

	st, body = doJSON(t, ts, http.MethodPut, "/api/self-nodes/pipeline", map[string]any{"pipeline": "selfpre"})
	if st != http.StatusOK || !strings.Contains(body, `"pipeline":"selfpre"`) {
		t.Fatalf("设置自建节点管线: status=%d body=%s", st, body)
	}
	st, body = doJSON(t, ts, http.MethodGet, "/api/self-nodes/pipeline", nil)
	if st != http.StatusOK || !strings.Contains(body, `"pipeline":"selfpre"`) {
		t.Fatalf("读取自建节点管线: status=%d body=%s", st, body)
	}
}
