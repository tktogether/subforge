// Package subforge 提供订阅管理的业务逻辑。
//
// 对标 sub-poly 的 D1 数据模型，用 SQLite 持久化订阅、规则、分组等。
package subforge

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tktogether/subforge/internal/store"
)

// Manager 是订阅管理的业务入口。
type Manager struct {
	db *store.DB
}

// NewManager 创建 Manager，打开（或创建）SQLite 数据库。
func NewManager(dsn string) (*Manager, error) {
	db, err := store.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &Manager{db: db}, nil
}

// Close 关闭数据库连接。
func (m *Manager) Close() error { return m.db.Close() }

// DB 返回底层 *store.DB。
func (m *Manager) DB() *store.DB { return m.db }

// --- Airport（订阅）管理 ---

// Airport 是一个订阅源。
type Airport struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	SubscriptionURL string    `json:"subscription_url"`
	Remarks         string    `json:"remarks"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListAirports 返回所有订阅。
func (m *Manager) ListAirports(ctx context.Context) ([]Airport, error) {
	rows, err := m.db.Raw().QueryContext(ctx,
		"SELECT id, name, subscription_url, remarks, enabled, sort_order, created_at, updated_at FROM airports ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Airport
	for rows.Next() {
		var a Airport
		if err := rows.Scan(&a.ID, &a.Name, &a.SubscriptionURL, &a.Remarks, &a.Enabled, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt); err != nil {
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
		"SELECT id, name, subscription_url, remarks, enabled, sort_order, created_at, updated_at FROM airports WHERE id = ?", id).
		Scan(&a.ID, &a.Name, &a.SubscriptionURL, &a.Remarks, &a.Enabled, &a.SortOrder, &a.CreatedAt, &a.UpdatedAt)
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

// --- ProxyGroup（代理分组）管理 ---

// ProxyGroup 是一个代理分组。
type ProxyGroup struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Regex     string    `json:"regex"`
	URL       string    `json:"url"`
	Interval  int       `json:"interval"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListProxyGroups 返回所有代理分组。
func (m *Manager) ListProxyGroups(ctx context.Context) ([]ProxyGroup, error) {
	rows, err := m.db.Raw().QueryContext(ctx,
		"SELECT id, name, type, regex, url, interval, sort_order, created_at, updated_at FROM proxy_groups ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProxyGroup
	for rows.Next() {
		var g ProxyGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// AddProxyGroup 新增代理分组。
func (m *Manager) AddProxyGroup(ctx context.Context, name, typ, regex, url string, interval int) (*ProxyGroup, error) {
	res, err := m.db.Raw().ExecContext(ctx,
		"INSERT INTO proxy_groups (name, type, regex, url, interval, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		name, typ, regex, url, interval, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var g ProxyGroup
	err = m.db.Raw().QueryRowContext(ctx,
		"SELECT id, name, type, regex, url, interval, sort_order, created_at, updated_at FROM proxy_groups WHERE id = ?", id).
		Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// DeleteProxyGroup 删除代理分组。
func (m *Manager) DeleteProxyGroup(ctx context.Context, id int64) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM proxy_groups WHERE id = ?", id)
	return err
}

// --- Rule（规则）管理 ---

// Rule 是一条分流规则。
type Rule struct {
	ID         int64     `json:"id"`
	RuleType   string    `json:"rule_type"`
	RuleParam  string    `json:"rule_param"`
	RuleConfig int64     `json:"rule_config"`
	ResolveDNS bool      `json:"resolve_dns"`
	Enabled    bool      `json:"enabled"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ListRules 返回所有规则。
func (m *Manager) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := m.db.Raw().QueryContext(ctx,
		"SELECT id, rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, created_at, updated_at FROM rules ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
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

// SubToken 是一个订阅令牌。
type SubToken struct {
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateSubToken 新增订阅令牌。
func (m *Manager) CreateSubToken(ctx context.Context, token, name string) error {
	_, err := m.db.Raw().ExecContext(ctx, "INSERT INTO sub_tokens (token, name) VALUES (?, ?)", token, name)
	return err
}

// GetSubToken 返回订阅令牌。
func (m *Manager) GetSubToken(ctx context.Context, token string) (*SubToken, error) {
	var t SubToken
	err := m.db.Raw().QueryRowContext(ctx, "SELECT token, name, created_at FROM sub_tokens WHERE token = ?", token).
		Scan(&t.Token, &t.Name, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("令牌 %s 不存在", token)
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteSubToken 删除订阅令牌。
func (m *Manager) DeleteSubToken(ctx context.Context, token string) error {
	_, err := m.db.Raw().ExecContext(ctx, "DELETE FROM sub_tokens WHERE token = ?", token)
	return err
}
