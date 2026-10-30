package api

import (
	"net/http"
	"time"
)

// overviewCounts 是概览页读取的持久化资源数量，不随服务重启清零。
type overviewCounts struct {
	Airports         int `json:"airports"`
	AirportsEnabled  int `json:"airports_enabled"`
	AirportNodes     int `json:"airport_nodes"`
	SelfNodes        int `json:"self_nodes"`
	SelfNodesEnabled int `json:"self_nodes_enabled"`
	Profiles         int `json:"profiles"`
	ProfilesEnabled  int `json:"profiles_enabled"`
	Tokens           int `json:"tokens"`
	Groups           int `json:"groups"`
	Rules            int `json:"rules"`
	RulesEnabled     int `json:"rules_enabled"`
	RuleSources      int `json:"rule_sources"`
	RuleSourcesReady int `json:"rule_sources_ready"`
	RuleSourceRules  int `json:"rule_source_rules"`
}

// handleAPIOverview 返回首页需要的资源快照与服务信息。
// 转换历史仍走 /api/stats 与 /api/history，这两份数据在内存中，重启后会清空。
func (s *Server) handleAPIOverview(w http.ResponseWriter, r *http.Request) {
	counts := overviewCounts{}
	if m := s.subManager; m != nil {
		airports, err := m.ListAirports(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		for _, a := range airports {
			counts.Airports++
			counts.AirportNodes += a.NodeCount
			if a.Enabled {
				counts.AirportsEnabled++
			}
		}
		selfNodes, err := m.ListSelfNodes(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		counts.SelfNodes = len(selfNodes)
		for _, n := range selfNodes {
			if n.Enabled {
				counts.SelfNodesEnabled++
			}
		}
		profiles, err := m.ListProfiles(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		counts.Profiles = len(profiles)
		for _, p := range profiles {
			if p.Enabled {
				counts.ProfilesEnabled++
			}
		}
		tokens, err := m.ListSubTokens(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		counts.Tokens = len(tokens)
		groups, err := m.ListProxyGroups(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		counts.Groups = len(groups)
		rules, err := m.ListRules(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		counts.Rules = len(rules)
		for _, rule := range rules {
			if rule.Enabled {
				counts.RulesEnabled++
			}
		}
		sources, err := m.ListRuleSources(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		counts.RuleSources = len(sources)
		for _, src := range sources {
			if src.Latest != "" {
				counts.RuleSourcesReady++
				counts.RuleSourceRules += src.RuleCount
			}
		}
	}

	version := ""
	if s.engine != nil {
		version = s.engine.Profile.Version
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":      s.cfg.Name,
		"version":   version,
		"startedAt": s.startedAt.Format(time.RFC3339),
		"uptime":    time.Since(s.startedAt).String(),
		"counts":    counts,
	})
}
