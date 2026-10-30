// 订阅方案（Profile）的 HTTP 层：/api/profiles 的 CRUD、层顺序校验，
// 以及令牌绑方案时共用的校验与回填助手。
//
// v2 契约：.tmp/ux-e2e/profiles2/CONTRACT-V2.md §4.1。
// 方案不再拥有规则/分组（那是全局库），只声明「启用哪些层、按什么优先级」。
package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/subforge"
)

// errProfileNameEmpty 是方案名为空（去空格后）时的错误（契约 §4.1）。
var errProfileNameEmpty = errors.New("方案名称不能为空")

// profileBody 是 POST/PUT /api/profiles 的请求体。
//
// enabled 用 *bool：POST 缺省为 true（新建即启用），PUT 缺省保持原值（契约 §4.1）。
// 两个层字段用 *[]string：nil=没给（保持/默认），空数组=恢复默认三层。
type profileBody struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Config      string    `json:"config"`
	Pipeline    string    `json:"pipeline"`
	Enabled     *bool     `json:"enabled"`
	SortOrder   int       `json:"sort_order"`
	RuleLayers  *[]string `json:"rule_layers"`
	GroupLayers *[]string `json:"group_layers"`
	RuleSources *[]string `json:"rule_sources"`
	// 节点来源：nil = 不限制（全部启用来源）；空数组 = 明确不使用该类来源。
	AirportIDs  *[]int64 `json:"airport_ids"`
	SelfNodeIDs *[]int64 `json:"self_node_ids"`
	// AllSources=true 时清空两个限制（恢复“全部启用来源”）；PUT 里省略=保持原值。
	AllSources bool `json:"all_sources"`
}

// 合法层键。规则层与分组层共用一套键。
var knownLayers = []string{"custom", "airport", "template"}

// normalizeLayers 校验一组层键并做规则层的 template 归一化。
//
//   - 空数组/空串：等价于默认值，返回 nil（由调用方/存储层补默认三层）；
//   - 未知键 → 错误「未知的层 X」；重复键 → 错误「重复的层 X」；
//   - lockTemplateLast=true（规则层）：template 若不在末位自动移到末位，不报错——
//     模板里的 MATCH/FINAL 必须最后，否则模板规则之后的规则全成死规则。
func normalizeLayers(layers []string, lockTemplateLast bool) ([]string, error) {
	if len(layers) == 0 {
		return nil, nil
	}
	seen := make(map[string]bool, len(layers))
	out := make([]string, 0, len(layers))
	for _, raw := range layers {
		layer := strings.TrimSpace(raw)
		if layer == "" {
			continue
		}
		if !isKnownLayer(layer) {
			return nil, fmt.Errorf("未知的层 %s", layer)
		}
		if seen[layer] {
			return nil, fmt.Errorf("重复的层 %s", layer)
		}
		seen[layer] = true
		out = append(out, layer)
	}
	if len(out) == 0 {
		return nil, nil
	}
	if lockTemplateLast && out[len(out)-1] != "template" && seen["template"] {
		moved := make([]string, 0, len(out))
		for _, layer := range out {
			if layer != "template" {
				moved = append(moved, layer)
			}
		}
		moved = append(moved, "template")
		out = moved
	}
	return out, nil
}

// isKnownLayer 报告层键是否合法。
func isKnownLayer(layer string) bool {
	for _, known := range knownLayers {
		if known == layer {
			return true
		}
	}
	return false
}

// validateProfileLayers 按契约校验请求里的两个层数组（只校验给了的）。
// 返回 (ruleLayers, groupLayers, ok)：nil 表示「未提供」；ok=false 时已写出 400。
func (s *Server) validateProfileLayers(w http.ResponseWriter, r *http.Request, body profileBody) ([]string, []string, bool) {
	var ruleLayers, groupLayers []string
	if body.RuleLayers != nil {
		normalized, err := normalizeLayers(*body.RuleLayers, true)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("规则层顺序非法：%w", err))
			return nil, nil, false
		}
		ruleLayers = normalized
	}
	if body.GroupLayers != nil {
		normalized, err := normalizeLayers(*body.GroupLayers, false)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("分组层顺序非法：%w", err))
			return nil, nil, false
		}
		groupLayers = normalized
	}
	return ruleLayers, groupLayers, true
}

// maxRuleSources 是单个方案可绑定的规则源数量上限（契约 §1.2）。
const maxRuleSources = 20

