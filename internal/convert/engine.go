// Engine 执行一次完整转换：拉取 -> 解析 -> 管线 -> 渲染。
//
// CLI 与 HTTP API 共用这个执行体，两边都只提供参数，避免各写一遍流程。
package convert

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/render"
)

// ErrNoSource 表示没有给出任何订阅来源。
var ErrNoSource = errors.New("至少需要一个订阅地址或本地文件")

// ErrNoNodes 表示来源可用但转换后没有节点（全部失败或被过滤/排序条件清空）。
var ErrNoNodes = errors.New("没有可用节点：检查来源内容，或放宽 include/exclude/type 条件")

// Engine 是一次转换的执行体。
type Engine struct {
	// Fetcher 负责拉取上游订阅。
	Fetcher *fetch.Fetcher
	// RendererFor 按 target 名取得渲染器；注入而非写死，便于替换与测试。
	RendererFor func(target string) (render.Renderer, error)
	// Profile 写进产物注释头的信息。
	Profile render.Profile
	// Now 生成时间戳来源，测试可固定。
	Now func() time.Time
	// BaseTemplate 返回当前生效的本地模板（-config 指定的 INI），可以为 nil。
	// 用函数而不是值：配置文件热重载会替换它，而 Engine 在请求之间是共享的。
	BaseTemplate func() *config.Template
	// LoadTemplate 解析请求参数 config 指向的模板（本地路径或远程 URL）。
	// 为 nil 且请求带了 config 参数时会报错，避免悄悄忽略用户明确要的模板。
	LoadTemplate func(ctx context.Context, ref string) (*config.Template, error)
	// RulesetLoader 拉取模板引用的远程规则集（ruleset=clash-domain:URL）。
	// 为 nil 时用默认 HTTP 客户端；两者都不能复用 Fetcher，理由见 rulesetBody。
	RulesetLoader config.RemoteLoader
}

// Result 是一次转换的产物。
type Result struct {
	// Body 渲染结果。
	Body []byte
	// Format 渲染器名。
	Format string
	// Nodes 管线后的节点数。
	Nodes int
	// RawNodes 拉取解析到的节点总数（管线前）。
	RawNodes int
	// Failures 失败来源的原因（部分成功时不为空）。
	Failures []string
	// Warnings 渲染阶段跳过或降级的项（如 list 模式里协议不支持分享链接的节点）。
	// 与 Failures 分开：这些不影响产物正确性，但静默丢弃没人会发现。
	Warnings []string
	// Cached 每个来源是否命中磁盘缓存。
	Cached []bool
	// Groups 生效的代理组数（覆盖合并之后、成员展开之后）。
	// 给 Dashboard 的状态概览用：只看节点数看不出"分组有没有生效"。
	Groups int
	// Rules 生效的规则行数（含展开后的远程规则集）。
	Rules int
}

