package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/traffic"
)

// handleAPITrafficStrategy 读写流量聚合策略。
//
// GET → {"strategy":{...}}（未配置时返回默认预设 sum = sum/sum/sum/min）
// PUT → 请求体是 traffic.Strategy，写回后回显**归一化**后的结果
func (s *Server) handleAPITrafficStrategy(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		strat, err := m.GetTrafficStrategy(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"strategy": strat})
	case http.MethodPut:
		var strat traffic.Strategy
		if !s.decodeBody(w, r, &strat) {
			return
		}
		if err := m.SetTrafficStrategy(r.Context(), strat); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		got, err := m.GetTrafficStrategy(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"strategy": got})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT"))
	}
}

// isTruthyQuery 判断 query 参数是否为真值：1/true/yes/on（大小写不敏感、忽略首尾空白）。
func isTruthyQuery(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// importRulesAfterRefresh 对所有刷新成功的机场导入规则与规则集；单个机场失败只记 err，不影响整体。
//
// 与手动导入保持同一语义：createGroups=true，缺失的目标分组会被补建。否则自动导入
// 的规则目标找不到分组，用户只能拿到一组指向 DIRECT/兜底的规则。
func (s *Server) importRulesAfterRefresh(ctx context.Context, m *subforge.Manager, results []subforge.TrafficRefreshResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, res := range results {
		if !res.OK {
			continue
		}
		item := map[string]any{
			"airport_id":      res.AirportID,
			"name":            res.Name,
			"rules_added":     0,
			"rules_skipped":   0,
			"providers_added": 0,
			"groups_created":  0,
			"warnings":        []string{},
			"err":             "",
		}
		if m == nil {
			item["err"] = "订阅管理未启用"
			out = append(out, item)
			continue
		}
		if s.engine == nil || s.engine.Fetcher == nil {
			item["err"] = "转换引擎未启用"
			out = append(out, item)
			continue
		}
		a, err := m.GetAirport(ctx, res.AirportID)
		if err != nil {
			item["err"] = err.Error()
			out = append(out, item)
			continue
		}
		ir, warnings, err := s.importRulesToLibrary(ctx, m, a, subforge.ImportOptions{
			CreateGroups: true,
			EnableRules:  true,
		})
		if err != nil {
			item["err"] = err.Error()
			out = append(out, item)
			continue
		}
		item["rules_added"] = 0
		item["rules_skipped"] = 0
		item["rules_enabled"] = 0
		item["providers_added"] = ir.ProvidersAdded
		item["groups_created"] = ir.GroupsCreated
		item["rule_source_name"] = ir.RuleSourceName
		item["rule_source_version"] = ir.RuleSourceVersion
		item["rule_source_rules"] = ir.RuleSourceRules
		item["warnings"] = warnings
		out = append(out, item)
	}
	return out
}

// handleAPITrafficRefresh 立即刷新所有启用机场的流量余额。
//
// POST → {"results":[...], "ok":N, "fail":M}；可选 ?with_rules=1 时额外带 "imports" 数组。
func (s *Server) handleAPITrafficRefresh(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errors.New("转换引擎未启用"))
		return
	}
	withRules := isTruthyQuery(r.URL.Query().Get("with_rules"))
	results, err := m.RefreshAirportTraffic(r.Context(), s.engine.Fetcher)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if results == nil {
		results = []subforge.TrafficRefreshResult{}
	}
	okCount := 0
	for _, res := range results {
		if res.OK {
			okCount++
		}
	}
	resp := map[string]any{
		"results": results,
		"ok":      okCount,
		"fail":    len(results) - okCount,
	}
	if withRules {
		resp["imports"] = s.importRulesAfterRefresh(r.Context(), m, results)
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleAPIAirportRefreshOne 刷新单个机场流量：POST /api/airports/{id}/refresh
// → 200 {"result":{...}}；机场不存在 → 404；未启用引擎 → 503；非 POST → 405「仅支持 POST」
func (s *Server) handleAPIAirportRefreshOne(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errors.New("转换引擎未启用"))
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	res, err := m.RefreshAirportTrafficByID(r.Context(), s.engine.Fetcher, id)
	if err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"result": res})
}

// handleAPITrafficPreview 试算聚合：GET /api/traffic/preview?strategy=<json>
// → 200 {"usage":{...},"header":"...","ok":bool}
func (s *Server) handleAPITrafficPreview(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET"))
		return
	}
	raw := r.URL.Query().Get("strategy")
	var strat traffic.Strategy
	if raw == "" {
		got, err := m.GetTrafficStrategy(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		strat = got
	} else {
		if err := json.Unmarshal([]byte(raw), &strat); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("strategy 参数不是合法 JSON"))
			return
		}
		strat = strat.Normalize()
	}
	u, h, ok, err := m.PreviewTraffic(r.Context(), strat)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"usage": u, "header": h, "ok": ok})
}
