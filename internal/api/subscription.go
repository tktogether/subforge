// 订阅管理 HTTP 处理器：/api/airports, /api/groups, /api/rules, /api/tokens
// 以及 /subscribe?token=xxx 生成订阅。
package api

import (
	"fmt"
	"os"
	"path/filepath"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/subforge"
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
			Name     string `json:"name"`
			Type     string `json:"type"`
			Regex    string `json:"regex"`
			URL      string `json:"url"`
			Interval int    `json:"interval"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		g, err := m.AddProxyGroup(r.Context(), req.Name, req.Type, req.Regex, req.URL, req.Interval)
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
			Name     string `json:"name"`
			Type     string `json:"type"`
			Regex    string `json:"regex"`
			URL      string `json:"url"`
			Interval int    `json:"interval"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if err := m.UpdateProxyGroup(r.Context(), id, req.Name, req.Type, req.Regex, req.URL, req.Interval); err != nil {
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
		if rules == nil {
			rules = []subforge.Rule{}
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
		if !s.decodeBody(w, r, &req) {
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
			Token string `json:"token"`
			Name  string `json:"name"`
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
		if err := m.CreateSubToken(r.Context(), req.Token, req.Name); err != nil {
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
	case http.MethodDelete:
		if err := m.DeleteSubToken(r.Context(), token); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/DELETE"))
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
			Link    string `json:"link"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Link) == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("链接不能为空"))
			return
		}
		n, err := m.AddSelfNode(r.Context(), req.Link, req.Name, req.Enabled)
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
			Link    string `json:"link"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Link) == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("链接不能为空"))
			return
		}
		if err := m.UpdateSelfNode(r.Context(), id, req.Link, req.Name, req.Enabled); err != nil {
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
	if _, err := m.GetSubToken(r.Context(), token); err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}

	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = "clash.meta"
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
		}
		line := strings.Join([]string{ru.RuleType, ru.RuleParam, policy}, ",")
		if ru.ResolveDNS {
			line += ",no-resolve"
		}
		renderRules = append(renderRules, line)
	}

	// 4. 交给与 CLI /api/convert 同一条执行路径，避免两处漂移。
	params := convert.Params{
		Target: target,
		URLs:   urls,
		Files:  selfNodeFiles,
		Groups: renderGroups,
		Rules:  renderRules,
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
	s.writeText(w, http.StatusOK, "text/plain; charset=utf-8", string(result.Body))
}
