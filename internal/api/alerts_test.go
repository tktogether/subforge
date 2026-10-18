package api

import (
	"context"
	"encoding/json"
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

func alertTestServer(t *testing.T) (*Server, *subforge.Manager) {
	t.Helper()
	mgr, err := subforge.NewManager(filepath.Join(t.TempDir(), "alert.db"))
	if err != nil {
		t.Fatalf("打开临时 SQLite: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test", Timeout: 5 * time.Second,
		Parallel: 4, Retries: 0, Backoff: 0, MaxBody: 64 << 20, Redirects: 3,
	}))
	srv := New(Config{
		Engine: &convert.Engine{
			Fetcher: fetcher,
			RendererFor: func(target string) (render.Renderer, error) {
				return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
			},
			Profile: render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	srv.SetSubManager(mgr)
	return srv, mgr
}

func TestAPIAlertChannelsGetEmpty(t *testing.T) {
	srv, _ := alertTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alert-channels", nil)
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"channels":[]`) {
		t.Fatalf("未配置应返回 []，实际: %s", rr.Body.String())
	}
	var out struct {
		Channels []subforge.AlertChannel `json:"channels"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Channels == nil {
		t.Error("channels 不应为 nil")
	}
}

func TestAPIAlertChannelsPutEchoMasked(t *testing.T) {
	srv, _ := alertTestServer(t)
	body := `{"channels":[{"kind":"telegram","enabled":true,"bot_token":"123:abc"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		Channels []subforge.AlertChannel `json:"channels"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(out.Channels) != 1 {
		t.Fatalf("len=%d", len(out.Channels))
	}
	if out.Channels[0].BotToken != subforge.MaskedSecret {
		t.Errorf("bot_token 应脱敏为 ***，实际 %q", out.Channels[0].BotToken)
	}
	if !out.Channels[0].Configured["bot_token"] {
		t.Errorf("configured[bot_token] 应为 true")
	}
}

func TestAPIAlertChannelsPutThenGet(t *testing.T) {
	srv, _ := alertTestServer(t)
	body := `{"channels":[{"kind":"telegram","enabled":true,"bot_token":"tok"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status=%d", rr.Code)
	}
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/alert-channels", nil)
	srv.handleAPIAlertChannels(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("GET status=%d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), `"bot_token":"***"`) {
		t.Fatalf("GET 应仍脱敏: %s", rr2.Body.String())
	}
}

func TestAPIAlertChannelsMaskedKeepsOriginal(t *testing.T) {
	srv, mgr := alertTestServer(t)
	body := `{"channels":[{"kind":"telegram","enabled":true,"bot_token":"REALTOKEN"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first PUT status=%d", rr.Code)
	}
	body2 := `{"channels":[{"kind":"telegram","enabled":true,"bot_token":"***"}]}`
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second PUT status=%d", rr2.Code)
	}
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/alert-channels", nil)
	srv.handleAPIAlertChannels(rr3, req3)
	if !strings.Contains(rr3.Body.String(), `"bot_token":"***"`) {
		t.Fatalf("GET 应脱敏: %s", rr3.Body.String())
	}
	raw, err := mgr.GetConfig(context.Background(), subforge.AlertChannelsKey)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !strings.Contains(raw, "REALTOKEN") {
		t.Fatalf("库中应保留真 token，实际: %s", raw)
	}
	if strings.Contains(raw, `"bot_token":"***"`) {
		t.Fatalf("库中不应存 ***: %s", raw)
	}
}

func TestAPIAlertChannelsPutMissingKind(t *testing.T) {
	srv, _ := alertTestServer(t)
	body := `{"channels":[{"enabled":true}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "告警通道缺少 kind") {
		t.Fatalf("错误文案应含 告警通道缺少 kind: %s", rr.Body.String())
	}
}

func TestAPIAlertChannelsPutInvalidJSON(t *testing.T) {
	srv, _ := alertTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAPIAlertChannelsPutReplacesList(t *testing.T) {
	srv, _ := alertTestServer(t)
	body := `{"channels":[{"kind":"telegram","enabled":true,"bot_token":"a"},{"kind":"dingtalk","enabled":true,"webhook":"http://x"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first PUT status=%d", rr.Code)
	}
	body2 := `{"channels":[{"kind":"telegram","enabled":true,"bot_token":"b"}]}`
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second PUT status=%d", rr2.Code)
	}
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/alert-channels", nil)
	srv.handleAPIAlertChannels(rr3, req3)
	var out struct {
		Channels []subforge.AlertChannel `json:"channels"`
	}
	if err := json.Unmarshal(rr3.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(out.Channels) != 1 {
		t.Fatalf("应只有 1 条，实际 %d", len(out.Channels))
	}
}

func TestAPIAlertChannelsMethodNotAllowed(t *testing.T) {
	srv, _ := alertTestServer(t)
	for _, m := range []string{http.MethodDelete, http.MethodPatch} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/alert-channels", nil)
		srv.handleAPIAlertChannels(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s status=%d body=%s", m, rr.Code, rr.Body.String())
			continue
		}
		if !strings.Contains(rr.Body.String(), "仅支持 GET/PUT") {
			t.Errorf("%s body=%s", m, rr.Body.String())
		}
	}
}

func TestAPIAlertChannelTestIndexOK(t *testing.T) {
	srv, _ := alertTestServer(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	body := `{"channels":[{"kind":"dingtalk","enabled":true,"webhook":"` + ts.URL + `"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/alert-channels/test", strings.NewReader(`{"index":0}`))
	req2.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannelTest(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr2.Code, rr2.Body.String())
	}
	var out struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !out.OK {
		t.Fatalf("应 ok=true，实际 err=%s", out.Error)
	}
	if out.Channel != "dingtalk" {
		t.Fatalf("channel=%q", out.Channel)
	}
}

func TestAPIAlertChannelTestIndexOutOfRange(t *testing.T) {
	srv, _ := alertTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/alert-channels/test", strings.NewReader(`{"index":99}`))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannelTest(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "通道下标越界") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAPIAlertChannelTestDirectSMTP(t *testing.T) {
	srv, _ := alertTestServer(t)
	body := `{"kind":"smtp","host":"h","port":25,"username":"u","password":"p","from":"a@b","to":"c@d"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/alert-channels/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannelTest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.OK {
		t.Fatal("smtp 应 ok=false")
	}
	if !strings.Contains(out.Error, "SMTP 通道尚未实现") {
		t.Fatalf("error=%q", out.Error)
	}
}

func TestAPIAlertChannelTestDirectUnknownKind(t *testing.T) {
	srv, _ := alertTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/alert-channels/test", strings.NewReader(`{"kind":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannelTest(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "未知的告警通道类型") {
		t.Fatalf("body=%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "nope") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAPIAlertChannelTestMethodNotAllowed(t *testing.T) {
	srv, _ := alertTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alert-channels/test", nil)
	srv.handleAPIAlertChannelTest(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "仅支持 POST") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestAPIAlertChannelsNoSubManager(t *testing.T) {
	srv := New(Config{
		Engine: &convert.Engine{
			RendererFor: func(string) (render.Renderer, error) { return nil, nil },
			Profile:     render.Profile{Name: "test", Version: "test-version"},
		},
		Logger:  quietLogger(),
		Timeout: 5 * time.Second,
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/alert-channels", nil)
	srv.handleAPIAlertChannels(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET status=%d", rr.Code)
	}
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/alert-channels/test", strings.NewReader(`{"kind":"smtp"}`))
	req2.Header.Set("Content-Type", "application/json")
	srv.handleAPIAlertChannelTest(rr2, req2)
	if rr2.Code != http.StatusServiceUnavailable {
		t.Fatalf("test status=%d", rr2.Code)
	}
}
