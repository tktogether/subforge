package store

// internal/store/profiles.go 的 rule_sources 补测：新列契约、Setter 往返、
// 与 rule_layers/group_layers 的隔离，以及老库迁移补齐该列。
//
// 纪律同 profiles_test.go：每个测试用 t.TempDir() 造独立 SQLite 文件（不用 :memory:）。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

// TestProfileRuleSourcesColumn 覆盖新列存在、类型 TEXT、NOT NULL、默认 ”。
func TestProfileRuleSourcesColumn(t *testing.T) {
	d, _ := newTestDB(t)
	if ok, err := columnExists(d.Raw(), "profiles", "rule_sources"); err != nil || !ok {
		t.Fatalf("profiles.rule_sources 应存在，实际 ok=%v err=%v", ok, err)
	}

	var (
		name    string
		ctype   string
		notNull int
		dflt    sql.NullString
		pk      int
	)
	row := d.Raw().QueryRow(
		`SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('profiles') WHERE name = 'rule_sources'`)
	if err := row.Scan(&name, &ctype, &notNull, &dflt, &pk); err != nil {
		t.Fatalf("读 rule_sources 列定义: %v", err)
	}
	if ctype != "TEXT" {
		t.Errorf("rule_sources 类型=%q，期望 TEXT", ctype)
	}
	if notNull != 1 {
		t.Errorf("rule_sources NOT NULL=%d，期望 1", notNull)
	}
	if !dflt.Valid || (dflt.String != "" && dflt.String != "''") {
		t.Errorf("rule_sources 默认值=%#v，期望空串", dflt)
	}
	if pk != 0 {
		t.Errorf("rule_sources 不应是主键，pk=%d", pk)
	}
}

// TestProfileRuleSourcesRoundTrip 覆盖 Add→默认未绑定、Setter 往返、空数组回落、JSON 编码与列序。
func TestProfileRuleSourcesRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "规则源方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.RuleSources != nil {
		t.Fatalf("新方案 RuleSources 应为 nil，实际 %v", got.RuleSources)
	}
	var raw string
	if err := d.Raw().QueryRowContext(ctx,
		"SELECT rule_sources FROM profiles WHERE id = ?", p.ID).Scan(&raw); err != nil {
		t.Fatalf("读库内 rule_sources: %v", err)
	}
	if raw != "" {
		t.Fatalf("新方案 rule_sources 应为空串，实际 %q", raw)
	}

	// Setter 往返（GetProfile 也走 scanProfile，覆盖新列在 select 列表里的位置）。
	if err := d.SetProfileRuleSources(ctx, p.ID, []string{"Custom_Clash", "Custom_SingBox"}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}
	got, err = d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile(设置后): %v", err)
	}
	if len(got.RuleSources) != 2 || got.RuleSources[0] != "Custom_Clash" || got.RuleSources[1] != "Custom_SingBox" {
		t.Fatalf("RuleSources 回读=%v，期望 [Custom_Clash Custom_SingBox]", got.RuleSources)
	}
	if err := d.Raw().QueryRowContext(ctx,
		"SELECT rule_sources FROM profiles WHERE id = ?", p.ID).Scan(&raw); err != nil {
		t.Fatalf("读库内 rule_sources: %v", err)
	}
	if raw != `["Custom_Clash","Custom_SingBox"]` {
		t.Fatalf("库内 JSON=%q，期望紧凑 JSON 数组", raw)
	}

	// 空数组 = 空串 = 未绑定：回读为 nil。
	if err := d.SetProfileRuleSources(ctx, p.ID, nil); err != nil {
		t.Fatalf("SetProfileRuleSources(nil): %v", err)
	}
	got, err = d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile(清空后): %v", err)
	}
	if got.RuleSources != nil {
		t.Fatalf("空数组应回读为 nil，实际 %v", got.RuleSources)
	}

	// JSON 字段名契约：API 层直接序列化 Profile。
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(Profile): %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := fields["rule_sources"]; !ok {
		t.Errorf("Profile JSON 缺少 rule_sources 字段，实际键：%s", string(b))
	}
	if _, ok := fields["rule_layers"]; !ok {
		t.Errorf("Profile JSON 不应丢掉 rule_layers 字段，实际键：%s", string(b))
	}
}

