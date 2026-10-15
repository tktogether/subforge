package ruleset

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/render"
)

// Format 是 /getruleset 的输出形态，编号沿用 subconverter 的 1~6，
// 这样存量模板与在线前端可以不动 type 参数就迁移过来。
type Format int

const (
	SurgeList           Format = 1 // Surge 2.x 规则集（list）
	QuantumultX         Format = 2 // Quantumult X 远程规则提供方
	ClashDomainProvider Format = 3 // Clash DOMAIN 类 provider（payload 只有取值）
	ClashIPCIDRProvider Format = 4 // Clash IP-CIDR provider
	SurgeDomainSet      Format = 5 // Surge DOMAIN-SET（裸域名列表）
	ClashClassical      Format = 6 // Clash 经典 rules 段
)

// formatOrder 固定形态的枚举顺序，供错误信息与文档列举使用。
var formatOrder = []Format{SurgeList, QuantumultX, ClashDomainProvider, ClashIPCIDRProvider, SurgeDomainSet, ClashClassical}

// spec 描述一种输出形态的完整行为。
type spec struct {
	// label 人类可读名，用于错误信息。
	label string
	// extension 客户端识别用的扩展名（含点）。
	extension string
	// contentType 响应头，YAML 形态用 text/yaml 便于客户端直接识别。
	contentType string
	// keep 该形态能表达哪些规则形态；不在内的会被丢弃并计入 Result.Skipped。
	// 刻意不把丢弃做成报错——用户拿一份大规则集只取 DOMAIN 部分是正常用法。
	keep map[Kind]bool
	// render 产出正文，收到的 rules 已经按 keep 过滤过。
	render func(name string, rules []Rule, opts RenderOpts) ([]byte, error)
}

var specs = map[Format]spec{
	SurgeList: {
		label:       "Surge list",
		extension:   ".surge-rules",
		contentType: "application/x-ruleset; charset=utf-8",
		keep:        domainKinds(Domain, DomainSuffix, DomainKeyword, DomainRegex),
		render:      renderSurgeList,
	},
	QuantumultX: {
		label:       "Quantumult X list",
		extension:   ".qxt",
		contentType: "text/plain; charset=utf-8",
		keep:        domainKinds(Domain, DomainSuffix, DomainKeyword, DomainRegex, IPCIDR, IPCIDR6),
		render:      renderQuantumultX,
	},
	ClashDomainProvider: {
		label:       "Clash domain provider",
		extension:   ".yaml",
		contentType: "text/yaml; charset=utf-8",
		keep:        domainKinds(Domain, DomainSuffix, DomainKeyword, DomainRegex),
		render:      func(name string, rules []Rule, opts RenderOpts) ([]byte, error) { return renderClashProvider(rules) },
	},
	ClashIPCIDRProvider: {
		label:       "Clash ipcidr provider",
		extension:   ".yaml",
		contentType: "text/yaml; charset=utf-8",
		keep:        map[Kind]bool{IPCIDR: true, IPCIDR6: true},
		render:      func(name string, rules []Rule, opts RenderOpts) ([]byte, error) { return renderClashProvider(rules) },
	},
	SurgeDomainSet: {
		label:       "Surge DOMAIN-SET",
		extension:   ".txt",
		contentType: "text/plain; charset=utf-8",
		// DOMAIN-SET 是纯域名列表，客户端按域名匹配，关键词与正则放进去没意义。
		keep:   domainKinds(Domain, DomainSuffix),
		render: renderSurgeDomainSet,
	},
	ClashClassical: {
		label:       "Clash classical",
		extension:   ".yaml",
		contentType: "text/yaml; charset=utf-8",
		keep: map[Kind]bool{
			Domain: true, DomainSuffix: true, DomainKeyword: true, DomainRegex: true,
			IPCIDR: true, IPCIDR6: true, GeoIP: true, GeoSite: true,
			Match: true, Final: true,
		},
		render: renderClashClassical,
	},
}

