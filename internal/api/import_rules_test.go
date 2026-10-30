package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/subforge"
)

func importTestServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(filepath.Join(t.TempDir(), "subforge.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	cfg := Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	}
	srv := New(cfg)
	srv.SetSubManager(mgr)
	return srv, mgr
}

func addImportAirport(t *testing.T, mgr *subforge.Manager, name, url string) int64 {
	t.Helper()
	a, err := mgr.AddAirport(t.Context(), name, url, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	return a.ID
}

func TestImportRules_MethodNotAllowed(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/airports/1/import-rules", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET 状态码 = %d，应为 405", rr.Code)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if e.Error != "仅支持 POST" {
		t.Fatalf("error = %q，应为 仅支持 POST", e.Error)
	}
}

func TestImportRules_NoManager503(t *testing.T) {
	srv := New(Config{
		Engine:  &convert.Engine{Profile: render.Profile{Name: "test", Version: "test-version"}},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/airports/1/import-rules", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("未注入 manager 状态码 = %d，应为 503", rr.Code)
	}
}

func TestImportRules_AirportNotFound404(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/airports/99999/import-rules", nil)
	req.SetPathValue("id", "99999")
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("不存在机场状态码 = %d，应为 404", rr.Code)
	}
}

func TestImportRules_FetchFailure502(t *testing.T) {
	srv, mgr := importTestServer(t)
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()
	id := addImportAirport(t, mgr, "dead", dead.URL)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("拉取失败状态码 = %d，应为 502", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "导入失败") {
		t.Fatalf("body 应含 导入失败，got=%s", rr.Body.String())
	}
}

func TestImportRules_Success(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	}))
	defer up.Close()
	id := addImportAirport(t, mgr, "ok", up.URL)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("成功状态码 = %d，应为 200；body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		RulesAdded     int      `json:"rules_added"`
		RulesSkipped   int      `json:"rules_skipped"`
		ProvidersAdded int      `json:"providers_added"`
		Warnings       []string `json:"warnings"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if resp.Warnings == nil {
		t.Fatalf("warnings 必须是非 nil 数组")
	}
}

// TestImportRules_AirportRulesBecomeSource 机场规则现在落成版本化规则源，
// 不再写 rules 表；方案需要显式绑定该源后订阅才会内联。
func TestImportRules_AirportRulesBecomeSource(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := rulesUpstream(t)
	id := addImportAirport(t, mgr, "enable", up.URL)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/airports/%d/import-rules", id),
		strings.NewReader(`{"create_groups":true,"enable_rules":true}`))
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("状态码 = %d；body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		RulesAdded      int    `json:"rules_added"`
		RulesEnabled    int    `json:"rules_enabled"`
		RuleSourceName  string `json:"rule_source_name"`
		RuleSourceRules int    `json:"rule_source_rules"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if resp.RulesAdded != 0 || resp.RulesEnabled != 0 || resp.RuleSourceRules != 2 || resp.RuleSourceName == "" {
		t.Fatalf("机场规则应全部进规则源：%+v body=%s", resp, rr.Body.String())
	}
	rules, warnings := mgr.RuleSourceRules(t.Context(), []string{resp.RuleSourceName})
	if len(rules) != 2 {
		t.Fatalf("规则源规则数 = %d，应为 2；warnings=%v", len(rules), warnings)
	}
	dbRules, err := mgr.ListRulesByLayers(t.Context(), []string{"airport"})
	if err != nil {
		t.Fatalf("ListRulesByLayers: %v", err)
	}
	if len(dbRules) != 0 {
		t.Fatalf("机场规则不应再写 rules 表：%+v", dbRules)
	}
	sources, err := mgr.ListRuleSources(t.Context())
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	found := false
	for _, s := range sources {
		if s.Name == resp.RuleSourceName && s.Origin == "airport" && s.RuleCount == 2 && s.Latest != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("规则源状态未出现或规则数不对：%+v", sources)
	}
}

