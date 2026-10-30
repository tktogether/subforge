// 订阅管理 HTTP 处理器：/api/airports, /api/groups, /api/rules, /api/tokens
// 以及 /subscribe?token=xxx 生成订阅。
package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/store"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/traffic"
)

// SubscriptionManager 返回 Manager 实例。
// 由 Server 在启动时注入。
func (s *Server) SubscriptionManager() *subforge.Manager {
	return s.subManager
}

// writeAPIError 是订阅管理接口的统一错误出口：JSON {"error":"..."}。
//
// 前端 api() 助手对每个响应都调 res.json()，因此失败响应绝不能是纯文本——
// 否则前端拿不到错误原因，只会看到 JSON 解析异常。
//
// 文案治理集中在 writeError（D-22）：JSON 解码错误翻译成中文可读文案并把原始
// 报文放进 detail，500 统一成「服务端内部错误（请求 ID …）」。
func (s *Server) writeAPIError(w http.ResponseWriter, r *http.Request, status int, err error) {
	s.writeError(w, r, status, err)
}

// subManagerOr503 取订阅管理器；未启用时写出 503 JSON 并返回 ok=false。
func (s *Server) subManagerOr503(w http.ResponseWriter, r *http.Request) (*subforge.Manager, bool) {
	if s.subManager == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errors.New("订阅管理未启用"))
		return nil, false
	}
	return s.subManager, true
}

// pathID 解析 {id} 路径参数；非法时写出 400 JSON 并返回 ok=false。
func (s *Server) pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("无效的 ID"))
		return 0, false
	}
	return id, true
}

// decodeBody 解析 JSON 请求体；失败时写出 400 JSON 并返回 false。
// 解码错误（字段类型不对、不是合法 JSON）由 writeError 翻译成中文可读文案，
// 原始 Go 报文只放在 detail 里给排障用。
func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return false
	}
	return true
}

// --- 输入校验（D-05 / D-07 / D-08 / D-10 / D-11）---
//
// 这些校验只负责"能不能入库"，不负责业务语义：名字/链接非空、URL 必须是 http(s)、
// 分组类型与正则合法、自建节点链接能被解析引擎逐节点解析。校验失败一律 400 JSON，
// 由 s.writeAPIError 统一出口，前端 api() 助手才能拿到错误原因。

// validateAirport 校验机场输入：名称非空，订阅地址必须是 http(s) URL。
func validateAirport(name, subscriptionURL string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("名称不能为空")
	}
	return validateSubscriptionURL(subscriptionURL)
}

// validateSubscriptionURL 校验订阅地址：非空且 scheme 必须是 http/https 且带 host。
func validateSubscriptionURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("订阅地址不能为空")
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("订阅地址必须以 http:// 或 https:// 开头")
	}
	return nil
}

// validGroupTypes 是分组 type 的合法集合（入库统一存小写）。
var validGroupTypes = map[string]bool{
	"select":       true,
	"url-test":     true,
	"fallback":     true,
	"load-balance": true,
}

// validateProxyGroup 校验分组输入，返回规范化后的 type（空 -> "select"，统一小写）。
func validateProxyGroup(name, typ, regex string, interval int) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("名称不能为空")
	}
	t := strings.ToLower(strings.TrimSpace(typ))
	if t == "" {
		t = "select"
	}
	if !validGroupTypes[t] {
		return "", errors.New("分组类型必须是 select/url-test/fallback/load-balance 之一")
	}
	if r := strings.TrimSpace(regex); r != "" {
		if _, err := regexp.Compile(r); err != nil {
			return "", fmt.Errorf("匹配正则不合法: %v", err)
		}
	}
	if interval < 0 {
		return "", errors.New("测活间隔不能为负数")
	}
	return t, nil
}

// validateRule 校验规则输入：类型与参数都非空。
func validateRule(ruleType, ruleParam string) error {
	if strings.TrimSpace(ruleType) == "" {
		return errors.New("类型不能为空")
	}
	if strings.TrimSpace(ruleParam) == "" {
		return errors.New("参数不能为空")
	}
	return nil
}

