package fetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/parse"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("读取 fixture %s: %v", name, err)
	}
	return buf
}

// fastCfg 是测试专用配置：退避为 0，避免用例被 sleep 拖长。
func fastCfg() Config {
	return Config{
		UserAgent: "subforge-test",
		Timeout:   5 * time.Second,
		Parallel:  4,
		Retries:   2,
		Backoff:   0,
		MaxBody:   64 << 20,
		Redirects: 3,
	}
}

// TestFetchFrom_ClientFor 按来源出网选择取客户端；取客户端失败与返回 nil 都要正确处理。
func TestFetchFrom_ClientFor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(readFixture(t, "mixed-subscription.txt"))
	}))
	t.Cleanup(srv.Close)

	f := New(WithConfig(fastCfg()))
	var got []string
	f.ClientFor = func(_ context.Context, outbound string) (*http.Client, error) {
		got = append(got, outbound)
		return http.DefaultClient, nil
	}
	res := f.FetchFrom(context.Background(), Source{URL: srv.URL, Outbound: "direct"})
	if res.Err != nil {
		t.Fatalf("FetchFrom: %v", res.Err)
	}
	if len(got) == 0 || got[0] != "direct" {
		t.Fatalf("ClientFor 入参 = %v", got)
	}

	f.ClientFor = func(context.Context, string) (*http.Client, error) {
		return nil, errors.New("没有可用节点")
	}
	res = f.FetchFrom(context.Background(), Source{URL: srv.URL, Outbound: "1"})
	if res.Err == nil || !strings.Contains(res.Err.Error(), "没有可用节点") {
		t.Fatalf("应返回取客户端错误: %v", res.Err)
	}

	f.ClientFor = func(context.Context, string) (*http.Client, error) { return nil, nil }
	res = f.FetchFrom(context.Background(), Source{URL: srv.URL})
	if res.Err != nil {
		t.Fatalf("ClientFor 返回 nil 应回落到默认客户端: %v", res.Err)
	}
}

func TestFetchFrom_LinksFormat(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	fixed := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)

	var gotUA, gotIno string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotIno = r.Header.Get("If-None-Match")
		w.Header().Set("Etag", `"v1"`)
		w.Header().Set("Last-Modified", fixed.UTC().Format(http.TimeFormat))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	f := New(WithConfig(fastCfg()))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err != nil {
		t.Fatalf("FetchFrom: %v", res.Err)
	}
	if res.HTTPCode != http.StatusOK || res.FromCache {
		t.Fatalf("HTTPCode=%d FromCache=%v", res.HTTPCode, res.FromCache)
	}
	if res.Sniff.Format != parse.FormatLinks {
		t.Fatalf("Sniff.Format=%v, want links", res.Sniff.Format)
	}
	if res.Etag != `"v1"` {
		t.Fatalf("Etag=%q", res.Etag)
	}
	if !res.Modified.Equal(fixed) {
		t.Fatalf("Modified=%v, want %v", res.Modified, fixed)
	}
	if res.Size != len(body) || !bytes.Equal(res.Content, body) {
		t.Fatalf("内容不一致: Size=%d want=%d", res.Size, len(body))
	}
	if res.Source.Name != "上游A" {
		t.Fatalf("Source.Name=%q", res.Source.Name)
	}
	if gotUA != "subforge-test" {
		t.Fatalf("User-Agent=%q", gotUA)
	}
	if gotIno != "" {
		t.Fatalf("首次请求不应带 If-None-Match，实际 %q", gotIno)
	}
}

func TestFetchFrom_ETag304(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Etag", `"v1"`)
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url := srv.URL + "/sub"
	f := New(WithConfig(fastCfg()), WithCache(c))
	src := Source{Name: "上游A", URL: url}

	first := f.FetchFrom(context.Background(), src)
	if first.Err != nil || first.FromCache {
		t.Fatalf("首次拉取失败: FromCache=%v err=%v", first.FromCache, first.Err)
	}
	second := f.FetchFrom(context.Background(), src)
	if second.Err != nil {
		t.Fatalf("二次拉取失败: %v", second.Err)
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Fatalf("上游请求次数 = %d, want 2", n)
	}
	if !second.FromCache {
		t.Fatal("第二次应为缓存命中（FromCache）")
	}
	if second.HTTPCode != http.StatusNotModified {
		t.Fatalf("HTTPCode=%d, want 304", second.HTTPCode)
	}
	if !bytes.Equal(first.Content, second.Content) {
		t.Fatal("304 缓存内容应与首次响应一致")
	}
}

