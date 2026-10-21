// Package store 提供 SQLite 持久化存储。
//
// 对标 sub-poly 的 D1 存储方案，SubForge 用 modernc.org/sqlite（纯 Go 驱动，
// 零 cgo 依赖）替代 D1，数据文件默认 ./data/subforge.db，路径可配。
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
//
// ProfileID 标识该分组归属的订阅方案，0 = 未分配（旧全局池）。
// ProfileName 不落库，由 API 层按 ProfileID 填。
type ProxyGroup struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Regex       string    `json:"regex"`
	URL         string    `json:"url"`
	Interval    int       `json:"interval"`
	SortOrder   int       `json:"sort_order"`
	ProfileID   int64     `json:"profile_id"`
	ProfileName string    `json:"profile_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

// resolveSortOrder 把"用户显式指定的顺序"转成最终落库值：
// >0 用用户值；<=0 表示"没指定"，回退到纳秒时间戳（按创建时间自然排序）。
func resolveSortOrder(sortOrder int) int {
	if sortOrder > 0 {
		return sortOrder
	}
	return int(time.Now().UnixNano())
}

// AddSelfNode 新增自建节点；sortOrder>0 用用户值，否则回退 time.Now().UnixNano()。
func (d *DB) AddSelfNode(ctx context.Context, link, name string, enabled bool, sortOrder int) (*SelfNode, error) {
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO self_nodes (link, name, enabled, sort_order) VALUES (?, ?, ?, ?)",
		link, name, enabled, resolveSortOrder(sortOrder))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return d.GetSelfNode(ctx, id)
}

// UpdateSelfNode 按 id 更新自建节点；sortOrder>0 覆盖，<=0 保持原值。
func (d *DB) UpdateSelfNode(ctx context.Context, id int64, link, name string, enabled bool, sortOrder int) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE self_nodes SET link = ?, name = ?, enabled = ?, "+
			"sort_order = CASE WHEN ? > 0 THEN ? ELSE sort_order END, updated_at = ? WHERE id = ?",
		link, name, enabled, sortOrder, sortOrder, time.Now(), id)
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
	ID         int64  `json:"id"`
	RuleType   string `json:"rule_type"`
	RuleParam  string `json:"rule_param"`
	RuleConfig int64  `json:"rule_config"`
	ResolveDNS bool   `json:"resolve_dns"`
	Enabled    bool   `json:"enabled"`
	// —— 上游导入溯源：source 标识来源，raw 保存原始规则行，origin_key 为去重键 ——
	Source    string `json:"source"`
	Raw       string `json:"raw"`
	OriginKey string `json:"origin_key"`
	SortOrder int    `json:"sort_order"`
	// ProfileID 标识该规则归属的订阅方案，0 = 未分配（旧全局池）。
	ProfileID int64     `json:"profile_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Script 是 scripts 表的一行：goja 脚本库。
// 时间列用 INTEGER unix 秒（与 fetch_audit 一致），由 Create/Update 写入。
type Script struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Kind 是脚本类型：filter（过滤节点）/ operate（加工节点）。
	// 老库/老脚本为空串，由 API 层给默认值（POST 默认 filter），store 原样存储。
	Kind      string    `json:"kind"`
	Code      string    `json:"code"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Pipeline 是 pipelines 表的一行：有序步骤管线。
// StepsJSON 原样存字符串，store 不解析步骤语义。
type Pipeline struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StepsJSON   string    `json:"steps_json"`
	Enabled     bool      `json:"enabled"`
	SortOrder   int       `json:"sort_order"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RuleProvider 是 rule_providers 表的一行：从订阅导入的远程规则集。