// validateSelfNode 校验自建节点：名称/链接非空，且链接能被解析引擎逐节点解析。
// 链接先经 parse.ConvertSubscription 转成节点列表，再逐个 parse.Validate——
// 任一节点解析失败都拒绝入库，避免"填错链接但节点静默不出现"。
func validateSelfNode(name, link string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("名称不能为空")
	}
	if strings.TrimSpace(link) == "" {
		return errors.New("分享链接不能为空")
	}
	nodes, err := parse.ConvertSubscription([]byte(strings.TrimSpace(link)))
	if err != nil {
		return fmt.Errorf("分享链接格式不正确：%v", err)
	}
	for _, node := range nodes {
		if err := parse.Validate(node); err != nil {
			return fmt.Errorf("分享链接格式不正确：%v", err)
		}
	}
	return nil
}

// --- /api/airports ---

func (s *Server) handleAPIAirports(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		airports, err := m.ListAirports(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if airports == nil {
			airports = []subforge.Airport{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"airports": airports})
	case http.MethodPost:
		var req struct {
			Name            string `json:"name"`
			SubscriptionURL string `json:"subscription_url"`
			Remarks         string `json:"remarks"`
			Enabled         bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := validateAirport(req.Name, req.SubscriptionURL); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		a, err := m.AddAirport(r.Context(), req.Name, req.SubscriptionURL, req.Remarks, req.Enabled)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"airport": a})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

func (s *Server) handleAPIAirport(w http.ResponseWriter, r *http.Request) {
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
		a, err := m.GetAirport(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"airport": a})
	case http.MethodPut:
		var req struct {
			Name            string `json:"name"`
			SubscriptionURL string `json:"subscription_url"`
			Remarks         string `json:"remarks"`
			Enabled         bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := validateAirport(req.Name, req.SubscriptionURL); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		a, err := m.UpdateAirport(r.Context(), id, req.Name, req.SubscriptionURL, req.Remarks, req.Enabled)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"airport": a})
	case http.MethodDelete:
		detached, derr := m.UnbindAirportRuleSource(r.Context(), id)
		if derr != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, derr)
			return
		}
		if err := m.DeleteAirport(r.Context(), id); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rule_source": detached})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}

// --- /api/groups ---

func (s *Server) handleAPIGroups(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		// ?source=&source_prefix=1 可选过滤：不带参数 → 全部；带参数 → 只看该来源。
		filter, ok := s.parseSourceFilter(w, r)
		if !ok {
			return
		}
		groups, err := m.ListProxyGroupsFiltered(r.Context(), filter)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if groups == nil {
			groups = []subforge.ProxyGroup{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Regex       string `json:"regex"`
			URL         string `json:"url"`
			Interval    int    `json:"interval"`
			SortOrder   int    `json:"sort_order"`
			Source      string `json:"source"`
			IncludeSelf bool   `json:"include_self"`
			// v2 起分组是全局库，只按来源分层；profile_id 已下线，出现即 400。
			ProfileID *int64 `json:"profile_id"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if req.ProfileID != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("分组不再归属订阅方案，请用 source 区分来源"))
			return
		}
		gtype, err := validateProxyGroup(req.Name, req.Type, req.Regex, req.Interval)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		g, err := m.AddProxyGroupWithMembersAndIncludeSelf(r.Context(), req.Name, gtype, req.Regex, req.URL, req.Interval, req.SortOrder, strings.TrimSpace(req.Source), nil, req.IncludeSelf)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"group": g})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

func (s *Server) handleAPIGroup(w http.ResponseWriter, r *http.Request) {
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
		g, err := m.GetProxyGroup(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"group": g})
	case http.MethodPut:
		var req struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Regex       string `json:"regex"`
			URL         string `json:"url"`
			Interval    int    `json:"interval"`
			SortOrder   int    `json:"sort_order"`
			IncludeSelf *bool  `json:"include_self"`
			// v2 起分组不再归属方案，profile_id 出现即 400。
			ProfileID *int64 `json:"profile_id"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if req.ProfileID != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("分组不再归属订阅方案，请用 source 区分来源"))
			return
		}
		gtype, err := validateProxyGroup(req.Name, req.Type, req.Regex, req.Interval)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		// subforge 层把「不存在」翻成中文文案后不再包 sql.ErrNoRows，只靠 errors.Is 判会漏成 500，
		// 所以先做一次存在性探测（同 handleAPIRule 的移方案分支）。
		if _, err := m.GetProxyGroup(r.Context(), id); err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("代理分组 %d 不存在", id))
			return
		}
		if err := m.UpdateProxyGroup(r.Context(), id, req.Name, gtype, req.Regex, req.URL, req.Interval, req.SortOrder); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if req.IncludeSelf != nil {
			if err := m.SetProxyGroupIncludeSelf(r.Context(), id, *req.IncludeSelf); err != nil {
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		g, err := m.GetProxyGroup(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"group": g})
	case http.MethodDelete:
		if err := m.DeleteProxyGroup(r.Context(), id); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}

