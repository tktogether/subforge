package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
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

// seedRuleProviderForTest 造一条 rule_providers 行并返回其 id（走 store 层已测 API）。
func seedRuleProviderForTest(t *testing.T, mgr *subforge.Manager, name string, enabled bool) int64 {
	t.Helper()
	if err := mgr.DB().UpsertRuleProvider(t.Context(), store.RuleProvider{
		Name: name, RawJSON: "{}", Source: "airport:HK", Enabled: enabled, SortOrder: 1,
	}); err != nil {
		t.Fatalf("seed rule provider: %v", err)
	}
	list, err := mgr.DB().ListRuleProviders(t.Context())
	if err != nil || len(list) != 1 {
		t.Fatalf("seed 后 ListRuleProviders: err=%v len=%d", err, len(list))
	}
	return list[0].ID
}

// TestRuleProviderSetEnabled_MethodNotAllowed 覆盖非 PUT 的 405。
func TestRuleProviderSetEnabled_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/rule-providers/1/enabled", strings.NewReader("{\"enabled\":true}"))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviderSetEnabled(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST 状态码 = %d，应为 405；body=%s", rr.Code, rr.Body.String())
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "仅支持 PUT" {
		t.Fatalf("error = %q", e.Error)
	}
}

// TestRuleProviderSetEnabled_BadBody400 覆盖 body 非 JSON 的 400。
func TestRuleProviderSetEnabled_BadBody400(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/rule-providers/1/enabled", strings.NewReader("not-json"))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviderSetEnabled(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("非法 body 状态码 = %d，应为 400；body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "enabled") {
		t.Fatalf("body 应含 enabled 提示，got=%s", rr.Body.String())
	}
}

// TestRuleProviderSetEnabled_BadID400 覆盖非法 id 的 400（走 s.pathID）。
func TestRuleProviderSetEnabled_BadID400(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/rule-providers/abc/enabled", strings.NewReader("{\"enabled\":true}"))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviderSetEnabled(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("非法 id 状态码 = %d，应为 400；body=%s", rr.Code, rr.Body.String())
	}
}

// TestRuleProviderSetEnabled_NotFound404 覆盖不存在 id 的 404。
func TestRuleProviderSetEnabled_NotFound404(t *testing.T) {
	srv, mgr := importTestServer(t)
	seedRuleProviderForTest(t, mgr, "rp1", true)
	req := httptest.NewRequest(http.MethodPut, "/api/rule-providers/9999/enabled", strings.NewReader("{\"enabled\":true}"))
	req.SetPathValue("id", "9999")
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviderSetEnabled(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("不存在 id 状态码 = %d，应为 404；body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "不存在") {
		t.Fatalf("body 应含 不存在，got=%s", rr.Body.String())
	}
}

// TestRuleProviderSetEnabled_Success 覆盖成功停用并影响 GET ?enabled=1/0 过滤。
func TestRuleProviderSetEnabled_Success(t *testing.T) {
	srv, mgr := importTestServer(t)
	id := seedRuleProviderForTest(t, mgr, "rp1", true)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rule-providers/%d/enabled", id), strings.NewReader("{\"enabled\":false}"))
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIRuleProviderSetEnabled(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("成功状态码 = %d，应为 200；body=%s", rr.Code, rr.Body.String())
	}
	var okResp struct {
		OK bool `json:"ok"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &okResp)
	if !okResp.OK {
		t.Fatalf("body 应为 {\"ok\":true}，got=%s", rr.Body.String())
	}

	reqEnabled := httptest.NewRequest(http.MethodGet, "/api/rule-providers?enabled=1", nil)
	rrEnabled := httptest.NewRecorder()
	srv.handleAPIRuleProviders(rrEnabled, reqEnabled)
	if rrEnabled.Code != http.StatusOK {
		t.Fatalf("GET enabled=1 状态码 = %d", rrEnabled.Code)
	}
	var resp struct {
		RuleProviders []store.RuleProvider `json:"rule_providers"`
	}
	_ = json.Unmarshal(rrEnabled.Body.Bytes(), &resp)
	if len(resp.RuleProviders) != 0 {
		t.Fatalf("停用后 enabled=1 列表应为空，got=%d 条: %s", len(resp.RuleProviders), rrEnabled.Body.String())
	}

	reqDisabled := httptest.NewRequest(http.MethodGet, "/api/rule-providers?enabled=0", nil)
	rrDisabled := httptest.NewRecorder()
	srv.handleAPIRuleProviders(rrDisabled, reqDisabled)
	var resp0 struct {
		RuleProviders []store.RuleProvider `json:"rule_providers"`
	}
	_ = json.Unmarshal(rrDisabled.Body.Bytes(), &resp0)
	if len(resp0.RuleProviders) != 1 || resp0.RuleProviders[0].Enabled {
		t.Fatalf("enabled=0 应含 1 条且 Enabled=false，got=%s", rrDisabled.Body.String())
	}
}

// TestRuleProviderSetEnabled_RouteReachable 走 Routes() 验证 chi 路由 {id} 与 PathValue 接线（handler 直调测不到路由层）。
func TestRuleProviderSetEnabled_RouteReachable(t *testing.T) {
	srv, mgr := importTestServer(t)
	id := seedRuleProviderForTest(t, mgr, "rp1", true)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rule-providers/%d/enabled", id), strings.NewReader("{\"enabled\":false}"))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Routes() PUT 状态码 = %d，应为 200；body=%s", rec.Code, rec.Body.String())
	}

	req404 := httptest.NewRequest(http.MethodPut, "/api/rule-providers/9999/enabled", strings.NewReader("{\"enabled\":true}"))
	rec404 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("Routes() 不存在 id 状态码 = %d，应为 404；body=%s", rec404.Code, rec404.Body.String())
	}
}
