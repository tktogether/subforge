package subforge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/traffic"
)

func TestUpdateAirportTraffic(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, err := m.AddAirport(ctx, "A", "http://up.example/sub", "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	u := traffic.Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	if err := m.UpdateAirportTraffic(ctx, a.ID, &u, 7, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic with usage: %v", err)
	}
	got, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got.Upload != 1 || got.Download != 2 || got.Total != 3 || got.Expire != 4 {
		t.Fatalf("traffic values = %d/%d/%d/%d, want 1/2/3/4", got.Upload, got.Download, got.Total, got.Expire)
	}
	if got.NodeCount != 7 {
		t.Fatalf("NodeCount = %d, want 7", got.NodeCount)
	}
	if got.TrafficAt <= 0 {
		t.Fatalf("TrafficAt = %d, want > 0", got.TrafficAt)
	}
	if got.TrafficErr != "" {
		t.Fatalf("TrafficErr = %q, want empty", got.TrafficErr)
	}
	if err := m.UpdateAirportTraffic(ctx, a.ID, nil, 0, "上游炸了"); err != nil {
		t.Fatalf("UpdateAirportTraffic nil usage: %v", err)
	}
	got2, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got2.Upload != 1 || got2.Download != 2 || got2.Total != 3 || got2.Expire != 4 {
		t.Fatalf("traffic values changed on nil usage: %d/%d/%d/%d, want 1/2/3/4", got2.Upload, got2.Download, got2.Total, got2.Expire)
	}
	if got2.TrafficErr != "上游炸了" {
		t.Fatalf("TrafficErr = %q, want %q", got2.TrafficErr, "上游炸了")
	}
	if err := m.UpdateAirportTraffic(ctx, a.ID, nil, 0, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic clear err: %v", err)
	}
	got3, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got3.TrafficErr != "" {
		t.Fatalf("TrafficErr = %q, want empty", got3.TrafficErr)
	}
	if err := m.UpdateAirportTraffic(ctx, 99999, &u, 1, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic nonexistent id: %v", err)
	}
}

func TestGetTrafficStrategy(t *testing.T) {
	ctx := context.Background()

	// 未配置 → 默认 sum
	m := newTestManager(t)
	s, err := m.GetTrafficStrategy(ctx)
	if err != nil {
		t.Fatalf("GetTrafficStrategy unconfigured: %v", err)
	}
	if s.Preset != "sum" {
		t.Fatalf("Preset = %q, want sum", s.Preset)
	}
	if s.Upload != traffic.FieldSum || s.Download != traffic.FieldSum || s.Total != traffic.FieldSum {
		t.Fatalf("Upload/Download/Total = %v/%v/%v, want sum/sum/sum", s.Upload, s.Download, s.Total)
	}
	if s.Expire != traffic.FieldMin {
		t.Fatalf("Expire = %v, want min", s.Expire)
	}

	// 预设名
	if err := m.setConfig(ctx, TrafficStrategyKey, "max"); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	s, err = m.GetTrafficStrategy(ctx)
	if err != nil {
		t.Fatalf("GetTrafficStrategy max: %v", err)
	}
	if s.Preset != "max" {
		t.Fatalf("Preset = %q, want max", s.Preset)
	}
	if s.Upload != traffic.FieldMax || s.Download != traffic.FieldMax || s.Total != traffic.FieldMax || s.Expire != traffic.FieldMax {
		t.Fatalf("fields = %v/%v/%v/%v, want max/max/max/max", s.Upload, s.Download, s.Total, s.Expire)
	}

	// 大小写/空白
	if err := m.setConfig(ctx, TrafficStrategyKey, "  OFF  "); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	s, err = m.GetTrafficStrategy(ctx)
	if err != nil {
		t.Fatalf("GetTrafficStrategy off: %v", err)
	}
	if s.Preset != "off" {
		t.Fatalf("Preset = %q, want off", s.Preset)
	}
	if s.Upload != traffic.FieldNone || s.Download != traffic.FieldNone || s.Total != traffic.FieldNone || s.Expire != traffic.FieldNone {
		t.Fatalf("fields = %v/%v/%v/%v, want none/none/none/none", s.Upload, s.Download, s.Total, s.Expire)
	}

	// JSON 字段级
	if err := m.setConfig(ctx, TrafficStrategyKey, `{"preset":"sum","upload":"min"}`); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	s, err = m.GetTrafficStrategy(ctx)
	if err != nil {
		t.Fatalf("GetTrafficStrategy json: %v", err)
	}
	if s.Upload != traffic.FieldMin {
		t.Fatalf("Upload = %v, want min", s.Upload)
	}
	if s.Download != traffic.FieldSum {
		t.Fatalf("Download = %v, want sum", s.Download)
	}
	if s.Total != traffic.FieldSum {
		t.Fatalf("Total = %v, want sum", s.Total)
	}
	if s.Expire != traffic.FieldMin {
		t.Fatalf("Expire = %v, want min", s.Expire)
	}

	// 非法 JSON → 回落默认
	if err := m.setConfig(ctx, TrafficStrategyKey, "{not json"); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	s, err = m.GetTrafficStrategy(ctx)
	if err != nil {
		t.Fatalf("GetTrafficStrategy invalid json: %v", err)
	}
	if s.Preset != "sum" {
		t.Fatalf("Preset = %q, want sum", s.Preset)
	}

	// getConfig 报错分支（关闭 DB）
	m2 := newTestManager(t)
	_ = m2.Close()
	_, err = m2.GetTrafficStrategy(ctx)
	if err == nil {
		t.Fatal("expected error from closed DB")
	}
}

func TestSetTrafficStrategyRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.SetTrafficStrategy(ctx, traffic.Strategy{Preset: "avg"}); err != nil {
		t.Fatalf("SetTrafficStrategy: %v", err)
	}
	s, err := m.GetTrafficStrategy(ctx)
	if err != nil {
		t.Fatalf("GetTrafficStrategy: %v", err)
	}
	if s.Preset != "avg" {
		t.Fatalf("Preset = %q, want avg", s.Preset)
	}
	raw, err := m.getConfig(ctx, TrafficStrategyKey)
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if !json.Valid([]byte(raw)) {
		t.Fatalf("raw not valid JSON: %s", raw)
	}
	if !strings.Contains(raw, "\"preset\":\"avg\"") {
		t.Fatalf("raw = %s, want contains preset avg", raw)
	}
}

func TestRefreshAirportTrafficNilFetcher(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, err := m.RefreshAirportTraffic(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil fetcher")
	}
	if !strings.Contains(err.Error(), "fetcher") {
		t.Fatalf("err = %v, want contains fetcher", err)
	}
}

func readFixture(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "fixtures", "mixed-subscription.txt")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestRefreshAirportTrafficHappy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	a, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	results, err := m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("RefreshAirportTraffic: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	r := results[0]
	if r.AirportID != a.ID {
		t.Fatalf("AirportID = %d, want %d", r.AirportID, a.ID)
	}
	if r.Name != "机场A" {
		t.Fatalf("Name = %q, want 机场A", r.Name)
	}
	if !r.OK {
		t.Fatalf("OK = false, want true; Err = %q", r.Err)
	}
	if r.Err != "" {
		t.Fatalf("Err = %q, want empty", r.Err)
	}
	if r.Usage != (traffic.Usage{Upload: 10, Download: 20, Total: 100, Expire: 1700000000}) {
		t.Fatalf("Usage = %+v, want {10 20 100 1700000000}", r.Usage)
	}
	if r.NodeCount != 12 {
		t.Fatalf("NodeCount = %d, want 12", r.NodeCount)
	}
	got, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got.Upload != 10 || got.Download != 20 || got.Total != 100 || got.Expire != 1700000000 {
		t.Fatalf("traffic = %d/%d/%d/%d, want 10/20/100/1700000000", got.Upload, got.Download, got.Total, got.Expire)
	}
	if got.NodeCount != 12 {
		t.Fatalf("NodeCount = %d, want 12", got.NodeCount)
	}
	if got.TrafficAt <= 0 {
		t.Fatalf("TrafficAt = %d, want > 0", got.TrafficAt)
	}
	if got.TrafficErr != "" {
		t.Fatalf("TrafficErr = %q, want empty", got.TrafficErr)
	}
	audits, err := m.DB().ListFetchAudit(ctx, 10)
	if err != nil {
		t.Fatalf("ListFetchAudit: %v", err)
	}
	if len(audits) < 1 {
		t.Fatalf("len(audits) = %d, want >= 1", len(audits))
	}
	if audits[0].AirportID != a.ID {
		t.Fatalf("AirportID = %d, want %d", audits[0].AirportID, a.ID)
	}
	if audits[0].URL != srv.URL {
		t.Fatalf("URL = %q, want %q", audits[0].URL, srv.URL)
	}
	if audits[0].StatusCode != 200 {
		t.Fatalf("StatusCode = %d, want 200", audits[0].StatusCode)
	}
	if !audits[0].OK {
		t.Fatalf("OK = false, want true")
	}
	if audits[0].Bytes <= 0 {
		t.Fatalf("Bytes = %d, want > 0", audits[0].Bytes)
	}
}

