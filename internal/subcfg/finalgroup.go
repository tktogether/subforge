// Package subcfg 保存订阅方案的可持久化配置。
//
// final_group 是「统一最终选择组」配置：把各机场/模板/规则源的最终节点选择组
// 收敛为一个组。配置以 JSON 文本存在 profiles.final_group，空串表示关闭。
package subcfg

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// DefaultFinalGroupName 是 auto 模式使用的标准组名，与主流 Clash 模板一致。
const DefaultFinalGroupName = "🚀 节点选择"

// FinalGroupEnabled 是模式取值。
const (
	FinalGroupModeAuto   = "auto"
	FinalGroupModeCustom = "custom"
)

// FinalGroupConfig 描述一次统一最终选择组的配置。
type FinalGroupConfig struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Name    string `json:"name"`
	// IncludeDirect 让统一组包含 DIRECT，保留手动直连入口。
	IncludeDirect bool `json:"include_direct"`
	// KeepSourceTerminals 把各来源最终组保留为统一组的子选项（默认关闭）。
	KeepSourceTerminals bool `json:"keep_source_terminals"`
	// Overrides 按来源覆盖识别结果：
	//   group:<source> -> 该来源的最终组名（["none"] 表示不处理）
	//   rule:<规则源名> -> 该规则源的选择目标名（["none"] 表示不处理）
	Overrides map[string][]string `json:"overrides,omitempty"`
}

// DefaultFinalGroupConfig 返回关闭状态的默认配置；JSON 响应始终有完整形状。
func DefaultFinalGroupConfig() FinalGroupConfig {
	return FinalGroupConfig{
		Mode:          FinalGroupModeAuto,
		Name:          DefaultFinalGroupName,
		IncludeDirect: true,
		Overrides:     map[string][]string{},
	}
}

// Normalize 校验并归一化配置；空字段按默认值补齐。
func (c *FinalGroupConfig) Normalize() error {
	if c.Mode == "" {
		c.Mode = FinalGroupModeAuto
	}
	c.Mode = strings.ToLower(strings.TrimSpace(c.Mode))
	switch c.Mode {
	case FinalGroupModeAuto, FinalGroupModeCustom:
	default:
		return fmt.Errorf("统一选择组模式必须是 auto 或 custom")
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		if c.Mode == FinalGroupModeCustom && c.Enabled {
			return fmt.Errorf("自定义统一选择组名称不能为空")
		}
		c.Name = DefaultFinalGroupName
	}
	if err := validateGroupName(c.Name); err != nil {
		return err
	}
	c.Overrides = normalizeOverrides(c.Overrides)
	return nil
}

// ResolvedName 返回最终使用的统一组名。
// auto 模式恒用标准名：name 字段只用于界面回显，不能影响系统自动指定的结果。
func (c FinalGroupConfig) ResolvedName() string {
	if c.Mode == FinalGroupModeCustom {
		if name := strings.TrimSpace(c.Name); name != "" {
			return name
		}
	}
	return DefaultFinalGroupName
}

// OverrideFor 返回指定来源的覆盖名单；ok=false 表示没有配置覆盖。
func (c FinalGroupConfig) OverrideFor(key string) ([]string, bool) {
	if c.Overrides == nil {
		return nil, false
	}
	v, ok := c.Overrides[key]
	return v, ok
}

// ParseFinalGroup 解析库里的 JSON 文本；空串返回关闭状态的默认配置。
func ParseFinalGroup(raw string) (FinalGroupConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultFinalGroupConfig(), nil
	}
	var c FinalGroupConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return DefaultFinalGroupConfig(), fmt.Errorf("解析 final_group 配置失败: %w", err)
	}
	if err := c.Normalize(); err != nil {
		return DefaultFinalGroupConfig(), err
	}
	return c, nil
}

// Marshal 把配置编码为库内 JSON 文本。
func (c FinalGroupConfig) Marshal() (string, error) {
	if err := c.Normalize(); err != nil {
		return "", err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// RuleSourceMeta 是规则源产物里与「最终选择组」相关的元数据。
// 规则行本身仍由 RuleSourceRules 返回；这里只带识别所需的统计。
type RuleSourceMeta struct {
	Name         string         `json:"name"`
	RuleCount    int            `json:"rule_count"`
	FinalTargets []string       `json:"final_targets,omitempty"`
	TargetCounts map[string]int `json:"target_counts,omitempty"`
}

// IsNoneOverride 报告覆盖名单是否表示「不处理该来源」。
func IsNoneOverride(names []string) bool {
	for _, n := range names {
		switch strings.ToLower(strings.TrimSpace(n)) {
		case "none", "-", "off", "不处理":
			return true
		}
	}
	return false
}

func normalizeOverrides(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(in))
	for rawKey, rawNames := range in {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		seen := make(map[string]bool, len(rawNames))
		names := make([]string, 0, len(rawNames))
		for _, raw := range rawNames {
			name := strings.TrimSpace(raw)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
		if len(names) > 0 {
			out[key] = names
		}
	}
	return out
}

func validateGroupName(name string) error {
	if utf8.RuneCountInString(name) > 100 {
		return fmt.Errorf("统一选择组名称不能超过 100 个字符")
	}
	if strings.ContainsAny(name, ",\r\n") {
		return fmt.Errorf("统一选择组名称不能包含逗号或换行")
	}
	for _, r := range name {
		if r < 0x20 && r != '\t' {
			return fmt.Errorf("统一选择组名称包含控制字符")
		}
	}
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE":
		return fmt.Errorf("统一选择组名称不能是内核内置出站 %q", name)
	}
	return nil
}
