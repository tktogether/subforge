package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
)

func TestTools(t *testing.T) {
	s := NewServer(convert.Engine{})
	tools := s.Tools()
	if len(tools) < 2 {
		t.Fatalf("至少应有 2 个工具，实际 %d", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
		t.Logf("tool: %s - %s", tool.Name, tool.Description)
	}
	if !names["convert"] {
		t.Error("缺少 convert 工具")
	}
	if !names["list_targets"] {
		t.Error("缺少 list_targets 工具")
	}
}

func TestCallTool_ListTargets(t *testing.T) {
	s := NewServer(convert.Engine{})
	result, err := s.CallTool(context.Background(), "list_targets", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("返回内容为空")
	}
	text := result.Content[0].Text
	for _, target := range []string{"clash.meta", "sing-box", "surge"} {
		if !strings.Contains(text, target) {
			t.Errorf("缺少目标 %s", target)
		}
	}
}

func TestCallTool_Unknown(t *testing.T) {
	s := NewServer(convert.Engine{})
	_, err := s.CallTool(context.Background(), "unknown", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("未知工具应报错")
	}
}

func TestHandleJSONRPC_Initialize(t *testing.T) {
	s := NewServer(convert.Engine{})
	var buf bytes.Buffer
	req := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	s.HandleJSONRPC(context.Background(), &buf, req)

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc 版本错误: %v", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatal("缺少 result")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("协议版本错误: %v", result["protocolVersion"])
	}
}

func TestHandleJSONRPC_ToolsList(t *testing.T) {
	s := NewServer(convert.Engine{})
	var buf bytes.Buffer
	req := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	s.HandleJSONRPC(context.Background(), &buf, req)

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatal("缺少 result")
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatal("工具列表为空")
	}
	t.Logf("工具数: %d", len(tools))
}

func TestHandleJSONRPC_UnknownMethod(t *testing.T) {
	s := NewServer(convert.Engine{})
	var buf bytes.Buffer
	req := json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"unknown","params":{}}`)
	s.HandleJSONRPC(context.Background(), &buf, req)

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("缺少 error")
	}
	code, _ := errObj["code"].(float64)
	if code != -32601 {
		t.Errorf("错误码应为 -32601，实际 %v", code)
	}
}
