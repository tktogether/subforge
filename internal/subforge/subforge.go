// Package subforge 提供订阅管理的业务逻辑。
//
// 对标 sub-poly 的 D1 数据模型，用 SQLite 持久化订阅、规则、分组等。
//
// 订阅产物不在数据层生成：每次请求由 HTTP 层把这里读到的 DB 当前状态映射成
// internal/convert.Params，交给 convert.Engine.Run 实时渲染，配置永不落盘静态文件。
package subforge

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/traffic"
)

// Manager 是订阅管理的业务入口。
type Manager struct {
	db      *store.DB
	dataDir string
}

// defaultDataDirFromDSN 从 DSN 推导数据根目录；无法推导时返回 ""（表示落 cwd）。
func defaultDataDirFromDSN(dsn string) string {
	s := strings.TrimSpace(dsn)
	if s == "" || s == ":memory:" || strings.Contains(s, "?") {
		return ""
	}
	dir := filepath.Dir(s)
	if dir == "." || dir == "" {
		return ""
	}
	return dir
}

// NewManager 创建 Manager，打开（或创建）SQLite 数据库。
func NewManager(dsn string) (*Manager, error) {
	return NewManagerWithDataDir(dsn, defaultDataDirFromDSN(dsn))
}

// NewManagerWithDataDir 创建 Manager，打开（或创建）SQLite 数据库，并显式指定数据根目录。
// dataDir 为空表示「未指定，落 cwd」。
func NewManagerWithDataDir(dsn, dataDir string) (*Manager, error) {
	db, err := store.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &Manager{db: db, dataDir: dataDir}, nil
}

// DataDir 返回数据根目录；空串表示未指定，落 cwd。
func (m *Manager) DataDir() string { return m.dataDir }

// Close 关闭数据库连接。
func (m *Manager) Close() error { return m.db.Close() }

// DB 返回底层 *store.DB。
func (m *Manager) DB() *store.DB { return m.db }

// --- Airport（订阅）管理 ---

// Airport 是一个订阅源。
//
// 行类型（含 JSON 标签）统一定义在 internal/store，这里用别名保持 subforge 的公开名字，
// 避免同一张表在两层各有一份结构体、字段各自漂移。
type Airport = store.Airport

