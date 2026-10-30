package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/subforge"
)

func newTrafficTestServer(t *testing.T) (*httptest.Server, *subforge.Manager, *Server) {
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
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return ts, mgr, srv
}

func TestAPITrafficStrategyGetDefault(t *testing.T) {
	ts, _, _ := newTrafficTestServer(t)
	status, _, body := uiGet(t, ts, "/api/traffic/strategy")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body=%s", status, body)
	}
	var got struct {
		Strategy struct {
			Preset   string `json:"preset"`
			Upload   string `json:"upload"`
			Download string `json:"download"`
			Total    string `json:"total"`
			Expire   string `json:"expire"`
		} `json:"strategy"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Strategy.Preset != "sum" {
		t.Fatalf("preset = %q", got.Strategy.Preset)
	}
	if got.Strategy.Upload != "sum" || got.Strategy.Download != "sum" || got.Strategy.Total != "sum" {
		t.Fatalf("fields = %+v", got.Strategy)
	}
	if got.Strategy.Expire != "min" {
		t.Fatalf("expire = %q", got.Strategy.Expire)
	}
}

func TestAPITrafficStrategyPutRoundTrip(t *testing.T) {
	ts, _, _ := newTrafficTestServer(t)
	status, _, body := uiDoJSON(t, ts, http.MethodPut, "/api/traffic/strategy", `{"preset":"max"}`)
	if status != http.StatusOK {
		t.Fatalf("put1 status=%d body=%s", status, body)
	}
	var got struct {
		Strategy struct {
			Preset   string `json:"preset"`
			Upload   string `json:"upload"`
			Download string `json:"download"`
			Total    string `json:"total"`
			Expire   string `json:"expire"`
		} `json:"strategy"`
	}
	json.Unmarshal([]byte(body), &got)
	// 预设名必须展开成字段策略（Normalize 语义），否则 Aggregate 会按默认 sum 聚合。
	if got.Strategy.Preset != "max" || got.Strategy.Upload != "max" || got.Strategy.Download != "max" ||
		got.Strategy.Total != "max" || got.Strategy.Expire != "max" {
		t.Fatalf("put1 = %+v", got.Strategy)
	}
	status, _, body = uiGet(t, ts, "/api/traffic/strategy")
	if status != http.StatusOK {
		t.Fatalf("get status=%d", status)
	}
	json.Unmarshal([]byte(body), &got)
	if got.Strategy.Preset != "max" {
		t.Fatalf("persisted preset = %q", got.Strategy.Preset)
	}
	status, _, body = uiDoJSON(t, ts, http.MethodPut, "/api/traffic/strategy", `{"preset":"sum","upload":"min"}`)
	if status != http.StatusOK {
		t.Fatalf("put2 status=%d body=%s", status, body)
	}
	json.Unmarshal([]byte(body), &got)
	if got.Strategy.Upload != "min" || got.Strategy.Download != "sum" || got.Strategy.Total != "sum" || got.Strategy.Expire != "min" {
		t.Fatalf("put2 = %+v", got.Strategy)
	}
}

func TestAPITrafficStrategyBadBody(t *testing.T) {
	ts, _, _ := newTrafficTestServer(t)
	status, hdr, body := uiDoJSON(t, ts, http.MethodPut, "/api/traffic/strategy", "{")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", status, body)
	}
	uiAssertJSONError(t, "bad body", body, hdr)
}

func TestAPITrafficStrategyMethodNotAllowed(t *testing.T) {
	_, _, srv := newTrafficTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/traffic/strategy", nil)
	srv.handleAPITrafficStrategy(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rr.Code)
	}
	uiAssertJSONError(t, "method not allowed", rr.Body.String(), rr.Header())
	if !strings.Contains(rr.Body.String(), "仅支持 GET/PUT") {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestAPITrafficStrategyNoManager(t *testing.T) {
	ts := newTestServer(t, nil)
	status, hdr, body := uiGet(t, ts, "/api/traffic/strategy")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body=%s", status, body)
	}
	uiAssertJSONError(t, "no manager", body, hdr)
}

func TestAPITrafficRefreshOK(t *testing.T) {
	ts, mgr, _ := newTrafficTestServer(t)
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	defer up.Close()
	ctx := t.Context()
	if _, err := mgr.AddAirport(ctx, "机场A", up.URL, "", true); err != nil {
		t.Fatal(err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/traffic/refresh", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Results []struct {
			OK        bool `json:"ok"`
			NodeCount int  `json:"node_count"`
			Usage     struct {
				Upload   int64 `json:"upload"`
				Download int64 `json:"download"`
				Total    int64 `json:"total"`
				Expire   int64 `json:"expire"`
			} `json:"usage"`
		} `json:"results"`
		OK   int `json:"ok"`
		Fail int `json:"fail"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.OK != 1 || got.Fail != 0 || len(got.Results) != 1 {
		t.Fatalf("ok=%d fail=%d len=%d", got.OK, got.Fail, len(got.Results))
	}
	if !got.Results[0].OK || got.Results[0].NodeCount != 12 {
		t.Fatalf("result = %+v", got.Results[0])
	}
	if got.Results[0].Usage.Upload != 10 || got.Results[0].Usage.Download != 20 ||
		got.Results[0].Usage.Total != 100 || got.Results[0].Usage.Expire != 1700000000 {
		t.Fatalf("usage = %+v", got.Results[0].Usage)
	}
}

