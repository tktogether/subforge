package store

// internal/store/profiles.go 的补测 + 契约 §2 的老库迁移测试。
//
// 纪律：不改 store.go 的行为；每个测试用 t.TempDir() 造独立 SQLite 文件（不用 :memory:），
// 因为 Open 会执行 WAL pragma，落文件最稳。

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestListProfilesEmpty 覆盖空库路径：返回非 nil 空切片，且层数组为空。
func TestListProfilesEmpty(t *testing.T) {
	d, ctx := newTestDB(t)
	got, err := d.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("空库 ListProfiles 失败: %v", err)
	}
	if got == nil {
		t.Fatal("空库 ListProfiles 返回 nil，期望非 nil 空切片")
	}
	if len(got) != 0 {
		t.Fatalf("空库 ListProfiles 长度 %d，期望 0", len(got))
	}
}

// TestProfileSourceIDsRoundTrip 覆盖来源列：nil=未限制、空数组=明确不使用。
func TestProfileSourceIDsRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "来源", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if p.AirportIDs != nil || p.SelfNodeIDs != nil {
		t.Fatalf("新方案来源应为 nil，实际 %+v/%+v", p.AirportIDs, p.SelfNodeIDs)
	}
	airports := []int64{2, 1}
	selfNodes := []int64{7}
	if err := d.SetProfileSources(ctx, p.ID, &airports, &selfNodes); err != nil {
		t.Fatalf("SetProfileSources: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.AirportIDs == nil || len(got.AirportIDs) != 2 || got.AirportIDs[0] != 2 || got.AirportIDs[1] != 1 {
		t.Fatalf("airport_ids 往返不符：%+v", got.AirportIDs)
	}
	if got.SelfNodeIDs == nil || len(got.SelfNodeIDs) != 1 || got.SelfNodeIDs[0] != 7 {
		t.Fatalf("self_node_ids 往返不符：%+v", got.SelfNodeIDs)
	}

	empty := []int64{}
	if err := d.SetProfileSources(ctx, p.ID, &empty, nil); err != nil {
		t.Fatalf("SetProfileSources(empty): %v", err)
	}
	got, err = d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.AirportIDs == nil || len(got.AirportIDs) != 0 {
		t.Fatalf("空数组应保持非 nil 空切片，实际 %+v", got.AirportIDs)
	}
	if len(got.SelfNodeIDs) != 1 {
		t.Fatalf("只传机场时不应改自建节点：%+v", got.SelfNodeIDs)
	}
	if err := d.SetProfileSources(ctx, 9999, &empty, nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("不存在方案应返回 sql.ErrNoRows，实际 %v", err)
	}
}

