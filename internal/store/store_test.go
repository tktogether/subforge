package store

// internal/store 补测：把 store.go 的 14 个零覆盖函数全部打到。
// 纪律：不改 store.go；每个测试用 t.TempDir() 造独立 SQLite 文件（不用 :memory:），
// 因为 Open 会执行 WAL pragma，落文件最稳。

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestDB 打开一个临时文件数据库，返回 *DB 与 context。
// Open 失败直接 Fatal——后续断言都建立在库已打开的前提上。
func newTestDB(t *testing.T) (*DB, context.Context) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "store.db")
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open(%q) 失败: %v", dsn, err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d, context.Background()
}

// 期望的 7 张表（migrate 建表，全部 CREATE TABLE IF NOT EXISTS）。
var wantTables = []string{
	"airports", "config", "pipelines", "proxy_groups", "rule_providers", "rules", "scripts", "self_nodes", "sub_tokens",
}

// TestOpenCloseRaw 覆盖 Open / Close / Raw 与 migrate 建表。
func TestOpenCloseRaw(t *testing.T) {
	d, ctx := newTestDB(t)

	// Raw() 非 nil 且可 Ping
	if d.Raw() == nil {
		t.Fatal("Raw() 返回 nil")
	}
	if err := d.Raw().PingContext(ctx); err != nil {
		t.Fatalf("Raw().PingContext 失败: %v", err)
	}

	// 7 张表都要存在
	rows, err := d.Raw().QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("查询 sqlite_master 失败: %v", err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Scan 表名失败: %v", err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历表名失败: %v", err)
	}
	for _, tbl := range wantTables {
		if !got[tbl] {
			t.Errorf("缺少表 %s", tbl)
		}
	}

	// Close 返回 nil
	if err := d.Close(); err != nil {
		t.Fatalf("Close() 返回 %v，期望 nil", err)
	}
}

// TestOpenIsIdempotent 覆盖 migrate 的重复执行路径（CREATE TABLE IF NOT EXISTS 幂等）。
func TestOpenIsIdempotent(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "store.db")
	d1, err := Open(dsn)
	if err != nil {
		t.Fatalf("首次 Open 失败: %v", err)
	}
	if err := d1.Close(); err != nil {
		t.Fatalf("首次 Close 失败: %v", err)
	}
	// 同一 dsn 再 Open，migrate 重复执行应无错
	d2, err := Open(dsn)
	if err != nil {
		t.Fatalf("二次 Open（重复 migrate）失败: %v", err)
	}
	defer d2.Close()
	if err := d2.Raw().PingContext(context.Background()); err != nil {
		t.Fatalf("二次 Open 后 Raw().PingContext 失败: %v", err)
	}
}

// TestSelfNodeCRUD 覆盖 ListSelfNodes / AddSelfNode / GetSelfNode / UpdateSelfNode / DeleteSelfNode。
func TestSelfNodeCRUD(t *testing.T) {
	d, ctx := newTestDB(t)

	// 空库 ListSelfNodes → len 0 且非 nil（源码 out := []SelfNode{}）
	empty, err := d.ListSelfNodes(ctx)
	if err != nil {
		t.Fatalf("空库 ListSelfNodes 失败: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("空库 ListSelfNodes 长度 %d，期望 0", len(empty))
	}
	if empty == nil {
		t.Fatal("空库 ListSelfNodes 返回 nil，期望非 nil 空切片")
	}

	// Add 两个节点
	n1, err := d.AddSelfNode(ctx, "vless://a", "n1", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode(n1) 失败: %v", err)
	}
	if n1.Link != "vless://a" || n1.Name != "n1" || !n1.Enabled || n1.ID <= 0 {
		t.Fatalf("AddSelfNode(n1) 返回字段不符: %+v", n1)
	}
	if _, err := d.AddSelfNode(ctx, "trojan://b", "n2", false, 0); err != nil {
		t.Fatalf("AddSelfNode(n2) 失败: %v", err)
	}

	// List 长度 2
	list, err := d.ListSelfNodes(ctx)
	if err != nil {
		t.Fatalf("ListSelfNodes 失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListSelfNodes 长度 %d，期望 2", len(list))
	}

	// GetSelfNode 命中
	got1, err := d.GetSelfNode(ctx, n1.ID)
	if err != nil {
		t.Fatalf("GetSelfNode(id1) 失败: %v", err)
	}
	if got1.Name != "n1" {
		t.Fatalf("GetSelfNode(id1).Name=%q，期望 n1", got1.Name)
	}

	// UpdateSelfNode 成功
	if err := d.UpdateSelfNode(ctx, n1.ID, "vmess://c", "n1b", false, 0); err != nil {
		t.Fatalf("UpdateSelfNode 失败: %v", err)
	}
	got1b, err := d.GetSelfNode(ctx, n1.ID)
	if err != nil {
		t.Fatalf("Update 后 GetSelfNode 失败: %v", err)
	}
	if got1b.Link != "vmess://c" || got1b.Name != "n1b" || got1b.Enabled {
		t.Fatalf("UpdateSelfNode 未生效: %+v", got1b)
	}

	// DeleteSelfNode 成功，再 Get → sql.ErrNoRows
	if err := d.DeleteSelfNode(ctx, n1.ID); err != nil {
		t.Fatalf("DeleteSelfNode 失败: %v", err)
	}
	if _, err := d.GetSelfNode(ctx, n1.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("删除后 GetSelfNode 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestSelfNodeNotFound 覆盖 GetSelfNode / UpdateSelfNode 不存在时的 sql.ErrNoRows 分支。
func TestSelfNodeNotFound(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.GetSelfNode(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetSelfNode(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdateSelfNode(ctx, 9999, "x", "y", true, 0); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateSelfNode(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestListSubTokens 覆盖 ListSubTokens 排序与 COALESCE(name,”) 兜底。
func TestListSubTokens(t *testing.T) {
	d, ctx := newTestDB(t)

	// 空库 → len 0 且非 nil
	empty, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("空库 ListSubTokens 失败: %v", err)
	}
	if len(empty) != 0 || empty == nil {
		t.Fatalf("空库 ListSubTokens = %v(len=%d)，期望非 nil 空切片", empty, len(empty))
	}

	// 直接 INSERT 两条（created_at 显式，验证排序）
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, created_at) VALUES (?, ?, ?)",
		"tok-b", "B", "2024-01-02 00:00:00"); err != nil {
		t.Fatalf("INSERT tok-b 失败: %v", err)
	}
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, created_at) VALUES (?, ?, ?)",
		"tok-a", "A", "2024-01-01 00:00:00"); err != nil {
		t.Fatalf("INSERT tok-a 失败: %v", err)
	}

	list, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens 失败: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListSubTokens 长度 %d，期望 2", len(list))
	}
	if list[0].Token != "tok-a" || list[1].Token != "tok-b" {
		t.Fatalf("ListSubTokens 排序错误: [%s %s]，期望 [tok-a tok-b]", list[0].Token, list[1].Token)
	}

	// name 为 NULL 的行 → COALESCE 兜底为 ""
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, created_at) VALUES (?, ?, ?)",
		"tok-c", nil, "2024-01-03 00:00:00"); err != nil {
		t.Fatalf("INSERT tok-c 失败: %v", err)
	}
	list2, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens(含 NULL) 失败: %v", err)
	}
	if len(list2) != 3 {
		t.Fatalf("ListSubTokens 长度 %d，期望 3", len(list2))
	}
	var c *SubToken
	for i := range list2 {
		if list2[i].Token == "tok-c" {
			c = &list2[i]
		}
	}
	if c == nil {
		t.Fatal("未找到 tok-c 行")
	}
	if c.Name != "" {
		t.Fatalf("tok-c.Name=%q，期望空字符串（COALESCE 兜底）", c.Name)
	}
}

