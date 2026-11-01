package subforge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/manifest"
)

// AddRuleSource 追加一个规则源并写回 manifest，返回 (清单路径, 是否新建, 错误)。
// name 为空时从 URL 推导；重名返回错误（消息含「已存在」）。
//
// 清单不存在时自动创建一份只含本次来源的最小清单，不再要求用户先点“生成默认清单”。
func (m *Manager) AddRuleSource(ctx context.Context, src manifest.Source) (string, bool, error) {
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(src.URL) == "" {
		return "", false, fmt.Errorf("url 不能为空")
	}
	mf := &manifest.Manifest{
		Output: manifest.Output{Format: "clash-meta", Path: "./rules"},
	}
	if _, statErr := os.Stat(path); statErr == nil {
		loaded, lerr := manifest.LoadManifestRaw(path)
		if lerr != nil {
			return "", false, fmt.Errorf("读取规则源清单 %s: %w", path, lerr)
		}
		mf = loaded
	}
	if strings.TrimSpace(mf.Output.Format) == "" {
		mf.Output.Format = "clash-meta"
	}
	if strings.TrimSpace(mf.Output.Path) == "" {
		mf.Output.Path = "./rules"
	}
	name := strings.TrimSpace(src.Name)
	if name == "" {
		name = manifest.DeriveSourceName(src.URL)
	}
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			return "", false, fmt.Errorf("规则源 %q 已存在", name)
		}
	}
	src.Name = name
	mf.Sources = append(mf.Sources, src)
	if err := mf.ValidateForWrite(); err != nil {
		return "", false, err
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// SetRuleSourceOutbound 设置某个远程规则源拉取时的出网选择。
// 机场规则源的出网跟随机场自身设置，不允许在清单里单独覆盖。
func (m *Manager) SetRuleSourceOutbound(ctx context.Context, name, outbound string) (string, error) {
	name = strings.TrimSpace(name)
	if isAirportRuleSource(name) {
		return "", fmt.Errorf("机场规则源的出网跟随机场设置，不能单独修改")
	}
	v, err := NormalizeOutboundNode(outbound)
	if err != nil {
		return "", err
	}
	if v != "" && v != "direct" {
		id, _ := strconv.ParseInt(v, 10, 64)
		if _, err := m.GetSelfNode(ctx, id); err != nil {
			return "", fmt.Errorf("自建节点 %d 不存在", id)
		}
	}
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", err
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	found := false
	for i := range mf.Sources {
		if mf.Sources[i].SourceName() == name {
			mf.Sources[i].OutboundNode = v
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("规则源 %q 不存在", name)
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		return "", err
	}
	return path, nil
}

// RemoveRuleSource 按名称删除规则源并写回 manifest。
// 不存在返回错误（消息含「不存在」）；删空返回错误（消息含「至少保留」）。
func (m *Manager) RemoveRuleSource(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if isAirportRuleSource(name) {
		id, ok := AirportIDFromRuleSourceName(name)
		if !ok {
			return "", fmt.Errorf("机场规则源名称不合法")
		}
		if _, gerr := m.GetAirport(ctx, id); gerr == nil {
			return "", fmt.Errorf("机场规则源由机场维护，不能单独删除")
		}
		// 机场已删除：源是历史快照，允许解绑后手动清理。
	}
	profiles, err := m.ListProfiles(ctx)
	if err != nil {
		return "", err
	}
	bound := make([]string, 0, 2)
	for _, p := range profiles {
		for _, used := range p.RuleSources {
			if used == name {
				bound = append(bound, p.Name)
				break
			}
		}
	}
	if len(bound) > 0 {
		return "", fmt.Errorf("规则源 %q 正被方案 %s 使用，请先在方案里解绑", name, strings.Join(bound, "、"))
	}
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", fmt.Errorf("规则源清单不存在，请先生成默认清单: %s", path)
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	kept := make([]manifest.Source, 0, len(mf.Sources))
	found := false
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			found = true
			continue
		}
		kept = append(kept, s)
	}
	if !found {
		return "", fmt.Errorf("规则源 %q 不存在", name)
	}
	if len(kept) == 0 {
		return "", fmt.Errorf("至少保留一个规则源")
	}
	mf.Sources = kept
	if err := mf.ValidateForWrite(); err != nil {
		return "", err
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		return "", err
	}
	return path, nil
}

