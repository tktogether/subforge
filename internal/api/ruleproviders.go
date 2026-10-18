package api

import (
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
