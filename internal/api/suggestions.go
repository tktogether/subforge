package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tktogether/subforge/internal/rulebot"
)

// suggestionQueue 保存 Rule-Bot 待确认建议队列。
// 由后续任务在启动时注入；为空表示队列未启用。
var suggestionQueue *rulebot.PendingQueue

// SetSuggestionQueue 注入建议队列（供启动装配调用）。
func SetSuggestionQueue(q *rulebot.PendingQueue) { suggestionQueue = q }

// handleAPISuggestions 实现 GET /api/suggestions。
func (s *Server) handleAPISuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	out := []*rulebot.Suggestion{}
	if suggestionQueue != nil {
		out = suggestionQueue.Pending()
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

func (s *Server) parseSuggestionIndex(w http.ResponseWriter, r *http.Request) (int, bool) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || idx < 0 {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("建议索引不存在"))
		return 0, false
	}
	return idx, true
}

// handleAPIConfirmSuggestion 实现 POST /api/suggestions/{index}/confirm。
func (s *Server) handleAPIConfirmSuggestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	if suggestionQueue == nil {
		s.writeAPIError(w, r, http.StatusNotImplemented, fmt.Errorf("Rule-Bot 建议队列未启用"))
		return
	}
	idx, ok := s.parseSuggestionIndex(w, r)
	if !ok {
		return
	}
	if err := suggestionQueue.Confirm(idx); err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAPIRejectSuggestion 实现 POST /api/suggestions/{index}/reject。
func (s *Server) handleAPIRejectSuggestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	if suggestionQueue == nil {
		s.writeAPIError(w, r, http.StatusNotImplemented, fmt.Errorf("Rule-Bot 建议队列未启用"))
		return
	}
	idx, ok := s.parseSuggestionIndex(w, r)
	if !ok {
		return
	}
	if err := suggestionQueue.Reject(idx); err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// suggestionLoop 保存 Rule-Bot 闭环（检测器 + 队列）。
// 由启动装配注入；为空表示闭环未启用。
var suggestionLoop *rulebot.Loop

// SetSuggestionLoop 注入建议闭环（供启动装配调用）。
func SetSuggestionLoop(l *rulebot.Loop) { suggestionLoop = l }

// handleAPIApplySuggestions 实现 POST /api/suggestions/apply：把已确认建议真正导入规则库。
// 成功返回 {"applied":N}；ApplyConfirmed 报错时 502，且 Confirmed 不会被清空（可重试）。
func (s *Server) handleAPIApplySuggestions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	mgr, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if suggestionLoop == nil {
		s.writeAPIError(w, r, http.StatusNotImplemented, fmt.Errorf("Rule-Bot 建议队列未启用"))
		return
	}
	n, err := suggestionLoop.ApplyConfirmed(r.Context(), "scheduler", func(ctx context.Context, source string, rules []string) error {
		_, err := mgr.ImportRules(ctx, source, rules, nil)
		return err
	})
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("应用建议失败: %w", err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"applied": n})
}
