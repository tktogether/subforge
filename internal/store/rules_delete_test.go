package store

import (
	"context"
	"path/filepath"
	"testing"
)

// insertRule 往 rules 表塞一行，返回自增 id。
func insertRule(t *testing.T, d *DB, param string) int64 {
	t.Helper()
	res, err := d.Raw().Exec(
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES ('DOMAIN', ?, 0, 0, 1, 0)",
		param)
	if err != nil {
		t.Fatalf("插入规则 %q 失败: %v", param, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("取规则 id 失败: %v", err)
	}
	return id
}

func countRules(t *testing.T, d *DB, ctx context.Context) int {
	t.Helper()
	var n int
	if err := d.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM rules").Scan(&n); err != nil {
		t.Fatalf("统计规则数失败: %v", err)
	}
	return n
}

// TestBatchDeleteRules_Empty 覆盖空 ids：不开事务，直接 0, nil。
func TestBatchDeleteRules_Empty(t *testing.T) {
	d, ctx := newTestDB(t)
	insertRule(t, d, "keep.example.com")

	deleted, err := d.BatchDeleteRules(ctx, nil)
	if err != nil {
		t.Fatalf("BatchDeleteRules(nil) 报错: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d，应为 0", deleted)
	}
	if got := countRules(t, d, ctx); got != 1 {
		t.Fatalf("剩余规则数 = %d，应为 1", got)
	}
}

// TestBatchDeleteRules_Some 覆盖正常批量删除：只删给出的 id，其余保留。
func TestBatchDeleteRules_Some(t *testing.T) {
	d, ctx := newTestDB(t)
	keep := insertRule(t, d, "keep.example.com")
	del1 := insertRule(t, d, "del1.example.com")
	del2 := insertRule(t, d, "del2.example.com")

	deleted, err := d.BatchDeleteRules(ctx, []int64{del1, del2})
	if err != nil {
		t.Fatalf("BatchDeleteRules 报错: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d，应为 2", deleted)
	}
	if got := countRules(t, d, ctx); got != 1 {
		t.Fatalf("剩余规则数 = %d，应为 1", got)
	}
	if _, err := d.GetRule(ctx, keep); err != nil {
		t.Fatalf("未选中的规则被误删: %v", err)
	}
	for _, id := range []int64{del1, del2} {
		if _, err := d.GetRule(ctx, id); err == nil {
			t.Fatalf("规则 %d 应已被删除", id)
		}
	}
}

// TestBatchDeleteRules_Nonexistent 覆盖不存在的 id：不算删除、不报错。
func TestBatchDeleteRules_Nonexistent(t *testing.T) {
	d, ctx := newTestDB(t)
	insertRule(t, d, "keep.example.com")

	deleted, err := d.BatchDeleteRules(ctx, []int64{987654321})
	if err != nil {
		t.Fatalf("BatchDeleteRules 报错: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d，应为 0", deleted)
	}
	if got := countRules(t, d, ctx); got != 1 {
		t.Fatalf("剩余规则数 = %d，应为 1", got)
	}
}

// TestBatchDeleteRules_Mixed 覆盖「存在 + 不存在」混合：只统计真正删掉的。
func TestBatchDeleteRules_Mixed(t *testing.T) {
	d, ctx := newTestDB(t)
	real := insertRule(t, d, "del.example.com")
	insertRule(t, d, "keep.example.com")

	deleted, err := d.BatchDeleteRules(ctx, []int64{real, 987654321})
	if err != nil {
		t.Fatalf("BatchDeleteRules 报错: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d，应为 1", deleted)
	}
	if got := countRules(t, d, ctx); got != 1 {
		t.Fatalf("剩余规则数 = %d，应为 1", got)
	}
}

var _ = filepath.Join