// TestProxyGroupGetUpdate 覆盖 GetProxyGroup / UpdateProxyGroup（含不存在分支）。
func TestProxyGroupGetUpdate(t *testing.T) {
	d, ctx := newTestDB(t)

	// store.go 没有 AddProxyGroup，直接 INSERT 造行
	res, err := d.Raw().ExecContext(ctx,
		"INSERT INTO proxy_groups (name, type, regex, url, interval, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		"g1", "select", "re:^HK", "http://x", 300, 1)
	if err != nil {
		t.Fatalf("INSERT proxy_groups 失败: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId 失败: %v", err)
	}

	g, err := d.GetProxyGroup(ctx, id)
	if err != nil {
		t.Fatalf("GetProxyGroup 失败: %v", err)
	}
	if g.Name != "g1" || g.Type != "select" || g.Regex != "re:^HK" || g.URL != "http://x" || g.Interval != 300 {
		t.Fatalf("GetProxyGroup 字段不符: %+v", g)
	}

	if err := d.UpdateProxyGroup(ctx, id, "g1b", "url-test", "re:^JP", "http://y", 600, 0); err != nil {
		t.Fatalf("UpdateProxyGroup 失败: %v", err)
	}
	g2, err := d.GetProxyGroup(ctx, id)
	if err != nil {
		t.Fatalf("Update 后 GetProxyGroup 失败: %v", err)
	}
	if g2.Name != "g1b" || g2.Type != "url-test" || g2.Regex != "re:^JP" || g2.URL != "http://y" || g2.Interval != 600 {
		t.Fatalf("UpdateProxyGroup 未生效: %+v", g2)
	}

	if _, err := d.GetProxyGroup(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetProxyGroup(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdateProxyGroup(ctx, 9999, "x", "y", "z", "w", 1, 0); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateProxyGroup(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestRuleGetUpdate 覆盖 GetRule / UpdateRule（含不存在分支）。
func TestRuleGetUpdate(t *testing.T) {
	d, ctx := newTestDB(t)

	res, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		"DOMAIN-SUFFIX", "example.com", 1, 1, 1, 1)
	if err != nil {
		t.Fatalf("INSERT rules 失败: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId 失败: %v", err)
	}

	r, err := d.GetRule(ctx, id)
	if err != nil {
		t.Fatalf("GetRule 失败: %v", err)
	}
	if r.RuleType != "DOMAIN-SUFFIX" || r.RuleParam != "example.com" || r.RuleConfig != 1 || !r.ResolveDNS || !r.Enabled {
		t.Fatalf("GetRule 字段不符: %+v", r)
	}

	if err := d.UpdateRule(ctx, id, "IP-CIDR", "1.2.3.0/24", 2, false, false); err != nil {
		t.Fatalf("UpdateRule 失败: %v", err)
	}
	r2, err := d.GetRule(ctx, id)
	if err != nil {
		t.Fatalf("Update 后 GetRule 失败: %v", err)
	}
	if r2.RuleType != "IP-CIDR" || r2.RuleParam != "1.2.3.0/24" || r2.RuleConfig != 2 || r2.ResolveDNS || r2.Enabled {
		t.Fatalf("UpdateRule 未生效: %+v", r2)
	}

	if _, err := d.GetRule(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetRule(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdateRule(ctx, 9999, "x", "y", 1, true, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateRule(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// --- 错误分支覆盖（以下测试用 BadDriver / 不可写路径 / closed-DB / 坏扫描 触发 err 分支）---

// TestOpenUnwritablePath 覆盖 Open 的 PRAGMA journal_mode=WAL 失败分支（L33-36）。
// Open 对不可写目录的 dsn 会在 db.Exec("PRAGMA ...") 报 "unable to open database file"。
func TestOpenUnwritablePath(t *testing.T) {
	// 把 dsn 放进一个不存在的子目录，WAL pragma 落盘失败。
	bad := filepath.Join(t.TempDir(), "missing_subdir", "store.db")
	d, err := Open(bad)
	if err == nil {
		_ = d.Close()
		t.Fatal("Open(不可写路径) 期望失败，实际成功")
	}
	// 错误信息应包含 "设置 WAL"（store.go L35 的 fmt.Errorf 前缀）
	if !strings.Contains(err.Error(), "设置 WAL") {
		t.Fatalf("Open(不可写路径) 错误=%v，期望包含 设置 WAL", err)
	}
}

// TestMigrateOnClosedDB 覆盖 migrate 的 db.Exec 失败分支（L336-338）。
// 直接调 migrate(已关闭的 *sql.DB)，建表 Exec 返回 "database is closed"。
func TestMigrateOnClosedDB(t *testing.T) {
	dir := t.TempDir()
	raw, err := sql.Open("sqlite", filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatalf("sql.Open 失败: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close 失败: %v", err)
	}
	// migrate 是包内未导出函数，本测试文件同包可直接调用。
	// db 已关闭，建表 Exec 返回 "database is closed"，走 L336-338 err 分支。
	err = migrate(raw)
	if err == nil {
		t.Fatal("migrate(已关闭 db) 期望失败，实际成功")
	}
	if !strings.Contains(err.Error(), "执行建表") {
		t.Fatalf("migrate(已关闭 db) 错误=%v，期望包含 执行建表", err)
	}
}

// --- closed-DB 错误分支：覆盖各函数 ExecContext/QueryContext 的 err 分支 ---
// 关闭连接后调用，触发 "sql: database is closed"。

func TestClosedDBErrors(t *testing.T) {
	d, ctx := newTestDB(t)
	// 先关闭，再用 ctx 调用各方法——ctx 有效但 db 已关，触发 err 分支。
	if err := d.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}

	// ListSelfNodes QueryContext err 分支（L100）
	if _, err := d.ListSelfNodes(ctx); err == nil {
		t.Error("ListSelfNodes(closed) 期望失败")
	}
	// AddSelfNode ExecContext err 分支（L132）
	if _, err := d.AddSelfNode(ctx, "a", "b", true, 0); err == nil {
		t.Error("AddSelfNode(closed) 期望失败")
	}
	// UpdateSelfNode ExecContext err 分支（L144）
	if err := d.UpdateSelfNode(ctx, 1, "a", "b", true, 0); err == nil {
		t.Error("UpdateSelfNode(closed) 期望失败")
	}
	// ListSubTokens QueryContext err 分支（L188）
	if _, err := d.ListSubTokens(ctx); err == nil {
		t.Error("ListSubTokens(closed) 期望失败")
	}
	// UpdateProxyGroup ExecContext err 分支（L220）
	if err := d.UpdateProxyGroup(ctx, 1, "a", "b", "c", "d", 1, 0); err == nil {
		t.Error("UpdateProxyGroup(closed) 期望失败")
	}
	// UpdateRule ExecContext err 分支（L250）
	if err := d.UpdateRule(ctx, 1, "a", "b", 1, true, true); err == nil {
		t.Error("UpdateRule(closed) 期望失败")
	}
}

// --- 坏扫描分支：覆盖 rows.Scan err 分支（L107, L195）---
// 往表里插不符合类型的数据（NULL bool / NULL time），让 Scan 失败。

// TestScanBadSelfNode 覆盖 ListSelfNodes 的 rows.Scan err 分支（L107）。
// self_nodes.enabled 列可存 NULL，但 Scan 进 bool 失败。
func TestScanBadSelfNode(t *testing.T) {
	d, ctx := newTestDB(t)
	// 插入 enabled=NULL 的行；ListSelfNodes Scan bool 失败。
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO self_nodes (link, name, enabled, sort_order) VALUES (?, ?, NULL, 0)",
		"x", "y"); err != nil {
		t.Fatalf("INSERT 坏数据失败: %v", err)
	}
	if _, err := d.ListSelfNodes(ctx); err == nil {
		t.Error("ListSelfNodes(enabled=NULL) 期望 Scan 失败，实际成功")
	}
}

// TestScanBadSubToken 覆盖 ListSubTokens 的 rows.Scan err 分支（L195）。
// sub_tokens.created_at 存非法时间字符串，Scan 进 time.Time 失败。
func TestScanBadSubToken(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, created_at) VALUES (?, ?, ?)",
		"tk", "n", "not-a-time"); err != nil {
		t.Fatalf("INSERT 坏时间失败: %v", err)
	}
	if _, err := d.ListSubTokens(ctx); err == nil {
		t.Error("ListSubTokens(created_at=非时间) 期望 Scan 失败，实际成功")
	}
}

// TestUpdateCancelledCtx 覆盖 Update* 函数的 ctx 取消错误分支。
// 先插入一行，再用已取消的 ctx 调 Update，触发 ExecContext 的 err 分支。
func TestUpdateCancelledCtx(t *testing.T) {
	d, ctx := newTestDB(t)
	sn, err := d.AddSelfNode(ctx, "x", "y", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode 失败: %v", err)
	}
	// 先造 proxy_groups / rules 的行（Update 的 WHERE 命中才走到 RowsAffected）
	pgRes, err := d.Raw().ExecContext(ctx,
		"INSERT INTO proxy_groups (name, type, regex, url, interval, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		"g", "select", "re", "http://x", 1, 1)
	if err != nil {
		t.Fatalf("INSERT proxy_groups 失败: %v", err)
	}
	pgID, _ := pgRes.LastInsertId()
	rlRes, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		"DOMAIN", "x", 1, 1, 1, 1)
	if err != nil {
		t.Fatalf("INSERT rules 失败: %v", err)
	}
	rlID, _ := rlRes.LastInsertId()

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := d.UpdateSelfNode(cancelled, sn.ID, "a", "b", false, 0); err == nil {
		t.Error("UpdateSelfNode(cancelled ctx) 期望失败")
	}
	if err := d.UpdateProxyGroup(cancelled, pgID, "a", "b", "c", "d", 1, 0); err == nil {
		t.Error("UpdateProxyGroup(cancelled ctx) 期望失败")
	}
	if err := d.UpdateRule(cancelled, rlID, "a", "b", 1, true, true); err == nil {
		t.Error("UpdateRule(cancelled ctx) 期望失败")
	}
}

// TestAirportsHasTrafficColumns 覆盖 airports 表的 7 个流量元数据列。
func TestAirportsHasTrafficColumns(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	cols := airportColumns(t, d)
	for _, want := range []string{"upload", "download", "total", "expire", "node_count", "traffic_at", "traffic_err"} {
		if !cols[want] {
			t.Errorf("airports 缺少列 %q，实际列：%v", want, cols)
		}
	}
}

// airportColumns 返回 airports 表的列名集合。
func airportColumns(t *testing.T, d *DB) map[string]bool {
	t.Helper()
	rows, err := d.Raw().Query("PRAGMA table_info(airports)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

// TestMigrateAddsTrafficColumnsToLegacyDB 覆盖老库迁移回归。
func TestMigrateAddsTrafficColumnsToLegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// 1) 造老库：只有旧版 airports 表，没有任何流量列
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE airports (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		name             TEXT NOT NULL,
		subscription_url TEXT NOT NULL,
		remarks          TEXT,
		enabled          INTEGER DEFAULT 1,
		sort_order       INTEGER DEFAULT 0,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP
	);`); err != nil {
		t.Fatalf("建老库: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO airports (name, subscription_url) VALUES ('旧机场', 'https://old.example/sub')"); err != nil {
		t.Fatalf("插老数据: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关老库: %v", err)
	}

	// 2) 用 store.Open 打开 → migrate 应自动补列，且老数据保留
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open(老库): %v", err)
	}
	cols := airportColumns(t, d)
	for _, want := range []string{"upload", "download", "total", "expire", "node_count", "traffic_at", "traffic_err"} {
		if !cols[want] {
			t.Errorf("老库迁移后缺少列 %q", want)
		}
	}
	var name string
	var upload int64
	if err := d.Raw().QueryRow("SELECT name, upload FROM airports WHERE id = 1").Scan(&name, &upload); err != nil {
		t.Fatalf("读老行: %v", err)
	}
	if name != "旧机场" {
		t.Errorf("老数据被破坏：name = %q", name)
	}
	if upload != 0 {
		t.Errorf("新列默认值应为 0，实际 %d", upload)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 3) 幂等：再 Open 一次不许报 duplicate column
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 应幂等，实际: %v", err)
	}
	defer d2.Close()
}

// TestFetchAuditRoundTrip 覆盖 fetch_audit 往返 + 排序。
func TestFetchAuditRoundTrip(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	base := time.Unix(1700000000, 0).UTC()
	recs := []FetchAudit{
		{AirportID: 1, URL: "https://a.example/sub", StatusCode: 200, Bytes: 111, OK: true, DurationMS: 12, CreatedAt: base},
		{AirportID: 2, URL: "https://b.example/sub", StatusCode: 500, Bytes: 0, OK: false, Error: "上游 500", DurationMS: 34, CreatedAt: base.Add(time.Second)},
		{AirportID: 3, URL: "https://c.example/sub", StatusCode: 200, Bytes: 333, OK: true, DurationMS: 56, CreatedAt: base.Add(2 * time.Second)},
	}
	for _, r := range recs {
		if err := d.AddFetchAudit(ctx, r); err != nil {
			t.Fatalf("AddFetchAudit: %v", err)
		}
	}

	got, err := d.ListFetchAudit(ctx, 0)
	if err != nil {
		t.Fatalf("ListFetchAudit: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("期望 3 条，实际 %d", len(got))
	}
	// created_at 倒序：最新的 c 在前
	if got[0].URL != "https://c.example/sub" || got[1].URL != "https://b.example/sub" || got[2].URL != "https://a.example/sub" {
		t.Fatalf("排序不对：%v", []string{got[0].URL, got[1].URL, got[2].URL})
	}
	// 逐字段往返
	b := got[1]
	if b.AirportID != 2 || b.StatusCode != 500 || b.Bytes != 0 || b.OK || b.Error != "上游 500" || b.DurationMS != 34 {
		t.Errorf("字段往返不一致：%+v", b)
	}
	if !b.CreatedAt.Equal(base.Add(time.Second)) {
		t.Errorf("CreatedAt 往返不一致：%v", b.CreatedAt)
	}
	if b.ID == 0 {
		t.Error("ID 应被回填")
	}
}

// TestListFetchAuditLimit 覆盖 ListFetchAudit 的 limit 语义。
func TestListFetchAuditLimit(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	base := time.Unix(1700000000, 0).UTC()
	for i := 0; i < 5; i++ {
		if err := d.AddFetchAudit(ctx, FetchAudit{AirportID: int64(i + 1), URL: "https://x.example", OK: true, CreatedAt: base.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatalf("AddFetchAudit: %v", err)
		}
	}
	// limit <= 0 视为 100，5 条都要回来
	all, err := d.ListFetchAudit(ctx, -1)
	if err != nil {
		t.Fatalf("ListFetchAudit(-1): %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("limit<=0 期望 5 条，实际 %d", len(all))
	}
	// limit=2 → 最新 2 条（AirportID 5、4）
	top, err := d.ListFetchAudit(ctx, 2)
	if err != nil {
		t.Fatalf("ListFetchAudit(2): %v", err)
	}
	if len(top) != 2 || top[0].AirportID != 5 || top[1].AirportID != 4 {
		t.Fatalf("limit=2 结果不对：%+v", top)
	}
}

// TestPruneFetchAudit 覆盖 PruneFetchAudit 保留最近 N 条。
func TestPruneFetchAudit(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	base := time.Unix(1700000000, 0).UTC()
	for i := 0; i < 5; i++ {
		if err := d.AddFetchAudit(ctx, FetchAudit{AirportID: int64(i + 1), URL: "https://x.example", OK: true, CreatedAt: base.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatalf("AddFetchAudit: %v", err)
		}
	}
	// keep<=0 不动数据
	n, err := d.PruneFetchAudit(ctx, 0)
	if err != nil {
		t.Fatalf("PruneFetchAudit(0): %v", err)
	}
	if n != 0 {
		t.Errorf("keep<=0 不应删除，实际删了 %d", n)
	}
	// keep=2 → 删 3 条，剩最新 2 条
	n, err = d.PruneFetchAudit(ctx, 2)
	if err != nil {
		t.Fatalf("PruneFetchAudit(2): %v", err)
	}
	if n != 3 {
		t.Errorf("期望删 3 条，实际 %d", n)
	}
	left, err := d.ListFetchAudit(ctx, 0)
	if err != nil {
		t.Fatalf("ListFetchAudit: %v", err)
	}
	if len(left) != 2 || left[0].AirportID != 5 || left[1].AirportID != 4 {
		t.Fatalf("保留结果不对：%+v", left)
	}
}

// TestAddFetchAuditZeroCreatedAt 覆盖 CreatedAt 为零值时用当前时间。
func TestAddFetchAuditZeroCreatedAt(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	before := time.Now().Add(-time.Minute)
	if err := d.AddFetchAudit(ctx, FetchAudit{AirportID: 7, URL: "https://z.example", OK: true}); err != nil {
		t.Fatalf("AddFetchAudit: %v", err)
	}
	got, err := d.ListFetchAudit(ctx, 1)
	if err != nil {
		t.Fatalf("ListFetchAudit: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 条，实际 %d", len(got))
	}
	if got[0].CreatedAt.IsZero() || got[0].CreatedAt.Before(before) {
		t.Errorf("零值 CreatedAt 应被填成当前时间，实际 %v", got[0].CreatedAt)
	}
}

// --- DB-1：scripts / pipelines / rules 溯源列 / rule_providers 索引 ---

// tableColumns 返回指定表的列名集合。
func tableColumns(t *testing.T, d *DB, table string) map[string]bool {
	t.Helper()
	rows, err := d.Raw().Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s): %v", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan %s: %v", table, err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows %s: %v", table, err)
	}
	return out
}

// indexExists 检查 sqlite_master 里是否存在指定名字的对象。
func indexExists(t *testing.T, d *DB, name string) bool {
	t.Helper()
	var n int
	err := d.Raw().QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name = ?", name).Scan(&n)
	if err != nil {
		t.Fatalf("查询索引 %s: %v", name, err)
	}
	return n > 0
}

// TestNewDBHasSchemaColumns 覆盖新库 scripts/pipelines/rules/rule_providers 列与索引齐全。
func TestNewDBHasSchemaColumns(t *testing.T) {
	d, ctx := newTestDB(t)

	scriptCols := tableColumns(t, d, "scripts")
	for _, want := range []string{"id", "name", "description", "kind", "code", "enabled", "created_at", "updated_at"} {
		if !scriptCols[want] {
			t.Errorf("scripts 缺少列 %q，实际：%v", want, scriptCols)
		}
	}
	pipeCols := tableColumns(t, d, "pipelines")
	for _, want := range []string{"id", "name", "description", "steps_json", "enabled", "sort_order", "source", "created_at", "updated_at"} {
		if !pipeCols[want] {
			t.Errorf("pipelines 缺少列 %q，实际：%v", want, pipeCols)
		}
	}
	ruleCols := tableColumns(t, d, "rules")
	for _, want := range []string{"source", "raw", "origin_key"} {
		if !ruleCols[want] {
			t.Errorf("rules 缺少列 %q，实际：%v", want, ruleCols)
		}
	}
	providerCols := tableColumns(t, d, "rule_providers")
	for _, want := range []string{"name", "raw_json", "source", "enabled", "sort_order"} {
		if !providerCols[want] {
			t.Errorf("rule_providers 缺少列 %q，实际：%v", want, providerCols)
		}
	}
	if !indexExists(t, d, "idx_rules_origin") {
		t.Error("新库缺少索引 idx_rules_origin")
	}
	if !indexExists(t, d, "idx_rule_providers_source_name") {
		t.Error("新库缺少索引 idx_rule_providers_source_name")
	}
	for _, tbl := range []string{"scripts", "pipelines"} {
		var n int
		err := d.Raw().QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name = ?", tbl).Scan(&n)
		if err != nil {
			t.Fatalf("查询 %s 索引: %v", tbl, err)
		}
		if n == 0 {
			t.Errorf("%s 应至少有 1 个索引（name 唯一约束）", tbl)
		}
	}
}

// TestScriptsCRUD 覆盖 scripts Create/Get/GetByName/Update/Delete 往返与 sql.ErrNoRows。
func TestScriptsCRUD(t *testing.T) {
	d, ctx := newTestDB(t)

	s, err := d.CreateScript(ctx, Script{Name: "s1", Description: "desc", Code: "return 1", Enabled: true})
	if err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	if s.ID <= 0 || s.Name != "s1" || s.Description != "desc" || s.Code != "return 1" || !s.Enabled {
		t.Fatalf("CreateScript 返回不符: %+v", s)
	}
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		t.Error("CreateScript 应写入 created_at/updated_at")
	}

	g, err := d.GetScript(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if g != *s {
		t.Errorf("GetScript 往返不一致：%+v vs %+v", g, *s)
	}
	gb, err := d.GetScriptByName(ctx, "s1")
	if err != nil {
		t.Fatalf("GetScriptByName: %v", err)
	}
	if gb != *s {
		t.Errorf("GetScriptByName 往返不一致：%+v vs %+v", gb, *s)
	}

	s.Name = "s1b"
	s.Code = "return 2"
	s.Enabled = false
	if err := d.UpdateScript(ctx, *s); err != nil {
		t.Fatalf("UpdateScript: %v", err)
	}
	g2, err := d.GetScript(ctx, s.ID)
	if err != nil {
		t.Fatalf("Update 后 GetScript: %v", err)
	}
	if g2.Name != "s1b" || g2.Code != "return 2" || g2.Enabled {
		t.Errorf("UpdateScript 未生效: %+v", g2)
	}

	if err := d.DeleteScript(ctx, s.ID); err != nil {
		t.Fatalf("DeleteScript: %v", err)
	}
	if _, err := d.GetScript(ctx, s.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("删除后 GetScript 错误=%v，期望 sql.ErrNoRows", err)
	}
	if _, err := d.GetScriptByName(ctx, "s1b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("删除后 GetScriptByName 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdateScript(ctx, *s); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateScript(已删除) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.DeleteScript(ctx, s.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeleteScript(已删除) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if _, err := d.GetScript(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetScript(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if _, err := d.GetScriptByName(ctx, "nope"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetScriptByName(nope) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestScriptsListOrder 覆盖 ListScripts 排序与空库返回非 nil 空切片。
func TestScriptsListOrder(t *testing.T) {
	d, ctx := newTestDB(t)

	empty, err := d.ListScripts(ctx)
	if err != nil {
		t.Fatalf("空库 ListScripts: %v", err)
	}
	if len(empty) != 0 || empty == nil {
		t.Fatalf("空库 ListScripts 应为非 nil 空切片，实际 %v", empty)
	}

	if _, err := d.CreateScript(ctx, Script{Name: "beta", Code: "b", Enabled: true}); err != nil {
		t.Fatalf("CreateScript(beta): %v", err)
	}
	if _, err := d.CreateScript(ctx, Script{Name: "alpha", Code: "a", Enabled: true}); err != nil {
		t.Fatalf("CreateScript(alpha): %v", err)
	}
	list, err := d.ListScripts(ctx)
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListScripts 长度 %d，期望 2", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Errorf("ListScripts 排序错误: %s, %s", list[0].Name, list[1].Name)
	}
}

// TestScriptCreateExplicitTimestamps 覆盖 CreateScript 非零时间分支。
func TestScriptCreateExplicitTimestamps(t *testing.T) {
	d, ctx := newTestDB(t)
	t0 := time.Unix(1700000000, 0).UTC()
	s, err := d.CreateScript(ctx, Script{Name: "ts", Code: "c", CreatedAt: t0, UpdatedAt: t0})
	if err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	if !s.CreatedAt.Equal(t0) || !s.UpdatedAt.Equal(t0) {
		t.Errorf("显式时间未保留：created=%v updated=%v，期望 %v", s.CreatedAt, s.UpdatedAt, t0)
	}
	t1 := time.Unix(1700000001, 0).UTC()
	s.UpdatedAt = t1
	if err := d.UpdateScript(ctx, *s); err != nil {
		t.Fatalf("UpdateScript: %v", err)
	}
	g, err := d.GetScript(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if !g.UpdatedAt.Equal(t1) {
		t.Errorf("UpdateScript 未用显式 UpdatedAt：%v", g.UpdatedAt)
	}
}

// TestScriptsKindRoundTrip 覆盖 script.kind 存取往返（D-01：kind 全链路丢失）。
func TestScriptsKindRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)

	s, err := d.CreateScript(ctx, Script{Name: "oper", Kind: "operate", Code: "node.port = 443", Enabled: true})
	if err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	if s.Kind != "operate" {
		t.Fatalf("CreateScript 返回 kind = %q，期望 operate", s.Kind)
	}
	g, err := d.GetScript(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetScript: %v", err)
	}
	if g.Kind != "operate" {
		t.Errorf("GetScript kind = %q，期望 operate", g.Kind)
	}
	gb, err := d.GetScriptByName(ctx, "oper")
	if err != nil {
		t.Fatalf("GetScriptByName: %v", err)
	}
	if gb.Kind != "operate" {
		t.Errorf("GetScriptByName kind = %q，期望 operate", gb.Kind)
	}

	// 未指定 kind：store 原样存空串，默认值属于 API 层（POST 默认 filter）。
	plain, err := d.CreateScript(ctx, Script{Name: "plain", Code: "return node"})
	if err != nil {
		t.Fatalf("CreateScript(plain): %v", err)
	}
	if plain.Kind != "" {
		t.Errorf("未指定 kind 时应为空串，实际 %q", plain.Kind)
	}

	// UpdateScript 改成 filter → 回来是 filter，列表同步。
	g.Kind = "filter"
	g.Code = "node.type === 'ss'"
	if err := d.UpdateScript(ctx, g); err != nil {
		t.Fatalf("UpdateScript: %v", err)
	}
	g2, err := d.GetScript(ctx, s.ID)
	if err != nil {
		t.Fatalf("Update 后 GetScript: %v", err)
	}
	if g2.Kind != "filter" || g2.Code != "node.type === 'ss'" {
		t.Errorf("UpdateScript 未生效: kind=%q code=%q", g2.Kind, g2.Code)
	}
	list, err := d.ListScripts(ctx)
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	found := false
	for _, it := range list {
		if it.ID == s.ID {
			found = true
			if it.Kind != "filter" {
				t.Errorf("ListScripts kind = %q，期望 filter", it.Kind)
			}
		}
	}
	if !found {
		t.Fatalf("ListScripts 未包含 id=%d", s.ID)
	}
}

// TestMigrateAddsScriptKindToLegacyDB 覆盖老库 scripts 表无 kind 列的迁移回归（D-01）。
func TestMigrateAddsScriptKindToLegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-scripts.db")

	// 1) 造老库：旧版 scripts 表（无 kind 列）+ 一条老脚本
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE scripts (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		code        TEXT NOT NULL,
		enabled     INTEGER NOT NULL DEFAULT 1,
		created_at  INTEGER NOT NULL DEFAULT 0,
		updated_at  INTEGER NOT NULL DEFAULT 0
	);`); err != nil {
		t.Fatalf("建老库: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO scripts (name, description, code, enabled) VALUES ('老脚本', '旧', 'return 1', 1)"); err != nil {
		t.Fatalf("插老数据: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关老库: %v", err)
	}

	// 2) store.Open 应自动补 kind 列；老记录 kind 为空串且原字段不被改写
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open(老库): %v", err)
	}
	cols := tableColumns(t, d, "scripts")
	if !cols["kind"] {
		t.Errorf("老库迁移后缺少列 kind，实际：%v", cols)
	}
	ctx := context.Background()
	old, err := d.GetScriptByName(ctx, "老脚本")
	if err != nil {
		t.Fatalf("GetScriptByName(老脚本): %v", err)
	}
	if old.Kind != "" {
		t.Errorf("老记录 kind 应为空串，实际 %q", old.Kind)
	}
	if old.Code != "return 1" {
		t.Errorf("老记录 code 被破坏：%q", old.Code)
	}
	created, err := d.CreateScript(ctx, Script{Name: "新脚本", Kind: "operate", Code: "node.port = 443", Enabled: true})
	if err != nil {
		t.Fatalf("迁移后 CreateScript: %v", err)
	}
	if created.Kind != "operate" {
		t.Errorf("迁移后 CreateScript kind = %q，期望 operate", created.Kind)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 3) 幂等：再 Open 一次不许报 duplicate column
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 应幂等，实际: %v", err)
	}
	defer d2.Close()
}

// TestPipelinesCRUD 覆盖 pipelines Create/Get/GetByName/Update/Delete 往返与排序。
func TestPipelinesCRUD(t *testing.T) {
	d, ctx := newTestDB(t)

	p0, err := d.CreatePipeline(ctx, Pipeline{Name: "p-default", Description: "", Enabled: true, SortOrder: 9})
	if err != nil {
		t.Fatalf("CreatePipeline(p-default): %v", err)
	}
	if p0.StepsJSON != "[]" || p0.Source != "ui" || p0.Enabled != true {
		t.Errorf("CreatePipeline 默认值不符: %+v", p0)
	}

	p1, err := d.CreatePipeline(ctx, Pipeline{Name: "p1", Description: "d1", StepsJSON: "[]", Enabled: true, SortOrder: 1, Source: "ui"})
	if err != nil {
		t.Fatalf("CreatePipeline(p1): %v", err)
	}
	g, err := d.GetPipeline(ctx, p1.ID)
	if err != nil {
		t.Fatalf("GetPipeline: %v", err)
	}
	if g.Name != "p1" || g.StepsJSON != p1.StepsJSON || g.Source != "ui" || !g.Enabled {
		t.Errorf("GetPipeline 往返不一致: %+v", g)
	}
	gb, err := d.GetPipelineByName(ctx, "p1")
	if err != nil {
		t.Fatalf("GetPipelineByName: %v", err)
	}
	if gb.Name != "p1" {
		t.Errorf("GetPipelineByName 不符: %+v", gb)
	}

	p1.StepsJSON = "[]"
	p1.Source = "template"
	p1.SortOrder = 5
	if err := d.UpdatePipeline(ctx, *p1); err != nil {
		t.Fatalf("UpdatePipeline: %v", err)
	}
	g2, err := d.GetPipeline(ctx, p1.ID)
	if err != nil {
		t.Fatalf("Update 后 GetPipeline: %v", err)
	}
	if g2.Source != "template" || g2.SortOrder != 5 {
		t.Errorf("UpdatePipeline 未生效: %+v", g2)
	}

	if err := d.DeletePipeline(ctx, p1.ID); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}
	if _, err := d.GetPipeline(ctx, p1.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("删除后 GetPipeline 错误=%v，期望 sql.ErrNoRows", err)
	}
	if _, err := d.GetPipelineByName(ctx, "p1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("删除后 GetPipelineByName 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdatePipeline(ctx, *p1); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdatePipeline(已删除) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.DeletePipeline(ctx, p1.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeletePipeline(已删除) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if _, err := d.GetPipeline(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetPipeline(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if _, err := d.GetPipelineByName(ctx, "nope"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetPipelineByName(nope) 错误=%v，期望 sql.ErrNoRows", err)
	}

	if _, err := d.CreatePipeline(ctx, Pipeline{Name: "b", StepsJSON: "[]", SortOrder: 2}); err != nil {
		t.Fatalf("CreatePipeline(b): %v", err)
	}
	if _, err := d.CreatePipeline(ctx, Pipeline{Name: "a", StepsJSON: "[]", SortOrder: 1}); err != nil {
		t.Fatalf("CreatePipeline(a): %v", err)
	}
	if _, err := d.CreatePipeline(ctx, Pipeline{Name: "c", StepsJSON: "[]", SortOrder: 1}); err != nil {
		t.Fatalf("CreatePipeline(c): %v", err)
	}
	list, err := d.ListPipelines(ctx)
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("ListPipelines 长度 %d，期望 4", len(list))
	}
	if list[0].Name != "a" || list[1].Name != "c" || list[2].Name != "b" || list[3].Name != "p-default" {
		t.Errorf("ListPipelines 排序错误: %s,%s,%s,%s", list[0].Name, list[1].Name, list[2].Name, list[3].Name)
	}
}

// TestRuleProvidersUpsert 覆盖 UpsertRuleProvider 幂等写入与 ListRuleProviders 排序。
func TestRuleProvidersUpsert(t *testing.T) {
	d, ctx := newTestDB(t)

	empty, err := d.ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("空库 ListRuleProviders: %v", err)
	}
	if len(empty) != 0 || empty == nil {
		t.Fatalf("空库 ListRuleProviders 应为非 nil 空切片")
	}

	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "rp1", RawJSON: "v1", Source: "airport:HK", Enabled: true, SortOrder: 1}); err != nil {
		t.Fatalf("UpsertRuleProvider(insert): %v", err)
	}
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "rp1", RawJSON: "v2", Source: "airport:HK", Enabled: false, SortOrder: 1}); err != nil {
		t.Fatalf("UpsertRuleProvider(update): %v", err)
	}
	list, err := d.ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("ListRuleProviders: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("同名 upsert 应只有一行，实际 %d", len(list))
	}
	if list[0].RawJSON != "v2" || list[0].Enabled {
		t.Errorf("Upsert 未更新 RawJSON/Enabled: %+v", list[0])
	}
	if list[0].Name != "rp1" || list[0].Source != "airport:HK" {
		t.Errorf("Upsert 字段不符: %+v", list[0])
	}
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "b", RawJSON: "{}", Source: "rulebot", SortOrder: 0}); err != nil {
		t.Fatalf("UpsertRuleProvider(b): %v", err)
	}
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "a", RawJSON: "{}", Source: "rulebot", SortOrder: 0}); err != nil {
		t.Fatalf("UpsertRuleProvider(a): %v", err)
	}
	list2, err := d.ListRuleProviders(ctx)
	if err != nil {
		t.Fatalf("ListRuleProviders(排序): %v", err)
	}
	if len(list2) != 3 {
		t.Fatalf("ListRuleProviders 长度 %d，期望 3", len(list2))
	}
	if list2[0].Name != "a" || list2[1].Name != "b" || list2[2].Name != "rp1" {
		t.Errorf("ListRuleProviders 排序错误: %s,%s,%s", list2[0].Name, list2[1].Name, list2[2].Name)
	}
}

