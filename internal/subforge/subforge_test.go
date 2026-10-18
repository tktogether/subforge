package subforge

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// newTestManager 创建临时 SQLite 管理器。
// 用 t.TempDir() 造文件库而非 :memory::Open 会执行 WAL pragma，
// 内存库不支持 WAL 模式会直接失败。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestNewManagerCloseDB(t *testing.T) {
	m := newTestManager(t)
	if m.DB() == nil {
		t.Fatal("DB() 返回 nil")
	}
	if err := m.DB().Raw().Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAirportCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	// 空库 List 返回空切片（非 nil）
	got, err := m.ListAirports(ctx)
	if err != nil {
		t.Fatalf("ListAirports: %v", err)
	}
	if got == nil {
		t.Fatal("ListAirports 返回 nil 切片")
	}
	if len(got) != 0 {
		t.Fatalf("空库应有 0 条, got %d", len(got))
	}

	a1, err := m.AddAirport(ctx, "a1", "https://x/sub", "r1", true)
	if err != nil {
		t.Fatalf("AddAirport a1: %v", err)
	}
	if a1.Name != "a1" || !a1.Enabled || a1.ID <= 0 {
		t.Fatalf("AddAirport a1 返回不符: %+v", a1)
	}

	if _, err := m.AddAirport(ctx, "a2", "https://y/sub", "", false); err != nil {
		t.Fatalf("AddAirport a2: %v", err)
	}

	list, err := m.ListAirports(ctx)
	if err != nil {
		t.Fatalf("ListAirports: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 条, got %d", len(list))
	}

	got1, err := m.GetAirport(ctx, a1.ID)
	if err != nil {
		t.Fatalf("GetAirport: %v", err)
	}
	if got1.Remarks != "r1" {
		t.Fatalf("Remarks 应为 r1, got %q", got1.Remarks)
	}

	upd, err := m.UpdateAirport(ctx, a1.ID, "a1b", "https://z/sub", "r2", false)
	if err != nil {
		t.Fatalf("UpdateAirport: %v", err)
	}
	if upd.Name != "a1b" || upd.Enabled || upd.Remarks != "r2" {
		t.Fatalf("UpdateAirport 返回不符: %+v", upd)
	}

	if err := m.DeleteAirport(ctx, a1.ID); err != nil {
		t.Fatalf("DeleteAirport: %v", err)
	}
	list, _ = m.ListAirports(ctx)
	if len(list) != 1 {
		t.Fatalf("删后应有 1 条, got %d", len(list))
	}
}

func TestAirportNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.GetAirport(ctx, 9999)
	if err == nil || err.Error() != "订阅 9999 不存在" {
		t.Fatalf("GetAirport 错误不符: %v", err)
	}

	_, err = m.UpdateAirport(ctx, 9999, "x", "y", "z", true)
	if err == nil || !contains(err.Error(), "不存在") {
		t.Fatalf("UpdateAirport 错误不符: %v", err)
	}

	if err := m.DeleteAirport(ctx, 9999); err != nil {
		t.Fatalf("DeleteAirport 不存在 id 应返回 nil, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestSelfNodeDelegate(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	n, err := m.AddSelfNode(ctx, "vmess://x", "node1", true)
	if err != nil {
		t.Fatalf("AddSelfNode: %v", err)
	}
	if n.Link != "vmess://x" || n.Name != "node1" || !n.Enabled {
		t.Fatalf("AddSelfNode 返回不符: %+v", n)
	}

	got, err := m.GetSelfNode(ctx, n.ID)
	if err != nil {
		t.Fatalf("GetSelfNode: %v", err)
	}
	if got.Name != "node1" || !got.Enabled {
		t.Fatalf("GetSelfNode 不符: %+v", got)
	}

	list, err := m.ListSelfNodes(ctx)
	if err != nil {
		t.Fatalf("ListSelfNodes: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("应有 1 条, got %d", len(list))
	}

	if err := m.UpdateSelfNode(ctx, n.ID, "vmess://y", "node2", false); err != nil {
		t.Fatalf("UpdateSelfNode: %v", err)
	}
	got, _ = m.GetSelfNode(ctx, n.ID)
	if got.Name != "node2" || got.Enabled {
		t.Fatalf("UpdateSelfNode 后不符: %+v", got)
	}

	if err := m.DeleteSelfNode(ctx, n.ID); err != nil {
		t.Fatalf("DeleteSelfNode: %v", err)
	}

	// SelfNode 系纯委托，查不到透传 sql.ErrNoRows
	_, err = m.GetSelfNode(ctx, 9999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetSelfNode 应 sql.ErrNoRows, got %v", err)
	}
	if err := m.UpdateSelfNode(ctx, 9999, "x", "y", true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateSelfNode 应 sql.ErrNoRows, got %v", err)
	}
}

func TestProxyGroupCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	got, err := m.ListProxyGroups(ctx)
	if err != nil {
		t.Fatalf("ListProxyGroups: %v", err)
	}
	if got == nil {
		t.Fatal("ListProxyGroups 返回 nil")
	}
	if len(got) != 0 {
		t.Fatalf("空库应有 0 条, got %d", len(got))
	}

	g, err := m.AddProxyGroup(ctx, "g1", "select", "re:^HK", "http://x", 300)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if g.Name != "g1" || g.Type != "select" || g.Interval != 300 || g.ID <= 0 {
		t.Fatalf("AddProxyGroup 返回不符: %+v", g)
	}

	list, _ := m.ListProxyGroups(ctx)
	if len(list) != 1 {
		t.Fatalf("应有 1 条, got %d", len(list))
	}

	if err := m.DeleteProxyGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteProxyGroup: %v", err)
	}
	list, _ = m.ListProxyGroups(ctx)
	if len(list) != 0 {
		t.Fatalf("删后应有 0 条, got %d", len(list))
	}
}

func TestProxyGroupNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.GetProxyGroup(ctx, 9999)
	if err == nil || err.Error() != "代理分组 9999 不存在" {
		t.Fatalf("GetProxyGroup 错误不符: %v", err)
	}
	if err := m.UpdateProxyGroup(ctx, 9999, "x", "y", "z", "w", 1); err == nil || err.Error() != "代理分组 9999 不存在" {
		t.Fatalf("UpdateProxyGroup 错误不符: %v", err)
	}
}

func TestProxyGroupGetUpdateHappy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	g, err := m.AddProxyGroup(ctx, "g1", "url-test", "re:^US", "http://a", 600)
	if err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}

	got, err := m.GetProxyGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetProxyGroup: %v", err)
	}
	if got.Regex != "re:^US" || got.URL != "http://a" || got.Interval != 600 {
		t.Fatalf("GetProxyGroup 不符: %+v", got)
	}

	if err := m.UpdateProxyGroup(ctx, g.ID, "g2", "select", "re:^JP", "http://b", 120); err != nil {
		t.Fatalf("UpdateProxyGroup: %v", err)
	}
	got, _ = m.GetProxyGroup(ctx, g.ID)
	if got.Name != "g2" || got.Type != "select" || got.Regex != "re:^JP" || got.URL != "http://b" || got.Interval != 120 {
		t.Fatalf("UpdateProxyGroup 后不符: %+v", got)
	}
}

func TestRuleCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	got, err := m.ListRules(ctx)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if got == nil {
		t.Fatal("ListRules 返回 nil")
	}
	if len(got) != 0 {
		t.Fatalf("空库应有 0 条, got %d", len(got))
	}

	r, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", 1, true, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if r.RuleType != "DOMAIN-SUFFIX" || r.RuleParam != "example.com" || r.RuleConfig != 1 || !r.ResolveDNS || !r.Enabled {
		t.Fatalf("AddRule 返回不符: %+v", r)
	}

	list, _ := m.ListRules(ctx)
	if len(list) != 1 {
		t.Fatalf("应有 1 条, got %d", len(list))
	}

	if err := m.DeleteRule(ctx, r.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	list, _ = m.ListRules(ctx)
	if len(list) != 0 {
		t.Fatalf("删后应有 0 条, got %d", len(list))
	}
}

func TestRuleNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.GetRule(ctx, 9999)
	if err == nil || err.Error() != "规则 9999 不存在" {
		t.Fatalf("GetRule 错误不符: %v", err)
	}
	if err := m.UpdateRule(ctx, 9999, "DOMAIN", "x", 1, true, true); err == nil || err.Error() != "规则 9999 不存在" {
		t.Fatalf("UpdateRule 错误不符: %v", err)
	}
}

func TestRuleGetUpdateHappy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	r, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "example.com", 1, true, true)
	if err != nil {
		t.Fatalf("AddRule: %v", err)
	}

	got, err := m.GetRule(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.RuleType != "DOMAIN-SUFFIX" || got.RuleParam != "example.com" || got.RuleConfig != 1 || !got.ResolveDNS || !got.Enabled {
		t.Fatalf("GetRule 不符: %+v", got)
	}

	if err := m.UpdateRule(ctx, r.ID, "DOMAIN-KEYWORD", "test", 2, false, false); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	got, _ = m.GetRule(ctx, r.ID)
	if got.RuleType != "DOMAIN-KEYWORD" || got.RuleParam != "test" || got.RuleConfig != 2 || got.ResolveDNS || got.Enabled {
		t.Fatalf("UpdateRule 后不符: %+v", got)
	}
}

func TestSubTokenCRUD(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	if err := m.CreateSubToken(ctx, "tok1", "n1"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}
	tok, err := m.GetSubToken(ctx, "tok1")
	if err != nil {
		t.Fatalf("GetSubToken: %v", err)
	}
	if tok.Name != "n1" {
		t.Fatalf("Name 应为 n1, got %q", tok.Name)
	}

	if err := m.CreateSubToken(ctx, "tok2", "n2"); err != nil {
		t.Fatalf("CreateSubToken tok2: %v", err)
	}
	list, err := m.ListSubTokens(ctx)
	if err != nil {
		t.Fatalf("ListSubTokens: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 条, got %d", len(list))
	}

	_, err = m.GetSubToken(ctx, "nope")
	if err == nil || err.Error() != "令牌 nope 不存在" {
		t.Fatalf("GetSubToken 错误不符: %v", err)
	}

	if err := m.DeleteSubToken(ctx, "tok1"); err != nil {
		t.Fatalf("DeleteSubToken: %v", err)
	}
	if _, err := m.GetSubToken(ctx, "tok1"); err == nil {
		t.Fatal("删后 GetSubToken 应报错")
	}

	// 重复 token 触发主键冲突（非 nil 错误）
	if err := m.CreateSubToken(ctx, "tok2", "dup"); err == nil {
		t.Fatal("重复 token 应返回错误")
	}
}

