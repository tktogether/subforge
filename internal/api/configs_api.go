package api

import (
	"fmt"
	"net/http"
	"strings"
)

// handleAPIConfigs 处理 GET /api/configs：返回令牌/转换可用的 config 候选。
// 数据源：当前生效模板 [common] 段的 config= 值（模板作者推荐的远程模板）。
func (s *Server) handleAPIConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	// 只判引擎缺失：nil 模板是合法空状态（convert/engine.go:304 对 BaseTemplate==nil 返回空 Params 且不报错，
	// engine.go:40-42 也写明模板可以为 nil）。此时没有候选，返回空列表比 503 更诚实。
	if s.engine == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, fmt.Errorf("转换引擎未启用"))
		return
	}
	// BaseTemplate 是函数字段：未注入时（CLI 未指定 -config）本身为 nil，直接调用会 panic，所以先判字段再调用。
	if s.engine.BaseTemplate == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"configs": []string{}, "default": ""})
		return
	}
	tpl := s.engine.BaseTemplate()
	if tpl == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"configs": []string{}, "default": ""})
		return
	}
	sem, err := tpl.Semantics()
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("解释模板失败: %w", err))
		return
	}
	seen := map[string]bool{}
	configs := make([]string, 0, 4)
	for _, v := range sem.Params["config"] {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		configs = append(configs, v)
	}
	def := ""
	if len(configs) > 0 {
		def = configs[0]
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"configs": configs, "default": def})
}
