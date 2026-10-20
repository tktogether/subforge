package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dop251/goja"

	"github.com/tktogether/subforge/internal/store"
)

// --- /api/scripts ---

// checkScriptKind 校验脚本 kind：只有 filter / operate 两种合法取值。
// 错误文案与 /api/scripts/test 端点保持一致（internal/api/scripts_test_api.go）。
func checkScriptKind(kind string) error {
	if kind != "filter" && kind != "operate" {
		return errors.New("未知的脚本类型: " + kind)
	}
	return nil
}

// scriptCompileName 是保存校验时传给 goja 的脚本名。
// 与 internal/transform/script.go 执行时保持同一套包装方式（goja.Compile，不注入宿主对象），
// 避免出现「保存时通过、执行时编译失败」或反过来的误报。
const scriptCompileName = "script"

// validateScriptCode 在脚本入库前编译一次，把语法错误拦在保存这一步。
// 空代码与语法错误都返回可读文案（带 goja 给出的行号），由调用方映射成 400。
func validateScriptCode(code string) error {
	if strings.TrimSpace(code) == "" {
		return errors.New("脚本代码语法错误：脚本代码不能为空")
	}
	if _, err := goja.Compile(scriptCompileName, code, false); err != nil {
		return fmt.Errorf("脚本代码语法错误：%v", err)
	}
	return nil
}

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
			Kind        string `json:"kind"`
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
		// kind 空值默认 filter（与 /api/scripts/test 的语义一致），非法值 400。
		kind := req.Kind
		if kind == "" {
			kind = "filter"
		}
		if err := checkScriptKind(kind); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		// 入库前先编译一次：语法错误的脚本不允许保存。
		if err := validateScriptCode(req.Code); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		sc, err := m.DB().CreateScript(r.Context(), store.Script{
			Name:        req.Name,
			Description: req.Description,
			Kind:        kind,
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
			Kind        string `json:"kind"`
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
		// kind 空值保留库里的原值：老脚本（kind 为空串）不会因 PUT 被静默改成 filter。
		kind := req.Kind
		if kind == "" {
			old, err := m.DB().GetScript(r.Context(), id)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					s.writeAPIError(w, r, http.StatusNotFound, err)
					return
				}
				s.writeAPIError(w, r, http.StatusInternalServerError, err)
				return
			}
			kind = old.Kind
		} else if err := checkScriptKind(kind); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		// 与 POST 同一套校验：语法错误不写库，库里保留旧代码。
		if err := validateScriptCode(req.Code); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		err := m.DB().UpdateScript(r.Context(), store.Script{
			ID:          id,
			Name:        req.Name,
			Description: req.Description,
			Kind:        kind,
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
