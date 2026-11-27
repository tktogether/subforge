package subforge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tktogether/subforge/internal/manifest"
	"github.com/tktogether/subforge/internal/store"
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
			// 运行期按 TrimSpace 后的名字解析绑定；删除保护必须同口径，
			// 否则 " base " 这类仍有效的绑定会绕过检查，删完留下悬空引用。
			if strings.TrimSpace(used) == name {
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
	// 删除后清理方案里指向该源的最终组覆盖键：键本身是名字，源不存在后
	// 会一直以「无效配置」形态留在界面上。
	var updates []store.ProfileBindingUpdate
	for _, p := range profiles {
		if _, ok := p.FinalGroup.Overrides["rule:"+name]; !ok {
			continue
		}
		fg := p.FinalGroup
		delete(fg.Overrides, "rule:"+name)
		updates = append(updates, store.ProfileBindingUpdate{ID: p.ID, RuleSources: p.RuleSources, FinalGroup: fg})
	}
	oldManifest, oldErr := os.ReadFile(path)
	if err := manifest.SaveManifest(path, mf); err != nil {
		// 保存可能已经截断目标文件：有原始字节就恢复，恢复失败一并上报。
		var cleanup []error
		if oldErr != nil {
			cleanup = append(cleanup, fmt.Errorf("读取原始清单失败，未能恢复: %w", oldErr))
		} else if werr := os.WriteFile(path, oldManifest, 0o644); werr != nil {
			cleanup = append(cleanup, fmt.Errorf("恢复规则源清单 %s: %w", path, werr))
		}
		return "", joinRuleSourceRenameInternal(fmt.Errorf("写规则源清单 %s: %w", path, err), cleanup)
	}
	if len(updates) > 0 {
		if err := m.db.UpdateProfileBindings(ctx, updates); err != nil {
			var cleanup []error
			if oldErr != nil {
				cleanup = append(cleanup, fmt.Errorf("读取原始清单失败，未能恢复: %w", oldErr))
			} else if werr := os.WriteFile(path, oldManifest, 0o644); werr != nil {
				cleanup = append(cleanup, fmt.Errorf("恢复规则源清单 %s: %w", path, werr))
			}
			return "", joinRuleSourceRenameInternal(fmt.Errorf("清理方案最终组覆盖: %w", err), cleanup)
		}
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
// 长度与尾随字符在改文件前拦截，避免最终由 os.Stat/os.Rename 报成 500。
func validateRuleSourceRenameName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("新名称不能为空")
	}
	if utf8.RuneCountInString(name) > 100 {
		return fmt.Errorf("新名称不能超过 100 个字符")
	}
	// 文件名长度按字节算：多字节字符即使 rune 数合规，也可能超过 NAME_MAX。
	// 预留 "-<时间戳>.<ext>" 的开销后取 200 字节。
	if len(name) > 200 {
		return fmt.Errorf("新名称过长（最多 200 字节，多字节字符按字节计）")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return fmt.Errorf("新名称不能包含 / \\ : * ? \" < > |")
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("新名称不能以点或空格结尾")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("新名称不能包含控制字符")
		}
	}
	if name == "." || name == ".." {
		return fmt.Errorf("新名称不合法")
	}
	return nil
}

// ErrRuleSourceRenameInternal 标记规则源改名过程中的服务端失败（文件/清单/数据库）。
// API 据此返回 500；名称、参数类错误保持 400。
var ErrRuleSourceRenameInternal = errors.New("规则源改名内部失败")

// renameCurrentVersion 把当前版本指针从旧源名迁移到新源名，并同步版本文件名。
// 指针文件损坏或写入失败时返回错误，由改名流程回滚已改的文件。
func renameCurrentVersion(dir, oldName, newName string) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	currentVersionsMu.Lock()
	defer currentVersionsMu.Unlock()
	versions, err := loadCurrentVersionsStrict(dir)
	if err != nil {
		return err
	}
	oldVersion, ok := versions[oldName]
	if !ok {
		return nil
	}
	// 指针值必须真是旧源的版本文件；只按名字前缀判断会把 base-extra-... 当成 base 的版本。
	if !manifest.IsVersionFileName(oldName, oldVersion) {
		return fmt.Errorf("当前版本指针 %q 不属于规则源 %q", oldVersion, oldName)
	}
	delete(versions, oldName)
	newVersion := newName + strings.TrimPrefix(oldVersion, oldName)
	if existing, exists := versions[newName]; exists && existing != newVersion {
		return fmt.Errorf("当前版本指针已存在规则源 %q 的记录（%s），请先清理后再改名", newName, existing)
	}
	versions[newName] = newVersion
	return saveCurrentVersions(dir, versions)
}

