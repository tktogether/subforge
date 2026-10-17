// web.go — M2 Web UI 的页面路由、静态资源与 JSON API。
package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"

	chi "github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/render/registry"
)

// handleDashboard 直出嵌入的 Dashboard 首页。
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.serveAsset(w, r, "index.html")
}

// handlePage 直出指定的 HTML 页面。
func (s *Server) handlePage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.serveAsset(w, r, name)
	}
}

// handleAsset 直出样式/脚本等单个静态资源。
func (s *Server) handleAsset(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.serveAsset(w, r, name)
	}
}

// handleJSAsset 服务 /js/* 下的页面脚本。
func (s *Server) handleJSAsset(w http.ResponseWriter, r *http.Request) {
	star := chi.URLParam(r, "*")
	if strings.Contains(star, "..") || star == "" {
		s.handleNotFound(w, r)
		return
	}
	s.serveAsset(w, r, "js/"+star)
}

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
	s.writeJSON(w, http.StatusOK, map[string]any{
		"history": map[string]any{
			"total":      total,
			"cachedHits": cached,
			"urlTotal":   urlTotal,
		},
		"logs": logs,
	})
}

func (s *Server) handleAPIHistory(w http.ResponseWriter, r *http.Request) {
	rows := s.historyBuf.Recent(50)
	if rows == nil {
		rows = []HistoryEntry{}
	}
	s.writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleAPIPipelines(w http.ResponseWriter, r *http.Request) {
	p, _ := s.engine.ResolveTemplates(r.Context(), r.URL.Query().Get("config"))
	names := make([]string, 0, len(p.Pipelines))
	for name := range p.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	s.writeJSON(w, http.StatusOK, names)
}

func (s *Server) handleAPITargets(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, registry.Available())
}

func (s *Server) handleAPIRulesets(w http.ResponseWriter, r *http.Request) {
	p, _ := s.engine.ResolveTemplates(r.Context(), r.URL.Query().Get("config"))
	m := p.Ruleset
	if m == nil {
		m = map[string]string{}
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"rulesets": m})
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
	if len(params.URLs) == 0 && len(req.URLs) > 0 {
		params.URLs = append([]string{}, req.URLs...)
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
	if s.historyBuf != nil {
		s.historyBuf.Record(HistoryEntry{
			Target:   params.Target,
			Format:   result.Format,
			Nodes:    result.Nodes,
			RawNodes: result.RawNodes,
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
