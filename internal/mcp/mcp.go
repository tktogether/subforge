// Package mcp 实现 MCP (Model Context Protocol) 服务器，
// 允许 AI agent 通过标准协议调用 SubForge 的转换引擎。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tktogether/subforge/internal/convert"
)

// Server 是 MCP 服务器。
type Server struct {
	engine convert.Engine
}

// NewServer 创建 MCP 服务器。
func NewServer(engine convert.Engine) *Server {
	return &Server{engine: engine}
}

// Tools 返回 MCP 工具列表。
func (s *Server) Tools() []Tool {
	return []Tool{
		{
			Name:        "convert",
			Description: "将订阅转换为目标客户端配置",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"urls": {"type": "array", "items": {"type": "string"}, "description": "订阅 URL 列表"},
					"target": {"type": "string", "description": "目标格式 (clash.meta, sing-box, surge, mellow, ...)"},
					"include": {"type": "array", "items": {"type": "string"}, "description": "包含过滤（可选）"},
					"exclude": {"type": "array", "items": {"type": "string"}, "description": "排除过滤（可选）"},
					"sort": {"type": "string", "description": "排序键（可选）"}
				},
				"required": ["urls", "target"]
			}`),
		},
		{
			Name:        "list_targets",
			Description: "列出所有支持的目标格式",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}
}

// Tool 是 MCP 工具定义。
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// CallTool 调用 MCP 工具。
func (s *Server) CallTool(ctx context.Context, name string, args json.RawMessage) (*ToolResult, error) {
	switch name {
	case "convert":
		return s.callConvert(ctx, args)
	case "list_targets":
		return s.callListTargets()
	default:
		return nil, fmt.Errorf("未知工具: %s", name)
	}
}

func (s *Server) callConvert(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
	var params struct {
		URLs    []string `json:"urls"`
		Target  string   `json:"target"`
		Include []string `json:"include"`
		Exclude []string `json:"exclude"`
		Sort    string   `json:"sort"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("解析参数: %w", err)
	}

	result, err := s.engine.Run(ctx, convert.Params{
		URLs:    params.URLs,
		Target:  params.Target,
		Include: params.Include,
		Exclude: params.Exclude,
		Sort:    params.Sort,
	})
	if err != nil {
		return &ToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("转换失败: %v", err)}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []Content{{Type: "text", Text: string(result.Body)}},
	}, nil
}

func (s *Server) callListTargets() (*ToolResult, error) {
	targets := []string{
		"clash.meta", "sing-box", "surge", "mellow", "ss-sub", "sharelink",
	}
	return &ToolResult{
		Content: []Content{{Type: "text", Text: strings.Join(targets, "\n")}},
	}, nil
}

// ToolResult 是工具调用结果。
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

// Content 是工具结果内容。
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// HandleJSONRPC 处理 JSON-RPC 请求（MCP 传输层）。
func (s *Server) HandleJSONRPC(ctx context.Context, w io.Writer, req json.RawMessage) {
	var rpcReq struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(req, &rpcReq); err != nil {
		s.writeError(w, nil, -32700, "解析错误")
		return
	}

	switch rpcReq.Method {
	case "initialize":
		s.writeResult(w, rpcReq.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "subforge",
				"version": "dev",
			},
		})
	case "tools/list":
		s.writeResult(w, rpcReq.ID, map[string]any{
			"tools": s.Tools(),
		})
	case "tools/call":
		var params struct {
			Name string          `json:"name"`
			Args json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(rpcReq.Params, &params); err != nil {
			s.writeError(w, rpcReq.ID, -32602, "无效参数")
			return
		}
		result, err := s.CallTool(ctx, params.Name, params.Args)
		if err != nil {
			s.writeError(w, rpcReq.ID, -32603, err.Error())
			return
		}
		s.writeResult(w, rpcReq.ID, result)
	default:
		s.writeError(w, rpcReq.ID, -32601, "方法不存在")
	}
}

func (s *Server) writeResult(w io.Writer, id json.RawMessage, result any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeError(w io.Writer, id json.RawMessage, code int, msg string) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	}
	json.NewEncoder(w).Encode(resp)
}
