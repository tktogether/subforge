package subforge

// Profile（订阅方案）Manager 包装层的单测。
//
// 覆盖 internal/subforge/profiles.go 的全部导出包装函数：happy path、
// 「不存在」错误翻译、按方案过滤、移动后计数，以及令牌绑定方案的回读。
// 基座复用 subforge_test.go 的 newTestManager(t)。

import (
	"context"
	"strings"
	"testing"
)

// canceledCtx 返回一个已取消的 context：用来触发各包装的「非 ErrNoRows 的通用错误」分支。
func canceledCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TestManagerListProfiles 覆盖 ListProfiles：空库非 nil、按 sort_order,id 升序返回。
func TestManagerListProfiles(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	got, err := m.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if got == nil {
		t.Fatal("ListProfiles 应返回非 nil 空切片，实际 nil")
	}
	if len(got) != 0 {
		t.Fatalf("空库应有 0 个方案，实际 %d", len(got))
	}

	p1, err := m.AddProfile(ctx, "守候规则", "desc-1", "ini://a", "pipe-a", true, 1)
	if err != nil {
		t.Fatalf("AddProfile p1: %v", err)
	}
	p2, err := m.AddProfile(ctx, "机场规则", "", "", "", false, 2)
	if err != nil {
		t.Fatalf("AddProfile p2: %v", err)
	}
	if p1.ID <= 0 || p2.ID <= 0 || p1.ID == p2.ID {
		t.Fatalf("AddProfile 返回的 id 异常: %+v / %+v", p1, p2)
	}

	got, err = m.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles 二次: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应有 2 个方案，实际 %d", len(got))
	}
	if got[0].ID != p1.ID || got[1].ID != p2.ID {
		t.Errorf("应按 sort_order,id 升序，实际 %+v", got)
	}
	if got[0].Name != "守候规则" || got[1].Name != "机场规则" {
		t.Errorf("方案名顺序错: %+v", got)
	}
	if !got[0].Enabled || got[1].Enabled {
		t.Errorf("enabled 未按入参落库: %+v", got)
	}
	if got[0].Config != "ini://a" || got[0].Pipeline != "pipe-a" {
		t.Errorf("config/pipeline 未落库: %+v", got[0])
	}
	if got[0].RuleCount != 0 || got[0].GroupCount != 0 {
		t.Errorf("空方案计数应为 0: %+v", got[0])
	}
}

// TestManagerGetProfile 覆盖 GetProfile 的三条分支：成功、不存在（翻译文案）、通用错误。
func TestManagerGetProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	p, err := m.AddProfile(ctx, "方案A", "d", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.ID != p.ID || got.Name != "方案A" {
		t.Fatalf("GetProfile 返回不符: %+v", got)
	}

	if _, err := m.GetProfile(ctx, 999999); err == nil {
		t.Fatal("GetProfile(不存在) 期望报错")
	} else if !strings.Contains(err.Error(), "订阅方案 999999 不存在") {
		t.Errorf("不存在文案不符: %v", err)
	}

	if _, err := m.GetProfile(canceledCtx(t), p.ID); err == nil {
		t.Fatal("GetProfile(canceled) 期望通用错误")
	}
}

// TestManagerProfileCRUD 覆盖 AddProfile / UpdateProfile / DeleteProfile 及两者的不存在分支。
func TestManagerProfileCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	p, err := m.AddProfile(ctx, "旧名", "旧描述", "ini://old", "pipe-old", true, 1)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	if err := m.UpdateProfile(ctx, p.ID, "新名", "新描述", "ini://new", "pipe-new", false, 5); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	got, err := m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Name != "新名" || got.Description != "新描述" || got.Config != "ini://new" || got.Pipeline != "pipe-new" {
		t.Errorf("UpdateProfile 未全量覆盖: %+v", got)
	}
	if got.Enabled {
		t.Error("UpdateProfile 应写入 enabled=false")
	}
	if got.SortOrder != 5 {
		t.Errorf("UpdateProfile 未写入 sort_order=5，实际 %d", got.SortOrder)
	}

	if err := m.DeleteProfile(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if _, err := m.GetProfile(ctx, p.ID); err == nil {
		t.Error("删除后 GetProfile 应报不存在")
	}

	if err := m.UpdateProfile(ctx, 999999, "x", "", "", "", true, 0); err == nil {
		t.Error("UpdateProfile(不存在) 期望报错")
	}
	if err := m.DeleteProfile(ctx, 999999); err == nil {
		t.Error("DeleteProfile(不存在) 期望报错")
	}
	if err := m.UpdateProfile(canceledCtx(t), p.ID, "x", "", "", "", true, 0); err == nil {
		t.Error("UpdateProfile(canceled) 期望通用错误")
	}
	if err := m.DeleteProfile(canceledCtx(t), p.ID); err == nil {
		t.Error("DeleteProfile(canceled) 期望通用错误")
	}
}

