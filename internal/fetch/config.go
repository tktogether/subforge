package fetch

import "time"

// Config 是 Fetcher 的行为参数。
type Config struct {
	// UserAgent 请求 User-Agent。
	UserAgent string
	// FallbackUserAgent 是缺少流量头时补取一次完整配置的备用 UA；
	// 为空表示不补取。
	FallbackUserAgent string
	// Timeout 单次请求上限（含重定向），超时后按错误路径重试。
	Timeout time.Duration
	// Parallel 并发上限。
	Parallel int
	// Retries 失败重试次数（不含首次尝试）。
	Retries int
	// Backoff 首次重试间隔，随后指数退避并加抖动。
	Backoff time.Duration
	// MaxBody 单源响应体积上限（字节），防止异常响应撑爆内存。
	MaxBody int64
	// Redirects 最大重定向次数。
	Redirects int
}

// DefaultConfig 给出保守默认值：上游是机场上游，网络抖动大，故超时与重试都偏宽。
func DefaultConfig() Config {
	return Config{
		UserAgent:         DefaultUserAgent(),
		FallbackUserAgent: fallbackClashUserAgent,
		Timeout:           30 * time.Second,
		Parallel:          8,
		Retries:           2,
		Backoff:           250 * time.Millisecond,
		MaxBody:           64 << 20,
		Redirects:         5,
	}
}
