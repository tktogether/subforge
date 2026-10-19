package store

// sub_token_config_test.go 覆盖 sub_tokens 五个定制列（target/pipeline/config/include/exclude）：
//   1. 旧库（无新列）打开后五列存在、默认空串、迁移幂等；
//   2. ListSubTokens / UpdateSubToken 的读写语义（部分更新、空串清空、不存在报 sql.ErrNoRows）。
// 纪律：不改 store.go 的生产逻辑；每个测试用 t.TempDir() 造独立 SQLite 文件。

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// subTokenConfigCols 是本次新增的五个定制列。
var subTokenConfigCols = []string{"target", "pipeline", "config", "include", "exclude"}

// colInfo 是 PRAGMA table_info 里我们关心的一行。
type colInfo struct {
	notNull int
	dflt    sql.NullString
}

// TestSubTokensMigrateLegacyDB 用「老库」验证迁移：
// 手工建只有 token/name/created_at 的 sub_tokens 表并插一行，再走 store.Open，
// 断言五列被补齐（NOT NULL DEFAULT ”）且该行读出空串。
func TestSubTokensMigrateLegacyDB(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "legacy.db")

	// 1. 造老库：只有三列的历史 schema（与 HEAD 4f92af5 的建表语句一致）。
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("打开老库: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE sub_tokens (
		token       TEXT PRIMARY KEY,
		name        TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("建老表: %v", err)
	}
	if _, err := raw.Exec(
		"INSERT INTO sub_tokens (token, name, created_at) VALUES (?, ?, ?)",
		"legacy", "old", "2024-01-01 00:00:00"); err != nil {
		t.Fatalf("插老行: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关老库: %v", err)
	}

	// 2. 用生产入口打开，migrate 必须幂等补齐五列。
	d, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open(老库) 失败: %v", err)
	}
	ctx := context.Background()

	rows, err := d.Raw().QueryContext(ctx, "PRAGMA table_info(sub_tokens)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()
	got := map[string]colInfo{}
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
			t.Fatalf("Scan table_info: %v", err)
		}
		got[name] = colInfo{notNull: notNull, dflt: dfltValue}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows: %v", err)
	}
	for _, col := range subTokenConfigCols {
		ci, ok := got[col]
		if !ok {
			t.Fatalf("老库迁移后缺列 %s（现有列: %v）", col, keysOf(got))
		}
		if ci.notNull != 1 {
			t.Errorf("%s 列 notnull=%d，期望 1", col, ci.notNull)
		}
		if !ci.dflt.Valid || ci.dflt.String != "''" {
			t.Errorf("%s 列默认值=%v，期望 ''", col, ci.dflt)
		}
	}

	// 3. 老行读出空串（不是 NULL 扫描错误）。
	list, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens(老库): %v", err)
	}
	if len(list) != 1 || list[0].Token != "legacy" {
		t.Fatalf("老库 ListSubTokens = %+v，期望 1 行 legacy", list)
	}
	for _, tc := range []struct {
		name string
		got  string
	}{
		{"target", list[0].Target},
		{"pipeline", list[0].Pipeline},
		{"config", list[0].Config},
		{"include", list[0].Include},
		{"exclude", list[0].Exclude},
	} {
		if tc.got != "" {
			t.Errorf("老行 %s=%q，期望空串", tc.name, tc.got)
		}
	}

	// 4. 再打开一次：addColumnIfMissing 幂等，不得报 duplicate column。
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	d2, err := Open(dsn)
	if err != nil {
		t.Fatalf("二次 Open(老库) 失败（迁移不幂等）: %v", err)
	}
	defer d2.Close()
	if _, err := d2.ListSubTokens(ctx); err != nil {
		t.Fatalf("二次 Open 后 ListSubTokens: %v", err)
	}
}

func keysOf(m map[string]colInfo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestListSubTokensConfigColumns 覆盖五列有值时的 SELECT/Scan。
func TestListSubTokensConfigColumns(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, target, pipeline, config, include, exclude, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"tok-cfg", "n", "list", "pipe1", "tpl1", "vless", "vmess", "2024-02-01 00:00:00"); err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	list, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("长度 %d，期望 1", len(list))
	}
	got := list[0]
	if got.Target != "list" || got.Pipeline != "pipe1" || got.Config != "tpl1" ||
		got.Include != "vless" || got.Exclude != "vmess" {
		t.Fatalf("五列读出 = %+v，与写入不符", got)
	}
}