// domainKinds 构造"只收域名类"的 keep 表。
func domainKinds(ks ...Kind) map[Kind]bool {
	out := make(map[Kind]bool, len(ks))
	for _, k := range ks {
		out[k] = true
	}
	return out
}

// nameAliases 是 type 参数的命名别名，方便不用记编号的场景。
var nameAliases = map[string]Format{
	"surge":                 SurgeList,
	"list":                  SurgeList,
	"qx":                    QuantumultX,
	"quantumult-x":          QuantumultX,
	"quantumult_x":          QuantumultX,
	"clash-domain":          ClashDomainProvider,
	"clash-domain-provider": ClashDomainProvider,
	"clash-ipcidr":          ClashIPCIDRProvider,
	"clash-ipcidr-provider": ClashIPCIDRProvider,
	"surge-domain-set":      SurgeDomainSet,
	"domain-set":            SurgeDomainSet,
	"clash-classical":       ClashClassical,
	"clash":                 ClashClassical,
}

// ParseFormat 解析 type 参数：接受 1~6，也接受命名别名。
func ParseFormat(s string) (Format, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, fmt.Errorf("输出形态不能为空")
	}
	if n, err := strconv.Atoi(s); err == nil {
		f := Format(n)
		if _, ok := specs[f]; ok {
			return f, nil
		}
	}
	if f, ok := nameAliases[s]; ok {
		return f, nil
	}
	return 0, fmt.Errorf("未知规则集形态 %q（支持 1~6 或 %s）", s, strings.Join(formatNames(), " / "))
}

// Label 返回形态的人类可读名。
func (f Format) Label() string {
	if s, ok := specs[f]; ok {
		return s.label
	}
	return "未知"
}

// formatNames 按固定顺序列出命名别名，用于错误信息。
func formatNames() []string {
	out := make([]string, 0, len(formatOrder))
	for _, f := range formatOrder {
		out = append(out, fmt.Sprintf("%d=%s", f, specs[f].label))
	}
	return out
}

// Filename 给出建议的下载文件名，扩展名与形态对应。
func (f Format) Filename(name string) string {
	spec, ok := specs[f]
	if !ok {
		return "ruleset.bin"
	}
	return sanitizeName(name) + spec.extension
}

// sanitizeName 去掉文件名里的路径分隔符与控制字符；空值退回默认名。
func sanitizeName(name string) string {
	n := path.Base(strings.TrimSpace(name)) // 只取末段，别把 "sub/geoip" 变成子目录
	n = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '_'
		}
		return r
	}, n)
	n = strings.TrimSpace(strings.Trim(n, " ."))
	if n == "" {
		return "ruleset"
	}
	return n
}

// Result 是一次渲染的产物与统计。
type Result struct {
	Body        []byte
	ContentType string
	Filename    string
	Format      Format
	// In 是过滤前的规则条数，Out 是实际写出的条数。
	In, Out int
	// Skipped 是目标形态无法表达而丢弃的条数（比如 DOMAIN-SET 里的域名正则）。
	Skipped int
}

// RenderOpts 是渲染时的可选取值。
type RenderOpts struct {
	// Action 是默认出站：只对被 keep 表选中、又没写第三段的规则补齐。
	// 只有 Clash 经典形态用得到——它必须每条规则都有出站，缺了内核会拒载。
	// 其它形态忽略该字段（Surge / QX 本来就默认按 PROXY 处理）。
	Action string
}

// ErrMissingAction 表示 Clash 经典规则缺出站。调用方可以拿 errors.Is 判 400。
var ErrMissingAction = errors.New("规则缺少出站")

