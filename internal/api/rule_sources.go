package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tktogether/subforge/internal/mrs"
	"github.com/tktogether/subforge/internal/subforge"
)

// handleAPIRuleSources 处理 GET /api/rule-sources。
func (s *Server) handleAPIRuleSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	manifestPath, err := m.ResolveRuleManifestPath(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	defaultManifestPath := manifestPath
	if dp, derr := m.DefaultRuleManifestPath(r.Context()); derr == nil {
		defaultManifestPath = dp
	}
	sources, err := m.ListRuleSources(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if sources == nil {
		sources = []subforge.RuleSourceStatus{}
	}
	resp := map[string]any{
		"manifest_path":         manifestPath,
		"default_manifest_path": defaultManifestPath,
		"sources":               sources,
		"manifest_template":     subforge.RuleManifestTemplate(),
	}
	if outDir, err := m.ResolveRulesOutputDir(r.Context()); err == nil {
		resp["output_dir"] = outDir
	}
	if _, statErr := os.Stat(manifestPath); statErr != nil {
		resp["exists"] = false
		resp["hint"] = "未找到规则源清单文件，可点击「生成默认清单」在本机创建一份模板，或在上方设置里改路径。"
	} else {
		resp["exists"] = true
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleAPIRuleSourcesInit 处理 POST /api/rule-sources/init：不存在时写入默认清单模板。
// 可选 ?force=1 覆盖已有文件（覆盖前备份为 <path>.bak）。
// → 200 {"path":"...","created":bool}；非 POST → 405「仅支持 POST」。
func (s *Server) handleAPIRuleSourcesInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	force := isTruthyQuery(r.URL.Query().Get("force"))
	path, created, err := m.EnsureRuleManifest(r.Context(), force)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"path": path, "created": created})
}

// handleAPIRuleSourcesUpdate 处理 POST /api/rule-sources/update。
func (s *Server) handleAPIRuleSourcesUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	results, err := m.UpdateRuleSources(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(results))
	for _, res := range results {
		errStr := ""
		if res.Err != nil {
			errStr = res.Err.Error()
		}
		out = append(out, map[string]any{
			"source":     res.Source,
			"version":    res.Version,
			"path":       res.Path,
			"from_cache": res.FromCache,
			"err":        errStr,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// handleAPIRuleSourcesRollback 处理 POST /api/rule-sources/rollback。
func (s *Server) handleAPIRuleSourcesRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("name 不能为空"))
		return
	}
	path, err := m.RollbackRuleSource(r.Context(), strings.TrimSpace(req.Name))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

// handleAPIRuleSourcesMRS 处理 POST /api/rule-sources/mrs。
func (s *Server) handleAPIRuleSourcesMRS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Behavior string `json:"behavior"`
		Format   string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("name 不能为空"))
		return
	}
	sources, err := m.ListRuleSources(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	var src *subforge.RuleSourceStatus
	for i := range sources {
		if sources[i].Name == strings.TrimSpace(req.Name) {
			src = &sources[i]
			break
		}
	}
	if src == nil {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则源 %s 不存在", req.Name))
		return
	}
	if len(src.Versions) == 0 || strings.TrimSpace(src.Latest) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("规则源 %s 暂无版本文件", req.Name))
		return
	}
	behavior := strings.TrimSpace(req.Behavior)
	if behavior == "" {
		behavior = "domain"
	}
	format := strings.TrimSpace(req.Format)
	if format == "" {
		format = "mrs"
	}
	srcPath := filepath.Join(src.Dir, src.Latest)
	ext := filepath.Ext(src.Latest)
	dstPath := filepath.Join(src.Dir, strings.TrimSuffix(src.Latest, ext)+".mrs")
	if err := mrs.GenerateFile(srcPath, dstPath, behavior, format); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	info, err := os.Stat(dstPath)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"path": dstPath, "bytes": info.Size()})
}