// versionRename 是一个待迁移的版本文件。
type versionRename struct {
	oldPath string
	newPath string
	oldName string
}

// withCleanup 把回滚失败项附到原始错误后面；没有失败项时原样返回。
func withCleanup(cause error, cleanup []error) error {
	if len(cleanup) == 0 {
		return cause
	}
	parts := make([]string, 0, len(cleanup))
	for _, item := range cleanup {
		parts = append(parts, item.Error())
	}
	return fmt.Errorf("%w（回滚未完全成功，需手工检查：%s）", cause, strings.Join(parts, "；"))
}

// renameVersionFiles 逐个改名；中途失败时把已经改过的文件反向改回。
// 回滚失败项随错误一起返回，不能被丢弃——否则调用方看不到磁盘停在了中间态。
func renameVersionFiles(renames []versionRename) error {
	done := make([]versionRename, 0, len(renames))
	for _, r := range renames {
		if err := os.Rename(r.oldPath, r.newPath); err != nil {
			return withCleanup(fmt.Errorf("重命名版本文件 %s: %w", r.oldName, err),
				rollbackVersionFiles(done))
		}
		done = append(done, r)
	}
	return nil
}

// rollbackVersionFiles 反向改回已迁移的版本文件（尽力而为，返回失败项）。
func rollbackVersionFiles(renames []versionRename) []error {
	var errs []error
	for i := len(renames) - 1; i >= 0; i-- {
		if err := os.Rename(renames[i].newPath, renames[i].oldPath); err != nil {
			errs = append(errs, fmt.Errorf("恢复版本文件 %s: %w", renames[i].oldName, err))
		}
	}
	return errs
}

// joinRuleSourceRenameInternal 把内部失败包装成可识别的错误，并附上回滚失败项。
// 回滚失败意味着磁盘可能停在中间态，错误文本必须提示人工检查。
func joinRuleSourceRenameInternal(cause error, cleanup []error) error {
	return fmt.Errorf("%w: %w", ErrRuleSourceRenameInternal, withCleanup(cause, cleanup))
}

