package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tktogether/subforge/internal/store"
)

// handleAPIRuleProviders 实现 GET /api/rule-providers（可选 ?source= / ?enabled=1|0）。
func (s *Server) handleAPIRuleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	providers, err := m.DB().ListRuleProviders(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	source := r.URL.Query().Get("source")
	enabled := r.URL.Query().Get("enabled")
	out := make([]store.RuleProvider, 0)
	for _, p := range providers {
		if source != "" && p.Source != source {
			continue
		}
		if enabled == "1" && !p.Enabled {
			continue
		}
		if enabled == "0" && p.Enabled {
			continue
		}
		out = append(out, p)
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"rule_providers": out})
}

// handleAPIRuleProviderSetEnabled 处理 PUT /api/rule-providers/{id}/enabled，body {"enabled": true|false}。
// 路径参数复用 s.pathID（subscription.go:50-58，非法 id 写 400）；目标行不存在写 404。
func (s *Server) handleAPIRuleProviderSetEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 PUT"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("body 必须是 {\"enabled\": true|false}"))
		return
	}
	found, err := m.DB().SetRuleProviderEnabled(r.Context(), id, *body.Enabled)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if !found {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则集 %d 不存在", id))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAPIRuleSourceCounts 处理 GET /api/rules/sources：
// 轻量返回规则来源及条数，供规则页来源筛选，避免为了下拉选项拉全量规则。
func (s *Server) handleAPIRuleSourceCounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	sources, err := m.ListRuleSourceCounts(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if sources == nil {
		sources = []store.RuleSourceCount{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}
