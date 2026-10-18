package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
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
