package subforge

import (
	"context"
	"fmt"
	"strings"
)

const (
	SettingKindBool   = "bool"
	SettingKindString = "string"
	SettingKindPath   = "path"
)

const SubscheckEnabledKey = "subscheck.enabled"

type SettingItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Placeholder string `json:"placeholder,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

type settingDef struct {
	key         string
	kind        string
	label       string
	description string
}

var settingDefs = []settingDef{
	{ScriptEnabledKey, SettingKindBool, "启用脚本步骤", "关闭时管线里的 ref= 脚本步骤会在编译阶段报错"},
	{SubscheckEnabledKey, SettingKindBool, "启用订阅测活", "关闭后定时测活任务不再执行"},
	{RuleManifestPathKey, SettingKindPath, "规则源清单路径", "manifest.yaml 的绝对路径"},
	{RulesOutputDirKey, SettingKindPath, "规则源输出目录", "规则集版本文件的落盘目录（留空则用 manifest 里的 output.path）"},
}

func findSettingDef(key string) (settingDef, bool) {
	for _, d := range settingDefs {
		if d.key == key {
			return d, true
		}
	}
	return settingDef{}, false
}

func normalizeBoolForList(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return "true", nil
	case "0", "false", "no", "off", "":
		return "false", nil
	default:
		return "", fmt.Errorf("非法的布尔值 %q", v)
	}
}

func parseBoolForSet(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return "true", nil
	case "0", "false", "no", "off":
		return "false", nil
	default:
		return "", fmt.Errorf("非法的布尔值 %q（须为 1/0/true/false/yes/no/on/off）", v)
	}
}

func (m *Manager) ListSettings(ctx context.Context) ([]SettingItem, error) {
	out := make([]SettingItem, 0, len(settingDefs))
	for _, d := range settingDefs {
		v, err := m.GetConfig(ctx, d.key)
		if err != nil {
			return nil, err
		}
		isDefault := strings.TrimSpace(v) == ""
		if d.kind == SettingKindBool {
			nb, err := normalizeBoolForList(v)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s: %w", d.key, err)
			}
			v = nb
		} else {
			v = strings.TrimSpace(v)
		}
		item := SettingItem{
			Key:         d.key,
			Value:       v,
			Kind:        d.kind,
			Label:       d.label,
			Description: d.description,
			IsDefault:   isDefault,
		}
		if d.kind == SettingKindPath {
			if d.key == RuleManifestPathKey {
				if rp, rerr := m.ResolveRuleManifestPath(ctx); rerr == nil {
					item.Placeholder = rp
				}
			} else if d.key == RulesOutputDirKey {
				if rp, rerr := m.ResolveRulesOutputDir(ctx); rerr == nil {
					item.Placeholder = rp
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (m *Manager) SetSettings(ctx context.Context, kv map[string]string) error {
	// 先全量校验，再写，保持原子性。
	for key, raw := range kv {
		def, ok := findSettingDef(key)
		if !ok {
			return fmt.Errorf("未知设置项 %q", key)
		}
		if def.kind == SettingKindBool {
			nb, err := parseBoolForSet(raw)
			if err != nil {
				return fmt.Errorf("设置项 %s: %w", key, err)
			}
			kv[key] = nb
		} else {
			kv[key] = strings.TrimSpace(raw)
		}
	}
	for key, v := range kv {
		if strings.TrimSpace(v) == "" {
			if err := m.deleteConfig(ctx, key); err != nil {
				return err
			}
			continue
		}
		if err := m.setConfig(ctx, key, v); err != nil {
			return err
		}
	}
	return nil
}
