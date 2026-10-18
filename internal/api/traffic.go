package api

import (
	"encoding/json"
	"errors"
	"net/http"

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

// handleAPITrafficRefresh 立即刷新所有启用机场的流量余额。
//
// POST → {"results":[...], "ok":N, "fail":M}
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
	s.writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"ok":      okCount,
		"fail":    len(results) - okCount,
	})
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
