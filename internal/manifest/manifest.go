// Package manifest 实现 M3.1 规则工厂的 manifest 聚合：
// 多上游规则集定时拉取 + 版本管理 + 回滚容错。
package manifest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/ini_rules"
)

// DefaultRetention 保留的最近版本数。
const DefaultRetention = 5

// Manifest 是 manifest.yaml 的结构。
type Manifest struct {
	Sources []Source `yaml:"sources"`
	Output  Output   `yaml:"output"`
}

// Source 是一个上游规则集来源。
type Source struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	Type           string `yaml:"type"`
	UpdateInterval string `yaml:"update_interval"`
	InputFormat    string `yaml:"input_format"`
	// OutboundNode 是该源拉取时的出网选择：空=跟随默认，direct=直连，数字=自建节点 ID。
	OutboundNode string `yaml:"outbound_node,omitempty"`
}

// Output 是输出配置。
type Output struct {
	Format string `yaml:"format"`
	Path   string `yaml:"path"`
}

// LoadManifest 读取并解析 manifest YAML 文件。
func LoadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("读取 manifest 文件 %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("解析 manifest YAML: %w", err)
	}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	m.applyDefaults()
	// 相对 output.path 相对 manifest 文件自身解析成绝对路径，避免 Update 写与 ResolveRulesOutputDir 读落在进程 cwd。
	if abs, err := filepath.Abs(path); err == nil {
		if !filepath.IsAbs(m.Output.Path) {
			m.Output.Path = filepath.Clean(filepath.Join(filepath.Dir(abs), m.Output.Path))
		}
	}
	return m, nil
}

func (m *Manifest) validate() error {
	if len(m.Sources) == 0 {
		return fmt.Errorf("manifest 至少需要一个 source")
	}
	seen := map[string]bool{}
	for i, s := range m.Sources {
		if strings.TrimSpace(s.URL) == "" {
			return fmt.Errorf("source #%d 缺少 url", i+1)
		}
		name := s.Name
		if name == "" {
			name = urlBasename(s.URL)
		}
		if seen[name] {
			return fmt.Errorf("source 名称重复 %q", name)
		}
		seen[name] = true
	}
	return nil
}

func (m *Manifest) applyDefaults() {
	if m.Output.Format == "" {
		m.Output.Format = "clash-meta"
	}
	if m.Output.Path == "" {
		m.Output.Path = "./rules"
	}
	for i := range m.Sources {
		if m.Sources[i].Name == "" {
			m.Sources[i].Name = urlBasename(m.Sources[i].URL)
		}
		if m.Sources[i].Type == "" {
			m.Sources[i].Type = "clash"
		}
	}
}

// SourceName 返回 source 的有效名称。
func (s Source) SourceName() string {
	if s.Name != "" {
		return s.Name
	}
	return urlBasename(s.URL)
}