// ListAirports 返回所有订阅。
// remarks 列可为 NULL，用 COALESCE 兜底，避免扫描时报 "converting NULL to string"。
func (m *Manager) ListAirports(ctx context.Context) ([]Airport, error) {
	rows, err := m.db.Raw().QueryContext(ctx,
		"SELECT id, name, subscription_url, COALESCE(remarks, ''), enabled, sort_order, "+
			"upload, download, total, expire, node_count, traffic_at, COALESCE(traffic_err, ''), "+
			"created_at, updated_at FROM airports ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Airport{}
	for rows.Next() {
		var a Airport
		if err := rows.Scan(&a.ID, &a.Name, &a.SubscriptionURL, &a.Remarks, &a.Enabled, &a.SortOrder,
			&a.Upload, &a.Download, &a.Total, &a.Expire, &a.NodeCount, &a.TrafficAt, &a.TrafficErr,
			&a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAirport 返回单个订阅。
func (m *Manager) GetAirport(ctx context.Context, id int64) (*Airport, error) {
	var a Airport
	err := m.db.Raw().QueryRowContext(ctx,
		"SELECT id, name, subscription_url, COALESCE(remarks, ''), enabled, sort_order, "+
			"upload, download, total, expire, node_count, traffic_at, COALESCE(traffic_err, ''), "+
			"created_at, updated_at FROM airports WHERE id = ?", id).
		Scan(&a.ID, &a.Name, &a.SubscriptionURL, &a.Remarks, &a.Enabled, &a.SortOrder,
			&a.Upload, &a.Download, &a.Total, &a.Expire, &a.NodeCount, &a.TrafficAt, &a.TrafficErr,
			&a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("订阅 %d 不存在", id)
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// AddAirport 新增订阅。
func (m *Manager) AddAirport(ctx context.Context, name, subscriptionURL, remarks string, enabled bool) (*Airport, error) {
	res, err := m.db.Raw().ExecContext(ctx,
		"INSERT INTO airports (name, subscription_url, remarks, enabled, sort_order) VALUES (?, ?, ?, ?, ?)",
		name, subscriptionURL, remarks, enabled, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return m.GetAirport(ctx, id)
}

// UpdateAirport 更新订阅。
func (m *Manager) UpdateAirport(ctx context.Context, id int64, name, subscriptionURL, remarks string, enabled bool) (*Airport, error) {
	_, err := m.db.Raw().ExecContext(ctx,
		"UPDATE airports SET name = ?, subscription_url = ?, remarks = ?, enabled = ?, updated_at = ? WHERE id = ?",
		name, subscriptionURL, remarks, enabled, time.Now(), id)
	if err != nil {
		return nil, err
	}
	return m.GetAirport(ctx, id)
}

// DeleteAirport 删除订阅。
func (m *Manager) DeleteAirport(ctx context.Context, id int64) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM airports WHERE id = ?", id)
	return err
}

// UpdateAirportTraffic 回写一次流量采集结果。
//
// u != nil：覆盖 upload/download/total/expire/node_count/traffic_at，并清空 traffic_err；
// u == nil：只写 traffic_err（errMsg 为空即清空），**保留上一次成功采集的流量值**——
// 缓存命中（304）或上游临时故障时，不能让下游客户端看到的余额被清零。
func (m *Manager) UpdateAirportTraffic(ctx context.Context, id int64, u *traffic.Usage, nodeCount int, errMsg string) error {
	now := time.Now()
	if u != nil {
		_, err := m.db.Raw().ExecContext(ctx,
			"UPDATE airports SET upload = ?, download = ?, total = ?, expire = ?, "+
				"node_count = ?, traffic_at = ?, traffic_err = '', updated_at = ? WHERE id = ?",
			u.Upload, u.Download, u.Total, u.Expire, nodeCount, now.Unix(), now, id)
		return err
	}
	_, err := m.db.Raw().ExecContext(ctx,
		"UPDATE airports SET traffic_err = ?, updated_at = ? WHERE id = ?",
		errMsg, now, id)
	return err
}

// UpdateAirportNodeCount 只回写节点数并清空 traffic_err，不动流量值与 traffic_at。
//
// 存在的理由：有些机场根本不返回 subscription-userinfo，拉取本身却是成功的。
// 这种"成功但拿不到流量"既不该记成采集失败，也不该把节点数丢掉，而
// UpdateAirportTraffic 在 u == nil 时只写 traffic_err、写不了节点数。
// traffic_at 保持不变是刻意的——它表示"上次拿到流量余额的时刻"，这次没拿到。
func (m *Manager) UpdateAirportNodeCount(ctx context.Context, id int64, nodeCount int) error {
	now := time.Now()
	_, err := m.db.Raw().ExecContext(ctx,
		"UPDATE airports SET node_count = ?, traffic_err = '', updated_at = ? WHERE id = ?",
		nodeCount, now, id)
	return err
}

// --- SelfNode（自建节点）管理 ---

// SelfNode 是一个自建节点。行类型定义在 internal/store。
type SelfNode = store.SelfNode

// ListSelfNodes 返回所有自建节点。
func (m *Manager) ListSelfNodes(ctx context.Context) ([]SelfNode, error) {
	return m.db.ListSelfNodes(ctx)
}

// GetSelfNode 返回单个自建节点。
func (m *Manager) GetSelfNode(ctx context.Context, id int64) (*SelfNode, error) {
	return m.db.GetSelfNode(ctx, id)
}

// AddSelfNode 新增自建节点。sortOrder>0 用用户值，<=0 表示"未指定"（由 store 回退时间戳）。
func (m *Manager) AddSelfNode(ctx context.Context, link, name string, enabled bool, sortOrder int) (*SelfNode, error) {
	return m.db.AddSelfNode(ctx, link, name, enabled, sortOrder)
}

// UpdateSelfNode 更新自建节点。sortOrder>0 覆盖，<=0 保持原值。
func (m *Manager) UpdateSelfNode(ctx context.Context, id int64, link, name string, enabled bool, sortOrder int) error {
	return m.db.UpdateSelfNode(ctx, id, link, name, enabled, sortOrder)
}

// DeleteSelfNode 删除自建节点。
func (m *Manager) DeleteSelfNode(ctx context.Context, id int64) error {
	return m.db.DeleteSelfNode(ctx, id)
}

// --- ProxyGroup（代理分组）管理 ---

// ProxyGroup 是一个代理分组。行类型定义在 internal/store。
type ProxyGroup = store.ProxyGroup

// ListProxyGroups 返回所有代理分组。
// regex / url / interval 列可为 NULL，用 COALESCE 兜底。
func (m *Manager) ListProxyGroups(ctx context.Context) ([]ProxyGroup, error) {
	rows, err := m.db.Raw().QueryContext(ctx,
		"SELECT id, name, type, COALESCE(regex, ''), COALESCE(url, ''), COALESCE(interval, 0), sort_order, created_at, updated_at FROM proxy_groups ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProxyGroup{}
	for rows.Next() {
		var g ProxyGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// AddProxyGroup 新增代理分组。sortOrder>0 用用户值，<=0 表示"未指定"（由 store 回退时间戳）。
func (m *Manager) AddProxyGroup(ctx context.Context, name, typ, regex, url string, interval, sortOrder int) (*ProxyGroup, error) {
	return m.db.AddProxyGroup(ctx, name, typ, regex, url, interval, sortOrder)
}

// DeleteProxyGroup 删除代理分组。
func (m *Manager) DeleteProxyGroup(ctx context.Context, id int64) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM proxy_groups WHERE id = ?", id)
	return err
}

// --- Rule（规则）管理 ---

// Rule 是一条分流规则。行类型定义在 internal/store。
type Rule = store.Rule

// ListRules 返回所有规则。
// rule_param 列可为 NULL，用 COALESCE 兜底。
func (m *Manager) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := m.db.Raw().QueryContext(ctx,
		"SELECT id, rule_type, COALESCE(rule_param, ''), rule_config, resolve_dns, enabled, COALESCE(source, ''), COALESCE(raw, ''), COALESCE(origin_key, ''), sort_order, created_at, updated_at FROM rules ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled, &r.Source, &r.Raw, &r.OriginKey, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddRule 新增规则。
func (m *Manager) AddRule(ctx context.Context, ruleType, ruleParam string, ruleConfig int64, resolveDNS, enabled bool) (*Rule, error) {
	res, err := m.db.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		ruleType, ruleParam, ruleConfig, resolveDNS, enabled, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var r Rule
	err = m.db.Raw().QueryRowContext(ctx,
		"SELECT id, rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, created_at, updated_at FROM rules WHERE id = ?", id).
		Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteRule 删除规则。
func (m *Manager) DeleteRule(ctx context.Context, id int64) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM rules WHERE id = ?", id)
	return err
}

// --- SubToken（订阅令牌）管理 ---

// SubToken 是一个订阅令牌。行类型定义在 internal/store。
type SubToken = store.SubToken

// CreateSubToken 新增订阅令牌。
// target/pipeline/config/include/exclude 是令牌的订阅默认值，空串表示不定制
// （沿用现有回落链）。优先级：query 显式参数 > 令牌字段 > 原回落链。
func (m *Manager) CreateSubToken(ctx context.Context, token, name, target, pipeline, configName, include, exclude string) error {
	_, err := m.db.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, target, pipeline, config, include, exclude) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)",
		token, name, target, pipeline, configName, include, exclude)
	return err
}

// GetSubToken 返回订阅令牌。五个定制列用 COALESCE 兜底为空串，与 ListSubTokens 一致。
func (m *Manager) GetSubToken(ctx context.Context, token string) (*SubToken, error) {
	var t SubToken
	err := m.db.Raw().QueryRowContext(ctx,
		"SELECT token, COALESCE(name, ''), COALESCE(target, ''), COALESCE(pipeline, ''), "+
			"COALESCE(config, ''), COALESCE(include, ''), COALESCE(exclude, ''), created_at "+
			"FROM sub_tokens WHERE token = ?", token).
		Scan(&t.Token, &t.Name, &t.Target, &t.Pipeline, &t.Config, &t.Include, &t.Exclude, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("令牌 %s 不存在", token)
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateSubToken 按 token 部分更新订阅令牌：只更新非 nil 字段，nil 字段保持原值；
// 非 nil 空串 = 清空该字段。token 不存在返回 sql.ErrNoRows。转发到 store 层。
func (m *Manager) UpdateSubToken(ctx context.Context, token string, u store.SubTokenUpdate) error {
	return m.db.UpdateSubToken(ctx, token, u)
}

// DeleteSubToken 删除订阅令牌。
func (m *Manager) DeleteSubToken(ctx context.Context, token string) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM sub_tokens WHERE token = ?", token)
	return err
}

// --- 分组 / 规则 / 令牌的单项读写（HTTP 层 GET/PUT 依赖，SQL 见 internal/store）---

// ListSubTokens 返回所有订阅令牌。
func (m *Manager) ListSubTokens(ctx context.Context) ([]SubToken, error) {
	return m.db.ListSubTokens(ctx)
}

// GetProxyGroup 返回单个代理分组。
func (m *Manager) GetProxyGroup(ctx context.Context, id int64) (*ProxyGroup, error) {
	g, err := m.db.GetProxyGroup(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("代理分组 %d 不存在", id)
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

// UpdateProxyGroup 更新代理分组。sortOrder>0 覆盖，<=0 保持原值。
func (m *Manager) UpdateProxyGroup(ctx context.Context, id int64, name, gtype, regex, url string, interval, sortOrder int) error {
	if err := m.db.UpdateProxyGroup(ctx, id, name, gtype, regex, url, interval, sortOrder); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("代理分组 %d 不存在", id)
		}
		return err
	}
	return nil
}

// GetRule 返回单条规则。
func (m *Manager) GetRule(ctx context.Context, id int64) (*Rule, error) {
	r, err := m.db.GetRule(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("规则 %d 不存在", id)
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// UpdateRule 更新单条规则。
func (m *Manager) UpdateRule(ctx context.Context, id int64, ruleType, ruleParam string, ruleConfig int64, resolveDNS, enabled bool) error {
	if err := m.db.UpdateRule(ctx, id, ruleType, ruleParam, ruleConfig, resolveDNS, enabled); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("规则 %d 不存在", id)
		}
		return err
	}
	return nil
}

// --- 订阅编译输入 ---

// SubscriptionInput 是一次订阅编译所需的 DB 当前状态快照。
//
// 它只是"编译输入"而不是产物：HTTP 层的 /subscribe 把 Airports 映射成
// convert.Params.URLs、Groups 映射成 render.Group、Rules 映射成规则行，
// 再交给 convert.Engine.Run 实时渲染。全程不落盘静态配置文件。
type SubscriptionInput struct {
	Airports []Airport    `json:"airports"`
	Groups   []ProxyGroup `json:"groups"`
	Rules    []Rule       `json:"rules"`
}

// GenerateSubscription 读取一次订阅编译所需的 DB 当前状态（仅启用中的订阅）。
//
// Deprecated: 订阅生成不再属于数据层。配置由 HTTP 层在每次 /subscribe 请求时，
// 用 internal/convert 的 Params + Engine.Run 按 DB 当前状态实时编译。
// 本方法因此不再返回 YAML 文本，改为返回结构化编译输入；仓库内已无调用方。
//
// 旧实现手写字符串拼 YAML，有三个真 bug（不要恢复）：
//  1. 每个节点只输出 name/type/server/port，其余协议字段全丢；
//  2. 代理组的 proxies 恒为 []，分组等于空组；
//  3. 规则行只有 TYPE,PARAM，缺第三列策略。
//
// 另外它用 p["port"].(int) 断言端口，而解析出来的端口是 float64，
// 断言失败直接 panic。该断言已彻底删除，本包不再有任何会 panic 的类型断言。
func (m *Manager) GenerateSubscription(ctx context.Context) (*SubscriptionInput, error) {
	airports, err := m.ListAirports(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载订阅列表失败: %w", err)
	}
	enabled := []Airport{}
	for _, a := range airports {
		if a.Enabled {
			enabled = append(enabled, a)
		}
	}
	groups, err := m.ListProxyGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载代理分组失败: %w", err)
	}
	rules, err := m.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载规则失败: %w", err)
	}
	return &SubscriptionInput{Airports: enabled, Groups: groups, Rules: rules}, nil
}

// getConfig 从 config 表读取配置值。
func (m *Manager) getConfig(ctx context.Context, key string) (string, error) {
	var val string
	err := m.db.Raw().QueryRowContext(ctx, "SELECT value FROM config WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// GetConfig 读取 config 表的单个键；不存在返回 ("", nil)。
func (m *Manager) GetConfig(ctx context.Context, key string) (string, error) {
	return m.getConfig(ctx, key)
}

// setConfig 写入配置值。
func (m *Manager) setConfig(ctx context.Context, key, value string) error {
	_, err := m.db.Raw().ExecContext(ctx,
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value)
	return err
}

// deleteConfig 删除配置键（把设置项恢复默认）。
func (m *Manager) deleteConfig(ctx context.Context, key string) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM config WHERE key = ?", key)
	return err
}
