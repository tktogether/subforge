package subforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// AlertChannelsKey 是告警通道配置在 config 表中的键。
const AlertChannelsKey = "alert.channels"

// MaskedSecret 是脱敏占位符：读取时替换真实密钥，写入时表示保持原值。
const MaskedSecret = "***"

// AlertChannel 是一条告警通道配置（plan §2.1b ⑤）。
type AlertChannel struct {
	Kind     string `json:"kind"` // telegram | dingtalk | smtp
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token,omitempty"`
	ChatID   string `json:"chat_id,omitempty"`
	Webhook  string `json:"webhook,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	// Configured 仅在脱敏读取时填充：密钥字段非空则为 true。
	Configured map[string]bool `json:"configured,omitempty"`
}

// markConfigured 记录某个密钥字段已配置。
func markConfigured(c *AlertChannel, key string) {
	if c.Configured == nil {
		c.Configured = map[string]bool{}
	}
	c.Configured[key] = true
}

// GetAlertChannels 读取告警通道并脱敏（bot_token / webhook / password）。
// 未配置时返回非 nil 空切片，保证 JSON 序列化为 [] 而不是 null。
func (m *Manager) GetAlertChannels(ctx context.Context) ([]AlertChannel, error) {
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return []AlertChannel{}, nil
	}
	var stored []AlertChannel
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, fmt.Errorf("解析告警通道配置: %w", err)
	}
	// 每次新分配，返回切片不与内部状态共享底层数组。
	out := make([]AlertChannel, 0, len(stored))
	for _, c := range stored {
		masked := c
		masked.Configured = nil
		if masked.BotToken != "" {
			masked.BotToken = MaskedSecret
			markConfigured(&masked, "bot_token")
		}
		if masked.Webhook != "" {
			masked.Webhook = MaskedSecret
			markConfigured(&masked, "webhook")
		}
		if masked.Password != "" {
			masked.Password = MaskedSecret
			markConfigured(&masked, "password")
		}
		out = append(out, masked)
	}
	return out, nil
}

// SetAlertChannels 写入告警通道：新列表即最终列表（不在新列表里的通道被删除）。
// 密钥字段等于 MaskedSecret 时按 Kind 沿用旧值（同 Kind 多条按出现顺序取第 N 条），空串则清空。
func (m *Manager) SetAlertChannels(ctx context.Context, chans []AlertChannel) error {
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		return err
	}
	var stored []AlertChannel
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &stored); err != nil {
			return fmt.Errorf("解析告警通道配置: %w", err)
		}
	}
	byKind := make(map[string][]AlertChannel, len(stored))
	for _, s := range stored {
		byKind[s.Kind] = append(byKind[s.Kind], s)
	}
	seen := make(map[string]int, len(chans))
	out := make([]AlertChannel, 0, len(chans))
	for _, c := range chans {
		if c.Kind == "" {
			return errors.New("告警通道缺少 kind")
		}
		idx := seen[c.Kind]
		seen[c.Kind] = idx + 1
		var prev AlertChannel
		if list := byKind[c.Kind]; idx < len(list) {
			prev = list[idx]
		}
		if c.BotToken == MaskedSecret {
			c.BotToken = prev.BotToken
		}
		if c.Webhook == MaskedSecret {
			c.Webhook = prev.Webhook
		}
		if c.Password == MaskedSecret {
			c.Password = prev.Password
		}
		c.Configured = nil
		out = append(out, c)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return m.setConfig(ctx, AlertChannelsKey, string(b))
}