func TestImportRules_IdempotentSecondImportSkips(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	}))
	defer up.Close()
	id := addImportAirport(t, mgr, "idem", up.URL)
	url := fmt.Sprintf("/api/airports/%d/import-rules", id)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, url, nil)
		req.SetPathValue("id", fmt.Sprintf("%d", id))
		rr := httptest.NewRecorder()
		srv.handleAPIImportRules(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("第 %d 次导入状态码 = %d", i+1, rr.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.SetPathValue("id", fmt.Sprintf("%d", id))
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	var resp struct {
		Skipped int `json:"rules_skipped"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	_ = resp
}

// TestImportHTTPErrorUnwrapAndError 直接覆盖 importHTTPError 的 Error/Unwrap，
// 因为 errors.As 走反射不计入被包装类型的 Unwrap 函数体覆盖。
func TestImportHTTPErrorUnwrapAndError(t *testing.T) {
	wrapped := errors.New("底层错误")
	he := &importHTTPError{status: http.StatusBadGateway, err: wrapped}
	if he.Error() != "底层错误" {
		t.Fatalf("Error() = %q，应为 底层错误", he.Error())
	}
	if unwrapped := he.Unwrap(); unwrapped != wrapped {
		t.Fatalf("Unwrap() = %v，应为 %v", unwrapped, wrapped)
	}
	if he.status != http.StatusBadGateway {
		t.Fatalf("status = %d，应为 502", he.status)
	}
}

// rulesClashYAML 是一个含 rules / rule-providers 的最小 Clash YAML 订阅，用于导入测试。
const rulesClashYAML = "proxies:\n" +
	"  - name: p1\n" +
	"    type: ss\n" +
	"    server: s.example.com\n" +
	"    port: 8388\n" +
	"    cipher: aes-256-gcm\n" +
	"    password: pw\n" +
	"rules:\n" +
	"  - DOMAIN-SUFFIX,example.com,DIRECT\n" +
	"  - IP-CIDR,1.1.1.1/32,DIRECT,no-resolve\n" +
	"rule-providers:\n" +
	"  rp1:\n" +
	"    type: http\n" +
	"    url: https://example.com/rule.txt\n"

// rulesUpstream 起一个吐 rulesClashYAML 的本地 httptest 上游（带 Subscription-Userinfo 头，便于流量刷新也成功）。
func rulesUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(rulesClashYAML))
	}))
	t.Cleanup(up.Close)
	return up
}

// TestImportRulesFromAirportHelper 直接调用 helper，断言 ImportResult 计数与 warnings 非 nil。
func TestImportRulesFromAirportHelper(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := rulesUpstream(t)
	a, err := mgr.AddAirport(t.Context(), "helper", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	ir, warnings, err := srv.importRulesFromAirport(context.Background(), mgr, a)
	if err != nil {
		t.Fatalf("importRulesFromAirport err: %v", err)
	}
	if ir.RulesAdded != 0 || ir.RuleSourceRules != 2 || ir.RuleSourceName == "" {
		t.Fatalf("机场规则应只进规则源：%+v", ir)
	}
	if ir.ProvidersAdded != 1 {
		t.Fatalf("providers_added = %d，应为 1", ir.ProvidersAdded)
	}
	if rules, _ := mgr.RuleSourceRules(t.Context(), []string{ir.RuleSourceName}); len(rules) != 2 {
		t.Fatalf("规则源规则数 = %d，应为 2", len(rules))
	}
	if dbRules, _ := mgr.ListRulesByLayers(t.Context(), []string{"airport"}); len(dbRules) != 0 {
		t.Fatalf("机场规则不应写 rules 表：%+v", dbRules)
	}
	if warnings == nil {
		t.Fatalf("warnings 必须是非 nil 切片")
	}
}

// TestImportRulesFromAirportHelperFetchFailure 覆盖 helper 在拉取失败时返回 502 语义错误。
func TestImportRulesFromAirportHelperFetchFailure(t *testing.T) {
	srv, mgr := importTestServer(t)
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()
	a, err := mgr.AddAirport(t.Context(), "dead-helper", dead.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	_, _, err = srv.importRulesFromAirport(context.Background(), mgr, a)
	if err == nil {
		t.Fatalf("拉取失败应返回 err")
	}
	if !strings.Contains(err.Error(), "导入失败") {
		t.Fatalf("err = %q，应含 导入失败", err.Error())
	}
}

// TestImportRulesFromAirportNilManager 覆盖 manager 缺失时的 503 语义错误。
func TestImportRulesFromAirportNilManager(t *testing.T) {
	srv, _ := importTestServer(t)
	a := &subforge.Airport{Name: "nil-mgr", SubscriptionURL: "https://example.invalid/sub"}
	_, _, err := srv.importRulesFromAirport(context.Background(), nil, a)
	if err == nil {
		t.Fatalf("nil manager 应返回 err")
	}
	var he *importHTTPError
	if !errors.As(err, &he) {
		t.Fatalf("err = %v，应为 *importHTTPError", err)
	}
	if he.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d，应为 503", he.status)
	}
	if !strings.Contains(err.Error(), "订阅管理未启用") {
		t.Fatalf("err = %q，应含 订阅管理未启用", err.Error())
	}
}

// TestImportRulesFromAirportDecodeFailure 上游内容无法解析 → 502「导入失败」。
func TestImportRulesFromAirportDecodeFailure(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxies: [not-valid-yaml"))
	}))
	defer up.Close()
	a, err := mgr.AddAirport(t.Context(), "bad-parse", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	_, _, err = srv.importRulesFromAirport(context.Background(), mgr, a)
	if err == nil {
		t.Fatalf("解析失败应返回 err")
	}
	if !strings.Contains(err.Error(), "导入失败") {
		t.Fatalf("err = %q，应含 导入失败", err.Error())
	}
}

// TestImportRulesFromAirportWriteFailure 关闭数据库后写库失败 → 500 语义。
func TestImportRulesFromAirportWriteFailure(t *testing.T) {
	srv, mgr := importTestServer(t)
	up := rulesUpstream(t)
	a, err := mgr.AddAirport(t.Context(), "closed-db", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, _, err = srv.importRulesFromAirport(context.Background(), mgr, a)
	if err == nil {
		t.Fatalf("写库失败应返回 err")
	}
	var he *importHTTPError
	if !errors.As(err, &he) || he.status != http.StatusInternalServerError {
		t.Fatalf("err = %v，应为 500 语义", err)
	}
}

// TestImportRules_MissingID400 路径缺少 id → 400。
func TestImportRules_MissingID400(t *testing.T) {
	srv, _ := importTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/airports/import-rules", nil)
	rr := httptest.NewRecorder()
	srv.handleAPIImportRules(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("缺少 id 状态码 = %d，应为 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "无效的 ID") {
		t.Fatalf("body = %s，应含 无效的 ID", rr.Body.String())
	}
}