// TestRuleProvenanceRoundTrip 覆盖 rules source/raw/origin_key 往返（UpdateRule 不清空溯源列）。
func TestRuleProvenanceRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)

	res, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, source, raw, origin_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"DOMAIN-SUFFIX", "a.com", 1, 1, 1, 1, "airport:HK", "DOMAIN-SUFFIX, a.com", "a.com")
	if err != nil {
		t.Fatalf("INSERT rules(溯源): %v", err)
	}
	id, _ := res.LastInsertId()

	r, err := d.GetRule(ctx, id)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if r.Source != "airport:HK" || r.Raw != "DOMAIN-SUFFIX, a.com" || r.OriginKey != "a.com" {
		t.Errorf("rules 溯源字段往返不符: source=%q raw=%q origin=%q", r.Source, r.Raw, r.OriginKey)
	}

	if err := d.UpdateRule(ctx, id, "IP-CIDR", "1.2.3.0/24", 2, false, false); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	r2, err := d.GetRule(ctx, id)
	if err != nil {
		t.Fatalf("Update 后 GetRule: %v", err)
	}
	if r2.RuleType != "IP-CIDR" || r2.RuleParam != "1.2.3.0/24" || r2.RuleConfig != 2 || r2.ResolveDNS || r2.Enabled {
		t.Errorf("UpdateRule 未生效: %+v", r2)
	}
	if r2.Source != "airport:HK" || r2.Raw != "DOMAIN-SUFFIX, a.com" || r2.OriginKey != "a.com" {
		t.Errorf("UpdateRule 不应清空溯源列: source=%q raw=%q origin=%q", r2.Source, r2.Raw, r2.OriginKey)
	}
}