// RenameRuleSource 重命名远程规则源，并迁移版本文件、当前版本指针和方案绑定。
//
// 机场生成的规则源名称跟随机场，不能在这里改名；请修改机场名称。
//
// 失败顺序被刻意约束：所有预检（版本目录、方案解析、覆盖键冲突、清单校验）
// 都在改任何文件之前完成；文件阶段与清单写入失败会反向恢复；方案绑定走单事务。
// 文件系统与 SQLite 无法共用一个事务，因此仍保留尽力回滚，并在回滚失败时
// 在错误文本里列出需要人工检查的项目。
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
		return "", joinRuleSourceRenameInternal(err, nil)
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", joinRuleSourceRenameInternal(fmt.Errorf("读取规则源清单 %s: %w", path, err), nil)
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
	dir = strings.TrimSpace(dir)

	// 1) 预检版本文件：目录不可读必须失败，不能静默跳过。
	var renames []versionRename
	if dir != "" {
		versions, lerr := manifest.ListVersions(dir, oldName, ext)
		switch {
		case lerr == nil:
			for _, oldFile := range versions {
				newFile := newName + strings.TrimPrefix(oldFile, oldName)
				newPath := filepath.Join(dir, newFile)
				if _, statErr := os.Stat(newPath); statErr == nil {
					return "", fmt.Errorf("目标版本文件已存在: %s", newFile)
				} else if !errors.Is(statErr, fs.ErrNotExist) {
					return "", joinRuleSourceRenameInternal(fmt.Errorf("检查目标版本文件 %s: %w", newFile, statErr), nil)
				}
				renames = append(renames, versionRename{
					oldPath: filepath.Join(dir, oldFile),
					newPath: newPath,
					oldName: oldFile,
				})
			}
		case errors.Is(lerr, fs.ErrNotExist):
			// 输出目录尚不存在（规则源从未更新过）：没有版本文件要迁移。
		default:
			return "", joinRuleSourceRenameInternal(fmt.Errorf("读取版本目录 %s: %w", dir, lerr), nil)
		}
	}

	// 2) 预检方案绑定与覆盖键冲突，全部在改文件之前完成。
	profiles, err := m.ListProfiles(ctx)
	if err != nil {
		return "", joinRuleSourceRenameInternal(fmt.Errorf("读取方案: %w", err), nil)
	}
	var updates []store.ProfileBindingUpdate
	for _, p := range profiles {
		next := make([]string, 0, len(p.RuleSources))
		changed := false
		for _, name := range p.RuleSources {
			if strings.TrimSpace(name) == oldName {
				name = newName
				changed = true
			}
			next = append(next, name)
		}
		fg := p.FinalGroup
		if names, ok := fg.Overrides["rule:"+oldName]; ok {
			if existing, exists := fg.Overrides["rule:"+newName]; exists && !slices.Equal(existing, names) {
				return "", fmt.Errorf("方案 %q 已有 rule:%s 的覆盖配置（%s），请先处理后再改名",
					p.Name, newName, strings.Join(existing, "、"))
			}
			delete(fg.Overrides, "rule:"+oldName)
			fg.Overrides["rule:"+newName] = names
			changed = true
		}
		if changed {
			updates = append(updates, store.ProfileBindingUpdate{ID: p.ID, RuleSources: next, FinalGroup: fg})
		}
	}

	// 3) 修改清单对象并校验；读取原始字节供失败恢复使用。
	for i := range mf.Sources {
		if mf.Sources[i].SourceName() == oldName {
			mf.Sources[i].Name = newName
		}
	}
	if err := mf.ValidateForWrite(); err != nil {
		return "", err
	}
	oldManifest, err := os.ReadFile(path)
	if err != nil {
		return "", joinRuleSourceRenameInternal(fmt.Errorf("读取规则源清单 %s: %w", path, err), nil)
	}

	// 4) 文件阶段：版本文件 + 当前版本指针。任一失败都反向恢复。
	if err := renameVersionFiles(renames); err != nil {
		return "", joinRuleSourceRenameInternal(err, nil)
	}
	pointerRenamed := false
	if dir != "" {
		if err := renameCurrentVersion(dir, oldName, newName); err != nil {
			return "", joinRuleSourceRenameInternal(
				fmt.Errorf("迁移当前版本指针: %w", err), rollbackVersionFiles(renames))
		}
		pointerRenamed = true
	}

	// 5) 写清单。失败时恢复指针、版本文件与可能被截断的清单。
	if err := manifest.SaveManifest(path, mf); err != nil {
		var cleanup []error
		if werr := os.WriteFile(path, oldManifest, 0o644); werr != nil {
			cleanup = append(cleanup, fmt.Errorf("恢复规则源清单 %s: %w", path, werr))
		}
		if pointerRenamed {
			if rerr := renameCurrentVersion(dir, newName, oldName); rerr != nil {
				cleanup = append(cleanup, fmt.Errorf("恢复当前版本指针: %w", rerr))
			}
		}
		cleanup = append(cleanup, rollbackVersionFiles(renames)...)
		return "", joinRuleSourceRenameInternal(fmt.Errorf("写规则源清单 %s: %w", path, err), cleanup)
	}

	// 6) 方案绑定单事务。失败时恢复清单、指针与版本文件。
	if err := m.db.UpdateProfileBindings(ctx, updates); err != nil {
		var cleanup []error
		if werr := os.WriteFile(path, oldManifest, 0o644); werr != nil {
			cleanup = append(cleanup, fmt.Errorf("恢复规则源清单 %s: %w", path, werr))
		}
		if pointerRenamed {
			if rerr := renameCurrentVersion(dir, newName, oldName); rerr != nil {
				cleanup = append(cleanup, fmt.Errorf("恢复当前版本指针: %w", rerr))
			}
		}
		cleanup = append(cleanup, rollbackVersionFiles(renames)...)
		return "", joinRuleSourceRenameInternal(fmt.Errorf("更新方案绑定: %w", err), cleanup)
	}
	return path, nil
}
