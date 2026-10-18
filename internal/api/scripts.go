package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/tktogether/subforge/internal/store"
)

// --- /api/scripts ---

func (s *Server) handleAPIScripts(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		scripts, err := m.DB().ListScripts(r.Context())
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		if scripts == nil {
			scripts = []store.Script{}
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"scripts": scripts})
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Code        string `json:"code"`
			Enabled     bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if req.Name == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("name 不能为空"))
			return
		}
		sc, err := m.DB().CreateScript(r.Context(), store.Script{
			Name:        req.Name,
			Description: req.Description,
			Code:        req.Code,
			Enabled:     req.Enabled,
		})
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"script": sc})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/POST"))
	}
}

func (s *Server) handleAPIScript(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	id, ok := s.pathID(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		sc, err := m.DB().GetScript(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, err)
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"script": sc})
	case http.MethodPut:
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Code        string `json:"code"`
			Enabled     bool   `json:"enabled"`
		}
		if !s.decodeBody(w, r, &req) {
			return
		}
		if req.Name == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("name 不能为空"))
			return
		}
		err := m.DB().UpdateScript(r.Context(), store.Script{
			ID:          id,
			Name:        req.Name,
			Description: req.Description,
			Code:        req.Code,
			Enabled:     req.Enabled,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, err)
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		sc, err := m.DB().GetScript(r.Context(), id)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"script": sc})
	case http.MethodDelete:
		if err := m.DB().DeleteScript(r.Context(), id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeAPIError(w, r, http.StatusNotFound, err)
				return
			}
			s.writeAPIError(w, r, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}
