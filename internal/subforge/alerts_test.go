package subforge

import (
	"context"
	"strings"
	"testing"
)

// TestGetAlertChannelsUnconfigured 覆盖未配置时返回非 nil 空切片。
func TestGetAlertChannelsUnconfigured(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	got, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels: %v", err)
	}
	if got == nil {
		t.Fatal("未配置时应返回非 nil 空切片")
	}
	if len(got) != 0 {
		t.Fatalf("未配置时 len = %d, want 0", len(got))
	}
}

// TestGetAlertChannelsMasksSecrets 覆盖读取脱敏、Configured 标记与返回切片独立性。
func TestGetAlertChannelsMasksSecrets(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	chans := []AlertChannel{
		{Kind: "telegram", Enabled: true, BotToken: "tok-123", ChatID: "chat-1"},
		{Kind: "dingtalk", Enabled: true, Webhook: "https://hook.example/ding"},
		{Kind: "smtp", Enabled: false, Host: "smtp.example", Port: 587, Username: "u1", Password: "pw-1", From: "a@example.com", To: "b@example.com"},
	}
	if err := m.SetAlertChannels(ctx, chans); err != nil {
		t.Fatalf("SetAlertChannels: %v", err)
	}
	got, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	tg, dd, smtp := got[0], got[1], got[2]
	if tg.BotToken != MaskedSecret {
		t.Errorf("telegram bot_token = %q, want %q", tg.BotToken, MaskedSecret)
	}
	if !tg.Configured["bot_token"] {
		t.Errorf("telegram configured.bot_token 应为 true: %v", tg.Configured)
	}
	if _, ok := tg.Configured["webhook"]; ok {
		t.Errorf("telegram 未设置 webhook，不应出现在 Configured: %v", tg.Configured)
	}
	if tg.ChatID != "chat-1" {
		t.Errorf("chat_id 应明文返回，实际 %q", tg.ChatID)
	}
	if dd.Webhook != MaskedSecret || !dd.Configured["webhook"] {
		t.Errorf("dingtalk webhook = %q, configured = %v", dd.Webhook, dd.Configured)
	}
	if smtp.Password != MaskedSecret || !smtp.Configured["password"] {
		t.Errorf("smtp password = %q, configured = %v", smtp.Password, smtp.Configured)
	}
	if smtp.Host != "smtp.example" || smtp.Port != 587 || smtp.Username != "u1" || smtp.From != "a@example.com" || smtp.To != "b@example.com" {
		t.Errorf("smtp 非密钥字段应明文返回: %+v", smtp)
	}
	// 返回切片不得与内部状态共享底层数组：改动返回值后再读应恢复原状。
	got[0].ChatID = "tampered"
	again, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels 二次: %v", err)
	}
	if again[0].ChatID != "chat-1" {
		t.Errorf("返回切片与内部状态共享底层数组: chat_id = %q", again[0].ChatID)
	}
}

// TestSetAlertChannelsKeepsMaskedSecrets 覆盖核心不变量：回写 *** 不丢真实密钥。
func TestSetAlertChannelsKeepsMaskedSecrets(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	orig := []AlertChannel{
		{Kind: "telegram", Enabled: true, BotToken: "tok-real", ChatID: "chat-1"},
		{Kind: "dingtalk", Enabled: true, Webhook: "https://hook.example/real"},
		{Kind: "smtp", Enabled: true, Host: "smtp.example", Port: 465, Username: "u1", Password: "pw-real", From: "a@example.com", To: "b@example.com"},
	}
	if err := m.SetAlertChannels(ctx, orig); err != nil {
		t.Fatalf("首次 SetAlertChannels: %v", err)
	}
	masked, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels: %v", err)
	}
	// 前端回写：原样把带 *** 的列表写回。
	if err := m.SetAlertChannels(ctx, masked); err != nil {
		t.Fatalf("回写 SetAlertChannels: %v", err)
	}
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	for _, secret := range []string{"tok-real", "https://hook.example/real", "pw-real"} {
		if !strings.Contains(raw, secret) {
			t.Errorf("回写后原始 JSON 丢失真实密钥 %q: %s", secret, raw)
		}
	}
	if strings.Contains(raw, MaskedSecret) {
		t.Errorf("原始 JSON 不应落库脱敏值 %q: %s", MaskedSecret, raw)
	}
	again, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels 二次: %v", err)
	}
	if again[0].BotToken != MaskedSecret || again[1].Webhook != MaskedSecret || again[2].Password != MaskedSecret {
		t.Errorf("回写后再读仍应脱敏: %+v", again)
	}
}

// TestSetAlertChannelsExplicitNewSecret 覆盖显式传新密钥时更新原值。
func TestSetAlertChannelsExplicitNewSecret(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.SetAlertChannels(ctx, []AlertChannel{{Kind: "telegram", Enabled: true, BotToken: "tok-old", ChatID: "c1"}}); err != nil {
		t.Fatalf("首次 SetAlertChannels: %v", err)
	}
	if err := m.SetAlertChannels(ctx, []AlertChannel{{Kind: "telegram", Enabled: true, BotToken: "tok-new", ChatID: "c1"}}); err != nil {
		t.Fatalf("二次 SetAlertChannels: %v", err)
	}
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !strings.Contains(raw, "tok-new") {
		t.Errorf("原始 JSON 应含新密钥: %s", raw)
	}
	if strings.Contains(raw, "tok-old") {
		t.Errorf("原始 JSON 不应再含旧密钥: %s", raw)
	}
}

