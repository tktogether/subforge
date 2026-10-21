package subforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

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

// ErrRollbackInsufficient 表示没有更早的版本可回滚（只有 1 版，或指针已指向首个版本）。
// 单独导出成哨兵错误，API 层可据此把它映射成 400 而不是 500。
var ErrRollbackInsufficient = errors.New("可回滚版本不足")

// CurrentVersionsFile 是「当前版本指针」状态文件名，落在规则输出目录下。
//
// 版本文件本身按时间升序排列（名字是 YYYYMMDD-HHMMSS），所以「最新」= 列表最后一个。
// 但回滚需要把「当前生效版本」钉在更早的版本上，且不能删任何版本文件（审计要求），
// 于是用这个状态文件记录 source → 版本文件名。指针缺失/损坏/指向已删除文件时
// 一律回退到最新版本，绝不报错——状态文件是显示层优化，不是正确性前提。
const CurrentVersionsFile = ".current-versions.json"

// currentVersionsMu 保护状态文件的读-改-写。规则源更新是低频操作（分钟级），
// 用一把包级互斥锁足够，且调用点串行更不容易出现半写状态。
var currentVersionsMu sync.Mutex

// currentVersionsPath 返回状态文件完整路径。
func currentVersionsPath(dir string) string {
	return filepath.Join(dir, CurrentVersionsFile)
}

// loadCurrentVersions 读取状态文件；不存在、内容损坏、结构不是 map 时返回空 map。
func loadCurrentVersions(dir string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(dir) == "" {
		return out
	}
	b, err := os.ReadFile(currentVersionsPath(dir))
	if err != nil {
		return out
	}
	var raw map[string]string
	if err := json.Unmarshal(b, &raw); err != nil || raw == nil {
		return out
	}
	for k, v := range raw {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// saveCurrentVersions 原子地写状态文件（先写临时文件再改名），避免读到半截 JSON。
func saveCurrentVersions(dir string, versions map[string]string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("规则输出目录为空，无法记录当前版本")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建规则输出目录 %s: %w", dir, err)
	}
	if versions == nil {
		versions = map[string]string{}
	}
	b, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化当前版本状态: %w", err)
	}
	b = append(b, '\n')
	path := currentVersionsPath(dir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("写当前版本状态 %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		// Windows 上目标文件被占用时 Rename 会失败，退回直写（内容很短，撕裂窗口可忽略）。
		if werr := os.WriteFile(path, b, 0o644); werr != nil {
			return fmt.Errorf("写当前版本状态 %s: %w", path, werr)
		}
	}
	return nil
}

// getCurrentVersion 返回某 source 记录的当前版本文件名；没有记录返回空串。
func getCurrentVersion(dir, name string) string {
	currentVersionsMu.Lock()
	defer currentVersionsMu.Unlock()
	return loadCurrentVersions(dir)[name]
}

// setCurrentVersion 把某 source 的当前版本设为 version（读-改-写，加锁）。
func setCurrentVersion(dir, name, version string) error {
	currentVersionsMu.Lock()
	defer currentVersionsMu.Unlock()
	versions := loadCurrentVersions(dir)
	versions[name] = version
	return saveCurrentVersions(dir, versions)
}

// pickCurrentVersion 计算「当前版本」：状态文件里的值（且必须真实存在于 versions 中）
// 优先，否则回退到 versions 最后一个（时间升序的最末 = 最新）。
func pickCurrentVersion(dir, name string, versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	if cur := getCurrentVersion(dir, name); cur != "" {
		for _, v := range versions {
			if v == cur {
				return cur
			}
		}
	}
	return versions[len(versions)-1]
}

// recordCurrentVersions 把一批更新结果里成功的源记为「当前版本」。
//
// 失败的源不动指针（它沿用上一版）；状态写失败也不让更新失败——版本文件已经落盘，
// 指针只是显示/回滚时的锚点，下一次成功更新会再写一次。
func (m *Manager) recordCurrentVersions(ctx context.Context, results []manifest.UpdateResult) {
	if len(results) == 0 {
		return
	}
	dir, err := m.ResolveRulesOutputDir(ctx)
	if err != nil {
		dir = ""
	}
	for _, res := range results {
		if res.Err != nil || strings.TrimSpace(res.Path) == "" {
			continue
		}
		target := dir
		if strings.TrimSpace(target) == "" {
			target = filepath.Dir(res.Path)
		}
		name := res.Source
		if strings.TrimSpace(name) == "" {
			continue
		}
		_ = setCurrentVersion(target, name, filepath.Base(res.Path))
	}
}

// ListRuleSources 列出每个源的版本状态。manifest 不存在时返回空切片 + nil error。
// Latest 是「当前版本」：状态文件指针优先，否则最新版本。
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
		status.Latest = pickCurrentVersion(dir, name, versions)
		out = append(out, status)
	}
	return out, nil
}

