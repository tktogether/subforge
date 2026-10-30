package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tktogether/subforge/internal/manifest"
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
		resp["hint"] = "未找到规则源清单文件。首次新增规则源时会自动创建；也可以在上方设置里改路径。"
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

// ruleSourceResults 把 manifest 的逐源结果转成响应体里的一份。
func ruleSourceResults(results []manifest.UpdateResult) []map[string]any {
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
	return out
}

// writeRuleSourceFailure 回 500，但把逐源结果一起带上。
//
// 全部源都失败时 manifest 返回的 error 只有一句「所有 N 个 source 更新失败」，
// 真正有用的信息（哪个源、哪一步、上游说了什么）全在 results 里。只回 error
// 的话，界面上只剩一句笼统的失败，用户无从下手。
func (s *Server) writeRuleSourceFailure(w http.ResponseWriter, err error, out []map[string]any) {
	s.writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error":   err.Error(),
		"results": out,
	})
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
	out := ruleSourceResults(results)
	if err != nil {
		s.writeRuleSourceFailure(w, err, out)
		return
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
		// 只有 1 版（或已指向首个版本）是可解释的用户错误，而不是服务端故障。
		if errors.Is(err, subforge.ErrRollbackInsufficient) {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"path": path})
}

// ruleSourceMRSFormat 推导 mrs.GenerateFile 需要的「源数据格式」。
//
// mrs 的 format 语义是源文件本身的格式（yaml/text/mrs），不是输出格式：把
// "mrs" 传给一个 .yaml 源，内核会按二进制去解析它，报 magic number mismatch。
// 请求显式给了 format 就以请求为准，但同样按这张表校验，避免脏值进内核。
func ruleSourceMRSFormat(ext, explicit string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(explicit))
	if format == "" {
		switch strings.ToLower(strings.TrimSpace(ext)) {
		case ".yaml", ".yml":
			format = "yaml"
		case ".txt", ".text":
			format = "text"
		case ".mrs":
			format = "mrs"
		default:
			return "", fmt.Errorf("无法从版本文件扩展名 %q 推导源格式，请显式指定 format（yaml/text/mrs）", ext)
		}
	}
	switch format {
	case "yaml", "yml":
		return "yaml", nil
	case "text", "txt":
		return "text", nil
	case "mrs":
		return "mrs", nil
	default:
		return "", fmt.Errorf("不支持的 format %q（仅支持 yaml/text/mrs）", explicit)
	}
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
	format, err := ruleSourceMRSFormat(filepath.Ext(src.Latest), req.Format)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
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

// handleAPIRuleSourcesAdd 处理 POST /api/rule-sources（新增规则源）。
func (s *Server) handleAPIRuleSourcesAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		InputFormat string `json:"input_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("url 不能为空"))
		return
	}
	path, created, err := m.AddRuleSource(r.Context(), manifest.Source{Name: req.Name, URL: req.URL, InputFormat: req.InputFormat})
	if err != nil {
		if strings.Contains(err.Error(), "已存在") {
			s.writeAPIError(w, r, http.StatusConflict, err)
			return
		}
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"path": path, "created": created})
}

// handleAPIRuleSourcesRemove 处理 POST /api/rule-sources/remove（删除规则源）。
func (s *Server) handleAPIRuleSourcesRemove(w http.ResponseWriter, r *http.Request) {
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
	path, err := m.RemoveRuleSource(r.Context(), strings.TrimSpace(req.Name))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		if strings.Contains(err.Error(), "至少保留") {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}

// handleAPIRuleSourcesUpdateOne 处理 POST /api/rule-sources/update-one（单源更新）。
func (s *Server) handleAPIRuleSourcesUpdateOne(w http.ResponseWriter, r *http.Request) {
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
	results, err := m.UpdateRuleSourceOne(r.Context(), strings.TrimSpace(req.Name))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeRuleSourceFailure(w, err, ruleSourceResults(results))
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
