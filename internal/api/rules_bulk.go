package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/store"
)

// bulkRulesBody 是 /api/rules/bulk 的请求体。
//
// v2 起批量操作按「来源」整批执行（机场导入的上万条规则终于能一次停用/删除），
// 也保留按 ids 精确操作。Source 用 *string 是为了区分「没给 source」与
// 「source 是空串」——后者表示自定义层，是合法筛选条件。
type bulkRulesBody struct {
	IDs          []int64 `json:"ids"`
	Source       *string `json:"source"`
	SourcePrefix bool    `json:"source_prefix"`
	Enabled      bool    `json:"enabled"`
	Delete       bool    `json:"delete"`
	// ProfileID 非 nil = 旧客户端的「移动到方案」参数，v2 已下线。
	ProfileID *int64 `json:"profile_id"`
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
	if body.ProfileID != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("规则不再归属订阅方案，请用 source 区分来源"))
		return
	}
	hasIDs := len(body.IDs) > 0
	hasSource := body.Source != nil
	if hasIDs && hasSource {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("ids 与 source 不能同时给出"))
		return
	}
	if !hasIDs && !hasSource {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("必须给出 ids 或 source"))
		return
	}
	source := ""
	if hasSource {
		source = strings.TrimSpace(*body.Source)
		if body.SourcePrefix && source == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("source_prefix 需要非空 source"))
			return
		}
	}
	filter := store.SourceFilter{Source: source, Prefix: body.SourcePrefix}

	// 删除：按来源整批删或按 ids 删，两者都走一条 SQL（来源批量是 v1 缺的能力：
	// 导入上万家规则后想「把这个机场的全删掉」只能一条条点）。delete 与启停互斥。
	if body.Delete {
		var deleted int64
		var err error
		if hasIDs {
			n, derr := m.DB().BatchDeleteRules(r.Context(), body.IDs)
			deleted, err = int64(n), derr
		} else {
			deleted, err = m.DB().BatchDeleteRulesBySource(r.Context(), filter)
		}
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "updated": 0})
		return
	}

	// 启停：按来源走一条 UPDATE（原子），按 ids 逐条更新（保持 v1 行为，非原子可接受）。
	if hasSource {
		updated, err := m.DB().BatchSetRulesEnabledBySource(r.Context(), filter, body.Enabled)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "deleted": 0})
		return
	}
	updated := 0
	for _, id := range body.IDs {
		rule, err := m.GetRule(r.Context(), id)
		if err != nil {
			continue
		}
		if err := m.UpdateRule(r.Context(), rule.ID, rule.RuleType, rule.RuleParam, rule.RuleConfig, rule.ResolveDNS, body.Enabled); err != nil {
			continue
		}
		updated++
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "deleted": 0})
}
