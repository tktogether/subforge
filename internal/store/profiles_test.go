package store

// internal/store/profiles.go 的补测 + 契约 §1.3 的老库迁移测试。
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

// TestListProfilesEmpty 覆盖空库路径：返回非 nil 空切片。
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

// TestListProfilesOrderAndCounts 覆盖 ORDER BY sort_order, id 与规则/分组计数子查询。
func TestListProfilesOrderAndCounts(t *testing.T) {
	d, ctx := newTestDB(t)

	a, err := d.AddProfile(ctx, "a", "", "", "", true, 20)
	if err != nil {
		t.Fatalf("AddProfile(a): %v", err)
	}
	b, err := d.AddProfile(ctx, "b", "", "", "", true, 10)
	if err != nil {
		t.Fatalf("AddProfile(b): %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := d.AddRuleWithProfile(ctx, a.ID, "DOMAIN-SUFFIX", "x.com", 0, true, true); err != nil {
			t.Fatalf("AddRuleWithProfile(a): %v", err)
		}
	}
	if _, err := d.AddRuleWithProfile(ctx, b.ID, "DOMAIN-SUFFIX", "y.com", 0, true, true); err != nil {
		t.Fatalf("AddRuleWithProfile(b): %v", err)
	}
	if _, err := d.AddProxyGroupWithProfile(ctx, a.ID, "g", "select", ".", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroupWithProfile(a): %v", err)
	}

	list, err := d.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListProfiles 长度 %d，期望 2", len(list))
	}
	if list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("ListProfiles 排序错误: [%s(%d) %s(%d)]，期望按 sort_order 升序",
			list[0].Name, list[0].SortOrder, list[1].Name, list[1].SortOrder)
	}
	if list[1].RuleCount != 2 || list[1].GroupCount != 1 {
		t.Fatalf("a 的计数应为 2 规则/1 分组，实际 %d/%d", list[1].RuleCount, list[1].GroupCount)
	}
	if list[0].RuleCount != 1 || list[0].GroupCount != 0 {
		t.Fatalf("b 的计数应为 1 规则/0 分组，实际 %d/%d", list[0].RuleCount, list[0].GroupCount)
	}
}

// TestRulesGroupsByProfile 覆盖 Add/List 的按方案隔离：profileID=0 走旧全局池。
func TestRulesGroupsByProfile(t *testing.T) {
	d, ctx := newTestDB(t)

	// 旧路径：profile_id=0（未分配）
	legacy, err := d.AddRuleWithProfile(ctx, 0, "DOMAIN", "legacy.com", 0, false, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile(0): %v", err)
	}
	if legacy.ProfileID != 0 {
		t.Fatalf("profileID=0 的规则 ProfileID=%d，期望 0", legacy.ProfileID)
	}

	p1, err := d.AddProfile(ctx, "p1", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile(p1): %v", err)
	}
	p2, err := d.AddProfile(ctx, "p2", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile(p2): %v", err)
	}
	r1, err := d.AddRuleWithProfile(ctx, p1.ID, "DOMAIN-SUFFIX", "a.com", 7, true, false)
	if err != nil {
		t.Fatalf("AddRuleWithProfile(p1): %v", err)
	}
	if r1.ProfileID != p1.ID || r1.RuleConfig != 7 || !r1.ResolveDNS || r1.Enabled {
		t.Fatalf("AddRuleWithProfile 返回字段不符: %+v", r1)
	}
	if _, err := d.AddRuleWithProfile(ctx, p2.ID, "DOMAIN-SUFFIX", "b.com", 0, false, true); err != nil {
		t.Fatalf("AddRuleWithProfile(p2): %v", err)
	}

	g1, err := d.AddProxyGroupWithProfile(ctx, p1.ID, "g1", "select", ".", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroupWithProfile(p1): %v", err)
	}
	if g1.ProfileID != p1.ID || g1.Type != "select" || g1.Regex != "." {
		t.Fatalf("AddProxyGroupWithProfile 返回字段不符: %+v", g1)
	}
	if _, err := d.AddProxyGroupWithProfile(ctx, 0, "g0", "url-test", "", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroupWithProfile(0): %v", err)
	}

	rules1, err := d.ListRulesByProfile(ctx, p1.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile(p1): %v", err)
	}
	if len(rules1) != 1 || rules1[0].ID != r1.ID || rules1[0].ProfileID != p1.ID {
		t.Fatalf("ListRulesByProfile(p1) = %+v，期望只有 r1", rules1)
	}
	rules0, err := d.ListRulesByProfile(ctx, 0)
	if err != nil {
		t.Fatalf("ListRulesByProfile(0): %v", err)
	}
	if len(rules0) != 1 || rules0[0].ID != legacy.ID {
		t.Fatalf("ListRulesByProfile(0) = %+v，期望只有旧规则", rules0)
	}

	groups1, err := d.ListProxyGroupsByProfile(ctx, p1.ID)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile(p1): %v", err)
	}
	if len(groups1) != 1 || groups1[0].ID != g1.ID {
		t.Fatalf("ListProxyGroupsByProfile(p1) = %+v，期望只有 g1", groups1)
	}
	groups0, err := d.ListProxyGroupsByProfile(ctx, 0)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile(0): %v", err)
	}
	if len(groups0) != 1 {
		t.Fatalf("ListProxyGroupsByProfile(0) 长度 %d，期望 1", len(groups0))
	}
}

