package store

import (
	"context"
	"fmt"
)

// BatchDeleteRules 在单个事务里按 id 批量删除规则，返回实际删除的行数。
//
// 语义（与逐条 DELETE FROM rules WHERE id = ? 完全一致，只是把 N 次 fsync 合成 1 次）：
//   - ids 为空时直接返回 0, nil（不开事务）；
//   - 不存在的 id 只是「没删到」，RowsAffected()==0，不计入 deleted 也不报错；
//   - 任一条 DELETE 出错立即回滚整批，并返回已累计的 deleted。
func (d *DB) BatchDeleteRules(ctx context.Context, ids []int64) (deleted int, err error) {
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("开始规则删除事务: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, "DELETE FROM rules WHERE id = ?")
	if err != nil {
		return 0, fmt.Errorf("准备规则删除: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		res, execErr := stmt.ExecContext(ctx, id)
		if execErr != nil {
			return deleted, fmt.Errorf("删除规则 %d: %w", id, execErr)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return deleted, fmt.Errorf("读取规则删除结果 %d: %w", id, raErr)
		}
		if n > 0 {
			deleted++
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return deleted, fmt.Errorf("提交规则删除事务: %w", commitErr)
	}
	committed = true
	return deleted, nil
}
