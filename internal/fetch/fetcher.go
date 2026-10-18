package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tktogether/subforge/internal/parse"
)

// Source 是一个待拉取的订阅来源。
type Source struct {
	// Name 来源名（溯源与错误定位）；为空时用 URL 代替。
	Name string
	// URL 订阅地址。
	URL string
	// Auth 附加请求头（token 等），可选。
	Auth map[string]string
}

// label 返回错误消息里的来源标识。
func label(s Source) string {
	if s.Name != "" {
		return s.Name
	}
	return s.URL
}

// Result 是单个来源的拉取结果。
type Result struct {
	Source   Source
	Content  []byte            // 拉取到的原文
	Sniff    parse.SniffResult // 内容形态识别结果（失败时为零值）
	HTTPCode int
	Etag     string
	Modified time.Time
	// SubscriptionUserinfo 是上游 subscription-userinfo 响应头原文（upload/download/total/expire）。
	// 缓存命中（304）或上游未提供时为空字符串；调用方不得用空值覆盖已采集到的流量数据。
	SubscriptionUserinfo string
	FromCache            bool
	Size                 int
	Err                  error
}

// PermanentError 标记不可重试的错误（资源下线、鉴权失败等）。
type PermanentError struct{ Reason string }

func (e *PermanentError) Error() string { return e.Reason }

// PartialError 表示部分来源失败；调用方可直接使用成功的结果。
type PartialError struct {
	// Failed 每个失败来源的 "序号: 原因"。
	Failed []string
}

func (e *PartialError) Error() string {
	return fmt.Sprintf("%d 个来源拉取失败: %s", len(e.Failed), strings.Join(e.Failed, "; "))
}

// Fetcher 并发拉取订阅来源，带重试与缓存。
type Fetcher struct {
	cfg    Config
	client *http.Client
	cache  *Cache
}

// Option 配置 Fetcher。
type Option func(*Fetcher)

// New 构造 Fetcher（默认参数 + 覆盖项）。
func New(opts ...Option) *Fetcher {
	f := &Fetcher{cfg: DefaultConfig()}
	for _, o := range opts {
		o(f)
	}
	if f.client == nil {
		f.client = f.newClient()
	}
	return f
}

func WithConfig(c Config) Option {
	return func(f *Fetcher) {
		f.cfg = c
		f.client = f.newClient()
	}
}

func WithHTTPClient(c *http.Client) Option {
	return func(f *Fetcher) { f.client = c }
}

func WithCache(c *Cache) Option {
	return func(f *Fetcher) { f.cache = c }
}

func (f *Fetcher) newClient() *http.Client {
	redirects := f.cfg.Redirects
	return &http.Client{
		Timeout: f.cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > redirects {
				return fmt.Errorf("超过最大重定向次数 %d", redirects)
			}
			return nil
		},
	}
}

// FetchFrom 拉取单个来源，按 Config.Retries 做指数退避重试；永久错误不重试。
func (f *Fetcher) FetchFrom(ctx context.Context, src Source) Result {
	var last error
	var lastRes Result
	for attempt := 0; attempt <= f.cfg.Retries; attempt++ {
		if attempt > 0 && !sleepBackoff(ctx, f.cfg.Backoff, attempt-1) {
			return errResult(Result{}, src, "", ctx.Err())
		}
		res, err := f.fetchOnce(ctx, src)
		if err == nil {
			return res
		}
		if isPermanent(err) {
			return errResult(res, src, "", err)
		}
		last = err
		if len(res.Content) > 0 || res.HTTPCode != 0 {
			lastRes = res
		}
	}
	suffix := fmt.Sprintf("（重试 %d 次后仍失败）", f.cfg.Retries)
	return errResult(lastRes, src, suffix, last)
}

// errResult 把错误挂到结果上，并保留 fetchOnce 已填好的诊断字段（HTTP 码、原文、缓存命中等），
// 避免重试包装层把上游响应内容丢掉——排障时这些原文往往正是关键证据。
func errResult(res Result, src Source, suffix string, err error) Result {
	msg := "拉取 " + label(src)
	if suffix != "" {
		msg += suffix
	}
	res.Source = src
	res.Err = fmt.Errorf("%s: %w", msg, err)
	return res
}

