// Package api 提供 HTTP 入口：路由、中间件与处理器。
//
// 转换流程不在这个包里——它只把 query 参数交给 internal/convert 的 Engine。
// CLI 与 HTTP 因此共用同一条执行路径，两边不会漂移。
package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/convert"
)

// Config 是启动一个 Server 需要的全部信息。
type Config struct {
	// Engine 执行体；必填。Fetcher 与渲染器工厂由调用方按来源（CLI / serve）组装。
	Engine *convert.Engine
	// Token 取当前访问令牌，返回空串表示不鉴权。
	// 用函数而不是字符串：令牌支持热重载（改配置文件即生效，不必重启）。
	Token func() string
	// Timeout 单次请求上限（含上游拉取）。默认 60s。
	Timeout time.Duration
	// HTTPClient 拉取远程规则文件用（/getruleset 的 url 参数）；为空时按 Timeout 造一个。
	HTTPClient *http.Client
	// Logger 日志目标；为空时用 log.Default()。
	Logger *log.Logger
	// StartedAt 服务启动时刻，用于 /version 的 uptime；零值取构造时间。
	StartedAt time.Time
	// Name 服务标识，写入响应头。
	Name string
}

// Server 是 HTTP 服务。
type Server struct {
	cfg       Config
	engine    *convert.Engine
	logger    *log.Logger
	startedAt time.Time
	http      *http.Client
}

// 默认请求超时：上游订阅偶尔很慢，但一个请求不该挂住连接池。
const defaultTimeout = 60 * time.Second

// New 构造 Server；Engine 缺失直接 panic，因为缺了它 Server 什么都做不了。
func New(cfg Config) *Server {
	if cfg.Engine == nil {
		panic("api: Engine 不能为空")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.StartedAt.IsZero() {
		cfg.StartedAt = time.Now().UTC()
	}
	if cfg.Name == "" {
		cfg.Name = "subforge"
	}
	if cfg.Token == nil {
		// 没给令牌函数就等于不鉴权；中间件里直接调用函数，这里先兜底。
		cfg.Token = func() string { return "" }
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &Server{
		cfg: cfg, engine: cfg.Engine, logger: cfg.Logger,
		startedAt: cfg.StartedAt, http: cfg.HTTPClient,
	}
}

// Routes 返回挂好全部中间件与路由的 handler。
//
// 路径名对齐 subconverter：/sub 是主入口，/profile 与 /render 是常见别名，
// 老配置换过来不用改路径。
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(requestLogger(s.logger))
	r.Use(chimw.Recoverer)
	r.Use(auth(s.cfg.Token))
	r.Use(chimw.Timeout(s.cfg.Timeout))
	r.Use(responseHeaders(s.cfg.Name))

	r.Get("/", s.handleIndex)
	r.Get("/version", s.handleVersion)
	r.Get("/healthz", s.handleHealth)

	r.Get("/sub", s.handleConvert)
	r.Get("/profile", s.handleConvert)
	r.Get("/render", s.handleConvert)

	r.Get("/getruleset", s.handleGetRuleset)
	r.Get("/inspect", s.handleInspect)

	r.NotFound(s.handleNotFound)
	return r
}

// handleIndex 给出一个可读的服务说明，顺带确认服务活着。
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		s.writeText(w, http.StatusOK, "text/plain; charset=utf-8",
			"subforge - 订阅转换 + 分流规则一体化引擎\n\n"+
				"  GET /sub          转换订阅（query 参数见 /inspect）\n"+
				"  GET /getruleset   规则集输出（type=1~6，见规则集文档）\n"+
				"  GET /profile      /sub 的别名\n"+
				"  GET /render       /sub 的别名\n"+
				"  GET /version      版本与运行时长\n"+
				"  GET /healthz      存活探针\n"+
				"  GET /inspect      回显解析后的参数（排查用）\n")
		return
	}
	s.handleNotFound(w, r)
}

// handleVersion 返回版本与运行时长。
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":      s.cfg.Name,
		"version":   s.engine.Profile.Version,
		"startedAt": s.startedAt.Format(time.RFC3339),
		"uptime":    time.Since(s.startedAt).String(),
		"time":      time.Now().UTC().Format(time.RFC3339),
	})
}

// handleHealth 是存活探针，不做任何重活。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// handleNotFound 统一 404 形状。
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.writeText(w, http.StatusNotFound, "text/plain; charset=utf-8", "404: 路径不存在\n")
}

// writeJSON 写出 JSON 响应。
func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeText 写出文本响应。
func (s *Server) writeText(w http.ResponseWriter, status int, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