// TestLegacyRulesAndProvidersMigration 覆盖老库（旧 schema）迁移：补列、回填、唯一索引、老数据保留、二次 Open 幂等。
func TestLegacyRulesAndProvidersMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	oldRules := "CREATE TABLE rules (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_type TEXT NOT NULL, rule_param TEXT, rule_config INTEGER NOT NULL, resolve_dns INTEGER DEFAULT 1, enabled INTEGER DEFAULT 1, sort_order INTEGER DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);"
	if _, err := raw.Exec(oldRules); err != nil {
		t.Fatalf("建老库 rules: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO rules (rule_type, rule_param, rule_config) VALUES ('DOMAIN', 'x', 1), ('DOMAIN', 'y', 1)"); err != nil {
		t.Fatalf("插老 rules 失败: %v", err)
	}
	oldProviders := "CREATE TABLE rule_providers (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, raw_json TEXT NOT NULL, source TEXT, enabled INTEGER DEFAULT 1, sort_order INTEGER DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);"
	if _, err := raw.Exec(oldProviders); err != nil {
		t.Fatalf("建老库 rule_providers: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO rule_providers (name, raw_json, source, enabled) VALUES ('dup', '{}', '', 1), ('dup', '{}', '', 1), ('other', '{}', '', 1)"); err != nil {
		t.Fatalf("插老 rule_providers 失败: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关老库: %v", err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open(老库): %v", err)
	}
	ruleCols := tableColumns(t, d, "rules")
	for _, want := range []string{"source", "raw", "origin_key"} {
		if !ruleCols[want] {
			t.Errorf("老库迁移后 rules 缺少列 %q", want)
		}
	}
	providerCols := tableColumns(t, d, "rule_providers")
	for _, want := range []string{"source", "name", "raw_json"} {
		if !providerCols[want] {
			t.Errorf("老库迁移后 rule_providers 缺少列 %q", want)
		}
	}
	if !indexExists(t, d, "idx_rules_origin") {
		t.Error("老库迁移后缺少索引 idx_rules_origin（多行 ('','') 不应导致建索引失败）")
	}
	if !indexExists(t, d, "idx_rule_providers_source_name") {
		t.Error("老库迁移后缺少索引 idx_rule_providers_source_name")
	}

	rows, err := d.Raw().QueryContext(context.Background(),
		"SELECT id, origin_key FROM rules ORDER BY id")
	if err != nil {
		t.Fatalf("读老 rules: %v", err)
	}
	defer rows.Close()
	backfilled := map[int64]string{}
	for rows.Next() {
		var id int64
		var ok string
		if err := rows.Scan(&id, &ok); err != nil {
			t.Fatalf("scan rules: %v", err)
		}
		backfilled[id] = ok
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if len(backfilled) != 2 {
		t.Fatalf("老 rules 应保留 2 行，实际 %d", len(backfilled))
	}
	for id, ok := range backfilled {
		if ok != "" {
			t.Errorf("rules.id=%d origin_key=%q，老行应保持空（不再回填 legacy:）", id, ok)
		}
	}
	// 部分索引：只约束 origin_key <> '' 的导入行。
	var idxSQL string
	if err := d.Raw().QueryRow(
		"SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_rules_origin'").Scan(&idxSQL); err != nil {
		t.Fatalf("读 idx_rules_origin 定义: %v", err)
	}
	if !strings.Contains(idxSQL, "WHERE origin_key <> ''") {
		t.Errorf("idx_rules_origin 应为部分索引（含 WHERE origin_key <> ''），实际: %s", idxSQL)
	}

	prov, err := d.ListRuleProviders(context.Background())
	if err != nil {
		t.Fatalf("ListRuleProviders(老库): %v", err)
	}
	if len(prov) != 2 {
		t.Fatalf("老库重复行去重后应剩 2 行，实际 %d", len(prov))
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	d2, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 应幂等，实际: %v", err)
	}
	defer d2.Close()
	if !indexExists(t, d2, "idx_rules_origin") {
		t.Error("二次 Open 后 idx_rules_origin 丢失")
	}
}

// TestNewMethodsClosedDB 覆盖 scripts/pipelines/rule_providers 各方法在关闭的 DB 上的错误分支。
func TestNewMethodsClosedDB(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := d.ListScripts(ctx); err == nil {
		t.Error("ListScripts(closed) 期望失败")
	}
	if _, err := d.GetScript(ctx, 1); err == nil {
		t.Error("GetScript(closed) 期望失败")
	}
	if _, err := d.GetScriptByName(ctx, "x"); err == nil {
		t.Error("GetScriptByName(closed) 期望失败")
	}
	if _, err := d.CreateScript(ctx, Script{Name: "x", Code: "c"}); err == nil {
		t.Error("CreateScript(closed) 期望失败")
	}
	if err := d.UpdateScript(ctx, Script{ID: 1, Name: "x"}); err == nil {
		t.Error("UpdateScript(closed) 期望失败")
	}
	if err := d.DeleteScript(ctx, 1); err == nil {
		t.Error("DeleteScript(closed) 期望失败")
	}
	if _, err := d.ListPipelines(ctx); err == nil {
		t.Error("ListPipelines(closed) 期望失败")
	}
	if _, err := d.GetPipeline(ctx, 1); err == nil {
		t.Error("GetPipeline(closed) 期望失败")
	}
	if _, err := d.GetPipelineByName(ctx, "x"); err == nil {
		t.Error("GetPipelineByName(closed) 期望失败")
	}
	if _, err := d.CreatePipeline(ctx, Pipeline{Name: "x"}); err == nil {
		t.Error("CreatePipeline(closed) 期望失败")
	}
	if err := d.UpdatePipeline(ctx, Pipeline{ID: 1, Name: "x"}); err == nil {
		t.Error("UpdatePipeline(closed) 期望失败")
	}
	if err := d.DeletePipeline(ctx, 1); err == nil {
		t.Error("DeletePipeline(closed) 期望失败")
	}
	if _, err := d.ListRuleProviders(ctx); err == nil {
		t.Error("ListRuleProviders(closed) 期望失败")
	}
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "x", RawJSON: "{}"}); err == nil {
		t.Error("UpsertRuleProvider(closed) 期望失败")
	}
}

// TestNewMethodsCancelledCtx 覆盖 scripts/pipelines/rule_providers 各方法的 ctx 取消错误分支。
func TestNewMethodsCancelledCtx(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.CreateScript(ctx, Script{Name: "seed", Code: "c"}); err != nil {
		t.Fatalf("CreateScript(seed): %v", err)
	}
	if _, err := d.CreatePipeline(ctx, Pipeline{Name: "seed"}); err != nil {
		t.Fatalf("CreatePipeline(seed): %v", err)
	}
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "seed", RawJSON: "{}"}); err != nil {
		t.Fatalf("UpsertRuleProvider(seed): %v", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := d.ListScripts(cancelled); err == nil {
		t.Error("ListScripts(cancelled) 期望失败")
	}
	if _, err := d.GetScript(cancelled, 1); err == nil {
		t.Error("GetScript(cancelled) 期望失败")
	}
	if _, err := d.GetScriptByName(cancelled, "seed"); err == nil {
		t.Error("GetScriptByName(cancelled) 期望失败")
	}
	if _, err := d.CreateScript(cancelled, Script{Name: "x", Code: "c"}); err == nil {
		t.Error("CreateScript(cancelled) 期望失败")
	}
	if err := d.UpdateScript(cancelled, Script{ID: 1, Name: "x"}); err == nil {
		t.Error("UpdateScript(cancelled) 期望失败")
	}
	if err := d.DeleteScript(cancelled, 1); err == nil {
		t.Error("DeleteScript(cancelled) 期望失败")
	}
	if _, err := d.ListPipelines(cancelled); err == nil {
		t.Error("ListPipelines(cancelled) 期望失败")
	}
	if _, err := d.GetPipeline(cancelled, 1); err == nil {
		t.Error("GetPipeline(cancelled) 期望失败")
	}
	if _, err := d.GetPipelineByName(cancelled, "seed"); err == nil {
		t.Error("GetPipelineByName(cancelled) 期望失败")
	}
	if _, err := d.CreatePipeline(cancelled, Pipeline{Name: "x"}); err == nil {
		t.Error("CreatePipeline(cancelled) 期望失败")
	}
	if err := d.UpdatePipeline(cancelled, Pipeline{ID: 1, Name: "x"}); err == nil {
		t.Error("UpdatePipeline(cancelled) 期望失败")
	}
	if err := d.DeletePipeline(cancelled, 1); err == nil {
		t.Error("DeletePipeline(cancelled) 期望失败")
	}
	if _, err := d.ListRuleProviders(cancelled); err == nil {
		t.Error("ListRuleProviders(cancelled) 期望失败")
	}
	if err := d.UpsertRuleProvider(cancelled, RuleProvider{Name: "seed", RawJSON: "{}"}); err == nil {
		t.Error("UpsertRuleProvider(cancelled) 期望失败")
	}
}

// TestManualRulesCoexist 修 DB-1b 回归：手工规则（origin_key=”）连续插入必须成功。
func TestManualRulesCoexist(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES ('DOMAIN-SUFFIX', 'a.com', 1, 1, 1, 1)"); err != nil {
		t.Fatalf("插入第一行手工规则失败: %v", err)
	}
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES ('DOMAIN-SUFFIX', 'b.com', 1, 1, 1, 2)"); err != nil {
		t.Fatalf("插入第二行手工规则失败（DB-1b 回归）: %v", err)
	}
	var n int
	if err := d.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules WHERE origin_key = ''").Scan(&n); err != nil {
		t.Fatalf("查询手工规则行数: %v", err)
	}
	if n != 2 {
		t.Fatalf("手工规则应有 2 行，实际 %d", n)
	}
}

