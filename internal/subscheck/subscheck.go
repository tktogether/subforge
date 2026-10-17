// Package subscheck 实现 M3.5 订阅测活：并发探测订阅 URL 的可达性与响应健康，
// 默认关闭，由调用方显式开启。
package subscheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DefaultConcurrency 是默认并发上限。
const DefaultConcurrency = 8

// DefaultTimeout 是单个 URL 的探测超时。
const DefaultTimeout = 15 * time.Second

// Options 探测参数。
type Options struct {
	Concurrency int           // 并发上限，<=0 取 DefaultConcurrency
	Timeout     time.Duration // 单 URL 超时，<=0 取 DefaultTimeout
	Method      string        // HTTP 方法，空则 GET
	Client      *http.Client  // 自定义 client，nil 则新建
}

func (o Options) normalize() Options {
	if o.Concurrency <= 0 {
		o.Concurrency = DefaultConcurrency
	}
	if o.Concurrency > 8 {
		o.Concurrency = 8
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.Method == "" {
		o.Method = http.MethodGet
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: o.Timeout}
	}
	return o
}

// Result 是单个 URL 的探测结果。
type Result struct {
	URL        string
	OK         bool
	StatusCode int
	Latency    time.Duration
	Bytes      int64
	Err        error
}

// Check 并发探测一组订阅 URL。
func Check(ctx context.Context, urls []string, opts Options) []Result {
	opts = opts.normalize()
	results := make([]Result, len(urls))
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = probe(ctx, url, opts)
		}(i, u)
	}
	wg.Wait()
	return results
}

func probe(ctx context.Context, url string, opts Options) Result {
	r := Result{URL: url}
	start := time.Now()
	reqCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, opts.Method, url, nil)
	if err != nil {
		r.Err = err
		r.Latency = time.Since(start)
		return r
	}
	resp, err := opts.Client.Do(req)
	r.Latency = time.Since(start)
	if err != nil {
		r.Err = err
		return r
	}
	defer resp.Body.Close()
	r.StatusCode = resp.StatusCode
	n, _ := io.Copy(io.Discard, resp.Body)
	r.Bytes = n
	r.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !r.OK {
		r.Err = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return r
}

// Summary 返回简要汇总。
func Summary(results []Result) (ok, fail int) {
	for _, r := range results {
		if r.OK {
			ok++
		} else {
			fail++
		}
	}
	return
}
