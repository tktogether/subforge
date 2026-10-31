package subforge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/manifest"
	"github.com/tktogether/subforge/internal/model"
)

// AirportRuleSourceResult 是一次机场规则源写入的结果。
type AirportRuleSourceResult struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Version   string `json:"version"`
	Rules     int    `json:"rules"`
	Providers int    `json:"providers"`
	Path      string `json:"path"`
}

// SaveAirportRuleSource 把机场订阅里的规则写成一个版本化规则源产品。
//
// 机场规则不再进入 rules 表：它们和远程规则源一样有版本、当前指针和保留策略，
// 由方案显式绑定后在订阅里内联。规则条目的目标仍是原分组名，分组由机场导入路径
// 单独维护。
func (m *Manager) SaveAirportRuleSource(ctx context.Context, airportName string, airportID int64, url string, rules []string, providers []model.RuleProvider) (AirportRuleSourceResult, error) {
	name := AirportRuleSourceName(airportName, airportID)
	// 机场改名后复用已有源名，避免同一个机场出现两个规则源、方案绑定留在旧源上。
	if sources, lerr := m.ListRuleSources(ctx); lerr == nil {
		for _, src := range sources {
			if src.Origin != "airport" {
				continue
			}
			if id, ok := AirportIDFromRuleSourceName(src.Name); ok && id == airportID {
				name = src.Name
				break
			}
		}
	}
	if err := m.upsertManifestSource(ctx, manifest.Source{
		Name: name, URL: url, Type: "clash", InputFormat: "clash",
	}); err != nil {
		return AirportRuleSourceResult{}, err
	}
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return AirportRuleSourceResult{}, err
	}
	dir, err := m.ResolveRulesOutputDir(ctx)
	if err != nil {
		return AirportRuleSourceResult{}, err
	}
	ext := extForRuleSourceFormat(mf.Output.Format)
	providerMap := make(map[string]any, len(providers))
	for _, p := range providers {
		providerMap[p.Name] = p.Raw
	}
	var content []byte
	if ext == "txt" {
		if len(providerMap) > 0 {
			return AirportRuleSourceResult{}, fmt.Errorf("规则源输出格式为 text，无法保存 rule-providers；请把输出格式改为 yaml")
		}
		content = []byte(strings.Join(rules, "\n") + "\n")
	} else {
		content, err = yaml.Marshal(struct {
			Rules         []string       `yaml:"rules"`
			RuleProviders map[string]any `yaml:"rule-providers,omitempty"`
		}{Rules: rules, RuleProviders: providerMap})
		if err != nil {
			return AirportRuleSourceResult{}, fmt.Errorf("序列化机场规则源 %s: %w", name, err)
		}
	}
	res, err := manifest.SaveProduct(dir, name, ext, content, manifest.DefaultRetention)
	if err != nil {
		return AirportRuleSourceResult{}, err
	}
	if err := setCurrentVersion(dir, name, filepath.Base(res.Path)); err != nil {
		return AirportRuleSourceResult{}, fmt.Errorf("记录规则源 %s 的当前版本: %w", name, err)
	}
	return AirportRuleSourceResult{
		Name:      name,
		Label:     AirportRuleSourceLabel(name),
		Version:   res.Version,
		Rules:     len(rules),
		Providers: len(providers),
		Path:      res.Path,
	}, nil
}

// upsertManifestSource 按内部名新增或更新 manifest source。
// 机场源使用保留前缀，重名时更新 URL/类型（机场地址可能被用户改过）。
func (m *Manager) upsertManifestSource(ctx context.Context, src manifest.Source) error {
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return err
	}
	mf := &manifest.Manifest{
		Output: manifest.Output{Format: "clash-meta", Path: "./rules"},
	}
	if _, statErr := os.Stat(path); statErr == nil {
		loaded, lerr := manifest.LoadManifestRaw(path)
		if lerr != nil {
			return fmt.Errorf("读取规则源清单 %s: %w", path, lerr)
		}
		mf = loaded
	}
	if strings.TrimSpace(mf.Output.Format) == "" {
		mf.Output.Format = "clash-meta"
	}
	if strings.TrimSpace(mf.Output.Path) == "" {
		mf.Output.Path = "./rules"
	}
	found := false
	for i := range mf.Sources {
		if mf.Sources[i].SourceName() != src.Name {
			continue
		}
		mf.Sources[i].URL = src.URL
		mf.Sources[i].Type = src.Type
		mf.Sources[i].InputFormat = src.InputFormat
		found = true
		break
	}
	if !found {
		mf.Sources = append(mf.Sources, src)
	}
	if err := mf.ValidateForWrite(); err != nil {
		return err
	}
	return manifest.SaveManifest(path, mf)
}

// UnbindAirportRuleSource 从所有方案的规则源绑定里移除指定机场的源。
// 删除机场前调用：源作为历史快照保留，方案不再引用它。
func (m *Manager) UnbindAirportRuleSource(ctx context.Context, airportID int64) (string, error) {
	sources, err := m.ListRuleSources(ctx)
	if err != nil {
		return "", err
	}
	name := ""
	for _, src := range sources {
		if src.Origin != "airport" {
			continue
		}
		if id, ok := AirportIDFromRuleSourceName(src.Name); ok && id == airportID {
			name = src.Name
			break
		}
	}
	if name == "" {
		return "", nil
	}
	profiles, err := m.ListProfiles(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range profiles {
		next := make([]string, 0, len(p.RuleSources))
		changed := false
		for _, used := range p.RuleSources {
			if used == name {
				changed = true
				continue
			}
			next = append(next, used)
		}
		if changed {
			if err := m.SetProfileRuleSources(ctx, p.ID, next); err != nil {
				return "", err
			}
		}
	}
	return name, nil
}
