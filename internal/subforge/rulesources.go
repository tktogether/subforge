package subforge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tktogether/subforge/internal/manifest"
)

const RuleManifestPathKey = "rules.manifest_path"
const RulesOutputDirKey = "rules.output_dir"

// RuleSourceStatus 描述一个规则源当前的版本状态（供 /api/rule-sources 渲染）。
type RuleSourceStatus struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Format   string   `json:"format"`
	Dir      string   `json:"dir"`
	Versions []string `json:"versions"`
	Latest   string   `json:"latest"`
}

// extForRuleSourceFormat 根据输出格式推导文件扩展名（manifest.extForFormat 未导出，此处等价复刻）。
func extForRuleSourceFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "clash-json", "sing-box", "json":
		return "json"
	case "mrs":
		return "mrs"
	case "text", "txt", "plain":
		return "txt"
	default:
		return "yaml"
	}
}

// GetRuleManifestPath 读取 rules.manifest_path；未配置返回空串（不报错）。
func (m *Manager) GetRuleManifestPath(ctx context.Context) (string, error) {
	return m.getConfig(ctx, RuleManifestPathKey)
}

// SetRuleManifestPath 写入 rules.manifest_path。
func (m *Manager) SetRuleManifestPath(ctx context.Context, path string) error {
	return m.setConfig(ctx, RuleManifestPathKey, path)
}

// LoadRuleManifest 读 manifest 文件；路径为空时报错。
func (m *Manager) LoadRuleManifest(ctx context.Context) (manifest.Manifest, error) {
	path, err := m.getConfig(ctx, RuleManifestPathKey)
	if err != nil {
		return manifest.Manifest{}, err
	}
	if strings.TrimSpace(path) == "" {
		return manifest.Manifest{}, errors.New("未配置规则源清单路径（rules.manifest_path）")
	}
	mf, err := manifest.LoadManifest(path)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	return mf, nil
}

// ListRuleSources 列出每个源的版本状态。ListVersions 报错时该源 Versions 置空并继续。
func (m *Manager) ListRuleSources(ctx context.Context) ([]RuleSourceStatus, error) {
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return nil, err
	}
	ext := extForRuleSourceFormat(mf.Output.Format)
	dir := mf.Output.Path
	out := make([]RuleSourceStatus, 0, len(mf.Sources))
	for _, s := range mf.Sources {
		name := s.SourceName()
		versions, err := manifest.ListVersions(dir, name, ext)
		if err != nil {
			versions = []string{}
		}
		if versions == nil {
			versions = []string{}
		}
		status := RuleSourceStatus{
			Name:     name,
			URL:      s.URL,
			Format:   mf.Output.Format,
			Dir:      dir,
			Versions: versions,
		}
		if len(versions) > 0 {
			status.Latest = versions[0]
		}
		out = append(out, status)
	}
	return out, nil
}

// UpdateRuleSources 执行一次全量更新，返回每个源的结果。
func (m *Manager) UpdateRuleSources(ctx context.Context) ([]manifest.UpdateResult, error) {
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return nil, err
	}
	return mf.Update(ctx, manifest.WithRetention(manifest.DefaultRetention))
}

// RollbackRuleSource 把指定源回滚到上一个版本，返回回滚后的文件路径。
// name 不存在时返回错误（错误消息里含该 name）。
func (m *Manager) RollbackRuleSource(ctx context.Context, name string) (string, error) {
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return "", err
	}
	ext := extForRuleSourceFormat(mf.Output.Format)
	dir := mf.Output.Path
	found := false
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("规则源 %s 不存在", name)
	}
	path, err := manifest.Rollback(dir, name, ext)
	if err != nil {
		return "", fmt.Errorf("回滚规则源 %s: %w", name, err)
	}
	return path, nil
}
