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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/scheduler"
	"github.com/tktogether/subforge/internal/subforge"
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
	// LogBuffer 日志环形缓冲区；用于 /api/logs。为空时构造默认容量。
	LogBuffer *LogBuffer
	// HistoryBuffer 转换历史环形缓冲区；用于 /api/history 与 /api/stats。为空时构造默认容量。
	HistoryBuffer *HistoryBuffer
	// StartedAt 服务启动时刻，用于 /version 的 uptime；零值取构造时间。
	StartedAt time.Time
	// Name 服务标识，写入响应头。
	Name string
	// DataDir 数据目录；用于订阅管理的 SQLite 数据库。
	// 为空时不启用订阅管理。
	DataDir string
}

// Server 是 HTTP 服务。
type Server struct {
	cfg        Config
	engine     *convert.Engine
	logger     *log.Logger
	startedAt  time.Time
	http       *http.Client
	logBuf     *LogBuffer
	historyBuf *HistoryBuffer
	subManager *subforge.Manager
	scheduler  *scheduler.Scheduler
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
	if cfg.LogBuffer == nil {
		cfg.LogBuffer = NewLogBuffer(0)
	}
	if cfg.HistoryBuffer == nil {
		cfg.HistoryBuffer = NewHistoryBuffer(0)
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	// 日志同时写原目标与环形缓冲区：/api/logs 靠它读最近的请求日志，
	// 而原 logger（stderr 或测试的 discard）保持不变。
	cfg.Logger = log.New(io.MultiWriter(cfg.Logger.Writer(), cfg.LogBuffer),
		cfg.Logger.Prefix(), cfg.Logger.Flags())
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
		logBuf: cfg.LogBuffer, historyBuf: cfg.HistoryBuffer,
	}
}

// SetSubManager 注入订阅管理器。
// 由调用方在创建 Server 后调用，避免在 New 里引入可选依赖。
func (s *Server) SetSubManager(m *subforge.Manager) {
	s.subManager = m
	// 控制台建的管线也要能被 pipeline=<name> 选中：引擎本身只认模板，
	// 不接这一步的话界面上列得出的名字拿去跑转换会直接报「未找到命名管线」。
	if s.engine != nil {
		s.engine.ExtraPipelines = s.uiPipelineSpecs
	}
	// 脚本库同样要注入：命名管线里的 ref=<脚本名> 只有拿到 resolver 才编译得出来。
	// 不接这一步，「脚本」页建的脚本在转换/订阅链路里 100% 报「脚本步骤需要脚本库支持」；
	// 顺带让 src= 内联脚本也真正受 script.enabled 这个安全开关约束（resolver 为 nil 时
	// resolveScriptSrc 会跳过启用检查）。
	if s.engine != nil {
		s.engine.ScriptResolver = m.ScriptResolver()
	}
}

// SetScheduler 注入定时调度器，供 GET /api/tasks 读取执行历史。
func (s *Server) SetScheduler(sch *scheduler.Scheduler) { s.scheduler = sch }