func TestAPITrafficRefreshUpstreamError(t *testing.T) {
	ts, mgr, _ := newTrafficTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer up.Close()
	ctx := t.Context()
	if _, err := mgr.AddAirport(ctx, "机场A", up.URL, "", true); err != nil {
		t.Fatal(err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/traffic/refresh", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Results []struct {
			OK  bool   `json:"ok"`
			Err string `json:"err"`
		} `json:"results"`
		OK   int `json:"ok"`
		Fail int `json:"fail"`
	}
	json.Unmarshal([]byte(body), &got)
	if got.OK != 0 || got.Fail != 1 || len(got.Results) != 1 {
		t.Fatalf("ok=%d fail=%d len=%d", got.OK, got.Fail, len(got.Results))
	}
	if got.Results[0].OK || got.Results[0].Err == "" {
		t.Fatalf("result = %+v", got.Results[0])
	}
}

func TestAPITrafficRefreshNoAirports(t *testing.T) {
	ts, _, _ := newTrafficTestServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/traffic/refresh", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if !strings.Contains(body, `"results":[]`) {
		t.Fatalf("body not empty array: %s", body)
	}
	var got struct {
		Results []interface{} `json:"results"`
		OK      int           `json:"ok"`
		Fail    int           `json:"fail"`
	}
	json.Unmarshal([]byte(body), &got)
	if got.Results == nil || len(got.Results) != 0 {
		t.Fatalf("results = %v", got.Results)
	}
	if got.OK != 0 || got.Fail != 0 {
		t.Fatalf("ok=%d fail=%d", got.OK, got.Fail)
	}
}

func TestAPITrafficRefreshMethodNotAllowed(t *testing.T) {
	_, _, srv := newTrafficTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/traffic/refresh", nil)
	srv.handleAPITrafficRefresh(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rr.Code)
	}
	uiAssertJSONError(t, "method not allowed", rr.Body.String(), rr.Header())
	if !strings.Contains(rr.Body.String(), "仅支持 POST") {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestAPITrafficRefreshNoManager(t *testing.T) {
	ts := newTestServer(t, nil)
	status, hdr, body := uiPostJSON(t, ts, "/api/traffic/refresh", "")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", status, body)
	}
	uiAssertJSONError(t, "no manager", body, hdr)
}