// Run 执行一次转换。没有任何来源时直接报错，避免产出空配置。
func (e *Engine) Run(ctx context.Context, p Params) (Result, error) {
	if e == nil || e.Fetcher == nil {
		return Result{}, fmt.Errorf("Engine 未初始化 Fetcher")
	}
	if e.RendererFor == nil {
		return Result{}, fmt.Errorf("Engine 未初始化 RendererFor")
	}
	if !p.HasSource() {
		return Result{}, ErrNoSource
	}

	// 覆盖优先级链在这里合并：内置默认 < 本地模板 < 远程模板 < 请求参数。
	eff, err := e.resolveParams(ctx, p)
	if err != nil {
		return Result{}, err
	}
	// 用户 override 是优先级链里"请求参数"之上的一层：在模板语义之后应用，
	// 组合并到模板组、规则 prepend/append。节点级覆盖（server/port）在拉取
	// 完成后、进管线前落到原始节点上。
	ov := p.Override
	if len(ov.Groups) > 0 {
		eff.Groups = config.MergeGroups(eff.Groups, ov.ToGroups())
	}
	if len(ov.Rules.Prepend) > 0 || len(ov.Rules.Append) > 0 {
		eff.Rules = config.MergeRules(eff.Rules, ov.Rules.Prepend, ov.Rules.Append)
	}
	// target 归一化必须早于编管线：协议收窄要落成类型过滤，节点数统计才与产物一致。
	eff, target := resolveTarget(eff)

	// 渲染器也在这里就解析掉。target 完全由调用方控制，提前查一次能在浪费
	// 任何一次上游拉取之前就报"格式没实现"；放在管线末尾的话，一个 target
	// 写错的请求会白跑一整轮拉取，然后把本该是 501 的错误报成 502。
	renderer, err := e.RendererFor(target)
	if err != nil {
		return Result{}, err
	}

	plan, err := BuildPlan(eff)
	if err != nil {
		return Result{}, err
	}

	var (
		list      model.NodeList
		failures  []string
		cached    []bool
		providers []model.Provider
	)
	// provider 按名去重：多个来源可能带同名 provider，首次出现的定义优先。
	seenProvider := map[string]bool{}
	collectProviders := func(ps []model.Provider) {
		for _, p := range ps {
			if seenProvider[p.Name] {
				continue
			}
			seenProvider[p.Name] = true
			providers = append(providers, p)
		}
	}
	for _, raw := range p.URLs {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		name := sourceName(u)
		res := e.Fetcher.FetchFrom(ctx, fetch.Source{Name: name, URL: u, Auth: p.Headers})
		if res.Err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", u, res.Err))
			continue
		}
		dec, err := parse.DecodeSource(res.Content)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", u, err))
			continue
		}
		list = append(list, model.NewNodeList(dec.Nodes, name)...)
		collectProviders(dec.Providers)
		cached = append(cached, res.FromCache)
	}
	for _, f := range p.Files {
		buf, err := os.ReadFile(f)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", f, err))
			continue
		}
		dec, err := parse.DecodeSource(buf)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", f, err))
			continue
		}
		list = append(list, model.NewNodeList(dec.Nodes, filepath.Base(f))...)
		collectProviders(dec.Providers)
	}

	if list.IsEmpty() {
		// 全部失败时把每条原因带上，"过滤后为空"时给一条明确提示。
		if len(failures) > 0 {
			return Result{}, fmt.Errorf("所有来源都失败:\n  %s", strings.Join(failures, "\n  "))
		}
		return Result{}, ErrNoNodes
	}

	rawNodes := list.Len()
	// 节点级覆盖（server/port）在这里落到原始节点上，早于管线：去重按指纹、
	// 排序键也可能用 server，都该看到覆盖后的值。匹配不到节点的覆盖记入告警——
	// 写错名字的覆盖应当被看见，而不是静默失效。
	overrideWarns, _ := applyProxyOverrides(list, p.Override.Proxies)
	planRes, err := plan.Run(list)
	if err != nil {
		return Result{}, err
	}
	nodes := planRes.Nodes
	if nodes.IsEmpty() {
		return Result{}, ErrNoNodes
	}

	profile := e.Profile
	if profile.Version != "" && profile.GeneratedAt == "" {
		profile.GeneratedAt = nowUTC(e.Now)
	}
	groups := ResolveGroups(eff.Groups, nodes.Names())
	body, err := renderer.Render(render.Request{
		Nodes:     nodes,
		Groups:    groups,
		Rules:     eff.Rules,
		Providers: providers,
		Profile:   profile,
	})
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Body:     body,
		Format:   renderer.Name(),
		Nodes:    nodes.Len(),
		RawNodes: rawNodes,
		Failures: failures,
		Cached:   cached,
		Groups:   len(groups),
		Rules:    len(eff.Rules),
	}
	if wr, ok := renderer.(render.WarningReporter); ok {
		result.Warnings = wr.Warnings()
	}
	if len(eff.TemplateWarnings) > 0 || len(planRes.Warnings) > 0 || len(overrideWarns) > 0 {
		// 模板提示与管线告警排在渲染阶段提示前面：模板层面的漏配比单个节点降级更致命。
		head := make([]string, 0, len(eff.TemplateWarnings)+len(planRes.Warnings)+len(overrideWarns))
		head = append(head, eff.TemplateWarnings...)
		head = append(head, planRes.Warnings...)
		head = append(head, overrideWarns...)
		result.Warnings = append(head, result.Warnings...)
	}
	return result, nil
}

