package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/tktogether/subforge/internal/rulebot"
	"testing"
)

func TestSuggestions_GetEmptyArrayNotNull(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/suggestions", nil)
	rr := httptest.NewRecorder()
	srv.handleAPISuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", rr.Code)
	}
	var resp struct {
		Suggestions []interface{} `json:"suggestions"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Suggestions == nil {
		t.Fatalf("suggestions 必须是非 nil 数组")
	}
}

func TestSuggestions_Confirm_NoQueue501(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/suggestions/0/confirm", nil)
	req.SetPathValue("index", "0")
	rr := httptest.NewRecorder()
	srv.handleAPIConfirmSuggestion(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("未注入队列 confirm 状态码 = %d，应为 501", rr.Code)
	}
}

func TestSuggestions_Reject_NoQueue501(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/suggestions/0/reject", nil)
	req.SetPathValue("index", "0")
	rr := httptest.NewRecorder()
	srv.handleAPIRejectSuggestion(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("未注入队列 reject 状态码 = %d，应为 501", rr.Code)
	}
}

func TestSuggestions_Confirm_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/suggestions/0/confirm", nil)
	req.SetPathValue("index", "0")
	rr := httptest.NewRecorder()
	srv.handleAPIConfirmSuggestion(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET confirm 状态码 = %d，应为 405", rr.Code)
	}
}

func TestSuggestions_Reject_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/suggestions/0/reject", nil)
	req.SetPathValue("index", "0")
	rr := httptest.NewRecorder()
	srv.handleAPIRejectSuggestion(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET reject 状态码 = %d，应为 405", rr.Code)
	}
}

func TestSuggestions_Confirm_WithQueue(t *testing.T) {
	SetSuggestionQueue(rulebot.NewPendingQueue())
	t.Cleanup(func() { SetSuggestionQueue(nil) })
	srv, _ := importTestServer(t)
	q := rulebot.NewPendingQueue()
	SetSuggestionQueue(q)
	q.Enqueue(&rulebot.Suggestion{Rule: "DOMAIN,example.com,DIRECT"})
	req := httptest.NewRequest(http.MethodPost, "/api/suggestions/0/confirm", nil)
	req.SetPathValue("index", "0")
	rr := httptest.NewRecorder()
	srv.handleAPIConfirmSuggestion(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("confirm 状态码 = %d，body=%s", rr.Code, rr.Body.String())
	}
	if len(q.Pending()) != 0 {
		t.Fatalf("确认后待确认队列应为空")
	}

	// 越界索引 → 404
	req2 := httptest.NewRequest(http.MethodPost, "/api/suggestions/5/reject", nil)
	req2.SetPathValue("index", "5")
	rr2 := httptest.NewRecorder()
	srv.handleAPIRejectSuggestion(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("越界 reject 状态码 = %d，应为 404", rr2.Code)
	}
}

func TestSuggestions_Get_WithQueue(t *testing.T) {
	q := rulebot.NewPendingQueue()
	SetSuggestionQueue(q)
	t.Cleanup(func() { SetSuggestionQueue(nil) })
	q.Enqueue(&rulebot.Suggestion{Rule: "DOMAIN,example.com,DIRECT"})
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/suggestions", nil)
	rr := httptest.NewRecorder()
	srv.handleAPISuggestions(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", rr.Code)
	}
	var resp struct {
		Suggestions []interface{} `json:"suggestions"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Suggestions) != 1 {
		t.Fatalf("suggestions 长度 = %d，应为 1", len(resp.Suggestions))
	}
}
