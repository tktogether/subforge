package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/subforge"
)

type bulkRulesBody struct {
	IDs     []int64 `json:"ids"`
	Source  string  `json:"source"`
	Enabled bool    `json:"enabled"`
	Delete  bool    `json:"delete"`
}

func (s *Server) handleAPIRulesBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var body bulkRulesBody
	if !s.decodeBody(w, r, &body) {
		return
	}
	hasIDs := len(body.IDs) > 0
	hasSource := strings.TrimSpace(body.Source) != ""

	// 批量删除只接受显式列出的 ids：source 的语义是「按来源整批」，放开等于给
	// 全表一个一键清空入口。删除与启用/禁用互斥，走各自独立的响应体。
	if body.Delete {
		if hasSource {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("批量删除仅支持 ids"))
			return
		}
		if !hasIDs {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("批量删除必须给出 ids"))
			return
		}
		deleted, err := m.DB().BatchDeleteRules(r.Context(), body.IDs)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "updated": 0})
		return
	}

	if !hasIDs && !hasSource {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("必须给出 ids 或 source"))
		return
	}
	if hasIDs && hasSource {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("ids 与 source 不能同时给出"))
		return
	}

	var targets []subforge.Rule
	if hasIDs {
		for _, id := range body.IDs {
			rule, err := m.GetRule(r.Context(), id)
			if err != nil {
				continue
			}
			targets = append(targets, *rule)
		}
	} else {
		all, err := m.ListRules(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		for _, rule := range all {
			if rule.Source == body.Source {
				targets = append(targets, rule)
			}
		}
	}

	updated := 0
	for _, rule := range targets {
		if err := m.UpdateRule(r.Context(), rule.ID, rule.RuleType, rule.RuleParam, rule.RuleConfig, rule.ResolveDNS, body.Enabled); err != nil {
			continue
		}
		updated++
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"updated": updated})
}
