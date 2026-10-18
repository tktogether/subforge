package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/subscheck"
)

type checkOut struct {
	Results []subscheck.Result `json:"results"`
	OK      int                `json:"ok"`
	Fail    int                `json:"fail"`
	At      string             `json:"at"`
}

func newCheckFakeSub(t *testing.T, status int) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte("proxies: []"))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func postCheck(t *testing.T, srv *Server, body string) (int, checkOut, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(http.MethodPost, "/api/airports/check", nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, "/api/airports/check", strings.NewReader(body))
	}
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPICheck(rr, req)
	var out checkOut
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out, rr.Body.String()
}

func getCheckLast(t *testing.T, srv *Server) (int, checkOut, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/airports/check/last", nil)
	srv.handleAPICheckLast(rr, req)
	var out checkOut
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out, rr.Body.String()
}

func TestAPICheckAllEnabledSuccess(t *testing.T) {
	srv, mgr := scr3TestServer(t)
	ts1 := newCheckFakeSub(t, http.StatusOK)
	ts2 := newCheckFakeSub(t, http.StatusOK)
	ctx := context.Background()
	if _, err := mgr.AddAirport(ctx, "a1", ts1.URL, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "a2", ts2.URL, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "disabled", ts1.URL, "", false); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	code, out, raw := postCheck(t, srv, `{}`)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if len(out.Results) != 2 {
		t.Fatalf("应只测 2 个启用机场，实际 %d: %s", len(out.Results), raw)
	}
	if out.OK != 2 || out.Fail != 0 {
		t.Fatalf("ok=%d fail=%d raw=%s", out.OK, out.Fail, raw)
	}
	if _, err := time.Parse(time.RFC3339, out.At); err != nil {
		t.Fatalf("at 非 RFC3339: %q", out.At)
	}
}

func TestAPICheckFailureCounted(t *testing.T) {
	srv, mgr := scr3TestServer(t)
	tsBad := newCheckFakeSub(t, http.StatusInternalServerError)
	if _, err := mgr.AddAirport(context.Background(), "bad", tsBad.URL, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	code, out, raw := postCheck(t, srv, `{}`)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if out.OK != 0 || out.Fail != 1 {
		t.Fatalf("ok=%d fail=%d raw=%s", out.OK, out.Fail, raw)
	}
	if len(out.Results) != 1 || out.Results[0].OK {
		t.Fatalf("结果应失败: %s", raw)
	}
}

func TestAPICheckIDsFilter(t *testing.T) {
	srv, mgr := scr3TestServer(t)
	ts1 := newCheckFakeSub(t, http.StatusOK)
	ts2 := newCheckFakeSub(t, http.StatusOK)
	ctx := context.Background()
	a1, err := mgr.AddAirport(ctx, "a1", ts1.URL, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if _, err := mgr.AddAirport(ctx, "a2", ts2.URL, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	code, out, raw := postCheck(t, srv, `{"ids":[`+strconv.FormatInt(a1.ID, 10)+`]}`)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if len(out.Results) != 1 || out.Results[0].URL != ts1.URL {
		t.Fatalf("ids 过滤失败: %s", raw)
	}
}

func TestAPICheckUnknownID(t *testing.T) {
	srv, _ := scr3TestServer(t)
	code, _, raw := postCheck(t, srv, `{"ids":[9999]}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !strings.Contains(raw, "机场 9999 不存在") {
		t.Fatalf("body=%s", raw)
	}
}

func TestAPICheckEmptyIDs(t *testing.T) {
	srv, _ := scr3TestServer(t)
	code, _, raw := postCheck(t, srv, `{"ids":[]}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !strings.Contains(raw, "没有可测活的机场") {
		t.Fatalf("body=%s", raw)
	}
}

func TestAPICheckMethodNotAllowed(t *testing.T) {
	srv, _ := scr3TestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/airports/check", nil)
	srv.handleAPICheck(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "仅支持 POST") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAPICheckLastMethodNotAllowed(t *testing.T) {
	srv, _ := scr3TestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/airports/check/last", nil)
	srv.handleAPICheckLast(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "仅支持 GET") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAPICheckNoSubManager(t *testing.T) {
	srv := New(Config{
		Engine: &convert.Engine{
			RendererFor: func(string) (render.Renderer, error) { return nil, nil },
			Profile:     render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	code, _, _ := postCheck(t, srv, `{}`)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("check status=%d", code)
	}
	code2, _, _ := getCheckLast(t, srv)
	if code2 != http.StatusServiceUnavailable {
		t.Fatalf("last status=%d", code2)
	}
}

func TestAPICheckLastNeverRun(t *testing.T) {
	srv, _ := scr3TestServer(t)
	code, out, raw := getCheckLast(t, srv)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if out.Results == nil || len(out.Results) != 0 {
		t.Fatalf("从未跑过应返回空数组: %s", raw)
	}
	if out.At != "" {
		t.Fatalf("从未跑过 at 应为空串: %q", out.At)
	}
}

func TestAPICheckLastAfterRun(t *testing.T) {
	srv, mgr := scr3TestServer(t)
	ts1 := newCheckFakeSub(t, http.StatusOK)
	if _, err := mgr.AddAirport(context.Background(), "a1", ts1.URL, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	code, runOut, raw := postCheck(t, srv, `{}`)
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	code2, lastOut, raw2 := getCheckLast(t, srv)
	if code2 != http.StatusOK {
		t.Fatalf("last status=%d body=%s", code2, raw2)
	}
	if len(lastOut.Results) != len(runOut.Results) {
		t.Fatalf("last 结果数不一致: %s", raw2)
	}
	if lastOut.At != runOut.At {
		t.Fatalf("last.at=%q run.at=%q", lastOut.At, runOut.At)
	}
	if _, err := time.Parse(time.RFC3339, lastOut.At); err != nil {
		t.Fatalf("at 非 RFC3339: %q", lastOut.At)
	}
}
