package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/render"
	"github.com/tktogether/subforge/internal/subcfg"
	"github.com/tktogether/subforge/internal/subforge"
)

// handleAPIProfileFinalGroupPreview 实现 POST /api/profiles/{id}/final-group/preview。
// 使用方案当前的分组/规则/规则源拼接输入，识别各来源最终组并返回改写报告；
// 不拉取节点、不落库。草稿配置通过 final_group 字段传入。
func (s *Server) handleAPIProfileFinalGroupPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	p, err := m.GetProfile(r.Context(), id)
	if err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}
	var body struct {
		FinalGroup *finalGroupBody `json:"final_group"`
	}
	if !s.decodeBody(w, r, &body) {
		return
	}
	cfg := p.FinalGroup
	if body.FinalGroup != nil {
		cfg, err = body.FinalGroup.config()
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("统一最终选择组配置非法：%w", err))
			return
		}
	}
	if !cfg.Enabled {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("统一最终选择组未启用"))
		return
	}

	groups, rules, metas, warnings, err := s.finalGroupPreviewInputs(r.Context(), m, p)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	_, _, report, unifyWarns := render.UnifyFinalGroup(groups, rules, nil, cfg, metas)
	warnings = append(warnings, unifyWarns...)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"report":   report,
		"warnings": warnings,
	})
}

// finalGroupPreviewInputs 复用订阅组装的口径，但不拉取机场订阅：
// 只读 DB 分组/规则、规则源当前产物，供预览识别使用。
func (s *Server) finalGroupPreviewInputs(ctx context.Context, m *subforge.Manager, p *subforge.Profile) ([]render.Group, []string, []subcfg.RuleSourceMeta, []string, error) {
	var warnings []string
	groups, err := m.ListProxyGroupsByLayers(ctx, p.GroupLayers)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	groupName := make(map[int64]string, len(groups))
	renderGroups := make([]render.Group, 0, len(groups))
	for _, g := range groups {
		groupName[g.ID] = g.Name
		rg := render.Group{
			Name:              g.Name,
			Type:              g.Type,
			Test:              g.URL,
			Options:           render.UrlTestOptions{Interval: g.Interval},
			IncludeSelf:       g.IncludeSelf,
			IncludeAllSources: g.IncludeAllSources,
			Source:            g.Source,
		}
		if len(g.Members) > 0 {
			rg.Proxies = append([]string{}, g.Members...)
		} else if re := strings.TrimSpace(g.Regex); re != "" {
			rg.Proxies = []string{re}
		}
		renderGroups = append(renderGroups, rg)
	}
	if hasLayer(p.GroupLayers, "template") {
		warnings = append(warnings,
			"预览未展开模板层分组/规则；实际订阅渲染会包含模板层，识别结果可能多于此处")
	}
	warnings = append(warnings,
		"预览按当前分组库统计，未执行订阅剪枝；实际产物分组数可能更少")

	rules, err := m.ListRulesByLayers(ctx, p.RuleLayers)
	if err != nil {
		return nil, nil, nil, nil, err
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
			renderRules = append(renderRules, raw)
			continue
		}
		line := strings.Join([]string{ru.RuleType, ru.RuleParam, policy}, ",")
		if ru.ResolveDNS {
			line += ",no-resolve"
		}
		renderRules = append(renderRules, line)
	}

	var metas []subcfg.RuleSourceMeta
	if len(p.RuleSources) > 0 {
		sourceRules, sourceMetas, sourceWarns := m.RuleSourceRulesWithMeta(ctx, p.RuleSources)
		warnings = append(warnings, sourceWarns...)
		if len(sourceRules) > 0 {
			renderRules = append(renderRules, sourceRules...)
			warnings = append(warnings, fmt.Sprintf("已用规则源产物替换模板规则：注入 %d 条", len(sourceRules)))
		}
		metas = sourceMetas
	}
	return renderGroups, renderRules, metas, warnings, nil
}
