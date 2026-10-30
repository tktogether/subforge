// 本文件把 M3.3 的定时任务接到 serve 命令：调度器骨架在 scheduler.go，
// 这里提供可装配的任务实现（函数式任务 + 规则源更新 + 订阅测活）。
package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/subscheck"
)

// 定时任务的默认 cron 表达式（5 字段：分 时 日 月 周）。
const (
	// DefaultRuleSourceCron 是规则源更新的默认调度时间（每天 04:00）。
	DefaultRuleSourceCron = "0 4 * * *"
	// DefaultSubscheckCron 是订阅测活的默认调度周期（每 30 分钟）。
	DefaultSubscheckCron = "*/30 * * * *"
)

// cron 表达式在 subforge 的 config 表里的键名。
const (
	// RuleSourceCronKey 是规则源更新的调度表达式键。
	RuleSourceCronKey = "rules.update_cron"
	// SubscheckCronKey 是订阅测活的调度表达式键。
	SubscheckCronKey = "subscheck.cron"
)

// FuncJob 用函数值定义一个定时任务，供 serve 侧按配置装配。
type FuncJob struct {
	jobName string
	spec    string
	run     func(ctx context.Context) error
}

// NewFuncJob 创建函数式任务；run 为 nil 时 Run 直接返回 nil（不 panic）。
func NewFuncJob(name, spec string, run func(ctx context.Context) error) *FuncJob {
	return &FuncJob{jobName: name, spec: spec, run: run}
}

// Name 返回任务名。
func (j *FuncJob) Name() string { return j.jobName }

// Spec 返回 cron 表达式。
func (j *FuncJob) Spec() string { return j.spec }

// Run 执行任务函数。
func (j *FuncJob) Run(ctx context.Context) error {
	if j.run == nil {
		return nil
	}
	return j.run(ctx)
}

// CronSpecFromConfig 读取 config 表里的 cron 表达式。
//
// 与 Manager.GetConfig 的区别：GetConfig 对“键不存在”和“值为空串”都返回 ""，
// 而这里要区分两者——键不存在时用 def（默认表达式），键存在但为空串时返回 ""
// （调用方据此跳过注册该任务）。
func CronSpecFromConfig(ctx context.Context, mgr *subforge.Manager, key, def string) (string, error) {
	var val string
	err := mgr.DB().Raw().QueryRowContext(ctx, "SELECT value FROM config WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return "", fmt.Errorf("读取配置 %s: %w", key, err)
	}
	return val, nil
}

// RuleSourceUpdateJob 定时更新规则源（按 manifest 拉取并落盘）。
type RuleSourceUpdateJob struct {
	mgr    *subforge.Manager
	logger *log.Logger
	spec   string
}

// NewRuleSourceUpdateJob 创建规则源更新任务，cron 默认 DefaultRuleSourceCron。
func NewRuleSourceUpdateJob(mgr *subforge.Manager, logger *log.Logger) *RuleSourceUpdateJob {
	if logger == nil {
		logger = log.Default()
	}
	return &RuleSourceUpdateJob{mgr: mgr, logger: logger, spec: DefaultRuleSourceCron}
}

// WithSpec 覆盖 cron 表达式（链式调用，便于 serve 侧从配置装配）。
func (j *RuleSourceUpdateJob) WithSpec(spec string) *RuleSourceUpdateJob {
	j.spec = spec
	return j
}

// Name 返回任务名。
func (j *RuleSourceUpdateJob) Name() string { return "rule-sources" }

// Spec 返回 cron 表达式。
func (j *RuleSourceUpdateJob) Spec() string { return j.spec }

// Run 执行一次规则源更新。
//
// manifest 路径未配置时直接返回 nil：这不是失败，不该触发重试与告警。
func (j *RuleSourceUpdateJob) Run(ctx context.Context) error {
	path, err := j.mgr.GetRuleManifestPath(ctx)
	if err != nil {
		return fmt.Errorf("读取规则源清单路径: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		j.logger.Printf("[scheduler] rule-sources: 未配置 %s，跳过本次更新", subforge.RuleManifestPathKey)
		return nil
	}
	results, err := j.mgr.UpdateRuleSources(ctx)
	if err != nil {
		return fmt.Errorf("更新规则源: %w", err)
	}
	j.logger.Printf("[scheduler] rule-sources: 更新完成，共 %d 个规则源", len(results))
	return nil
}

// SubscheckJob 定时对所有“启用且已配置订阅地址”的机场做测活。
type SubscheckJob struct {
	mgr    *subforge.Manager
	logger *log.Logger
	spec   string
	// fetcher 由 serve 侧注入，与转换链路共用上游配置。subscheck.Check 目前
	// 自建 client（subscheck.Options.Client 需要 *http.Client，而 fetch.Fetcher
	// 未暴露 client/config 访问器），所以该字段暂不参与探测；保留注入点是为了
	// 将来把 fetcher 的 transport 接进来时不用改构造签名。
	fetcher *fetch.Fetcher
	client  *http.Client
}

// NewSubscheckJob 创建订阅测活任务，cron 默认 DefaultSubscheckCron。
func NewSubscheckJob(mgr *subforge.Manager, fetcher *fetch.Fetcher, logger *log.Logger) *SubscheckJob {
	if logger == nil {
		logger = log.Default()
	}
	return &SubscheckJob{mgr: mgr, fetcher: fetcher, logger: logger, spec: DefaultSubscheckCron}
}

// WithSpec 覆盖 cron 表达式（链式调用）。
func (j *SubscheckJob) WithSpec(spec string) *SubscheckJob {
	j.spec = spec
	return j
}

// WithClient 注入与转换链路相同的出站客户端（含 SSRF 限制）。
func (j *SubscheckJob) WithClient(client *http.Client) *SubscheckJob {
	j.client = client
	return j
}

// Name 返回任务名。
func (j *SubscheckJob) Name() string { return "subscheck" }

// Spec 返回 cron 表达式。
func (j *SubscheckJob) Spec() string { return j.spec }

// Run 执行一次订阅测活。
//
// 没有可测活的机场时直接返回 nil：空集合不是失败。
func (j *SubscheckJob) Run(ctx context.Context) error {
	airports, err := j.mgr.ListAirports(ctx)
	if err != nil {
		return fmt.Errorf("读取机场列表: %w", err)
	}
	urls := make([]string, 0, len(airports))
	for _, a := range airports {
		if !a.Enabled || strings.TrimSpace(a.SubscriptionURL) == "" {
			continue
		}
		urls = append(urls, a.SubscriptionURL)
	}
	if len(urls) == 0 {
		j.logger.Printf("[scheduler] subscheck: 没有启用且已配置订阅地址的机场，跳过本次测活")
		return nil
	}
	results := subscheck.Check(ctx, urls, subscheck.Options{Client: j.client})
	ok, fail := subscheck.Summary(results)
	for _, r := range results {
		if !r.OK {
			j.logger.Printf("[scheduler] subscheck: %s 失败: %v", r.URL, r.Err)
		}
	}
	j.logger.Printf("[scheduler] subscheck: 完成，成功 %d / 失败 %d（共 %d 个订阅）", ok, fail, len(urls))
	return nil
}
