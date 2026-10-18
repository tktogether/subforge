package api

import (
	"fmt"
	"net/http"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/parse"
)

// handleAPIImportRules 实现 POST /api/airports/{id}/import-rules。
func (s *Server) handleAPIImportRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, fmt.Errorf("转换引擎未启用"))
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	a, err := m.GetAirport(r.Context(), id)
	if err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("机场不存在"))
		return
	}
	res := s.engine.Fetcher.FetchFrom(r.Context(), fetch.Source{Name: a.Name, URL: a.SubscriptionURL})
	if res.Err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("导入失败: %w", res.Err))
		return
	}
	dec, err := parse.DecodeSource(res.Content)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("导入失败: %w", err))
		return
	}
	ir, err := m.ImportRules(r.Context(), "airport:"+a.Name, dec.Rules, dec.RuleProviders)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, fmt.Errorf("导入失败: %w", err))
		return
	}
	warnings := make([]string, 0, len(dec.Warnings)+len(ir.Warnings))
	warnings = append(warnings, dec.Warnings...)
	warnings = append(warnings, ir.Warnings...)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"rules_added":     ir.RulesAdded,
		"rules_skipped":   ir.RulesSkipped,
		"providers_added": ir.ProvidersAdded,
		"warnings":        warnings,
	})
}
