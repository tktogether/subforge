// 订阅管理 HTTP 处理器：/api/airports, /api/groups, /api/rules, /api/tokens
// 以及 /subscribe?token=xxx 生成订阅。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tktogether/subforge/internal/subforge"
)

// SubscriptionManager 返回 Manager 实例。
// 由 Server 在启动时注入。
func (s *Server) SubscriptionManager() *subforge.Manager {
	return s.subManager
}

// --- /api/airports ---

func (s *Server) handleAPIAirports(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	switch r.Method {
	case http.MethodGet:
		airports, err := m.ListAirports(r.Context())
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"airports": airports})
	case http.MethodPost:
		var req struct {
			Name            string `json:"name"`
			SubscriptionURL string `json:"subscription_url"`
			Remarks         string `json:"remarks"`
			Enabled         bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		a, err := m.AddAirport(r.Context(), req.Name, req.SubscriptionURL, req.Remarks, req.Enabled)
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusCreated, a)
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 GET/POST\n")
	}
}

func (s *Server) handleAPIAirport(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", "无效的 ID\n")
		return
	}
	switch r.Method {
	case http.MethodGet:
		a, err := m.GetAirport(r.Context(), id)
		if err != nil {
			s.writeText(w, http.StatusNotFound, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, a)
	case http.MethodPut:
		var req struct {
			Name            string `json:"name"`
			SubscriptionURL string `json:"subscription_url"`
			Remarks         string `json:"remarks"`
			Enabled         bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		a, err := m.UpdateAirport(r.Context(), id, req.Name, req.SubscriptionURL, req.Remarks, req.Enabled)
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, a)
	case http.MethodDelete:
		if err := m.DeleteAirport(r.Context(), id); err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 GET/PUT/DELETE\n")
	}
}

// --- /api/groups ---

func (s *Server) handleAPIGroups(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	switch r.Method {
	case http.MethodGet:
		groups, err := m.ListProxyGroups(r.Context())
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Regex    string `json:"regex"`
			URL      string `json:"url"`
			Interval int    `json:"interval"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		g, err := m.AddProxyGroup(r.Context(), req.Name, req.Type, req.Regex, req.URL, req.Interval)
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusCreated, g)
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 GET/POST\n")
	}
}

func (s *Server) handleAPIGroup(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", "无效的 ID\n")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := m.DeleteProxyGroup(r.Context(), id); err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 DELETE\n")
	}
}

// --- /api/rules ---

func (s *Server) handleAPIRules(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := m.ListRules(r.Context())
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
	case http.MethodPost:
		var req struct {
			RuleType   string `json:"rule_type"`
			RuleParam  string `json:"rule_param"`
			RuleConfig int64  `json:"rule_config"`
			ResolveDNS bool   `json:"resolve_dns"`
			Enabled    bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		rule, err := m.AddRule(r.Context(), req.RuleType, req.RuleParam, req.RuleConfig, req.ResolveDNS, req.Enabled)
		if err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusCreated, rule)
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 GET/POST\n")
	}
}

func (s *Server) handleAPIRule(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", "无效的 ID\n")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := m.DeleteRule(r.Context(), id); err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 DELETE\n")
	}
}

// --- /api/tokens ---

func (s *Server) handleAPITokens(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	switch r.Method {
	case http.MethodGet:
		// List tokens — need to add ListSubTokens to Manager
		s.writeJSON(w, http.StatusOK, map[string]any{"tokens": []any{}})
	case http.MethodPost:
		var req struct {
			Token string `json:"token"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		if err := m.CreateSubToken(r.Context(), req.Token, req.Name); err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"token": req.Token})
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 GET/POST\n")
	}
}

func (s *Server) handleAPIToken(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	token := r.PathValue("token")
	switch r.Method {
	case http.MethodGet:
		t, err := m.GetSubToken(r.Context(), token)
		if err != nil {
			s.writeText(w, http.StatusNotFound, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, t)
	case http.MethodDelete:
		if err := m.DeleteSubToken(r.Context(), token); err != nil {
			s.writeText(w, http.StatusInternalServerError, "text/plain; charset=utf-8", err.Error()+"\n")
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeText(w, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", "仅支持 GET/DELETE\n")
	}
}

// --- /subscribe?token=xxx ---

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	m := s.subManager
	if m == nil {
		s.writeText(w, http.StatusServiceUnavailable, "text/plain; charset=utf-8", "订阅管理未启用\n")
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		s.writeText(w, http.StatusBadRequest, "text/plain; charset=utf-8", "缺少 token 参数\n")
		return
	}
	_, err := m.GetSubToken(r.Context(), token)
	if err != nil {
		s.writeText(w, http.StatusNotFound, "text/plain; charset=utf-8", err.Error()+"\n")
		return
	}
	// TODO: generate subscription from DB (airports → merge → rules)
	s.writeText(w, http.StatusOK, "text/plain; charset=utf-8", "# TODO: 生成订阅\n")
}


