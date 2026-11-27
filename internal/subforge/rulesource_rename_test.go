package subforge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/manifest"
	"github.com/tktogether/subforge/internal/subcfg"
)

// renameFixture 准备一个清单、一个带版本文件的规则源，以及可选的绑定方案。
type renameFixture struct {
	m       *Manager
	dir     string
	path    string
	out     string
	version string
	profile *Profile
}

func setupRenameFixture(t *testing.T, withProfile bool) *renameFixture {
	t.Helper()
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	out := filepath.Join(dir, "rules")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	mf := &manifest.Manifest{
		Sources: []manifest.Source{{Name: "base", URL: "http://example.com/base.yaml"}},
		Output:  manifest.Output{Format: "clash-meta", Path: out},
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	version := "base-20260101-000000.000000000.yaml"
	if err := os.WriteFile(filepath.Join(out, version), []byte("rules: []\n"), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, ".current-versions.json"), []byte(`{"base":"`+version+`"}`), 0o644); err != nil {
		t.Fatalf("write pointer: %v", err)
	}
	f := &renameFixture{m: m, dir: dir, path: path, out: out, version: version}
	if withProfile {
		p, err := m.AddProfile(ctx, "方案", "", "", "", true, 0)
		if err != nil {
			t.Fatalf("AddProfile: %v", err)
		}
		if err := m.SetProfileRuleSources(ctx, p.ID, []string{"base"}); err != nil {
			t.Fatalf("SetProfileRuleSources: %v", err)
		}
		f.profile = p
	}
	return f
}

// assertSourceName 断言清单里第一个规则源的名称。
func (f *renameFixture) assertSourceName(t *testing.T, want string) {
	t.Helper()
	mf, err := manifest.LoadManifestRaw(f.path)
	if err != nil {
		t.Fatalf("LoadManifestRaw: %v", err)
	}
	if len(mf.Sources) != 1 || mf.Sources[0].SourceName() != want {
		t.Fatalf("清单源名 = %+v，期望 %q", mf.Sources, want)
	}
}

func TestManagerRenameRuleSource(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	mf := &manifest.Manifest{Sources: []manifest.Source{{Name: "base", URL: "http://example.com/base.yaml"}}, Output: manifest.Output{Format: "clash-meta", Path: filepath.Join(dir, "rules")}}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	out := filepath.Join(dir, "rules")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	version := "base-20260101-000000.000000000.yaml"
	if err := os.WriteFile(filepath.Join(out, version), []byte("rules: []\n"), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, ".current-versions.json"), []byte(`{"base":"`+version+`"}`), 0o644); err != nil {
		t.Fatalf("write pointer: %v", err)
	}
	p, err := m.AddProfile(ctx, "方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := m.SetProfileRuleSources(ctx, p.ID, []string{"base"}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}
	fg := subcfg.DefaultFinalGroupConfig()
	fg.Enabled = true
	fg.Overrides = map[string][]string{
		"rule:base":  {"出口"},
		"group:self": {"自建出口"},
		"rule:other": {"其它"},
	}
	if err := m.SetProfileFinalGroup(ctx, p.ID, fg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	// 第二个方案没有任何覆盖配置：改名循环要能跳过它（ok=false 分支）。
	if _, err := m.AddProfile(ctx, "方案二", "", "", "", true, 0); err != nil {
		t.Fatalf("AddProfile(方案二): %v", err)
	}

	if _, err := m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "renamed-20260101-000000.000000000.yaml")); err != nil {
		t.Fatalf("new version missing: %v", err)
	}
	profiles, _ := m.ListProfiles(ctx)
	var got Profile
	for _, item := range profiles {
		if item.ID == p.ID {
			got = item
		}
	}
	if len(got.RuleSources) != 1 || got.RuleSources[0] != "renamed" {
		t.Fatalf("profile binding not renamed: %+v", profiles)
	}
	overrides := got.FinalGroup.Overrides
	if got := overrides["rule:renamed"]; len(got) != 1 || got[0] != "出口" {
		t.Fatalf("最终组覆盖键未迁移: %+v", overrides)
	}
	if _, ok := overrides["rule:base"]; ok {
		t.Fatalf("旧覆盖键仍存在: %+v", overrides)
	}
	if got := overrides["group:self"]; len(got) != 1 || got[0] != "自建出口" {
		t.Fatalf("来源覆盖键被误改: %+v", overrides)
	}
	if got := overrides["rule:other"]; len(got) != 1 || got[0] != "其它" {
		t.Fatalf("其它规则源覆盖键被误改: %+v", overrides)
	}
	cur, _ := os.ReadFile(filepath.Join(out, ".current-versions.json"))
	if !strings.Contains(string(cur), "renamed") {
		t.Fatalf("pointer not renamed: %s", cur)
	}
}

