package subforge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tktogether/subforge/internal/store"
)

const (
	SettingKindBool   = "bool"
	SettingKindString = "string"
	SettingKindPath   = "path"
	// SettingKindSelect 是固定候选值的下拉项；Options 由列表接口按当前数据填充。
	SettingKindSelect = "select"
	// SettingKindNumber 是非负整数项；前端用数字输入框，后端按业务范围校验。
	SettingKindNumber = "number"
)

const SubscheckEnabledKey = "subscheck.enabled"

// OutboundNodeKey 是上游拉取的出网节点设置：值为自建节点 ID，空串=直连。
const OutboundNodeKey = "fetch.outbound_node"

// SourceRefreshKey 是机场原文缓存的后台刷新间隔（分钟）；0=关闭缓存与后台刷新。
const SourceRefreshKey = "fetch.refresh_minutes"

// DefaultSourceRefreshMinutes 是未配置时的默认刷新间隔。
const DefaultSourceRefreshMinutes = 30

type SettingOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type SettingItem struct {
	Key         string          `json:"key"`
	Value       string          `json:"value"`
	Kind        string          `json:"kind"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Placeholder string          `json:"placeholder,omitempty"`
	Options     []SettingOption `json:"options,omitempty"`
	IsDefault   bool            `json:"is_default"`
}

type settingDef struct {
	key         string
	kind        string
	label       string
	description string
}

var settingDefs = []settingDef{
	{ScriptEnabledKey, SettingKindBool, "启用脚本步骤", "关闭后，引用脚本的管线步骤会在编译阶段报错。"},
	{SubscheckEnabledKey, SettingKindBool, "启用订阅可用性检测", "关闭后，定时可用性检测任务不再执行。"},
	{OutboundNodeKey, SettingKindSelect, "默认出网节点", "机场与规则源未单独指定出网节点时使用；规则集与模板的拉取也用它。留空直连。节点停用或删除后自动直连。"},
	{SourceRefreshKey, SettingKindNumber, "节点缓存刷新间隔（分钟）", "间隔内订阅直接使用缓存，超过后重新拉取；后台任务每隔一分钟检查并刷新过期缓存。0 表示关闭缓存与后台刷新。"},
	{RuleManifestPathKey, SettingKindPath, "规则源清单路径", "规则源清单文件（manifest.yaml）的绝对路径。"},
	{RulesOutputDirKey, SettingKindPath, "规则源输出目录", "规则源版本文件的保存目录；留空时使用清单中的输出路径。"},
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
		if d.kind == SettingKindNumber && d.key == SourceRefreshKey {
			item.Placeholder = strconv.Itoa(DefaultSourceRefreshMinutes)
		}
		if d.kind == SettingKindSelect && d.key == OutboundNodeKey {
			nodes, nerr := m.ListSelfNodes(ctx)
			if nerr != nil {
				return nil, nerr
			}
			item.Options = []SettingOption{{Value: "", Label: "直连（默认）"}}
			for _, n := range nodes {
				label := strings.TrimSpace(n.Name)
				if label == "" {
					label = fmt.Sprintf("节点 %d", n.ID)
				}
				if !n.Enabled {
					label += "（已停用）"
				}
				item.Options = append(item.Options, SettingOption{Value: strconv.FormatInt(n.ID, 10), Label: label})
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
		if def.kind == SettingKindSelect {
			if err := m.validateSettingSelect(ctx, key, kv[key]); err != nil {
				return fmt.Errorf("设置项 %s: %w", key, err)
			}
		}
		if def.kind == SettingKindNumber {
			if err := validateSettingNumber(key, kv[key]); err != nil {
				return fmt.Errorf("设置项 %s: %w", key, err)
			}
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

// validateSettingNumber 校验数字设置项的取值。
func validateSettingNumber(key, value string) error {
	switch key {
	case SourceRefreshKey:
		if value == "" {
			return nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 10080 {
			return fmt.Errorf("需要 0-10080 的整数（分钟）")
		}
	}
	return nil
}

// validateSettingSelect 校验下拉项的候选值：候选项由当前数据生成，
// 写入时只拦下明显失效的值，避免把不存在的节点静默存进去。
func (m *Manager) validateSettingSelect(ctx context.Context, key, value string) error {
	switch key {
	case OutboundNodeKey:
		if value == "" {
			return nil
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("节点 ID 不合法")
		}
		if _, err := m.GetSelfNode(ctx, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("自建节点 %d 不存在", id)
			}
			return err
		}
	}
	return nil
}

// SourceRefreshInterval 返回机场缓存/后台刷新间隔；0 表示关闭。
// 未配置时返回默认 30 分钟。
func (m *Manager) SourceRefreshInterval(ctx context.Context) (time.Duration, error) {
	v, err := m.GetConfig(ctx, SourceRefreshKey)
	if err != nil {
		return 0, err
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Duration(DefaultSourceRefreshMinutes) * time.Minute, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 10080 {
		return 0, fmt.Errorf("设置项 %s 需要 0-10080 的整数（分钟）", SourceRefreshKey)
	}
	return time.Duration(n) * time.Minute, nil
}

// GetSourceCache 读取机场原文缓存（convert.SourceCache 实现）。
func (m *Manager) GetSourceCache(ctx context.Context, url string) ([]byte, time.Time, bool, error) {
	return m.db.GetSourceCache(ctx, url)
}

// PutSourceCache 写入机场原文缓存（convert.SourceCache 实现）。
func (m *Manager) PutSourceCache(ctx context.Context, url string, content []byte) error {
	return m.db.PutSourceCache(ctx, url, content)
}

// OutboundNodeFor 解析来源级出网节点：""=跟随设置页默认，direct=强制直连，
// 数字=自建节点；节点失效（删除/停用/空链接）时回落默认。
func (m *Manager) OutboundNodeFor(ctx context.Context, raw string) (store.SelfNode, error) {
	v := strings.TrimSpace(raw)
	if v == "direct" {
		return store.SelfNode{}, nil
	}
	if v == "" {
		return m.OutboundNode(ctx)
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return m.OutboundNode(ctx)
	}
	n, err := m.GetSelfNode(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return m.OutboundNode(ctx)
	}
	if err != nil {
		return store.SelfNode{}, err
	}
	if n == nil || !n.Enabled || strings.TrimSpace(n.Link) == "" {
		return m.OutboundNode(ctx)
	}
	return *n, nil
}

// OutboundNode 返回当前设置的出网自建节点。
// 未配置、节点已停用或已删除时返回零值，调用方按直连处理。
func (m *Manager) OutboundNode(ctx context.Context) (store.SelfNode, error) {
	v, err := m.GetConfig(ctx, OutboundNodeKey)
	if err != nil {
		return store.SelfNode{}, err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || id <= 0 {
		return store.SelfNode{}, nil
	}
	n, err := m.GetSelfNode(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SelfNode{}, nil
	}
	if err != nil {
		return store.SelfNode{}, err
	}
	if n == nil || !n.Enabled || strings.TrimSpace(n.Link) == "" {
		return store.SelfNode{}, nil
	}
	return *n, nil
}
