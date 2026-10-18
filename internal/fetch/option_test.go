package fetch

import (
	"net/http"
	"testing"
	"time"
)

func TestWithHTTPClient(t *testing.T) {
	c := &http.Client{Timeout: 7 * time.Second}
	f := &Fetcher{}
	WithHTTPClient(c)(f)
	if f.client != c {
		t.Fatalf("client 未被设置")
	}
	// 也经由 New 路径验证
	f2 := New(WithHTTPClient(c))
	if f2.client != c {
		t.Fatalf("New(WithHTTPClient) 未设置 client")
	}
}