// RuleSourceRules 读取给定规则源名们「当前版本」产物，并把产物解析成订阅用的规则行。
// 它永远不返回 error：任何失败都降级为一条 warning，并在 rules 中跳过该源。
// rules 按 names 顺序拼接（每个源内按产物顺序），warnings 同样按 names 顺序。
func (m *Manager) RuleSourceRules(ctx context.Context, names []string) (rules []string, warnings []string) {
	// 1) trim + 去重保序，丢弃空串；全空 -> 直接返回。
	ordered := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		ordered = append(ordered, n)
	}
	if len(ordered) == 0 {
		return nil, nil
	}

	// 2) 读取全部规则源清单；清单读取失败整体降级。
	sources, err := m.ListRuleSources(ctx)
	if err != nil {
		return nil, []string{fmt.Sprintf("读取规则源清单失败：%v", err)}
	}

	// 3) 建 name -> RuleSourceStatus 映射。
	byName := make(map[string]RuleSourceStatus, len(sources))
	for _, st := range sources {
		byName[st.Name] = st
	}

	// 4~5) 逐名解析产物。
	for _, name := range ordered {
		status, ok := byName[name]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("规则源 %s 不存在，已跳过", name))
			continue
		}
		if strings.TrimSpace(status.Latest) == "" {
			warnings = append(warnings, fmt.Sprintf("规则源 %s 还没有产物（先在规则源页执行更新），已跳过", name))
			continue
		}

		versionPath := filepath.Join(status.Dir, status.Latest)
		b, rerr := os.ReadFile(versionPath)
		if rerr != nil {
			warnings = append(warnings, fmt.Sprintf("读取规则源 %s 的产物失败：%v，已跳过", name, rerr))
			continue
		}

		ext := strings.ToLower(filepath.Ext(status.Latest))
		var lines []string
		switch ext {
		case ".yaml", ".yml":
			var doc struct {
				Rules []string `yaml:"rules"`
			}
			if yerr := yaml.Unmarshal(b, &doc); yerr != nil || len(doc.Rules) == 0 {
				warnings = append(warnings, fmt.Sprintf("规则源 %s 的产物里没有 rules 列表，已跳过", name))
				continue
			}
			lines = doc.Rules
		case ".txt", ".text":
			lines = strings.Split(string(b), "\n")
		default:
			warnings = append(warnings, fmt.Sprintf("规则源 %s 的产物格式 %s 不支持内联到订阅，已跳过（请把规则源输出格式设为 yaml 或 text）", name, ext))
			continue
		}

		// 5) 逐行清洗；统计并丢弃 FINAL/MATCH 兜底规则。
		kept := make([]string, 0, len(lines))
		dropped := 0
		for _, raw := range lines {
			rule := strings.TrimSpace(raw)
			if rule == "" {
				continue
			}
			if ext == ".txt" || ext == ".text" {
				if strings.HasPrefix(rule, "#") {
					continue
				}
			}
			head := strings.ToUpper(strings.TrimSpace(strings.SplitN(rule, ",", 2)[0]))
			if head == "FINAL" || head == "MATCH" {
				dropped++
				continue
			}
			kept = append(kept, rule)
		}
		if dropped > 0 {
			warnings = append(warnings, fmt.Sprintf("规则源 %s 含 %d 条 FINAL/MATCH 兜底规则，已忽略", name, dropped))
		}
		rules = append(rules, kept...)
	}

	return rules, warnings
}

// UpdateRuleSources 执行一次全量更新，返回每个源的结果。
// 成功写出的源会把「当前版本」指针指向刚生成的新版本。
func (m *Manager) UpdateRuleSources(ctx context.Context) ([]manifest.UpdateResult, error) {
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return nil, err
	}
	dir, err := m.ResolveRulesOutputDir(ctx)
	if err == nil && dir != "" {
		mf.Output.Path = dir
	}
	results, uerr := mf.Update(ctx, manifest.WithRetention(manifest.DefaultRetention))
	m.recordCurrentVersions(ctx, results)
	return results, uerr
}

// RollbackRuleSource 把指定源的「当前版本」切到升序列表里的前一个版本，返回该版本完整路径。
// 不删除任何版本文件；只有 1 版（或已指向首个版本）时返回可回滚版本不足。
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
	versions, verr := manifest.ListVersions(dir, name, ext)
	if verr != nil {
		versions = nil
	}
	cur := pickCurrentVersion(dir, name, versions)
	idx := -1
	for i, v := range versions {
		if v == cur {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return "", fmt.Errorf("规则源 %s %w（当前版本 %q，共 %d 个版本）", name, ErrRollbackInsufficient, cur, len(versions))
	}
	prev := versions[idx-1]
	if err := setCurrentVersion(dir, name, prev); err != nil {
		return "", fmt.Errorf("回滚规则源 %s: %w", name, err)
	}
	return filepath.Join(dir, prev), nil
}
