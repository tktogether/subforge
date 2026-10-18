package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
)

func TestRuleProviders_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/rule-providers", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviders(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST 状态码 = %d，应为 405", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "仅支持 GET" {
		t.Fatalf("error = %q", e.Error)
	}
}

func TestRuleProviders_NoManager503(t *testing.T) {
	srv := New(Config{
		Engine:  &convert.Engine{Profile: render.Profile{Name: "test", Version: "test-version"}},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/rule-providers", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviders(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("未注入 manager 状态码 = %d，应为 503", rr.Code)
	}
}

func TestRuleProviders_EmptyArrayNotNull(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/rule-providers", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviders(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", rr.Code)
	}
	var resp struct {
		RuleProviders []interface{} `json:"rule_providers"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.RuleProviders == nil {
		t.Fatalf("rule_providers 必须是非 nil 数组")
	}
}

func TestRuleProviders_EnabledFilter(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/rule-providers?enabled=1", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviders(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", rr.Code)
	}
	var resp struct {
		RuleProviders []interface{} `json:"rule_providers"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.RuleProviders == nil {
		t.Fatalf("rule_providers 必须是非 nil 数组")
	}
}