// TestSetRuleAndGroupProfile 覆盖 Set 的命中与 sql.ErrNoRows 分支。
func TestSetRuleAndGroupProfile(t *testing.T) {
	d, ctx := newTestDB(t)

	p, err := d.AddProfile(ctx, "p", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	r, err := d.AddRuleWithProfile(ctx, 0, "DOMAIN", "a.com", 0, true, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile: %v", err)
	}
	g, err := d.AddProxyGroupWithProfile(ctx, 0, "g", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroupWithProfile: %v", err)
	}

	if err := d.SetRuleProfile(ctx, r.ID, p.ID); err != nil {
		t.Fatalf("SetRuleProfile 失败: %v", err)
	}
	got, err := d.GetRule(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.ProfileID != p.ID {
		t.Fatalf("SetRuleProfile 未生效，ProfileID=%d，期望 %d", got.ProfileID, p.ID)
	}

	if err := d.SetProxyGroupProfile(ctx, g.ID, p.ID); err != nil {
		t.Fatalf("SetProxyGroupProfile 失败: %v", err)
	}
	g2, err := d.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if g2.ProfileID != p.ID {
		t.Fatalf("SetProxyGroupProfile 未生效，ProfileID=%d，期望 %d", g2.ProfileID, p.ID)
	}

	if err := d.SetRuleProfile(ctx, 9999, p.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetRuleProfile(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
	if err := d.SetProxyGroupProfile(ctx, 9999, p.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProxyGroupProfile(9999) 错误=%v，期望 sql.ErrNoRows", err)
	}
}

// TestMoveRulesToProfile 覆盖空 ids、批量移动与唯一索引冲突文案。
func TestMoveRulesToProfile(t *testing.T) {
	d, ctx := newTestDB(t)

	p1, err := d.AddProfile(ctx, "p1", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile(p1): %v", err)
	}
	p2, err := d.AddProfile(ctx, "p2", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile(p2): %v", err)
	}

	// 空 ids：直接 0, nil（不开事务）
	n, err := d.MoveRulesToProfile(ctx, nil, p2.ID)
	if err != nil || n != 0 {
		t.Fatalf("MoveRulesToProfile(nil) = %d, %v，期望 0, nil", n, err)
	}

	// p1 里已有一条同来源同去重键的规则，p2 里再移一条同 key 规则 → 撞唯一索引
	r1, err := d.AddRuleWithProfile(ctx, p1.ID, "DOMAIN-SUFFIX", "dup.com", 0, true, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile(p1): %v", err)
	}
	if _, err := d.Raw().ExecContext(ctx,
		"UPDATE rules SET source = 'airport:HK', origin_key = 'dup.com' WHERE id = ?", r1.ID); err != nil {
		t.Fatalf("设置 r1 溯源列: %v", err)
	}
	r2, err := d.AddRuleWithProfile(ctx, 0, "DOMAIN-SUFFIX", "dup.com", 0, true, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile(0): %v", err)
	}
	if _, err := d.Raw().ExecContext(ctx,
		"UPDATE rules SET source = 'airport:HK', origin_key = 'dup.com' WHERE id = ?", r2.ID); err != nil {
		t.Fatalf("设置 r2 溯源列: %v", err)
	}

	// 正常批量移动
	r3, err := d.AddRuleWithProfile(ctx, 0, "DOMAIN", "move.com", 0, true, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile(r3): %v", err)
	}
	moved, err := d.MoveRulesToProfile(ctx, []int64{r3.ID, 4242}, p2.ID)
	if err != nil {
		t.Fatalf("MoveRulesToProfile 失败: %v", err)
	}
	if moved != 1 {
		t.Fatalf("MoveRulesToProfile 返回 %d，期望 1（不存在的 id 不计）", moved)
	}
	got, err := d.GetRule(ctx, r3.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.ProfileID != p2.ID {
		t.Fatalf("r3.ProfileID=%d，期望 %d", got.ProfileID, p2.ID)
	}

	// 撞唯一索引：返回固定文案，且不吞成 nil
	_, err = d.MoveRulesToProfile(ctx, []int64{r2.ID}, p1.ID)
	if err == nil {
		t.Fatal("撞唯一索引的 MoveRulesToProfile 应返回错误")
	}
	if err.Error() != "目标方案里已存在同来源同去重键的规则，无法移动" {
		t.Fatalf("冲突文案=%q", err.Error())
	}
	if !strings.Contains(err.Error(), "目标方案") {
		t.Fatalf("冲突文案缺少「目标方案」字样: %q", err.Error())
	}
}

// TestMoveRulesBySourceToProfile 覆盖空 source 与按来源批量移动。
func TestMoveRulesBySourceToProfile(t *testing.T) {
	d, ctx := newTestDB(t)

	p, err := d.AddProfile(ctx, "p", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	// 空 source：直接 0, nil，绝不能把全表搬到方案里
	n, err := d.MoveRulesBySourceToProfile(ctx, "", p.ID)
	if err != nil || n != 0 {
		t.Fatalf("MoveRulesBySourceToProfile(empty) = %d, %v，期望 0, nil", n, err)
	}

	for _, param := range []string{"a.com", "b.com"} {
		r, err := d.AddRuleWithProfile(ctx, 0, "DOMAIN-SUFFIX", param, 0, true, true)
		if err != nil {
			t.Fatalf("AddRuleWithProfile: %v", err)
		}
		if _, err := d.Raw().ExecContext(ctx,
			"UPDATE rules SET source = 'airport:HK', origin_key = ? WHERE id = ?", param, r.ID); err != nil {
			t.Fatalf("设置溯源列: %v", err)
		}
	}
	moved, err := d.MoveRulesBySourceToProfile(ctx, "airport:HK", p.ID)
	if err != nil {
		t.Fatalf("MoveRulesBySourceToProfile 失败: %v", err)
	}
	if moved != 2 {
		t.Fatalf("MoveRulesBySourceToProfile 返回 %d，期望 2", moved)
	}
	left, err := d.ListRulesByProfile(ctx, 0)
	if err != nil {
		t.Fatalf("ListRulesByProfile(0): %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("移动后全局池应清空，实际剩 %d 条", len(left))
	}
	inP, err := d.ListRulesByProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile(p): %v", err)
	}
	if len(inP) != 2 {
		t.Fatalf("方案内应有 2 条，实际 %d", len(inP))
	}
}

// TestMoveGroupsToProfile 覆盖空 ids 与批量移动（proxy_groups 无唯一索引）。
func TestMoveGroupsToProfile(t *testing.T) {
	d, ctx := newTestDB(t)

	p, err := d.AddProfile(ctx, "p", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	n, err := d.MoveGroupsToProfile(ctx, nil, p.ID)
	if err != nil || n != 0 {
		t.Fatalf("MoveGroupsToProfile(nil) = %d, %v，期望 0, nil", n, err)
	}

	g1, err := d.AddProxyGroupWithProfile(ctx, 0, "g1", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroupWithProfile(g1): %v", err)
	}
	moved, err := d.MoveGroupsToProfile(ctx, []int64{g1.ID}, p.ID)
	if err != nil {
		t.Fatalf("MoveGroupsToProfile 失败: %v", err)
	}
	if moved != 1 {
		t.Fatalf("MoveGroupsToProfile 返回 %d，期望 1", moved)
	}
	got, err := d.GetProxyGroup(ctx, g1.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if got.ProfileID != p.ID {
		t.Fatalf("g1.ProfileID=%d，期望 %d", got.ProfileID, p.ID)
	}
}

// TestCreateSubTokenWithProfile 覆盖令牌的方案绑定与 ListSubTokens/UpdateSubToken 的读写。
func TestCreateSubTokenWithProfile(t *testing.T) {
	d, ctx := newTestDB(t)

	p, err := d.AddProfile(ctx, "p", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := d.CreateSubTokenWithProfile(ctx, "tok-p", "绑方案", "", "", "", "", "", p.ID); err != nil {
		t.Fatalf("CreateSubTokenWithProfile 失败: %v", err)
	}
	if err := d.CreateSubTokenWithProfile(ctx, "tok-0", "未绑", "", "", "", "", "", 0); err != nil {
		t.Fatalf("CreateSubTokenWithProfile(0) 失败: %v", err)
	}

	list, err := d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListSubTokens 长度 %d，期望 2", len(list))
	}
	byToken := map[string]SubToken{}
	for _, tok := range list {
		byToken[tok.Token] = tok
	}
	if byToken["tok-p"].ProfileID != p.ID {
		t.Fatalf("tok-p.ProfileID=%d，期望 %d", byToken["tok-p"].ProfileID, p.ID)
	}
	if byToken["tok-0"].ProfileID != 0 {
		t.Fatalf("tok-0.ProfileID=%d，期望 0", byToken["tok-0"].ProfileID)
	}

	// UpdateSubToken 的 ProfileID 指针路径（0 合法 = 解绑）
	zero := int64(0)
	if err := d.UpdateSubToken(ctx, "tok-p", SubTokenUpdate{ProfileID: &zero}); err != nil {
		t.Fatalf("UpdateSubToken(ProfileID=0) 失败: %v", err)
	}
	if _, err := d.ListSubTokens(ctx); err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	list, err = d.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	for _, tok := range list {
		if tok.Token == "tok-p" && tok.ProfileID != 0 {
			t.Fatalf("解绑后 tok-p.ProfileID=%d，期望 0", tok.ProfileID)
		}
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
	if _, err := d.ListRulesByProfile(ctx, 0); err == nil {
		t.Error("ListRulesByProfile(closed) 期望失败")
	}
	if _, err := d.ListProxyGroupsByProfile(ctx, 0); err == nil {
		t.Error("ListProxyGroupsByProfile(closed) 期望失败")
	}
	if _, err := d.AddRuleWithProfile(ctx, 0, "T", "P", 0, true, true); err == nil {
		t.Error("AddRuleWithProfile(closed) 期望失败")
	}
	if _, err := d.AddProxyGroupWithProfile(ctx, 0, "g", "select", "", "", 0, 0); err == nil {
		t.Error("AddProxyGroupWithProfile(closed) 期望失败")
	}
	if err := d.SetRuleProfile(ctx, 1, 0); err == nil {
		t.Error("SetRuleProfile(closed) 期望失败")
	}
	if err := d.SetProxyGroupProfile(ctx, 1, 0); err == nil {
		t.Error("SetProxyGroupProfile(closed) 期望失败")
	}
	if _, err := d.MoveRulesToProfile(ctx, []int64{1}, 0); err == nil {
		t.Error("MoveRulesToProfile(closed) 期望失败")
	}
	if _, err := d.MoveRulesBySourceToProfile(ctx, "airport:HK", 0); err == nil {
		t.Error("MoveRulesBySourceToProfile(closed) 期望失败")
	}
	if _, err := d.MoveGroupsToProfile(ctx, []int64{1}, 0); err == nil {
		t.Error("MoveGroupsToProfile(closed) 期望失败")
	}
	if err := d.CreateSubTokenWithProfile(ctx, "t", "n", "", "", "", "", "", 0); err == nil {
		t.Error("CreateSubTokenWithProfile(closed) 期望失败")
	}
	if ok, err := d.ProfileExists(ctx, 1); err == nil || ok {
		t.Error("ProfileExists(closed) 期望失败")
	}
}

// --- 契约 §1.3：老库迁移测试 ---

// legacySchemaDDL 是「老 schema」的最小集合：rules 无 profile_id、旧的两列唯一索引，
// 并且先插一条带 origin_key 的规则（模拟真实老库已导入过的数据）。
var legacySchemaDDL = []string{
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

// TestLegacySchemaMigrationAddsProfileIDAndRebuildsIndex 是契约 §1.3 的迁移测试：
// 老库（rules 无 profile_id、旧两列索引、已有 1 条带 origin_key 的规则）→ Open →
// ①迁移成功；②rules 含 profile_id；③idx_rules_origin 三列依次为 profile_id/source/origin_key；
// ④再 Open 一次仍成功（幂等）；⑤老数据保留且落在未分配方案。
func TestLegacySchemaMigrationAddsProfileIDAndRebuildsIndex(t *testing.T) {
	path := openLegacyDB(t, legacySchemaDDL)

	d, err := Open(path) // 等价 subforge.NewManager(path)：内部就是 store.Open → migrate
	if err != nil {
		t.Fatalf("①迁移失败: %v", err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	// ②rules / proxy_groups / sub_tokens 都应有 profile_id
	for _, tbl := range []string{"rules", "proxy_groups", "sub_tokens"} {
		cols := tableColumns(t, d, tbl)
		if !cols["profile_id"] {
			t.Errorf("②%s 缺少 profile_id，实际列：%v", tbl, cols)
		}
	}
	if !tableColumns(t, d, "profiles")["id"] {
		t.Error("②新库缺少 profiles 表")
	}

	// ③索引三列顺序
	idxCols := indexColumns(t, d, "idx_rules_origin")
	want := []string{"profile_id", "source", "origin_key"}
	if len(idxCols) != len(want) {
		t.Fatalf("③idx_rules_origin 列数 %d，期望 %d（%v）", len(idxCols), len(want), idxCols)
	}
	for i, c := range want {
		if idxCols[i] != c {
			t.Errorf("③idx_rules_origin 第 %d 列=%q，期望 %q（全列 %v）", i, idxCols[i], c, idxCols)
		}
	}

	// ⑤老数据保留，且归属未分配（profile_id=0）
	var n int
	if err := d.Raw().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM rules").Scan(&n); err != nil {
		t.Fatalf("读老 rules 计数: %v", err)
	}
	if n != 1 {
		t.Fatalf("⑤老 rules 应保留 1 行，实际 %d", n)
	}
	var profileID int64
	if err := d.Raw().QueryRowContext(context.Background(), "SELECT profile_id FROM rules WHERE origin_key = 'a.com'").
		Scan(&profileID); err != nil {
		t.Fatalf("读老 rules.profile_id: %v", err)
	}
	if profileID != 0 {
		t.Errorf("⑤老数据 profile_id=%d，期望 0", profileID)
	}

	// ④二次 Open 幂等
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("④二次 Open 应幂等，实际: %v", err)
	}
	defer func() {
		if err := d2.Close(); err != nil {
			t.Fatalf("Close(d2): %v", err)
		}
	}()
	if cols := indexColumns(t, d2, "idx_rules_origin"); strings.Join(cols, ",") != strings.Join(want, ",") {
		t.Errorf("④二次 Open 后索引列=%v，期望 %v", cols, want)
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