// TestManagerRulesGroupsByProfile 覆盖 ListRulesByProfile / ListProxyGroupsByProfile：按方案过滤与计数。
func TestManagerRulesGroupsByProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	a, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	b, err := m.AddProfile(ctx, "方案B", "", "", "", true, 2)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}

	// profile_id=0 的旧全局池也走同一条 SQL（WHERE profile_id = ?）。
	if _, err := m.AddRule(ctx, "DOMAIN", "legacy.test", 0, false, true); err != nil {
		t.Fatalf("AddRule 未分配: %v", err)
	}
	ra1, err := m.AddRuleWithProfile(ctx, a.ID, "DOMAIN", "a1.test", 0, false, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile A: %v", err)
	}
	if _, err := m.AddRuleWithProfile(ctx, a.ID, "DOMAIN", "a2.test", 0, false, true); err != nil {
		t.Fatalf("AddRuleWithProfile A 2: %v", err)
	}
	if _, err := m.AddRuleWithProfile(ctx, b.ID, "DOMAIN", "b1.test", 0, false, true); err != nil {
		t.Fatalf("AddRuleWithProfile B: %v", err)
	}

	ga, err := m.AddProxyGroupWithProfile(ctx, a.ID, "GA", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroupWithProfile A: %v", err)
	}
	if _, err := m.AddProxyGroupWithProfile(ctx, b.ID, "GB", "select", "", "", 0, 0); err != nil {
		t.Fatalf("AddProxyGroupWithProfile B: %v", err)
	}

	rulesA, err := m.ListRulesByProfile(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile A: %v", err)
	}
	if len(rulesA) != 2 {
		t.Fatalf("方案A 应有 2 条规则，实际 %d", len(rulesA))
	}
	for _, r := range rulesA {
		if r.ProfileID != a.ID {
			t.Errorf("规则 %d 的 ProfileID = %d, want %d", r.ID, r.ProfileID, a.ID)
		}
	}
	rulesB, err := m.ListRulesByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile B: %v", err)
	}
	if len(rulesB) != 1 || rulesB[0].RuleParam != "b1.test" {
		t.Fatalf("方案B 规则不符: %+v", rulesB)
	}
	legacy, err := m.ListRulesByProfile(ctx, 0)
	if err != nil {
		t.Fatalf("ListRulesByProfile 0: %v", err)
	}
	if len(legacy) != 1 || legacy[0].RuleParam != "legacy.test" {
		t.Fatalf("profile_id=0 应保留旧全局规则，实际 %+v", legacy)
	}

	groupsA, err := m.ListProxyGroupsByProfile(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile A: %v", err)
	}
	if len(groupsA) != 1 || groupsA[0].Name != "GA" {
		t.Fatalf("方案A 分组不符: %+v", groupsA)
	}
	if groupsA[0].ProfileID != a.ID {
		t.Errorf("分组 %d 的 ProfileID = %d, want %d", groupsA[0].ID, groupsA[0].ProfileID, a.ID)
	}
	groupsB, err := m.ListProxyGroupsByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile B: %v", err)
	}
	if len(groupsB) != 1 || groupsB[0].Name != "GB" {
		t.Fatalf("方案B 分组不符: %+v", groupsB)
	}

	// 计数列应随方案内容变化。
	pa, err := m.GetProfile(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetProfile A: %v", err)
	}
	if pa.RuleCount != 2 || pa.GroupCount != 1 {
		t.Errorf("方案A 计数 = %d/%d, want 2/1", pa.RuleCount, pa.GroupCount)
	}
	_ = ra1
	_ = ga
}