type RuleProvider struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	RawJSON   string    `json:"raw_json"`
	Source    string    `json:"source"`
	Enabled   bool      `json:"enabled"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubToken 是 sub_tokens 表的一行。
//
// 五个定制字段 target/pipeline/config/include/exclude 是订阅默认值：
// 空串表示「不定制，沿用现有回落链」。优先级为 query 显式参数 > 令牌字段 > 原回落链，
// 详见 internal/api/subscription.go handleSubscribe。
type SubToken struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Target   string `json:"target"`
	Pipeline string `json:"pipeline"`
	Config   string `json:"config"`
	Include  string `json:"include"`
	Exclude  string `json:"exclude"`
	// ProfileID 标识令牌绑定的订阅方案，0 = 未绑定（行为与改动前完全一致）。
	// ProfileName 不落库，由 API 层按 ProfileID 填。
	ProfileID   int64     `json:"profile_id"`
	ProfileName string    `json:"profile_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListSubTokens 返回所有订阅令牌，按创建时间与令牌排序。
// name 与五个定制列可为 NULL（老库迁移前或未写），统一用 COALESCE 兜底为空串，
// 避免扫描时报 "converting NULL to string"。
func (d *DB) ListSubTokens(ctx context.Context) ([]SubToken, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT token, COALESCE(name, ''), COALESCE(target, ''), COALESCE(pipeline, ''), "+
			"COALESCE(config, ''), COALESCE(include, ''), COALESCE(exclude, ''), profile_id, created_at "+
			"FROM sub_tokens ORDER BY created_at, token")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SubToken{}
	for rows.Next() {
		var t SubToken
		if err := rows.Scan(&t.Token, &t.Name, &t.Target, &t.Pipeline, &t.Config,
			&t.Include, &t.Exclude, &t.ProfileID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SubTokenUpdate 是 UpdateSubToken 的部分更新输入：nil 指针 = 不动该列，
// 非 nil 指针 = 写入该值（空串 = 清空）。Name 同样按指针判定是否更新。
type SubTokenUpdate struct {
	Name      *string
	Target    *string
	Pipeline  *string
	Config    *string
	Include   *string
	Exclude   *string
	ProfileID *int64
}

// UpdateSubToken 按 token 部分更新订阅令牌：只 SET 非 nil 字段，nil 字段保持原值。
// 任何字段都没有提供（全 nil）时直接成功返回（幂等空更新）。
// token 不存在时返回 sql.ErrNoRows，与 store 内其他 Update 函数一致。
//
// 动态拼 SET 子句：列名是代码内常量（不来自用户输入），值走占位符，无注入面。
func (d *DB) UpdateSubToken(ctx context.Context, token string, u SubTokenUpdate) error {
	sets := make([]string, 0, 7)
	args := make([]any, 0, 8)
	if u.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *u.Name)
	}
	if u.Target != nil {
		sets = append(sets, "target = ?")
		args = append(args, *u.Target)
	}
	if u.Pipeline != nil {
		sets = append(sets, "pipeline = ?")
		args = append(args, *u.Pipeline)
	}
	if u.Config != nil {
		sets = append(sets, "config = ?")
		args = append(args, *u.Config)
	}
	if u.Include != nil {
		sets = append(sets, "include = ?")
		args = append(args, *u.Include)
	}
	if u.Exclude != nil {
		sets = append(sets, "exclude = ?")
		args = append(args, *u.Exclude)
	}
	if u.ProfileID != nil {
		sets = append(sets, "profile_id = ?")
		args = append(args, *u.ProfileID)
	}
	if len(sets) == 0 {
		// 全 nil：仍需校验 token 是否存在，否则空更新会静默成功一个不存在的令牌。
		var exists string
		err := d.db.QueryRowContext(ctx, "SELECT token FROM sub_tokens WHERE token = ?", token).Scan(&exists)
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return err
	}
	query := "UPDATE sub_tokens SET " + strings.Join(sets, ", ") + " WHERE token = ?"
	args = append(args, token)
	res, err := d.db.ExecContext(ctx, query, args...)
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

// AddProxyGroup 新增代理分组；sortOrder>0 用用户值，否则回退 time.Now().UnixNano()。
//
// 旧签名保持不变：分组归属「未分配」（profile_id=0），等价于 AddProxyGroupWithProfile(ctx, 0, …)。
func (d *DB) AddProxyGroup(ctx context.Context, name, gtype, regex, url string, interval, sortOrder int) (*ProxyGroup, error) {
	return d.AddProxyGroupWithProfile(ctx, 0, name, gtype, regex, url, interval, sortOrder)
}

// GetProxyGroup 按 id 返回代理分组；不存在时返回 sql.ErrNoRows。
func (d *DB) GetProxyGroup(ctx context.Context, id int64) (*ProxyGroup, error) {
	var g ProxyGroup
	err := d.db.QueryRowContext(ctx,
		"SELECT id, name, type, COALESCE(regex, ''), COALESCE(url, ''), COALESCE(interval, 0), sort_order, profile_id, created_at, updated_at FROM proxy_groups WHERE id = ?", id).
		Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval, &g.SortOrder, &g.ProfileID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// UpdateProxyGroup 按 id 更新代理分组（SQL 参数化）；目标不存在时返回 sql.ErrNoRows。
// sortOrder>0 覆盖，<=0 保持原值。
func (d *DB) UpdateProxyGroup(ctx context.Context, id int64, name, gtype, regex, url string, interval, sortOrder int) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE proxy_groups SET name = ?, type = ?, regex = ?, url = ?, interval = ?, "+
			"sort_order = CASE WHEN ? > 0 THEN ? ELSE sort_order END, updated_at = ? WHERE id = ?",
		name, gtype, regex, url, interval, sortOrder, sortOrder, time.Now(), id)
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
		"SELECT id, rule_type, COALESCE(rule_param, ''), rule_config, resolve_dns, enabled, COALESCE(source, ''), COALESCE(raw, ''), COALESCE(origin_key, ''), sort_order, profile_id, created_at, updated_at FROM rules WHERE id = ?", id).
		Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled, &r.Source, &r.Raw, &r.OriginKey, &r.SortOrder, &r.ProfileID, &r.CreatedAt, &r.UpdatedAt)
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
	// sub_tokens 五个定制列：老库用 ALTER 幂等补齐。
	// 不改上面的 CREATE TABLE（建表语句保持三列历史形态），保证「新库/老库走同一条加列路径」，
	// 列定义只有一处事实源。空串 = 现状行为（不定制，沿用回落链）。
	for _, c := range []struct{ col, decl string }{
		{"target", "TEXT NOT NULL DEFAULT ''"},
		{"pipeline", "TEXT NOT NULL DEFAULT ''"},
		{"config", "TEXT NOT NULL DEFAULT ''"},
		{"include", "TEXT NOT NULL DEFAULT ''"},
		{"exclude", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := addColumnIfMissing(db, "sub_tokens", c.col, c.decl); err != nil {
			return fmt.Errorf("迁移 sub_tokens.%s: %w", c.col, err)
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
	// scripts / pipelines 新表（CREATE TABLE IF NOT EXISTS 幂等）。
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS scripts (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			kind        TEXT NOT NULL DEFAULT '',
			code        TEXT NOT NULL,
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  INTEGER NOT NULL DEFAULT 0,
			updated_at  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS pipelines (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			steps_json  TEXT NOT NULL DEFAULT '[]',
			enabled     INTEGER NOT NULL DEFAULT 1,
			sort_order  INTEGER NOT NULL DEFAULT 0,
			source      TEXT NOT NULL DEFAULT 'ui',
			created_at  INTEGER NOT NULL DEFAULT 0,
			updated_at  INTEGER NOT NULL DEFAULT 0
		)`,
		// 订阅方案（Profile）：一个方案 = 一套模板 + 一套规则 + 一套分组。
		// config 为 INI 模板（远程 URL 或本地路径），空串 = 用默认配置；
		// pipeline 为默认管线名，空串 = 用全局回落；enabled=0 = 方案停用。
		`CREATE TABLE IF NOT EXISTS profiles (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			config      TEXT NOT NULL DEFAULT '',
			pipeline    TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			sort_order  INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表: %w", err)
		}
	}
	// scripts.kind：老库用 ALTER 幂等补齐（与上面 CREATE TABLE 同一定义，
	// 老脚本迁移后 kind 为空串，不改变历史数据语义）。
	if err := addColumnIfMissing(db, "scripts", "kind", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("迁移 scripts.kind: %w", err)
	}
	// rules 溯源列：老库用 ALTER 幂等补齐。
	for _, c := range []struct{ col, decl string }{
		{"source", "TEXT NOT NULL DEFAULT ''"},
		{"raw", "TEXT NOT NULL DEFAULT ''"},
		{"origin_key", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := addColumnIfMissing(db, "rules", c.col, c.decl); err != nil {
			return fmt.Errorf("迁移 rules.%s: %w", c.col, err)
		}
	}
	// 方案归属列 profile_id：rules / proxy_groups / sub_tokens 三表统一补齐。
	// 0 = 未分配（未绑定方案），语义与改动前完全一致，老数据迁移后一律落在 0。
	for _, t := range []string{"rules", "proxy_groups", "sub_tokens"} {
		if err := addColumnIfMissing(db, t, "profile_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("迁移 %s.profile_id: %w", t, err)
		}
	}
	// rule_providers 唯一索引：先按 (source, name) 去重，保留最小 id，避免老库重复行导致建索引失败。
	if _, err := db.Exec("DELETE FROM rule_providers WHERE id NOT IN (SELECT MIN(id) FROM rule_providers GROUP BY source, name)"); err != nil {
		return fmt.Errorf("去重 rule_providers: %w", err)
	}
	// 部分索引：只约束真正带 origin_key 的导入行；手工规则（origin_key=''）不被约束，允许多行共存。
	// 先 DROP 再 CREATE：老库的索引是 (source, origin_key)，光靠 IF NOT EXISTS 会把旧索引留下来，
	// 导致不同方案的「同来源同去重键」规则无法共存。DROP 每次启动执行，成本可接受。
	if _, err := db.Exec("DROP INDEX IF EXISTS idx_rules_origin"); err != nil {
		return fmt.Errorf("删除旧索引 idx_rules_origin: %w", err)
	}
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_rules_origin ON rules(profile_id, source, origin_key) WHERE origin_key <> ''"); err != nil {
		return fmt.Errorf("创建 idx_rules_origin: %w", err)
	}
	// 同理：只约束带 source 的导入行；手工 provider（source=''）可共存。
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_providers_source_name ON rule_providers(source, name) WHERE source <> ''"); err != nil {
		return fmt.Errorf("创建 idx_rule_providers_source_name: %w", err)
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

// --- scripts（goja 脚本库）---

// ListScripts 返回所有脚本，按 name 排序。
func (d *DB) ListScripts(ctx context.Context) ([]Script, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT id, name, COALESCE(description, ''), kind, code, enabled, created_at, updated_at FROM scripts ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Script{}
	for rows.Next() {
		var (
			s          Script
			createdSec int64
			updatedSec int64
		)
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Kind, &s.Code, &s.Enabled, &createdSec, &updatedSec); err != nil {
			return nil, err
		}
		s.CreatedAt = time.Unix(createdSec, 0).UTC()
		s.UpdatedAt = time.Unix(updatedSec, 0).UTC()
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetScript 按 id 返回脚本；不存在时返回 sql.ErrNoRows。
func (d *DB) GetScript(ctx context.Context, id int64) (Script, error) {
	var (
		s          Script
		createdSec int64
		updatedSec int64
	)
	err := d.db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(description, ''), kind, code, enabled, created_at, updated_at FROM scripts WHERE id = ?", id).
		Scan(&s.ID, &s.Name, &s.Description, &s.Kind, &s.Code, &s.Enabled, &createdSec, &updatedSec)
	if err != nil {
		return Script{}, err
	}
	s.CreatedAt = time.Unix(createdSec, 0).UTC()
	s.UpdatedAt = time.Unix(updatedSec, 0).UTC()
	return s, nil
}

// GetScriptByName 按 name 返回脚本；不存在时返回 sql.ErrNoRows（热路径）。
func (d *DB) GetScriptByName(ctx context.Context, name string) (Script, error) {
	var (
		s          Script
		createdSec int64
		updatedSec int64
	)
	err := d.db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(description, ''), kind, code, enabled, created_at, updated_at FROM scripts WHERE name = ?", name).
		Scan(&s.ID, &s.Name, &s.Description, &s.Kind, &s.Code, &s.Enabled, &createdSec, &updatedSec)
	if err != nil {
		return Script{}, err
	}
	s.CreatedAt = time.Unix(createdSec, 0).UTC()
	s.UpdatedAt = time.Unix(updatedSec, 0).UTC()
	return s, nil
}

// CreateScript 新增脚本，返回带 ID 的副本。
func (d *DB) CreateScript(ctx context.Context, s Script) (*Script, error) {
	now := time.Now().UTC()
	created := s.CreatedAt
	if created.IsZero() {
		created = now
	}
	updated := s.UpdatedAt
	if updated.IsZero() {
		updated = now
	}
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO scripts (name, description, kind, code, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		s.Name, s.Description, s.Kind, s.Code, s.Enabled, created.Unix(), updated.Unix())
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	got, err := d.GetScript(ctx, id)
	if err != nil {
		return nil, err
	}
	return &got, nil
}

// UpdateScript 按 id 更新脚本；RowsAffected==0 返回 sql.ErrNoRows。
func (d *DB) UpdateScript(ctx context.Context, s Script) error {
	now := time.Now().UTC()
	updated := s.UpdatedAt
	if updated.IsZero() {
		updated = now
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE scripts SET name = ?, description = ?, kind = ?, code = ?, enabled = ?, updated_at = ? WHERE id = ?",
		s.Name, s.Description, s.Kind, s.Code, s.Enabled, updated.Unix(), s.ID)
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

// DeleteScript 按 id 删除脚本；RowsAffected==0 返回 sql.ErrNoRows。
func (d *DB) DeleteScript(ctx context.Context, id int64) error {
	res, err := d.db.ExecContext(ctx, "DELETE FROM scripts WHERE id = ?", id)
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

// --- pipelines（有序步骤管线）---

// ListPipelines 返回所有管线，按 sort_order, name 排序。
func (d *DB) ListPipelines(ctx context.Context) ([]Pipeline, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT id, name, COALESCE(description, ''), steps_json, enabled, sort_order, COALESCE(source, ''), created_at, updated_at FROM pipelines ORDER BY sort_order, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Pipeline{}
	for rows.Next() {
		var (
			p          Pipeline
			createdSec int64
			updatedSec int64
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.StepsJSON, &p.Enabled, &p.SortOrder, &p.Source, &createdSec, &updatedSec); err != nil {
			return nil, err
		}
		p.CreatedAt = time.Unix(createdSec, 0).UTC()
		p.UpdatedAt = time.Unix(updatedSec, 0).UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPipeline 按 id 返回管线；不存在时返回 sql.ErrNoRows。
func (d *DB) GetPipeline(ctx context.Context, id int64) (Pipeline, error) {
	var (
		p          Pipeline
		createdSec int64
		updatedSec int64
	)
	err := d.db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(description, ''), steps_json, enabled, sort_order, COALESCE(source, ''), created_at, updated_at FROM pipelines WHERE id = ?", id).
		Scan(&p.ID, &p.Name, &p.Description, &p.StepsJSON, &p.Enabled, &p.SortOrder, &p.Source, &createdSec, &updatedSec)
	if err != nil {
		return Pipeline{}, err
	}
	p.CreatedAt = time.Unix(createdSec, 0).UTC()
	p.UpdatedAt = time.Unix(updatedSec, 0).UTC()
	return p, nil
}

// GetPipelineByName 按 name 返回管线；不存在时返回 sql.ErrNoRows。
func (d *DB) GetPipelineByName(ctx context.Context, name string) (Pipeline, error) {
	var (
		p          Pipeline
		createdSec int64
		updatedSec int64
	)
	err := d.db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(description, ''), steps_json, enabled, sort_order, COALESCE(source, ''), created_at, updated_at FROM pipelines WHERE name = ?", name).
		Scan(&p.ID, &p.Name, &p.Description, &p.StepsJSON, &p.Enabled, &p.SortOrder, &p.Source, &createdSec, &updatedSec)
	if err != nil {
		return Pipeline{}, err
	}
	p.CreatedAt = time.Unix(createdSec, 0).UTC()
	p.UpdatedAt = time.Unix(updatedSec, 0).UTC()
	return p, nil
}

// CreatePipeline 新增管线，返回带 ID 的副本。
func (d *DB) CreatePipeline(ctx context.Context, p Pipeline) (*Pipeline, error) {
	now := time.Now().UTC()
	created := p.CreatedAt
	if created.IsZero() {
		created = now
	}
	updated := p.UpdatedAt
	if updated.IsZero() {
		updated = now
	}
	if p.StepsJSON == "" {
		p.StepsJSON = "[]"
	}
	if p.Source == "" {
		p.Source = "ui"
	}
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO pipelines (name, description, steps_json, enabled, sort_order, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		p.Name, p.Description, p.StepsJSON, p.Enabled, p.SortOrder, p.Source, created.Unix(), updated.Unix())
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	got, err := d.GetPipeline(ctx, id)
	if err != nil {
		return nil, err
	}
	return &got, nil
}

// UpdatePipeline 按 id 更新管线；RowsAffected==0 返回 sql.ErrNoRows。
func (d *DB) UpdatePipeline(ctx context.Context, p Pipeline) error {
	now := time.Now().UTC()
	updated := p.UpdatedAt
	if updated.IsZero() {
		updated = now
	}
	steps := p.StepsJSON
	if steps == "" {
		steps = "[]"
	}
	src := p.Source
	if src == "" {
		src = "ui"
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE pipelines SET name = ?, description = ?, steps_json = ?, enabled = ?, sort_order = ?, source = ?, updated_at = ? WHERE id = ?",
		p.Name, p.Description, steps, p.Enabled, p.SortOrder, src, updated.Unix(), p.ID)
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

// DeletePipeline 按 id 删除管线；RowsAffected==0 返回 sql.ErrNoRows。
func (d *DB) DeletePipeline(ctx context.Context, id int64) error {
	res, err := d.db.ExecContext(ctx, "DELETE FROM pipelines WHERE id = ?", id)
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

// --- rule_providers（远程规则集）---

// ListRuleProviders 返回所有 rule providers，按 sort_order, name 排序。
func (d *DB) ListRuleProviders(ctx context.Context) ([]RuleProvider, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT id, name, raw_json, COALESCE(source, ''), enabled, sort_order, created_at, updated_at FROM rule_providers ORDER BY sort_order, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RuleProvider{}
	for rows.Next() {
		var rp RuleProvider
		if err := rows.Scan(&rp.ID, &rp.Name, &rp.RawJSON, &rp.Source, &rp.Enabled, &rp.SortOrder, &rp.CreatedAt, &rp.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rp)
	}
	return out, rows.Err()
}

// UpsertRuleProvider 按 (source, name) 幂等写入。
func (d *DB) UpsertRuleProvider(ctx context.Context, p RuleProvider) error {
	now := time.Now().UTC()
	// 查找已有行
	var id int64
	err := d.db.QueryRowContext(ctx,
		"SELECT id FROM rule_providers WHERE source = ? AND name = ?", p.Source, p.Name).
		Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	enabled := p.Enabled
	source := p.Source
	if source == "" {
		source = ""
	}
	if err == sql.ErrNoRows {
		_, err = d.db.ExecContext(ctx,
			"INSERT INTO rule_providers (name, raw_json, source, enabled, sort_order, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			p.Name, p.RawJSON, source, enabled, p.SortOrder, now, now)
		return err
	}
	_, err = d.db.ExecContext(ctx,
		"UPDATE rule_providers SET raw_json = ?, enabled = ?, sort_order = ?, updated_at = ? WHERE id = ?",
		p.RawJSON, enabled, p.SortOrder, now, id)
	return err
}

// SetRuleProviderEnabled 按 id 更新规则集启用状态；行不存在返回 (false, nil)。
func (d *DB) SetRuleProviderEnabled(ctx context.Context, id int64, enabled bool) (bool, error) {
	now := time.Now().UTC()
	res, err := d.db.ExecContext(ctx,
		"UPDATE rule_providers SET enabled = ?, updated_at = ? WHERE id = ?",
		enabled, now, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
