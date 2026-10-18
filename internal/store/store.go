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
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	SubscriptionURL string `json:"subscription_url"`
	Remarks         string `json:"remarks"`
	Enabled         bool   `json:"enabled"`
	SortOrder       int    `json:"sort_order"`
	// —— 流量元数据：来自上游 subscription-userinfo 响应头 ——
	Upload     int64     `json:"upload"`      // 已用上行字节
	Download   int64     `json:"download"`    // 已用下行字节
	Total      int64     `json:"total"`       // 配额总字节，0 = 无限/未知
	Expire     int64     `json:"expire"`      // 到期 unix 秒，0 = 未知
	NodeCount  int       `json:"node_count"`  // 该源解析出的节点数（按节点加权策略用）
	TrafficAt  int64     `json:"traffic_at"`  // 上次成功采集的 unix 秒，0 = 从未成功
	TrafficErr string    `json:"traffic_err"` // 上次采集失败原因，成功时清空
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// FetchAudit 是 fetch_audit 表的一行：一次订阅拉取的审计记录。
type FetchAudit struct {
	ID         int64     `json:"id"`
	AirportID  int64     `json:"airport_id"`
	URL        string    `json:"url"`
	StatusCode int       `json:"status_code"`
	Bytes      int64     `json:"bytes"`
	OK         bool      `json:"ok"`
	Error      string    `json:"error"`
	DurationMS int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
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

// SelfNode 是 self_nodes 表的一行。
type SelfNode struct {
	ID        int64     `json:"id"`
	Link      string    `json:"link"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListSelfNodes 返回所有自建节点。
func (d *DB) ListSelfNodes(ctx context.Context) ([]SelfNode, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT id, link, COALESCE(name, ''), enabled, sort_order, created_at, updated_at FROM self_nodes ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SelfNode{}
	for rows.Next() {
		var n SelfNode
		if err := rows.Scan(&n.ID, &n.Link, &n.Name, &n.Enabled, &n.SortOrder, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetSelfNode 按 id 返回自建节点；不存在时返回 sql.ErrNoRows。
func (d *DB) GetSelfNode(ctx context.Context, id int64) (*SelfNode, error) {
	var n SelfNode
	err := d.db.QueryRowContext(ctx,
		"SELECT id, link, COALESCE(name, ''), enabled, sort_order, created_at, updated_at FROM self_nodes WHERE id = ?", id).
		Scan(&n.ID, &n.Link, &n.Name, &n.Enabled, &n.SortOrder, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// AddSelfNode 新增自建节点。
func (d *DB) AddSelfNode(ctx context.Context, link, name string, enabled bool) (*SelfNode, error) {
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO self_nodes (link, name, enabled, sort_order) VALUES (?, ?, ?, ?)",
		link, name, enabled, time.Now().UnixNano())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.GetSelfNode(ctx, id)
}

// UpdateSelfNode 按 id 更新自建节点。
func (d *DB) UpdateSelfNode(ctx context.Context, id int64, link, name string, enabled bool) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE self_nodes SET link = ?, name = ?, enabled = ?, updated_at = ? WHERE id = ?",
		link, name, enabled, time.Now(), id)
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

// DeleteSelfNode 按 id 删除自建节点。
func (d *DB) DeleteSelfNode(ctx context.Context, id int64) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM self_nodes WHERE id = ?", id)
	return err
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

// addColumnIfMissing 幂等加列：SQLite 没有 ADD COLUMN IF NOT EXISTS，
// 用 PRAGMA table_info 判定该列是否已存在，避免重复迁移报 duplicate column。
func addColumnIfMissing(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + decl)
	return err
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
		// 拉取审计（每次订阅拉取一条，只保留最近 N 条）
		`CREATE TABLE IF NOT EXISTS fetch_audit (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			airport_id  INTEGER NOT NULL,
			url         TEXT    NOT NULL,
			status_code INTEGER NOT NULL DEFAULT 0,
			bytes       INTEGER NOT NULL DEFAULT 0,
			ok          INTEGER NOT NULL DEFAULT 0,
			error       TEXT    NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at  INTEGER NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_fetch_audit_created ON fetch_audit(created_at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表: %w", err)
		}
	}
	// airports 的流量元数据列：老库用 ALTER 幂等补齐。
	// 不改上面的 CREATE TABLE，保证「新库/老库走同一条加列路径」，
	// 列定义只有一处事实源。
	for _, c := range []struct{ col, decl string }{
		{"upload", "INTEGER NOT NULL DEFAULT 0"},
		{"download", "INTEGER NOT NULL DEFAULT 0"},
		{"total", "INTEGER NOT NULL DEFAULT 0"},
		{"expire", "INTEGER NOT NULL DEFAULT 0"},
		{"node_count", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_at", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_err", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := addColumnIfMissing(db, "airports", c.col, c.decl); err != nil {
			return fmt.Errorf("迁移 airports.%s: %w", c.col, err)
		}
	}
	return nil
}

// AddFetchAudit 记一条拉取审计。CreatedAt 为零值时取当前时间。
func (d *DB) AddFetchAudit(ctx context.Context, a FetchAudit) error {
	sec := a.CreatedAt.Unix()
	if a.CreatedAt.IsZero() {
		sec = time.Now().Unix()
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO fetch_audit (airport_id, url, status_code, bytes, ok, error, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.AirportID, a.URL, a.StatusCode, a.Bytes, a.OK, a.Error, a.DurationMS, sec)
	return err
}

// ListFetchAudit 按 created_at 倒序、id 倒序返回最近 limit 条；limit <= 0 视为 100。
func (d *DB) ListFetchAudit(ctx context.Context, limit int) ([]FetchAudit, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, airport_id, url, status_code, bytes, ok, error, duration_ms, created_at
		 FROM fetch_audit ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FetchAudit{}
	for rows.Next() {
		var (
			a   FetchAudit
			sec int64
		)
		if err := rows.Scan(&a.ID, &a.AirportID, &a.URL, &a.StatusCode, &a.Bytes, &a.OK, &a.Error, &a.DurationMS, &sec); err != nil {
			return nil, err
		}
		a.CreatedAt = time.Unix(sec, 0).UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// PruneFetchAudit 只保留最近 keep 条；keep <= 0 时不做任何事。返回删除的行数。
func (d *DB) PruneFetchAudit(ctx context.Context, keep int) (int64, error) {
	if keep <= 0 {
		return 0, nil
	}
	res, err := d.db.ExecContext(ctx,
		`DELETE FROM fetch_audit WHERE id NOT IN (
			SELECT id FROM fetch_audit ORDER BY created_at DESC, id DESC LIMIT ?
		 )`, keep)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
