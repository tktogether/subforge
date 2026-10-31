package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// handleAPIRuleSourceRules 处理 GET /api/rule-sources/rules：
// 读取某个规则源当前版本的规则，供规则页「从规则源挑选」预览/搜索/分页。
func (s *Server) handleAPIRuleSourceRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("name 不能为空"))
		return
	}
	sources, err := m.ListRuleSources(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	found := false
	for _, src := range sources {
		if src.Name == name {
			found = true
			break
		}
	}
	if !found {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则源 %q 不存在", name))
		return
	}
	rules, warnings := m.RuleSourceRules(r.Context(), []string{name})
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if query != "" {
		filtered := make([]string, 0, len(rules))
		for _, line := range rules {
			if strings.Contains(strings.ToLower(line), query) {
				filtered = append(filtered, line)
			}
		}
		rules = filtered
	}
	offset := 0
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		v, perr := strconv.Atoi(raw)
		if perr != nil || v < 0 {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("offset 需要非负整数"))
			return
		}
		offset = v
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, perr := strconv.Atoi(raw)
		if perr != nil || v <= 0 || v > 200 {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("limit 需要 1-200 的整数"))
			return
		}
		limit = v
	}
	total := len(rules)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":     name,
		"total":    total,
		"offset":   offset,
		"limit":    limit,
		"rules":    rules[offset:end],
		"warnings": warnings,
	})
}

// handleAPIRuleSourceImport 处理 POST /api/rule-sources/import-rules：
// 把用户从规则源挑选的规则复制到自定义层；重复规则默认跳过，conflict=overwrite 时覆盖。
func (s *Server) handleAPIRuleSourceImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name     string   `json:"name"`
		Rules    []string `json:"rules"`
		Conflict string   `json:"conflict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("name 不能为空"))
		return
	}
	if len(req.Rules) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("rules 不能为空"))
		return
	}
	if len(req.Rules) > 20000 {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("一次最多导入 20000 条规则"))
		return
	}
	sources, err := m.ListRuleSources(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	found := false
	for _, src := range sources {
		if src.Name == req.Name {
			found = true
			break
		}
	}
	if !found {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则源 %q 不存在", req.Name))
		return
	}
	res, err := m.ImportRulesToCustom(r.Context(), req.Name, req.Rules, strings.EqualFold(strings.TrimSpace(req.Conflict), "overwrite"))
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"added":     res.Added,
		"skipped":   res.Skipped,
		"overwrote": res.Overwrote,
		"warnings":  res.Warnings,
	})
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
	sources, err := m.ListRuleSources(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	remoteResults, remoteErr := m.UpdateRemoteRuleSources(r.Context())
	byName := make(map[string]manifest.UpdateResult, len(remoteResults))
	for _, res := range remoteResults {
		byName[res.Source] = res
	}
	results := make([]manifest.UpdateResult, 0, len(sources))
	allFailed := len(sources) > 0
	for _, src := range sources {
		var res manifest.UpdateResult
		if src.Origin == "airport" {
			res = s.updateAirportRuleSource(r.Context(), m, src.Name)
		} else if got, found := byName[src.Name]; found {
			res = got
		} else {
			res = manifest.UpdateResult{Source: src.Name, Err: fmt.Errorf("远程源未更新")}
		}
		if res.Err == nil {
			allFailed = false
		}
		results = append(results, res)
	}
	if allFailed {
		msg := remoteErr
		if msg == nil {
			msg = fmt.Errorf("所有 %d 个 source 更新失败", len(results))
		}
		s.writeRuleSourceFailure(w, msg, ruleSourceResults(results))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"results": ruleSourceResults(results)})
}

// updateAirportRuleSource 从机场订阅重新提取规则并写规则源，避免把完整机场
// YAML（含节点配置）写进规则产物。规则源页的“更新/全部更新”也走这里。
func (s *Server) updateAirportRuleSource(ctx context.Context, m *subforge.Manager, name string) manifest.UpdateResult {
	res := manifest.UpdateResult{Source: name}
	id, ok := subforge.AirportIDFromRuleSourceName(name)
	if !ok {
		res.Err = fmt.Errorf("机场规则源名称不合法")
		return res
	}
	a, err := m.GetAirport(ctx, id)
	if err != nil {
		res.Err = fmt.Errorf("机场不存在: %w", err)
		return res
	}
	ir, _, err := s.importRulesToLibrary(ctx, m, a, subforge.ImportOptions{CreateGroups: true, EnableRules: true})
	if err != nil {
		res.Err = err
		return res
	}
	res.Version = ir.RuleSourceVersion
	if sources, serr := m.ListRuleSources(ctx); serr == nil {
		for _, src := range sources {
			if src.Name == name && src.Latest != "" {
				res.Path = filepath.Join(src.Dir, src.Latest)
			}
		}
	}
	return res
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
		if strings.Contains(err.Error(), "机场规则源") {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		if strings.Contains(err.Error(), "正被方案") {
			s.writeAPIError(w, r, http.StatusConflict, err)
			return
		}
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
	name := strings.TrimSpace(req.Name)
	if strings.HasPrefix(name, subforge.AirportRuleSourcePrefix) {
		res := s.updateAirportRuleSource(r.Context(), m, name)
		if res.Err != nil {
			s.writeRuleSourceFailure(w, res.Err, ruleSourceResults([]manifest.UpdateResult{res}))
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"results": ruleSourceResults([]manifest.UpdateResult{res})})
		return
	}
	results, err := m.UpdateRuleSourceOne(r.Context(), name)
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

// handleAPIRuleSourceConflicts 处理 GET /api/rule-sources/conflicts?names=a,b：
// 轻量比较多个源的当前规则，按 type+param 找出跨源重复，供方案绑定界面提示。
// 不返回全量规则，只返回总数与前几个示例。
func (s *Server) handleAPIRuleSourceConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	raw := strings.Split(r.URL.Query().Get("names"), ",")
	names := make([]string, 0, len(raw))
	for _, n := range raw {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	type sample struct {
		Type   string `json:"type"`
		Param  string `json:"param"`
		First  string `json:"first"`
		Second string `json:"second"`
	}
	total := 0
	samples := []sample{}
	if len(names) >= 2 {
		seen := make(map[string]string)
		for _, name := range names {
			rules, _ := m.RuleSourceRules(r.Context(), []string{name})
			for _, line := range rules {
				parts := strings.SplitN(line, ",", 3)
				if len(parts) < 2 {
					continue
				}
				typ := strings.ToUpper(strings.TrimSpace(parts[0]))
				param := strings.TrimSpace(parts[1])
				if typ == "" || param == "" {
					continue
				}
				key := typ + "," + param
				first, found := seen[key]
				if !found {
					seen[key] = name
					continue
				}
				if first == name {
					continue
				}
				total++
				if len(samples) < 5 {
					samples = append(samples, sample{Type: typ, Param: param, First: first, Second: name})
				}
			}
		}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"total": total, "samples": samples})
}

// handleAPIRuleSourceRename 处理 POST /api/rule-sources/rename。
func (s *Server) handleAPIRuleSourceRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name    string `json:"name"`
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("请求参数格式不正确: %w", err))
		return
	}
	path, err := m.RenameRuleSource(r.Context(), req.Name, req.NewName)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "manifest_path": path, "name": strings.TrimSpace(req.NewName)})
}
