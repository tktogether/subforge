package api

import (
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