// TestFetchFrom_CacheSelfHeal 覆盖 304 但缓存原文缺失（缓存被清理/损坏）的自愈路径：
// 清掉条件标识后重试，不应把坏状态留在缓存里。
func TestFetchFrom_CacheSelfHeal(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Etag", `"v1"`)
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	url := srv.URL + "/sub"
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Save(url, body, cacheMeta{ETag: `"v1"`}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(c.contentPath(url)); err != nil { // 模拟缓存原文丢失
		t.Fatal(err)
	}

	f := New(WithConfig(fastCfg()), WithCache(c))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: url})
	if res.Err != nil {
		t.Fatalf("自愈后仍失败: %v", res.Err)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("上游请求次数 = %d, want 2（一次 304 + 一次回源）", atomic.LoadInt32(&hits))
	}
	if res.FromCache {
		t.Fatal("回源结果不应标记为缓存命中")
	}
	if etag, _ := c.Meta(url); etag != `"v1"` {
		t.Fatalf("回源后应重新写入元数据，实际 etag=%q", etag)
	}
}

func TestFetchFrom_RetryThenSucceed(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	f := New(WithConfig(fastCfg()))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err != nil {
		t.Fatalf("两次 503 后应重试成功: %v", res.Err)
	}
	if atomic.LoadInt32(&hits) != 3 {
		t.Fatalf("请求次数 = %d, want 3", atomic.LoadInt32(&hits))
	}
}

func TestFetchFrom_RetriesExhausted(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 1
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err == nil {
		t.Fatal("持续 503 应返回错误")
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("请求次数 = %d, want 2（1 次尝试 + 1 次重试）", atomic.LoadInt32(&hits))
	}
	if !strings.Contains(res.Err.Error(), "重试 1 次后仍失败") {
		t.Fatalf("错误消息缺少重试信息: %v", res.Err)
	}
}

// TestFetchFrom_PermanentNoRetry 永久错误（404/410/401/403）必须立即返回，不能重试。
func TestFetchFrom_PermanentNoRetry(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := New(WithConfig(fastCfg()))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err == nil {
		t.Fatal("404 应返回错误")
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("404 不应重试，实际请求 %d 次", n)
	}
	var pe *PermanentError
	if !errors.As(res.Err, &pe) {
		t.Fatalf("错误类型应为 PermanentError，实际 %T: %v", res.Err, res.Err)
	}
	if !strings.Contains(pe.Reason, "404") {
		t.Fatalf("Reason=%q", pe.Reason)
	}
}

func TestFetchFrom_MaxBody(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.MaxBody = 64
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err == nil || !strings.Contains(res.Err.Error(), "体积上限") {
		t.Fatalf("超过 MaxBody 应报体积上限错误，实际 %v", res.Err)
	}
}

func TestFetchFrom_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("   \n"))
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err == nil || !strings.Contains(res.Err.Error(), "响应内容为空") {
		t.Fatalf("空响应应报错，实际 %v", res.Err)
	}
}

// TestFetchFrom_Unrecognizable 内容无法识别时：报错误，但原文必须留在 Content 中，
// 便于上层输出诊断（而不是把用户订阅原文吞掉）。
func TestFetchFrom_Unrecognizable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world\nthis is not a subscription"))
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/sub"})
	if res.Err == nil {
		t.Fatal("无法识别的内容应报错")
	}
	if !strings.Contains(res.Err.Error(), "无法识别") {
		t.Fatalf("错误消息缺少形态识别说明: %v", res.Err)
	}
	if !strings.Contains(string(res.Content), "hello world") {
		t.Fatal("原文应保留在 Result.Content 中")
	}
	if res.HTTPCode != http.StatusOK || res.Size == 0 {
		t.Fatalf("HTTPCode=%d Size=%d", res.HTTPCode, res.Size)
	}
}

func TestFetchFrom_ClashYAML(t *testing.T) {
	yaml := []byte("mixed-port: 7890\nproxies:\n" +
		"  - name: ss-one\n    type: ss\n    server: a.example.com\n    port: \"8388\"\n    cipher: aes-256-gcm\n    password: p1\n" +
		"  - name: vmess-one\n    type: vmess\n    server: b.example.com\n    port: \"443\"\n    uuid: 11111111-2222-3333-4444-555555555555\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(yaml)
	}))
	defer srv.Close()

	f := New(WithConfig(fastCfg()))
	res := f.FetchFrom(context.Background(), Source{Name: "上游A", URL: srv.URL + "/cfg.yaml"})
	if res.Err != nil {
		t.Fatalf("FetchFrom: %v", res.Err)
	}
	if res.Sniff.Format != parse.FormatClashYAML {
		t.Fatalf("Sniff.Format=%v, want clash-yaml", res.Sniff.Format)
	}
	nodes, err := parse.DecodeSubscription(res.Content)
	if err != nil {
		t.Fatalf("DecodeSubscription: %v", err)
	}
	if len(nodes) != 2 || parse.NameOf(nodes[0]) != "ss-one" {
		t.Fatalf("节点数/节点名: %d / %q", len(nodes), parse.NameOf(nodes[0]))
	}
}

