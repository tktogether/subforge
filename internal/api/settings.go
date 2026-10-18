package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tktogether/subforge/internal/subforge"
)

// handleAPISettings 处理 GET/PUT /api/settings。
func (s *Server) handleAPISettings(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := m.ListSettings(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if items == nil {
			items = []subforge.SettingItem{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"settings": items})
	case http.MethodPut:
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		kv := map[string]string{}
		if v, ok := raw["settings"]; ok && len(v) > 0 && string(v) != "null" {
			var sub map[string]string
			if err := json.Unmarshal(v, &sub); err != nil {
				s.writeAPIError(w, r, http.StatusBadRequest, err)
				return
			}
			kv = sub
		} else {
			for k, v := range raw {
				var str string
				if err := json.Unmarshal(v, &str); err != nil {
					s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("设置项 %q 的值必须是字符串", k))
					return
				}
				kv[k] = str
			}
		}
		if err := m.SetSettings(r.Context(), kv); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		items, err := m.ListSettings(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if items == nil {
			items = []subforge.SettingItem{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"settings": items})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET/PUT"))
	}
}
