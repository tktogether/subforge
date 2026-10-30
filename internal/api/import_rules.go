package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/subforge"
)

// importHTTPError 给导入失败带上 HTTP 状态语义：
// 上游拉取/解析失败 → 502；写库失败 → 500；前置条件缺失 → 503。
type importHTTPError struct {
	status int
	err    error
}

func (e *importHTTPError) Error() string { return e.err.Error() }
func (e *importHTTPError) Unwrap() error { return e.err }

// importRulesFromAirport 拉取一个机场的订阅并导入规则与规则集。
// 导入的规则来源恒为 airport:<机场名>，进入全局规则库，不再归属任何订阅方案。
// warnings = dec.Warnings + ir.Warnings 合并（非 nil 切片）。
func (s *Server) importRulesFromAirport(ctx context.Context, m *subforge.Manager, a *subforge.Airport) (subforge.ImportResult, []string, error) {
	return s.importRulesToLibrary(ctx, m, a, subforge.ImportOptions{})
}

// importRulesToLibrary 是导入的唯一入口：source 由机场名派生；
// opts.CreateGroups 时自动补建缺失的目标分组（分组的 source 跟随规则来源），
// opts.EnableRules 时新规则直接启用（否则进候选队列）。
func (s *Server) importRulesToLibrary(ctx context.Context, m *subforge.Manager, a *subforge.Airport, opts subforge.ImportOptions) (subforge.ImportResult, []string, error) {
	if m == nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusServiceUnavailable, errors.New("订阅管理未启用")}
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusServiceUnavailable, errors.New("转换引擎未启用")}
	}
	res := s.engine.Fetcher.FetchFrom(ctx, fetch.Source{Name: a.Name, URL: a.SubscriptionURL})
	if res.Err != nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusBadGateway, fmt.Errorf("导入失败: %w", res.Err)}
	}
	dec, err := parse.DecodeSource(res.Content)
	if err != nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusBadGateway, fmt.Errorf("导入失败: %w", err)}
	}
	if opts.CreateGroups {
		opts.UpstreamGroups = dec.ProxyGroups
		opts.UpstreamNodeNames = make([]string, 0, len(dec.Nodes))
		for _, n := range dec.Nodes {
			opts.UpstreamNodeNames = append(opts.UpstreamNodeNames, parse.NameOf(n))
		}
	}
	// 机场规则不再写 rules 表；解析规则只为按目标名补建分组。
	opts.SkipRules = true
	ir, err := m.ImportRulesToLibraryWithOptions(ctx, "airport:"+a.Name, dec.Rules, dec.RuleProviders, opts)
	if err != nil {
		return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusInternalServerError, fmt.Errorf("导入失败: %w", err)}
	}
	if len(dec.Rules) > 0 {
		src, serr := m.SaveAirportRuleSource(ctx, a.Name, a.ID, a.SubscriptionURL, dec.Rules, dec.RuleProviders)
		if serr != nil {
			return subforge.ImportResult{Warnings: []string{}}, []string{}, &importHTTPError{http.StatusInternalServerError, fmt.Errorf("保存机场规则源失败: %w", serr)}
		}
		ir.RuleSourceName = src.Name
		ir.RuleSourceVersion = src.Version
		ir.RuleSourceRules = src.Rules
	}
	warnings := make([]string, 0, len(dec.Warnings)+len(ir.Warnings))
	warnings = append(warnings, dec.Warnings...)
	warnings = append(warnings, ir.Warnings...)
	return ir, warnings, nil
}

// handleAPIImportRules 实现 POST /api/airports/{id}/import-rules。
func (s *Server) handleAPIImportRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, fmt.Errorf("转换引擎未启用"))
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	a, err := m.GetAirport(r.Context(), id)
	if err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("机场不存在"))
		return
	}
	// body 可选：空 body / EOF → 不自动建组，兼容旧调用。
	// v2 起规则/分组是全局库，导入不再有归属方案；旧客户端仍传 profile_id 时明确拒绝，
	// 避免它以为「导到某个方案」而实际进了全局库。
	var body struct {
		ProfileID    *int64 `json:"profile_id"`
		CreateGroups bool   `json:"create_groups"`
		// EnableRules 省略或 false 时保持旧语义：只进候选，不自动生效。
		EnableRules bool `json:"enable_rules"`
	}
	if r.Body != nil {
		if derr := json.NewDecoder(r.Body).Decode(&body); derr != nil && !errors.Is(derr, io.EOF) {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("请求参数格式不正确: %w", derr))
			return
		}
	}
	if body.ProfileID != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("导入的规则不再归属订阅方案（来源恒为 airport:%s），请改用规则页按来源管理", a.Name))
		return
	}
	ir, warnings, err := s.importRulesToLibrary(r.Context(), m, a, subforge.ImportOptions{
		CreateGroups: body.CreateGroups,
		EnableRules:  body.EnableRules,
	})
	if err != nil {
		status := http.StatusInternalServerError
		var he *importHTTPError
		if errors.As(err, &he) {
			status = he.status
		}
		s.writeAPIError(w, r, status, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"rules_added":         ir.RulesAdded,
		"rules_skipped":       ir.RulesSkipped,
		"rules_enabled":       0,
		"providers_added":     ir.ProvidersAdded,
		"groups_created":      ir.GroupsCreated,
		"rule_source_name":    ir.RuleSourceName,
		"rule_source_version": ir.RuleSourceVersion,
		"rule_source_rules":   ir.RuleSourceRules,
		"warnings":            warnings,
	})
}
