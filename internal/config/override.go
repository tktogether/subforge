package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/template"
	"gopkg.in/yaml.v3"
)

// Override 是用户本地覆盖层（M2 §1.2 分层覆盖 DSL）。
//
// 覆盖优先级链（高 → 低）：URL 参数 > 用户 override > 远程模板 > 内置默认。
// merge 语义：数组追加（去重），标量替换。
type Override struct {
	Groups  []GroupOverride `yaml:"groups"`
	Rules   RulesOverride   `yaml:"rules"`
	Proxies []ProxyOverride `yaml:"proxies"`
}

// GroupOverride 是 override 中的一条代理组覆盖。
// 同名组按 mergeGroups 规则合并；数组追加，标量替换。
type GroupOverride struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	// Use 引用的 proxy-provider 名（mihomo 组的 use 字段）。
	Use      []string `yaml:"use"`
	Strategy string   `yaml:"strategy"`
	Test     string   `yaml:"test"`
	Lazy     bool     `yaml:"lazy"`
}

// RulesOverride 是规则的 prepend/append 覆盖。
type RulesOverride struct {
	Prepend []string `yaml:"prepend"`
	Append  []string `yaml:"append"`
}

// ProxyOverride 是节点级标量覆盖（按 name 正则匹节点）。
type ProxyOverride struct {
	Name   string `yaml:"name"`
	Server string `yaml:"server"`
	Port   string `yaml:"port"`
}

func (o Override) validate() error {
	seenGroup := map[string]bool{}
	for _, g := range o.Groups {
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("override.group 缺少 name")
		}
		if seenGroup[g.Name] {
			return fmt.Errorf("override.group 重复定义 %q", g.Name)
		}
		seenGroup[g.Name] = true
	}
	seenProxy := map[string]bool{}
	for _, p := range o.Proxies {
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("override.proxy 缺少 name")
		}
		if seenProxy[p.Name] {
			return fmt.Errorf("override.proxy 重复定义 %q", p.Name)
		}
		seenProxy[p.Name] = true
		// name 按正则匹节点，解析期就校验，避免非法正则在转换中途炸。
		if _, err := regexp.Compile(p.Name); err != nil {
			return fmt.Errorf("override.proxy 名称正则 %q 非法: %v", p.Name, err)
		}
	}
	return nil
}

// LoadOverrideFile 根据扩展名选择 YAML / INI 解析。
func LoadOverrideFile(path string) (Override, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Override{}, fmt.Errorf("读取 override 文件 %s: %w", path, err)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ini":
		return ParseOverrideINI(b)
	default:
		return ParseOverrideYAML(b)
	}
}

// ParseOverrideYAML 解析 override YAML。
func ParseOverrideYAML(b []byte) (Override, error) {
	var o Override
	if err := yaml.Unmarshal(b, &o); err != nil {
		return Override{}, fmt.Errorf("解析 override YAML: %w", err)
	}
	if err := o.validate(); err != nil {
		return Override{}, err
	}
	return o, nil
}

