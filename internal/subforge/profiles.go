package subforge

// 订阅方案（Profile）的业务包装层：把 store 层的方案读写转成 Manager 方法，
// 并把「不存在」翻译成中文文案（API 层直接展示给用户）。

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tktogether/subforge/internal/store"
)

// Profile 是一个订阅方案。行类型定义在 internal/store。
type Profile = store.Profile

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

// ListRulesByProfile 返回指定方案下的全部规则。
func (m *Manager) ListRulesByProfile(ctx context.Context, profileID int64) ([]Rule, error) {
	return m.db.ListRulesByProfile(ctx, profileID)
}

// ListProxyGroupsByProfile 返回指定方案下的全部代理分组。
func (m *Manager) ListProxyGroupsByProfile(ctx context.Context, profileID int64) ([]ProxyGroup, error) {
	return m.db.ListProxyGroupsByProfile(ctx, profileID)
}

// AddRuleWithProfile 新增归属指定方案的规则。
func (m *Manager) AddRuleWithProfile(ctx context.Context, profileID int64, ruleType, ruleParam string, ruleConfig int64, resolveDNS, enabled bool) (*Rule, error) {
	return m.db.AddRuleWithProfile(ctx, profileID, ruleType, ruleParam, ruleConfig, resolveDNS, enabled)
}

// AddProxyGroupWithProfile 新增归属指定方案的代理分组。
func (m *Manager) AddProxyGroupWithProfile(ctx context.Context, profileID int64, name, gtype, regex, url string, interval, sortOrder int) (*ProxyGroup, error) {
	return m.db.AddProxyGroupWithProfile(ctx, profileID, name, gtype, regex, url, interval, sortOrder)
}

// SetRuleProfile 把一条规则移到指定方案；不存在时返回「规则 %d 不存在」。
func (m *Manager) SetRuleProfile(ctx context.Context, id, profileID int64) error {
	if err := m.db.SetRuleProfile(ctx, id, profileID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("规则 %d 不存在", id)
		}
		return err
	}
	return nil
}

// SetProxyGroupProfile 把一个代理分组移到指定方案；不存在时返回「代理分组 %d 不存在」。
func (m *Manager) SetProxyGroupProfile(ctx context.Context, id, profileID int64) error {
	if err := m.db.SetProxyGroupProfile(ctx, id, profileID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("代理分组 %d 不存在", id)
		}
		return err
	}
	return nil
}

// MoveRulesToProfile 把若干条规则移到指定方案，返回实际更新的行数。
func (m *Manager) MoveRulesToProfile(ctx context.Context, ids []int64, profileID int64) (int64, error) {
	return m.db.MoveRulesToProfile(ctx, ids, profileID)
}

// MoveRulesBySourceToProfile 把某个来源的全部规则移到指定方案，返回实际更新的行数。
func (m *Manager) MoveRulesBySourceToProfile(ctx context.Context, source string, profileID int64) (int64, error) {
	return m.db.MoveRulesBySourceToProfile(ctx, source, profileID)
}

// MoveGroupsToProfile 把若干代理分组移到指定方案，返回实际更新的行数。
func (m *Manager) MoveGroupsToProfile(ctx context.Context, ids []int64, profileID int64) (int64, error) {
	return m.db.MoveGroupsToProfile(ctx, ids, profileID)
}

// CreateSubTokenWithProfile 新增绑定指定方案的订阅令牌。
func (m *Manager) CreateSubTokenWithProfile(ctx context.Context, token, name, target, pipeline, configName, include, exclude string, profileID int64) error {
	return m.db.CreateSubTokenWithProfile(ctx, token, name, target, pipeline, configName, include, exclude, profileID)
}
