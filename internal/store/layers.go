// 来源分层查询层。
//
// v2 里规则与分组都是全局库，方案只通过「层」来声明自己看到哪些行：
//
//	custom    → source = ''            自定义规则 / 自定义分组
//	airport   → source LIKE 'airport:%' 机场导入（自动建组）
//	template  → DB 不贡献任何行          模板层由引擎提供
//
// 层顺序即拼接顺序：按 layers 数组从高到低取行，同层内按 sort_order, id 升序。
package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// DefaultLayers 是方案的默认层顺序（优先级从高到低）。
var DefaultLayers = []string{"custom", "airport", "template"}

// SourceFilter 是「按来源过滤」的参数。
//
// 零值（Source=""、Prefix=false、All=false）= 只看自定义（source = ”），
// 与契约 §4.2「source 为空串 = 只看自定义」一致；
// All=true 表示不过滤、返回全部行（此时 Source / Prefix 被忽略）。
type SourceFilter struct {
	Source string
	Prefix bool
	All    bool
}

// filterClause 把 SourceFilter 翻译成 WHERE 片段与参数；All=true 时返回空片段（即无 WHERE）。
func filterClause(f SourceFilter) (string, []any, error) {
	if f.All {
		return "", nil, nil
	}
	if f.Prefix {
		if f.Source == "" {
			return "", nil, errors.New("前缀过滤需要非空来源")
		}
		return "source LIKE ? ESCAPE '\\'", []any{escapeLike(f.Source) + "%"}, nil
	}
	return "source = ?", []any{f.Source}, nil
}

// batchFilterClause 与 filterClause 相同，但禁止 All：批量启停/删除必须带筛选条件，
// 否则一条 UPDATE/DELETE 会波及全表。
func batchFilterClause(f SourceFilter) (string, []any, error) {
	if f.All {
		return "", nil, errors.New("必须指定来源才能批量操作")
	}
	return filterClause(f)
}

// escapeLike 转义 LIKE 通配符，避免来源里的 % 与 _ 被当成模式。
func escapeLike(s string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(s)
}

// effectiveLayers 归一化层数组：空数组（或 nil）取默认三层。
func effectiveLayers(layers []string) []string {
	if len(layers) == 0 {
		return append([]string(nil), DefaultLayers...)
	}
	return layers
}

// layerWhere 返回某个层在 DB 里的 WHERE 片段。
// template 层返回 ok=false：DB 不贡献任何行，由引擎提供模板规则。
func layerWhere(layer string) (string, bool) {
	switch layer {
	case "custom":
		return "source = ''", true
	case "airport":
		return "source LIKE 'airport:%'", true
	default:
		// template 及未知层键都不贡献 DB 行（层键校验在 API 层）。
		return "", false
	}
}