// TestManagerAddRuleWithProfile 覆盖 AddRuleWithProfile：回读 ProfileID 且进入对应方案列表。
func TestManagerAddRuleWithProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	p, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	r, err := m.AddRuleWithProfile(ctx, p.ID, "DOMAIN-SUFFIX", "only.test", 0, true, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile: %v", err)
	}
	if r.ProfileID != p.ID {
		t.Fatalf("新规则 ProfileID = %d, want %d", r.ProfileID, p.ID)
	}
	if !r.ResolveDNS || !r.Enabled {
		t.Errorf("resolve_dns/enabled 未按入参落库: %+v", r)
	}
	got, err := m.GetRule(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.ProfileID != p.ID {
		t.Errorf("GetRule 回读 ProfileID = %d, want %d", got.ProfileID, p.ID)
	}
	if _, err := m.AddRuleWithProfile(canceledCtx(t), p.ID, "DOMAIN", "x.test", 0, false, false); err == nil {
		t.Error("AddRuleWithProfile(canceled) 期望通用错误")
	}
}

// TestManagerAddProxyGroupWithProfile 覆盖 AddProxyGroupWithProfile：回读 ProfileID。
func TestManagerAddProxyGroupWithProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	p, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	g, err := m.AddProxyGroupWithProfile(ctx, p.ID, "G1", "url-test", "", "https://example.com", 30, 0)
	if err != nil {
		t.Fatalf("AddProxyGroupWithProfile: %v", err)
	}
	if g.ProfileID != p.ID {
		t.Fatalf("新分组 ProfileID = %d, want %d", g.ProfileID, p.ID)
	}
	if g.Interval != 30 || g.Type != "url-test" {
		t.Errorf("分组字段未落库: %+v", g)
	}
	got, err := m.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if got.ProfileID != p.ID {
		t.Errorf("GetProxyGroup 回读 ProfileID = %d, want %d", got.ProfileID, p.ID)
	}
	if _, err := m.AddProxyGroupWithProfile(canceledCtx(t), p.ID, "G2", "select", "", "", 0, 0); err == nil {
		t.Error("AddProxyGroupWithProfile(canceled) 期望通用错误")
	}
}

// TestManagerSetRuleProfile 覆盖 SetRuleProfile：移动成功、不存在翻译、通用错误。
func TestManagerSetRuleProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	b, err := m.AddProfile(ctx, "方案B", "", "", "", true, 2)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}
	r, err := m.AddRuleWithProfile(ctx, a.ID, "DOMAIN", "move.test", 0, false, true)
	if err != nil {
		t.Fatalf("AddRuleWithProfile: %v", err)
	}

	if err := m.SetRuleProfile(ctx, r.ID, b.ID); err != nil {
		t.Fatalf("SetRuleProfile: %v", err)
	}
	got, err := m.GetRule(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.ProfileID != b.ID {
		t.Errorf("SetRuleProfile 后 ProfileID = %d, want %d", got.ProfileID, b.ID)
	}
	inA, err := m.ListRulesByProfile(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile A: %v", err)
	}
	if len(inA) != 0 {
		t.Errorf("方案A 不应再含该规则: %+v", inA)
	}
	inB, err := m.ListRulesByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile B: %v", err)
	}
	if len(inB) != 1 {
		t.Errorf("方案B 应含该规则，实际 %d", len(inB))
	}

	err = m.SetRuleProfile(ctx, 999999, b.ID)
	if err == nil || !strings.Contains(err.Error(), "规则 999999 不存在") {
		t.Errorf("SetRuleProfile(不存在) 文案不符: %v", err)
	}
	if err := m.SetRuleProfile(canceledCtx(t), r.ID, b.ID); err == nil {
		t.Error("SetRuleProfile(canceled) 期望通用错误")
	}
}

// TestManagerSetProxyGroupProfile 覆盖 SetProxyGroupProfile：移动成功、不存在翻译、通用错误。
func TestManagerSetProxyGroupProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	b, err := m.AddProfile(ctx, "方案B", "", "", "", true, 2)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}
	g, err := m.AddProxyGroupWithProfile(ctx, a.ID, "GA", "select", "", "", 0, 0)
	if err != nil {
		t.Fatalf("AddProxyGroupWithProfile: %v", err)
	}

	if err := m.SetProxyGroupProfile(ctx, g.ID, b.ID); err != nil {
		t.Fatalf("SetProxyGroupProfile: %v", err)
	}
	got, err := m.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if got.ProfileID != b.ID {
		t.Errorf("SetProxyGroupProfile 后 ProfileID = %d, want %d", got.ProfileID, b.ID)
	}
	inB, err := m.ListProxyGroupsByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile B: %v", err)
	}
	if len(inB) != 1 {
		t.Errorf("方案B 应含该分组，实际 %d", len(inB))
	}

	err = m.SetProxyGroupProfile(ctx, 999999, b.ID)
	if err == nil || !strings.Contains(err.Error(), "代理分组 999999 不存在") {
		t.Errorf("SetProxyGroupProfile(不存在) 文案不符: %v", err)
	}
	if err := m.SetProxyGroupProfile(canceledCtx(t), g.ID, b.ID); err == nil {
		t.Error("SetProxyGroupProfile(canceled) 期望通用错误")
	}
}