// TestSetAlertChannelsEmptyClearsSecret 覆盖空串清空密钥。
func TestSetAlertChannelsEmptyClearsSecret(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.SetAlertChannels(ctx, []AlertChannel{{Kind: "telegram", Enabled: true, BotToken: "tok-x", ChatID: "c1"}}); err != nil {
		t.Fatalf("首次 SetAlertChannels: %v", err)
	}
	if err := m.SetAlertChannels(ctx, []AlertChannel{{Kind: "telegram", Enabled: true, ChatID: "c1"}}); err != nil {
		t.Fatalf("清空 SetAlertChannels: %v", err)
	}
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if strings.Contains(raw, "tok-x") || strings.Contains(raw, "bot_token") {
		t.Errorf("空串应清空 bot_token: %s", raw)
	}
	got, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels: %v", err)
	}
	if got[0].BotToken != "" {
		t.Errorf("清空后 bot_token = %q, want 空串", got[0].BotToken)
	}
	if _, ok := got[0].Configured["bot_token"]; ok {
		t.Errorf("清空后 Configured 不应含 bot_token: %v", got[0].Configured)
	}
}

// TestGetAlertChannelsInvalidJSON 覆盖配置损坏时报错且不覆盖原值。
func TestGetAlertChannelsInvalidJSON(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.setConfig(ctx, AlertChannelsKey, "{not-json"); err != nil {
		t.Fatalf("setConfig: %v", err)
	}
	if _, err := m.GetAlertChannels(ctx); err == nil {
		t.Fatal("非法 JSON 应返回错误")
	} else if !strings.Contains(err.Error(), "解析告警通道配置") {
		t.Errorf("错误文本 = %q, 应含 解析告警通道配置", err.Error())
	}
	if err := m.SetAlertChannels(ctx, []AlertChannel{{Kind: "telegram", Enabled: true}}); err == nil {
		t.Fatal("SetAlertChannels 遇到非法 JSON 应返回错误")
	} else if !strings.Contains(err.Error(), "解析告警通道配置") {
		t.Errorf("SetAlertChannels 错误文本 = %q", err.Error())
	}
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if raw != "{not-json" {
		t.Errorf("报错时不应覆盖原值, got %q", raw)
	}
}

// TestSetAlertChannelsMissingKind 覆盖缺少 kind 的条目报错且不落库。
func TestSetAlertChannelsMissingKind(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	err := m.SetAlertChannels(ctx, []AlertChannel{{Kind: "telegram", Enabled: true, BotToken: "t"}, {Kind: "", Enabled: true}})
	if err == nil {
		t.Fatal("缺少 kind 应返回错误")
	}
	if err.Error() != "告警通道缺少 kind" {
		t.Errorf("错误文本 = %q, want 告警通道缺少 kind", err.Error())
	}
	got, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("报错时不应写入任何通道, got %+v", got)
	}
}

// TestSetAlertChannelsReplacesList 固定「新列表即最终列表」语义：
// 不在新列表里的通道被删除；同 Kind 第 N 条的 *** 沿用旧列表同 Kind 第 N 条的真实值，
// 没有旧值的 *** 落成空串。
func TestSetAlertChannelsReplacesList(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if err := m.SetAlertChannels(ctx, []AlertChannel{
		{Kind: "telegram", Enabled: true, BotToken: "tok-1", ChatID: "c1"},
		{Kind: "dingtalk", Enabled: true, Webhook: "https://hook.example/1"},
		{Kind: "dingtalk", Enabled: false, Webhook: "https://hook.example/2"},
	}); err != nil {
		t.Fatalf("首次 SetAlertChannels: %v", err)
	}
	if err := m.SetAlertChannels(ctx, []AlertChannel{
		{Kind: "dingtalk", Enabled: true, Webhook: MaskedSecret},
		{Kind: "dingtalk", Enabled: false, Webhook: MaskedSecret},
		{Kind: "smtp", Enabled: true, Password: MaskedSecret},
	}); err != nil {
		t.Fatalf("二次 SetAlertChannels: %v", err)
	}
	raw, err := m.GetConfig(ctx, AlertChannelsKey)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if strings.Contains(raw, "telegram") || strings.Contains(raw, "tok-1") {
		t.Errorf("不在新列表里的通道应被删除: %s", raw)
	}
	if !strings.Contains(raw, "https://hook.example/1") || !strings.Contains(raw, "https://hook.example/2") {
		t.Errorf("同 Kind 第 N 条应沿用旧列表第 N 条密钥: %s", raw)
	}
	if strings.Contains(raw, "password") {
		t.Errorf("没有旧值的 *** 应落成空串: %s", raw)
	}
	got, err := m.GetAlertChannels(ctx)
	if err != nil {
		t.Fatalf("GetAlertChannels: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[2].Password != "" {
		t.Errorf("无旧值的 *** 应落成空串, got %q", got[2].Password)
	}
}
