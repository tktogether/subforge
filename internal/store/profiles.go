// Profile（订阅方案）数据层。
//
// v2：方案不再「拥有」规则或分组。规则与分组是全局库，用 source 区分来源；
// 方案只声明「我要哪些层、按什么优先级拼接」：
// rule_layers / group_layers 是层键数组（高到低），默认 custom / airport / template。
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// Profile 是 profiles 表的一行：一套订阅配置 + 层顺序配置。
type Profile struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Config      string `json:"config"`
	Pipeline    string `json:"pipeline"`
	// Target/Include/Exclude 是方案的输出定制（原令牌字段迁移而来）。
	// 令牌字段仍作为旧版兼容保留，但 Web 只在这里维护。
	Target    string `json:"target"`
	Include   string `json:"include"`
	Exclude   string `json:"exclude"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order"`
	// RuleLayers / GroupLayers 是层的优先级列表（高到低），库内以 JSON 文本保存；
	// 空串解析为 nil，由调用方按契约补默认三层。
	RuleLayers  []string `json:"rule_layers"`
	GroupLayers []string `json:"group_layers"`
	// RuleSources 是方案绑定的规则源名列表（规则工厂 manifest 的 source 名），
	// 库内以 JSON 文本保存（编码同 rule_layers）；空串解析为 nil = 未绑定。
	RuleSources []string `json:"rule_sources"`
	// AirportIDs / SelfNodeIDs 是方案使用的节点来源。
	// nil = 未限制（使用全部启用来源）；空数组 = 明确不使用该类来源。
	AirportIDs  []int64 `json:"airport_ids"`
	SelfNodeIDs []int64 `json:"self_node_ids"`
	// RuleCount / GroupCount 是按「已启用层」实时统计的条数，由查询后填充。
	RuleCount  int       `json:"rule_count"`
	GroupCount int       `json:"group_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// profileSelectCols 是方案列表/详情共用的列。
// 规则数与分组数不再用 profile_id 子查询（v2 已无该列），
// 改为读出层配置后由 CountRulesByLayers / CountGroupsByLayers 实时统计。
const profileSelectCols = `SELECT p.id, p.name, p.description, p.config, p.pipeline,
       COALESCE(p.target, ''), COALESCE(p.include, ''), COALESCE(p.exclude, ''),
       p.enabled, p.sort_order,
       COALESCE(p.rule_layers, ''),
       COALESCE(p.group_layers, ''),
       COALESCE(p.rule_sources, ''),
       COALESCE(p.airport_ids, ''),
       COALESCE(p.self_node_ids, ''),
       p.created_at, p.updated_at