// UpdateRuleSourceOne 只更新指定名称的规则源，返回该源的结果。
// name 不存在返回错误（消息含「不存在」）。
func (m *Manager) UpdateRuleSourceOne(ctx context.Context, name string) ([]manifest.UpdateResult, error) {
	name = strings.TrimSpace(name)
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return nil, err
	}
	dir, derr := m.ResolveRulesOutputDir(ctx)
	if derr == nil && dir != "" {
		mf.Output.Path = dir
	}
	target := make([]manifest.Source, 0, 1)
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			target = append(target, s)
		}
	}
	if len(target) == 0 {
		return nil, fmt.Errorf("规则源 %q 不存在", name)
	}
	mf.Sources = target
	results, uerr := mf.Update(ctx, m.manifestUpdateOptions()...)
	// 单源更新也要把该源的「当前版本」指向刚写出的新版本（与全量更新一致）。
	m.recordCurrentVersions(ctx, results)
	return results, uerr
}

// validateRuleSourceRenameName 校验可用于文件名的新名称。
func validateRuleSourceRenameName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("新名称不能为空")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return fmt.Errorf("新名称不能包含 / \\ : * ? \" < > |")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("新名称不合法")
	}
	return nil
}

// renameCurrentVersion 把当前版本指针从旧源名迁移到新源名，并同步版本文件名。
func renameCurrentVersion(dir, oldName, newName string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	currentVersionsMu.Lock()
	defer currentVersionsMu.Unlock()
	versions := loadCurrentVersions(dir)
	oldVersion, ok := versions[oldName]
	if !ok {
		return
	}
	delete(versions, oldName)
	if strings.HasPrefix(oldVersion, oldName+"-") {
		oldVersion = newName + strings.TrimPrefix(oldVersion, oldName)
	}
	versions[newName] = oldVersion
	_ = saveCurrentVersions(dir, versions)
}

// RenameRuleSource 重命名远程规则源，并迁移版本文件、当前版本指针和方案绑定。
//
// 机场生成的规则源名称跟随机场，不能在这里改名；请修改机场名称。
func (m *Manager) RenameRuleSource(ctx context.Context, oldName, newName string) (string, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" {
		return "", fmt.Errorf("原名称不能为空")
	}
	if isAirportRuleSource(oldName) {
		return "", fmt.Errorf("机场规则源名称跟随机场，不能单独改名；请修改机场名称")
	}
	if err := validateRuleSourceRenameName(newName); err != nil {
		return "", err
	}
	if oldName == newName {
		return "", fmt.Errorf("新名称不能与原名称相同")
	}
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", err
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	found := false
	for _, s := range mf.Sources {
		if s.SourceName() == newName {
			return "", fmt.Errorf("规则源 %q 已存在", newName)
		}
		if s.SourceName() == oldName {
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("规则源 %q 不存在", oldName)
	}

	ext := extForRuleSourceFormat(mf.Output.Format)
	dir, derr := m.ResolveRulesOutputDir(ctx)
	if derr != nil {
		dir = mf.Output.Path
	}
	if strings.TrimSpace(dir) != "" {
		versions, lerr := manifest.ListVersions(dir, oldName, ext)
		if lerr == nil {
			for _, oldFile := range versions {
				newFile := newName + strings.TrimPrefix(oldFile, oldName)
				if _, statErr := os.Stat(filepath.Join(dir, newFile)); statErr == nil {
					return "", fmt.Errorf("目标版本文件已存在: %s", newFile)
				}
				if rerr := os.Rename(filepath.Join(dir, oldFile), filepath.Join(dir, newFile)); rerr != nil {
					return "", fmt.Errorf("重命名版本文件 %s: %w", oldFile, rerr)
				}
			}
		}
		renameCurrentVersion(dir, oldName, newName)
	}

	for i := range mf.Sources {
		if mf.Sources[i].SourceName() == oldName {
			mf.Sources[i].Name = newName
		}
	}
	if err := mf.ValidateForWrite(); err != nil {
		return "", err
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		return "", err
	}

	profiles, err := m.ListProfiles(ctx)
	if err != nil {
		return path, err
	}
	for _, p := range profiles {
		changed := false
		next := make([]string, 0, len(p.RuleSources))
		for _, name := range p.RuleSources {
			if name == oldName {
				name = newName
				changed = true
			}
			next = append(next, name)
		}
		if changed {
			if err := m.SetProfileRuleSources(ctx, p.ID, next); err != nil {
				return path, fmt.Errorf("更新方案 %q 的规则源绑定: %w", p.Name, err)
			}
		}
	}
	return path, nil
}