// --- /api/rules ---

// ruleView 是 GET /api/rules 的响应视图：在 store.Rule 基础上追加
// group_name（由 rule_config 经 ListProxyGroups 映射得到；映射不上或 rule_config=0 时为空字符串）。
type ruleView struct {
	subforge.Rule
	GroupName string `json:"group_name"`
}

// parseSourceFilter 解析 GET /api/rules、/api/groups 的来源过滤参数：
//   - 不带 source：SourceFilter{All:true}（= 全部，保持 v1「无参数返回全部」的行为）
//   - ?source=xxx：精确匹配；再给 ?source_prefix=1 时按前缀匹配
func (s *Server) parseSourceFilter(w http.ResponseWriter, r *http.Request) (subforge.SourceFilter, bool) {
	q := r.URL.Query()
	if !q.Has("source") {
		return subforge.SourceFilter{All: true}, true
	}
	source := q.Get("source")
	prefix := false
	if raw := strings.TrimSpace(q.Get("source_prefix")); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("source_prefix 需要布尔值"))
			return subforge.SourceFilter{}, false
		}
		prefix = v
	}
	if prefix && strings.TrimSpace(source) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("source_prefix 需要非空 source"))
		return subforge.SourceFilter{}, false
	}
	return subforge.SourceFilter{Source: source, Prefix: prefix}, true
}

// isDuplicateRuleSourceError 判断错误是否来自「改来源撞上同来源同去重键」的唯一冲突。
// store 层用固定文案表达该冲突（不 %w 包装 SQLite 原始错误），因此按文案判断。
func isDuplicateRuleSourceError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "目标来源里已存在同来源同去重键的规则")
}

func (s *Server) handleAPIRules(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		// ?source=&source_prefix=1 可选过滤：不带参数 → 全部；带参数 → 只看该来源。
		filter, ok := s.parseSourceFilter(w, r)
		if !ok {
			return
		}
		rules, err := m.ListRulesFiltered(r.Context(), filter)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		// 分组名映射：一次查询，循环里只查 map。
		groupNameByID := map[int64]string{}
		if groups, gerr := m.ListProxyGroups(r.Context()); gerr == nil {
			for _, g := range groups {
				groupNameByID[g.ID] = g.Name
			}
		}
		views := make([]ruleView, 0, len(rules))
		for _, ru := range rules {
			views = append(views, ruleView{
				Rule:      ru,
				GroupName: groupNameByID[ru.RuleConfig],
			})
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"rules": views})
	case http.MethodPost:
		var req struct {
			RuleType   string `json:"rule_type"`
			RuleParam  string `json:"rule_param"`
			RuleConfig int64  `json:"rule_config"`
			ResolveDNS bool   `json:"resolve_dns"`
			Enabled    bool   `json:"enabled"`
			Source     string `json:"source"`
			// v2 起规则只按来源分层；profile_id 已下线，出现即 400。
			ProfileID *int64 `json:"profile_id"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if req.ProfileID != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("规则不再归属订阅方案，请用 source 区分来源"))
			return
		}
		if err := validateRule(req.RuleType, req.RuleParam); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		rule, err := m.AddRuleWithSource(r.Context(), subforge.Rule{
			RuleType:   req.RuleType,
			RuleParam:  req.RuleParam,
			RuleConfig: req.RuleConfig,
			ResolveDNS: req.ResolveDNS,
			Enabled:    req.Enabled,
			Source:     strings.TrimSpace(req.Source),
		})
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"rule": rule})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

func (s *Server) handleAPIRule(w http.ResponseWriter, r *http.Request) {
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
		rule, err := m.GetRule(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"rule": rule})
	case http.MethodPut:
		var req struct {
			RuleType   string `json:"rule_type"`
			RuleParam  string `json:"rule_param"`
			RuleConfig int64  `json:"rule_config"`
			ResolveDNS bool   `json:"resolve_dns"`
			Enabled    bool   `json:"enabled"`
			// Source 非 nil = 改来源（空串=改为自定义）；nil = 不动来源。
			Source *string `json:"source"`
			// v2 起规则只按来源分层；profile_id 已下线，出现即 400。
			ProfileID *int64 `json:"profile_id"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if req.ProfileID != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("规则不再归属订阅方案，请用 source 区分来源"))
			return
		}
		if err := validateRule(req.RuleType, req.RuleParam); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		// 先改来源再覆盖其余字段：撞唯一索引 (source, origin_key) 时整次更新失败。
		if req.Source != nil {
			if err := m.DB().SetRuleSource(r.Context(), id, strings.TrimSpace(*req.Source)); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则 %d 不存在", id))
					return
				}
				if isDuplicateRuleSourceError(err) {
					s.writeAPIError(w, r, http.StatusConflict, err)
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
		}
		if err := m.UpdateRule(r.Context(), id, req.RuleType, req.RuleParam, req.RuleConfig, req.ResolveDNS, req.Enabled); err != nil {
			// store 对不存在的 id 返回 sql.ErrNoRows；不翻成 404 会漏成 500。
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则 %d 不存在", id))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		rule, err := m.GetRule(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"rule": rule})
	case http.MethodDelete:
		if err := m.DeleteRule(r.Context(), id); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}

