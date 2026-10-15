package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/template"
	"github.com/tktogether/subforge/internal/transform"
)

// 模板语义层：把 INI 段落解释成一次转换请求能用的参数。
//
// 覆盖优先级链在这里落地：内置默认 < 本地 config < 远程 config < URL 参数。
// 前两级都是 Template（INI 文本），用 Merge 合并；最后一级是请求参数，由
// convert.ApplyTemplate 压在最上面。解析层（internal/template）不理解任何键的含义，
// 语义全部集中在本文件。

// RemoteLoader 拉取远程配置内容，用于 config= 指向 URL 的场景。
type RemoteLoader func(ctx context.Context, url string) ([]byte, error)

// maxConfigBody 限制单份配置的体积：模板配置通常只有几 KB，几 MB 就足以容纳最繁重的情况。
const maxConfigBody = 4 << 20

// HTTPLoader 用普通 HTTP GET 拉远程配置，是 RemoteLoader 的标准实现。
//
// 刻意不复用 fetch.Fetcher：那是为订阅内容做的，会把响应体当订阅去识别形态，
// 于是 INI 文本被判成"无法识别订阅内容形态"而整体失败。配置文件只需要一次简单 GET。
func HTTPLoader(client *http.Client, headers map[string]string) RemoteLoader {
	if client == nil {
		client = http.DefaultClient
	}
	return func(ctx context.Context, ref string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxConfigBody+1))
		if err != nil {
			return nil, err
		}
		if len(body) > maxConfigBody {
			return nil, fmt.Errorf("配置体积超过上限 %d 字节", maxConfigBody)
		}
		return body, nil
	}
}

// Template 是一份解析好的 INI 模板，可能已经按优先级链合并过。
type Template struct {
	cfg  *template.Config
	path string
}

// Origin 是模板的来源标识（文件路径或 URL）。
func (t *Template) Origin() string {
	if t == nil {
		return ""
	}
	return t.path
}

// LoadFile 从本地文件加载并展开 !include。
func LoadFile(path string) (*Template, error) {
	cfg, err := template.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return &Template{cfg: cfg, path: path}, nil
}

// LoadBody 从内存内容加载（远程配置拉回来之后走这里）。
func LoadBody(body []byte, origin string) (*Template, error) {
	cfg, err := template.Parse(body, origin)
	if err != nil {
		return nil, err
	}
	return &Template{cfg: cfg, path: origin}, nil
}

// LoadSource 按引用加载模板：带 "://" 的当远程配置交给 loader，否则按本地路径读。
func LoadSource(ctx context.Context, ref string, loader RemoteLoader) (*Template, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("配置引用为空")
	}
	if strings.Contains(ref, "://") {
		if loader == nil {
			return nil, fmt.Errorf("无法拉取远程配置 %q：未配置加载器", ref)
		}
		body, err := loader(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("拉取配置 %s: %w", ref, err)
		}
		return LoadBody(body, ref)
	}
	return LoadFile(ref)
}

// Merge 把若干模板按优先级从低到高合并成一份，覆盖链里的"本地 config < 远程 config"这一级。
// 普通段落按键覆盖（后者胜）；[clash_proxy_group] 按组追加——远程模板的代理组是加进来，
// 而不是把本地模板的组顶掉。
func Merge(chains ...*Template) (*Template, error) {
	var out *Template
	origins := []string{}
	for _, t := range chains {
		if t == nil || t.cfg == nil {
			continue
		}
		if out == nil {
			out = &Template{cfg: t.cfg.Clone()}
			origins = append(origins, t.path)
			continue
		}
		out.cfg.OverlayExcept(t.cfg, func(name string) bool {
			return strings.EqualFold(name, template.GroupSectionName)
		})
		out.appendGroups(t.cfg)
		origins = append(origins, t.path)
	}
	if out == nil {
		return nil, nil
	}
	out.path = strings.Join(origins, " + ")
	return out, nil
}

