package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// importRulesFromAirport 拉取一个机场的订阅并导入规则与规则集（导入到未分配池）。
// warnings = dec.Warnings + ir.Warnings 合并（非 nil 切片）。
func (s *Server) importRulesFromAirport(ctx context.Context, m *subforge.Manager, a *subforge.Airport) (subforge.ImportResult, []string, error) {
	return s.importRulesToProfile(ctx, m, a, 0, false)
}

// importRulesToProfile 是 importRulesFromAirport 的方案版：profileID 归属哪个方案，
// createGroups=true 时为目标方案自动补建缺失的目标分组。
func (s *Server) importRulesToProfile(ctx context.Context, m *subforge.Manager, a *subforge.Airport, profileID int64, createGroups bool) (subforge.ImportResult, []string, error) {
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
	ir, err := m.ImportRulesToProfile(ctx, profileID, "airport:"+a.Name, dec.Rules, dec.RuleProviders, createGroups)
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
	// body 可选：空 body / EOF → 未分配（profile_id=0）且不自动建组，兼容旧调用。
	var body struct {
		ProfileID    int64 `json:"profile_id"`
		CreateGroups bool  `json:"create_groups"`
	}
	if r.Body != nil {
		if derr := json.NewDecoder(r.Body).Decode(&body); derr != nil && !errors.Is(derr, io.EOF) {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("请求参数格式不正确: %w", derr))
			return
		}
	}
	if !s.profileExists(w, r, m, body.ProfileID) {
		return
	}
	ir, warnings, err := s.importRulesToProfile(r.Context(), m, a, body.ProfileID, body.CreateGroups)
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
		"groups_created":  ir.GroupsCreated,
		"warnings":        warnings,
	})
}
