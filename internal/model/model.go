// Package model 定义 SubForge 的内部领域模型。
//
// 核心约定（见 docs/adr/0001）：节点的唯一事实源是原始 Clash 配置 map
// （map[string]any）。内核 outbound 适配器没有 Marshal 方法，无法从适配器还原配置，
// 因此渲染一律走 raw map 往返；Node 上的 Name/Type/Server/Port 只是只读派生视图，
// 供过滤、排序、去重使用，改配置必须改 Raw 再重新派生。
package model

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Node 是一个代理节点。
type Node struct {
	// Raw 原始 Clash 配置 map，渲染的唯一事实源。
	Raw map[string]any
	// Name 派生自 Raw["name"]。
	Name string
	// Type 派生自 Raw["type"]（如 vmess/vless/ss/ssr/trojan/hysteria2/anytls/tuic）。
	Type string
	// Server 派生自 Raw["server"]。
	Server string
	// Port 派生自 Raw["port"]，统一为字符串（内核可能给出字符串或数字）。
	Port string
	// Source 标记节点来源（订阅名或导入批次），用于溯源与调试。
	Source string
}

// NodeFromRaw 从原始配置 map 构造节点，派生只读视图字段。
func NodeFromRaw(raw map[string]any, source string) Node {
	return Node{
		Raw:    raw,
		Name:   MapString(raw, "name"),
		Type:   MapString(raw, "type"),
		Server: MapString(raw, "server"),
		Port:   MapString(raw, "port"),
		Source: source,
	}
}

// PortInt 返回端口整数；缺失或非法时返回错误。
func (n Node) PortInt() (int, error) {
	s := strings.TrimSpace(n.Port)
	if s == "" {
		return 0, fmt.Errorf("节点 %q 缺 port", n.Name)
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("节点 %q 的 port %q 非法", n.Name, n.Port)
	}
	return v, nil
}

// Fingerprint 返回去重/分组用的身份指纹：类型 + 服务器 + 端口 + 协议凭据。
// 对齐设计书 Q1 的归一化指纹思路（服务器 + 端口 + 协议 + 凭据）。
func (n Node) Fingerprint() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s", n.Type, n.Server, n.Port)
	for _, k := range []string{"uuid", "id", "password", "key", "secret"} {
		if v := MapString(n.Raw, k); v != "" {
			fmt.Fprintf(&b, "|%s=%s", k, v)
		}
	}
	return b.String()
}

// NodeList 是一组节点；顺序敏感（渲染结果受顺序影响）。
type NodeList []Node

// NewNodeList 从原始配置 map 批量构造节点。
func NewNodeList(raws []map[string]any, source string) NodeList {
	out := make(NodeList, len(raws))
	for i, raw := range raws {
		out[i] = NodeFromRaw(raw, source)
	}
	return out
}

// Len 返回节点数。
func (ns NodeList) Len() int { return len(ns) }

// IsEmpty 报告是否为空。
func (ns NodeList) IsEmpty() bool { return len(ns) == 0 }

// Raws 按原顺序返回原始配置 map（渲染入口）。
func (ns NodeList) Raws() []map[string]any {
	out := make([]map[string]any, len(ns))
	for i, n := range ns {
		out[i] = n.Raw
	}
	return out
}

// Names 按原顺序返回节点名。
func (ns NodeList) Names() []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Name
	}
	return out
}

// DistinctTypes 返回去重并按字典序排列的协议类型列表。
func (ns NodeList) DistinctTypes() []string {
	seen := make(map[string]bool)
	for _, n := range ns {
		if n.Type != "" {
			seen[n.Type] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Filter 返回满足谓词的子集（保持原顺序）。
func (ns NodeList) Filter(pred func(Node) bool) NodeList {
	var out NodeList
	for _, n := range ns {
		if pred(n) {
			out = append(out, n)
		}
	}
	return out
}

// Dedupe 保留首次出现的节点，按 Fingerprint 去重。
func (ns NodeList) Dedupe() NodeList {
	seen := make(map[string]bool, len(ns))
	var out NodeList
	for _, n := range ns {
		key := n.Fingerprint()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, n)
	}
	return out
}

// MapString 把 map 中的值归一化为字符串，便于统一处理内核的类型抖动
// （"port": "443" 与 "port": 443 均可能出现）。
func MapString(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		if s, ok := v.(fmt.Stringer); ok {
			return s.String()
		}
		return fmt.Sprint(v)
	}
}

// NewNodeFromParts 由离散字段构造一个最小节点（测试与合成夹具用）。
func NewNodeFromParts(name, typ, server, port string, extra map[string]any) Node {
	raw := map[string]any{
		"name":   name,
		"type":   typ,
		"server": server,
		"port":   port,
	}
	for k, v := range extra {
		raw[k] = v
	}
	return NodeFromRaw(raw, "")
}

// ParseClashProxies 从 Clash/mihomo YAML 配置中提取 proxies 列表。
// 直接解析为 []map[string]any，与分享链接解析产物同构——两种订阅来源共享同一事实源，
// 渲染与过滤逻辑因此不需要区分来源。该函数不做任何字段归一化。
func ParseClashProxies(yamlBuf []byte) ([]map[string]any, error) {
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(yamlBuf, &cfg); err != nil {
		return nil, fmt.Errorf("解析 YAML: %w", err)
	}
	if len(cfg.Proxies) == 0 {
		return nil, errors.New("配置中不含 proxies")
	}
	return cfg.Proxies, nil
}