// Render 把规则集渲染成指定形态。不在此判空：空集是调用方的输入问题，
// 交给接口层报 400，渲染层只负责把给它的东西写成合法的文档。
func (s Set) Render(f Format, name string, opts RenderOpts) (Result, error) {
	spec, ok := specs[f]
	if !ok {
		return Result{}, fmt.Errorf("未知规则集形态 %d", f)
	}
	in := len(s)
	kept := make([]Rule, 0, len(s))
	for _, r := range s {
		if spec.keep[r.Kind] {
			kept = append(kept, r)
		}
	}
	body, err := spec.render(name, kept, opts)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Body:        body,
		ContentType: spec.contentType,
		Filename:    f.Filename(name),
		Format:      f,
		In:          in,
		Out:         len(kept),
		Skipped:     in - len(kept),
	}, nil
}

// qxEname 是 Quantumult X 头部用的提供方名。
func qxEname(name string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	return "ruleset"
}

// renderSurgeList 产出 Surge 2.x 的 list 规则集。
// CNAME = 1 让 Surge 检查 CNAME 解析结果，域名规则集的标准写法。
func renderSurgeList(name string, rules []Rule, opts RenderOpts) ([]byte, error) {
	var b strings.Builder
	b.WriteString("[General]\nCNAME = 1\n\n[Rule]\n")
	for _, r := range rules {
		b.WriteString(r.String())
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// renderQuantumultX 产出 Quantumult X 远程规则提供方。
// expires 是 QX 远程提供方文档约定的缓存时长（30 天）：想要实时生效的用法
// 是自己套一层模板改成 expires = 0，这里不默认注入更激进的取值。
func renderQuantumultX(name string, rules []Rule, opts RenderOpts) ([]byte, error) {
	var b strings.Builder
	b.WriteString("[Header]\n")
	b.WriteString("name = " + qxEname(name) + "\n")
	b.WriteString("type = remote\n")
	b.WriteString("app = Quantumult X\n")
	b.WriteString("expires = 2592000\n\n")
	b.WriteString("[Rule]\n")
	for _, r := range rules {
		b.WriteString(r.String())
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

// renderClashProvider 产出 Clash provider 的 payload 段（domain / ipcidr 共用）。
// provider 只会匹配单一形态，所以这里只写取值、不写类型前缀。
func renderClashProvider(rules []Rule) ([]byte, error) {
	values := make([]string, 0, len(rules))
	for _, r := range rules {
		values = append(values, r.Value)
	}
	node, err := render.MappingNode("payload", render.StringSequence(values))
	if err != nil {
		return nil, err
	}
	return render.MarshalDoc(node)
}

// renderSurgeDomainSet 产出裸域名列表，一行一条。
func renderSurgeDomainSet(name string, rules []Rule, opts RenderOpts) ([]byte, error) {
	values := make([]string, 0, len(rules))
	for _, r := range rules {
		values = append(values, r.Value)
	}
	return []byte(strings.Join(values, "\n") + "\n"), nil
}

// renderClashClassical 产出 Clash 经典 rules 段，可以直接粘进配置。
// 不自动追加收尾规则（MATCH/FINAL）：粘到已有配置里再加一遍会与之冲突。
//
// 内核要求每条规则都带出站（缺了会报 "rules[i] error: format invalid" 拒载），
// 所以这里对缺第三段的规则要么补默认出站，要么直接报错——把失败留在我们这边，
// 比让客户端拿到一份内核拒载的配置强。
func renderClashClassical(name string, rules []Rule, opts RenderOpts) ([]byte, error) {
	lines := make([]string, 0, len(rules))
	for _, r := range rules {
		if r.needsOutbound() {
			if opts.Action == "" {
				return nil, fmt.Errorf("%w: %v，请写 \"TYPE,VALUE,组名\"，或用 out 指定默认出站",
					ErrMissingAction, r)
			}
			r.Action = opts.Action
		}
		lines = append(lines, r.String())
	}
	node, err := render.MappingNode("rules", render.StringSequence(lines))
	if err != nil {
		return nil, err
	}
	return render.MarshalDoc(node)
}
