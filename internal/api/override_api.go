package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/subforge"
)

const overridePathKey = "override.path"

// handleAPIOverride 处理 GET/PUT /api/override。
func (s *Server) handleAPIOverride(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		path := s.resolveOverridePath(r, m)
		if path == "" {
			s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("override 路径未配置"))
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
		path := s.resolveOverridePath(r, m)
		if path == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("override 路径未配置"))
			return
		}
		var err error
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ini":
			_, err = config.ParseOverrideINI([]byte(req.Content))
		default:
			_, err = config.ParseOverrideYAML([]byte(req.Content))
		}
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("override 语法错误: %w", err))
			return
		}
		if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET/PUT"))
	}
}

func (s *Server) resolveOverridePath(r *http.Request, m *subforge.Manager) string {
	if m != nil {
		if v, err := m.GetConfig(r.Context(), overridePathKey); err == nil && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