// Routes 返回挂好全部中间件与路由的 handler。
//
// 路径名对齐 subconverter：/sub 是主入口，/profile 与 /render 是常见别名，
// 老配置换过来不用改路径。
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(requestLogger(s.logger))
	r.Use(chimw.Recoverer)
	// /subscribe 用订阅令牌（sub_token）自鉴权，必须豁免全局访问令牌校验：
	// middleware.go 的 presented() 会把 query 上的 token 当服务端令牌比对，不豁免则永远 401。
	r.Use(authExceptSubscribe(s.cfg.Token))
	r.Use(chimw.Timeout(s.cfg.Timeout))
	r.Use(responseHeaders(s.cfg.Name))

	// 前端为 SPA（Vite 产物）：页面路由交给浏览器，Go 侧只负责「静态资源 + fallback」。
	// 命中真实文件按文件服务；带扩展名却找不到的一律 404（绝不回落成 HTML）；
	// 其余无扩展名路径回落到 index.html。详见 spa.go。
	r.Get("/*", s.handleSPAFallback)
	r.Head("/*", s.handleSPAFallback)

	r.Get("/version", s.handleVersion)
	r.Get("/healthz", s.handleHealth)

	r.Get("/sub", s.handleConvert)
	r.Get("/profile", s.handleConvert)
	r.Get("/render", s.handleConvert)

	r.Get("/getruleset", s.handleGetRuleset)
	r.Get("/inspect", s.handleInspect)

	r.Route("/api", func(r chi.Router) {
		r.Get("/version", s.handleAPIVersion)
		r.Get("/stats", s.handleAPIStats)
		r.Get("/history", s.handleAPIHistory)
		r.Get("/pipelines", s.handleAPIPipelines)
		r.Post("/pipelines", s.handleAPIPipelinesCreate)
		r.Get("/pipelines/{id}", s.handleAPIPipeline)
		r.Put("/pipelines/{id}", s.handleAPIPipeline)
		r.Delete("/pipelines/{id}", s.handleAPIPipeline)
		r.Get("/targets", s.handleAPITargets)
		r.Get("/rulesets", s.handleAPIRulesets)
		r.Get("/logs", s.handleAPILogs)
		r.Post("/convert", s.handleAPIConvert)
		r.Post("/diff", s.handleAPIDiff)
	})

	// 订阅管理路由
	r.Route("/api/airports", func(r chi.Router) {
		r.Get("/", s.handleAPIAirports)
		r.Post("/", s.handleAPIAirports)
		r.Post("/refresh", s.handleAPITrafficRefresh) // 全部启用机场（静态段优先于 {id}）
		r.Post("/check", s.handleAPICheck)
		r.Get("/check/last", s.handleAPICheckLast)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleAPIAirport)
			r.Put("/", s.handleAPIAirport)
			r.Delete("/", s.handleAPIAirport)
			r.Post("/refresh", s.handleAPIAirportRefreshOne)
			r.Post("/import-rules", s.handleAPIImportRules)
		})
	})
	r.Route("/api/groups", func(r chi.Router) {
		r.Get("/", s.handleAPIGroups)
		r.Post("/", s.handleAPIGroups)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleAPIGroup)
			r.Put("/", s.handleAPIGroup)
			r.Delete("/", s.handleAPIGroup)
		})
	})
	// 订阅方案：一套模板 + 规则 + 分组，令牌绑定后彼此隔离。
	r.Route("/api/profiles", func(r chi.Router) {
		r.Get("/", s.handleAPIProfiles)
		r.Post("/", s.handleAPIProfiles)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleAPIProfile)
			r.Put("/", s.handleAPIProfile)
			r.Delete("/", s.handleAPIProfile)
		})
	})
	r.Route("/api/rules", func(r chi.Router) {
		r.Get("/", s.handleAPIRules)
		r.Post("/", s.handleAPIRules)
		r.Post("/bulk", s.handleAPIRulesBulk)
		r.Get("/sources", s.handleAPIRuleSourceCounts)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleAPIRule)
			r.Put("/", s.handleAPIRule)
			r.Delete("/", s.handleAPIRule)
		})
	})
	r.Route("/api/tokens", func(r chi.Router) {
		r.Get("/", s.handleAPITokens)
		r.Post("/", s.handleAPITokens)
	})
	// 单项令牌：GET 读、PUT 部分更新（五字段 + name，省略即不动、空串即清空）、DELETE 删。
	r.Get("/api/tokens/{token}", s.handleAPIToken)
	r.Put("/api/tokens/{token}", s.handleAPIToken)
	r.Delete("/api/tokens/{token}", s.handleAPIToken)
	r.Route("/api/self-nodes", func(r chi.Router) {
		r.Get("/", s.handleAPISelfNodes)
		r.Post("/", s.handleAPISelfNodes)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleAPISelfNode)
			r.Put("/", s.handleAPISelfNode)
			r.Delete("/", s.handleAPISelfNode)
		})
	})
	r.Route("/api/traffic", func(r chi.Router) {
		r.Get("/strategy", s.handleAPITrafficStrategy)
		r.Put("/strategy", s.handleAPITrafficStrategy)
		r.Post("/refresh", s.handleAPITrafficRefresh)
		r.Get("/preview", s.handleAPITrafficPreview)
	})
	r.Route("/api/scripts", func(r chi.Router) {
		r.Get("/", s.handleAPIScripts)
		r.Post("/", s.handleAPIScripts)
		r.Post("/test", s.handleAPIScriptsTest)
		r.Get("/{id}", s.handleAPIScript)
		r.Put("/{id}", s.handleAPIScript)
		r.Delete("/{id}", s.handleAPIScript)
	})
	r.Route("/api/tasks", func(r chi.Router) {
		r.Get("/", s.handleAPITasks)
	})
	r.Route("/api/alert-channels", func(r chi.Router) {
		r.Get("/", s.handleAPIAlertChannels)
		r.Put("/", s.handleAPIAlertChannels)
		r.Post("/test", s.handleAPIAlertChannelTest)
	})
	r.Get("/api/rule-providers", s.handleAPIRuleProviders)
	r.Put("/api/rule-providers/{id}/enabled", s.handleAPIRuleProviderSetEnabled)
	r.Get("/api/suggestions", s.handleAPISuggestions)
	r.Post("/api/suggestions/apply", s.handleAPIApplySuggestions)
	r.Post("/api/suggestions/{index}/confirm", s.handleAPIConfirmSuggestion)
	r.Post("/api/suggestions/{index}/reject", s.handleAPIRejectSuggestion)
	r.Get("/api/rule-sources", s.handleAPIRuleSources)
	r.Get("/api/rule-sources/rules", s.handleAPIRuleSourceRules)
	r.Post("/api/rule-sources/import-rules", s.handleAPIRuleSourceImport)
	r.Post("/api/rule-sources/init", s.handleAPIRuleSourcesInit)
	r.Post("/api/rule-sources/update", s.handleAPIRuleSourcesUpdate)
	r.Post("/api/rule-sources/rollback", s.handleAPIRuleSourcesRollback)
	r.Post("/api/rule-sources/mrs", s.handleAPIRuleSourcesMRS)
	r.Post("/api/rule-sources", s.handleAPIRuleSourcesAdd)
	r.Post("/api/rule-sources/remove", s.handleAPIRuleSourcesRemove)
	r.Post("/api/rule-sources/update-one", s.handleAPIRuleSourcesUpdateOne)
	r.Get("/api/configs", s.handleAPIConfigs)
	r.Route("/api/settings", func(r chi.Router) {
		r.Get("/", s.handleAPISettings)
		r.Put("/", s.handleAPISettings)
	})
	r.Route("/api/template", func(r chi.Router) {
		r.Get("/", s.handleAPITemplate)
		r.Put("/", s.handleAPITemplate)
	})
	r.Route("/api/override", func(r chi.Router) {
		r.Get("/", s.handleAPIOverride)
		r.Put("/", s.handleAPIOverride)
	})
	r.Get("/subscribe", s.handleSubscribe)

	// pprof 端点（仅 debug 模式）
	r.Get("/debug/pprof/", s.handlePprofIndex)
	r.Get("/debug/pprof/cmdline", s.handlePprofCmdline)
	r.Get("/debug/pprof/profile", s.handlePprofProfile)
	r.Get("/debug/pprof/symbol", s.handlePprofSymbol)
	r.Get("/debug/pprof/trace", s.handlePprofTrace)
	r.Get("/debug/pprof/heap", s.handlePprofHeap)
	r.Get("/debug/pprof/goroutine", s.handlePprofGoroutine)
	r.Get("/debug/pprof/allocs", s.handlePprofAllocs)
	r.Get("/debug/pprof/block", s.handlePprofBlock)
	r.Get("/debug/pprof/mutex", s.handlePprofMutex)
	r.Get("/debug/pprof/threadcreate", s.handlePprofThreadcreate)

	// 运行时指标
	r.Get("/api/metrics", s.handleMetrics)

	r.NotFound(s.handleNotFound)
	r.MethodNotAllowed(s.handleMethodNotAllowed)
	return r
}