// TestUpdateSubTokenPartial 覆盖部分更新：只动非 nil 字段，nil 保持原值，空串清空。
func TestUpdateSubTokenPartial(t *testing.T) {
	d, ctx := newTestDB(t)
	if _, err := d.Raw().ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, target, pipeline, config, include, exclude, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"tok1", "n1", "list", "p1", "c1", "vless", "vmess", "2024-03-01 00:00:00"); err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	// 只改 target：其余五列必须原样保留。
	newTarget := "clash.meta"
	if err := d.UpdateSubToken(ctx, "tok1", SubTokenUpdate{Target: &newTarget}); err != nil {
		t.Fatalf("UpdateSubToken(target): %v", err)
	}
	tok := mustGetSubToken(t, d, ctx, "tok1")
	if tok.Target != "clash.meta" {
		t.Errorf("Target=%q，期望 clash.meta", tok.Target)
	}
	if tok.Name != "n1" || tok.Pipeline != "p1" || tok.Config != "c1" || tok.Include != "vless" || tok.Exclude != "vmess" {
		t.Errorf("部分更新误伤其他列: %+v", tok)
	}

	// 空串清空 include，且不动 target。
	empty := ""
	if err := d.UpdateSubToken(ctx, "tok1", SubTokenUpdate{Include: &empty}); err != nil {
		t.Fatalf("UpdateSubToken(include=''): %v", err)
	}
	tok = mustGetSubToken(t, d, ctx, "tok1")
	if tok.Include != "" {
		t.Errorf("Include=%q，期望被清空", tok.Include)
	}
	if tok.Target != "clash.meta" {
		t.Errorf("清空 include 误伤 Target=%q", tok.Target)
	}

	// 逐列覆盖剩余分支：pipeline / config / exclude / name 都能单独更新。
	np, nc, ne, nn := "p2", "c2", "ss", "n2"
	if err := d.UpdateSubToken(ctx, "tok1", SubTokenUpdate{
		Pipeline: &np, Config: &nc, Exclude: &ne, Name: &nn,
	}); err != nil {
		t.Fatalf("UpdateSubToken(多列): %v", err)
	}
	tok = mustGetSubToken(t, d, ctx, "tok1")
	if tok.Pipeline != "p2" || tok.Config != "c2" || tok.Exclude != "ss" || tok.Name != "n2" {
		t.Fatalf("多列更新后 = %+v", tok)
	}

	// 全 nil：存在的 token 成功返回，且不改变任何列。
	before := tok
	if err := d.UpdateSubToken(ctx, "tok1", SubTokenUpdate{}); err != nil {
		t.Fatalf("UpdateSubToken(全 nil, 存在): %v", err)
	}
	if after := mustGetSubToken(t, d, ctx, "tok1"); after != before {
		t.Fatalf("全 nil 更新改变了行: before=%+v after=%+v", before, after)
	}
}

// TestUpdateSubTokenNotFound 覆盖不存在的 token：无论是否带字段都返回 sql.ErrNoRows。
func TestUpdateSubTokenNotFound(t *testing.T) {
	d, ctx := newTestDB(t)

	v := "x"
	if err := d.UpdateSubToken(ctx, "missing", SubTokenUpdate{Target: &v}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("带字段更新不存在 token 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdateSubToken(ctx, "missing", SubTokenUpdate{}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("全 nil 更新不存在 token 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestUpdateSubTokenClosedDB 覆盖 ExecContext / QueryRowContext 的错误分支。
func TestUpdateSubTokenClosedDB(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	v := "x"
	if err := d.UpdateSubToken(ctx, "tok1", SubTokenUpdate{Target: &v}); err == nil {
		t.Error("关闭的库带字段更新应报错")
	}
	if err := d.UpdateSubToken(ctx, "tok1", SubTokenUpdate{}); err == nil {
		t.Error("关闭的库全 nil 更新应报错")
	}
}

// mustGetSubToken 取一行并断言存在。
func mustGetSubToken(t *testing.T, d *DB, ctx context.Context, token string) SubToken {
	t.Helper()
	list, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	for _, tok := range list {
		if tok.Token == token {
			return tok
		}
	}
	t.Fatalf("未找到 token %q", token)
	return SubToken{}
}