func TestManagerRenameRuleSourceRejectsAirport(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	mf := &manifest.Manifest{Sources: []manifest.Source{{Name: "airport-1-hk", URL: "http://example.com/a.yaml"}}, Output: manifest.Output{Format: "clash-meta", Path: "./rules"}}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "airport-1-hk", "renamed"); err == nil || !strings.Contains(err.Error(), "机场") {
		t.Fatalf("机场规则源改名应被拒绝: %v", err)
	}
}

// TestManagerRenameRuleSourceUnreadableVersionDir 版本目录不可读时必须失败，
// 不能返回成功却只改清单（改名后源没有版本，订阅静默丢规则）。
func TestManagerRenameRuleSourceUnreadableVersionDir(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	// 路径组件超长：Windows 下 os.ReadDir 返回「文件名语法不正确」，不是 fs.ErrNotExist。
	// 直接写 rules.output_dir 配置，避免 LoadManifest 对清单 output.path 做目录校验后回落。
	notDir := filepath.Join(dir, strings.Repeat("a", 300))
	if err := m.setConfig(ctx, RulesOutputDirKey, notDir); err != nil {
		t.Fatalf("setConfig(rules.output_dir): %v", err)
	}
	mf := &manifest.Manifest{
		Sources: []manifest.Source{{Name: "base", URL: "http://example.com/base.yaml"}},
		Output:  manifest.Output{Format: "clash-meta", Path: "./rules"},
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	_, err := m.RenameRuleSource(ctx, "base", "renamed")
	if err == nil || !errors.Is(err, ErrRuleSourceRenameInternal) {
		t.Fatalf("版本目录不可读应返回内部错误，实际 %v", err)
	}
	f := &renameFixture{m: m, path: path}
	f.assertSourceName(t, "base")
}

// TestManagerRenameRuleSourceRejectsOverrideConflict 目标覆盖键已存在且内容不同时
// 必须在改文件前拒绝，不能静默覆盖用户配置。
func TestManagerRenameRuleSourceRejectsOverrideConflict(t *testing.T) {
	f := setupRenameFixture(t, true)
	ctx := t.Context()
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Enabled = true
	cfg.Overrides = map[string][]string{
		"rule:base":    {"出口"},
		"rule:renamed": {"旧配置"},
	}
	if err := f.m.SetProfileFinalGroup(ctx, f.profile.ID, cfg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}

	_, err := f.m.RenameRuleSource(ctx, "base", "renamed")
	if err == nil || !strings.Contains(err.Error(), "覆盖配置") {
		t.Fatalf("冲突覆盖键应被拒绝，实际 %v", err)
	}
	f.assertSourceName(t, "base")
	if _, statErr := os.Stat(filepath.Join(f.out, f.version)); statErr != nil {
		t.Fatalf("版本文件不应被改名: %v", statErr)
	}
}

// TestManagerRenameRuleSourcePreflightsProfiles 方案数据损坏时必须在改文件前失败，
// 旧名仍可重试。
func TestManagerRenameRuleSourcePreflightsProfiles(t *testing.T) {
	f := setupRenameFixture(t, true)
	ctx := t.Context()
	if _, err := f.m.DB().Raw().ExecContext(ctx,
		"UPDATE profiles SET final_group = '{' WHERE id = ?", f.profile.ID); err != nil {
		t.Fatalf("制造损坏 final_group: %v", err)
	}
	if _, err := f.m.RenameRuleSource(ctx, "base", "renamed"); err == nil || !errors.Is(err, ErrRuleSourceRenameInternal) {
		t.Fatalf("方案预检失败应返回内部错误，实际 %v", err)
	}
	f.assertSourceName(t, "base")
	if _, statErr := os.Stat(filepath.Join(f.out, f.version)); statErr != nil {
		t.Fatalf("版本文件不应被改名: %v", statErr)
	}
}

// TestManagerRenameRuleSourceRollsBackOnBindingFailure 方案绑定事务失败时，
// 已改名的版本文件、当前版本指针与清单都要恢复。
func TestManagerRenameRuleSourceRollsBackOnBindingFailure(t *testing.T) {
	f := setupRenameFixture(t, true)
	ctx := t.Context()
	if _, err := f.m.DB().Raw().ExecContext(ctx,
		`CREATE TRIGGER block_profile_update BEFORE UPDATE ON profiles BEGIN SELECT RAISE(ABORT, 'blocked'); END`); err != nil {
		t.Fatalf("创建触发器: %v", err)
	}
	_, err := f.m.RenameRuleSource(ctx, "base", "renamed")
	if err == nil || !errors.Is(err, ErrRuleSourceRenameInternal) {
		t.Fatalf("绑定写入失败应返回内部错误，实际 %v", err)
	}
	f.assertSourceName(t, "base")
	if _, statErr := os.Stat(filepath.Join(f.out, f.version)); statErr != nil {
		t.Errorf("版本文件未恢复: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(f.out, "renamed-20260101-000000.000000000.yaml")); !os.IsNotExist(statErr) {
		t.Errorf("改名后的版本文件仍存在: %v", statErr)
	}
	cur, err := os.ReadFile(filepath.Join(f.out, ".current-versions.json"))
	if err != nil {
		t.Fatalf("读指针: %v", err)
	}
	if !strings.Contains(string(cur), `"base"`) || strings.Contains(string(cur), `"renamed"`) {
		t.Errorf("指针未恢复: %s", cur)
	}
	got, err := f.m.GetProfile(ctx, f.profile.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.RuleSources) != 1 || got.RuleSources[0] != "base" {
		t.Errorf("方案绑定被部分迁移: %+v", got.RuleSources)
	}
}

// TestManagerRenameRuleSourceWithoutVersionDir 输出目录尚不存在时没有版本文件要迁移，
// 改名应正常完成。
func TestManagerRenameRuleSourceWithoutVersionDir(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	mf := &manifest.Manifest{
		Sources: []manifest.Source{{Name: "base", URL: "http://example.com/base.yaml"}},
		Output:  manifest.Output{Format: "clash-meta", Path: filepath.Join(dir, "rules-not-created")},
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	f := &renameFixture{path: path}
	f.assertSourceName(t, "renamed")
}

// TestManagerRenameRuleSourceTargetVersionExists 目标版本文件已存在时，
// 必须在改任何东西之前拒绝。
func TestManagerRenameRuleSourceTargetVersionExists(t *testing.T) {
	f := setupRenameFixture(t, false)
	ctx := t.Context()
	target := filepath.Join(f.out, "renamed-20260101-000000.000000000.yaml")
	if err := os.WriteFile(target, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	_, err := f.m.RenameRuleSource(ctx, "base", "renamed")
	if err == nil || !strings.Contains(err.Error(), "目标版本文件已存在") {
		t.Fatalf("目标版本冲突应被拒绝，实际 %v", err)
	}
	f.assertSourceName(t, "base")
	if _, statErr := os.Stat(filepath.Join(f.out, f.version)); statErr != nil {
		t.Errorf("原版本文件不应被改名: %v", statErr)
	}
}

// TestManagerRenameRuleSourceCorruptPointer 当前版本指针损坏时返回内部错误，
// 已改名的版本文件要恢复。
func TestManagerRenameRuleSourceCorruptPointer(t *testing.T) {
	f := setupRenameFixture(t, false)
	ctx := t.Context()
	if err := os.WriteFile(filepath.Join(f.out, ".current-versions.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("写损坏指针: %v", err)
	}
	_, err := f.m.RenameRuleSource(ctx, "base", "renamed")
	if err == nil || !errors.Is(err, ErrRuleSourceRenameInternal) {
		t.Fatalf("指针损坏应返回内部错误，实际 %v", err)
	}
	f.assertSourceName(t, "base")
	if _, statErr := os.Stat(filepath.Join(f.out, f.version)); statErr != nil {
		t.Errorf("版本文件未恢复: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(f.out, "renamed-20260101-000000.000000000.yaml")); !os.IsNotExist(statErr) {
		t.Errorf("改名后的版本文件仍存在: %v", statErr)
	}
}

// TestManagerRenameRuleSourceRejectsLongName 超长新名称在改文件前按参数错误拒绝。
func TestManagerRenameRuleSourceRejectsLongName(t *testing.T) {
	f := setupRenameFixture(t, false)
	ctx := t.Context()
	long := strings.Repeat("a", 300)
	_, err := f.m.RenameRuleSource(ctx, "base", long)
	if err == nil || !strings.Contains(err.Error(), "不能超过") {
		t.Fatalf("超长名称应被拒绝，实际 %v", err)
	}
	if errors.Is(err, ErrRuleSourceRenameInternal) {
		t.Errorf("超长名称属于参数错误，不应标记为内部错误")
	}
	f.assertSourceName(t, "base")
	if _, statErr := os.Stat(filepath.Join(f.out, f.version)); statErr != nil {
		t.Errorf("版本文件不应被改名: %v", statErr)
	}
}

// TestManagerRenameRuleSourceKeepsOtherSourceVersions 源名互为前缀时，
// 改名只迁移自己的版本文件，不能把 base-extra 的文件也改成 renamed-extra。
func TestManagerRenameRuleSourceKeepsOtherSourceVersions(t *testing.T) {
	f := setupRenameFixture(t, false)
	ctx := t.Context()
	other := "base-extra-20260101-000000.000000000.yaml"
	if err := os.WriteFile(filepath.Join(f.out, other), []byte("rules: []\n"), 0o644); err != nil {
		t.Fatalf("写另一源版本: %v", err)
	}
	if _, err := f.m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.out, other)); err != nil {
		t.Errorf("另一源的版本文件被误迁移: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.out, "renamed-extra-20260101-000000.000000000.yaml")); !os.IsNotExist(err) {
		t.Errorf("不应出现 renamed-extra 版本文件: %v", err)
	}
}

// TestManagerRenameRuleSourceMatchesPaddedBinding 方案绑定带首尾空白时，
// 按运行期 TrimSpace 口径迁移。
func TestManagerRenameRuleSourceMatchesPaddedBinding(t *testing.T) {
	f := setupRenameFixture(t, false)
	ctx := t.Context()
	p, err := f.m.AddProfile(ctx, "方案", "", "", "", true, 0)
	if err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := f.m.SetProfileRuleSources(ctx, p.ID, []string{" base "}); err != nil {
		t.Fatalf("SetProfileRuleSources: %v", err)
	}
	if _, err := f.m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	got, err := f.m.GetProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(got.RuleSources) != 1 || got.RuleSources[0] != "renamed" {
		t.Errorf("空白绑定未迁移: %v", got.RuleSources)
	}
}

// TestManagerRenameRuleSourceAllowsSameOverrideValue 目标覆盖键与旧键内容相同时，
// 视为同一条配置迁移，不触发冲突错误。
func TestManagerRenameRuleSourceAllowsSameOverrideValue(t *testing.T) {
	f := setupRenameFixture(t, true)
	ctx := t.Context()
	cfg := subcfg.DefaultFinalGroupConfig()
	cfg.Enabled = true
	cfg.Overrides = map[string][]string{
		"rule:base":    {"出口"},
		"rule:renamed": {"出口"},
	}
	if err := f.m.SetProfileFinalGroup(ctx, f.profile.ID, cfg); err != nil {
		t.Fatalf("SetProfileFinalGroup: %v", err)
	}
	if _, err := f.m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	got, err := f.m.GetProfile(ctx, f.profile.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if _, ok := got.FinalGroup.Overrides["rule:base"]; ok {
		t.Errorf("旧覆盖键仍存在: %v", got.FinalGroup.Overrides)
	}
	if values := got.FinalGroup.Overrides["rule:renamed"]; len(values) != 1 || values[0] != "出口" {
		t.Errorf("新覆盖键内容不符: %v", got.FinalGroup.Overrides)
	}
}

// TestManagerRenameRuleSourceValidation 覆盖名称校验与源存在性分支。
func TestManagerRenameRuleSourceValidation(t *testing.T) {
	f := setupRenameFixture(t, false)
	ctx := t.Context()
	cases := []struct{ oldName, newName, want string }{
		{"", "x", "原名称不能为空"},
		{"base", "bad/name", "不能包含"},
		{"base", "base", "不能与原名称相同"},
		{"base", strings.Repeat("汉", 80), "过长"},
	}
	for _, tc := range cases {
		_, err := f.m.RenameRuleSource(ctx, tc.oldName, tc.newName)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("RenameRuleSource(%q,%q) 错误=%v，期望包含 %q", tc.oldName, tc.newName, err, tc.want)
		}
	}
}

// TestManagerRenameRuleSourceDuplicateAndMissing 覆盖重名目标、未知源与缺清单。
func TestManagerRenameRuleSourceDuplicateAndMissing(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	mf := &manifest.Manifest{
		Sources: []manifest.Source{
			{Name: "base", URL: "http://example.com/base.yaml"},
			{Name: "other", URL: "http://example.com/other.yaml"},
		},
		Output: manifest.Output{Format: "clash-meta", Path: dir},
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "base", "other"); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Errorf("重名目标应报已存在: %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "ghost", "x"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Errorf("未知源应报不存在: %v", err)
	}
	missing := filepath.Join(dir, "missing", "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, missing); err != nil {
		t.Fatalf("SetRuleManifestPath(missing): %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "base", "x"); err == nil || !strings.Contains(err.Error(), "读取规则源清单") {
		t.Errorf("缺清单应报读取失败: %v", err)
	}
}

// TestRuleSourceRenameCleanupHelpers 覆盖回滚失败聚合与错误保留语义。
func TestRuleSourceRenameCleanupHelpers(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing-20240101-000000.000000000.yaml")
	// 反向恢复时新名文件不存在 → 回滚失败被收集。
	errs := rollbackVersionFiles([]versionRename{{
		oldPath: filepath.Join(dir, "old.yaml"),
		newPath: missing,
		oldName: "old.yaml",
	}})
	if len(errs) != 1 {
		t.Fatalf("回滚失败项 = %d，期望 1", len(errs))
	}
	wrapped := withCleanup(errors.New("cause"), errs)
	if !strings.Contains(wrapped.Error(), "回滚未完全成功") || !strings.Contains(wrapped.Error(), "恢复版本文件") {
		t.Errorf("回滚失败信息不完整: %v", wrapped)
	}
	if got := withCleanup(errors.New("plain"), nil); got.Error() != "plain" {
		t.Errorf("无回滚失败时应保留原错误: %v", got)
	}
	// 版本文件改名失败也要走同一条错误路径。
	if err := renameVersionFiles([]versionRename{{
		oldPath: missing,
		newPath: filepath.Join(dir, "new.yaml"),
		oldName: "missing-20240101-000000.000000000.yaml",
	}}); err == nil || !strings.Contains(err.Error(), "重命名版本文件") {
		t.Errorf("改名失败应返回错误，实际 %v", err)
	}
}

// TestManagerRenameRuleSourceShortStamp Update 产物使用短时间戳，
// 改名与指针迁移必须兼容。
func TestManagerRenameRuleSourceShortStamp(t *testing.T) {
	m := newTestManager(t)
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	if err := m.SetRuleManifestPath(ctx, path); err != nil {
		t.Fatalf("SetRuleManifestPath: %v", err)
	}
	out := filepath.Join(dir, "rules")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := manifest.SaveManifest(path, &manifest.Manifest{
		Sources: []manifest.Source{{Name: "base", URL: "http://example.com/base.yaml"}},
		Output:  manifest.Output{Format: "clash-meta", Path: out},
	}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
	short := "base-20240101-000000.yaml"
	if err := os.WriteFile(filepath.Join(out, short), []byte("rules: []\n"), 0o644); err != nil {
		t.Fatalf("写版本文件: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, ".current-versions.json"), []byte(`{"base":"`+short+`"}`), 0o644); err != nil {
		t.Fatalf("写指针: %v", err)
	}
	if _, err := m.RenameRuleSource(ctx, "base", "renamed"); err != nil {
		t.Fatalf("RenameRuleSource: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "renamed-20240101-000000.yaml")); err != nil {
		t.Errorf("短时间戳版本文件未迁移: %v", err)
	}
	cur, err := os.ReadFile(filepath.Join(out, ".current-versions.json"))
	if err != nil {
		t.Fatalf("读指针: %v", err)
	}
	if !strings.Contains(string(cur), "renamed-20240101-000000.yaml") {
		t.Errorf("指针未迁移: %s", cur)
	}
}
