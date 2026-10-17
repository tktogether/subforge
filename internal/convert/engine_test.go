package convert

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
)

// newEngine 起一个只依赖本地 mock 上游的 Engine。
// 退避与重试都关掉：聚合用例不该被 sleep 拖长，失败路径也要求立即返回。
func newEngine(t *testing.T) *Engine {
	t.Helper()
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test",
		Timeout:   5 * time.Second,
		Parallel:  4,
		Retries:   0,
		Backoff:   0,
		MaxBody:   64 << 20,
		Redirects: 3,
	}))
	return &Engine{
		Fetcher: fetcher,
		RendererFor: func(target string) (render.Renderer, error) {
			return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
		},
		Profile: render.Profile{Name: "test", Version: "test-version"},
	}
}

// ssLink 生成一条 ss 分享链接。server 各不相同，去重指纹才判为不同节点。
func ssLink(host string, port int, name string) string {
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:pass"))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, host, port, name)
}

// subServer 起一个吐固定链接列表的假上游。
func subServer(t *testing.T, lines ...string) *httptest.Server {
	t.Helper()
	body := []byte(strings.Join(lines, "\n") + "\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestEngine_MultiURLAggregate 三个订阅聚合成一次转换：节点数是各订阅之和，
// 来源名逐个带进节点，全部成功时没有失败项。这是 §1.3「3 订阅聚合为 1 URL」的验收。
func TestEngine_MultiURLAggregate(t *testing.T) {
	srvA := subServer(t,
		ssLink("10.0.0.1", 8388, "A-01"),
		ssLink("10.0.0.2", 8388, "A-02"),
	)
	srvB := subServer(t,
		ssLink("10.0.1.1", 8388, "B-01"),
	)
	srvC := subServer(t,
		ssLink("10.0.2.1", 8388, "C-01"),
		ssLink("10.0.2.2", 8388, "C-02"),
		ssLink("10.0.2.3", 8388, "C-03"),
	)

	e := newEngine(t)
	res, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{srvA.URL, srvB.URL, srvC.URL},
		Raw:    map[string][]string{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.RawNodes != 6 || res.Nodes != 6 {
		t.Errorf("节点数 Raw=%d Nodes=%d，want 6/6", res.RawNodes, res.Nodes)
	}
	if len(res.Failures) != 0 {
		t.Errorf("全部成功时不应有失败项: %v", res.Failures)
	}
	if len(res.Cached) != 3 {
		t.Errorf("Cached 长度 = %d, want 3（每个成功来源一条）", len(res.Cached))
	}
	if !strings.Contains(string(res.Body), "A-01") || !strings.Contains(string(res.Body), "C-03") {
		t.Error("产物应同时包含各订阅的节点")
	}
}

// TestEngine_PartialFailure 一个来源 404 时其余来源仍产出：失败原因记进 Failures，
// 成功节点照常渲染。这是 §1.3「部分失败容错」的验收。
func TestEngine_PartialFailure(t *testing.T) {
	ok1 := subServer(t, ssLink("10.1.0.1", 8388, "OK-01"), ssLink("10.1.0.2", 8388, "OK-02"))
	ok2 := subServer(t, ssLink("10.2.0.1", 8388, "OK-03"))
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(bad.Close)

	e := newEngine(t)
	res, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{ok1.URL, bad.URL, ok2.URL},
		Raw:    map[string][]string{},
	})
	if err != nil {
		t.Fatalf("部分失败不应整体报错: %v", err)
	}
	if res.Nodes != 3 {
		t.Errorf("Nodes = %d, want 3（两个成功来源之和）", res.Nodes)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %v, want 1 条", res.Failures)
	}
	if !strings.Contains(res.Failures[0], bad.URL) {
		t.Errorf("失败原因应带上来源地址: %q", res.Failures[0])
	}
	if !strings.Contains(string(res.Body), "OK-03") {
		t.Error("失败来源不应拖垮成功来源的产物")
	}
}

// TestEngine_AllFail 全部来源失败时整体报错，并把每条原因带出来。
func TestEngine_AllFail(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(bad.Close)

	e := newEngine(t)
	_, err := e.Run(context.Background(), Params{
		Target: "clash.meta",
		URLs:   []string{bad.URL},
		Raw:    map[string][]string{},
	})
	if err == nil {
		t.Fatal("全部失败时应报错")
	}
	if !strings.Contains(err.Error(), bad.URL) {
		t.Errorf("错误信息应带上失败来源: %v", err)
	}
}
