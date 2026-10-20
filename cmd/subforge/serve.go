// serve 命令：启动 HTTP 订阅服务。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tktogether/subforge/internal/api"
	"github.com/tktogether/subforge/internal/buildinfo"
	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/render/clashmeta"
	"github.com/tktogether/subforge/internal/render/registry"
	"github.com/tktogether/subforge/internal/rulebot"
	"github.com/tktogether/subforge/internal/runtime"
	"github.com/tktogether/subforge/internal/scheduler"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/template"
)

const serveUsage = `subforge serve - 启动 HTTP 订阅服务

用法:
  subforge serve [-listen 127.0.0.1:25500] [选项]

选项:
  -listen     监听地址（默认 127.0.0.1:25500）
  -token      访问令牌；为空表示不鉴权
  -config     INI 配置文件，改动后自动热重载（-token 是它的初始值）
  -mixed-port clash.meta 输出的 mixed-port（默认 7890）
  -timeout    上游请求与单请求超时秒数（默认 30）
  -shutdown   收到停止信号后的优雅关闭秒数（默认 10）
  -cache-dir  上游内容磁盘缓存目录（默认不缓存）
  -data-dir   订阅管理数据目录（SQLite，默认不启用）
  -name      写入产物注释头的配置名（启动时固定）

配置文件 [subforge] 段落:
  token       访问令牌（热重载生效）
  mixed_port  clash.meta mixed-port（1-65535，热重载生效）
  name        配置名（需重启生效）
  token 也兼容写在 subconverter 风格的 [common] 段落。

配置文件其余段落按 subconverter 语义解释并热重载，URL 参数优先级更高:
  [common]  [node_pref]  [rename]  [emoji]  [clash_proxy_group]  [ruleset]

端点:
  GET /sub          转换订阅（target/url/include/exclude/... 见 /inspect）
  GET /profile      /sub 的别名
  GET /render       /sub 的别名
  GET /version      版本与运行时长
  GET /healthz      存活探针
  GET /inspect      回显解析后的参数
  GET /getruleset   规则集（type 1~6 六种形态）
  GET /subscribe    订阅令牌鉴权，返回纯文本配置
  GET /api/version  API 版本
  GET /api/stats    统计信息
  GET /api/history  转换历史
  GET /api/pipelines 管道信息
  GET /api/targets  目标类型
  GET /api/rulesets 规则集列表
  GET /api/logs     日志
  POST /api/convert 转换订阅
  POST /api/diff    差异对比
  /api/airports     机场 CRUD（GET/POST/PUT/DELETE + refresh/check/import-rules）
  /api/groups       分组 CRUD
  /api/rules        规则 CRUD + 批量
  /api/tokens       访问令牌 CRUD
  /api/self-nodes   自建节点 CRUD
  /api/traffic      流量策略/刷新/预览
  /api/scripts      脚本库 CRUD + 测试
  GET /api/tasks    调度器任务历史
  /api/alert-channels 告警通道配置与测试
  GET /api/rule-providers 规则提供者
  /api/suggestions  Rule-Bot 建议队列（confirm/reject）
  /api/rule-sources 规则源管理
  /api/settings     设置（GET/PUT）
  /api/template     模板（GET/PUT）
  /api/override     覆盖（GET/PUT）
  GET /api/metrics  运行时指标（JSON）
  /debug/pprof/*    性能分析
`

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	var (
		listen, token, cacheDir, dataDir, name, configFile string
		mixedPort, timeoutSeconds, shutdownSeconds         int
	)
	fs.StringVar(&listen, "listen", "127.0.0.1:25500", "监听地址")
	fs.StringVar(&token, "token", "", "访问令牌（空表示不鉴权）")
	fs.StringVar(&cacheDir, "cache-dir", "", "上游内容磁盘缓存目录（默认不缓存）")
	fs.StringVar(&dataDir, "data-dir", "", "订阅管理数据目录（SQLite，默认不启用）")
	fs.StringVar(&name, "name", "", "写入产物注释头的配置名")
	fs.StringVar(&configFile, "config", "", "INI 配置文件，改动后自动热重载")
	fs.IntVar(&mixedPort, "mixed-port", config.DefaultMixedPort, "clash.meta 输出的 mixed-port")
	fs.IntVar(&timeoutSeconds, "timeout", 30, "上游请求与单请求超时秒数")
	fs.IntVar(&shutdownSeconds, "shutdown", 10, "收到停止信号后的优雅关闭秒数")
	fs.Usage = func() { fmt.Print(serveUsage) }

	if err := fs.Parse(args); err != nil {
		return err
	}

	// 命令行参数是最低优先级：配置文件里的同名项覆盖它。
	snap, err := loadSettings(configFile, config.Settings{Token: token, MixedPort: mixedPort, Name: name})
	if err != nil {
		return err
	}

	// 默认只监听本机。这个服务会向外部发订阅请求，一旦绑到所有网卡又没有令牌，
	// 就成了一个能借用本机网络去拉别人订阅的开放中转——启动时把这个组合警告出来。
	// 这里只警告不拦截：用户可能在反向代理或内网隔离环境里刻意这么部署。
	if bindAllInterfaces(listen) && snap.Load().Token == "" {
		fmt.Fprintln(os.Stderr, "警告: 监听所有网卡但未设置访问令牌，同网段任何主机都能使用本服务。")
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	fetcher, err := newFetcher(cacheDir, timeout)
	if err != nil {
		return err
	}

	engine := &convert.Engine{
		Fetcher: fetcher,
		// mixed-port 从快照读，所以改配置文件能立即影响后续请求的产物。
		RendererFor: func(target string) (render.Renderer, error) {
			opts := clashmeta.Options{MixedPort: snap.Load().MixedPort}
			return registry.Lookup(target, opts)
		},
		// 本地模板也从同一个快照读，改 INI 里的 [node_pref] / [clash_proxy_group] 同样立刻生效。
		BaseTemplate: func() *config.Template { return snap.Load().Template },
		// 请求参数 config= 指向的模板优先级更高，可以是本地路径或远程 URL。
		LoadTemplate: func(ctx context.Context, ref string) (*config.Template, error) {
			return config.LoadSource(ctx, ref, config.HTTPLoader(&http.Client{Timeout: timeout}, nil))
		},
		Profile: render.Profile{Name: name, Version: buildinfo.Full()},
	}

	srv := api.New(api.Config{
		Engine:  engine,
		Token:   func() string { return snap.Load().Token },
		Timeout: timeout,
		Name:    "subforge",
		Logger:  log.Default(),
		DataDir: dataDir,
	})

	if dataDir != "" {
		mgr, err := subforge.NewManagerWithDataDir(filepath.Join(dataDir, "subforge.db"), dataDir)
		if err != nil {
			return fmt.Errorf("初始化订阅管理: %w", err)
		}
		defer mgr.Close()
		srv.SetSubManager(mgr)
		fmt.Fprintf(os.Stderr, "订阅管理已启用: %s\n", dataDir)

		// Rule-Bot 建议队列：空队列，生产运行下由调度器/手动触发填充。
		q := rulebot.NewPendingQueue()
		api.SetSuggestionQueue(q)
		loop := rulebot.NewLoop(rulebot.NewDetector(), q)
		api.SetSuggestionLoop(loop)

		// 调度器：把已启用的告警通道接进来，再注册两个定时任务。
		sch := scheduler.New()
		sch.SetOnResult(func(r scheduler.JobResult) {
			if !r.Success && r.Err != "" {
				loop.Ingest("scheduler", []string{r.JobName + ": " + r.Err})
			}
		})
		channels, chErr := mgr.GetAlertChannels(context.Background())
		if chErr != nil {
			log.Printf("[serve] 读取告警通道失败: %v", chErr)
		} else {
			for _, c := range channels {
				if !c.Enabled {
					continue
				}
				switch c.Kind {
				case "telegram":
					sch.AddAlertChannel(scheduler.NewTelegramChannel(c.BotToken, c.ChatID))
				case "dingtalk":
					sch.AddAlertChannel(scheduler.NewDingTalkChannel(c.Webhook))
				case "smtp":
					log.Printf("[serve] 告警通道 %q 为 smtp，SMTPChannel 尚未实现，已跳过", c.Kind)
				default:
					log.Printf("[serve] 未知告警通道 kind=%q，已跳过", c.Kind)
				}
			}
		}

		// 从 config 表读取 cron 表达式：键不存在用默认；键存在但为空串则不注册。
		cronCtx := context.Background()
		ruleSpec, ruleErr := scheduler.CronSpecFromConfig(cronCtx, mgr, scheduler.RuleSourceCronKey, scheduler.DefaultRuleSourceCron)
		if ruleErr != nil {
			log.Printf("[serve] 读取 %s 失败: %v，使用默认", scheduler.RuleSourceCronKey, ruleErr)
			ruleSpec = scheduler.DefaultRuleSourceCron
		}
		if ruleSpec != "" {
			job := scheduler.NewRuleSourceUpdateJob(mgr, log.Default()).WithSpec(ruleSpec)
			if err := sch.AddJob(job); err != nil {
				log.Printf("[serve] 注册 rule-sources 任务失败: %v", err)
			}
		}

		subSpec, subErr := scheduler.CronSpecFromConfig(cronCtx, mgr, scheduler.SubscheckCronKey, scheduler.DefaultSubscheckCron)
		if subErr != nil {
			log.Printf("[serve] 读取 %s 失败: %v，使用默认", scheduler.SubscheckCronKey, subErr)
			subSpec = scheduler.DefaultSubscheckCron
		}
		if subSpec != "" {
			job := scheduler.NewSubscheckJob(mgr, fetcher, log.Default()).WithSpec(subSpec)
			if err := sch.AddJob(job); err != nil {
				log.Printf("[serve] 注册 subscheck 任务失败: %v", err)
			}
		}

		srv.SetScheduler(sch)
		log.Printf("[serve] rule-bot stats: %v", loop.Stats())
		sch.Start()
		// defer 后进先出：sch.Stop 先于 mgr.Close 执行，确保调度器在 manager 关闭前停止。
		defer sch.Stop()
	}

	httpServer := &http.Server{
		Addr:              listen,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// 写超时留两倍余量：单请求最多 timeout，加上排队与写回。
		WriteTimeout: timeout * 2,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if configFile != "" {
		go watchConfig(ctx, configFile, config.Settings{Token: token, MixedPort: mixedPort}, snap)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()

	fmt.Fprintf(os.Stderr, "subforge %s 监听 %s（格式: %s）\n",
		buildinfo.Version, listen, strings.Join(registry.Available(), ", "))
	if configFile != "" {
		fmt.Fprintf(os.Stderr, "配置: %s（热重载已启用）\n", configFile)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case s := <-sig:
		fmt.Fprintf(os.Stderr, "收到 %s，正在关闭（最多 %d 秒）\n", s, shutdownSeconds)
		shutCtx, shutCancel := context.WithTimeout(context.Background(), time.Duration(shutdownSeconds)*time.Second)
		defer shutCancel()
		if err := httpServer.Shutdown(shutCtx); err != nil {
			// 还有请求没跑完：先打印再硬关，别静默吞掉。
			fmt.Fprintf(os.Stderr, "优雅关闭超时: %v\n", err)
			_ = httpServer.Close()
		}
		cancel()
		fmt.Fprintln(os.Stderr, "已关闭")
		return nil
	}
}

// liveSettings 是一次热重载的原子单元：服务设置与转换模板同源同换，
// 避免出现"token 已是新配置、[node_pref] 还是旧配置"的中间状态。
type liveSettings struct {
	config.Settings
	// Template 是同一份 INI 按 subconverter 语义解释出的模板，未配置时为 nil。
	Template *config.Template
}

// loadSettings 按"命令行默认 <- 配置文件"的顺序取设置，同一份文件也顺带解析出模板。
func loadSettings(path string, base config.Settings) (*runtime.Snapshot[liveSettings], error) {
	cur := liveSettings{Settings: base}
	if path != "" {
		cfg, err := template.ParseFile(path)
		if err != nil {
			return nil, err
		}
		cur.Settings = config.FromINI(cfg, base)
		// 两层解释各解析一遍：设置层只读 [subforge]，模板层只读转换段落，互不假设对方实现。
		tpl, err := config.LoadFile(path)
		if err != nil {
			return nil, err
		}
		cur.Template = tpl
	}
	return runtime.NewSnapshot(cur), nil
}

// watchConfig 轮询配置文件，改动后重新解析并整体替换设置快照。
//
// 解析失败保留旧设置并继续轮询：配置写错不应该把正在跑的服务打挂。
func watchConfig(ctx context.Context, path string, base config.Settings, snap *runtime.Snapshot[liveSettings]) {
	if err := runtime.Watch(ctx, path, time.Second, func(body []byte) error {
		if body == nil {
			// 文件暂时读不到（被编辑器移走重建等），沿用当前设置。
			return nil
		}
		cfg, err := template.Parse(body, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "配置 %s 解析失败，沿用当前设置: %v\n", path, err)
			return nil
		}
		next := liveSettings{Settings: config.FromINI(cfg, base)}
		if next.Template, err = config.LoadBody(body, path); err != nil {
			fmt.Fprintf(os.Stderr, "配置 %s 模板解析失败，沿用当前模板: %v\n", path, err)
			return nil
		}
		prev := snap.Store(next)
		if prev.Token != next.Token || prev.MixedPort != next.MixedPort {
			fmt.Fprintf(os.Stderr, "配置已重载: mixed_port %d -> %d, token %s -> %s\n",
				prev.MixedPort, next.MixedPort, dash(prev.Token), dash(next.Token))
		}
		if prev.Template == nil && next.Template != nil || prev.Template != nil && next.Template != nil {
			fmt.Fprintf(os.Stderr, "配置模板已重载: %s\n", path)
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "配置热重载停止: %v\n", err)
	}
}

// bindAllInterfaces 判定监听地址是否会暴露到本机网卡上。
func bindAllInterfaces(listen string) bool {
	host := listen
	if i := strings.LastIndex(listen, ":"); i >= 0 {
		host = listen[:i]
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return host == "0.0.0.0" || host == "::" || host == ""
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
