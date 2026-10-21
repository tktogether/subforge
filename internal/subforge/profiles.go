package subforge

// 订阅方案（Profile）与「规则/分组全局库」的业务包装层：把 store 层的方案读写、
// 来源过滤与层拼接转成 Manager 方法，并把「不存在」翻译成中文文案（API 层直接展示给用户）。
//
// v2：规则与分组不再归属任何方案，而是全局库，用 source 区分来源
// （自定义 source 为空串，机场导入 source 为 airport:<机场名>）。
// 方案只声明启用哪些层（rule_layers / group_layers）以及层的先后顺序，
// 渲染时按层顺序拼接：custom -> airport -> template。

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tktogether/subforge/internal/store"
)

// Profile 是一个订阅方案。行类型定义在 internal/store。
type Profile = store.Profile

// SourceFilter 是「按来源过滤」条件（v2）。与 store.SourceFilter 一一对应：
// All=true 表示不过滤、返回全部来源；
// All=false 且 Prefix=false 时按来源精确匹配，来源为空串表示只看自定义；
// Prefix=true 时按前缀匹配（如机场前缀 airport:）。
type SourceFilter = store.SourceFilter

// ListProfiles 返回全部订阅方案，按 sort_order, id 排序。
func (m *Manager) ListProfiles(ctx context.Context) ([]Profile, error) {
	return m.db.ListProfiles(ctx)
}

// GetProfile 返回单个订阅方案；不存在时返回「订阅方案 %d 不存在」。
func (m *Manager) GetProfile(ctx context.Context, id int64) (*Profile, error) {
	p, err := m.db.GetProfile(ctx, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("订阅方案 %d 不存在", id)
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// AddProfile 新增订阅方案，返回带计数的新行。
func (m *Manager) AddProfile(ctx context.Context, name, description, configName, pipeline string, enabled bool, sortOrder int) (*Profile, error) {
	return m.db.AddProfile(ctx, name, description, configName, pipeline, enabled, sortOrder)
}

// UpdateProfile 全量覆盖方案的 name/description/config/pipeline/enabled/sort_order；
// 目标不存在时返回 sql.ErrNoRows。
func (m *Manager) UpdateProfile(ctx context.Context, id int64, name, description, configName, pipeline string, enabled bool, sortOrder int) error {
	return m.db.UpdateProfile(ctx, id, name, description, configName, pipeline, enabled, sortOrder)
}

// DeleteProfile 删除订阅方案；目标不存在时返回 sql.ErrNoRows。
func (m *Manager) DeleteProfile(ctx context.Context, id int64) error {
	return m.db.DeleteProfile(ctx, id)
}

// CreateSubTokenWithProfile 新增绑定指定方案的订阅令牌。
// 令牌仍可绑定方案（订阅取数按方案取层），规则与分组本身不再属于方案。
func (m *Manager) CreateSubTokenWithProfile(ctx context.Context, token, name, target, pipeline, configName, include, exclude string, profileID int64) error {
	return m.db.CreateSubTokenWithProfile(ctx, token, name, target, pipeline, configName, include, exclude, profileID)
}

// --- 规则：按来源过滤与批量操作 ---

// ListRulesFiltered 按来源过滤规则。
// 过滤语义完全交给 store：All=true 返回全部来源；否则按 Source 精确匹配（空串=只看自定义），
// Prefix=true 时按前缀匹配。
func (m *Manager) ListRulesFiltered(ctx context.Context, f SourceFilter) ([]Rule, error) {
	return m.db.ListRulesFiltered(ctx, f)
}

// BatchSetRulesEnabledBySource 按来源批量启用/停用规则，返回实际更新的行数。
// All=true 时 store 拒绝（全表批量操作有风险），返回「必须指定来源才能批量操作」。
func (m *Manager) BatchSetRulesEnabledBySource(ctx context.Context, f SourceFilter, enabled bool) (int64, error) {
	return m.db.BatchSetRulesEnabledBySource(ctx, f, enabled)
}

// BatchDeleteRulesBySource 按来源批量删除规则，返回实际删除的行数。
// All=true 时 store 拒绝（全表批量操作有风险），返回「必须指定来源才能批量操作」。
func (m *Manager) BatchDeleteRulesBySource(ctx context.Context, f SourceFilter) (int64, error) {
	return m.db.BatchDeleteRulesBySource(ctx, f)
}

// --- 分组：按来源过滤与新增 ---

// ListProxyGroupsFiltered 按来源过滤代理分组，过滤语义同 ListRulesFiltered。
func (m *Manager) ListProxyGroupsFiltered(ctx context.Context, f SourceFilter) ([]ProxyGroup, error) {
	return m.db.ListProxyGroupsFiltered(ctx, f)
}

// AddProxyGroupWithSource 新增带来源的代理分组。
// source 为空串表示自定义分组；机场导入自动建组时传 airport:<机场名>。
func (m *Manager) AddProxyGroupWithSource(ctx context.Context, name, gtype, regex, url string, interval, sortOrder int, source string) (ProxyGroup, error) {
	return m.db.AddProxyGroupWithSource(ctx, name, gtype, regex, url, interval, sortOrder, source)
}

// --- 方案层（rule_layers / group_layers） ---

// SetProfileLayers 设置方案启用的规则层与分组层及其顺序。
// 层名取 custom / airport / template；传空切片表示默认三层。
// 目标方案不存在时返回 sql.ErrNoRows。
func (m *Manager) SetProfileLayers(ctx context.Context, id int64, ruleLayers, groupLayers []string) error {
	return m.db.SetProfileLayers(ctx, id, ruleLayers, groupLayers)
}

// ListRulesByLayers 按层顺序拼接规则（只取 enabled=true 的规则）。
// 层内按 sort_order、id 升序；同名层重复出现时按首次出现的位置取值。
func (m *Manager) ListRulesByLayers(ctx context.Context, layers []string) ([]Rule, error) {
	return m.db.ListRulesByLayers(ctx, layers)
}

// ListProxyGroupsByLayers 按层顺序拼接代理分组（只取 enabled=true 的分组）。
// 层内按 sort_order、id 升序；分组重名时先到的层优先（先到者胜）。
func (m *Manager) ListProxyGroupsByLayers(ctx context.Context, layers []string) ([]ProxyGroup, error) {
	return m.db.ListProxyGroupsByLayers(ctx, layers)
}

// CountRulesByLayers 统计层内启用的规则条数（只算 enabled=true）。
func (m *Manager) CountRulesByLayers(ctx context.Context, layers []string) (int64, error) {
	return m.db.CountRulesByLayers(ctx, layers)
}

// CountGroupsByLayers 统计层内启用的分组条数（只算 enabled=true）。
func (m *Manager) CountGroupsByLayers(ctx context.Context, layers []string) (int64, error) {
	return m.db.CountGroupsByLayers(ctx, layers)
}

// --- 来源清单 ---

// ListAllRuleSources 返回规则库中出现过的全部来源（含空串，即自定义），按字典序。
func (m *Manager) ListAllRuleSources(ctx context.Context) ([]string, error) {
	return m.db.ListAllRuleSources(ctx)
}

// ListAllGroupSources 返回分组库中出现过的全部来源（含空串，即自定义），按字典序。
func (m *Manager) ListAllGroupSources(ctx context.Context) ([]string, error) {
	return m.db.ListAllGroupSources(ctx)
}
