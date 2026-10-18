package subforge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/traffic"
)

const (
	// TrafficStrategyKey 是 config 表里存流量聚合策略的键。
	TrafficStrategyKey = "traffic.strategy"
	// trafficRefreshParallel 是刷新流量时对上游的最大并发。
	trafficRefreshParallel = 4
	// trafficAuditRetention 是 fetch_audit 保留的最近条数。
	trafficAuditRetention = 500
)

// TrafficRefreshResult 是单个机场一次流量刷新的结果。
type TrafficRefreshResult struct {
	AirportID int64         `json:"airport_id"`
	Name      string        `json:"name"`
	OK        bool          `json:"ok"`
	Usage     traffic.Usage `json:"usage"`
	NodeCount int           `json:"node_count"`
	Err       string        `json:"err"`
}

// GetTrafficStrategy 读取流量聚合策略。
//
// 取值优先级：config 表 traffic.strategy 的原文 → 预设名（sum/min/max/avg/first/last/
// min_quota/max_quota/weighted/off，大小写不敏感）→ JSON 形态的字段级策略。
// 未配置或无法解析时回落到默认预设 "sum"（= sum,sum,sum,min），**不返回错误**。
func (m *Manager) GetTrafficStrategy(ctx context.Context) (traffic.Strategy, error) {
	raw, err := m.getConfig(ctx, TrafficStrategyKey)
	if err != nil {
		return traffic.Strategy{}, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		def, _ := traffic.PresetStrategy("sum")
		return def, nil
	}
	if s, ok := traffic.PresetStrategy(raw); ok {
		return s, nil
	}
	var s traffic.Strategy
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		def, _ := traffic.PresetStrategy("sum")
		return def, nil
	}
	return s.Normalize(), nil
}

// SetTrafficStrategy 写入流量聚合策略（JSON 形态，保留字段级覆盖与 Preset）。
func (m *Manager) SetTrafficStrategy(ctx context.Context, s traffic.Strategy) error {
	s = s.Normalize()
	buf, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return m.setConfig(ctx, TrafficStrategyKey, string(buf))
}

// RefreshAirportTraffic 拉取所有启用的机场订阅，采集 subscription-userinfo 与节点数，
// 回写 airports 表并记 fetch_audit。对上游并发上限 trafficRefreshParallel。
//
// 语义（plan §3.3）：
//   - 拉取失败：只写 traffic_err，**保留上次成功的流量值**；
//   - 缓存命中（304，SubscriptionUserinfo 为空）：只清 traffic_err，不覆盖流量值；
//   - 拉取成功但上游未提供该头：流量值保持原样，traffic_err 记「上游未提供 subscription-userinfo」；
//   - 拉取成功且有该头：覆盖流量值 + node_count + traffic_at，清空 traffic_err。
//
// 单个机场失败不影响其他机场；返回结果顺序与 ListAirports 的启用项一致。
func (m *Manager) RefreshAirportTraffic(ctx context.Context, f *fetch.Fetcher) ([]TrafficRefreshResult, error) {
	if f == nil {
		return nil, errors.New("刷新流量需要非 nil 的 fetcher")
	}
	airports, err := m.ListAirports(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]Airport, 0, len(airports))
	for _, a := range airports {
		if a.Enabled && strings.TrimSpace(a.SubscriptionURL) != "" {
			targets = append(targets, a)
		}
	}
	results := make([]TrafficRefreshResult, len(targets))
	sem := make(chan struct{}, trafficRefreshParallel)
	var wg sync.WaitGroup
	for i, a := range targets {
		wg.Add(1)
		go func(i int, a Airport) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = m.refreshOneAirport(ctx, f, a)
		}(i, a)
	}
	wg.Wait()
	// 审计裁剪失败不阻断刷新结果
	_, _ = m.db.PruneFetchAudit(ctx, trafficAuditRetention)
	return results, nil
}