// applyProxyOverrides 把节点级覆盖（server/port）落到原始节点上。
// 按 name 正则匹节点名；改完用 model.NodeFromRaw 重新派生只读视图。
// 返回告警（正则非法或匹配不到节点）与覆盖次数。
func applyProxyOverrides(list model.NodeList, overrides []config.ProxyOverride) ([]string, int) {
	if len(overrides) == 0 || list.IsEmpty() {
		return nil, 0
	}
	var warns []string
	applied := 0
	for _, o := range overrides {
		re, err := regexp.Compile(o.Name)
		if err != nil {
			warns = append(warns, fmt.Sprintf("override.proxy 正则 %q 非法: %v", o.Name, err))
			continue
		}
		matched := 0
		for i, n := range list {
			if !re.MatchString(n.Name) {
				continue
			}
			raw := n.Raw
			changed := false
			if o.Server != "" {
				raw["server"] = o.Server
				changed = true
			}
			if o.Port != "" {
				// port 在内核里可能是数字也可能是字符串，统一按字符串写回，
				// 渲染器两种都吃。
				raw["port"] = o.Port
				changed = true
			}
			if changed {
				list[i] = model.NodeFromRaw(raw, n.Source)
				matched++
			}
		}
		if matched == 0 {
			warns = append(warns, fmt.Sprintf("override.proxy %q 未匹配到任何节点", o.Name))
			continue
		}
		applied += matched
	}
	return warns, applied
}

// resolveTarget 归一化输出目标，返回实际使用的渲染器名。
// 分享链接形态不产出客户端配置，代理组与规则对纯清单没有意义；协议收窄在这里落成类型过滤，
// 而不是丢给渲染器再切一遍——否则节点数统计与产物对不上。
// 请求或模板已经给了 -type / include_types 时不覆盖：那是更显式的指令。
//
// 收窄用的类型名一律是内核规范化后的真名（render.LinkTargets 里写死），
// 因为 mihomo 写进 type 字段的是分享链接 scheme（converter.go 里是 `ss["type"] = scheme`，
// 即 ss 而不是 shadowsocks）。写成长名会命中零个节点，请求退化成"没有可用节点"。
// 用户按长名写 -type 时的容错在 transform.TypeFilter 的别名表。
func resolveTarget(p Params) (Params, string) {
	lt, ok := render.NormalizeTarget(p.Target)
	if !ok {
		if p.List {
			return p, "list"
		}
		return p, p.Target
	}
	if len(lt.Types) > 0 && len(p.Types) == 0 {
		p.Types = lt.Types
	}
	return p, lt.Renderer
}

// ResolveTemplates 只解释模板链（本地模板 + config 指向的远程模板），返回其中的
// 规则集映射与规则行。/getruleset 走这条路径：它需要模板语义，但不需要跑转换。
func (e *Engine) ResolveTemplates(ctx context.Context, configRef string) (Params, error) {
	if e == nil || (e.BaseTemplate == nil && configRef == "") {
		return Params{}, nil
	}
	return e.resolveParams(ctx, Params{Config: configRef})
}

// resolveParams 把本地模板与请求参数 config 指向的远程模板合并进来。
func (e *Engine) resolveParams(ctx context.Context, base Params) (Params, error) {
	chains := make([]*config.Template, 0, 2)
	if e.BaseTemplate != nil {
		if tpl := e.BaseTemplate(); tpl != nil {
			chains = append(chains, tpl)
		}
	}
	if base.Config != "" {
		if e.LoadTemplate == nil {
			return Params{}, fmt.Errorf("请求指定了 config=%q，但引擎没有配置模板加载器", base.Config)
		}
		tpl, err := e.LoadTemplate(ctx, base.Config)
		if err != nil {
			return Params{}, fmt.Errorf("加载配置模板 %q: %w", base.Config, err)
		}
		chains = append(chains, tpl)
	}
	if len(chains) == 0 {
		return base, nil
	}
	merged, err := config.Merge(chains...)
	if err != nil {
		return Params{}, err
	}
	sem, err := merged.Semantics()
	if err != nil {
		return Params{}, err
	}
	out, err := ApplyTemplate(sem, base)
	if err != nil {
		return Params{}, err
	}
	// 远程规则集在这里展开：Semantics 是纯解析没有网络能力，只有拿到模板引用才拉。
	expanded, warns := e.expandRemoteRulesets(ctx, out)
	if len(warns) > 0 {
		expanded.TemplateWarnings = append(expanded.TemplateWarnings, warns...)
	}
	return expanded, nil
}

func nowUTC(now func() time.Time) string {
	if now == nil {
		now = time.Now
	}
	return now().UTC().Format(time.RFC3339)
}

// sourceName 从 URL 里取一个可读的来源名（最后一个路径段），失败时退回整串。
func sourceName(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil && u.Path != "" {
		seg := strings.TrimRight(u.Path, "/")
		if i := strings.LastIndex(seg, "/"); i >= 0 {
			seg = seg[i+1:]
		}
		if seg != "" && seg != "." {
			return seg
		}
	}
	return filepath.Base(strings.TrimSpace(rawURL))
}