func TestRefreshAirportTrafficUpstreamErrorKeepsTraffic(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	a, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	// 先成功采集一次建立基线
	_, err = m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("first RefreshAirportTraffic: %v", err)
	}
	// 切到 500
	fail.Store(true)
	results, err := m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("second RefreshAirportTraffic: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	r := results[0]
	if r.OK {
		t.Fatalf("OK = true, want false")
	}
	if r.Err == "" {
		t.Fatalf("Err empty, want non-empty")
	}
	got, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got.Upload != 10 || got.Download != 20 || got.Total != 100 || got.Expire != 1700000000 {
		t.Fatalf("traffic changed on error: %d/%d/%d/%d, want 10/20/100/1700000000", got.Upload, got.Download, got.Total, got.Expire)
	}
	if got.TrafficErr == "" {
		t.Fatalf("TrafficErr empty, want non-empty")
	}
	audits, err := m.DB().ListFetchAudit(ctx, 10)
	if err != nil {
		t.Fatalf("ListFetchAudit: %v", err)
	}
	if len(audits) < 1 {
		t.Fatalf("len(audits) = %d, want >= 1", len(audits))
	}
	if audits[0].OK {
		t.Fatalf("audit OK = true, want false")
	}
	if audits[0].Error == "" {
		t.Fatalf("audit Error empty, want non-empty")
	}
	if audits[0].StatusCode != 0 {
		t.Fatalf("audit StatusCode = %d, want 0 (fetcher returns zero Result on 5xx)", audits[0].StatusCode)
	}
}

func TestRefreshAirportTrafficMissingUserinfo(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	// 先成功采集一次建立基线
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	a, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	_, err = m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("first RefreshAirportTraffic: %v", err)
	}
	base, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if base.NodeCount <= 0 {
		t.Fatalf("基线节点数 = %d, want > 0", base.NodeCount)
	}
	// 切到无 header 的 server
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fixture)
	}))
	t.Cleanup(srv2.Close)
	// 更新机场 URL
	_, err = m.db.Raw().ExecContext(ctx, "UPDATE airports SET subscription_url = ? WHERE id = ?", srv2.URL, a.ID)
	if err != nil {
		t.Fatalf("update url: %v", err)
	}
	results, err := m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("RefreshAirportTraffic: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	r := results[0]
	// 上游不返回流量头不是故障：订阅拉到了、节点也解析出来了，只是没有余额可读。
	// 以前这里算失败，还会把节点数丢掉，界面上就成了"节点数 0 + 采集失败"。
	if !r.OK {
		t.Fatalf("OK = false, want true（缺流量头不是采集失败）: err=%q", r.Err)
	}
	if r.Err != "" {
		t.Fatalf("Err = %q, want 空串", r.Err)
	}
	if !strings.Contains(r.Note, "subscription-userinfo") {
		t.Fatalf("Note 应说明原因, got %q", r.Note)
	}
	got, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got.Upload != 10 || got.Download != 20 || got.Total != 100 || got.Expire != 1700000000 {
		t.Fatalf("traffic changed: %d/%d/%d/%d, want 10/20/100/1700000000", got.Upload, got.Download, got.Total, got.Expire)
	}
	if got.NodeCount != base.NodeCount || got.NodeCount != r.NodeCount {
		t.Fatalf("node_count = %d, want %d（缺流量头也要写回节点数）", got.NodeCount, base.NodeCount)
	}
	if got.TrafficErr != "" {
		t.Fatalf("traffic_err = %q, want 空串（没有失败就不该留错误）", got.TrafficErr)
	}
}

func TestRefreshAirportTrafficUnparsableUserinfo(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "garbage")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	_, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	results, err := m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("RefreshAirportTraffic: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	r := results[0]
	if r.OK {
		t.Fatalf("OK = true, want false")
	}
	if !strings.HasPrefix(r.Err, "subscription-userinfo 无法解析: ") {
		t.Fatalf("Err = %q, want prefix", r.Err)
	}
	if !strings.Contains(r.Err, "garbage") {
		t.Fatalf("Err = %q, want contains garbage", r.Err)
	}
}

