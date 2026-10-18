package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
