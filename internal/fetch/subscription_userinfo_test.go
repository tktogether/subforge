package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubscriptionUserinfo_Collected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=100; download=200; total=300; expire=1700000000")
		w.Write([]byte("ss://YWVzLTEyOC1nY206cGFzc0BleGFtcGxlLmNvbToxMjM0#node1\n"))
	}))
	defer srv.Close()

	f := New()
	res := f.FetchFrom(context.Background(), Source{URL: srv.URL})
	if res.Err != nil {
		t.Fatalf("res.Err: %v", res.Err)
	}
	if res.SubscriptionUserinfo != "upload=100; download=200; total=300; expire=1700000000" {
		t.Fatalf("SubscriptionUserinfo = %q", res.SubscriptionUserinfo)
	}
}

func TestSubscriptionUserinfo_Missing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ss://YWVzLTEyOC1nY206cGFzc0BleGFtcGxlLmNvbToxMjM0#node1\n"))
	}))
	defer srv.Close()

	f := New()
	res := f.FetchFrom(context.Background(), Source{URL: srv.URL})
	if res.Err != nil {
		t.Fatalf("res.Err: %v", res.Err)
	}
	if res.SubscriptionUserinfo != "" {
		t.Fatalf("SubscriptionUserinfo = %q, want empty", res.SubscriptionUserinfo)
	}
}

func TestSubscriptionUserinfo_FromCacheEmpty(t *testing.T) {
	body := []byte("ss://YWVzLTEyOC1nY206cGFzc0BleGFtcGxlLmNvbToxMjM0#node1\n")
	resp := &http.Response{StatusCode: 304, Header: http.Header{}}
	f := New()
	res, err := f.finish(Source{}, body, true, resp)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if !res.FromCache {
		t.Fatal("FromCache should be true")
	}
	if res.SubscriptionUserinfo != "" {
		t.Fatalf("SubscriptionUserinfo = %q, want empty", res.SubscriptionUserinfo)
	}
}

// TestFallbackUAForDegradedClash 部分机场按 UA 只返回 node-only 精简版；
// 缺少流量头时用 Clash.Meta 标识补取完整配置。
func TestFallbackUAForDegradedClash(t *testing.T) {
	var uas []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uas = append(uas, r.UserAgent())
		if r.UserAgent() == fallbackClashUserAgent {
			w.Header().Set("Subscription-Userinfo", "upload=1; download=2; total=3; expire=4")
			_, _ = w.Write([]byte("proxies:\n  - name: n1\n    type: ss\n    server: a.example.com\n    port: 1\nrules:\n  - MATCH,DIRECT\nproxy-groups:\n  - name: g\n    type: select\n    proxies: [n1]\n"))
			return
		}
		_, _ = w.Write([]byte("proxies:\n  - name: n1\n    type: ss\n    server: a.example.com\n    port: 1\n"))
	}))
	defer srv.Close()

	cfg := fastCfg()
	cfg.UserAgent = "mihomo/1.19.31 SubForge/test"
	cfg.FallbackUserAgent = fallbackClashUserAgent
	f := New(WithConfig(cfg))
	res := f.FetchFrom(context.Background(), Source{URL: srv.URL})
	if res.Err != nil {
		t.Fatalf("FetchFrom: %v", res.Err)
	}
	if res.SubscriptionUserinfo == "" {
		t.Fatal("回退 UA 后应拿到流量头")
	}
	if !strings.Contains(string(res.Content), "rules:") {
		t.Fatalf("回退 UA 后应拿到完整配置: %s", res.Content)
	}
	if len(uas) != 2 || uas[0] == fallbackClashUserAgent || uas[1] != fallbackClashUserAgent {
		t.Fatalf("请求 UA 序列不符: %v", uas)
	}
}