// --- /api/tokens ---

func (s *Server) handleAPITokens(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		tokens, err := m.ListSubTokens(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if tokens == nil {
			tokens = []subforge.SubToken{}
		}
		// profile_name 不落库，由 profiles 映射统一回填（未绑定方案时省略）。
		names, err := profileNameByID(r.Context(), m)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		for i := range tokens {
			tokens[i].ProfileName = names[tokens[i].ProfileID]
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
	case http.MethodPost:
		var req struct {
			Token     string `json:"token"`
			Name      string `json:"name"`
			Target    string `json:"target"`
			Pipeline  string `json:"pipeline"`
			Config    string `json:"config"`
			Include   string `json:"include"`
			Exclude   string `json:"exclude"`
			ProfileID int64  `json:"profile_id"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		// 留空即由服务端生成：16 字节 crypto/rand → 32 位 hex。
		if strings.TrimSpace(req.Token) == "" {
			buf := make([]byte, 16)
			if _, err := rand.Read(buf); err != nil {
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
			req.Token = hex.EncodeToString(buf)
		}
		// profile_id > 0 必须先确认方案存在，避免令牌指向空方案。
		if !s.profileExists(w, r, m, req.ProfileID) {
			return
		}
		if err := m.CreateSubTokenWithProfile(r.Context(), req.Token, req.Name,
			req.Target, req.Pipeline, req.Config, req.Include, req.Exclude, req.ProfileID); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		t, err := m.GetSubToken(r.Context(), req.Token)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if err := s.fillTokenProfileName(r.Context(), m, t); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"token": t})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

func (s *Server) handleAPIToken(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	switch r.Method {
	case http.MethodGet:
		t, err := m.GetSubToken(r.Context(), token)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, errors.New("令牌不存在"))
			return
		}
		if err := s.fillTokenProfileName(r.Context(), m, t); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"token": t})
	case http.MethodPut:
		// 部分更新：JSON 里省略的字段指针为 nil → 不动该列；
		// 显式提供（含空串）→ 写入该值，空串即清空。
		// 全部用 *string 才能区分「未提供」与「清空」。
		var req struct {
			Name     *string `json:"name"`
			Target   *string `json:"target"`
			Pipeline *string `json:"pipeline"`
			Config   *string `json:"config"`
			Include  *string `json:"include"`
			Exclude  *string `json:"exclude"`
			// ProfileID 非 nil = 改绑方案（0 = 解绑，合法）；nil = 不动归属。
			ProfileID *int64 `json:"profile_id"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		// profile_id > 0 必须先确认方案存在；0 是解绑，不校验。
		if req.ProfileID != nil && !s.profileExists(w, r, m, *req.ProfileID) {
			return
		}
		upd := store.SubTokenUpdate{
			Name:      req.Name,
			Target:    req.Target,
			Pipeline:  req.Pipeline,
			Config:    req.Config,
			Include:   req.Include,
			Exclude:   req.Exclude,
			ProfileID: req.ProfileID,
		}
		if err := m.UpdateSubToken(r.Context(), token, upd); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, errors.New("令牌不存在"))
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		t, err := m.GetSubToken(r.Context(), token)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if err := s.fillTokenProfileName(r.Context(), m, t); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"token": t})
	case http.MethodDelete:
		if err := m.DeleteSubToken(r.Context(), token); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}

// --- /subscribe?token=xxx ---

