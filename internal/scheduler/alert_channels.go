package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TelegramChannel 通过 Telegram Bot API 发送告警。
type TelegramChannel struct {
	BotToken string
	ChatID   string
	client   *http.Client
}

func NewTelegramChannel(botToken, chatID string) *TelegramChannel {
	return &TelegramChannel{
		BotToken: botToken,
		ChatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TelegramChannel) Name() string { return "telegram" }

func (t *TelegramChannel) Send(ctx context.Context, msg string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	data := url.Values{
		"chat_id": {t.ChatID},
		"text":    {msg},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("Telegram 发送: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram 响应码 %d", resp.StatusCode)
	}
	return nil
}

// DingTalkChannel 通过钉钉自定义机器人 Webhook 发送告警。
type DingTalkChannel struct {
	WebhookURL string
	client     *http.Client
}

func NewDingTalkChannel(webhookURL string) *DingTalkChannel {
	return NewDingTalkChannelWithClient(webhookURL, nil)
}

// NewDingTalkChannelWithClient 允许调用方传入带出站地址防护的客户端。
func NewDingTalkChannelWithClient(webhookURL string, client *http.Client) *DingTalkChannel {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &DingTalkChannel{WebhookURL: webhookURL, client: client}
}

func (d *DingTalkChannel) Name() string { return "dingtalk" }

func (d *DingTalkChannel) Send(ctx context.Context, msg string) error {
	payload := fmt.Sprintf(`{"msgtype":"text","text":{"content":%q}}`, msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.WebhookURL, bytes.NewBufferString(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("DingTalk 发送: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DingTalk 响应码 %d", resp.StatusCode)
	}
	return nil
}

// SMTPChannel 占位：SMTP 告警通道（后续补齐 net/smtp 实现）。
// 设计 §2.3 要求 Telegram / DingTalk / SMTP 三通道，前两者已实现，
// SMTP 个人场景使用频率低，先留接口位。
type SMTPChannel struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       string
}

func (s *SMTPChannel) Name() string { return "smtp" }

func (s *SMTPChannel) Send(_ context.Context, _ string) error {
	return fmt.Errorf("SMTP 通道尚未实现，请使用 Telegram 或 DingTalk")
}