func TestFetchFrom_BadURL(t *testing.T) {
	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	for _, url := range []string{"://bad", ""} {
		res := f.FetchFrom(context.Background(), Source{Name: "坏URL", URL: url})
		if res.Err == nil {
			t.Fatalf("URL %q 应报错", url)
		}
		if !strings.Contains(res.Err.Error(), "坏URL") {
			t.Fatalf("错误消息应含来源名: %v", res.Err)
		}
	}
}

func TestFetchFrom_AuthHeader(t *testing.T) {
	var gotAuth, gotExtra string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotExtra = r.Header.Get("X-Token")
		_, _ = w.Write(readFixture(t, "mixed-subscription.txt"))
	}))
	defer srv.Close()

	f := New(WithConfig(fastCfg()))
	res := f.FetchFrom(context.Background(), Source{
		Name: "上游A",
		URL:  srv.URL + "/sub",
		Auth: map[string]string{"Authorization": "Bearer s3cret", "X-Token": "abc"},
	})
	if res.Err != nil {
		t.Fatalf("FetchFrom: %v", res.Err)
	}
	if gotAuth != "Bearer s3cret" || gotExtra != "abc" {
		t.Fatalf("附加请求头未透传: %q %q", gotAuth, gotExtra)
	}
}

func TestFetch_PartialFailure(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "dead") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	sources := []Source{
		{Name: "ok-1", URL: srv.URL + "/alive"},
		{Name: "dead", URL: srv.URL + "/dead"},
		{Name: "ok-2", URL: srv.URL + "/also-alive"},
	}
	results, err := f.Fetch(context.Background(), sources)
	if err == nil {
		t.Fatal("应返回错误")
	}
	var pe *PartialError
	if !errors.As(err, &pe) {
		t.Fatalf("错误类型应为 PartialError，实际 %T: %v", err, err)
	}
	if len(results) != 3 {
		t.Fatalf("结果数 = %d", len(results))
	}
	if results[0].Err != nil || results[2].Err != nil || results[1].Err == nil {
		t.Fatalf("失败位置不对: %v / %v / %v", results[0].Err, results[1].Err, results[2].Err)
	}
	if len(pe.Failed) != 1 || !strings.Contains(pe.Failed[0], "#1") {
		t.Fatalf("Failed=%v", pe.Failed)
	}
	if !strings.Contains(pe.Error(), "1 个来源拉取失败") {
		t.Fatalf("错误消息: %v", pe.Error())
	}
}

func TestFetch_AllFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	results, err := f.Fetch(context.Background(), []Source{
		{Name: "a", URL: srv.URL + "/a"}, {Name: "b", URL: srv.URL + "/b"},
	})
	if err == nil {
		t.Fatal("全部失败应返回错误")
	}
	var pe *PartialError
	if errors.As(err, &pe) {
		t.Fatalf("全部失败不应是 PartialError: %v", pe)
	}
	if !strings.Contains(err.Error(), "全部 2 个来源拉取失败") {
		t.Fatalf("错误消息: %v", err)
	}
	if len(results) != 2 || results[0].Err == nil || results[1].Err == nil {
		t.Fatalf("应返回完整结果集: %v", results)
	}
}

func TestFetch_AllOKAndOrder(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	f := New(WithConfig(fastCfg()))
	urls := []string{"/a", "/b", "/c", "/d"}
	sources := make([]Source, len(urls))
	for i, u := range urls {
		sources[i] = Source{Name: "上游-" + u, URL: srv.URL + u}
	}
	results, err := f.Fetch(context.Background(), sources)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(results) != len(sources) {
		t.Fatalf("结果数 = %d, want %d", len(results), len(sources))
	}
	for i, res := range results {
		if res.Source.URL != sources[i].URL {
			t.Errorf("结果 %d 来源错乱: %q vs %q", i, res.Source.URL, sources[i].URL)
		}
		if res.Err != nil || len(res.Content) == 0 {
			t.Errorf("结果 %d 异常: %v", i, res.Err)
		}
	}
}

