package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/store"
)

func TestAPIPipelineTestRun(t *testing.T) {
	ts, _, _ := profileServer(t)
	body := `{"name":"试运行","steps":[{"op":"op.rename","args":{"pattern":"^","repl":"T-"}}],"nodes":[{"name":"HK-01","type":"ss","server":"hk.example.com","port":8388}]}`
	st, resp := doJSON(t, ts, http.MethodPost, "/api/pipelines/test", json.RawMessage(body))
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, resp)
	}
	var got struct {
		BeforeCount int `json:"before_count"`
		AfterCount  int `json:"after_count"`
		After       []struct {
			Name string `json:"name"`
		} `json:"after"`
	}
	if err := json.Unmarshal([]byte(resp), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, resp)
	}
	if got.BeforeCount != 1 || got.AfterCount != 1 || len(got.After) != 1 || got.After[0].Name != "T-HK-01" {
		t.Fatalf("试运行结果不符: %+v body=%s", got, resp)
	}
}

func TestAPIPipelineTestRunBadNodes(t *testing.T) {
	ts, _, _ := profileServer(t)
	st, resp := doJSON(t, ts, http.MethodPost, "/api/pipelines/test", map[string]any{
		"steps": []any{}, "nodes": "not-array",
	})
	if st != http.StatusBadRequest || !strings.Contains(resp, "节点") {
		t.Fatalf("非法节点应 400: status=%d body=%s", st, resp)
	}
}

func TestAPIAirportPreviewProcess(t *testing.T) {
	ts, _, mgr := profileServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: HK-01, type: ss, server: hk.example.com, port: 8388, cipher: aes-256-gcm, password: pw}\n"))
	}))
	t.Cleanup(up.Close)
	a, err := mgr.AddAirport(t.Context(), "机场预览", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if _, err := mgr.CreatePipeline(t.Context(), "preview-rename", "", `[{"op":"op.rename","args":{"pattern":"^","repl":"P-"}}]`, true); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if err := mgr.SetAirportPipeline(t.Context(), a.ID, "preview-rename"); err != nil {
		t.Fatalf("SetAirportPipeline: %v", err)
	}
	st, resp := doJSON(t, ts, http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, resp)
	}
	var got struct {
		BeforeCount int    `json:"before_count"`
		AfterCount  int    `json:"after_count"`
		Pipeline    string `json:"pipeline"`
		After       []struct {
			Name string `json:"name"`
		} `json:"after"`
	}
	if err := json.Unmarshal([]byte(resp), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, resp)
	}
	if got.BeforeCount != 1 || got.AfterCount != 1 || got.Pipeline != "preview-rename" {
		t.Fatalf("机场预览计数不符: %+v", got)
	}
	if len(got.After) != 1 || got.After[0].Name != "P-HK-01" {
		t.Fatalf("机场预览节点不符: %+v", got)
	}
}

func TestAPISelfNodesPreviewProcess(t *testing.T) {
	ts, _, mgr := profileServer(t)
	if _, err := mgr.AddSelfNode(t.Context(), "trojan://pass@127.0.0.1:30001?allowInsecure=1#SELF-01", "SELF-01", true, 1); err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	if _, err := mgr.CreatePipeline(t.Context(), "self-preview", "", `[{"op":"op.rename","args":{"pattern":"^","repl":"S-"}}]`, true); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if err := mgr.SetSelfSourcePipeline(t.Context(), "self-preview"); err != nil {
		t.Fatalf("SetSelfSourcePipeline: %v", err)
	}
	st, resp := doJSON(t, ts, http.MethodPost, "/api/self-nodes/preview-process", map[string]any{})
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, resp)
	}
	if !strings.Contains(resp, `"name":"S-SELF-01"`) || !strings.Contains(resp, `"pipeline":"self-preview"`) {
		t.Fatalf("自建节点预览不符: %s", resp)
	}
}

func TestAPIPipelineTestRunDefaultNodes(t *testing.T) {
	ts, _, _ := profileServer(t)
	st, resp := doJSON(t, ts, http.MethodPost, "/api/pipelines/test", map[string]any{
		"name": "默认节点", "steps": []any{},
	})
	if st != http.StatusOK {
		t.Fatalf("status=%d body=%s", st, resp)
	}
	if !strings.Contains(resp, `"before_count":3`) {
		t.Fatalf("默认节点数不符: %s", resp)
	}
}