func TestGenerateSubscription(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	sub, err := m.GenerateSubscription(ctx)
	if err != nil {
		t.Fatalf("GenerateSubscription: %v", err)
	}
	if len(sub.Airports) != 0 || len(sub.Groups) != 0 || len(sub.Rules) != 0 {
		t.Fatalf("空库应全 0: %+v", sub)
	}

	if _, err := m.AddAirport(ctx, "on", "https://on", "", true); err != nil {
		t.Fatalf("AddAirport on: %v", err)
	}
	if _, err := m.AddAirport(ctx, "off", "https://off", "", false); err != nil {
		t.Fatalf("AddAirport off: %v", err)
	}
	if _, err := m.AddProxyGroup(ctx, "g1", "select", "re:^HK", "http://x", 300); err != nil {
		t.Fatalf("AddProxyGroup: %v", err)
	}
	if _, err := m.AddRule(ctx, "DOMAIN-SUFFIX", "a.com", 1, true, true); err != nil {
		t.Fatalf("AddRule 1: %v", err)
	}
	if _, err := m.AddRule(ctx, "DOMAIN", "b.com", 1, true, true); err != nil {
		t.Fatalf("AddRule 2: %v", err)
	}

	sub, err = m.GenerateSubscription(ctx)
	if err != nil {
		t.Fatalf("GenerateSubscription: %v", err)
	}
	if len(sub.Airports) != 1 || sub.Airports[0].Name != "on" {
		t.Fatalf("应只含 enabled 机场: %+v", sub.Airports)
	}
	if len(sub.Groups) != 1 {
		t.Fatalf("分组应为 1: %+v", sub.Groups)
	}
	if len(sub.Rules) != 2 {
		t.Fatalf("规则应为 2: %+v", sub.Rules)
	}
}

func TestConfigGetSet(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	// 查不到的 key 返回 ("", nil)，不报错
	v, err := m.getConfig(ctx, "missing")
	if err != nil {
		t.Fatalf("getConfig missing 应 nil, got %v", err)
	}
	if v != "" {
		t.Fatalf("getConfig missing 应空串, got %q", v)
	}

	if err := m.setConfig(ctx, "k", "v1"); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	v, err = m.getConfig(ctx, "k")
	if err != nil || v != "v1" {
		t.Fatalf("getConfig 应 v1, got %q err=%v", v, err)
	}

	// 同名 key 覆盖
	if err := m.setConfig(ctx, "k", "v2"); err != nil {
		t.Fatalf("setConfig v2: %v", err)
	}
	v, _ = m.getConfig(ctx, "k")
	if v != "v2" {
		t.Fatalf("覆盖后应 v2, got %q", v)
	}
}

// TestNewManagerBadDSN 覆盖 NewManager 的错误返回分支（store.Open 失败）。
func TestNewManagerBadDSN(t *testing.T) {
	_, err := NewManager(filepath.Join(t.TempDir(), "no-such-dir", "t.db"))
	if err == nil {
		t.Fatal("非法路径应返回错误")
	}
}

