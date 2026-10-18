package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
)

func TestRulesBulk_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/rules/bulk", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET 状态码 = %d，应为 405", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "仅支持 POST" {
		t.Fatalf("error = %q", e.Error)
	}
}

func TestRulesBulk_NoManager503(t *testing.T) {
	srv := New(Config{
		Engine:  &convert.Engine{Profile: render.Profile{Name: "test", Version: "test-version"}},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("未注入 manager 状态码 = %d，应为 503", rr.Code)
	}
}

func TestRulesBulk_MissingBoth400(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("两者都缺 状态码 = %d，应为 400", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "必须给出 ids 或 source" {
		t.Fatalf("error = %q", e.Error)
	}
}

func TestRulesBulk_BothGiven400(t *testing.T) {
	srv, _ := importTestServer(t)
	body := `{"ids":[1],"source":"airport:x","enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("两者都给 状态码 = %d，应为 400", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "ids 与 source 不能同时给出" {
		t.Fatalf("error = %q", e.Error)
	}
}

func TestRulesBulk_ByIds(t *testing.T) {
	srv, mgr := importTestServer(t)
	rule, err := mgr.AddRule(t.Context(), "DOMAIN", "example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	body := fmt.Sprintf(`{"ids":[%d],"enabled":false}`, rule.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Updated int `json:"updated"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Updated != 1 {
		t.Fatalf("updated = %d，应为 1", resp.Updated)
	}
	got, _ := mgr.GetRule(t.Context(), rule.ID)
	if got.Enabled {
		t.Fatalf("规则应被禁用")
	}
}

func TestRulesBulk_BySource(t *testing.T) {
	srv, mgr := importTestServer(t)
	rule, err := mgr.AddRule(t.Context(), "DOMAIN", "example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	// 设置 source
	_ = rule
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader(`{"source":"airport:x","enabled":false}`))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，body=%s", rr.Code, rr.Body.String())
	}
}