// TestManagerMoveRulesToProfile 覆盖 MoveRulesToProfile：返回移动行数、移动后按方案计数、空 ids。
func TestManagerMoveRulesToProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	b, err := m.AddProfile(ctx, "方案B", "", "", "", true, 2)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}

	var ids []int64
	for _, p := range []string{"m1.test", "m2.test", "m3.test"} {
		r, err := m.AddRuleWithProfile(ctx, a.ID, "DOMAIN", p, 0, false, true)
		if err != nil {
			t.Fatalf("AddRuleWithProfile %s: %v", p, err)
		}
		ids = append(ids, r.ID)
	}

	// 空 ids 直接返回 0, nil。
	if n, err := m.MoveRulesToProfile(ctx, nil, b.ID); err != nil || n != 0 {
		t.Errorf("空 ids 应返回 0,nil，实际 %d,%v", n, err)
	}

	n, err := m.MoveRulesToProfile(ctx, ids[:2], b.ID)
	if err != nil {
		t.Fatalf("MoveRulesToProfile: %v", err)
	}
	if n != 2 {
		t.Fatalf("MoveRulesToProfile 返回 %d, want 2", n)
	}
	inB, err := m.ListRulesByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile B: %v", err)
	}
	if len(inB) != 2 {
		t.Errorf("方案B 应有 2 条规则，实际 %d", len(inB))
	}
	inA, err := m.ListRulesByProfile(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile A: %v", err)
	}
	if len(inA) != 1 || inA[0].RuleParam != "m3.test" {
		t.Errorf("方案A 应只剩 m3.test，实际 %+v", inA)
	}
	if _, err := m.MoveRulesToProfile(canceledCtx(t), []int64{ids[2]}, b.ID); err == nil {
		t.Error("MoveRulesToProfile(canceled) 期望通用错误")
	}
}

// TestManagerMoveRulesBySourceToProfile 覆盖 MoveRulesBySourceToProfile：按来源搬移、空来源短路。
func TestManagerMoveRulesBySourceToProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	b, err := m.AddProfile(ctx, "方案B", "", "", "", true, 2)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}

	for _, p := range []string{"s1.test", "s2.test"} {
		if _, err := m.AddRule(ctx, "DOMAIN", p, 0, false, true); err != nil {
			t.Fatalf("AddRule %s: %v", p, err)
		}
	}
	if _, err := m.DB().Raw().ExecContext(ctx,
		"UPDATE rules SET source = 'airport:HK' WHERE rule_param IN ('s1.test','s2.test')"); err != nil {
		t.Fatalf("设置 source: %v", err)
	}
	if _, err := m.AddRule(ctx, "DOMAIN", "other.test", 0, false, true); err != nil {
		t.Fatalf("AddRule other: %v", err)
	}

	// 空来源（含纯空白）直接返回 0, nil。
	if n, err := m.MoveRulesBySourceToProfile(ctx, "", b.ID); err != nil || n != 0 {
		t.Errorf("空来源应返回 0,nil，实际 %d,%v", n, err)
	}
	if n, err := m.MoveRulesBySourceToProfile(ctx, "   ", b.ID); err != nil || n != 0 {
		t.Errorf("纯空白来源应返回 0,nil，实际 %d,%v", n, err)
	}

	n, err := m.MoveRulesBySourceToProfile(ctx, "airport:HK", b.ID)
	if err != nil {
		t.Fatalf("MoveRulesBySourceToProfile: %v", err)
	}
	if n != 2 {
		t.Fatalf("MoveRulesBySourceToProfile 返回 %d, want 2", n)
	}
	inB, err := m.ListRulesByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListRulesByProfile B: %v", err)
	}
	if len(inB) != 2 {
		t.Errorf("方案B 应有 2 条来源规则，实际 %d", len(inB))
	}
	if _, err := m.MoveRulesBySourceToProfile(canceledCtx(t), "airport:HK", b.ID); err == nil {
		t.Error("MoveRulesBySourceToProfile(canceled) 期望通用错误")
	}
}