// normalizeRuleSources 归一化方案绑定的规则源名：trim、丢空、去重保序、最多
// maxRuleSources 个。不校验「源是否存在」：manifest 是文件、随时可变，
// 存在性在订阅组装时以 warning 暴露（RuleSourceRules）。
func normalizeRuleSources(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) > maxRuleSources {
		out = out[:maxRuleSources]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeIDListPtr 去重并丢掉非法 id；nil 保持 nil（未限制），空数组保持空数组（不使用）。
func normalizeIDListPtr(ids *[]int64) *[]int64 {
	if ids == nil {
		return nil
	}
	seen := make(map[int64]bool, len(*ids))
	out := make([]int64, 0, len(*ids))
	for _, id := range *ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return &out
}

// validateProfileSourceIDs 校验方案选择的来源 id 存在，避免保存后订阅静默为空。
func (s *Server) validateProfileSourceIDs(ctx context.Context, m *subforge.Manager, airportIDs, selfNodeIDs *[]int64) error {
	if airportIDs == nil && selfNodeIDs == nil {
		return nil
	}
	if airportIDs != nil {
		airports, err := m.ListAirports(ctx)
		if err != nil {
			return err
		}
		known := make(map[int64]bool, len(airports))
		for _, a := range airports {
			known[a.ID] = true
		}
		for _, id := range *airportIDs {
			if !known[id] {
				return fmt.Errorf("机场 %d 不存在", id)
			}
		}
	}
	if selfNodeIDs != nil {
		nodes, err := m.ListSelfNodes(ctx)
		if err != nil {
			return err
		}
		known := make(map[int64]bool, len(nodes))
		for _, n := range nodes {
			known[n.ID] = true
		}
		for _, id := range *selfNodeIDs {
			if !known[id] {
				return fmt.Errorf("自建节点 %d 不存在", id)
			}
		}
	}
	return nil
}

// normalizeProfileSourcesForJSON 保证响应里的 rule_sources 一定是数组：
// 未绑定（nil）时输出 []，前端多选控件才能按数组统一处理，而不是 null。
func normalizeProfileSourcesForJSON(p *subforge.Profile) {
	if p == nil {
		return
	}
	if p.RuleSources == nil {
		p.RuleSources = []string{}
	}
}

// profileNameByID 读出全部方案的 id→名称映射，用于回填令牌的 profile_name。
func profileNameByID(ctx context.Context, m *subforge.Manager) (map[int64]string, error) {
	profiles, err := m.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(profiles))
	for _, p := range profiles {
		names[p.ID] = p.Name
	}
	return names, nil
}

// isUniqueConstraintError 判断错误是否来自 SQLite 唯一索引冲突。
// 用于把「方案名唯一冲突」翻译成 409。
func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE")
}

// profileExists 校验 id 绑定的方案是否存在。id <= 0 表示「未分配」，直接放行；
// id > 0 且不存在时写出 400「订阅方案 N 不存在」并返回 false。
func (s *Server) profileExists(w http.ResponseWriter, r *http.Request, m *subforge.Manager, id int64) bool {
	if id <= 0 {
		return true
	}
	ok, err := m.DB().ProfileExists(r.Context(), id)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return false
	}
	if !ok {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("订阅方案 %d 不存在", id))
		return false
	}
	return true
}

// fillTokenProfileName 回填单个令牌的 profile_name（未绑定方案时为空串，由 omitempty 抑制）。
func (s *Server) fillTokenProfileName(ctx context.Context, m *subforge.Manager, t *subforge.SubToken) error {
	names, err := profileNameByID(ctx, m)
	if err != nil {
		return err
	}
	t.ProfileName = names[t.ProfileID]
	return nil
}

// --- /api/profiles ---