// extForFormat 根据输出格式返回文件扩展名。
func extForFormat(format string) string {
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

// shouldExpandINI 判断某 source 是否按 INI 规则源处理。
// input_format 取值：auto（缺省/空）/ ini；auto 时嗅探内容。
func shouldExpandINI(inputFormat string, data []byte) bool {
	v := strings.ToLower(strings.TrimSpace(inputFormat))
	switch v {
	case "ini":
		return true
	case "auto", "":
		return ini_rules.IsINI(data)
	default:
		return false
	}
}

// urlBasename 从 URL 提取末段作为名称。
func urlBasename(u string) string {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	if u == "" {
		return "source"
	}
	if i := strings.LastIndex(u, "."); i >= 0 {
		u = u[:i]
	}
	return u
}

// UpdateResult 记录一次更新的结果。
type UpdateResult struct {
	Source    string `yaml:"source"`
	Version   string `yaml:"version"`
	Path      string `yaml:"path"`
	FromCache bool   `yaml:"from_cache"`
	Err       error  `yaml:"-"`
}

// Update 拉取所有 source 并写入版本化文件。
// 并发度 <= 4（由 fetch.Fetcher 控制）。失败时保留上一版并返回错误。
func (m Manifest) Update(ctx context.Context, opts ...UpdateOption) ([]UpdateResult, error) {
	o := updateOptions{retention: DefaultRetention}
	for _, fn := range opts {
		fn(&o)
	}

	if err := os.MkdirAll(m.Output.Path, 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录 %s: %w", m.Output.Path, err)
	}

	cfg := fetch.DefaultConfig()
	cfg.Parallel = 4
	cfg.Timeout = 60 * time.Second
	cfg.Retries = 3
	fetcher := fetch.New(fetch.WithConfig(cfg))
	if o.clientFor != nil {
		fetcher.ClientFor = o.clientFor
	}
	sources := make([]fetch.Source, len(m.Sources))
	for i, s := range m.Sources {
		sources[i] = fetch.Source{Name: s.SourceName(), URL: s.URL, Outbound: s.OutboundNode}
	}
	results, err := fetcher.Fetch(ctx, sources)
	if err != nil && len(results) == 0 {
		return nil, err
	}

	ext := extForFormat(m.Output.Format)
	out := make([]UpdateResult, len(m.Sources))
	var wg sync.WaitGroup
	for i := range m.Sources {
		wg.Add(1)
		go func(i int, res fetch.Result) {
			defer wg.Done()
			src := m.Sources[i]
			name := src.SourceName()
			r := UpdateResult{Source: name}
			// 容忍「嗅探失败但原文非空」仅限真正走 INI 展开的源；其它源（如 200+HTML 错误页）照旧报错，
			// 防止垃圾内容被当作规则落盘。
			if res.Err != nil && !(len(res.Content) > 0 && shouldExpandINI(src.InputFormat, res.Content)) {
				r.Err = res.Err
				out[i] = r
				return
			}
			version := time.Now().UTC().Format("20060102-150405")
			filename := fmt.Sprintf("%s-%s.%s", name, version, ext)
			fullPath := filepath.Join(m.Output.Path, filename)

			// INI 规则源：input_format=ini 或 auto 命中嗅探 → 展开 → 按 output.format 落盘。
			content := res.Content
			if shouldExpandINI(src.InputFormat, res.Content) {
				doc, err := ini_rules.Parse(res.Content)
				if err != nil {
					r.Err = fmt.Errorf("解析 INI 规则源 %s: %w", name, err)
					out[i] = r
					return
				}
				rules, err := ini_rules.Expand(ctx, doc, ini_rules.NewRuleFetcher(fetcher))
				if err != nil {
					r.Err = fmt.Errorf("展开 INI 规则源 %s: %w", name, err)
					out[i] = r
					return
				}
				if strings.ToLower(strings.TrimSpace(m.Output.Format)) == "text" ||
					strings.ToLower(strings.TrimSpace(m.Output.Format)) == "txt" ||
					strings.ToLower(strings.TrimSpace(m.Output.Format)) == "plain" {
					content = ini_rules.FormatRulesText(rules)
				} else {
					content = ini_rules.FormatRulesYAML(rules)
				}
			}
			if err := os.WriteFile(fullPath, content, 0o644); err != nil {
				r.Err = fmt.Errorf("写入 %s: %w", fullPath, err)
				out[i] = r
				return
			}
			r.Version = version
			r.Path = fullPath
			r.FromCache = res.FromCache
			out[i] = r
			pruneVersions(m.Output.Path, name, ext, o.retention)
		}(i, results[i])
	}
	wg.Wait()

	allFailed := true
	for _, r := range out {
		if r.Err == nil {
			allFailed = false
			break
		}
	}
	if allFailed && len(out) > 0 {
		return out, fmt.Errorf("所有 %d 个 source 更新失败", len(out))
	}
	return out, nil
}

type updateOptions struct {
	retention int
	clientFor func(ctx context.Context, outbound string) (*http.Client, error)
}

// UpdateOption 配置 Update 行为。
type UpdateOption func(*updateOptions)

// WithRetention 设置保留的最近版本数。
func WithRetention(n int) UpdateOption {
	return func(o *updateOptions) {
		if n > 0 {
			o.retention = n
		}
	}
}

// WithClientFor 按来源的出网选择返回 HTTP 客户端（控制台注入节点/直连切换）。
func WithClientFor(f func(ctx context.Context, outbound string) (*http.Client, error)) UpdateOption {
	return func(o *updateOptions) {
		o.clientFor = f
	}
}

// pruneVersions 保留最近 retention 个版本文件，删除其余。
// SaveProduct 把调用方已生成的内容写成 source 的一个新版本。
// 与 Update 共用版本命名、保留策略；当前版本指针由调用方写（subforge 层负责）。
func SaveProduct(dir, name, ext string, content []byte, retention int) (UpdateResult, error) {
	if retention <= 0 {
		retention = DefaultRetention
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return UpdateResult{}, fmt.Errorf("创建规则输出目录 %s: %w", dir, err)
	}
	version := time.Now().UTC().Format("20060102-150405.000000000")
	filename := fmt.Sprintf("%s-%s.%s", name, version, ext)
	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		return UpdateResult{}, fmt.Errorf("写入 %s: %w", fullPath, err)
	}
	pruneVersions(dir, name, ext, retention)
	return UpdateResult{Source: name, Version: version, Path: fullPath}, nil
}

func pruneVersions(dir, name, ext string, retention int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var versions []string
	prefix := name + "-"
	suffix := "." + ext
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, prefix) || !strings.HasSuffix(n, suffix) {
			continue
		}
		versions = append(versions, n)
	}
	sort.Strings(versions)
	if len(versions) <= retention {
		return
	}
	for _, v := range versions[:len(versions)-retention] {
		_ = os.Remove(filepath.Join(dir, v))
	}
}

// ListVersions 列出某 source 的所有版本文件（按时间升序）。
func ListVersions(dir, name, ext string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var versions []string
	prefix := name + "-"
	suffix := "." + ext
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, prefix) && strings.HasSuffix(n, suffix) {
			versions = append(versions, n)
		}
	}
	sort.Strings(versions)
	return versions, nil
}

// Rollback 找到上一版文件路径（不删除当前版，保留可审计）。
func Rollback(dir, name, ext string) (string, error) {
	versions, err := ListVersions(dir, name, ext)
	if err != nil {
		return "", err
	}
	if len(versions) < 2 {
		return "", fmt.Errorf("source %q 可回滚版本不足（需要至少 2 个版本）", name)
	}
	prev := versions[len(versions)-2]
	return filepath.Join(dir, prev), nil
}