// TestSetProfileRuleSourcesUnknownProfile 覆盖不存在 id → sql.ErrNoRows。
func TestSetProfileRuleSourcesUnknownProfile(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.SetProfileRuleSources(ctx, 9999, []string{"a"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProfileRuleSources(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestSetProfileRuleSourcesIsolatedFromLayers 覆盖 Setter 只动 rule_sources：
// 设置规则源不得改写 rule_layers/group_layers，反之亦然。
func TestSetProfileRuleSourcesIsolatedFromLayers(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "隔离方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	layers := []string{"custom", "airport"}
	if err := d.SetProfileLayers(ctx, p.ID, layers, layers); err != nil {
		t.Fatalf("SetProfileLayers: %v", err)
	}

	if err := d.SetProfileRuleSources(ctx, p.ID, []string{"a", "b"}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.RuleLayers) != 2 || len(got.GroupLayers) != 2 {
		t.Fatalf("设置规则源不应改层，实际 rule=%v group=%v", got.RuleLayers, got.GroupLayers)
	}
	if len(got.RuleSources) != 2 {
		t.Fatalf("RuleSources=%v，期望 [a b]", got.RuleSources)
	}

	// 反向：清空层不应影响已绑定的规则源。
	if err := d.SetProfileLayers(ctx, p.ID, nil, nil); err != nil {
		t.Fatalf("SetProfileLayers(nil): %v", err)
	}
	got, err = d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile(清层后): %v", err)
	}
	if got.RuleLayers != nil || got.GroupLayers != nil {
		t.Fatalf("层应为 nil，实际 %v / %v", got.RuleLayers, got.GroupLayers)
	}
	if len(got.RuleSources) != 2 || got.RuleSources[1] != "b" {
		t.Fatalf("清空层不应动规则源，实际 %v", got.RuleSources)
	}

	// updated_at 被刷新（否则 API 层的「已修改」提示会丢）。
	if _, err := d.Raw().ExecContext(ctx,
		"UPDATE profiles SET updated_at = '2000-01-01 00:00:00' WHERE id = ?", p.ID); err != nil {
		t.Fatalf("把 updated_at 改回旧值: %v", err)
	}
	if err := d.SetProfileRuleSources(ctx, p.ID, []string{"c"}); err != nil {
		t.Fatalf("SetProfileRuleSources(刷新时间戳): %v", err)
	}
	var ts string
	if err := d.Raw().QueryRowContext(ctx,
		"SELECT updated_at FROM profiles WHERE id = ?", p.ID).Scan(&ts); err != nil {
		t.Fatalf("读 updated_at: %v", err)
	}
	if ts == "2000-01-01 00:00:00" || ts == "" {
		t.Fatalf("updated_at 未被刷新，实际 %q", ts)
	}
}

// TestSetProfileRuleSourcesClosedDB 覆盖连接关闭时的错误分支。
func TestSetProfileRuleSourcesClosedDB(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := d.SetProfileRuleSources(ctx, 1, []string{"a"}); err == nil {
		t.Error("SetProfileRuleSources(closed) 期望失败")
	}
}

// TestLegacyV018AddsRuleSourcesColumn 覆盖老库迁移也补齐 rule_sources。
func TestLegacyV018AddsRuleSourcesColumn(t *testing.T) {
	path := openLegacyDB(t, legacyV018SchemaDDL)
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open(老库): %v", err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()
	if ok, err := columnExists(d.Raw(), "profiles", "rule_sources"); err != nil || !ok {
		t.Fatalf("老库迁移后应有 profiles.rule_sources，实际 ok=%v err=%v", ok, err)
	}
	ctx := context.Background()
	p, err := d.AddProfile(ctx, "老库方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("老库 AddProfile: %v", err)
	}
	if err := d.SetProfileRuleSources(ctx, p.ID, []string{"legacy-a"}); err != nil {
		t.Fatalf("老库 SetProfileRuleSources: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("老库 GetProfile: %v", err)
	}
	if len(got.RuleSources) != 1 || got.RuleSources[0] != "legacy-a" {
		t.Fatalf("老库 RuleSources=%v，期望 [legacy-a]", got.RuleSources)
	}
}
