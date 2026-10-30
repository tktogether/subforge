package netguard

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fc00::1", "fe80::1", "ff02::1"}
	for _, raw := range blocked {
		if !IsBlockedIP(net.ParseIP(raw)) {
			t.Fatalf("%s 应被拒绝", raw)
		}
	}
	allowed := []string{"1.1.1.1", "8.8.8.8", "104.21.26.21", "2606:4700:4700::1111"}
	for _, raw := range allowed {
		if IsBlockedIP(net.ParseIP(raw)) {
			t.Fatalf("%s 不应被拒绝", raw)
		}
	}
	if !IsBlockedIP(nil) {
		t.Fatal("nil IP 应被拒绝")
	}
}

func TestNewHTTPClientBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	blocked := NewHTTPClient(3*time.Second, false)
	if _, err := blocked.Get(srv.URL); err == nil {
		t.Fatal("默认应拒绝 127.0.0.1")
	}
	allowed := NewHTTPClient(3*time.Second, true)
	resp, err := allowed.Get(srv.URL)
	if err != nil {
		t.Fatalf("allowPrivate 应允许本地：%v", err)
	}
	resp.Body.Close()
}

func TestValidateHTTPURL(t *testing.T) {
	for _, raw := range []string{"http://example.com/a", "https://example.com:8443/a?b=1"} {
		if err := ValidateHTTPURL(raw); err != nil {
			t.Fatalf("%s 应合法：%v", raw, err)
		}
	}
	for _, raw := range []string{"", "ftp://example.com/a", "file:///etc/passwd", "http:///nohost", "not-a-url"} {
		if err := ValidateHTTPURL(raw); err == nil {
			t.Fatalf("%s 应被拒绝", raw)
		}
	}
}

func TestBlockedClientCanceledContext(t *testing.T) {
	c := NewHTTPClient(time.Second, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1/", nil)
	if _, err := c.Do(req); err == nil {
		t.Fatal("取消的 context 应失败")
	}
}
