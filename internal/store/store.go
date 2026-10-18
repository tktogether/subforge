// Package store 提供 SQLite 持久化存储。
//
// 对标 sub-poly 的 D1 存储方案，SubForge 用 modernc.org/sqlite（纯 Go 驱动，
// 零 cgo 依赖）替代 D1，数据文件默认 ./data/subforge.db，路径可配。
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB 是数据库连接池的封装。
type DB struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库，执行 WAL 模式与建表。
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	// 纯 Go 驱动用 database/sql 的连接池，限制为 1 避免锁竞争。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// WAL 模式：读写并发更好，崩溃恢复更安全。
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("建表: %w", err)
	}
	return &DB{db: db}, nil
}

// Close 关闭数据库连接。
func (d *DB) Close() error {
	return d.db.Close()
}

// Raw 返回底层 *sql.DB（供需要直接执行 SQL 的场景）。
func (d *DB) Raw() *sql.DB {
	return d.db
}

// --- 行类型与查询（internal/subforge 的订阅管理复用）---

// Airport 是 airports 表的一行。
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

// ProxyGroup 是 proxy_groups 表的一行。
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

// Rule 是 rules 表的一行。
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

// SubToken 是 sub_tokens 表的一行。
type SubToken struct {
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ListSubTokens 返回所有订阅令牌，按创建时间与令牌排序。
// name 列可为 NULL，用 COALESCE 兜底，避免扫描时报 "converting NULL to string"。
func (d *DB) ListSubTokens(ctx context.Context) ([]SubToken, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT token, COALESCE(name, ''), created_at FROM sub_tokens ORDER BY created_at, token")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SubToken{}
	for rows.Next() {
		var t SubToken
		if err := rows.Scan(&t.Token, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetProxyGroup 按 id 返回代理分组；不存在时返回 sql.ErrNoRows。
func (d *DB) GetProxyGroup(ctx context.Context, id int64) (*ProxyGroup, error) {
	var g ProxyGroup
	err := d.db.QueryRowContext(ctx,
		"SELECT id, name, type, COALESCE(regex, ''), COALESCE(url, ''), COALESCE(interval, 0), sort_order, created_at, updated_at FROM proxy_groups WHERE id = ?", id).
		Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// UpdateProxyGroup 按 id 更新代理分组（SQL 参数化）；目标不存在时返回 sql.ErrNoRows。
func (d *DB) UpdateProxyGroup(ctx context.Context, id int64, name, gtype, regex, url string, interval int) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE proxy_groups SET name = ?, type = ?, regex = ?, url = ?, interval = ?, updated_at = ? WHERE id = ?",
		name, gtype, regex, url, interval, time.Now(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetRule 按 id 返回规则；不存在时返回 sql.ErrNoRows。
func (d *DB) GetRule(ctx context.Context, id int64) (*Rule, error) {
	var r Rule
	err := d.db.QueryRowContext(ctx,
		"SELECT id, rule_type, COALESCE(rule_param, ''), rule_config, resolve_dns, enabled, sort_order, created_at, updated_at FROM rules WHERE id = ?", id).
		Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateRule 按 id 更新规则（SQL 参数化）；目标不存在时返回 sql.ErrNoRows。
func (d *DB) UpdateRule(ctx context.Context, id int64, ruleType, ruleParam string, ruleConfig int64, resolveDNS, enabled bool) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE rules SET rule_type = ?, rule_param = ?, rule_config = ?, resolve_dns = ?, enabled = ?, updated_at = ? WHERE id = ?",
		ruleType, ruleParam, ruleConfig, resolveDNS, enabled, time.Now(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// migrate 建表（幂等）。
func migrate(db *sql.DB) error {
	stmts := []string{
		// 订阅（airport）管理
		`CREATE TABLE IF NOT EXISTS airports (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT NOT NULL,
			subscription_url TEXT NOT NULL,
			remarks         TEXT,
			enabled         INTEGER DEFAULT 1,
			sort_order      INTEGER DEFAULT 0,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 代理分组管理
		`CREATE TABLE IF NOT EXISTS proxy_groups (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL,
			type        TEXT NOT NULL DEFAULT 'select',
			regex       TEXT,
			url         TEXT,
			interval    INTEGER,
			sort_order  INTEGER DEFAULT 0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 规则管理
		`CREATE TABLE IF NOT EXISTS rules (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_type   TEXT NOT NULL,
			rule_param  TEXT,
			rule_config INTEGER NOT NULL,
			resolve_dns INTEGER DEFAULT 1,
			enabled     INTEGER DEFAULT 1,
			sort_order  INTEGER DEFAULT 0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// rule-providers（从订阅导入的远程规则集）
		`CREATE TABLE IF NOT EXISTS rule_providers (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL,
			raw_json    TEXT NOT NULL,
			source      TEXT,
			enabled     INTEGER DEFAULT 1,
			sort_order  INTEGER DEFAULT 0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 自建节点
		`CREATE TABLE IF NOT EXISTS self_nodes (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			link        TEXT NOT NULL,
			name        TEXT,
			enabled     INTEGER DEFAULT 1,
			sort_order  INTEGER DEFAULT 0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 通用配置（DNS、mixed-port 等全局设置）
		`CREATE TABLE IF NOT EXISTS config (
			key         TEXT PRIMARY KEY,
			value       TEXT NOT NULL,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 订阅令牌（生成 /subscribe?token=xxx）
		`CREATE TABLE IF NOT EXISTS sub_tokens (
			token       TEXT PRIMARY KEY,
			name        TEXT,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表: %w", err)
		}
	}
	return nil
}