// TestProfileCRUD 覆盖 Add/Get/Update/Delete 往返、全量覆盖、sql.ErrNoRows 与重名冲突。
func TestProfileCRUD(t *testing.T) {
	d, ctx := newTestDB(t)

	p, err := d.AddProfile(ctx, "守候", "描述", "http://x/ini", "p1", true, 5)
	if err != nil {
		t.Fatalf("AddProfile 失败: %v", err)
	}
	if p.ID <= 0 || p.Name != "守候" || p.Description != "描述" || p.Config != "http://x/ini" ||
		p.Pipeline != "p1" || !p.Enabled || p.SortOrder != 5 {
		t.Fatalf("AddProfile 返回字段不符: %+v", p)
	}
	if p.RuleCount != 0 || p.GroupCount != 0 {
		t.Fatalf("空方案计数应为 0/0，实际 %d/%d", p.RuleCount, p.GroupCount)
	}
	if p.RuleLayers != nil || p.GroupLayers != nil {
		t.Fatalf("新方案的层应为 nil（由调用方补默认），实际 %v / %v", p.RuleLayers, p.GroupLayers)
	}

	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile 失败: %v", err)
	}
	if got.Name != "守候" || got.Config != "http://x/ini" {
		t.Fatalf("GetProfile 字段不符: %+v", got)
	}

	// 全量覆盖：enabled=false、sort_order=0（<=0 保持原值）
	if err := d.UpdateProfile(ctx, p.ID, "新名", "新描述", "", "", false, 0); err != nil {
		t.Fatalf("UpdateProfile 失败: %v", err)
	}
	got, err = d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("Update 后 GetProfile 失败: %v", err)
	}
	if got.Name != "新名" || got.Description != "新描述" || got.Enabled ||
		got.Config != "" || got.Pipeline != "" || got.SortOrder != 5 {
		t.Fatalf("UpdateProfile 未按预期覆盖: %+v", got)
	}

	// 不存在：Get/Update/Delete 都返回 sql.ErrNoRows
	if _, err := d.GetProfile(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetProfile(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.UpdateProfile(ctx, 9999, "x", "", "", "", true, 0); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateProfile(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.DeleteProfile(ctx, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteProfile(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}

	// 重名：SQLite UNIQUE 报错
	if _, err := d.AddProfile(ctx, "新名", "", "", "", true, 0); err == nil {
		t.Fatal("重名 AddProfile 应返回错误")
	} else if !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("重名错误文案=%q，期望含 UNIQUE", err.Error())
	}

	// Delete 成功后 Get → sql.ErrNoRows
	if err := d.DeleteProfile(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProfile 失败: %v", err)
	}
	if _, err := d.GetProfile(ctx, p.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("删除后 GetProfile 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestListProfilesOrderAndLayerCounts 覆盖排序、层配置默认值与按层计数。
//
// v2 里规则/分组是全局库，方案的 rule_count/group_count 由「已启用层」统计：
// 规则只数 enabled=true，template 层不贡献任何行。
func TestListProfilesOrderAndLayerCounts(t *testing.T) {
	d, ctx := newTestDB(t)

	a, err := d.AddProfile(ctx, "a", "", "", "", true, 20)
	if err != nil {
		t.Fatalf("AddProfile(a): %v", err)
	}
	b, err := d.AddProfile(ctx, "b", "", "", "", true, 10)
	if err != nil {
		t.Fatalf("AddProfile(b): %v", err)
	}

	// 自定义层：2 条启用 + 1 条停用
	for _, param := range []string{"x1.com", "x2.com"} {
		if _, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN-SUFFIX", RuleParam: param, Enabled: true}); err != nil {
			t.Fatalf("AddRuleWithSource(custom): %v", err)
		}
	}
	if _, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN-SUFFIX", RuleParam: "off.com", Enabled: false}); err != nil {
		t.Fatalf("AddRuleWithSource(disabled): %v", err)
	}
	// 机场层：1 条启用
	if _, err := d.AddRuleWithSource(ctx, Rule{RuleType: "DOMAIN-SUFFIX", RuleParam: "hk.com", Enabled: true, Source: "airport:HK"}); err != nil {
		t.Fatalf("AddRuleWithSource(airport): %v", err)
	}
	// 分组：2 自定义 + 1 机场
	for _, name := range []string{"g1", "g2"} {
		if _, err := d.AddProxyGroupWithSource(ctx, name, "select", "", "", 0, 0, ""); err != nil {
			t.Fatalf("AddProxyGroupWithSource(%s): %v", name, err)
		}
	}
	if _, err := d.AddProxyGroupWithSource(ctx, "airport-group", "select", "", "", 0, 0, "airport:HK"); err != nil {
		t.Fatalf("AddProxyGroupWithSource(airport): %v", err)
	}

	// 默认层 = custom + airport + template：启用规则 3 条（自定义 2 + 机场 1），分组 3 条。
	for _, p := range []*Profile{a, b} {
		got, err := d.GetProfile(ctx, p.ID)
		if err != nil {
			t.Fatalf("GetProfile(%d): %v", p.ID, err)
		}
		if got.RuleCount != 3 || got.GroupCount != 3 {
			t.Fatalf("%s 默认层计数=%d/%d，期望 3/3", got.Name, got.RuleCount, got.GroupCount)
		}
	}

	// 只保留自定义层：规则 2 条（停用的不计），分组 2 条。
	if err := d.SetProfileLayers(ctx, b.ID, []string{"custom"}, []string{"custom"}); err != nil {
		t.Fatalf("SetProfileLayers: %v", err)
	}
	got, err := d.GetProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetProfile(b): %v", err)
	}
	if got.RuleCount != 2 || got.GroupCount != 2 {
		t.Fatalf("b 只看自定义层计数=%d/%d，期望 2/2", got.RuleCount, got.GroupCount)
	}

	// ListProfiles 排序仍是 sort_order, id：b(10) 在 a(20) 前
	list, err := d.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("ListProfiles 顺序=%v，期望 [b, a]", []int64{list[0].ID, list[1].ID})
	}
	if list[0].RuleLayers == nil || len(list[0].RuleLayers) != 1 || list[0].RuleLayers[0] != "custom" {
		t.Fatalf("b 的层未回读: %v", list[0].RuleLayers)
	}
}

// TestSetProfileLayersRoundTrip 覆盖层配置的空数组语义与不存在的方案。
func TestSetProfileLayersRoundTrip(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "层", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	layers := []string{"custom", "airport"}
	if err := d.SetProfileLayers(ctx, p.ID, layers, layers); err != nil {
		t.Fatalf("SetProfileLayers: %v", err)
	}
	got, err := d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.RuleLayers) != 2 || got.RuleLayers[0] != "custom" || got.RuleLayers[1] != "airport" {
		t.Fatalf("RuleLayers 回读=%v，期望 [custom airport]", got.RuleLayers)
	}
	if len(got.GroupLayers) != 2 || got.GroupLayers[1] != "airport" {
		t.Fatalf("GroupLayers 回读=%v，期望 [custom airport]", got.GroupLayers)
	}

	// 空数组 = 空串 = 默认三层：回读为 nil（由调用方补默认）
	if err := d.SetProfileLayers(ctx, p.ID, []string{}, nil); err != nil {
		t.Fatalf("SetProfileLayers(空): %v", err)
	}
	got, err = d.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile(空层): %v", err)
	}
	if got.RuleLayers != nil || got.GroupLayers != nil {
		t.Fatalf("空数组应回读为 nil，实际 %v / %v", got.RuleLayers, got.GroupLayers)
	}
	var raw string
	if err := d.Raw().QueryRowContext(ctx, "SELECT rule_layers FROM profiles WHERE id = ?", p.ID).Scan(&raw); err != nil {
		t.Fatalf("读库内 rule_layers: %v", err)
	}
	if raw != "" {
		t.Fatalf("空数组应落库为空串，实际 %q", raw)
	}

	if err := d.SetProfileLayers(ctx, 9999, layers, layers); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProfileLayers(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestCreateSubTokenWithProfile 覆盖保留的令牌→方案绑定。
func TestCreateSubTokenWithProfile(t *testing.T) {
	d, ctx := newTestDB(t)
	p, err := d.AddProfile(ctx, "令牌方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := d.CreateSubTokenWithProfile(ctx, "tok-p", "名字", "https://sub", "p1", "ini", "a", "b", p.ID); err != nil {
		t.Fatalf("CreateSubTokenWithProfile: %v", err)
	}
	if err := d.CreateSubTokenWithProfile(ctx, "tok-0", "", "", "", "", "", "", 0); err != nil {
		t.Fatalf("CreateSubTokenWithProfile(0): %v", err)
	}
	list, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	byToken := map[string]SubToken{}
	for _, tok := range list {
		byToken[tok.Token] = tok
	}
	if byToken["tok-p"].ProfileID != p.ID || byToken["tok-p"].Name != "名字" ||
		byToken["tok-p"].Target != "https://sub" || byToken["tok-p"].Pipeline != "p1" ||
		byToken["tok-p"].Config != "ini" || byToken["tok-p"].Include != "a" || byToken["tok-p"].Exclude != "b" {
		t.Fatalf("tok-p 字段不符: %+v", byToken["tok-p"])
	}
	if byToken["tok-0"].ProfileID != 0 {
		t.Fatalf("tok-0.ProfileID=%d，期望 0", byToken["tok-0"].ProfileID)
	}
}

// TestColumnExists 覆盖新加的列探测助手。
func TestColumnExists(t *testing.T) {
	d, ctx := newTestDB(t)
	if ok, err := columnExists(d.Raw(), "rules", "source"); err != nil || !ok {
		t.Fatalf("rules.source 应存在，实际 ok=%v err=%v", ok, err)
	}
	if ok, err := columnExists(d.Raw(), "profiles", "rule_layers"); err != nil || !ok {
		t.Fatalf("profiles.rule_layers 应存在，实际 ok=%v err=%v", ok, err)
	}
	if ok, err := columnExists(d.Raw(), "rules", "profile_id"); err != nil || ok {
		t.Fatalf("新库 rules 不应有 profile_id，实际 ok=%v err=%v", ok, err)
	}
	if _, err := columnExists(d.Raw(), "no_such_table", "id"); err != nil {
		t.Fatalf("不存在的表不应报错，实际 %v", err)
	}
	if ctx == nil {
		t.Fatal("ctx 不应为 nil")
	}
}

// TestProfileMethodsClosedDB 覆盖新函数在关闭连接上的错误分支。
func TestProfileMethodsClosedDB(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := d.ListProfiles(ctx); err == nil {
		t.Error("ListProfiles(closed) 期望失败")
	}
	if _, err := d.GetProfile(ctx, 1); err == nil {
		t.Error("GetProfile(closed) 期望失败")
	}
	if _, err := d.AddProfile(ctx, "x", "", "", "", true, 0); err == nil {
		t.Error("AddProfile(closed) 期望失败")
	}
	if err := d.UpdateProfile(ctx, 1, "x", "", "", "", true, 0); err == nil {
		t.Error("UpdateProfile(closed) 期望失败")
	}
	if err := d.DeleteProfile(ctx, 1); err == nil {
		t.Error("DeleteProfile(closed) 期望失败")
	}
	if _, err := d.ListRulesFiltered(ctx, SourceFilter{All: true}); err == nil {
		t.Error("ListRulesFiltered(closed) 期望失败")
	}
	if _, err := d.ListProxyGroupsFiltered(ctx, SourceFilter{All: true}); err == nil {
		t.Error("ListProxyGroupsFiltered(closed) 期望失败")
	}
	if _, err := d.AddRuleWithSource(ctx, Rule{}); err == nil {
		t.Error("AddRuleWithSource(closed) 期望失败")
	}
	if _, err := d.AddProxyGroupWithSource(ctx, "g", "select", "", "", 0, 0, ""); err == nil {
		t.Error("AddProxyGroupWithSource(closed) 期望失败")
	}
	if err := d.SetProfileLayers(ctx, 1, nil, nil); err == nil {
		t.Error("SetProfileLayers(closed) 期望失败")
	}
	if _, err := d.CountRulesByLayers(ctx, nil); err == nil {
		t.Error("CountRulesByLayers(closed) 期望失败")
	}
	if _, err := d.CountGroupsByLayers(ctx, nil); err == nil {
		t.Error("CountGroupsByLayers(closed) 期望失败")
	}
	if _, err := d.ListRulesByLayers(ctx, nil); err == nil {
		t.Error("ListRulesByLayers(closed) 期望失败")
	}
	if _, err := d.ListProxyGroupsByLayers(ctx, nil); err == nil {
		t.Error("ListProxyGroupsByLayers(closed) 期望失败")
	}
	if _, err := d.ListAllRuleSources(ctx); err == nil {
		t.Error("ListAllRuleSources(closed) 期望失败")
	}
	if _, err := d.ListAllGroupSources(ctx); err == nil {
		t.Error("ListAllGroupSources(closed) 期望失败")
	}
	if _, err := d.BatchSetRulesEnabledBySource(ctx, SourceFilter{Source: "x"}, true); err == nil {
		t.Error("BatchSetRulesEnabledBySource(closed) 期望失败")
	}
	if _, err := d.BatchDeleteRulesBySource(ctx, SourceFilter{Source: "x"}); err == nil {
		t.Error("BatchDeleteRulesBySource(closed) 期望失败")
	}
	if err := d.CreateSubTokenWithProfile(ctx, "t", "n", "", "", "", "", "", 0); err == nil {
		t.Error("CreateSubTokenWithProfile(closed) 期望失败")
	}
	if ok, err := d.ProfileExists(ctx, 1); err == nil || ok {
		t.Error("ProfileExists(closed) 期望失败")
	}
}

// --- 契约 §2：老库迁移测试 ---

// legacyV1SchemaDDL 是「v1 老库」的最小集合：rules / proxy_groups 有 profile_id 列、
// 旧的三列唯一索引，并且两个方案各有一条同 (source, origin_key) 的规则
// （模拟跨方案重复导入的真实场景）。
var legacyV1SchemaDDL = []string{
	`CREATE TABLE rules (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_type   TEXT NOT NULL,
		rule_param  TEXT,
		rule_config INTEGER NOT NULL,
		resolve_dns INTEGER DEFAULT 1,
		enabled     INTEGER DEFAULT 1,
		sort_order  INTEGER DEFAULT 0,
		source      TEXT NOT NULL DEFAULT '',
		raw         TEXT NOT NULL DEFAULT '',
		origin_key  TEXT NOT NULL DEFAULT '',
		profile_id  INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE UNIQUE INDEX idx_rules_origin ON rules(profile_id, source, origin_key) WHERE origin_key <> '';`,
	`INSERT INTO rules (rule_type, rule_param, rule_config, source, origin_key, profile_id)
	 VALUES ('DOMAIN-SUFFIX', 'a.com', 1, 'airport:HK', 'a.com', 1);`,
	`INSERT INTO rules (rule_type, rule_param, rule_config, source, origin_key, profile_id)
	 VALUES ('DOMAIN-SUFFIX', 'a.com', 1, 'airport:HK', 'a.com', 2);`,
	`INSERT INTO rules (rule_type, rule_param, rule_config, source, origin_key, profile_id)
	 VALUES ('DOMAIN-SUFFIX', 'hand.com', 1, '', '', 1);`,
	`CREATE TABLE proxy_groups (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		type        TEXT NOT NULL DEFAULT 'select',
		regex       TEXT,
		url         TEXT,
		interval    INTEGER,
		sort_order  INTEGER DEFAULT 0,
		profile_id  INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE profiles (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE sub_tokens (
		token       TEXT PRIMARY KEY,
		name        TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
}

// legacyV018SchemaDDL 是「v0.1.8 老库」：rules / proxy_groups 从来没有 profile_id 列，
// 只有旧的两列唯一索引，并且已有一条带 origin_key 的导入规则。
var legacyV018SchemaDDL = []string{
	`CREATE TABLE rules (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_type   TEXT NOT NULL,
		rule_param  TEXT,
		rule_config INTEGER NOT NULL,
		resolve_dns INTEGER DEFAULT 1,
		enabled     INTEGER DEFAULT 1,
		sort_order  INTEGER DEFAULT 0,
		source      TEXT NOT NULL DEFAULT '',
		raw         TEXT NOT NULL DEFAULT '',
		origin_key  TEXT NOT NULL DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE UNIQUE INDEX idx_rules_origin ON rules(source, origin_key) WHERE origin_key <> '';`,
	`INSERT INTO rules (rule_type, rule_param, rule_config, source, origin_key)
	 VALUES ('DOMAIN-SUFFIX', 'a.com', 1, 'airport:HK', 'a.com');`,
	`CREATE TABLE proxy_groups (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		type        TEXT NOT NULL DEFAULT 'select',
		regex       TEXT,
		url         TEXT,
		interval    INTEGER,
		sort_order  INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE sub_tokens (
		token       TEXT PRIMARY KEY,
		name        TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);`,
}

// openLegacyDB 用老 schema 造一个临时库文件，返回路径。
func openLegacyDB(t *testing.T, ddl []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(legacy): %v", err)
	}
	for _, stmt := range ddl {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("建老库语句失败 %q: %v", stmt, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关老库: %v", err)
	}
	return path
}

// TestLegacyV018SchemaMigratesToV2 覆盖契约 §2 的第 5 条：
// 从未有过 profile_id 的旧库跳过 DROP 分支，直接补齐新列与两列唯一索引。
func TestLegacyV018SchemaMigratesToV2(t *testing.T) {
	path := openLegacyDB(t, legacyV018SchemaDDL)

	d, err := Open(path)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	// ①rules / proxy_groups 不得出现 profile_id
	for _, tbl := range []string{"rules", "proxy_groups"} {
		if cols := tableColumns(t, d, tbl); cols["profile_id"] {
			t.Errorf("%s 不应有 profile_id，实际列：%v", tbl, cols)
		}
	}
	// ②proxy_groups 补 source，profiles 补 rule_layers/group_layers/rule_sources
	if !tableColumns(t, d, "proxy_groups")["source"] {
		t.Error("proxy_groups 缺少 source 列")
	}
	for _, col := range []string{"rule_layers", "group_layers", "rule_sources"} {
		if !tableColumns(t, d, "profiles")[col] {
			t.Errorf("profiles 缺少 %s 列", col)
		}
	}
	// ③索引是两列版本
	want := []string{"source", "origin_key"}
	cols := indexColumns(t, d, "idx_rules_origin")
	if strings.Join(cols, ",") != strings.Join(want, ",") {
		t.Fatalf("idx_rules_origin 列=%v，期望 %v", cols, want)
	}
	// ④老数据保留
	var n int
	if err := d.Raw().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM rules").Scan(&n); err != nil {
		t.Fatalf("读老 rules 计数: %v", err)
	}
	if n != 1 {
		t.Fatalf("老 rules 应保留 1 行，实际 %d", n)
	}
	// ⑤二次 Open 幂等
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 应幂等，实际: %v", err)
	}
	defer func() {
		if err := d2.Close(); err != nil {
			t.Fatalf("Close(d2): %v", err)
		}
	}()
	if cols := indexColumns(t, d2, "idx_rules_origin"); strings.Join(cols, ",") != strings.Join(want, ",") {
		t.Errorf("二次 Open 后索引列=%v，期望 %v", cols, want)
	}
	if tableColumns(t, d2, "rules")["profile_id"] {
		t.Error("二次 Open 不应补回 profile_id")
	}
}

// TestLegacyV1SchemaDropsProfileIDAndDedupes 覆盖契约 §2 的第 3 条：
// v1 老库（带 profile_id 与三列索引、两个方案各一条同 (source, origin_key)）→
// ①DROP 旧索引；②去重保留 MIN(id)；③rules / proxy_groups 的 profile_id 列被删除；
// ④索引重建为 (source, origin_key)；⑤二次 Open 幂等。
func TestLegacyV1SchemaDropsProfileIDAndDedupes(t *testing.T) {
	path := openLegacyDB(t, legacyV1SchemaDDL)

	d, err := Open(path)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	// ①去重：跨方案同 (source, origin_key) 只剩一行（保留 MIN(id)）
	var n int
	if err := d.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM rules WHERE source = 'airport:HK' AND origin_key = 'a.com'").Scan(&n); err != nil {
		t.Fatalf("读去重后计数: %v", err)
	}
	if n != 1 {
		t.Fatalf("跨方案同 (source, origin_key) 应只剩 1 行，实际 %d", n)
	}
	// origin_key='' 的手工规则不受去重影响
	if err := d.Raw().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM rules WHERE origin_key = ''").Scan(&n); err != nil {
		t.Fatalf("读手工规则计数: %v", err)
	}
	if n != 1 {
		t.Fatalf("手工规则（origin_key=''）应保留 1 行，实际 %d", n)
	}

	// ②profile_id 列已从两张表删除，sub_tokens 保留
	for _, tbl := range []string{"rules", "proxy_groups"} {
		if cols := tableColumns(t, d, tbl); cols["profile_id"] {
			t.Errorf("%s 的 profile_id 应被删除，实际列：%v", tbl, cols)
		}
	}
	if !tableColumns(t, d, "sub_tokens")["profile_id"] {
		t.Error("sub_tokens 的 profile_id 应保留")
	}

	// ③索引是两列版本
	want := []string{"source", "origin_key"}
	cols := indexColumns(t, d, "idx_rules_origin")
	if strings.Join(cols, ",") != strings.Join(want, ",") {
		t.Fatalf("idx_rules_origin 列=%v，期望 %v", cols, want)
	}

	// ④唯一约束仍然生效：同 (source, origin_key) 再插应失败
	if _, err := d.AddRuleWithSource(context.Background(),
		Rule{RuleType: "DOMAIN-SUFFIX", RuleParam: "a.com", Source: "airport:HK", OriginKey: "a.com", Enabled: true}); err == nil {
		t.Fatal("迁移后同 (source, origin_key) 的规则应被唯一索引拒绝")
	} else if err.Error() != errDuplicateOriginInSource.Error() {
		t.Fatalf("唯一冲突文案=%q，期望 %q", err.Error(), errDuplicateOriginInSource.Error())
	}

	// ⑤二次 Open 幂等
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 应幂等，实际: %v", err)
	}
	defer func() {
		if err := d2.Close(); err != nil {
			t.Fatalf("Close(d2): %v", err)
		}
	}()
	if cols := indexColumns(t, d2, "idx_rules_origin"); strings.Join(cols, ",") != strings.Join(want, ",") {
		t.Errorf("二次 Open 后索引列=%v，期望 %v", cols, want)
	}
	if tableColumns(t, d2, "rules")["profile_id"] {
		t.Error("二次 Open 不应补回 profile_id")
	}
}

// indexColumns 返回指定索引的列名（按 seqno 升序），即 PRAGMA index_info 的 name 列。
func indexColumns(t *testing.T, d *DB, index string) []string {
	t.Helper()
	rows, err := d.Raw().Query("PRAGMA index_info(" + index + ")")
	if err != nil {
		t.Fatalf("PRAGMA index_info(%s): %v", index, err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var (
			seqno int
			cid   int
			name  string
		)
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			t.Fatalf("scan index_info(%s): %v", index, err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows index_info(%s): %v", index, err)
	}
	return out
}
