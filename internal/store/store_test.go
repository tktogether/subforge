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
	"airports", "config", "proxy_groups", "rule_providers", "rules", "self_nodes", "sub_tokens",
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
	n1, err := d.AddSelfNode(ctx, "vless://a", "n1", true)
	if err != nil {
		t.Fatalf("AddSelfNode(n1) 失败: %v", err)
	}
	if n1.Link != "vless://a" || n1.Name != "n1" || !n1.Enabled || n1.ID <= 0 {
		t.Fatalf("AddSelfNode(n1) 返回字段不符: %+v", n1)
	}
	if _, err := d.AddSelfNode(ctx, "trojan://b", "n2", false); err != nil {
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
	if err := d.UpdateSelfNode(ctx, n1.ID, "vmess://c", "n1b", false); err != nil {
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
	if err := d.UpdateSelfNode(ctx, 9999, "x", "y", true); !errors.Is(err, sql.ErrNoRows) {
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

	if err := d.UpdateProxyGroup(ctx, id, "g1b", "url-test", "re:^JP", "http://y", 600); err != nil {
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
	if err := d.UpdateProxyGroup(ctx, 9999, "x", "y", "z", "w", 1); !errors.Is(err, sql.ErrNoRows) {
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
	if _, err := d.AddSelfNode(ctx, "a", "b", true); err == nil {
		t.Error("AddSelfNode(closed) 期望失败")
	}
	// UpdateSelfNode ExecContext err 分支（L144）
	if err := d.UpdateSelfNode(ctx, 1, "a", "b", true); err == nil {
		t.Error("UpdateSelfNode(closed) 期望失败")
	}
	// ListSubTokens QueryContext err 分支（L188）
	if _, err := d.ListSubTokens(ctx); err == nil {
		t.Error("ListSubTokens(closed) 期望失败")
	}
	// UpdateProxyGroup ExecContext err 分支（L220）
	if err := d.UpdateProxyGroup(ctx, 1, "a", "b", "c", "d", 1); err == nil {
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
	sn, err := d.AddSelfNode(ctx, "x", "y", true)
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
	if err := d.UpdateSelfNode(cancelled, sn.ID, "a", "b", false); err == nil {
		t.Error("UpdateSelfNode(cancelled ctx) 期望失败")
	}
	if err := d.UpdateProxyGroup(cancelled, pgID, "a", "b", "c", "d", 1); err == nil {
		t.Error("UpdateProxyGroup(cancelled ctx) 期望失败")
	}
	if err := d.UpdateRule(cancelled, rlID, "a", "b", 1, true, true); err == nil {
		t.Error("UpdateRule(cancelled ctx) 期望失败")
	}
}