// TestImportOriginKeyUnique 覆盖部分索引：导入行（origin_key != ”）重复时被唯一约束挡住。
func TestImportOriginKeyUnique(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, source, origin_key) VALUES ('DOMAIN', 'x', 1, 'airport:HK', 'x')"); err != nil {
		t.Fatalf("插入第一条导入规则失败: %v", err)
	}
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, source, origin_key) VALUES ('DOMAIN', 'x2', 1, 'airport:HK', 'x')"); err == nil {
		t.Fatal("重复 (source, origin_key) 导入行应被唯一约束挡住，实际成功")
	}
}

// TestEmptyOriginKeyCoexist 覆盖 origin_key=” 的多行可以共存。
func TestEmptyOriginKeyCoexist(t *testing.T) {
	d, ctx := newTestDB(t)
	for i := 0; i < 3; i++ {
		if _, err := d.Raw().ExecContext(ctx,
			"INSERT INTO rules (rule_type, rule_param, rule_config, origin_key) VALUES ('DOMAIN', 'x', 1, '')"); err != nil {
			t.Fatalf("插入第 %d 行失败: %v", i+1, err)
		}
	}
	var n int
	if err := d.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules WHERE origin_key = ''").Scan(&n); err != nil {
		t.Fatalf("查询: %v", err)
	}
	if n != 3 {
		t.Fatalf("应有 3 行，实际 %d", n)
	}
}

