package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/subforge"
)

// importHTTPError 给导入失败带上 HTTP 状态语义：
// 上游拉取/解析失败 → 502；写库失败 → 500；前置条件缺失 → 503。
type importHTTPError struct {
	status int
	err    error
}

func (e *importHTTPError) Error() string { return e.err.Error() }
func (e *importHTTPError) Unwrap() error { return e.err }

// importRulesFromAirport 拉取一个机场的订阅并导入规则与规则集。
// warnings = dec.Warnings + ir.Warnings 合并（非 nil 切片）。
func (s *Server) importRulesFromAirport(ctx context.Context, m *subforge.Manager, a *subforge.Airport) (subforge.ImportResult, []string, error) {
	if m == nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusServiceUnavailable, errors.New("订阅管理未启用")}
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusServiceUnavailable, errors.New("转换引擎未启用")}
	}
	res := s.engine.Fetcher.FetchFrom(ctx, fetch.Source{Name: a.Name, URL: a.SubscriptionURL})
	if res.Err != nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusBadGateway, fmt.Errorf("导入失败: %w", res.Err)}
	}
	dec, err := parse.DecodeSource(res.Content)
	if err != nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusBadGateway, fmt.Errorf("导入失败: %w", err)}
	}
	ir, err := m.ImportRules(ctx, "airport:"+a.Name, dec.Rules, dec.RuleProviders)
	if err != nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusInternalServerError, fmt.Errorf("导入失败: %w", err)}
	}
	warnings := make([]string, 0, len(dec.Warnings)+len(ir.Warnings))
	warnings = append(warnings, dec.Warnings...)
	warnings = append(warnings, ir.Warnings...)
	return ir, warnings, nil
}

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
	ir, warnings, err := s.importRulesFromAirport(r.Context(), m, a)
	if err != nil {
		status := http.StatusInternalServerError
		var he *importHTTPError
		if errors.As(err, &he) {
			status = he.status
		}
		s.writeAPIError(w, r, status, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"rules_added":     ir.RulesAdded,
		"rules_skipped":   ir.RulesSkipped,
		"providers_added": ir.ProvidersAdded,
		"warnings":        warnings,
	})
}