// handleSubscribe 按数据库当前状态实时编译一份订阅配置。
//
// 每次请求都重新读 airports / proxy_groups / rules，所以任何一项改了，
// 客户端下一次拉取就是新配置——不存在落盘的静态产物。
// 鉴权用订阅令牌（sub_token）而不是服务端访问令牌；因此 server.go 的全局
// auth 中间件必须豁免 /subscribe，否则 query 上的 sub_token 会被拿去和
// 服务端令牌比对，永远 401。
// --- /api/self-nodes ---

func (s *Server) handleAPISelfNodes(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		nodes, err := m.ListSelfNodes(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if nodes == nil {
			nodes = []subforge.SelfNode{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"self_nodes": nodes})
	case http.MethodPost:
		var req struct {
			Link      string `json:"link"`
			Name      string `json:"name"`
			Enabled   bool   `json:"enabled"`
			SortOrder int    `json:"sort_order"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := validateSelfNode(req.Name, req.Link); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		n, err := m.AddSelfNode(r.Context(), req.Link, req.Name, req.Enabled, req.SortOrder)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"self_node": n})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

func (s *Server) handleAPISelfNode(w http.ResponseWriter, r *http.Request) {
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
		n, err := m.GetSelfNode(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"self_node": n})
	case http.MethodPut:
		var req struct {
			Link      string `json:"link"`
			Name      string `json:"name"`
			Enabled   bool   `json:"enabled"`
			SortOrder int    `json:"sort_order"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := validateSelfNode(req.Name, req.Link); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		if err := m.UpdateSelfNode(r.Context(), id, req.Link, req.Name, req.Enabled, req.SortOrder); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		if err := m.DeleteSelfNode(r.Context(), id); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("缺少 token 参数"))
		return
	}
	subTok, err := m.GetSubToken(r.Context(), token)
	if err != nil {
		// 不回显令牌值：订阅令牌本身就是凭据，错误文案与日志都不该二次扩散。
		s.writeAPIError(w, r, http.StatusNotFound, errors.New("订阅令牌不存在"))
		return
	}

	// 令牌绑定的方案：停用 → 400，不存在 → 404；命中后模板/管线/规则/分组都按方案取数。
	profileID := subTok.ProfileID
	var profile *subforge.Profile
	if profileID > 0 {
		p, perr := m.GetProfile(r.Context(), profileID)
		if perr != nil {
			s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("令牌绑定的订阅方案 %d 不存在", profileID))
			return
		}
		if !p.Enabled {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("订阅方案 “%s” 已停用", p.Name))
			return
		}
		profile = p
	}

	// 五个定制字段的取值优先级：query 显式参数 > 令牌字段 > 原回落链。
	// 令牌字段为空串（未定制）时逐字节等价于改动前的行为。
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = strings.TrimSpace(subTok.Target)
	}
	if target == "" && profile != nil {
		target = strings.TrimSpace(profile.Target)
	}
	if target == "" {
		target = "clash.meta"
	}

	// pipeline：显式 query 优先；其次令牌字段；都为空时回落到 config 表的 subscribe.pipeline。
	pipeline := strings.TrimSpace(r.URL.Query().Get("pipeline"))
	if pipeline == "" {
		pipeline = strings.TrimSpace(subTok.Pipeline)
	}
	// 令牌未定制时用方案的默认管线（方案层介于令牌与全局配置之间）。
	if pipeline == "" && profile != nil {
		pipeline = strings.TrimSpace(profile.Pipeline)
	}
	if pipeline == "" {
		if v, err := m.GetConfig(r.Context(), "subscribe.pipeline"); err == nil {
			pipeline = strings.TrimSpace(v)
		}
	}
	configName := strings.TrimSpace(r.URL.Query().Get("config"))
	if configName == "" {
		configName = strings.TrimSpace(subTok.Config)
	}
	// 令牌未定制模板时用方案的默认模板；仍为空且绑定了方案时再查全局配置表。
	if configName == "" && profile != nil {
		configName = strings.TrimSpace(profile.Config)
		if configName == "" {
			if v, cerr := m.GetConfig(r.Context(), "subscribe.config"); cerr == nil {
				configName = strings.TrimSpace(v)
			}
		}
	}
	includeList := splitListQuery(r.URL.Query().Get("include"))
	if len(includeList) == 0 {
		includeList = splitListQuery(subTok.Include)
	}
	if len(includeList) == 0 && profile != nil {
		includeList = splitListQuery(profile.Include)
	}
	excludeList := splitListQuery(r.URL.Query().Get("exclude"))
	if len(excludeList) == 0 {
		excludeList = splitListQuery(subTok.Exclude)
	}
	if len(excludeList) == 0 && profile != nil {
		excludeList = splitListQuery(profile.Exclude)
	}

	// 1. 启用的订阅源（ListAirports 已按 sort_order, id 排序）。
	airports, err := m.ListAirports(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	// 方案可限制来源：nil = 全部启用来源；空数组 = 该类来源一个都不用。
	airportFilter := idFilter(profile, func(p *subforge.Profile) []int64 { return p.AirportIDs })
	selfFilter := idFilter(profile, func(p *subforge.Profile) []int64 { return p.SelfNodeIDs })

	var urls []string
	usedAirports := make([]subforge.Airport, 0, len(airports))
	for _, a := range airports {
		if airportFilter != nil && !airportFilter[a.ID] {
			continue
		}
		if a.Enabled && strings.TrimSpace(a.SubscriptionURL) != "" {
			urls = append(urls, a.SubscriptionURL)
			usedAirports = append(usedAirports, a)
		}
	}

	// 自建节点：每个启用的节点链接写入临时文件，作为 File 来源传给引擎。
	var selfNodeFiles []string
	selfNodeNames := []string{}
	selfNodes, err := m.ListSelfNodes(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if len(selfNodes) > 0 {
		tmpDir, err := os.MkdirTemp("", "subforge-self-*")
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		defer os.RemoveAll(tmpDir)
		for _, n := range selfNodes {
			if selfFilter != nil && !selfFilter[n.ID] {
				continue
			}
			if !n.Enabled || strings.TrimSpace(n.Link) == "" {
				continue
			}
			fp := filepath.Join(tmpDir, fmt.Sprintf("self-%d.txt", n.ID))
			if err := os.WriteFile(fp, []byte(applySelfNodeName(n.Link, n.Name)), 0o600); err != nil {
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
			selfNodeFiles = append(selfNodeFiles, fp)
			if name := strings.TrimSpace(n.Name); name != "" {
				selfNodeNames = append(selfNodeNames, name)
			}
		}
	}

	if len(urls) == 0 && len(selfNodeFiles) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("没有启用的订阅或自建节点"))
		return
	}

	// 2. 代理分组：regex 非空时作为成员模式交给引擎按节点名展开。
	//    v2 起按方案的 group_layers 顺序合并（同名先到者胜），模板分组由引擎在 DB 分组
	//    之后追加（DB 优先，修 D-36）；未绑定方案时用默认三层。
	groupLayers, ruleLayers := subscribeLayers(profile)
	groups, err := m.ListProxyGroupsByLayers(r.Context(), groupLayers)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	groupName := make(map[int64]string, len(groups))
	renderGroups := make([]render.Group, 0, len(groups))
	for _, g := range groups {
		groupName[g.ID] = g.Name
		rg := render.Group{
			Name:    g.Name,
			Type:    g.Type,
			Test:    g.URL,
			Options: render.UrlTestOptions{Interval: g.Interval},
		}
		// 机场自带分组按成员原样渲染；成员可能是节点名，也可能是子组名，
		// ResolveGroups 会保留已知组名、展开正则/节点名。手工分组仍走 Regex。
		if len(g.Members) > 0 {
			rg.Proxies = append([]string{}, g.Members...)
		} else if re := strings.TrimSpace(g.Regex); re != "" {
			rg.Proxies = []string{re}
		}
		if g.IncludeSelf {
			rg.Proxies = appendMissingNames(rg.Proxies, selfNodeNames)
		}
		renderGroups = append(renderGroups, rg)
	}

	// 3. 规则：rule_config 指向的 group id → 策略名，0 或找不到 → DIRECT。
	//    v2 起按方案的 rule_layers 顺序拼接（只取已启用；模板规则由引擎在 DB 规则后展开）。
	rules, err := m.ListRulesByLayers(r.Context(), ruleLayers)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	renderRules := make([]string, 0, len(rules))
	for _, ru := range rules {
		if !ru.Enabled {
			continue
		}
		policy := "DIRECT"
		if ru.RuleConfig > 0 {
			if name, found := groupName[ru.RuleConfig]; found && strings.TrimSpace(name) != "" {
				policy = name
			}
		} else if raw := strings.TrimSpace(ru.Raw); raw != "" {
			// 导入的规则如果目标分组当时还不存在，rule_config 是 0，但 raw 里保留着
			// 规则原文——原目标名不能在这里丢掉：渲染阶段会按名称把它映射到实际
			// 存在的分组（内部点名的机场分组常与模板分组叫法不同，见
			// internal/render/ruletarget.go）。直接写 DIRECT 会把规则悄悄改成直连。
			renderRules = append(renderRules, raw)
			continue
		}
		line := strings.Join([]string{ru.RuleType, ru.RuleParam, policy}, ",")
		if ru.ResolveDNS {
			line += ",no-resolve"
		}
		renderRules = append(renderRules, line)
	}

	// 规则源产物：按方案的 rule_sources 顺序读各源的「当前版本」产物并内联进订阅。
	// 注入成功（>0 条）时替换模板规则：模板规则里的 MATCH 会排在注入规则之前，
	// 不替换的话产物规则永远命中不到。注入 0 条（源缺失/无产物/格式不支持）时
	// 保留模板规则，warning 只是可见提示，不阻断订阅。
	var sourceRules, sourceWarns []string
	if profile != nil && len(profile.RuleSources) > 0 {
		sourceRules, sourceWarns = m.RuleSourceRules(r.Context(), profile.RuleSources)
		if len(sourceRules) > 0 {
			renderRules = append(renderRules, sourceRules...)
			sourceWarns = append(sourceWarns, fmt.Sprintf("已用规则源产物替换模板规则：注入 %d 条", len(sourceRules)))
		}
	}

	// 规则源产物里的 rule-providers：绑定源 + 已启用的自定义 provider 一起渲染，
	// 否则 RULE-SET 规则找不到规则集。同名 provider 先出现的源优先。
	var ruleProviders []model.RuleProvider
	if profile != nil && len(profile.RuleSources) > 0 {
		rp, rpWarns := m.RuleSourceRuleProviders(r.Context(), profile.RuleSources)
		ruleProviders = append(ruleProviders, rp...)
		sourceWarns = append(sourceWarns, rpWarns...)
	}
	if all, perr := m.DB().ListRuleProviders(r.Context()); perr == nil {
		for _, p := range all {
			if !p.Enabled || p.Source != "" {
				continue
			}
			var raw map[string]any
			if json.Unmarshal([]byte(p.RawJSON), &raw) == nil && len(raw) > 0 {
				ruleProviders = append(ruleProviders, model.RuleProvider{Name: p.Name, Raw: raw})
			}
		}
	}
	uniqueProviders := make([]model.RuleProvider, 0, len(ruleProviders))
	seenProvider := map[string]bool{}
	for _, p := range ruleProviders {
		if p.Name == "" || seenProvider[p.Name] {
			continue
		}
		seenProvider[p.Name] = true
		uniqueProviders = append(uniqueProviders, p)
	}

	// 4. 交给与 CLI /api/convert 同一条执行路径，避免两处漂移。
	params := convert.Params{
		Target:        target,
		URLs:          urls,
		Files:         selfNodeFiles,
		Groups:        renderGroups,
		Rules:         renderRules,
		RuleProviders: uniqueProviders,
		Pipeline:      pipeline,
		Config:        configName,
		Include:       includeList,
		Exclude:       excludeList,
		// 模板层不在层数组里 = 该方案不用 INI 模板的规则/分组。
		// 规则源产物注入后模板规则必须整层停用，否则产物规则排在模板 MATCH 之后全成死规则。
		DisableTemplateRules:  !hasLayer(ruleLayers, "template") || len(sourceRules) > 0,
		DisableTemplateGroups: !hasLayer(groupLayers, "template"),
		TemplateWarnings:      sourceWarns,
	}
	result, err := s.engine.Run(r.Context(), params)
	if err != nil {
		s.writeAPIError(w, r, statusFor(err), err)
		return
	}
	if len(result.Failures) > 0 {
		s.logger.Printf("rid=%s /subscribe 部分来源失败(%d): %s",
			dash(chimw.GetReqID(r.Context())), len(result.Failures),
			strings.Join(result.Failures, " | "))
	}
	// 5. 流量余额透传：把上游 subscription-userinfo 按策略聚合后写进响应头，
	//    下游 mihomo/clash 客户端据此显示余额与到期。
	//
	// 只统计「已成功采集过」的机场（TrafficAt > 0）：
	// 从未采集成功的机场流量是零值，混进来会把 min/max/avg 拉偏。
	// 读策略失败（DB 故障）时静默跳过——订阅本身仍然要能生成。
	if strat, err := m.GetTrafficStrategy(r.Context()); err == nil {
		usages := make([]traffic.Usage, 0, len(usedAirports))
		nodeCounts := make([]int, 0, len(usedAirports))
		for _, a := range usedAirports {
			if a.TrafficAt <= 0 {
				continue
			}
			usages = append(usages, traffic.Usage{
				Upload: a.Upload, Download: a.Download, Total: a.Total, Expire: a.Expire,
			})
			nodeCounts = append(nodeCounts, a.NodeCount)
		}
		// Aggregate 在「无可聚合数据」或「策略为不透传（全 none）」时返回 ok=false。
		if u, ok := traffic.Aggregate(usages, nodeCounts, strat); ok {
			if header := traffic.FormatWithStrategy(u, strat); header != "" {
				w.Header().Set("Subscription-Userinfo", header)
			}
		}
	}
	s.writeText(w, http.StatusOK, "text/plain; charset=utf-8", string(result.Body))
}

