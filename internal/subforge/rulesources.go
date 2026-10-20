package subforge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tktogether/subforge/internal/manifest"
)

// ErrRuleManifestMissing 表示规则源清单文件不存在（区别于读取/解析失败）。
var ErrRuleManifestMissing = errors.New("规则源清单文件不存在")

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

// baseDir 返回数据根目录；为空时落 cwd。
func (m *Manager) baseDir() (string, error) {
	if strings.TrimSpace(m.dataDir) != "" {
		return m.dataDir, nil
	}
	return os.Getwd()
}

// DefaultRuleManifestPath 返回不受配置影响的默认清单路径：<baseDir>/manifest.yaml。
// 模板里的 output.path: ./rules 是相对路径，会相对清单自身所在目录解析，
// 所以默认布局下规则集版本落到 <baseDir>/rules，不会出现 rules/rules。
func (m *Manager) DefaultRuleManifestPath(ctx context.Context) (string, error) {
	bd, err := m.baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bd, "manifest.yaml"), nil
}

// ResolveRuleManifestPath 解析规则源清单路径：配置优先 → 默认。
func (m *Manager) ResolveRuleManifestPath(ctx context.Context) (string, error) {
	v, err := m.getConfig(ctx, RuleManifestPathKey)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(v) != "" {
		return v, nil
	}
	return m.DefaultRuleManifestPath(ctx)
}

// ResolveRulesOutputDir 解析规则源输出目录：配置优先 → manifest output.path → 默认。
func (m *Manager) ResolveRulesOutputDir(ctx context.Context) (string, error) {
	v, err := m.getConfig(ctx, RulesOutputDirKey)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(v) != "" {
		return v, nil
	}
	mf, err := m.LoadRuleManifest(ctx)
	if err == nil && strings.TrimSpace(mf.Output.Path) != "" {
		return mf.Output.Path, nil
	}
	bd, err := m.baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(bd, "rules"), nil
}

// RuleManifestTemplate 返回可直接落盘的 manifest YAML 模板（包级函数）。
func RuleManifestTemplate() string {
	return "# SubForge 规则源清单\n" +
		"# 每个 source 会被抓取并落盘为 <name>-<UTC 时间戳>.<ext>，默认保留最近 5 个版本。\n" +
		"sources:\n" +
		"  - name: reject\n" +
		// meta-rules-dat 没有 geoip/reject.yaml（该路径 404），广告/拦截域名清单在
		// geosite/category-ads-all.yaml 下。默认清单是全仓库唯一被"开箱即用"的地方，
		// 指向一个不存在的地址等于每次更新都留一条假失败。
		"    url: https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/category-ads-all.yaml\n" +
		"    format: clash-meta\n" +
		"  - name: private\n" +
		"    url: https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/private.yaml\n" +
		"    format: clash-meta\n" +
		"output:\n" +
		"  format: clash-meta\n" +
		"  path: ./rules\n"
}

// EnsureRuleManifest 不存在时写入默认清单模板；force 时先备份 <path>.bak 再覆盖。
func (m *Manager) EnsureRuleManifest(ctx context.Context, force bool) (string, bool, error) {
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", false, err
	}
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if exists && !force {
		return path, false, nil
	}
	if exists && force {
		oldBytes, rerr := os.ReadFile(path)
		if rerr == nil {
			_ = os.WriteFile(path+".bak", oldBytes, 0o644)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, err
	}
	if err := os.WriteFile(path, []byte(RuleManifestTemplate()), 0o644); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// LoadRuleManifest 读 manifest 文件；路径不存在时返回 ErrRuleManifestMissing。
func (m *Manager) LoadRuleManifest(ctx context.Context) (manifest.Manifest, error) {
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return manifest.Manifest{}, err
	}
	mf, err := manifest.LoadManifest(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return manifest.Manifest{}, fmt.Errorf("%w: %s", ErrRuleManifestMissing, path)
		}
		return manifest.Manifest{}, fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	return mf, nil
}

// ListRuleSources 列出每个源的版本状态。manifest 不存在时返回空切片 + nil error。
func (m *Manager) ListRuleSources(ctx context.Context) ([]RuleSourceStatus, error) {
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		if errors.Is(err, ErrRuleManifestMissing) {
			return []RuleSourceStatus{}, nil
		}
		return nil, err
	}
	ext := extForRuleSourceFormat(mf.Output.Format)
	dir, err := m.ResolveRulesOutputDir(ctx)
	if err != nil {
		dir = mf.Output.Path
	}
	out := make([]RuleSourceStatus, 0, len(mf.Sources))
	for _, s := range mf.Sources {
		name := s.SourceName()
		versions, verr := manifest.ListVersions(dir, name, ext)
		if verr != nil {
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
	dir, err := m.ResolveRulesOutputDir(ctx)
	if err == nil && dir != "" {
		mf.Output.Path = dir
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
	dir, err := m.ResolveRulesOutputDir(ctx)
	if err != nil {
		dir = mf.Output.Path
	}
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