// layerOrClause 把若干层合成 "(clause OR clause ...)"，用于 COUNT 之类的一次性统计。
// 层键去重（同层重复只算一次），template / 未知层键被忽略；
// 没有任何可命中层时返回空串（调用方按 0 处理）。
func layerOrClause(layers []string) string {
	seen := map[string]bool{}
	parts := []string{}
	for _, layer := range effectiveLayers(layers) {
		if seen[layer] {
			continue
		}
		seen[layer] = true
		clause, ok := layerWhere(layer)
		if !ok {
			continue
		}
		parts = append(parts, clause)
	}
	if len(parts) == 0 {
		return ""
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// ListRulesFiltered 按来源过滤返回规则（All=true 返回全部）。
func (d *DB) ListRulesFiltered(ctx context.Context, f SourceFilter) ([]Rule, error) {
	where, args, err := filterClause(f)
	if err != nil {
		return nil, err
	}
	query := "SELECT " + ruleColumns + " FROM rules"
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY sort_order, id"
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListProxyGroupsFiltered 按来源过滤返回代理分组（All=true 返回全部）。
func (d *DB) ListProxyGroupsFiltered(ctx context.Context, f SourceFilter) ([]ProxyGroup, error) {
	where, args, err := filterClause(f)
	if err != nil {
		return nil, err
	}
	query := "SELECT " + groupColumns + " FROM proxy_groups"
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY sort_order, id"
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProxyGroup{}
	for rows.Next() {
		g, err := scanProxyGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// BatchSetRulesEnabledBySource 批量启用/停用某个来源的规则，返回受影响行数。
func (d *DB) BatchSetRulesEnabledBySource(ctx context.Context, f SourceFilter, enabled bool) (int64, error) {
	where, args, err := batchFilterClause(f)
	if err != nil {
		return 0, err
	}
	params := append([]any{enabled, time.Now()}, args...)
	res, err := d.db.ExecContext(ctx,
		"UPDATE rules SET enabled = ?, updated_at = ? WHERE "+where, params...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// BatchDeleteRulesBySource 批量删除某个来源的规则，返回删除行数。
func (d *DB) BatchDeleteRulesBySource(ctx context.Context, f SourceFilter) (int64, error) {
	where, args, err := batchFilterClause(f)
	if err != nil {
		return 0, err
	}
	res, err := d.db.ExecContext(ctx, "DELETE FROM rules WHERE "+where, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetRuleSource 修改单条规则的来源（空串=自定义），用于把手工规则归到某个来源。
// 目标不存在返回 sql.ErrNoRows；撞唯一索引 (source, origin_key) 时返回固定文案错误。
func (d *DB) SetRuleSource(ctx context.Context, id int64, source string) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE rules SET source = ?, updated_at = ? WHERE id = ?", source, time.Now(), id)
	if err != nil {
		return wrapUniqueRuleError(err)
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

// SetProfileLayers 保存方案的层顺序配置；空数组落库为 ""（空串 = 默认三层）。
// 目标不存在时返回 sql.ErrNoRows。
func (d *DB) SetProfileLayers(ctx context.Context, id int64, ruleLayers, groupLayers []string) error {
	ruleJSON, err := marshalLayerList(ruleLayers)
	if err != nil {
		return err
	}
	groupJSON, err := marshalLayerList(groupLayers)
	if err != nil {
		return err
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE profiles SET rule_layers = ?, group_layers = ?, updated_at = ? WHERE id = ?",
		ruleJSON, groupJSON, time.Now(), id)
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

// CountRulesByLayers 统计已启用规则中属于给定层的条数（仅 enabled=true；template 不计）。
func (d *DB) CountRulesByLayers(ctx context.Context, layers []string) (int64, error) {
	clause := layerOrClause(layers)
	if clause == "" {
		return 0, nil
	}
	var n int64
	err := d.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rules WHERE enabled = 1 AND "+clause).Scan(&n)
	return n, err
}

// CountGroupsByLayers 统计属于给定层的分组条数（DB 条数，同名不去重）。
func (d *DB) CountGroupsByLayers(ctx context.Context, layers []string) (int64, error) {
	clause := layerOrClause(layers)
	if clause == "" {
		return 0, nil
	}
	var n int64
	err := d.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM proxy_groups WHERE "+clause).Scan(&n)
	return n, err
}

// ListRulesByLayers 按层顺序拼接规则：只取已启用规则，同层内按 sort_order, id 升序。
func (d *DB) ListRulesByLayers(ctx context.Context, layers []string) ([]Rule, error) {
	out := []Rule{}
	for _, layer := range effectiveLayers(layers) {
		clause, ok := layerWhere(layer)
		if !ok {
			continue
		}
		rows, err := d.db.QueryContext(ctx,
			"SELECT "+ruleColumns+" FROM rules WHERE enabled = 1 AND "+clause+" ORDER BY sort_order, id")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			r, err := scanRule(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, r)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// ListProxyGroupsByLayers 按层顺序拼接代理分组，同名分组先到者胜（后层同名被丢弃）。
func (d *DB) ListProxyGroupsByLayers(ctx context.Context, layers []string) ([]ProxyGroup, error) {
	out := []ProxyGroup{}
	seen := map[string]bool{}
	for _, layer := range effectiveLayers(layers) {
		clause, ok := layerWhere(layer)
		if !ok {
			continue
		}
		rows, err := d.db.QueryContext(ctx,
			"SELECT "+groupColumns+" FROM proxy_groups WHERE "+clause+" ORDER BY sort_order, id")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			g, err := scanProxyGroup(rows)
			if err != nil {
				rows.Close()
				return nil, err
			}
			if seen[g.Name] {
				continue
			}
			seen[g.Name] = true
			out = append(out, g)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// ListAllRuleSources 返回 rules 表里出现过的全部来源（DISTINCT，含空串）。
func (d *DB) ListAllRuleSources(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT DISTINCT source FROM rules ORDER BY source")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListAllGroupSources 返回 proxy_groups 表里出现过的全部来源（DISTINCT，含空串）。
func (d *DB) ListAllGroupSources(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT DISTINCT source FROM proxy_groups ORDER BY source")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// errDuplicateOriginInSource 是「目标来源里已存在同来源同去重键的规则」的固定文案。
// 不用 %w 包装原始 SQLite 错误：API 层按这段文案判 409。
var errDuplicateOriginInSource = errors.New("目标来源里已存在同来源同去重键的规则")

// wrapUniqueRuleError 把唯一索引冲突翻译成固定中文文案，其余错误原样返回。
func wrapUniqueRuleError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE") {
		return errDuplicateOriginInSource
	}
	return err
}
