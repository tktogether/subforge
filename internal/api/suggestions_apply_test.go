package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/tktogether/subforge/internal/rulebot"
	"testing"
)

func TestSuggestions_Apply_NoLoop501(t *testing.T) {
	SetSuggestionLoop(nil)
	t.Cleanup(func() { SetSuggestionLoop(nil) })
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/suggestions/apply", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIApplySuggestions(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("无 loop apply 状态码 = %d，应为 501", rr.Code)
	}
}

func TestSuggestions_Apply_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/suggestions/apply", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIApplySuggestions(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET apply 状态码 = %d，应为 405", rr.Code)
	}
}

func TestSuggestions_Apply_Success(t *testing.T) {
	q := rulebot.NewPendingQueue()
	loop := rulebot.NewLoop(rulebot.NewDetector(), q)
	q.Enqueue(&rulebot.Suggestion{Rule: "DOMAIN,example.com,DIRECT"})
	if err := q.Confirm(0); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	SetSuggestionQueue(q)
	SetSuggestionLoop(loop)
	t.Cleanup(func() {
		SetSuggestionQueue(nil)
		SetSuggestionLoop(nil)
	})
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/suggestions/apply", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIApplySuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("apply 状态码 = %d，body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Applied int `json:"applied"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Applied != 1 {
		t.Fatalf("applied = %d，应为 1", resp.Applied)
	}
}

func TestSuggestions_Apply_Error502AndRetry(t *testing.T) {
	q := rulebot.NewPendingQueue()
	loop := rulebot.NewLoop(rulebot.NewDetector(), q)
	q.Enqueue(&rulebot.Suggestion{Rule: "DOMAIN,retry.com,DIRECT"})
	if err := q.Confirm(0); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	SetSuggestionQueue(q)
	SetSuggestionLoop(loop)
	t.Cleanup(func() {
		SetSuggestionQueue(nil)
		SetSuggestionLoop(nil)
	})

	srv1, mgr1 := importTestServer(t)
	_ = mgr1.Close()
	req1 := httptest.NewRequest(http.MethodPost, "/api/suggestions/apply", nil)
	rr1 := httptest.NewRecorder()
	srv1.handleAPIApplySuggestions(rr1, req1)
	if rr1.Code != http.StatusBadGateway {
		t.Fatalf("首次 apply 状态码 = %d，应为 502，body=%s", rr1.Code, rr1.Body.String())
	}
	if len(q.Confirmed()) != 1 {
		t.Fatalf("报错后 Confirmed 应保留，got %d", len(q.Confirmed()))
	}

	srv2, _ := importTestServer(t)
	req2 := httptest.NewRequest(http.MethodPost, "/api/suggestions/apply", nil)
	rr2 := httptest.NewRecorder()
	srv2.handleAPIApplySuggestions(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("重试 apply 状态码 = %d，body=%s", rr2.Code, rr2.Body.String())
	}
	var resp struct {
		Applied int `json:"applied"`
	}
	_ = json.Unmarshal(rr2.Body.Bytes(), &resp)
	if resp.Applied != 1 {
		t.Fatalf("重试 applied = %d，应为 1", resp.Applied)
	}
}

func TestSuggestions_Apply_RoutePriority(t *testing.T) {
	q := rulebot.NewPendingQueue()
	loop := rulebot.NewLoop(rulebot.NewDetector(), q)
	q.Enqueue(&rulebot.Suggestion{Rule: "DOMAIN,route.com,DIRECT"})
	if err := q.Confirm(0); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	SetSuggestionQueue(q)
	SetSuggestionLoop(loop)
	t.Cleanup(func() {
		SetSuggestionQueue(nil)
		SetSuggestionLoop(nil)
	})
	srv, _ := importTestServer(t)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/suggestions/apply", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("POST /api/suggestions/apply 命中了 {index} 分支（404），路由优先级错误")
	}
	t.Logf("POST /api/suggestions/apply -> %d", resp.StatusCode)
}
