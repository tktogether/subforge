// Package parse 是 SubForge 中唯一 import mihomo 内核的包（见 docs/adr/0001）。
//
// 职责边界：
//   - 订阅原文 / 分享链接 → Clash 配置 map（原始 map 即内部节点模型的事实源，全程不转结构体）
//   - 用内核校验配置 map 的合法性（构建运行期代理，不发起网络连接）
//
// 设计约束：内核 outbound 适配器没有 Marshal 方法（v1.19.31 实读），
// 因此渲染永远走原始 map 往返；其余包一律不 import 内核，只依赖 map 与自有模型。
package parse

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	mihomoadapter "github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/convert"

	"github.com/tktogether/subforge/internal/model"
)

// ErrEmptySubscription 表示订阅内容中没有解析出任何节点。
var ErrEmptySubscription = errors.New("订阅中未解析出任何节点")

// ConvertSubscription 将订阅原文（分享链接，可整体为 Base64）转换为 Clash 配置 map 列表。
// 支持的协议由内核 common/convert 决定：vmess / vless(含 reality) / hysteria / hysteria2 /
// tuic / trojan / ss / ssr / socks(5) / http(s) / anytls / mierus。
func ConvertSubscription(buf []byte) ([]map[string]any, error) {
	nodes, err := convert.ConvertsV2Ray(buf)
	if err != nil {
		return nil, fmt.Errorf("解析订阅内容: %w", err)
	}
	if len(nodes) == 0 {
		return nil, ErrEmptySubscription
	}
	return nodes, nil
}

// Validate 用内核构建运行期代理，校验单个 Clash 配置 map 是否被内核接受。
// 不发起网络连接，仅做结构校验与构造。
func Validate(mapping map[string]any) error {
	if _, err := mihomoadapter.ParseProxy(mapping); err != nil {
		return fmt.Errorf("节点 %q: %w", NameOf(mapping), err)
	}
	return nil
}

// Format 表示订阅内容解码后的类型。
type Format int

const (
	// FormatLinks 明文分享链接（换行分隔）。
	FormatLinks Format = iota
	// FormatClashYAML Clash/mihomo YAML 配置（含 proxies 列表）。
	FormatClashYAML
)

// String 返回格式名（日志与 API 用）。
func (f Format) String() string {
	switch f {
	case FormatLinks:
		return "links"
	case FormatClashYAML:
		return "clash-yaml"
	default:
		return "unknown"
	}
}

// SniffResult 是订阅内容形态识别结果。
type SniffResult struct {
	// Encoded 外层是否为整体 Base64 编码。
	Encoded bool
	// Format 解码后的内容类型。
	Format Format
	// Decoded 解码后的内容（Encoded=false 时为去空白后的原文）。
	Decoded []byte
}

// Sniff 识别订阅内容的编码形态与内层类型。
// 判定顺序：明文链接 → Clash/YAML 配置 → 整体 Base64（递归识别内层）。
// 识别失败返回 error，调用方应终止该来源（而不是带着不可辨认内容继续）。
func Sniff(buf []byte) (SniffResult, error) {
	b := bytes.TrimSpace(buf)
	if len(b) == 0 {
		return SniffResult{}, ErrEmptySubscription
	}

	line, _, _ := strings.Cut(string(b), "\n")
	if strings.Contains(line, "://") {
		return SniffResult{Format: FormatLinks, Decoded: b}, nil
	}
	if strings.Contains(line, ":") {
		return SniffResult{Format: FormatClashYAML, Decoded: b}, nil
	}

	if decoded, ok := tryBase64Blob(b); ok {
		if res, err := Sniff(decoded); err == nil {
			res.Encoded = true
			return res, nil
		}
	}
	return SniffResult{}, fmt.Errorf("无法识别订阅内容形态（前 64 字节: %q）", preview(b))
}