// appendGroups 把 other 里的代理组逐块追加到 out 的段落末尾。
func (t *Template) appendGroups(other *template.Config) {
	otherSec := other.Lookup(template.GroupSectionName)
	if otherSec == nil {
		return
	}
	blocks := otherSec.GroupBlocks()
	if len(blocks) == 0 {
		return
	}
	target := t.cfg.Section(template.GroupSectionName)
	for _, block := range blocks {
		// 分隔符加在块之前：target 里已有的块和刚追加的块要靠它分开。
		target.AddGroupSeparator()
		for _, e := range block {
			target.Add(e.Key, e.Value)
		}
	}
}

// Semantics 是模板解释后的结果。
type Semantics struct {
	// Params 段落翻译成的请求参数，键名与 URL 参数一致，可直接交给 ParseMulti。
	Params map[string][]string
	// NodePref 节点名前缀规则（[node_pref] 段落）。
	NodePref transform.NodePrefSpec
	// Groups 代理组定义（[clash_proxy_group] 段落，用 "---" 分隔多个）。
	Groups []render.Group
}

// Semantics 把模板解释成转换请求能用的参数。
//
// 段落语义：
//   - [common]             键名即参数名，"!key" 与 "key" 等价（subconverter 约定）。
//   - [node_pref]          协议类型 -> 前缀；include / exclude 按节点名限定生效范围。
//   - [rename] / [emoji]   一行一条规则，键是正则、值是替换；值里带分号表示多条。
//   - [clash_proxy_group]  代理组，"---" 分隔多个。
//   - [ruleset]            规则集名 -> 类型。
func (t *Template) Semantics() (Semantics, error) {
	if t == nil || t.cfg == nil {
		return Semantics{}, nil
	}
	out := Semantics{Params: map[string][]string{}}

	if s := t.cfg.Lookup("common"); s != nil {
		for _, e := range s.Entries() {
			key := strings.TrimPrefix(e.Key, "!")
			if key == "" || commonSkip[key] {
				continue
			}
			if v := strings.TrimSpace(e.Value.Raw); v != "" {
				out.Params[key] = append(out.Params[key], v)
			}
		}
	}

	for _, sec := range []string{"rename", "emoji"} {
		var rules []string
		// 同一来源内的两处写法是叠加关系：段落规则先生效，[common] 里的行规则后生效
		// （后写的能改前一个规则留下的名字）。跨来源的覆盖由 ApplyTemplate 处理。
		rules = append(rules, t.sectionRules(sec)...)
		if v, ok := commonValue(t.cfg, sec); ok && v != "" {
			rules = append(rules, splitRules(v)...)
		}
		if len(rules) > 0 {
			out.Params[sec] = append(out.Params[sec], rules...)
		}
	}

	if s := t.cfg.Lookup("node_pref"); s != nil {
		out.NodePref = nodePrefSpec(s)
	}

	if s := t.cfg.Lookup(template.GroupSectionName); s != nil {
		for _, block := range s.GroupBlocks() {
			if g, ok := groupFromBlock(block); ok {
				out.Groups = append(out.Groups, g)
			}
		}
	}

	if s := t.cfg.Lookup("ruleset"); s != nil {
		for _, e := range s.Entries() {
			name := strings.TrimSpace(e.Key)
			typ := strings.TrimSpace(e.Value.Raw)
			if name == "" || typ == "" {
				continue
			}
			out.Params["ruleset"] = append(out.Params["ruleset"], name+":"+typ)
		}
	}
	return out, nil
}

// commonSkip 是 [common] 里的模板控制键：它们不是单次转换参数，不能原样透传。
// rename / emoji 也在这里拦下，因为它们还要走 sectionRules 合并段落与行两种写法。
var commonSkip = map[string]bool{
	"node_pref":  true, // 选择 [node_pref] 块；M1 只解释唯一的那一块
	"local":      true, // 模板来源开关，M2 再做
	"remote":     true, // 模板来源开关，M2 再做
	"token":      true, // 服务访问令牌，由 [subforge] 读取，不是转换参数
	"emoji_type": true, // emoji 表选择；M1 只有内置表
	"node_order": true, // 节点排序规则，M2 再做
	"node-order": true,
	"rename":     true,
	"emoji":      true,
}

