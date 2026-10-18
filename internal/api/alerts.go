package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/scheduler"
	"github.com/tktogether/subforge/internal/subforge"
)

// handleAPIAlertChannels 处理 GET/PUT /api/alert-channels。
//
// GET 返回脱敏后的通道列表（未配置时为 [] 而不是 null）。
// PUT 整体替换：新列表即最终列表；密钥字段为 "***" 时沿用同 Kind 旧值。
func (s *Server) handleAPIAlertChannels(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		chans, err := m.GetAlertChannels(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"channels": chans})
	case http.MethodPut:
		var req struct {
			Channels []subforge.AlertChannel `json:"channels"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := m.SetAlertChannels(r.Context(), req.Channels); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		chans, err := m.GetAlertChannels(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"channels": chans})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT"))
	}
}

// alertTestReq 是 POST /api/alert-channels/test 的请求体。
// 两种形态：{"index":N} 取已存第 N 条；或直接给一条通道字段（与 AlertChannel 同构）。
type alertTestReq struct {
	Index    int    `json:"index"`
	Kind     string `json:"kind"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	Webhook  string `json:"webhook"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// handleAPIAlertChannelTest 处理 POST /api/alert-channels/test。
// 这是连通性测试：Send 失败也返回 200，错误写进 {"ok":false,"error":...}。
func (s *Server) handleAPIAlertChannelTest(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	var req alertTestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	c, ok := s.alertChannelForTest(w, m, r, &req)
	if !ok {
		return
	}
	ch, err := newSchedulerAlertChannel(c)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	resp := alertTestResp{Channel: ch.Name()}
	if err := ch.Send(r.Context(), "SubForge 告警通道测试消息"); err != nil {
		resp.OK = false
		resp.Error = err.Error()
	} else {
		resp.OK = true
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// alertChannelForTest 根据请求体解析出要测试的通道配置。
// 返回 ok=false 表示已写出错误响应（下标越界等），调用方应直接返回。
func (s *Server) alertChannelForTest(w http.ResponseWriter, m *subforge.Manager, r *http.Request, req *alertTestReq) (subforge.AlertChannel, bool) {
	if req.Kind != "" {
		return subforge.AlertChannel{
			Kind: req.Kind, BotToken: req.BotToken, ChatID: req.ChatID,
			Webhook: req.Webhook, Host: req.Host, Port: req.Port,
			Username: req.Username, Password: req.Password, From: req.From, To: req.To,
		}, true
	}
	// index 形态：读取未脱敏的原始列表（脱敏值 "***" 无法用于真实发送）。
	raw, err := m.GetConfig(r.Context(), subforge.AlertChannelsKey)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return subforge.AlertChannel{}, false
	}
	var stored []subforge.AlertChannel
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &stored); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return subforge.AlertChannel{}, false
		}
	}
	if req.Index < 0 || req.Index >= len(stored) {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("通道下标越界"))
		return subforge.AlertChannel{}, false
	}
	return stored[req.Index], true
}

// newSchedulerAlertChannel 把一条通道配置构造成 scheduler.AlertChannel。
func newSchedulerAlertChannel(c subforge.AlertChannel) (scheduler.AlertChannel, error) {
	switch c.Kind {
	case "telegram":
		return scheduler.NewTelegramChannel(c.BotToken, c.ChatID), nil
	case "dingtalk":
		return scheduler.NewDingTalkChannel(c.Webhook), nil
	case "smtp":
		return &scheduler.SMTPChannel{
			Host: c.Host, Port: c.Port, Username: c.Username,
			Password: c.Password, From: c.From, To: c.To,
		}, nil
	default:
		return nil, fmt.Errorf("未知的告警通道类型: %s", c.Kind)
	}
}

// alertTestResp 是 POST /api/alert-channels/test 的响应。
type alertTestResp struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Channel string `json:"channel"`
}