// Fetch 并发拉取多个来源，结果顺序与入参一致。
//
// 返回的 error 语义：
//   - nil：全部成功
//   - *PartialError：部分失败，成功的结果可直接使用
//   - 其他 error：全部失败
//
// 单个来源的失败原因见对应 Result.Err。
func (f *Fetcher) Fetch(ctx context.Context, sources []Source) ([]Result, error) {
	results := make([]Result, len(sources))
	sem := make(chan struct{}, f.cfg.Parallel)
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = f.FetchFrom(ctx, src)
		}(i, src)
	}
	wg.Wait()

	var failed []string
	for i, r := range results {
		if r.Err != nil {
			failed = append(failed, fmt.Sprintf("#%d %s", i, r.Err))
		}
	}
	if len(results) > 0 && len(failed) == len(results) {
		return results, fmt.Errorf("全部 %d 个来源拉取失败（首个: %s）", len(results), failed[0])
	}
	if len(failed) > 0 {
		return results, &PartialError{Failed: failed}
	}
	return results, nil
}

func (f *Fetcher) fetchOnce(ctx context.Context, src Source) (Result, error) {
	if src.URL == "" {
		return Result{}, errors.New("来源 URL 为空")
	}

	etag, modified := "", time.Time{}
	if f.cache != nil {
		etag, modified = f.cache.Meta(src.URL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", f.cfg.UserAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if !modified.IsZero() {
		req.Header.Set("If-Modified-Since", modified.UTC().Format(http.TimeFormat))
	}
	for k, v := range src.Auth {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		body, err := f.cache.Content(src.URL)
		if err != nil {
			// 缓存原文缺失或损坏：清掉条件标识，下一次重试不再带条件
			f.cache.ClearMeta(src.URL)
			return Result{}, fmt.Errorf("上游返回 304 但缓存原文缺失: %w", err)
		}
		return f.finish(src, body, true, resp)
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone ||
			resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return Result{}, &PermanentError{
				Reason: fmt.Sprintf("上游返回 %d（不可重试）", resp.StatusCode),
			}
		}
		return Result{}, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}

	// 不设 Accept-Encoding：由 http.Client 透明解压 gzip，拿到的始终是解码后的原文
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.cfg.MaxBody+1))
	if err != nil {
		return Result{}, fmt.Errorf("读取响应: %w", err)
	}
	if int64(len(body)) > f.cfg.MaxBody {
		return Result{}, fmt.Errorf("响应超过体积上限 %d 字节", f.cfg.MaxBody)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return Result{}, errors.New("响应内容为空")
	}

	if f.cache != nil {
		// 缓存失败不阻断本次结果
		_ = f.cache.Save(src.URL, body, cacheMeta{ETag: resp.Header.Get("Etag"), Modified: parseModified(resp)})
	}
	return f.finish(src, body, false, resp)
}

// finish 附加形态识别结果；识别失败返回错误（原文仍保留在 Result.Content 中）。
func (f *Fetcher) finish(src Source, body []byte, fromCache bool, resp *http.Response) (Result, error) {
	res := Result{
		Source:               src,
		Content:              body,
		HTTPCode:             resp.StatusCode,
		Etag:                 resp.Header.Get("Etag"),
		Modified:             parseModified(resp),
		SubscriptionUserinfo: resp.Header.Get("Subscription-Userinfo"),
		FromCache:            fromCache,
		Size:                 len(body),
	}
	sniff, err := parse.Sniff(body)
	res.Sniff = sniff
	if err != nil {
		res.Err = fmt.Errorf("内容形态无法识别: %w", err)
		return res, res.Err
	}
	return res, nil
}

func parseModified(resp *http.Response) time.Time {
	v := resp.Header.Get("Last-Modified")
	if v == "" {
		return time.Time{}
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return time.Time{}
	}
	return t
}

func isPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}

func sleepBackoff(ctx context.Context, base time.Duration, attempt int) bool {
	if base <= 0 {
		return ctx.Err() == nil
	}
	delay := base << attempt
	if delay > 8*time.Second {
		delay = 8 * time.Second
	}
	// 追加最多 25% 抖动：多实例同时重试时打散请求，避免惊群
	delay += time.Duration(mrand.Int64N(int64(delay)/4 + 1))
	select {
	case <-time.After(delay):
		return true
	case <-ctx.Done():
		return false
	}
}
