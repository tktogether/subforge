package store

// rules_batch.go 的补测。
//
// BatchInsertImportedRules / MaxRulesSortOrder 只在 internal/subforge 的导入路径里
// 被间接调用，而 go test -coverprofile 只统计被测包自身的覆盖——store 包的门禁是
// -min-func 100（每个函数至少执行一次），所以在 store 包里直接测这两个函数。

import (
	"testing"
	"time"
)

// batchRow 造一行待导入规则；origin 同时用作 origin_key 与规则参数后缀。
func batchRow(source, origin string, sortOrder int, ts time.Time) ImportedRuleRow {
	return ImportedRuleRow{
		RuleType:   "DOMAIN-SUFFIX",
		RuleParam:  origin + ".test",
		RuleConfig: 1,
		ResolveDNS: true,
		Enabled:    false,
		SortOrder:  sortOrder,
		Source:     source,
		Raw:        "DOMAIN-SUFFIX," + origin + ".test,DIRECT",
		OriginKey:  origin,
		CreatedAt:  ts,
		UpdatedAt:  ts,
	}
}

// TestMaxRulesSortOrderEmpty 空表返回 0（COALESCE 兜底）。
func TestMaxRulesSortOrderEmpty(t *testing.T) {
	d, ctx := newTestDB(t)
	got, err := d.MaxRulesSortOrder(ctx)
	if err != nil {
		t.Fatalf("MaxRulesSortOrder 空表失败: %v", err)
	}
	if got != 0 {
		t.Fatalf("空表 MaxRulesSortOrder = %d, want 0", got)
	}
}

// TestBatchInsertImportedRulesEmpty 空批次不开事务，直接返回 0,0,nil。
func TestBatchInsertImportedRulesEmpty(t *testing.T) {
	d, ctx := newTestDB(t)
	added, skipped, err := d.BatchInsertImportedRules(ctx, nil)
	if err != nil || added != 0 || skipped != 0 {
		t.Fatalf("空批次 = (%d,%d,%v), want (0,0,nil)", added, skipped, err)
	}
}

// TestBatchInsertImportedRulesInsertAndSkip 首次整批写入，重复导入整批跳过并刷新 updated_at。
func TestBatchInsertImportedRulesInsertAndSkip(t *testing.T) {
	d, ctx := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	rows := []ImportedRuleRow{
		batchRow("airport:HK", "a", 1, now),
		batchRow("airport:HK", "b", 2, now),
	}

	added, skipped, err := d.BatchInsertImportedRules(ctx, rows)
	if err != nil {
		t.Fatalf("首次批量导入失败: %v", err)
	}
	if added != 2 || skipped != 0 {
		t.Fatalf("首次批量导入 = (%d,%d), want (2,0)", added, skipped)
	}
	if got, err := d.MaxRulesSortOrder(ctx); err != nil || got != 2 {
		t.Fatalf("MaxRulesSortOrder = (%d,%v), want (2,nil)", got, err)
	}

	later := now.Add(2 * time.Minute)
	rows[0].UpdatedAt = later
	rows[1].UpdatedAt = later
	added, skipped, err = d.BatchInsertImportedRules(ctx, rows)
	if err != nil {
		t.Fatalf("重复批量导入失败: %v", err)
	}
	if added != 0 || skipped != 2 {
		t.Fatalf("重复批量导入 = (%d,%d), want (0,2)", added, skipped)
	}

	rows2, err := d.Raw().QueryContext(ctx, "SELECT origin_key, updated_at FROM rules ORDER BY id")
	if err != nil {
		t.Fatalf("查询 rules 失败: %v", err)
	}
	defer rows2.Close()
	var n int
	for rows2.Next() {
		var origin string
		var updated time.Time
		if err := rows2.Scan(&origin, &updated); err != nil {
			t.Fatalf("扫描 rules 行失败: %v", err)
		}
		n++
		if !updated.Equal(later) {
			t.Errorf("规则 %q 的 updated_at = %v, want %v（重复导入应刷新时间戳）", origin, updated, later)
		}
	}
	if err := rows2.Err(); err != nil {
		t.Fatalf("遍历 rules 失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("rules 行数 = %d, want 2（重复导入不应新增行）", n)
	}
}

// TestBatchInsertImportedRulesEmptyOriginKeyAlwaysAdded origin_key 为空的行不受
// (source, origin_key) 部分唯一索引约束，每次导入都新增。
func TestBatchInsertImportedRulesEmptyOriginKeyAlwaysAdded(t *testing.T) {
	d, ctx := newTestDB(t)
	now := time.Now().UTC()
	rows := []ImportedRuleRow{
		batchRow("manual", "", 0, now),
		batchRow("manual", "", 0, now),
	}
	added, skipped, err := d.BatchInsertImportedRules(ctx, rows)
	if err != nil {
		t.Fatalf("origin_key 为空的批量导入失败: %v", err)
	}
	if added != 2 || skipped != 0 {
		t.Fatalf("origin_key 为空的批量导入 = (%d,%d), want (2,0)", added, skipped)
	}
}
