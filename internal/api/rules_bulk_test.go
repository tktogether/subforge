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

// TestRulesBulk_DeleteByIds 覆盖批量删除：按 ids 删除并返回 deleted 计数。
func TestRulesBulk_DeleteByIds(t *testing.T) {
	srv, mgr := importTestServer(t)
	del1, err := mgr.AddRule(t.Context(), "DOMAIN", "del1.example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	del2, err := mgr.AddRule(t.Context(), "DOMAIN", "del2.example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	keep, err := mgr.AddRule(t.Context(), "DOMAIN", "keep.example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	body := fmt.Sprintf("{\"ids\":[%d,%d],\"delete\":true}", del1.ID, del2.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["deleted"] != 2 {
		t.Fatalf("deleted = %d，应为 2", resp["deleted"])
	}
	if resp["updated"] != 0 {
		t.Fatalf("updated = %d，应为 0", resp["updated"])
	}
	if _, err := mgr.GetRule(t.Context(), del1.ID); err == nil {
		t.Fatalf("规则 %d 应已被删除", del1.ID)
	}
	if _, err := mgr.GetRule(t.Context(), del2.ID); err == nil {
		t.Fatalf("规则 %d 应已被删除", del2.ID)
	}
	if _, err := mgr.GetRule(t.Context(), keep.ID); err != nil {
		t.Fatalf("未选中的规则被误删: %v", err)
	}
}

// TestRulesBulk_DeleteEmptyIDs400 覆盖批量删除缺 ids：400。
func TestRulesBulk_DeleteEmptyIDs400(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader("{\"delete\":true}"))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d，应为 400", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "批量删除必须给出 ids" {
		t.Fatalf("error = %q", e.Error)
	}
}

// TestRulesBulk_DeleteWithSource400 覆盖批量删除 + source：400，避免整来源误删。
func TestRulesBulk_DeleteWithSource400(t *testing.T) {
	srv, _ := importTestServer(t)
	body := "{\"ids\":[1],\"source\":\"airport:x\",\"delete\":true}"
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d，应为 400", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "批量删除仅支持 ids" {
		t.Fatalf("error = %q", e.Error)
	}
}

// TestRulesBulk_DeleteNonexistent 覆盖 id 不存在：200 且 deleted=0，已有规则不受影响。
func TestRulesBulk_DeleteNonexistent(t *testing.T) {
	srv, mgr := importTestServer(t)
	keep, err := mgr.AddRule(t.Context(), "DOMAIN", "keep.example.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	body := "{\"ids\":[987654321],\"delete\":true}"
	req := httptest.NewRequest(http.MethodPost, "/api/rules/bulk", strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleAPIRulesBulk(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["deleted"] != 0 {
		t.Fatalf("deleted = %d，应为 0", resp["deleted"])
	}
	if _, err := mgr.GetRule(t.Context(), keep.ID); err != nil {
		t.Fatalf("已有规则被误删: %v", err)
	}
}