FROM profiles p`

// scanProfile 从 rows 的一行读出 Profile（供 List/Get 共用，列序必须与 profileSelectCols 对齐）。
func scanProfile(row interface {
	Scan(dest ...any) error
}) (Profile, error) {
	var p Profile
	var ruleLayers, groupLayers, ruleSources, airportIDs, selfNodeIDs string
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Config, &p.Pipeline,
		&p.Target, &p.Include, &p.Exclude,
		&p.Enabled, &p.SortOrder,
		&ruleLayers, &groupLayers, &ruleSources, &airportIDs, &selfNodeIDs, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Profile{}, err
	}
	p.RuleLayers = parseLayerList(ruleLayers)
	p.GroupLayers = parseLayerList(groupLayers)
	p.RuleSources = parseLayerList(ruleSources)
	p.AirportIDs = parseIDList(airportIDs)
	p.SelfNodeIDs = parseIDList(selfNodeIDs)
	return p, nil
}

// parseIDList 解析库内的 JSON id 数组。
// 空串 = 未限制（nil）；"[]" = 明确不使用该类来源（空切片）。
// 非法 JSON 回退为 nil（未限制），避免旧库/手改数据把订阅整个打空。
func parseIDList(raw string) []int64 {
	if raw == "" {
		return nil
	}
	var out []int64
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if out == nil {
		out = []int64{}
	}
	return out
}

// marshalIDList 把 id 数组序列化成库内 JSON 文本。
// nil = 未限制（空串）；空切片 = 明确不使用（"[]"）。
func marshalIDList(ids []int64) (string, error) {
	if ids == nil {
		return "", nil
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseLayerList 解析库内的 JSON 层数组。
// 空串、空数组或非法 JSON 一律返回 nil（调用方按契约补默认三层）。
func parseLayerList(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// marshalLayerList 把层数组序列化成库内 JSON 文本。
// 空数组落库为 ""（空串 = 默认三层）。
func marshalLayerList(layers []string) (string, error) {
	if len(layers) == 0 {
		return "", nil
	}
	b, err := json.Marshal(layers)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ListProfiles 返回全部订阅方案，按 sort_order, id 排序；空表返回非 nil 空切片。
// 每个方案的 rule_count / group_count 按其层配置实时统计。
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := d.fillProfileCounts(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetProfile 按 id 返回订阅方案；不存在时返回 sql.ErrNoRows。
func (d *DB) GetProfile(ctx context.Context, id int64) (*Profile, error) {
	p, err := scanProfile(d.db.QueryRowContext(ctx, profileSelectCols+" WHERE p.id = ?", id))
	if err != nil {
		return nil, err
	}
	if err := d.fillProfileCounts(ctx, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// fillProfileCounts 按方案的层配置统计规则数与分组数。
// 统计口径与契约一致：规则只数 enabled=true 的；template 层不贡献任何 DB 行。
func (d *DB) fillProfileCounts(ctx context.Context, p *Profile) error {
	rules, err := d.CountRulesByLayers(ctx, p.RuleLayers)
	if err != nil {
		return err
	}
	groups, err := d.CountGroupsByLayers(ctx, p.GroupLayers)
	if err != nil {
		return err
	}
	p.RuleCount = int(rules)
	p.GroupCount = int(groups)
	return nil
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

// ruleColumns 是 rules 表的读取列（v2 已无 profile_id），供 List/Get 类查询共用。
const ruleColumns = "id, rule_type, COALESCE(rule_param, ''), rule_config, resolve_dns, enabled, " +
	"COALESCE(source, ''), COALESCE(raw, ''), COALESCE(origin_key, ''), sort_order, created_at, updated_at"

// scanRule 从一行读出 Rule（列序必须与 ruleColumns 对齐）。
func scanRule(row interface {
	Scan(dest ...any) error
}) (Rule, error) {
	var r Rule
	if err := row.Scan(&r.ID, &r.RuleType, &r.RuleParam, &r.RuleConfig, &r.ResolveDNS, &r.Enabled,
		&r.Source, &r.Raw, &r.OriginKey, &r.SortOrder, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// groupColumns 是 proxy_groups 表的读取列（v2 已无 profile_id，新增 source）。
const groupColumns = "id, name, type, COALESCE(regex, ''), COALESCE(url, ''), COALESCE(interval, 0), " +
	"sort_order, COALESCE(source, ''), created_at, updated_at"

// scanProxyGroup 从一行读出 ProxyGroup（列序必须与 groupColumns 对齐）。
func scanProxyGroup(row interface {
	Scan(dest ...any) error
}) (ProxyGroup, error) {
	var g ProxyGroup
	if err := row.Scan(&g.ID, &g.Name, &g.Type, &g.Regex, &g.URL, &g.Interval,
		&g.SortOrder, &g.Source, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return ProxyGroup{}, err
	}
	return g, nil
}

// CreateSubTokenWithProfile 新增绑定指定方案的订阅令牌。
// profileID=0 即「未绑定方案」，与旧行为完全一致（v2 仍保留令牌→方案的绑定）。
func (d *DB) CreateSubTokenWithProfile(ctx context.Context, token, name, target, pipeline, configName, include, exclude string, profileID int64) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO sub_tokens (token, name, target, pipeline, config, include, exclude, profile_id) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		token, name, target, pipeline, configName, include, exclude, profileID)
	return err
}

// SetProfileRenderOptions 保存方案的 target/include/exclude；nil = 不修改该列。
func (d *DB) SetProfileRenderOptions(ctx context.Context, id int64, target, include, exclude *string) error {
	if target == nil && include == nil && exclude == nil {
		return nil
	}
	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)
	for _, f := range []struct {
		col string
		val *string
	}{{"target", target}, {"include", include}, {"exclude", exclude}} {
		if f.val != nil {
			sets = append(sets, f.col+" = ?")
			args = append(args, strings.TrimSpace(*f.val))
		}
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now(), id)
	res, err := d.db.ExecContext(ctx, "UPDATE profiles SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
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

// SetProfileSources 保存方案使用的机场/自建节点来源。
// 任一参数为 nil 时不动该列；非 nil 时按 nil=未限制、空数组=明确不使用的语义落库。
func (d *DB) SetProfileSources(ctx context.Context, id int64, airportIDs, selfNodeIDs *[]int64) error {
	if airportIDs == nil && selfNodeIDs == nil {
		return nil
	}
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if airportIDs != nil {
		raw, err := marshalIDList(*airportIDs)
		if err != nil {
			return err
		}
		sets = append(sets, "airport_ids = ?")
		args = append(args, raw)
	}
	if selfNodeIDs != nil {
		raw, err := marshalIDList(*selfNodeIDs)
		if err != nil {
			return err
		}
		sets = append(sets, "self_node_ids = ?")
		args = append(args, raw)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now(), id)
	res, err := d.db.ExecContext(ctx, "UPDATE profiles SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
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

// SetProfileRuleSources 保存方案绑定的规则源名；空数组落库为 ""（空串 = 未绑定）。
// 只更新 rule_sources 一列：PUT 只带 rule_sources 时不得改写 rule_layers / group_layers。
// 目标不存在时返回 sql.ErrNoRows。
func (d *DB) SetProfileRuleSources(ctx context.Context, id int64, sources []string) error {
	sourcesJSON, err := marshalLayerList(sources)
	if err != nil {
		return err
	}
	res, err := d.db.ExecContext(ctx,
		"UPDATE profiles SET rule_sources = ?, updated_at = ? WHERE id = ?",
		sourcesJSON, time.Now(), id)
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
