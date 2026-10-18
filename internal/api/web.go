// web.go — M2 Web UI 的页面路由、静态资源与 JSON API。
package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render/registry"
)

// serveAsset 从嵌入的 Vite 产物读取单个文件并直出；文件不存在时走 404。
func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(webFS, name)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", assetContentType(name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func assetContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// --- /api/* ---

func (s *Server) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	total, cached, urlTotal := s.historyBuf.Stats()
	logs := 0
	if s.logBuf != nil {
		logs = len(s.logBuf.All())
	}
	cacheHitRate := 0.0
	if urlTotal > 0 {
		cacheHitRate = float64(cached) / float64(urlTotal)
	}
	resp := map[string]any{
		"history": map[string]any{
			"total":        total,
			"cachedHits":   cached,
			"urlTotal":     urlTotal,
			"cacheHitRate": cacheHitRate,
		},
		"logs": logs,
	}
	// 最近一次转换的快照（节点数/分组数/规则数/缓存命中率）。
	recent := s.historyBuf.Recent(1)
	if len(recent) > 0 {
		e := recent[0]
		lastRate := 0.0
		if e.URLCount > 0 {
			lastRate = float64(e.Cached) / float64(e.URLCount)
		}
		resp["last"] = map[string]any{
			"nodes":        e.Nodes,
			"rawNodes":     e.RawNodes,
			"groups":       e.Groups,
			"rules":        e.Rules,
			"cached":       e.Cached,
			"urlCount":     e.URLCount,
			"cacheHitRate": lastRate,
		}
	} else {
		resp["last"] = nil
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAPIHistory(w http.ResponseWriter, r *http.Request) {
	rows := s.historyBuf.Recent(50)
	if rows == nil {
		rows = []HistoryEntry{}
	}
	s.writeJSON(w, http.StatusOK, rows)
}

// handleAPIPipelines 是 GET /api/pipelines 的入口。plan §4.1 要求合并 DB + 模板两条来源，
// 实现放在 pipelines.go（handleAPIPipelinesMerged）；这里保留函数名与路由表不动，
// 避免与既有 BUG-3 回归用例（web_ui_test.go:469）的直接调用点冲突。
// 模板加载失败仍按 502 写出，不吞错。
func (s *Server) handleAPIPipelines(w http.ResponseWriter, r *http.Request) {
	s.handleAPIPipelinesMerged(w, r)
}

func (s *Server) handleAPITargets(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, registry.Available())
}

func (s *Server) handleAPIRulesets(w http.ResponseWriter, r *http.Request) {
	p, err := s.engine.ResolveTemplates(r.Context(), r.URL.Query().Get("config"))
	if err != nil {
		s.fail(w, r, http.StatusBadGateway, err)
		return
	}
	m := p.Ruleset
	if m == nil {
		m = map[string]string{}
	}
	rules := p.Rules
	if rules == nil {
		rules = []string{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"rulesets": m, "rules": rules})
}

func (s *Server) handleAPILogs(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("since")), 10, 64)
	entries := s.logBuf.Since(since, 200)
	if entries == nil {
		entries = []LogEntry{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"logs": entries})
}

type apiConvertReq struct {
	Target   string   `json:"target"`
	URLs     []string `json:"urls"`
	Text     string   `json:"text"`
	Types    []string `json:"types"`
	Include  []string `json:"include"`
	Exclude  []string `json:"exclude"`
	Pipeline string   `json:"pipeline"`
	Config   string   `json:"config"`
	Sort     string   `json:"sort"`
	List     bool     `json:"list"`
	Override string   `json:"override"`
}