func setupAirportWithTraffic(t *testing.T) (*httptest.Server, *subforge.Manager, int64, *httptest.Server) {
	t.Helper()
	ts, mgr, _ := newTrafficTestServer(t)
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt"))
	if err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(up.Close)
	ctx := t.Context()
	a, err := mgr.AddAirport(ctx, "机场A", up.URL, "", true)
	if err != nil {
		t.Fatal(err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/traffic/refresh", "")
	if status != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", status, body)
	}
	return ts, mgr, a.ID, up
}

func TestAPIAirportsRefreshAll(t *testing.T) {
	ts, mgr, _, _ := setupAirportWithTraffic(t)
	_ = mgr
	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if !strings.Contains(body, "\"results\"") || !strings.Contains(body, "\"ok\"") || !strings.Contains(body, "\"fail\"") {
		t.Fatalf("body missing results/ok/fail: %s", body)
	}
}

func TestAPIAirportRefreshOneOK(t *testing.T) {
	ts, _, id, _ := setupAirportWithTraffic(t)
	path := "/api/airports/" + strconv.FormatInt(id, 10) + "/refresh"
	status, _, body := uiPostJSON(t, ts, path, "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if !strings.Contains(body, "\"result\"") {
		t.Fatalf("body missing result: %s", body)
	}
}

// TestAPIAirportRefreshOneWithRules 单机场刷新带 with_rules=1 时同时更新规则源。
func TestAPIAirportRefreshOneWithRules(t *testing.T) {
	ts, mgr := newWithRulesServer(t)
	airports, err := mgr.ListAirports(t.Context())
	if err != nil || len(airports) != 1 {
		t.Fatalf("ListAirports: %v len=%d", err, len(airports))
	}
	path := "/api/airports/" + strconv.FormatInt(airports[0].ID, 10) + "/refresh?with_rules=1"
	status, _, body := uiPostJSON(t, ts, path, "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Result json.RawMessage `json:"result"`
		Import *refreshImportItem `json:"import"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if len(got.Result) == 0 {
		t.Fatalf("缺少 result: %s", body)
	}
	if got.Import == nil || got.Import.Err != "" || got.Import.RuleSourceRules != 2 {
		t.Fatalf("import 不符: %+v body=%s", got.Import, body)
	}
}

// 不带 with_rules 时不出现 import 键，保持旧响应形状。
func TestAPIAirportRefreshOneWithoutRulesHasNoImportKey(t *testing.T) {
	ts, _, id, _ := setupAirportWithTraffic(t)
	path := "/api/airports/" + strconv.FormatInt(id, 10) + "/refresh"
	status, _, body := uiPostJSON(t, ts, path, "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Contains(body, "\"import\"") {
		t.Fatalf("不带 with_rules 时不应出现 import 键: %s", body)
	}
}

func TestAPIAirportRefreshOneNotFound(t *testing.T) {
	ts, _, _, _ := setupAirportWithTraffic(t)
	status, _, body := uiPostJSON(t, ts, "/api/airports/999999/refresh", "")
	if status != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestAPIAirportsRefreshWrongMethod(t *testing.T) {
	ts, _, _, _ := setupAirportWithTraffic(t)
	status, _, body := uiGet(t, ts, "/api/airports/refresh")
	if status == http.StatusOK {
		t.Fatalf("GET /api/airports/refresh should not be 200; body=%s", body)
	}
}

func TestAPIAirportsRefreshStaticPriority(t *testing.T) {
	ts, _, _, _ := setupAirportWithTraffic(t)
	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh", "")
	if status == http.StatusBadRequest && strings.Contains(body, "无效的 ID") {
		t.Fatalf("static route hijacked by {id}: body=%s", body)
	}
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestAPITrafficPreviewDefault(t *testing.T) {
	ts, _, _, _ := setupAirportWithTraffic(t)
	status, _, body := uiGet(t, ts, "/api/traffic/preview")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Usage struct {
			Upload   int64 `json:"upload"`
			Download int64 `json:"download"`
			Total    int64 `json:"total"`
		} `json:"usage"`
		Header string `json:"header"`
		OK     bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if !got.OK {
		t.Fatalf("ok=false, want true; body=%s", body)
	}
	if got.Header == "" {
		t.Fatalf("header empty; body=%s", body)
	}
	if got.Usage.Upload != 10 || got.Usage.Download != 20 || got.Usage.Total != 100 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestAPITrafficPreviewOff(t *testing.T) {
	ts, _, _, _ := setupAirportWithTraffic(t)
	status, _, body := uiGet(t, ts, "/api/traffic/preview?strategy="+url.QueryEscape(`{"preset":"off"}`))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Header string `json:"header"`
		OK     bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.OK {
		t.Fatalf("ok=true, want false")
	}
	if got.Header != "" {
		t.Fatalf("header=%q, want empty", got.Header)
	}
}

func TestAPITrafficPreviewBadJSON(t *testing.T) {
	ts, _, _, _ := setupAirportWithTraffic(t)
	status, _, body := uiGet(t, ts, "/api/traffic/preview?strategy=not-json")
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if !strings.Contains(body, "strategy 参数不是合法 JSON") {
		t.Fatalf("body missing error text: %s", body)
	}
}

func TestAPITrafficPreviewNoData(t *testing.T) {
	ts, mgr, _ := newTrafficTestServer(t)
	_ = mgr
	ctx := t.Context()
	if _, err := mgr.AddAirport(ctx, "机场B", "http://example.com/sub", "", true); err != nil {
		t.Fatal(err)
	}
	status, _, body := uiGet(t, ts, "/api/traffic/preview")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got struct {
		Header string `json:"header"`
		OK     bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.OK {
		t.Fatalf("ok=true, want false")
	}
	if got.Header != "" {
		t.Fatalf("header=%q, want empty", got.Header)
	}
}

func TestAPITrafficPreviewNoPersist(t *testing.T) {
	ts, mgr, id, _ := setupAirportWithTraffic(t)
	ctx := t.Context()
	before, err := mgr.GetAirport(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	status, _, body := uiGet(t, ts, "/api/traffic/preview")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	after, err := mgr.GetAirport(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if before.Upload != after.Upload || before.Download != after.Download ||
		before.Total != after.Total || before.Expire != after.Expire ||
		before.NodeCount != after.NodeCount || before.TrafficAt != after.TrafficAt ||
		before.TrafficErr != after.TrafficErr {
		t.Fatalf("preview modified airport row: before=%+v after=%+v", before, after)
	}
}

// newWithRulesServer 起一个启用订阅管理的服务，并预置一个指向 rulesUpstream 的启用机场。
func newWithRulesServer(t *testing.T) (*httptest.Server, *subforge.Manager) {
	ts, mgr, _ := newTrafficTestServer(t)
	up := rulesUpstream(t)
	if _, err := mgr.AddAirport(t.Context(), "机场R", up.URL, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	return ts, mgr
}

// TestAPIAirportsRefreshWithoutWithRulesHasNoImportsKey 不带 query 时响应必须与旧版逐字节一致：只有 results/ok/fail。
func TestAPIAirportsRefreshWithoutWithRulesHasNoImportsKey(t *testing.T) {
	ts, _ := newWithRulesServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if strings.Contains(body, "imports") {
		t.Fatalf("不带 with_rules 时不应出现 imports 键: %s", body)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if len(got) != 3 {
		t.Fatalf("顶层键应为 3 个（results/ok/fail），got=%v", got)
	}
	for _, k := range []string{"results", "ok", "fail"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("缺少键 %s: %s", k, body)
		}
	}
}

type refreshImportItem struct {
	AirportID         int64    `json:"airport_id"`
	Name              string   `json:"name"`
	RulesAdded        int      `json:"rules_added"`
	RulesSkipped      int      `json:"rules_skipped"`
	ProvidersAdded    int      `json:"providers_added"`
	GroupsCreated     int      `json:"groups_created"`
	RuleSourceName    string   `json:"rule_source_name"`
	RuleSourceRules   int      `json:"rule_source_rules"`
	RuleSourceVersion string   `json:"rule_source_version"`
	Warnings          []string `json:"warnings"`
	Err               string   `json:"err"`
}

type refreshWithImports struct {
	Results []struct {
		OK bool `json:"ok"`
	} `json:"results"`
	OK      int                 `json:"ok"`
	Fail    int                 `json:"fail"`
	Imports []refreshImportItem `json:"imports"`
}

// TestAPIAirportsRefreshWithRulesImports ?with_rules=1 时对刷新成功的机场导入规则与规则集。
func TestAPIAirportsRefreshWithRulesImports(t *testing.T) {
	ts, _ := newWithRulesServer(t)
	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh?with_rules=1", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got refreshWithImports
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.OK != 1 || got.Fail != 0 {
		t.Fatalf("ok=%d fail=%d body=%s", got.OK, got.Fail, body)
	}
	if len(got.Imports) != 1 {
		t.Fatalf("imports 长度 = %d，应为 1；body=%s", len(got.Imports), body)
	}
	item := got.Imports[0]
	if item.Err != "" {
		t.Fatalf("imports[0].err = %q，应为空", item.Err)
	}
	if item.RulesAdded != 0 || item.RuleSourceRules != 2 || item.RuleSourceName == "" {
		t.Fatalf("机场规则应写入规则源：%+v", item)
	}
	if item.ProvidersAdded != 1 {
		t.Fatalf("providers_added = %d，应为 1", item.ProvidersAdded)
	}
	if item.Warnings == nil {
		t.Fatalf("warnings 必须是非 nil 数组")
	}
	if item.Name != "机场R" {
		t.Fatalf("name = %q，应为 机场R", item.Name)
	}
}

// TestAPIAirportsRefreshWithRulesCreatesGroups 自动导入必须和手动导入一样补建目标分组，
// 否则规则目标找不到分组，订阅只能退化成 DIRECT/兜底。
func TestAPIAirportsRefreshWithRulesCreatesGroups(t *testing.T) {
	ts, mgr, _ := newTrafficTestServer(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("proxies:\n" +
			"  - name: p1\n" +
			"    type: ss\n" +
			"    server: s.example.com\n" +
			"    port: 8388\n" +
			"    cipher: aes-256-gcm\n" +
			"    password: pw\n" +
			"rules:\n" +
			"  - DOMAIN-SUFFIX,auto-group.test,自动建组\n"))
	}))
	t.Cleanup(up.Close)

	a, err := mgr.AddAirport(t.Context(), "机场G", up.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if a == nil {
		t.Fatal("AddAirport 返回 nil")
	}

	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh?with_rules=1", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got refreshWithImports
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if len(got.Imports) != 1 {
		t.Fatalf("imports 长度 = %d；body=%s", len(got.Imports), body)
	}
	if got.Imports[0].GroupsCreated != 1 {
		t.Fatalf("groups_created = %d，应为 1；body=%s", got.Imports[0].GroupsCreated, body)
	}
	groups, err := mgr.ListProxyGroups(t.Context())
	if err != nil {
		t.Fatalf("ListProxyGroups: %v", err)
	}
	found := false
	for _, g := range groups {
		if g.Name == "自动建组" && g.Source == "airport:机场G" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("自动导入未创建目标分组，groups=%+v", groups)
	}
	if rules, err := mgr.ListRulesByLayers(t.Context(), []string{"airport"}); err != nil {
		t.Fatalf("ListRulesByLayers: %v", err)
	} else if len(rules) != 0 {
		t.Fatalf("机场规则不应写 rules 表：%+v", rules)
	}
	sources, err := mgr.ListRuleSources(t.Context())
	if err != nil {
		t.Fatalf("ListRuleSources: %v", err)
	}
	var srcName string
	for _, s := range sources {
		if s.Origin == "airport" {
			srcName = s.Name
			break
		}
	}
	if srcName == "" {
		t.Fatalf("自动导入未生成机场规则源：%+v", sources)
	}
	rules, _ := mgr.RuleSourceRules(t.Context(), []string{srcName})
	if !strings.Contains(strings.Join(rules, "\n"), "DOMAIN-SUFFIX,auto-group.test,自动建组") {
		t.Fatalf("机场规则源缺少 auto-group.test：%v", rules)
	}
}

// TestAPIAirportsRefreshWithRulesTruthyVariants 只有 1/true/yes/on（大小写不敏感、可带空白）才算真。
func TestAPIAirportsRefreshWithRulesTruthyVariants(t *testing.T) {
	for _, v := range []string{"1", "true", "YES", " on "} {
		ts, _ := newWithRulesServer(t)
		status, _, body := uiPostJSON(t, ts, "/api/airports/refresh?with_rules="+url.QueryEscape(v), "")
		if status != http.StatusOK {
			t.Fatalf("with_rules=%q status=%d body=%s", v, status, body)
		}
		if !strings.Contains(body, "\"imports\"") {
			t.Fatalf("with_rules=%q 应带 imports 键: %s", v, body)
		}
	}
	for _, v := range []string{"0", "false", "", "bogus"} {
		ts, _ := newWithRulesServer(t)
		status, _, body := uiPostJSON(t, ts, "/api/airports/refresh?with_rules="+url.QueryEscape(v), "")
		if status != http.StatusOK {
			t.Fatalf("with_rules=%q status=%d body=%s", v, status, body)
		}
		if strings.Contains(body, "imports") {
			t.Fatalf("with_rules=%q 不应带 imports 键: %s", v, body)
		}
	}
}

// TestAPIAirportsRefreshWithRulesImportFailureDoesNotFailRefresh 单机场导入失败只记 err，整体仍 200。
func TestAPIAirportsRefreshWithRulesImportFailureDoesNotFailRefresh(t *testing.T) {
	ts, mgr, _ := newTrafficTestServer(t)
	var calls int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
			_, _ = w.Write([]byte(rulesClashYAML))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer up.Close()
	if _, err := mgr.AddAirport(t.Context(), "机场F", up.URL, "", true); err != nil {
		t.Fatal(err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh?with_rules=1", "")
	if status != http.StatusOK {
		t.Fatalf("导入失败不应让整体非 200: status=%d body=%s", status, body)
	}
	var got refreshWithImports
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.OK != 1 {
		t.Fatalf("刷新应成功 ok=%d body=%s", got.OK, body)
	}
	if len(got.Imports) != 1 {
		t.Fatalf("imports 长度 = %d，应为 1；body=%s", len(got.Imports), body)
	}
	if got.Imports[0].Err == "" {
		t.Fatalf("imports[0].err 应非空；body=%s", body)
	}
	if !strings.Contains(got.Imports[0].Err, "导入失败") {
		t.Fatalf("imports[0].err = %q，应含 导入失败", got.Imports[0].Err)
	}
	if got.Imports[0].RulesAdded != 0 || got.Imports[0].ProvidersAdded != 0 {
		t.Fatalf("失败时计数应为 0: %+v", got.Imports[0])
	}
}

// TestAPIAirportsRefreshWithRulesOnlyOKAirportsImported 刷新失败的机场不进入 imports。
func TestAPIAirportsRefreshWithRulesOnlyOKAirportsImported(t *testing.T) {
	ts, mgr, _ := newTrafficTestServer(t)
	up := rulesUpstream(t)
	if _, err := mgr.AddAirport(t.Context(), "机场OK", up.URL, "", true); err != nil {
		t.Fatal(err)
	}
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()
	if _, err := mgr.AddAirport(t.Context(), "机场坏", dead.URL, "", true); err != nil {
		t.Fatal(err)
	}
	status, _, body := uiPostJSON(t, ts, "/api/airports/refresh?with_rules=1", "")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var got refreshWithImports
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, body)
	}
	if got.OK != 1 || got.Fail != 1 {
		t.Fatalf("ok=%d fail=%d body=%s", got.OK, got.Fail, body)
	}
	if len(got.Imports) != 1 {
		t.Fatalf("imports 只应含刷新成功的机场，长度 = %d；body=%s", len(got.Imports), body)
	}
	if got.Imports[0].Name != "机场OK" {
		t.Fatalf("imports[0].name = %q", got.Imports[0].Name)
	}
}

// TestIsTruthyQuery 直接覆盖真值解析的边界。
func TestIsTruthyQuery(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "Yes", "on", " ON ", "\ttrue\n"} {
		if !isTruthyQuery(v) {
			t.Errorf("isTruthyQuery(%q) = false，应为 true", v)
		}
	}
	for _, v := range []string{"", "0", "false", "off", "no", "bogus", "2"} {
		if isTruthyQuery(v) {
			t.Errorf("isTruthyQuery(%q) = true，应为 false", v)
		}
	}
}

// TestImportRulesAfterRefreshNilManager 覆盖 manager 缺失时的每元素 err（不新增 503 路径）。
func TestImportRulesAfterRefreshNilManager(t *testing.T) {
	srv, _ := importTestServer(t)
	got := srv.importRulesAfterRefresh(context.Background(), nil, []subforge.TrafficRefreshResult{{AirportID: 1, Name: "a", OK: true}})
	if len(got) != 1 {
		t.Fatalf("len = %d，应为 1", len(got))
	}
	if got[0]["err"] != "订阅管理未启用" {
		t.Fatalf("err = %v，应为 订阅管理未启用", got[0]["err"])
	}
}

// TestImportRulesAfterRefreshNilEngine 覆盖引擎缺失时的每元素 err。
func TestImportRulesAfterRefreshNilEngine(t *testing.T) {
	// New 要求 Engine 非空，这里直接构造零值 Server 以覆盖 engine==nil 的防御分支。
	srv := &Server{}
	mgr, err := subforge.NewManager(filepath.Join(t.TempDir(), "engine.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	got := srv.importRulesAfterRefresh(context.Background(), mgr, []subforge.TrafficRefreshResult{{AirportID: 1, Name: "a", OK: true}})
	if len(got) != 1 {
		t.Fatalf("len = %d，应为 1", len(got))
	}
	if got[0]["err"] != "转换引擎未启用" {
		t.Fatalf("err = %v，应为 转换引擎未启用", got[0]["err"])
	}
}

// TestImportRulesAfterRefreshMissingAirport 覆盖 GetAirport 失败分支。
func TestImportRulesAfterRefreshMissingAirport(t *testing.T) {
	srv, mgr := importTestServer(t)
	got := srv.importRulesAfterRefresh(context.Background(), mgr, []subforge.TrafficRefreshResult{{AirportID: 99999, Name: "ghost", OK: true}})
	if len(got) != 1 {
		t.Fatalf("len = %d，应为 1", len(got))
	}
	if got[0]["err"] == "" {
		t.Fatalf("不存在的机场应记 err，got=%v", got[0])
	}
}
