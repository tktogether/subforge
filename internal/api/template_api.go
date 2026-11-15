package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/subforge"
	"github.com/tktogether/subforge/internal/template"
)

const templatePathKey = "template.path"

// handleAPITemplate 处理 GET/PUT /api/template。
func (s *Server) handleAPITemplate(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		path := s.resolveTemplatePath(r, m)
		if path == "" {
			s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("模板路径未配置"))
			return
		}
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				s.writeAPIError(w, r, http.StatusNotFound, err)
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"path": path, "content": string(b)})
	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		path := s.resolveTemplatePath(r, m)
		if path == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("模板路径未配置"))
			return
		}
		if _, err := template.ParseString(req.Content, path); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("模板语法错误: %w", err))
			return
		}
		if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		// 模板落盘是文件操作，store 的 bump 够不着：显式失效订阅产物缓存。
		// bump 失败只记日志不阻断——保存本身已成功，漏一次 bump 由惰性重建自愈。
		if err := m.BumpArtifactVersion(r.Context()); err != nil {
			s.logger.Printf("[api] 保存模板后 bump 订阅产物版本号失败: %v", err)
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET/PUT"))
	}
}

func (s *Server) resolveTemplatePath(r *http.Request, m *subforge.Manager) string {
	if m != nil {
		if v, err := m.GetConfig(r.Context(), templatePathKey); err == nil && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if s.engine != nil && s.engine.BaseTemplate != nil {
		if tpl := s.engine.BaseTemplate(); tpl != nil {
			if o := strings.TrimSpace(tpl.Origin()); o != "" {
				return o
			}
		}
	}
	_ = config.Template{}
	return ""
}