// applySelfNodeName 把控制台填写的名称写进分享链接。
//
// 自建节点的「名称」是用户在界面上明确配置的输出名，链接自带的 fragment/ps
// 只应作为兜底；直接写原始链接会让界面上的名称完全失效（VMess 链接的 ps
// 常为空，输出甚至会出现无名字节点）。URI 类链接写 fragment，vmess 写
// base64 JSON 的 ps 字段。
func applySelfNodeName(link, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return link
	}
	if strings.HasPrefix(strings.ToLower(link), "vmess://") {
		if out, ok := vmessWithName(link, name); ok {
			return out
		}
		return link
	}
	base := link
	if i := strings.IndexByte(base, '#'); i >= 0 {
		base = base[:i]
	}
	escaped := strings.ReplaceAll(url.PathEscape(name), "+", "%2B")
	return base + "#" + escaped
}

// vmessWithName 解出 vmess 链接的 Base64 JSON，写入 ps 后重新编码。
// 链接不是合法 vmess 时返回 ok=false，由调用方保留原文（校验在入库时已完成）。
func vmessWithName(link, name string) (string, bool) {
	raw := strings.TrimSpace(link[len("vmess://"):])
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSpace(raw)
	encodings := []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	}
	var decoded []byte
	ok := false
	for _, enc := range encodings {
		if b, err := enc.DecodeString(raw); err == nil {
			decoded, ok = b, true
			break
		}
	}
	if !ok {
		return "", false
	}
	var obj map[string]any
	if err := json.Unmarshal(decoded, &obj); err != nil {
		return "", false
	}
	obj["ps"] = name
	b, err := json.Marshal(obj)
	if err != nil {
		return "", false
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(b), true
}

