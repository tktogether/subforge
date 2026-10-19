package subforge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tktogether/subforge/internal/manifest"
)

// AddRuleSource 追加一个规则源并写回 manifest，返回 (清单路径, 是否新建, 错误)。
// name 为空时从 URL 推导；重名返回错误（消息含「已存在」）。
func (m *Manager) AddRuleSource(ctx context.Context, src manifest.Source) (string, bool, error) {
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", false, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", false, fmt.Errorf("规则源清单不存在，请先生成默认清单: %s", path)
	}
	if strings.TrimSpace(src.URL) == "" {
		return "", false, fmt.Errorf("url 不能为空")
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", false, fmt.Errorf("读取规则源清单 %s: %w", path, err)
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

// RemoveRuleSource 按名称删除规则源并写回 manifest。
// 不存在返回错误（消息含「不存在」）；删空返回错误（消息含「至少保留」）。
func (m *Manager) RemoveRuleSource(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
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
	return mf.Update(ctx, manifest.WithRetention(manifest.DefaultRetention))
}
