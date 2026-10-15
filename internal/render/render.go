// Package render 把节点列表渲染成目标客户端配置（Clash/mihomo、Surge、SSTap…）。
//
// 约定（见 docs/adr/0001）：渲染只读 Node.Raw，绝不在渲染时改写节点内容；
// 节点改名/去重必须在 transform 阶段完成。输出必须是确定性的（键顺序稳定），
// 否则 golden 测试无法工作。
package render

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tktogether/subforge/internal/model"
)

// Request 是一次渲染请求的全部输入。
type Request struct {
	// Nodes 已过滤、已改名、已排序的节点。
	Nodes model.NodeList
	// Groups 代理组定义；为空时由渲染器给出最小可用默认组。
	Groups []Group
	// Rules 规则行（原样透传，如 "GEOIP,CN,DIRECT"）；为空时用最小可用默认规则。
	Rules []string
	// Profile 配置元信息，用于输出注释头。
	Profile Profile
}

// Profile 是配置来源与生成信息的展示用元数据。
type Profile struct {
	// Name 配置名（写入注释头）。
	Name string
	// Version SubForge 版本。
	Version string
	// GeneratedAt 生成时间（RFC3339）。
	GeneratedAt string
}

// UrlTestOptions 是 url-test/fallback 组的可选参数。
type UrlTestOptions struct {
	// Interval 测活间隔秒数（0 表示省略）。
	Interval int
	// Tolerance 容忍延迟毫秒（0 表示省略）。
	Tolerance int
	// Idle 无连接后重新测试的秒数（0 表示省略）。
	Idle int
	// Timeout 单次测活超时毫秒（0 表示省略）。
	Timeout int
}

// Group 是一个代理组。
type Group struct {
	// Name 组名。
	Name string
	// Type select / url-test / fallback / relay / direct / pass / reject。
	Type string
	// Proxies 组成员（节点名或上游组名）。
	Proxies []string
	// Strategy url-test 的负载均衡策略：global / compatible。
	Strategy string
	// Test 测活地址（url-test/fallback 可用）。
	Test string
	// Options 测活参数。
	Options UrlTestOptions
	// Lazy 延迟加载（url-test 系）。
	Lazy bool
}

// Renderer 把一个 Request 渲染成一种目标格式。
type Renderer interface {
	// Name 目标名（如 "clash.meta"）。
	Name() string
	// Render 产出配置文本。
	Render(req Request) ([]byte, error)
}

// WarningReporter 由会跳过无法表达的节点的渲染器实现（list 模式遇到不支持的协议、
// Surge 丢掉表达不了的字段等）。渲染器是每次请求新建的，字段里带本次结果即可。
//
// 降级不能静默：跳过的节点名必须回传给调用方当作警告，否则用户会以为节点都在。
type WarningReporter interface {
	// Warnings 返回最近一次 Render 中被跳过/降级的说明。
	Warnings() []string
}

// --- 确定性 YAML 构造助手 ---
//
// yaml.v3 对 map[string]any 的键顺序不做保证，而 golden 测试要求字节级稳定，
// 因此这里手工构造 yaml.Node 树，键顺序完全由代码决定。

// scalarNode 把任意值转成标量节点（先走 YAML 序列化，再取回节点，保证转义正确）。
func scalarNode(v any) (yaml.Node, error) {
	buf, err := yaml.Marshal(v)
	if err != nil {
		return yaml.Node{}, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(buf, &doc); err != nil {
		return yaml.Node{}, err
	}
	// Unmarshal 出来的顶层节点恒为 DocumentNode，取第一个子节点才是真正的值。
	if len(doc.Content) == 0 {
		return doc, nil
	}
	out := *doc.Content[0]
	plainStyle(&out)
	return out, nil
}

// MappingNode 按给定顺序构造映射（键值交替）。
func MappingNode(entries ...any) (yaml.Node, error) {
	node := yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(entries); i += 2 {
		key, err := scalarNode(entries[i])
		if err != nil {
			return yaml.Node{}, fmt.Errorf("键 %v: %w", entries[i], err)
		}
		val, err := scalarNode(entries[i+1])
		if err != nil {
			return yaml.Node{}, fmt.Errorf("键 %v 的值: %w", entries[i], err)
		}
		k, v := key, val
		node.Content = append(node.Content, &k, &v)
	}
	plainStyle(&node)
	return node, nil
}

// SequenceNode 构造序列（子节点保持传入顺序）。
func SequenceNode(items ...*yaml.Node) yaml.Node {
	node := yaml.Node{Kind: yaml.SequenceNode, Content: items}
	plainStyle(&node)
	return node
}

// StringSequence 构造字符串序列（规则行等）。不带 Tag，让编码器自行推断类型。
func StringSequence(items []string) yaml.Node {
	children := make([]*yaml.Node, 0, len(items))
	for _, s := range items {
		n := yaml.Node{Kind: yaml.ScalarNode, Value: s}
		children = append(children, &n)
	}
	return SequenceNode(children...)
}

// plainStyle 递归清除节点风格，交给编码器统一决定引号、缩进与折叠方式。
// Tag 必须保留：它记录了标量的原始类型（"443" 是字符串、443 是整数），
// 清掉会导致编码后类型漂移，破坏"原始配置即事实源"的往返一致性。
func plainStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		plainStyle(c)
	}
}

// MarshalDoc 把根映射编码为 YAML 文本。
// 注意：不能手工包一层 DocumentNode 再 Marshal——yaml.v3 会报
// "expected SCALAR ... but got document start"（Marshal 自己负责文档层）。
func MarshalDoc(root yaml.Node) ([]byte, error) {
	r := root
	plainStyle(&r)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // 对齐主流客户端配置习惯
	if err := enc.Encode(&r); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return unescapeAstral(buf.Bytes()), nil
}

// astralEscape 匹配 yaml.v3 输出的补充平面字符转义（\U 后跟 8 位十六进制）。
var astralEscape = regexp.MustCompile(`\\U[0-9A-Fa-f]{8}`)

// unescapeAstral 把 yaml.v3 的补充平面转义还原为 UTF-8 原文。
// yaml.v3 v3.0.1 的 is_printable 只识别到 3 字节 UTF-8 序列，遇到 emoji 这类
// 4 字节字符就判定为"特殊字符"，只能输出 "\U0001F680 节点选择" 这种形式。
// 转义本身合法，但订阅转换产物要给用户直接阅读，观感不能这么差，
// 因此在编码完成后按模式还原（保留双引号，仍是合法 YAML）。
func unescapeAstral(body []byte) []byte {
	return astralEscape.ReplaceAllFunc(body, func(m []byte) []byte {
		c, err := strconv.ParseUint(string(m[2:]), 16, 32)
		if err != nil || c > 0x10FFFF {
			return m
		}
		return []byte(string(rune(c)))
	})
}

// commentBlock 生成配置头注释。
func CommentBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(strings.Join(lines, "\n# "))
	b.WriteString("\n")
	return b.String()
}
