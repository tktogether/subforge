// Profile（订阅方案）数据层。
//
// 一个方案 = 一套 INI 模板（config）+ 一套默认管线（pipeline）+ 一套规则 + 一套分组。
// 令牌绑定方案后彼此隔离；profile_id = 0 表示「未分配」，语义与改动前完全一致。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Profile 是 profiles 表的一行：一套按方案隔离的订阅配置。
type Profile struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Config      string    `json:"config"`
	Pipeline    string    `json:"pipeline"`
	Enabled     bool      `json:"enabled"`
	SortOrder   int       `json:"sort_order"`
	RuleCount   int       `json:"rule_count"`
	GroupCount  int       `json:"group_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 方案列表共用的列：规则数与分组数用子查询实时统计，避免 N+1 或额外的 COUNT 往返。
const profileSelectCols = `SELECT p.id, p.name, p.description, p.config, p.pipeline, p.enabled, p.sort_order,
       (SELECT COUNT(*) FROM rules r WHERE r.profile_id = p.id),
       (SELECT COUNT(*) FROM proxy_groups g WHERE g.profile_id = p.id),
       p.created_at, p.updated_at
FROM profiles p`

// scanProfile 从 rows 的一行读出 Profile（供 List/Get 共用，列序必须与 profileSelectCols 对齐）。
func scanProfile(row interface {
	Scan(dest ...any) error
}) (Profile, error) {
	var p Profile
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Config, &p.Pipeline, &p.Enabled, &p.SortOrder,
		&p.RuleCount, &p.GroupCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// ListProfiles 返回全部订阅方案，按 sort_order, id 排序；空表返回非 nil 空切片。
func (d *DB) ListProfiles(ctx context.Context) ([]Profile, error) {
	rows, err := d.db.QueryContext(ctx, profileSelectCols+" ORDER BY sort_order, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProfile 按 id 返回订阅方案；不存在时返回 sql.ErrNoRows。
func (d *DB) GetProfile(ctx context.Context, id int64) (*Profile, error) {
	p, err := scanProfile(d.db.QueryRowContext(ctx, profileSelectCols+" WHERE p.id = ?", id))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// AddProfile 新增订阅方案，返回带计数的新行。name 允许空串（由 API 层拒绝）。
// sortOrder>0 用用户值，<=0 表示未指定，落库时回退纳秒时间戳（与分组/规则一致）。
func (d *DB) AddProfile(ctx context.Context, name, description, configName, pipeline string, enabled bool, sortOrder int) (*Profile, error) {
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO profiles (name, description, config, pipeline, enabled, sort_order) VALUES (?, ?, ?, ?, ?, ?)",
		name, description, configName, pipeline, enabled, resolveSortOrder(sortOrder))
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return d.GetProfile(ctx, id)
}

// UpdateProfile 全量覆盖方案的 name/description/config/pipeline/enabled/sort_order；
// 目标不存在时返回 sql.ErrNoRows。
func (d *DB) UpdateProfile(ctx context.Context, id int64, name, description, configName, pipeline string, enabled bool, sortOrder int) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE profiles SET name = ?, description = ?, config = ?, pipeline = ?, enabled = ?, "+
			"sort_order = CASE WHEN ? > 0 THEN ? ELSE sort_order END, updated_at = ? WHERE id = ?",
		name, description, configName, pipeline, enabled, sortOrder, sortOrder, time.Now(), id)
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

// DeleteProfile 按 id 删除订阅方案；目标不存在时返回 sql.ErrNoRows。
// 注意：本函数不校验方案下是否还有规则/分组，那是 API 层 DELETE 的业务约束。
func (d *DB) DeleteProfile(ctx context.Context, id int64) error {
	res, err := d.db.ExecContext(ctx, "DELETE FROM profiles WHERE id = ?", id)
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

// --- 按方案取规则 / 分组 ---

// ruleColumns 是 rules 表的读取列（含 profile_id），供 List/Get 类查询共用。
const ruleColumns = "id, rule_type, COALESCE(rule_param, ''), rule_config, resolve_dns, enabled, " +
	"COALESCE(source, ''), COALESCE(raw, ''), COALESCE(origin_key, ''), sort_order, profile_id, created_at, updated_at"

// scanRule 从一行读出 Rule（列序必须与 ruleColumns 对齐）。
func scanRule(row interface {
	Scan(dest ...any) error
}) (Rule, error) {
	var r Rule
	if err := row.Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled,
		&r.Source, &r.Raw, &r.OriginKey, &r.SortOrder, &r.ProfileID, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// ListRulesByProfile 返回指定方案下的全部规则（profileID=0 即旧的「未分配」全局池）。
func (d *DB) ListRulesByProfile(ctx context.Context, profileID int64) ([]Rule, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT "+ruleColumns+" FROM rules WHERE profile_id = ? ORDER BY sort_order, id", profileID)
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

// groupColumns 是 proxy_groups 表的读取列（含 profile_id）。
const groupColumns = "id, name, type, COALESCE(regex, ''), COALESCE(url, ''), COALESCE(interval, 0), " +
	"sort_order, profile_id, created_at, updated_at"

// scanProxyGroup 从一行读出 ProxyGroup（列序必须与 groupColumns 对齐）。
func scanProxyGroup(row interface {
	Scan(dest ...any) error
}) (ProxyGroup, error) {
	var g ProxyGroup
	if err := row.Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval,
		&g.SortOrder, &g.ProfileID, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return ProxyGroup{}, err
	}
	return g, nil
}

// ListProxyGroupsByProfile 返回指定方案下的全部代理分组（profileID=0 即旧的「未分配」全局池）。
func (d *DB) ListProxyGroupsByProfile(ctx context.Context, profileID int64) ([]ProxyGroup, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT "+groupColumns+" FROM proxy_groups WHERE profile_id = ? ORDER BY sort_order, id", profileID)
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

// AddRuleWithProfile 新增归属指定方案的规则，返回新行。
func (d *DB) AddRuleWithProfile(ctx context.Context, profileID int64, ruleType, ruleParam string, ruleConfig int64, resolveDNS, enabled bool) (*Rule, error) {
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order, profile_id) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)",
		ruleType, ruleParam, ruleConfig, resolveDNS, enabled, resolveSortOrder(0), profileID)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	r, err := scanRule(d.db.QueryRowContext(ctx, "SELECT "+ruleColumns+" FROM rules WHERE id = ?", id))
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// AddProxyGroupWithProfile 新增归属指定方案的代理分组，返回新行。
// sortOrder>0 用用户值，否则回退 time.Now().UnixNano()（与 AddProxyGroup 一致）。
func (d *DB) AddProxyGroupWithProfile(ctx context.Context, profileID int64, name, gtype, regex, url string, interval, sortOrder int) (*ProxyGroup, error) {
	res, err := d.db.ExecContext(ctx,
		"INSERT INTO proxy_groups (name, type, regex, url, interval, sort_order, profile_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		name, gtype, regex, url, interval, resolveSortOrder(sortOrder), profileID)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	g, err := scanProxyGroup(d.db.QueryRowContext(ctx, "SELECT "+groupColumns+" FROM proxy_groups WHERE id = ?", id))
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// SetRuleProfile 把一条规则移到指定方案；不存在时返回 sql.ErrNoRows。
func (d *DB) SetRuleProfile(ctx context.Context, id, profileID int64) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE rules SET profile_id = ?, updated_at = ? WHERE id = ?", profileID, time.Now(), id)
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

// SetProxyGroupProfile 把一个代理分组移到指定方案；不存在时返回 sql.ErrNoRows。
func (d *DB) SetProxyGroupProfile(ctx context.Context, id, profileID int64) error {
	res, err := d.db.ExecContext(ctx,
		"UPDATE proxy_groups SET profile_id = ?, updated_at = ? WHERE id = ?", profileID, time.Now(), id)
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

// errDuplicateOriginInProfile 是「目标方案里已存在同来源同去重键的规则」的固定文案。
// 不用 %w 包装原始 SQLite 错误：API 层按这段文案判 409。
var errDuplicateOriginInProfile = errors.New("目标方案里已存在同来源同去重键的规则，无法移动")

// wrapUniqueRuleError 把唯一索引冲突翻译成固定中文文案，其余错误原样返回。
func wrapUniqueRuleError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE") {
		return errDuplicateOriginInProfile
	}
	return err
}

// placeholderList 生成 ?, ?, ? 占位符串（n>=1）。
func placeholderList(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// MoveRulesToProfile 把若干条规则移到指定方案，返回实际更新的行数。
// 空 ids 直接返回 0, nil；撞唯一索引时返回固定文案错误（不吞原始错误语义，直接换文案）。
func (d *DB) MoveRulesToProfile(ctx context.Context, ids []int64, profileID int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	args := make([]any, 0, len(ids)+2)
	args = append(args, profileID, time.Now())
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE rules SET profile_id = ?, updated_at = ? WHERE id IN ("+placeholderList(len(ids))+")", args...)
	if err != nil {
		return 0, wrapUniqueRuleError(err)
	}
	return res.RowsAffected()
}

// MoveRulesBySourceToProfile 把某个来源的全部规则移到指定方案，返回实际更新的行数。
// source 为空表示「没有来源筛选」，直接返回 0, nil（避免全表搬移）。
func (d *DB) MoveRulesBySourceToProfile(ctx context.Context, source string, profileID int64) (int64, error) {
	if strings.TrimSpace(source) == "" {
		return 0, nil
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE rules SET profile_id = ?, updated_at = ? WHERE source = ?", profileID, time.Now(), source)
	if err != nil {
		return 0, wrapUniqueRuleError(err)
	}
	return res.RowsAffected()
}

// MoveGroupsToProfile 把若干代理分组移到指定方案，返回实际更新的行数。
// proxy_groups 上没有唯一索引，不存在则不报错（与批量移动规则的语义一致）。
func (d *DB) MoveGroupsToProfile(ctx context.Context, ids []int64, profileID int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, profileID)
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE proxy_groups SET profile_id = ? WHERE id IN ("+placeholderList(len(ids))+")", args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CreateSubTokenWithProfile 新增绑定指定方案的订阅令牌。
// profileID=0 即「未绑定方案」，与旧行为完全一致。
func (d *DB) CreateSubTokenWithProfile(ctx context.Context, token, name, target, pipeline, configName, include, exclude string, profileID int64) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, target, pipeline, config, include, exclude, profile_id) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		token, name, target, pipeline, configName, include, exclude, profileID)
	return err
}

// ProfileExists 报告 id 对应的方案是否存在（API 层做 400 校验用）。
func (d *DB) ProfileExists(ctx context.Context, id int64) (bool, error) {
	var got int64
	err := d.db.QueryRowContext(ctx, "SELECT id FROM profiles WHERE id = ?", id).Scan(&got)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// errUnusedProfileImport 防止 fmt 被误删（保留占位，编译期无副作用）。
var _ = fmt.Sprintf