// TestSetRuleProviderEnabled 覆盖启用切换与不存在 id 的 (false, nil) 语义。
func TestSetRuleProviderEnabled(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "rp1", RawJSON: "{}", Source: "airport:HK", Enabled: true, SortOrder: 1}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	list, err := d.ListRuleProviders(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("seed list: %v len=%d", err, len(list))
	}
	id := list[0].ID

	found, err := d.SetRuleProviderEnabled(ctx, id, false)
	if err != nil || !found {
		t.Fatalf("SetRuleProviderEnabled(existing) = (%v, %v)，应为 (true, nil)", found, err)
	}
	list, _ = d.ListRuleProviders(ctx)
	if list[0].Enabled {
		t.Error("禁用后 Enabled 应为 false")
	}

	found, err = d.SetRuleProviderEnabled(ctx, 9999, true)
	if err != nil || found {
		t.Fatalf("SetRuleProviderEnabled(missing) = (%v, %v)，应为 (false, nil)", found, err)
	}
}

// TestSortOrderPersisted 覆盖 sort_order 的落库语义：
// Add 用用户值（>0），0 则回退时间戳；Update 用用户值覆盖，0 则保持原值。
func TestSortOrderPersisted(t *testing.T) {
	d, ctx := newTestDB(t)

	// --- proxy_groups ---
	g1, err := d.AddProxyGroup(ctx, "g1", "select", "", "", 0, 7)
	if err != nil {
		t.Fatalf("AddProxyGroup(g1): %v", err)
	}
	if g1.SortOrder != 7 {
		t.Fatalf("AddProxyGroup(g1).SortOrder = %d，应为 7", g1.SortOrder)
	}
	g2, err := d.AddProxyGroup(ctx, "g2", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup(g2): %v", err)
	}
	if g2.SortOrder <= 0 {
		t.Fatalf("AddProxyGroup(g2) 未指定顺序时应回退为正时间戳，got %d", g2.SortOrder)
	}

	// Update 传 0：保持原值
	if err := d.UpdateProxyGroup(ctx, g1.ID, "g1", "select", "", "", 0, 0); err != nil {
		t.Fatalf("UpdateProxyGroup(g1, 0): %v", err)
	}
	got, err := d.GetProxyGroup(ctx, g1.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup(g1): %v", err)
	}
	if got.SortOrder != 7 {
		t.Fatalf("UpdateProxyGroup(sortOrder=0) 后 SortOrder = %d，应保持 7", got.SortOrder)
	}

	// Update 传正数：覆盖
	if err := d.UpdateProxyGroup(ctx, g1.ID, "g1", "select", "", "", 0, 3); err != nil {
		t.Fatalf("UpdateProxyGroup(g1, 3): %v", err)
	}
	got, err = d.GetProxyGroup(ctx, g1.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup(g1): %v", err)
	}
	if got.SortOrder != 3 {
		t.Fatalf("UpdateProxyGroup(sortOrder=3) 后 SortOrder = %d，应为 3", got.SortOrder)
	}

	// List 按 sort_order 升序（g1=3 在 g2 之前，因为 g2 是纳秒时间戳）
	// store 层没有 ListProxyGroup（列表在 Manager 里用 raw SQL 查），这里直接查库验证排序。
	rows, err := d.Raw().QueryContext(ctx, "SELECT id FROM proxy_groups ORDER BY sort_order, id")
	if err != nil {
		t.Fatalf("查询 proxy_groups 失败: %v", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("Scan 失败: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历失败: %v", err)
	}
	if len(ids) != 2 || ids[0] != g1.ID || ids[1] != g2.ID {
		t.Fatalf("proxy_groups 排序不对: %+v", ids)
	}

	// --- self_nodes ---
	n1, err := d.AddSelfNode(ctx, "ss://a", "n1", true, 9)
	if err != nil {
		t.Fatalf("AddSelfNode(n1): %v", err)
	}
	if n1.SortOrder != 9 {
		t.Fatalf("AddSelfNode(n1).SortOrder = %d，应为 9", n1.SortOrder)
	}
	n2, err := d.AddSelfNode(ctx, "ss://b", "n2", true, 0)
	if err != nil {
		t.Fatalf("AddSelfNode(n2): %v", err)
	}
	if n2.SortOrder <= 0 {
		t.Fatalf("AddSelfNode(n2) 未指定顺序时应回退为正时间戳，got %d", n2.SortOrder)
	}
	if err := d.UpdateSelfNode(ctx, n1.ID, "ss://a", "n1", true, 0); err != nil {
		t.Fatalf("UpdateSelfNode(n1, 0): %v", err)
	}
	gotNode, err := d.GetSelfNode(ctx, n1.ID)
	if err != nil {
		t.Fatalf("GetSelfNode(n1): %v", err)
	}
	if gotNode.SortOrder != 9 {
		t.Fatalf("UpdateSelfNode(sortOrder=0) 后 SortOrder = %d，应保持 9", gotNode.SortOrder)
	}
	if err := d.UpdateSelfNode(ctx, n1.ID, "ss://a", "n1", true, 4); err != nil {
		t.Fatalf("UpdateSelfNode(n1, 4): %v", err)
	}
	gotNode, err = d.GetSelfNode(ctx, n1.ID)
	if err != nil {
		t.Fatalf("GetSelfNode(n1): %v", err)
	}
	if gotNode.SortOrder != 4 {
		t.Fatalf("UpdateSelfNode(sortOrder=4) 后 SortOrder = %d，应为 4", gotNode.SortOrder)
	}
	nodes, err := d.ListSelfNodes(ctx)
	if err != nil {
		t.Fatalf("ListSelfNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("ListSelfNodes 应有 2 条，got %d", len(nodes))
	}
	if nodes[0].ID != n1.ID || nodes[1].ID != n2.ID {
		t.Fatalf("ListSelfNodes 顺序不对: %+v", nodes)
	}
}

// TestProxyGroupMembersRoundTrip 机场自带分组的成员列表应能落库并读回。
func TestProxyGroupMembersRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)
	g, err := d.AddProxyGroupWithMembers(ctx, "PROXY", "select", "", "http://x/204", 300, 0, "airport:fx", []string{"HK-1", "JP-1"})
	if err != nil {
		t.Fatalf("AddProxyGroupWithMembers: %v", err)
	}
	got, err := d.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if strings.Join(got.Members, ",") != "HK-1,JP-1" {
		t.Fatalf("成员未落库: %+v", got)
	}
	list, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
	if err != nil || len(list) != 1 || strings.Join(list[0].Members, ",") != "HK-1,JP-1" {
		t.Fatalf("列表成员不符: %+v err=%v", list, err)
	}
	plain, err := d.AddProxyGroup(ctx, "AUTO", "url-test", ".", "", 300, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if plain.Members != nil {
		t.Fatalf("手工分组 members 应为 nil: %+v", plain.Members)
	}
}

// TestProxyGroupIncludeSelfRoundTrip 分组「包含自建节点」开关应能落库、读回并更新。
func TestProxyGroupIncludeSelfRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)
	g, err := d.AddProxyGroupWithMembersAndIncludeSelf(ctx, "ALL", "select", "", "", 0, 0, "airport:fx", []string{"HK-1"}, true)
	if err != nil {
		t.Fatalf("AddProxyGroupWithMembersAndIncludeSelf: %v", err)
	}
	got, err := d.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if !got.IncludeSelf {
		t.Fatalf("include_self 未落库: %+v", got)
	}
	list, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true})
	if err != nil || len(list) != 1 || !list[0].IncludeSelf {
		t.Fatalf("列表 include_self 不符: %+v err=%v", list, err)
	}
	if err := d.SetProxyGroupIncludeSelf(ctx, g.ID, false); err != nil {
		t.Fatalf("SetProxyGroupIncludeSelf: %v", err)
	}
	got, err = d.GetProxyGroup(ctx, g.ID)
	if err != nil || got.IncludeSelf {
		t.Fatalf("更新后 include_self = %v err=%v，应为 false", got.IncludeSelf, err)
	}
	if err := d.SetProxyGroupIncludeSelf(ctx, 99999, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("不存在分组错误 = %v，期望 sql.ErrNoRows", err)
	}
	plain, err := d.AddProxyGroup(ctx, "PLAIN", "select", ".", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if plain.IncludeSelf {
		t.Fatalf("手工分组 include_self 默认应为 false: %+v", plain)
	}
}