// TestManagerDBClosedErrors 覆盖各函数在 DB 已关闭时的错误分支。
// 关掉连接后调用，SQL 操作统一返回 error，从而命中每处的 "if err != nil return err"。
func TestManagerDBClosedErrors(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := m.ListAirports(ctx); err == nil {
		t.Fatal("ListAirports 应返回错误")
	}
	if _, err := m.GetAirport(ctx, 1); err == nil {
		t.Fatal("GetAirport 应返回错误")
	}
	if _, err := m.AddAirport(ctx, "a", "b", "c", true); err == nil {
		t.Fatal("AddAirport 应返回错误")
	}
	if _, err := m.UpdateAirport(ctx, 1, "a", "b", "c", true); err == nil {
		t.Fatal("UpdateAirport 应返回错误")
	}
	if err := m.DeleteAirport(ctx, 1); err == nil {
		t.Fatal("DeleteAirport 应返回错误")
	}
	if _, err := m.ListSelfNodes(ctx); err == nil {
		t.Fatal("ListSelfNodes 应返回错误")
	}
	if _, err := m.GetSelfNode(ctx, 1); err == nil {
		t.Fatal("GetSelfNode 应返回错误")
	}
	if _, err := m.AddSelfNode(ctx, "x", "y", true); err == nil {
		t.Fatal("AddSelfNode 应返回错误")
	}
	if err := m.UpdateSelfNode(ctx, 1, "x", "y", true); err == nil {
		t.Fatal("UpdateSelfNode 应返回错误")
	}
	if err := m.DeleteSelfNode(ctx, 1); err == nil {
		t.Fatal("DeleteSelfNode 应返回错误")
	}
	if _, err := m.ListProxyGroups(ctx); err == nil {
		t.Fatal("ListProxyGroups 应返回错误")
	}
	if _, err := m.AddProxyGroup(ctx, "g", "t", "r", "u", 1); err == nil {
		t.Fatal("AddProxyGroup 应返回错误")
	}
	if err := m.DeleteProxyGroup(ctx, 1); err == nil {
		t.Fatal("DeleteProxyGroup 应返回错误")
	}
	if _, err := m.ListRules(ctx); err == nil {
		t.Fatal("ListRules 应返回错误")
	}
	if _, err := m.AddRule(ctx, "T", "P", 1, true, true); err == nil {
		t.Fatal("AddRule 应返回错误")
	}
	if err := m.DeleteRule(ctx, 1); err == nil {
		t.Fatal("DeleteRule 应返回错误")
	}
	if err := m.CreateSubToken(ctx, "t", "n"); err == nil {
		t.Fatal("CreateSubToken 应返回错误")
	}
	if _, err := m.GetSubToken(ctx, "t"); err == nil {
		t.Fatal("GetSubToken 应返回错误")
	}
	if err := m.DeleteSubToken(ctx, "t"); err == nil {
		t.Fatal("DeleteSubToken 应返回错误")
	}
	if _, err := m.ListSubTokens(ctx); err == nil {
		t.Fatal("ListSubTokens 应返回错误")
	}
	if _, err := m.GetProxyGroup(ctx, 1); err == nil {
		t.Fatal("GetProxyGroup 应返回错误")
	}
	if err := m.UpdateProxyGroup(ctx, 1, "n", "t", "r", "u", 1); err == nil {
		t.Fatal("UpdateProxyGroup 应返回错误")
	}
	if _, err := m.GetRule(ctx, 1); err == nil {
		t.Fatal("GetRule 应返回错误")
	}
	if err := m.UpdateRule(ctx, 1, "T", "P", 1, true, true); err == nil {
		t.Fatal("UpdateRule 应返回错误")
	}
	if _, err := m.GenerateSubscription(ctx); err == nil {
		t.Fatal("GenerateSubscription 应返回错误")
	}
	if _, err := m.getConfig(ctx, "k"); err == nil {
		t.Fatal("getConfig 应返回错误")
	}
	if err := m.setConfig(ctx, "k", "v"); err == nil {
		t.Fatal("setConfig 应返回错误")
	}
}

// TestListScanErrors 覆盖 List* 的 rows.Scan 错误分支。
// sort_order 为 NULL 时 Scan 进 int 会失败，触发 "return nil, err"。
func TestListScanErrors(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	db := m.DB().Raw()

	if _, err := db.ExecContext(ctx, "INSERT INTO airports (name, subscription_url, remarks, enabled, sort_order) VALUES ('a','u','r',1,NULL)"); err != nil {
		t.Fatalf("insert airport: %v", err)
	}
	if _, err := m.ListAirports(ctx); err == nil {
		t.Fatal("ListAirports 应返回错误")
	}

	m2 := newTestManager(t)
	db2 := m2.DB().Raw()
	if _, err := db2.ExecContext(ctx, "INSERT INTO proxy_groups (name, type, regex, url, interval, sort_order) VALUES ('g','t','r','u',1,NULL)"); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	if _, err := m2.ListProxyGroups(ctx); err == nil {
		t.Fatal("ListProxyGroups 应返回错误")
	}

	m3 := newTestManager(t)
	db3 := m3.DB().Raw()
	if _, err := db3.ExecContext(ctx, "INSERT INTO rules (rule_type, rule_param, rule_config, resolve_dns, enabled, sort_order) VALUES ('T','P',1,1,1,NULL)"); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	if _, err := m3.ListRules(ctx); err == nil {
		t.Fatal("ListRules 应返回错误")
	}
}
