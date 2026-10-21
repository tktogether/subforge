package store

import (
	"context"
	"fmt"
	"time"
)

// ImportedRuleRow 是一行待导入的候选规则，字段与 rules 表导入所需的列一一对应。
//
// 导入规则一律进入候选队列（Enabled=false），由用户在「规则」页显式启用。
type ImportedRuleRow struct {
	RuleType   string
	RuleParam  string
	RuleConfig int64
	ResolveDNS bool
	Enabled    bool
	SortOrder  int
	Source     string
	Raw        string
	OriginKey  string
	// ProfileID 标识该导入规则归属的订阅方案，0 = 未分配（旧的全局池）。
	ProfileID int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BatchInsertImportedRules 在单个事务里批量写入导入规则。
//
// 语义与逐条 INSERT 完全一致，只是把 N 次 fsync 合成 1 次：
//   - INSERT OR IGNORE 命中 (source, origin_key) 部分唯一索引时忽略该行，
//     RowsAffected()==0 视为「已存在」，skipped++ 并刷新 updated_at（与原逻辑一致）；
//   - 其余任何错误立即回滚并返回已统计的 added/skipped，调用方语义不变；
//   - rows 为空时直接返回 0, 0, nil（不开事务）。
func (d *DB) BatchInsertImportedRules(ctx context.Context, rows []ImportedRuleRow) (added, skipped int, err error) {
	if len(rows) == 0 {
		return 0, 0, nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("开始导入事务: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	insertStmt, err := tx.PrepareContext(ctx,
		"INSERT OR IGNORE INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, profile_id, source, raw, origin_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, 0, fmt.Errorf("准备规则插入: %w", err)
	}
	defer insertStmt.Close()

	// WHERE 里的 origin_key <> '' 是必须的：唯一索引 idx_rules_origin 是部分索引
	// （WHERE origin_key <> ''），SQLite 只有在查询条件能推出该谓词时才会用索引；
	// 少了它，每次改时间戳都会全表扫一遍，1 万条重复导入会退化到 20s+。
	// 语义不受影响：origin_key='' 的行不受唯一约束，永远不会走到「已存在」分支。
	// 索引已改为 (profile_id, source, origin_key)，刷新时需带上 profile_id 避免误刷其他方案的同 key 行。
	updateStmt, err := tx.PrepareContext(ctx,
		"UPDATE rules SET updated_at = ? WHERE profile_id = ? AND source = ? AND origin_key = ? AND origin_key <> ''")
	if err != nil {
		return 0, 0, fmt.Errorf("准备规则更新时间戳: %w", err)
	}
	defer updateStmt.Close()

	for _, row := range rows {
		res, execErr := insertStmt.ExecContext(ctx,
			row.RuleType, row.RuleParam, row.RuleConfig, row.ResolveDNS, row.Enabled,
			row.SortOrder, row.ProfileID, row.Source, row.Raw, row.OriginKey, row.CreatedAt, row.UpdatedAt)
		if execErr != nil {
			return added, skipped, fmt.Errorf("写入规则 %q: %w", row.OriginKey, execErr)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return added, skipped, fmt.Errorf("读取规则写入结果 %q: %w", row.OriginKey, raErr)
		}
		if n == 0 {
			// UNIQUE 冲突（已导入过）：跳过但刷新时间戳，不中断整批。
			skipped++
			if _, upErr := updateStmt.ExecContext(ctx, row.UpdatedAt, row.ProfileID, row.Source, row.OriginKey); upErr != nil {
				return added, skipped, fmt.Errorf("刷新规则时间戳 %q: %w", row.OriginKey, upErr)
			}
			continue
		}
		added++
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return added, skipped, fmt.Errorf("提交导入事务: %w", commitErr)
	}
	committed = true
	return added, skipped, nil
}

// MaxRulesSortOrder 返回 rules 表当前最大的 sort_order；空表返回 0。
//
// 与 ListRules 后取 max(SortOrder) 语义一致，但只扫索引/一行聚合，
// 避免为了一个新序号把上万行规则全部读进内存。
func (d *DB) MaxRulesSortOrder(ctx context.Context) (int, error) {
	var maxSort int
	if err := d.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sort_order), 0) FROM rules").Scan(&maxSort); err != nil {
		return 0, fmt.Errorf("查询规则最大排序号: %w", err)
	}
	return maxSort, nil
}