// TestManagerMoveGroupsToProfile 覆盖 MoveGroupsToProfile：返回移动行数、移动后按方案计数。
func TestManagerMoveGroupsToProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile A: %v", err)
	}
	b, err := m.AddProfile(ctx, "方案B", "", "", "", true, 2)
	if err != nil {
		t.Fatalf("AddProfile B: %v", err)
	}

	var ids []int64
	for _, name := range []string{"GA1", "GA2"} {
		g, err := m.AddProxyGroupWithProfile(ctx, a.ID, name, "select", "", "", 0, 0)
		if err != nil {
			t.Fatalf("AddProxyGroupWithProfile %s: %v", name, err)
		}
		ids = append(ids, g.ID)
	}

	if n, err := m.MoveGroupsToProfile(ctx, nil, b.ID); err != nil || n != 0 {
		t.Errorf("空 ids 应返回 0,nil，实际 %d,%v", n, err)
	}
	n, err := m.MoveGroupsToProfile(ctx, ids, b.ID)
	if err != nil {
		t.Fatalf("MoveGroupsToProfile: %v", err)
	}
	if n != 2 {
		t.Fatalf("MoveGroupsToProfile 返回 %d, want 2", n)
	}
	inB, err := m.ListProxyGroupsByProfile(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile B: %v", err)
	}
	if len(inB) != 2 {
		t.Errorf("方案B 应有 2 个分组，实际 %d", len(inB))
	}
	for _, g := range inB {
		if g.ProfileID != b.ID {
			t.Errorf("分组 %d ProfileID = %d, want %d", g.ID, g.ProfileID, b.ID)
		}
	}
	inA, err := m.ListProxyGroupsByProfile(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListProxyGroupsByProfile A: %v", err)
	}
	if len(inA) != 0 {
		t.Errorf("方案A 应已无分组，实际 %+v", inA)
	}
	if _, err := m.MoveGroupsToProfile(canceledCtx(t), []int64{ids[0]}, b.ID); err == nil {
		t.Error("MoveGroupsToProfile(canceled) 期望通用错误")
	}
}

// TestManagerCreateSubTokenWithProfile 覆盖 CreateSubTokenWithProfile：绑定方案后 GetSubToken 回读，
// 以及 profileID=0（旧全局路径）仍写 0。
func TestManagerCreateSubTokenWithProfile(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	p, err := m.AddProfile(ctx, "方案A", "", "", "", true, 1)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}

	if err := m.CreateSubTokenWithProfile(ctx, "tok-A", "名称A", "https://a/sub", "pipe-a", "ini://a", "*.a.com", "b.com", p.ID); err != nil {
		t.Fatalf("CreateSubTokenWithProfile A: %v", err)
	}
	got, err := m.GetSubToken(ctx, "tok-A")
	if err != nil {
		t.Fatalf("GetSubToken: %v", err)
	}
	if got.ProfileID != p.ID {
		t.Fatalf("令牌 ProfileID = %d, want %d", got.ProfileID, p.ID)
	}
	if got.Name != "名称A" || got.Target != "https://a/sub" || got.Pipeline != "pipe-a" || got.Config != "ini://a" {
		t.Errorf("令牌定制字段未落库: %+v", got)
	}

	if err := m.CreateSubTokenWithProfile(ctx, "tok-0", "未绑定", "", "", "", "", "", 0); err != nil {
		t.Fatalf("CreateSubTokenWithProfile 0: %v", err)
	}
	legacy, err := m.GetSubToken(ctx, "tok-0")
	if err != nil {
		t.Fatalf("GetSubToken 0: %v", err)
	}
	if legacy.ProfileID != 0 {
		t.Errorf("profileID=0 应保持旧行为，实际 ProfileID = %d", legacy.ProfileID)
	}

	list, err := m.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 个令牌，实际 %d", len(list))
	}
	if err := m.CreateSubTokenWithProfile(canceledCtx(t), "tok-X", "", "", "", "", "", "", 0); err == nil {
		t.Error("CreateSubTokenWithProfile(canceled) 期望通用错误")
	}
}