func (s *Server) handleAPIConvert(w http.ResponseWriter, r *http.Request) {
	var req apiConvertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}
	// 文本来源：引擎只吃 URL 与本地文件，把粘贴的文本落到临时文件再交给它。
	// 请求结束即删除，避免临时目录堆积。
	var tempFile string
	if strings.TrimSpace(req.Text) != "" {
		f, err := os.CreateTemp("", "subforge-text-*")
		if err == nil {
			if _, werr := f.WriteString(req.Text); werr == nil {
				f.Close()
				tempFile = f.Name()
			} else {
				f.Close()
				_ = os.Remove(f.Name())
			}
		}
	}
	if tempFile != "" {
		defer os.Remove(tempFile)
	}
	m := map[string][]string{}
	if req.Target != "" {
		m["target"] = []string{req.Target}
	}
	if len(req.URLs) > 0 {
		m["url"] = append([]string{}, req.URLs...)
	}
	if len(req.Types) > 0 {
		m["type"] = append([]string{}, req.Types...)
	}
	if len(req.Include) > 0 {
		m["include"] = append([]string{}, req.Include...)
	}
	if len(req.Exclude) > 0 {
		m["exclude"] = append([]string{}, req.Exclude...)
	}
	if req.Pipeline != "" {
		m["pipeline"] = []string{req.Pipeline}
	}
	if req.Config != "" {
		m["config"] = []string{req.Config}
	}
	if req.Sort != "" {
		m["sort"] = []string{req.Sort}
	}
	if req.List {
		m["list"] = []string{"1"}
	}
	params, err := convert.ParseMulti(m)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err)
		return
	}
	if tempFile != "" {
		params.Files = append(params.Files, tempFile)
	}
	if params.Target == "" {
		params.Target = "clash.meta"
	}
	if strings.TrimSpace(req.Override) != "" {
		txt := strings.TrimSpace(req.Override)
		var ov config.Override
		if strings.HasPrefix(txt, "[") {
			ov, err = config.ParseOverrideINI([]byte(txt))
		} else {
			ov, err = config.ParseOverrideYAML([]byte(txt))
		}
		if err != nil {
			s.fail(w, r, http.StatusBadRequest, err)
			return
		}
		params.Override = ov
	}

	result, err := s.engine.Run(r.Context(), params)
	if err != nil {
		s.fail(w, r, statusFor(err), err)
		return
	}
	if len(result.Failures) > 0 {
		s.logger.Printf("rid=%s 部分来源失败(%d): %s",
			chimw.GetReqID(r.Context()), len(result.Failures),
			strings.Join(result.Failures, " | "))
	}
	// 分组数/规则数按引擎的覆盖链语义统计：模板（含远程规则集展开）优先，
	// 请求侧未指定时用模板值，再叠加 override 的合并结果。
	groups, rules := len(params.Groups), len(params.Rules)
	if tplParams, terr := s.engine.ResolveTemplates(r.Context(), req.Config); terr == nil {
		if len(params.Groups) == 0 {
			groups = len(tplParams.Groups)
		}
		if len(params.Rules) == 0 {
			rules = len(tplParams.Rules)
		}
		ov := params.Override
		if len(ov.Groups) > 0 {
			base := tplParams.Groups
			if len(params.Groups) > 0 {
				base = params.Groups
			}
			groups = len(config.MergeGroups(base, ov.ToGroups()))
		}
		if len(ov.Rules.Prepend) > 0 || len(ov.Rules.Append) > 0 {
			base := tplParams.Rules
			if len(params.Rules) > 0 {
				base = params.Rules
			}
			rules = len(config.MergeRules(base, ov.Rules.Prepend, ov.Rules.Append))
		}
	}
	if s.historyBuf != nil {
		s.historyBuf.Record(HistoryEntry{
			Target:   params.Target,
			Format:   result.Format,
			Nodes:    result.Nodes,
			RawNodes: result.RawNodes,
			Groups:   groups,
			Rules:    rules,
			Failures: result.Failures,
			Warnings: result.Warnings,
			Cached:   countCached(result),
			URLCount: len(params.URLs),
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"body":     string(result.Body),
		"format":   result.Format,
		"nodes":    result.Nodes,
		"rawNodes": result.RawNodes,
		"failures": result.Failures,
		"warnings": result.Warnings,
		"cached":   countCached(result),
	})
}

func (s *Server) handleAPIDiff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, r, http.StatusBadRequest, fmt.Errorf("解析请求体失败: %w", err))
		return
	}
	lines := DiffLines(req.A, req.B)
	if lines == nil {
		lines = []DiffLine{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
}

// HistoryEntry 使用的字段已在 history.go 定义，这里仅引用。
var _ = HistoryEntry{}
