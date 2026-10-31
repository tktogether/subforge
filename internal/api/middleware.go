// 中间件：请求 ID、日志、令牌鉴权与统一响应头。
package api

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// responseHeaders 给每个响应加上固定的元信息。
// Cache-Control 是 no-store：转换结果里通常带着节点凭证，不该被代理或浏览器落盘。
func responseHeaders(name string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Server", name)
			next.ServeHTTP(w, r)
		})
	}
}

// requestLogger 记录一行结构化日志：method path status 耗时 字节 请求 ID 客户端。
// 手写而不是用 chi 的 Logger，是为了把请求 ID 打进日志并在响应头回显。
func requestLogger(l *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// chi 的 RequestID 只把 ID 放进 context，不会写回请求头，所以必须从 context 取。
			// 在 handler 写出任何字节之前设置，否则 Header().Set 是空操作。
			id := chimw.GetReqID(r.Context())
			if id == "" {
				id = r.Header.Get("X-Request-ID")
			}
			if id != "" {
				w.Header().Set("X-Request-ID", id)
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			l.Printf("%s %s -> %d (%d B, %s, rid=%s) %s",
				r.Method, redactPath(r.URL.Path), rec.status, rec.bytes,
				time.Since(start).Round(time.Microsecond), dash(id), clientOf(r))
		})
	}
}

// statusRecorder 捕获状态码与写出字节数。
type statusRecorder struct {
	http.ResponseWriter
	status    int
	bytes     int
	wroteOnce bool
}

// WriteHeader 只透传第一次调用；超时中间件在 handler 返回后可能再写一次，
// 忽略掉以免日志里出现 superfluous WriteHeader 的告警。
func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteOnce {
		return
	}
	r.wroteOnce = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// auth 在校验令牌时逐个请求取当前值，因此令牌可以在运行期热更换。
// 令牌可以走 Authorization: Bearer、X-Token 头，或 query 的 token（subconverter 习惯）。
func auth(tokenFor func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := tokenFor()
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			if tokenMatch(token, presented(r)) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", "Bearer realm=\"subforge\"")
			http.Error(w, "401: 缺少或错误的访问令牌\n", http.StatusUnauthorized)
		})
	}
}

// presented 取出客户端提供的令牌，优先级：Authorization > X-Token > query。
func presented(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		scheme, rest, ok := strings.Cut(h, " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(rest)
		}
		return h
	}
	if v := r.Header.Get("X-Token"); v != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// tokenMatch 恒定时间比较，避免按字节提前退出泄漏令牌长度。
func tokenMatch(expected, got string) bool {
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func clientOf(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return fmt.Sprintf("%s ua=%q", host, r.UserAgent())
}