// ParseOverrideINI 解析 override INI。
//
// 段落语法（见文件头）：
//
//	[override.group:AI]
//	type = select
//	proxy = HK
//	proxy = JP
//	strategy = url-test
//	[override.rules]
//	prepend = DOMAIN-SUFFIX,openai.com,AI
//	append = DOMAIN-SUFFIX,claude.ai,AI
//	[override.proxy:HK]
//	server = new-hk.example.com
func ParseOverrideINI(b []byte) (Override, error) {
	cfg, err := template.Parse(b, "")
	if err != nil {
		return Override{}, fmt.Errorf("解析 override INI: %w", err)
	}
	var o Override
	for _, name := range cfg.SectionNames() {
		lower := strings.ToLower(strings.TrimSpace(name))
		switch {
		case lower == "override.rules":
			for _, e := range cfg.Section(name).Entries() {
				v := strings.TrimSpace(e.Value.Raw)
				switch strings.ToLower(strings.TrimSpace(e.Key)) {
				case "prepend":
					o.Rules.Prepend = append(o.Rules.Prepend, v)
				case "append":
					o.Rules.Append = append(o.Rules.Append, v)
				default:
					return Override{}, fmt.Errorf("[%s] 未知键 %q（只认 prepend/append）", name, e.Key)
				}
			}
		case strings.HasPrefix(lower, "override.group:"):
			gname := strings.TrimSpace(name[len("override.group:"):])
			g := GroupOverride{Name: gname}
			// 同名段落在 INI 层合并（标量键后者覆盖前者，proxy 追加），
			// 重复定义只能在 YAML 路径的 validate 里检测。
			for _, e := range cfg.Section(name).Entries() {
				key := strings.ToLower(strings.TrimSpace(e.Key))
				v := strings.TrimSpace(e.Value.Raw)
				switch key {
				case "type":
					g.Type = v
				case "strategy":
					g.Strategy = v
				case "test":
					g.Test = v
				case "lazy":
					g.Lazy = v == "1" || strings.EqualFold(v, "true")
				case "proxy":
					if v != "" {
						g.Proxies = append(g.Proxies, v)
					}
				case "use":
					if v != "" {
						g.Use = append(g.Use, v)
					}
				default:
					return Override{}, fmt.Errorf("[%s] 未知键 %q", name, e.Key)
				}
			}
			o.Groups = append(o.Groups, g)
		case strings.HasPrefix(lower, "override.proxy:"):
			pname := strings.TrimSpace(name[len("override.proxy:"):])
			p := ProxyOverride{Name: pname}
			for _, e := range cfg.Section(name).Entries() {
				switch strings.ToLower(strings.TrimSpace(e.Key)) {
				case "server":
					p.Server = strings.TrimSpace(e.Value.Raw)
				case "port":
					p.Port = strings.TrimSpace(e.Value.Raw)
				default:
					return Override{}, fmt.Errorf("[%s] 未知键 %q（只认 server/port）", name, e.Key)
				}
			}
			o.Proxies = append(o.Proxies, p)
		default:
			return Override{}, fmt.Errorf("未知段落 [%s]（override 只认 [override.rules] / [override.group:<名>] / [override.proxy:<名>]）", name)
		}
	}
	if err := o.validate(); err != nil {
		return Override{}, err
	}
	return o, nil
}

// ToGroups 把 GroupOverride 转成 render.Group 列表（用于合并）。
func (o Override) ToGroups() []render.Group {
	out := make([]render.Group, len(o.Groups))
	for i, g := range o.Groups {
		out[i] = g.toGroup()
	}
	return out
}

func (g GroupOverride) toGroup() render.Group {
	return render.Group{
		Name:     g.Name,
		Type:     g.Type,
		Proxies:  append([]string{}, g.Proxies...),
		Use:      append([]string{}, g.Use...),
		Strategy: g.Strategy,
		Test:     g.Test,
		Lazy:     g.Lazy,
	}
}

// MergeGroups 用 override 的代理组合并基础组（数组追加，标量替换）。
func MergeGroups(base []render.Group, overrides []render.Group) []render.Group {
	if len(overrides) == 0 {
		return append([]render.Group{}, base...)
	}
	idx := map[string]int{}
	out := append([]render.Group{}, base...)
	for i, g := range out {
		idx[g.Name] = i
	}
	for _, g := range overrides {
		if i, ok := idx[g.Name]; ok {
			out[i] = mergeGroup(out[i], g)
		} else {
			out = append(out, g)
			idx[g.Name] = len(out) - 1
		}
	}
	return out
}

func mergeGroup(base, over render.Group) render.Group {
	res := base
	if over.Type != "" {
		res.Type = over.Type
	}
	if over.Strategy != "" {
		res.Strategy = over.Strategy
	}
	if over.Test != "" {
		res.Test = over.Test
	}
	if over.Lazy {
		res.Lazy = true
	}
	if over.Options != (render.UrlTestOptions{}) {
		res.Options = over.Options
	}
	seen := map[string]bool{}
	for _, p := range res.Proxies {
		seen[p] = true
	}
	for _, p := range over.Proxies {
		if !seen[p] {
			seen[p] = true
			res.Proxies = append(res.Proxies, p)
		}
	}
	// use 与 proxies 同为成员列表，按同样的追加去重语义合并。
	seenUse := map[string]bool{}
	for _, u := range res.Use {
		seenUse[u] = true
	}
	for _, u := range over.Use {
		if !seenUse[u] {
			seenUse[u] = true
			res.Use = append(res.Use, u)
		}
	}
	return res
}

// MergeRules 把 prepend / base / append 拼成最终规则列表。
func MergeRules(base, prepend, append_ []string) []string {
	out := make([]string, 0, len(prepend)+len(base)+len(append_))
	out = append(out, prepend...)
	out = append(out, base...)
	out = append(out, append_...)
	return out
}