// nodePrefControl 是 [node_pref] 里的控制键，它们不是协议类型。
var nodePrefControl = map[string]bool{
	"include": true, "exclude": true, "sort": true,
	"prefix": true, "suffix": true, "remove": true,
}

// nodePrefSpec 解释 [node_pref] 段落。
func nodePrefSpec(s *template.Section) transform.NodePrefSpec {
	spec := transform.NodePrefSpec{Prefixes: map[string]string{}}
	for _, e := range s.Entries() {
		key := strings.ToLower(e.Key)
		if nodePrefControl[key] {
			switch key {
			case "include":
				spec.Include = append(spec.Include, splitRules(e.Value.Raw)...)
			case "exclude":
				spec.Exclude = append(spec.Exclude, splitRules(e.Value.Raw)...)
			}
			continue
		}
		// 前缀原样保留，包括尾部空格：空格就是前缀的一部分，补不补由配置者决定。
		if v := e.Value.Raw; strings.TrimSpace(v) != "" {
			spec.Prefixes[key] = v
		}
	}
	return spec
}

// sectionRules 取 [rename] / [emoji] 段落的规则。
//
// subconverter 的段落写法是"一行一条规则"：键是正则，值是替换。
// 一行里带分号时按多规则列表处理（值是 "正则,替换;正则,替换"，键只是标签）。
func (t *Template) sectionRules(sec string) []string {
	section := t.cfg.Lookup(sec)
	if section == nil {
		return nil
	}
	var out []string
	for _, e := range section.Entries() {
		if strings.TrimSpace(e.Key) == "" {
			continue
		}
		if strings.Contains(e.Value.Raw, ";") {
			out = append(out, splitRules(e.Value.Raw)...)
			continue
		}
		out = append(out, e.Key+","+e.Value.Raw)
	}
	return out
}

// commonValue 取 [common] 里的值，"!key" 与 "key" 都认。
func commonValue(cfg *template.Config, key string) (string, bool) {
	s := cfg.Lookup("common")
	if s == nil {
		return "", false
	}
	if v, ok := s.Get(key); ok {
		return strings.TrimSpace(v.Raw), true
	}
	if v, ok := s.Get("!" + key); ok {
		return strings.TrimSpace(v.Raw), true
	}
	return "", false
}

// groupFromBlock 把一个 [clash_proxy_group] 块解释成代理组。
// 返回 false 表示这个块不能用——没有名字的话 mihomo 里无法引用，
// 跳过比编一个名字掩盖模板错误更诚实。
func groupFromBlock(block []template.Entry) (render.Group, bool) {
	g := render.Group{Type: "select"}
	for _, e := range block {
		switch strings.ToLower(e.Key) {
		case "name":
			g.Name = strings.TrimSpace(e.Value.Raw)
		case "type":
			if v := strings.ToLower(strings.TrimSpace(e.Value.Raw)); v != "" {
				g.Type = v
			}
		case "nodes", "proxies", "nodes-list", "proxy-list":
			g.Proxies = append(g.Proxies, splitRules(e.Value.Raw)...)
		case "interval", "tolerance", "timeout", "idle":
			n, err := strconv.Atoi(strings.TrimSpace(e.Value.Raw))
			if err != nil || n < 0 {
				continue
			}
			switch strings.ToLower(e.Key) {
			case "interval":
				g.Options.Interval = n
			case "tolerance":
				g.Options.Tolerance = n
			case "timeout":
				g.Options.Timeout = n
			case "idle":
				g.Options.Idle = n
			}
		case "url", "test":
			if v := strings.TrimSpace(e.Value.Raw); v != "" {
				g.Test = v
			}
		case "strategy":
			if v := strings.ToLower(strings.TrimSpace(e.Value.Raw)); v == "global" || v == "compatible" {
				g.Strategy = v
			}
		case "lazy":
			g.Lazy = truthy(e.Value.Raw)
		}
	}
	if g.Name == "" {
		return render.Group{}, false
	}
	return g, true
}

// splitRules 按分号拆成多条规则。
// 逗号不动：rename / emoji 规则是"正则,替换"，逗号本身有语义。
func splitRules(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// truthy 判定布尔值：1 / true / yes / on 视为真，其余一律假（与参数解析一致）。
func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
