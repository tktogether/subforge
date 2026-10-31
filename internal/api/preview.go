package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/fetch"
	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/parse"
	"github.com/tktogether/subforge/internal/transform"
)

// previewNode 是试运行/来源预览返回的节点摘要，不包含凭据。
type previewNode struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Server string `json:"server"`
	Port   string `json:"port"`
}

// previewDefaultNodes 是管线试运行的默认样例，覆盖常见地区关键词，便于验证过滤/改名步骤。
func previewDefaultNodes() []map[string]any {
	return []map[string]any{
		{"name": "香港 01", "type": "ss", "server": "hk.example.com", "port": 8388, "cipher": "aes-256-gcm", "password": "preview"},
		{"name": "日本 01", "type": "vmess", "server": "jp.example.com", "port": 443, "uuid": "00000000-0000-0000-0000-000000000001"},
		{"name": "美国 01", "type": "trojan", "server": "us.example.com", "port": 443, "password": "preview"},
	}
}

func summarizePreviewNodes(ns model.NodeList, limit int) []previewNode {
	if limit <= 0 || limit > len(ns) {
		limit = len(ns)
	}
	out := make([]previewNode, 0, limit)
	for _, n := range ns[:limit] {
		out = append(out, previewNode{Name: n.Name, Type: n.Type, Server: n.Server, Port: n.Port})
	}
	return out
}

func decodePreviewNodes(raw json.RawMessage) (model.NodeList, error) {
	if len(raw) == 0 || string(raw) == "null" {
		raw = previewJSON(previewDefaultNodes())
	}
	var nodes []map[string]any
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, fmt.Errorf("节点必须是 JSON 数组: %w", err)
	}
	if len(nodes) == 0 {
		return nil, errors.New("至少需要一个测试节点")
	}
	return model.NewNodeList(nodes, "preview"), nil
}

func previewJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func previewSteps(raw []apiPipelineStep) []config.PipelineStep {
	out := make([]config.PipelineStep, 0, len(raw))
	for _, st := range raw {
		out = append(out, config.PipelineStep{Kind: st.Op, Args: st.Args, OnError: st.OnError, Line: st.Line})
	}
	return out
}

func (s *Server) compilePreviewPlan(name string, steps []config.PipelineStep) (transform.Plan, error) {
	spec := config.PipelineSpec{Name: name, Steps: steps}
	return spec.CompileWithResolver(s.engine.ScriptResolver)
}

// previewPipelineSpec 返回模板 + 控制台管线的合并定义；控制台同名覆盖模板。
func (s *Server) previewPipelineSpec(ctx context.Context, name string) (config.PipelineSpec, error) {
	specs := map[string]config.PipelineSpec{}
	if s.engine != nil {
		params, err := s.engine.ResolveTemplates(ctx, "")
		if err != nil {
			return config.PipelineSpec{}, err
		}
		for n, spec := range params.Pipelines {
			specs[n] = spec
		}
	}
	ui, err := s.uiPipelineSpecs(ctx)
	if err != nil {
		return config.PipelineSpec{}, err
	}
	for n, spec := range ui {
		specs[n] = spec
	}
	spec, ok := specs[name]
	if !ok {
		return config.PipelineSpec{}, fmt.Errorf("未找到启用的管线 %q", name)
	}
	return spec, nil
}

// handleAPIPipelineTest 试运行一组管线步骤（可来自编辑中的草稿）。
func (s *Server) handleAPIPipelineTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	if s.engine == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errors.New("转换引擎未启用"))
		return
	}
	var req struct {
		Name  string            `json:"name"`
		Steps []apiPipelineStep `json:"steps"`
		Nodes json.RawMessage   `json:"nodes"`
	}
	if !s.decodeBody(w, r, &req) {
		return
	}
	list, err := decodePreviewNodes(req.Nodes)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	plan, err := s.compilePreviewPlan(req.Name, previewSteps(req.Steps))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	res, err := plan.Run(list)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":         req.Name,
		"before_count": list.Len(),
		"after_count":  res.Nodes.Len(),
		"before":       summarizePreviewNodes(list, 50),
		"after":        summarizePreviewNodes(res.Nodes, 50),
		"warnings":     res.Warnings,
	})
}

// handleAPIAirportPreviewProcess 预览某个机场按指定管线处理后的节点。
func (s *Server) handleAPIAirportPreviewProcess(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	if s.engine == nil || s.engine.Fetcher == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, errors.New("转换引擎未启用"))
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	a, err := m.GetAirport(r.Context(), id)
	if err != nil {
		s.writeAPIError(w, r, http.StatusNotFound, errors.New("机场不存在"))
		return
	}
	var req struct {
		Pipeline string `json:"pipeline"`
		Limit    int    `json:"limit"`
	}
	if !s.decodeBody(w, r, &req) {
		return
	}
	pipeline := strings.TrimSpace(req.Pipeline)
	if pipeline == "" {
		pipeline = strings.TrimSpace(a.Pipeline)
	}
	limit := clampPreviewLimit(req.Limit)

	fetched := s.engine.Fetcher.FetchFrom(r.Context(), fetch.Source{Name: a.Name, URL: a.SubscriptionURL})
	if fetched.Err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("拉取机场订阅失败: %w", fetched.Err))
		return
	}
	dec, err := parse.DecodeSource(fetched.Content)
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("解析机场订阅失败: %w", err))
		return
	}
	s.writeSourcePreview(w, r, a.Name, pipeline, model.NewNodeList(dec.Nodes, a.Name), limit)
}

// handleAPISelfNodesPreviewProcess 预览自建节点集合按指定管线处理后的节点。
func (s *Server) handleAPISelfNodesPreviewProcess(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	var req struct {
		Pipeline string `json:"pipeline"`
		Limit    int    `json:"limit"`
	}
	if !s.decodeBody(w, r, &req) {
		return
	}
	pipeline := strings.TrimSpace(req.Pipeline)
	if pipeline == "" {
		pipeline, _ = m.GetSelfSourcePipeline(r.Context())
	}
	limit := clampPreviewLimit(req.Limit)
	nodes, err := m.ListSelfNodes(r.Context())
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	links := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Enabled && strings.TrimSpace(n.Link) != "" {
			links = append(links, applySelfNodeName(n.Link, n.Name))
		}
	}
	if len(links) == 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("没有启用的自建节点"))
		return
	}
	dec, err := parse.DecodeSource([]byte(strings.Join(links, "\n")))
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("解析自建节点失败: %w", err))
		return
	}
	s.writeSourcePreview(w, r, "自建节点", pipeline, model.NewNodeList(dec.Nodes, "自建节点"), limit)
}

func clampPreviewLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (s *Server) writeSourcePreview(w http.ResponseWriter, r *http.Request, name, pipeline string, list model.NodeList, limit int) {
	before := list
	warnings := []string{}
	if pipeline != "" {
		spec, err := s.previewPipelineSpec(r.Context(), pipeline)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		plan, err := spec.CompileWithResolver(s.engine.ScriptResolver)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		res, err := plan.Run(list)
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		warnings = res.Warnings
		list = res.Nodes
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":         name,
		"pipeline":     pipeline,
		"before_count": before.Len(),
		"after_count":  list.Len(),
		"before":       summarizePreviewNodes(before, limit),
		"after":        summarizePreviewNodes(list, limit),
		"warnings":     warnings,
	})
}