func TestFetch_NoSources(t *testing.T) {
	f := New(WithConfig(fastCfg()))
	results, err := f.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("空来源列表不应报错: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("结果数 = %d, want 0", len(results))
	}
}

func TestFetch_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write(readFixture(t, "mixed-subscription.txt"))
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	res := f.FetchFrom(ctx, Source{Name: "慢上游", URL: srv.URL + "/sub"})
	elapsed := time.Since(start)
	if res.Err == nil {
		t.Fatal("超时应返回错误")
	}
	if !strings.Contains(res.Err.Error(), "context deadline") {
		t.Fatalf("错误应体现上下文取消: %v", res.Err)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("超时未生效，耗时 %v", elapsed)
	}
}

// TestFetch_MaxParallel 验证并发上限真的生效（并发超限会让上游被打爆）。
func TestFetch_MaxParallel(t *testing.T) {
	var inflight, peak int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if cur <= p || atomic.CompareAndSwapInt32(&peak, p, cur) {
				break
			}
		}
		time.Sleep(60 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		_, _ = w.Write(readFixture(t, "mixed-subscription.txt"))
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Parallel = 2
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	sources := make([]Source, 8)
	for i := range sources {
		sources[i] = Source{Name: "上游", URL: srv.URL + "/sub"}
	}
	results, err := f.Fetch(context.Background(), sources)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	for i, res := range results {
		if res.Err != nil {
			t.Errorf("结果 %d 异常: %v", i, res.Err)
		}
	}
	if p := atomic.LoadInt32(&peak); p > 2 {
		t.Fatalf("瞬时并发 = %d, 超过 Parallel=%d", p, cfg.Parallel)
	}
}

func TestFetch_RedirectWithinLimit(t *testing.T) {
	body := readFixture(t, "mixed-subscription.txt")
	var hops int32
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer final.Close()
	jumper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.StoreInt32(&hops, 1)
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer jumper.Close()

	cfg := fastCfg()
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{Name: "跳转上游", URL: jumper.URL + "/first"})
	if res.Err != nil {
		t.Fatalf("一次重定向应在限制内: %v", res.Err)
	}
	if atomic.LoadInt32(&hops) != 1 || len(res.Content) != len(body) {
		t.Fatalf("hops=%d content=%d", atomic.LoadInt32(&hops), len(res.Content))
	}
}

func TestFetch_RedirectLimit(t *testing.T) {
	var hits int32
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 无限循环重定向
		atomic.AddInt32(&hits, 1)
		http.Redirect(w, r, srvURL+"/loop", http.StatusFound)
	}))
	defer srv.Close()
	srvURL = srv.URL

	cfg := fastCfg()
	cfg.Redirects = 2
	cfg.Retries = 0
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{Name: "跳转上游", URL: srvURL + "/start"})
	if res.Err == nil {
		t.Fatal("循环重定向应报错")
	}
	if !strings.Contains(res.Err.Error(), "重定向") {
		t.Fatalf("错误消息应体现重定向限制: %v", res.Err)
	}
	if n := atomic.LoadInt32(&hits); n < 3 {
		t.Fatalf("重定向应被跟随到上限才失败，实际访问 %d 次", n)
	}
}

// TestFetchFrom_SniffFailure_NoRetry 钉住「内容形态无法识别」不可重试。
// 嗅探是纯函数：同一份字节重试多少次结果都一样。曾经它按普通错误重试，
// 单条 ruleset 引用白等 3 次退避，INI 模板一次展开几十个引用就会撞上 API
// 的 60s 请求上限，把尾部规则静默截断（见 internal/ini_rules/parse.go 的
// ruleset 展开）。这里同时钉住：失败仍要保留原文，交给上层自行判断。
func TestFetchFrom_SniffFailure_NoRetry(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("DOMAIN-SUFFIX,ref-0.example\n"))
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.Retries = 3 // 若被当作可重试错误，这里会访问 4 次
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{URL: srv.URL + "/r/0.txt"})
	if res.Err == nil {
		t.Fatal("规则文本不是订阅内容形态，应识别失败并报错")
	}
	var pe *PermanentError
	if !errors.As(res.Err, &pe) {
		t.Fatalf("嗅探失败应标记为 PermanentError，实际: %v", res.Err)
	}
	if !strings.Contains(res.Err.Error(), "内容形态无法识别") {
		t.Fatalf("错误消息应保留识别失败原文，实际: %v", res.Err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("嗅探失败不应重试，实际请求 %d 次", got)
	}
	if string(res.Content) != "DOMAIN-SUFFIX,ref-0.example\n" {
		t.Fatalf("嗅探失败也要保留原文，实际 %q", string(res.Content))
	}
}