// Decoded 是一次来源解码的完整结果：节点 + 透传的 proxy-provider 定义 + 内置规则 + rule-provider 定义。
type Decoded struct {
	// Nodes 解析出的节点（原始 config map）。
	Nodes []map[string]any
	// Providers Clash YAML 里的 proxy-providers 定义；分享链接来源恒为空。
	Providers []model.Provider
	// Rules Clash YAML 里的 rules 规则行；分享链接来源恒为空。
	// 底层能力：供后续 SQLite 持久化导入扫描复用，不在转换路径上隐式合并。
	Rules []string
	// RuleProviders Clash YAML 里的 rule-providers 定义；分享链接来源恒为空。
	RuleProviders []model.RuleProvider
	// ProxyGroups Clash YAML 里的 proxy-groups 定义；分享链接来源恒为空。
	// 导入时按成员落库，避免机场自带分组在解析阶段丢失。
	ProxyGroups []model.UpstreamGroup
	// Warnings 解析告警（如 rule-providers 提取失败）；非阻断。
	Warnings []string
}

// DecodeSubscription 把订阅原文解析为节点列表（原始 config map），自动识别
// 明文链接 / 整体 Base64 链接 / Clash YAML 配置。fetch → model 管线统一走这里。
func DecodeSubscription(buf []byte) ([]map[string]any, error) {
	d, err := DecodeSource(buf)
	if err != nil {
		return nil, err
	}
	return d.Nodes, nil
}

// DecodeSource 在 DecodeSubscription 基础上额外提取 Clash YAML 里的 proxy-providers，
// 供 Engine 原样透传到产物（mihomo 的 provider 字段兼容）。分享链接来源没有 provider 概念。
func DecodeSource(buf []byte) (Decoded, error) {
	sniff, err := Sniff(buf)
	if err != nil {
		return Decoded{}, err
	}
	switch sniff.Format {
	case FormatClashYAML:
		proxies, err := model.ParseClashProxies(sniff.Decoded)
		if err != nil {
			return Decoded{}, fmt.Errorf("解析 Clash YAML: %w", err)
		}
		// provider 解析失败不该拖垮节点：节点是主体，provider 只是附加透传。
		providers, _ := model.ParseClashProviders(sniff.Decoded)
		// 内置规则同样不该拖垮节点：规则是附加项，提取失败只记 nil。
		rules, _ := model.ParseClashRules(sniff.Decoded)
		// rule-providers 是附加项：解析失败只记告警，不拖垮节点。
		ruleProviders, err := model.ParseClashRuleProviders(sniff.Decoded)
		var warnings []string
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("解析 rule-providers 失败: %v", err))
		}
		// proxy-groups 同样只是附加项：解析失败不拖垮节点。
		proxyGroups, _ := model.ParseClashProxyGroups(sniff.Decoded)
		return Decoded{Nodes: proxies, Providers: providers, Rules: rules, RuleProviders: ruleProviders,
			ProxyGroups: proxyGroups, Warnings: warnings}, nil
	default:
		nodes, err := convert.ConvertsV2Ray(sniff.Decoded)
		if err != nil {
			return Decoded{}, fmt.Errorf("解析订阅内容: %w", err)
		}
		if len(nodes) == 0 {
			return Decoded{}, ErrEmptySubscription
		}
		return Decoded{Nodes: nodes}, nil
	}
}

// NameOf 从原始配置 map 取节点名（校验与错误消息用）。
func NameOf(mapping map[string]any) string {
	if s, ok := mapping["name"].(string); ok {
		return s
	}
	return fmt.Sprint(mapping["name"])
}

func tryBase64Blob(b []byte) ([]byte, bool) {
	s := strings.TrimSpace(string(b))
	// 单块 Base64（订阅常见形态）；多行 blob 不识别，避免把链接列表误判为编码内容。
	if len(s) < 16 || strings.ContainsAny(s, "\r\n") {
		return nil, false
	}
	if !isBase64Alphabet(s) {
		return nil, false
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		if out, err := enc.Strict().DecodeString(s); err == nil {
			return out, true
		}
	}
	return nil, false
}

func isBase64Alphabet(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}

func preview(b []byte) string {
	if len(b) > 64 {
		b = b[:64]
	}
	return string(b)
}