// authExceptSubscribe 包装全局 auth，对两类请求例外。
//
// 一是 /subscribe：它用订阅令牌（sub_token）鉴权，而 auth 校验的是服务端访问令牌；
// middleware.go 的 presented() 优先级为 Authorization > X-Token > query token，
// 所以 /subscribe?token=<sub_token> 会被拿去和服务端令牌比对，永远 401。
//
// 二是控制台的静态外壳：index.html 与 /assets/* 是构建产物，不含任何用户数据，
// 但浏览器必须先把它们加载起来，控制台才有机会弹出输入框让用户填访问令牌。
// 不豁免时，设了 -token 的部署打开控制台只会看到一行 401 纯文本。
// 数据接口（/api/*、/sub、/getruleset 等）鉴权强度完全不变。
func authExceptSubscribe(tokenFor func() string) func(http.Handler) http.Handler {
	guarded := auth(tokenFor)
	return func(next http.Handler) http.Handler {
		inner := guarded(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authExemptPath(r) {
				next.ServeHTTP(w, r)
				return
			}
			inner.ServeHTTP(w, r)
		})
	}
}

// authExemptPath 判断请求是否可以跳过服务端访问令牌校验。
//
// 豁免范围刻意收得很窄：/subscribe、控制台外壳（index.html、favicon、静态资源）
// 与前端路由的深层链接（如 /subscriptions，刷新页面或直接访问子页面时要能打开）。
// 深层链接判定复用 spa.go 的 spaExcluded 与 lastSegment，与 SPA 回退逻辑同源，
// 因此凡是会返回 index.html 的路径都豁免，凡是交给真实处理器或 /api/* 的都不豁免。
func authExemptPath(r *http.Request) bool {
	p := r.URL.Path
	if p == "/subscribe" || p == "/favicon.ico" || p == "/favicon.svg" || strings.HasPrefix(p, "/assets/") {
		return true
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if spaExcluded(p) {
		return false
	}
	name := strings.TrimPrefix(p, "/")
	return name == "" || !strings.Contains(lastSegment(name), ".")
}

// handleVersion 返回 subconverter 形态的纯文本版本串。
//
// 这是 sub-web（CareyWang/sub-web）"可直连"的硬契约：它的 BackendService
// 直接 GET <backend>/version，拿到 response.data 后按字符串处理——
// formatVersion 会剥掉结尾的 "backend\n" 与串里的 "subconverter" 再显示。
// 返回 JSON 会让 .replace 抛异常并被静默吞掉，页面上版本号永远是空的。
//
// 形态对齐 subconverter v0.9.0 实测输出（text/plain、"<名字> v<版本> backend\n"）；
// 名字用自己的，协议只约定形状不约定产品名。
// 需要结构化的版本信息走 /api/version。
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.writeText(w, http.StatusOK, "text/plain; charset=utf-8", versionLine(s.engine.Profile.Version))
}

// versionLine 拼 subconverter 兼容的版本行。
func versionLine(version string) string {
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	return "subforge " + version + " backend\n"
}

// handleAPIVersion 返回结构化的版本与运行时长，供本仓 Web UI 与脚本使用。
// 与 /version 分开：/version 是给外部前端的兼容契约，不能塞 JSON。
func (s *Server) handleAPIVersion(w http.ResponseWriter, r *http.Request) {
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

// handleNotFound 统一 404 形状：路由未命中与业务 404 都走 JSON，
// 用 code 区分「路由没注册」与「资源不存在」，便于前端与服务探测。
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "路径不存在", "code": "route_not_found"})
}

// handleMethodNotAllowed 统一路由层 405 形状。
func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "方法不允许", "code": "method_not_allowed"})
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
