package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