// appendMissingNames 把 extra 里尚未出现的名字追加到 base 末尾，保持顺序并去重。
func appendMissingNames(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(extra))
	for _, v := range base {
		seen[v] = true
	}
	for _, v := range extra {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		base = append(base, v)
	}
	return base
}

// idFilter 把方案里的来源 id 数组转成查找表。
// nil = 未限制（返回 nil，调用方按“全部启用来源”处理）；空数组 = 明确不使用。
func idFilter(p *subforge.Profile, pick func(*subforge.Profile) []int64) map[int64]bool {
	if p == nil {
		return nil
	}
	ids := pick(p)
	if ids == nil {
		return nil
	}
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// subscribeLayers 返回订阅组装要用的分组层与规则层顺序：
// 令牌绑定了方案就用方案配置，未绑定（或方案未配置）用默认三层。
func subscribeLayers(profile *subforge.Profile) (groupLayers, ruleLayers []string) {
	groupLayers = store.DefaultLayers
	ruleLayers = store.DefaultLayers
	if profile == nil {
		return groupLayers, ruleLayers
	}
	if len(profile.GroupLayers) > 0 {
		groupLayers = profile.GroupLayers
	}
	if len(profile.RuleLayers) > 0 {
		ruleLayers = profile.RuleLayers
	}
	return groupLayers, ruleLayers
}

// hasLayer 报告层数组里是否含某层键。
func hasLayer(layers []string, want string) bool {
	for _, layer := range layers {
		if layer == want {
			return true
		}
	}
	return false
}

// splitListQuery 把逗号分隔的 query 值切成去空白的非空切片。
func splitListQuery(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
