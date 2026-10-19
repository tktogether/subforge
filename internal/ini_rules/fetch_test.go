package ini_rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tktogether/subforge/internal/fetch"
)

// fetchStubHandler 喂一段最小规则文本，供透传测试使用。
func fetchStubHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("vless://00000000-0000-0000-0000-000000000000@example.com:443?encryption=none#n1\n"))
	}
}

// TestNewRuleFetcherFetch 透传语义：RuleFetcher.Fetch 原样走 *fetch.Fetcher。
func TestNewRuleFetcherFetch(t *testing.T) {
	srv := httptest.NewServer(fetchStubHandler())
	defer srv.Close()
	f := NewRuleFetcher(fetch.New())
	results, err := f.Fetch(context.Background(), []fetch.Source{{Name: "a", URL: srv.URL}})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results 数 = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("Result.Err = %v", results[0].Err)
	}
	if len(results[0].Content) == 0 {
		t.Fatal("Content 为空")
	}
}