func TestAPIPreviewErrorBranches(t *testing.T) {
	ts, srv, mgr := profileServer(t)
	ctx := t.Context()

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: HK-01, type: ss, server: hk.example.com, port: 8388, cipher: aes-256-gcm, password: pw}\n"))
	}))
	t.Cleanup(up.Close)
	a, err := mgr.AddAirport(ctx, "预览机场", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}

	// handler 自身的 method 校验（路由只挂了 POST，直接调用才能覆盖）。
	for name, fn := range map[string]http.HandlerFunc{
		"pipeline": srv.handleAPIPipelineTest,
		"airport":  srv.handleAPIAirportPreviewProcess,
		"self":     srv.handleAPISelfNodesPreviewProcess,
	} {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s method check = %d", name, rec.Code)
		}
	}

	st, _ := doJSON(t, ts, http.MethodPost, "/api/pipelines/test", json.RawMessage(`{"steps":"bad"}`))
	if st != http.StatusBadRequest {
		t.Fatalf("bad pipeline body = %d", st)
	}
	st, _ = doJSON(t, ts, http.MethodPost, "/api/pipelines/test", map[string]any{
		"name": "bad", "steps": []map[string]any{{"op": "op.unknown"}},
	})
	if st != http.StatusBadRequest {
		t.Fatalf("bad pipeline step = %d", st)
	}
	st, _ = doJSON(t, ts, http.MethodPost, "/api/airports/999999/preview-process", map[string]any{})
	if st != http.StatusNotFound {
		t.Fatalf("airport not found = %d", st)
	}
	st, _ = doJSON(t, ts, http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), map[string]any{"pipeline": "missing-pipeline"})
	if st != http.StatusBadRequest {
		t.Fatalf("missing source pipeline = %d", st)
	}
	st, _ = doJSON(t, ts, http.MethodPost, "/api/self-nodes/preview-process", map[string]any{})
	if st != http.StatusBadRequest {
		t.Fatalf("self without nodes = %d", st)
	}
	st, resp := doJSON(t, ts, http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), map[string]any{"limit": 1000})
	if st != http.StatusOK || !strings.Contains(resp, `"after_count":1`) {
		t.Fatalf("limit clamp preview = %d %s", st, resp)
	}
}

func TestAPIPreviewBadUpstream(t *testing.T) {
	ts, _, mgr := profileServer(t)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not a subscription"))
	}))
	t.Cleanup(bad.Close)
	a, err := mgr.AddAirport(t.Context(), "坏上游", bad.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	st, _ := doJSON(t, ts, http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), map[string]any{})
	if st != http.StatusBadGateway {
		t.Fatalf("bad upstream = %d", st)
	}
}

func TestAPIPreviewNilEngineAndEmptyNodes(t *testing.T) {
	ts, srv, _ := profileServer(t)
	srv.engine = nil
	st, _ := doJSON(t, ts, http.MethodPost, "/api/pipelines/test", map[string]any{"steps": []any{}})
	if st != http.StatusServiceUnavailable {
		t.Fatalf("pipeline without engine = %d", st)
	}
	st, _ = doJSON(t, ts, http.MethodPost, "/api/airports/1/preview-process", map[string]any{})
	if st != http.StatusServiceUnavailable {
		t.Fatalf("airport without engine = %d", st)
	}

	full, _, mgr := profileServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: HK-01, type: ss, server: hk.example.com, port: 8388, cipher: aes-256-gcm, password: pw}\n"))
	}))
	t.Cleanup(up.Close)
	a, err := mgr.AddAirport(t.Context(), "空节点预览", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	st, _ = doJSON(t, full, http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), map[string]any{"limit": -1})
	if st != http.StatusOK {
		t.Fatalf("negative limit = %d", st)
	}
	st, _ = doJSON(t, full, http.MethodPost, "/api/pipelines/test", map[string]any{"steps": []any{}, "nodes": []any{}})
	if st != http.StatusBadRequest {
		t.Fatalf("empty nodes = %d", st)
	}
}

func TestAPIPreviewDecodeAndLimitBranches(t *testing.T) {
	ts, srv, mgr := profileServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies:\n  - {name: HK-01, type: ss, server: hk.example.com, port: 8388, cipher: aes-256-gcm, password: pw}\n"))
	}))
	t.Cleanup(up.Close)
	a, err := mgr.AddAirport(t.Context(), "分支机场", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	// 非法 JSON 走 decodeBody 失败分支。
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), strings.NewReader("{"))
	req.SetPathValue("id", fmt.Sprintf("%d", a.ID))
	rec := httptest.NewRecorder()
	srv.handleAPIAirportPreviewProcess(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("airport bad body = %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/self-nodes/preview-process", strings.NewReader("{"))
	rec = httptest.NewRecorder()
	srv.handleAPISelfNodesPreviewProcess(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self bad body = %d", rec.Code)
	}
	st, _ := doJSON(t, ts, http.MethodPost, fmt.Sprintf("/api/airports/%d/preview-process", a.ID), map[string]any{"limit": 10})
	if st != http.StatusOK {
		t.Fatalf("positive small limit = %d", st)
	}
}

func TestClampPreviewLimit(t *testing.T) {
	cases := map[int]int{-1: 20, 0: 20, 10: 10, 100: 100, 101: 100, 1000: 100}
	for in, want := range cases {
		if got := clampPreviewLimit(in); got != want {
			t.Fatalf("clampPreviewLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestAPIPipelineTestRuntimeError(t *testing.T) {
	ts, _, mgr := profileServer(t)
	if _, err := mgr.DB().CreateScript(t.Context(), store.Script{Name: "boom", Kind: "operate", Code: "throw new Error('boom')", Enabled: true}); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	st, resp := doJSON(t, ts, http.MethodPost, "/api/pipelines/test", map[string]any{
		"name": "boom", "steps": []map[string]any{{"op": "op.script", "args": map[string]string{"ref": "boom"}}},
	})
	if st != http.StatusBadRequest || !strings.Contains(resp, "boom") {
		t.Fatalf("runtime error = %d %s", st, resp)
	}
}

func TestPreviewPipelineSpecDBError(t *testing.T) {
	_, srv, mgr := profileServer(t)
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := srv.previewPipelineSpec(t.Context(), "any"); err == nil {
		t.Fatal("关闭 DB 后应返回错误")
	}
}
