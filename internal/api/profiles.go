// 订阅方案（Profile）的 HTTP 层：/api/profiles 的 CRUD，
// 以及规则/分组/令牌按方案隔离时共用的校验与回填助手。
//
// 状态码与错误文案严格照 .tmp/ux-e2e/profiles/CONTRACT.md §2.1/§2.2/§2.3。
package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/tktogether/subforge/internal/subforge"
)

// errProfileNameEmpty 是方案名为空（去空格后）时的错误（契约 §2.1）。
var errProfileNameEmpty = errors.New("方案名称不能为空")

// errProfileIDInvalid 是 profile_id 过滤参数非法时的错误（契约 §2.2）。
var errProfileIDInvalid = errors.New("profile_id 必须是非负整数")

// profileBody 是 POST/PUT /api/profiles 的请求体。
//
// enabled 用 *bool：POST 缺省为 true（新建即启用），PUT 缺省保持原值（契约 §2.1）。
type profileBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Config      string `json:"config"`
	Pipeline    string `json:"pipeline"`
	Enabled     *bool  `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

// profileNameByID 读出全部方案的 id→名称映射，用于回填 profile_name。
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

// isDuplicateRuleMoveError 判断错误是否来自「移动规则撞上同源同去重键」的唯一冲突。
// store 把这种冲突替换为固定文案（不再含 "UNIQUE"），这里按文案判 409。
func isDuplicateRuleMoveError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "目标方案里已存在")
}

// profileExists 校验 id 绑定的方案是否存在。id <= 0 表示「未分配」，直接放行；
// id > 0 且不存在时写出 400「订阅方案 N 不存在」并返回 false。
//
// 仅用于创建/移动前的事前校验：GET/PUT/DELETE 复用 m.GetProfile 自己的 404 文案。
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

// parseProfileIDFilter 解析 query 上的可选 profile_id 过滤参数。
// 返回 (profileID, present, ok)：present=false 表示「无该参数（返回全部）」；
// ok=false 表示参数非法，已写出 400。
func (s *Server) parseProfileIDFilter(w http.ResponseWriter, r *http.Request) (int64, bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("profile_id"))
	if raw == "" {
		return 0, false, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, errProfileIDInvalid)
		return 0, false, false
	}
	return id, true, true
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
		// enabled 缺省 = true（新建即启用）；显式给 false 才停用。
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		p, err := m.AddProfile(r.Context(), name, body.Description, body.Config, body.Pipeline, enabled, body.SortOrder)
		if err != nil {
			if isUniqueConstraintError(err) {
				s.writeAPIError(w, r, http.StatusConflict, fmt.Errorf("方案名称 “%s” 已存在", name))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
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
		// enabled 缺省 = 保持原值（契约 §2.1）：先读当前行，用它当兜底。
		enabled := true
		if cur, err := m.GetProfile(r.Context(), id); err == nil {
			enabled = cur.Enabled
		}
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		err := m.UpdateProfile(r.Context(), id, name, body.Description, body.Config, body.Pipeline, enabled, body.SortOrder)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("订阅方案 %d 不存在", id))
				return
			}
			if isUniqueConstraintError(err) {
				s.writeAPIError(w, r, http.StatusConflict, fmt.Errorf("方案名称 “%s” 已存在", name))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		p, err := m.GetProfile(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"profile": p})
	case http.MethodDelete:
		// 先取，既当「不存在→404」判断，也拿 rule_count/group_count 做删除前校验。
		p, err := m.GetProfile(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		if p.RuleCount > 0 || p.GroupCount > 0 {
			s.writeAPIError(w, r, http.StatusConflict, fmt.Errorf(
				"方案 “%s” 下还有 %d 条规则、%d 个分组，请先移动或删除", p.Name, p.RuleCount, p.GroupCount))
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

// bulkGroupsBody 是 POST /api/groups/bulk 的请求体：把 ids 里的分组移到 profile_id。
type bulkGroupsBody struct {
	IDs       []int64 `json:"ids"`
	ProfileID int64   `json:"profile_id"`
}

// handleAPIGroupsBulk 实现 POST /api/groups/bulk（批量移动分组的方案归属）。
func (s *Server) handleAPIGroupsBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var body bulkGroupsBody
	if !s.decodeBody(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("必须给出 ids"))
		return
	}
	if !s.profileExists(w, r, m, body.ProfileID) {
		return
	}
	moved, err := m.MoveGroupsToProfile(r.Context(), body.IDs, body.ProfileID)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"updated": moved})
}