// refreshOneAirport 采集单个机场并回写。错误一律吞进结果里，不让单个机场炸掉整批。
func (m *Manager) refreshOneAirport(ctx context.Context, f *fetch.Fetcher, a Airport) TrafficRefreshResult {
	out := TrafficRefreshResult{AirportID: a.ID, Name: a.Name}
	start := time.Now()
	res := f.FetchFrom(ctx, fetch.Source{Name: a.Name, URL: a.SubscriptionURL})
	audit := store.FetchAudit{
		AirportID:  a.ID,
		URL:        a.SubscriptionURL,
		StatusCode: res.HTTPCode,
		Bytes:      int64(res.Size),
		DurationMS: time.Since(start).Milliseconds(),
	}
	// fail 统一收口：记审计 + 只写 traffic_err（不动流量值）
	fail := func(msg string) TrafficRefreshResult {
		out.Err = msg
		audit.Error = msg
		_ = m.db.AddFetchAudit(ctx, audit)
		_ = m.UpdateAirportTraffic(ctx, a.ID, nil, 0, msg)
		return out
	}
	if res.Err != nil {
		return fail(res.Err.Error())
	}
	audit.OK = true
	nodeCount := 0
	if nodes, err := parse.DecodeSubscription(res.Content); err == nil {
		nodeCount = len(nodes)
	}
	out.NodeCount = nodeCount
	raw := strings.TrimSpace(res.SubscriptionUserinfo)
	if raw == "" {
		if res.FromCache {
			// 304：上游没回该头是正常的，只清错误
			return fail("")
		}
		return fail("上游未提供 subscription-userinfo")
	}
	u, ok := traffic.ParseSubscriptionUserinfo(raw)
	if !ok {
		return fail("subscription-userinfo 无法解析: " + raw)
	}
	out.OK = true
	out.Usage = u
	_ = m.db.AddFetchAudit(ctx, audit)
	_ = m.UpdateAirportTraffic(ctx, a.ID, &u, nodeCount, "")
	return out
}

// RefreshAirportTrafficByID 只刷新一个机场的流量余额。
// 机场不存在时返回零值与错误（透传 store 的 sql.ErrNoRows 或 Manager 的中文错误）。
func (m *Manager) RefreshAirportTrafficByID(ctx context.Context, f *fetch.Fetcher, id int64) (TrafficRefreshResult, error) {
	if f == nil {
		return TrafficRefreshResult{}, errors.New("刷新流量需要非 nil 的 fetcher")
	}
	a, err := m.GetAirport(ctx, id)
	if err != nil {
		return TrafficRefreshResult{}, err
	}
	return m.refreshOneAirport(ctx, f, *a), nil
}

// PreviewTraffic 用已存数据按给定策略试算聚合结果，不落库、不访问网络。
// 返回 (usage, header, ok, err)：ok=false 表示无可聚合数据或策略为不透传（header 为空串）。
func (m *Manager) PreviewTraffic(ctx context.Context, s traffic.Strategy) (traffic.Usage, string, bool, error) {
	airports, err := m.ListAirports(ctx)
	if err != nil {
		return traffic.Usage{}, "", false, err
	}
	usages := make([]traffic.Usage, 0, len(airports))
	nodeCounts := make([]int, 0, len(airports))
	for _, a := range airports {
		if !a.Enabled || strings.TrimSpace(a.SubscriptionURL) == "" || a.TrafficAt <= 0 {
			continue
		}
		usages = append(usages, traffic.Usage{
			Upload: a.Upload, Download: a.Download, Total: a.Total, Expire: a.Expire,
		})
		nodeCounts = append(nodeCounts, a.NodeCount)
	}
	u, ok := traffic.Aggregate(usages, nodeCounts, s)
	if !ok {
		return traffic.Usage{}, "", false, nil
	}
	h := traffic.FormatWithStrategy(u, s)
	if h == "" {
		return u, "", false, nil
	}
	return u, h, true, nil
}