func TestRefreshAirportTrafficSkipsDisabledAndEmptyURL(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3; expire=4")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	_, err := m.AddAirport(ctx, "停用", srv.URL, "", false)
	if err != nil {
		t.Fatalf("AddAirport disabled: %v", err)
	}
	a2, err := m.AddAirport(ctx, "启用", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport enabled: %v", err)
	}
	_, err = m.AddAirport(ctx, "空URL", "", "", true)
	if err != nil {
		t.Fatalf("AddAirport empty url: %v", err)
	}
	results, err := m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("RefreshAirportTraffic: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Name != "启用" {
		t.Fatalf("Name = %q, want 启用", results[0].Name)
	}
	if results[0].AirportID != a2.ID {
		t.Fatalf("AirportID = %d, want %d", results[0].AirportID, a2.ID)
	}
}

func TestRefreshAirportTrafficByIDNilFetcher(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	_, err := m.RefreshAirportTrafficByID(ctx, nil, 1)
	if err == nil {
		t.Fatal("expected error for nil fetcher")
	}
	if !strings.Contains(err.Error(), "fetcher") {
		t.Fatalf("err = %v, want contains fetcher", err)
	}
}

func TestRefreshAirportTrafficByIDNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	_, err := m.RefreshAirportTrafficByID(ctx, fetcher, 999999)
	if err == nil {
		t.Fatal("expected error for nonexistent id")
	}
}

func TestRefreshAirportTrafficByIDHappy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	a, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	res, err := m.RefreshAirportTrafficByID(ctx, fetcher, a.ID)
	if err != nil {
		t.Fatalf("RefreshAirportTrafficByID: %v", err)
	}
	if res.AirportID != a.ID {
		t.Fatalf("AirportID = %d, want %d", res.AirportID, a.ID)
	}
	if !res.OK {
		t.Fatalf("OK = false, want true; Err = %q", res.Err)
	}
	if res.Usage != (traffic.Usage{Upload: 10, Download: 20, Total: 100, Expire: 1700000000}) {
		t.Fatalf("Usage = %+v, want {10 20 100 1700000000}", res.Usage)
	}
	got, err := m.GetAirport(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got.Upload != 10 || got.Download != 20 || got.Total != 100 || got.Expire != 1700000000 {
		t.Fatalf("traffic = %d/%d/%d/%d, want 10/20/100/1700000000", got.Upload, got.Download, got.Total, got.Expire)
	}
}

func TestPreviewTrafficEmptyDB(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	u, h, ok, err := m.PreviewTraffic(ctx, traffic.Strategy{})
	if err != nil {
		t.Fatalf("PreviewTraffic err = %v", err)
	}
	if ok {
		t.Fatalf("ok = true, want false")
	}
	if h != "" {
		t.Fatalf("header = %q, want empty", h)
	}
	if u != (traffic.Usage{}) {
		t.Fatalf("usage = %+v, want zero", u)
	}
}

func TestPreviewTrafficOffStrategy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	a, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	_, _ = m.RefreshAirportTraffic(ctx, fetcher)
	_ = a
	strat, _ := traffic.PresetStrategy("off")
	u, h, ok, err := m.PreviewTraffic(ctx, strat)
	if err != nil {
		t.Fatalf("PreviewTraffic err = %v", err)
	}
	if ok {
		t.Fatalf("ok = true, want false (off preset)")
	}
	if h != "" {
		t.Fatalf("header = %q, want empty", h)
	}
	_ = u
}

func TestPreviewTrafficSumStrategy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	fixture := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=10; download=20; total=100; expire=1700000000")
		w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{UserAgent: "subforge-test", Timeout: 5 * time.Second, Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3}))
	_, err := m.AddAirport(ctx, "机场A", srv.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	_, err = m.RefreshAirportTraffic(ctx, fetcher)
	if err != nil {
		t.Fatalf("RefreshAirportTraffic: %v", err)
	}
	strat, _ := traffic.PresetStrategy("sum")
	u, h, ok, err := m.PreviewTraffic(ctx, strat)
	if err != nil {
		t.Fatalf("PreviewTraffic err = %v", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true")
	}
	if h == "" {
		t.Fatalf("header empty, want non-empty")
	}
	if u.Upload != 10 || u.Download != 20 || u.Total != 100 {
		t.Fatalf("usage = %+v, want 10/20/100 (got %d/%d/%d)", u, u.Upload, u.Download, u.Total)
	}
	// 不落库：PreviewTraffic 不应写库（流量字段 unchanged 已由 Refresh 设置，再跑一次 preview 不应变化）
	u2, h2, ok2, err2 := m.PreviewTraffic(ctx, strat)
	if err2 != nil {
		t.Fatalf("PreviewTraffic err = %v", err2)
	}
	if !ok2 || h2 != h || u2 != u {
		t.Fatalf("preview not deterministic: u=%+v/%+v h=%q/%q ok=%v/%v", u, u2, h, h2, ok, ok2)
	}
}
