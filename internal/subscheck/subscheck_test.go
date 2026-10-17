package subscheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheck_AllOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	urls := []string{srv.URL, srv.URL, srv.URL}
	results := Check(context.Background(), urls, Options{Concurrency: 2, Timeout: 5 * time.Second})
	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}
	ok, fail := Summary(results)
	if ok != 3 || fail != 0 {
		t.Fatalf("ok=%d fail=%d, want 3/0", ok, fail)
	}
}

func TestCheck_Mixed(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okSrv.Close()
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badSrv.Close()
	urls := []string{okSrv.URL, badSrv.URL}
	results := Check(context.Background(), urls, Options{Concurrency: 2, Timeout: 5 * time.Second})
	ok, fail := Summary(results)
	if ok != 1 || fail != 1 {
		t.Fatalf("ok=%d fail=%d, want 1/1", ok, fail)
	}
}

func TestCheck_ConcurrencyCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	urls := make([]string, 20)
	for i := range urls {
		urls[i] = srv.URL
	}
	opts := Options{Concurrency: 100, Timeout: 5 * time.Second}
	opts = opts.normalize()
	if opts.Concurrency != 8 {
		t.Fatalf("concurrency = %d, want 8", opts.Concurrency)
	}
	results := Check(context.Background(), urls, opts)
	ok, _ := Summary(results)
	if ok != 20 {
		t.Fatalf("ok = %d, want 20", ok)
	}
}

func TestCheck_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	results := Check(context.Background(), []string{srv.URL}, Options{Timeout: 50 * time.Millisecond})
	if results[0].OK {
		t.Fatal("expected timeout failure")
	}
	if results[0].Err == nil {
		t.Fatal("expected error")
	}
}
