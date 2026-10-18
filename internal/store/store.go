// Package store 提供 SQLite 持久化存储。
//
// 对标 sub-poly 的 D1 存储方案，SubForge 用 modernc.org/sqlite（纯 Go 驱动，
// 零 cgo 依赖）替代 D1，数据文件默认 ./data/subforge.db，路径可配。
package store

import (
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
