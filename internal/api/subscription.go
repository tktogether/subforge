// 订阅管理 HTTP 处理器：/api/airports, /api/groups, /api/rules, /api/tokens
// 以及 /subscribe?token=xxx 生成订阅。
package api

import (
	"crypto/rand"
	"database/sql"
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
func (s *Server) writeAPIError(w http.ResponseWriter, r *http.Request, status int, err error) {
	s.fail(w, r, status, err)
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
		if err := m.DeleteAirport(r.Context(), id); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
		groups, err := m.ListProxyGroups(r.Context())
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
			Name      string `json:"name"`
			Type      string `json:"type"`
			Regex     string `json:"regex"`
			URL       string `json:"url"`
			Interval  int    `json:"interval"`
			SortOrder int    `json:"sort_order"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		gtype, err := validateProxyGroup(req.Name, req.Type, req.Regex, req.Interval)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		g, err := m.AddProxyGroup(r.Context(), req.Name, gtype, req.Regex, req.URL, req.Interval, req.SortOrder)
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
			Name      string `json:"name"`
			Type      string `json:"type"`
			Regex     string `json:"regex"`
			URL       string `json:"url"`
			Interval  int    `json:"interval"`
			SortOrder int    `json:"sort_order"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		gtype, err := validateProxyGroup(req.Name, req.Type, req.Regex, req.Interval)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		if err := m.UpdateProxyGroup(r.Context(), id, req.Name, gtype, req.Regex, req.URL, req.Interval, req.SortOrder); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
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

// ruleView 是 GET /api/rules 的响应视图：在 store.Rule 基础上追加 group_name（由 rule_config 经
// ListProxyGroups 映射得到；映射不上或 rule_config=0 时为空字符串）。
type ruleView struct {
	subforge.Rule
	GroupName string `json:"group_name"`
}

func (s *Server) handleAPIRules(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rules, err := m.ListRules(r.Context())
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
			views = append(views, ruleView{Rule: ru, GroupName: groupNameByID[ru.RuleConfig]})
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"rules": views})
	case http.MethodPost:
		var req struct {
			RuleType   string `json:"rule_type"`
			RuleParam  string `json:"rule_param"`
			RuleConfig int64  `json:"rule_config"`
			ResolveDNS bool   `json:"resolve_dns"`
			Enabled    bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := validateRule(req.RuleType, req.RuleParam); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		rule, err := m.AddRule(r.Context(), req.RuleType, req.RuleParam, req.RuleConfig, req.ResolveDNS, req.Enabled)
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
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := validateRule(req.RuleType, req.RuleParam); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		if err := m.UpdateRule(r.Context(), id, req.RuleType, req.RuleParam, req.RuleConfig, req.ResolveDNS, req.Enabled); err != nil {
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
		s.writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
	case http.MethodPost:
		var req struct {
			Token    string `json:"token"`
			Name     string `json:"name"`
			Target   string `json:"target"`
			Pipeline string `json:"pipeline"`
			Config   string `json:"config"`
			Include  string `json:"include"`
			Exclude  string `json:"exclude"`
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
		if err := m.CreateSubToken(r.Context(), req.Token, req.Name,
			req.Target, req.Pipeline, req.Config, req.Include, req.Exclude); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		t, err := m.GetSubToken(r.Context(), req.Token)
		if err != nil {
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
			s.writeAPIError(w, r, http.StatusNotFound, err)
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
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		upd := store.SubTokenUpdate{
			Name:     req.Name,
			Target:   req.Target,
			Pipeline: req.Pipeline,
			Config:   req.Config,
			Include:  req.Include,
			Exclude:  req.Exclude,
		}
		if err := m.UpdateSubToken(r.Context(), token, upd); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, errors.New("令牌 "+token+" 不存在"))
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
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}

	// 五个定制字段的取值优先级：query 显式参数 > 令牌字段 > 原回落链。
	// 令牌字段为空串（未定制）时逐字节等价于改动前的行为。
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = strings.TrimSpace(subTok.Target)
	}
	if target == "" {
		target = "clash.meta"
	}

	// pipeline：显式 query 优先；其次令牌字段；都为空时回落到 config 表的 subscribe.pipeline。
	pipeline := strings.TrimSpace(r.URL.Query().Get("pipeline"))
	if pipeline == "" {
		pipeline = strings.TrimSpace(subTok.Pipeline)
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
	includeList := splitListQuery(r.URL.Query().Get("include"))
	if len(includeList) == 0 {
		includeList = splitListQuery(subTok.Include)
	}
	excludeList := splitListQuery(r.URL.Query().Get("exclude"))
	if len(excludeList) == 0 {
		excludeList = splitListQuery(subTok.Exclude)
	}

	// 1. 启用的订阅源（ListAirports 已按 sort_order, id 排序）。
	airports, err := m.ListAirports(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	var urls []string
	for _, a := range airports {
		if a.Enabled && strings.TrimSpace(a.SubscriptionURL) != "" {
			urls = append(urls, a.SubscriptionURL)
		}
	}

	// 自建节点：每个启用的节点链接写入临时文件，作为 File 来源传给引擎。
	var selfNodeFiles []string
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
			if !n.Enabled || strings.TrimSpace(n.Link) == "" {
				continue
			}
			fp := filepath.Join(tmpDir, fmt.Sprintf("self-%d.txt", n.ID))
			if err := os.WriteFile(fp, []byte(n.Link), 0o600); err != nil {
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
			selfNodeFiles = append(selfNodeFiles, fp)
		}
	}

	if len(urls) == 0 && len(selfNodeFiles) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("没有启用的订阅或自建节点"))
		return
	}

	// 2. 代理分组：regex 非空时作为成员模式交给引擎按节点名展开。
	groups, err := m.ListProxyGroups(r.Context())
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
		if re := strings.TrimSpace(g.Regex); re != "" {
			rg.Proxies = []string{re}
		}
		renderGroups = append(renderGroups, rg)
	}

	// 3. 规则：rule_config 指向的 group id → 策略名，0 或找不到 → DIRECT。
	rules, err := m.ListRules(r.Context())
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

	// 4. 交给与 CLI /api/convert 同一条执行路径，避免两处漂移。
	params := convert.Params{
		Target:   target,
		URLs:     urls,
		Files:    selfNodeFiles,
		Groups:   renderGroups,
		Rules:    renderRules,
		Pipeline: pipeline,
		Config:   configName,
		Include:  includeList,
		Exclude:  excludeList,
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
		usages := make([]traffic.Usage, 0, len(airports))
		nodeCounts := make([]int, 0, len(airports))
		for _, a := range airports {
			if !a.Enabled || strings.TrimSpace(a.SubscriptionURL) == "" || a.TrafficAt <= 0 {
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