// handleAPIProfiles 实现 /api/profiles 的 GET（列表）与 POST（创建）。
func (s *Server) handleAPIProfiles(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		profiles, err := m.ListProfiles(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if profiles == nil {
			profiles = []subforge.Profile{}
		}
		for i := range profiles {
			normalizeProfileSourcesForJSON(&profiles[i])
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
	case http.MethodPost:
		var body profileBody
		if !s.decodeBody(w, r, &body) {
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errProfileNameEmpty)
			return
		}
		ruleLayers, groupLayers, ok := s.validateProfileLayers(w, r, body)
		if !ok {
			return
		}
		// enabled 缺省 = true（新建即启用）；显式给 false 才停用。
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		// 规则源与层一样是「给了才写」：nil=不绑定，空数组=绑定空列表。
		ruleSourcesProvided := body.RuleSources != nil
		var normalizedRuleSources []string
		if ruleSourcesProvided {
			normalizedRuleSources = normalizeRuleSources(*body.RuleSources)
		}
		airportIDs := normalizeIDListPtr(body.AirportIDs)
		selfNodeIDs := normalizeIDListPtr(body.SelfNodeIDs)
		if body.AllSources {
			var unlimited []int64
			airportIDs, selfNodeIDs = &unlimited, &unlimited
		}
		if err := s.validateProfileSourceIDs(r.Context(), m, airportIDs, selfNodeIDs); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		p, err := m.AddProfile(r.Context(), name, body.Description, body.Config, body.Pipeline, enabled, body.SortOrder)
		if err != nil {
			if isUniqueConstraintError(err) {
				s.writeAPIError(w, r, http.StatusConflict, fmt.Errorf("订阅方案 “%s” 已存在", name))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		// 层配置单独写：AddProfile 只负责基本列，避免把「层」塞进多个函数签名。
		if ruleLayers != nil || groupLayers != nil {
			if err := m.SetProfileLayers(r.Context(), p.ID, ruleLayers, groupLayers); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", p.ID))
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		if ruleSourcesProvided {
			if err := m.SetProfileRuleSources(r.Context(), p.ID, normalizedRuleSources); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", p.ID))
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		if airportIDs != nil || selfNodeIDs != nil {
			if err := m.SetProfileSources(r.Context(), p.ID, airportIDs, selfNodeIDs); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", p.ID))
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		if ruleLayers != nil || groupLayers != nil || ruleSourcesProvided || airportIDs != nil || selfNodeIDs != nil {
			if fresh, err := m.GetProfile(r.Context(), p.ID); err == nil {
				p = fresh
			}
		}
		normalizeProfileSourcesForJSON(p)
		s.writeJSON(w, http.StatusCreated, map[string]any{"profile": p})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

// handleAPIProfile 实现 /api/profiles/{id} 的 GET/PUT/DELETE。
func (s *Server) handleAPIProfile(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		p, err := m.GetProfile(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		normalizeProfileSourcesForJSON(p)
		s.writeJSON(w, http.StatusOK, map[string]any{"profile": p})
	case http.MethodPut:
		var body profileBody
		if !s.decodeBody(w, r, &body) {
			return
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errProfileNameEmpty)
			return
		}
		ruleLayers, groupLayers, ok := s.validateProfileLayers(w, r, body)
		if !ok {
			return
		}
		// enabled 缺省 = 保持原值（契约 §4.1）：先读当前行，用它当兜底。
		enabled := true
		if cur, err := m.GetProfile(r.Context(), id); err == nil {
			enabled = cur.Enabled
		}
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		// 规则源与层一样是「给了才写」：nil=保持原绑定，空数组=清空绑定。
		ruleSourcesProvided := body.RuleSources != nil
		var normalizedRuleSources []string
		if ruleSourcesProvided {
			normalizedRuleSources = normalizeRuleSources(*body.RuleSources)
		}
		airportIDs := normalizeIDListPtr(body.AirportIDs)
		selfNodeIDs := normalizeIDListPtr(body.SelfNodeIDs)
		if body.AllSources {
			var unlimited []int64
			airportIDs, selfNodeIDs = &unlimited, &unlimited
		}
		if err := s.validateProfileSourceIDs(r.Context(), m, airportIDs, selfNodeIDs); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		err := m.UpdateProfile(r.Context(), id, name, body.Description, body.Config, body.Pipeline, enabled, body.SortOrder)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", id))
				return
			}
			if isUniqueConstraintError(err) {
				s.writeAPIError(w, r, http.StatusConflict, fmt.Errorf("订阅方案 “%s” 已存在", name))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if ruleLayers != nil || groupLayers != nil {
			if err := m.SetProfileLayers(r.Context(), id, ruleLayers, groupLayers); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", id))
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		// 只带 rule_sources 的请求不能顺手清掉层：层有自己的 nil 语义，没给就保持原值。
		if ruleSourcesProvided {
			if err := m.SetProfileRuleSources(r.Context(), id, normalizedRuleSources); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", id))
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		if airportIDs != nil || selfNodeIDs != nil {
			if err := m.SetProfileSources(r.Context(), id, airportIDs, selfNodeIDs); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", id))
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		p, err := m.GetProfile(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		normalizeProfileSourcesForJSON(p)
		s.writeJSON(w, http.StatusOK, map[string]any{"profile": p})
	case http.MethodDelete:
		// 删除前只拦「还有令牌绑着」：v2 的规则/分组是全局库，方案里没有内容可清，
		// 直接删方案不会丢数据；但令牌绑着被删的方案会让订阅静默失效，必须拦。
		p, err := m.GetProfile(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		tokens, terr := m.ListSubTokens(r.Context())
		if terr != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, terr)
			return
		}
		used := 0
		for _, t := range tokens {
			if t.ProfileID == id {
				used++
			}
		}
		if used > 0 {
			s.writeAPIError(w, r, http.StatusConflict, fmt.Errorf(
				"订阅方案 “%s” 仍有 %d 个令牌在用，请先改绑或删除令牌", p.Name, used))
			return
		}
		if err := m.DeleteProfile(r.Context(), id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", id))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}
