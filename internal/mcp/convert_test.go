package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
)

func newTestEngine(t *testing.T) *convert.Engine {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzcw==@10.0.0.1:8388#N1" + "\n"))
	}))
	t.Cleanup(srv.Close)
	fetcher := fetch.New(fetch.WithConfig(fetch.Config{
		UserAgent: "subforge-test",
		Timeout:   5 * time.Second,
		Parallel:  4,
		Retries:   0,
		Backoff:   0,
		MaxBody:   64 << 20,
		Redirects: 3,
	}))
	return &convert.Engine{
		Fetcher: fetcher,
		RendererFor: func(target string) (render.Renderer, error) {
			return registry.Lookup(target, clashmeta.Options{MixedPort: 7890})
		},
		Profile: render.Profile{Name: "test", Version: "test-version"},
	}
}

func TestCallConvert_InvalidJSON(t *testing.T) {
	s := NewServer(convert.Engine{})
	res, err := s.callConvert(context.Background(), json.RawMessage("{"))
	if err == nil {
		t.Fatal("illegal JSON should error")
	}
	if res != nil {
		t.Fatalf("res should be nil, got %+v", res)
	}
	if !strings.Contains(err.Error(), "解析参数") {
		t.Fatalf("error missing phrase: %v", err)
	}
}

func TestCallConvert_EngineFail(t *testing.T) {
	s := NewServer(convert.Engine{})
	res, err := s.callConvert(context.Background(), json.RawMessage("{\"urls\":[],\"target\":\"clash.meta\"}"))
	if err != nil {
		t.Fatalf("fail path should not return error: %v", err)
	}
	if res == nil {
		t.Fatal("res should not be nil")
	}
	if !res.IsError {
		t.Fatal("IsError should be true")
	}
	if len(res.Content) == 0 || res.Content[0].Type != "text" {
		t.Fatalf("content bad: %+v", res.Content)
	}
	if !strings.HasPrefix(res.Content[0].Text, "转换失败: ") {
		t.Fatalf("Text bad: %q", res.Content[0].Text)
	}
}

func TestCallConvert_Success(t *testing.T) {
	e := newTestEngine(t)
	s := NewServer(*e)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ss://YWVzLTI1Ni1nY206cGFzcw==@10.0.0.1:8388#N1" + "\n"))
	}))
	t.Cleanup(srv.Close)
	args := "{\"urls\":[\"" + srv.URL + "\"],\"target\":\"clash.meta\"}"
	res, err := s.callConvert(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("success path should not error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("res bad: %+v", res)
	}
	if len(res.Content) == 0 || res.Content[0].Type != "text" {
		t.Fatalf("content bad: %+v", res.Content)
	}
	_ = time.Second
}
