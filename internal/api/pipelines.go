package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/store"
)

// --- /api/pipelines（plan §4.1：DB 定义优先于模板，模板来源只读）---

// apiPipeline 是 GET /api/pipelines 数组的一个元素。
// 内嵌 store.Pipeline 让 id/name/description/steps_json/enabled/sort_order/source 的 JSON
// 口径与 store 完全一致，避免两处漂移；ReadOnly 只对模板来源为 true。
type apiPipeline struct {
	store.Pipeline
	ReadOnly bool `json:"read_only"`
}

// apiPipelineStep 是 steps_json 里的一步：op 直接复用 convert 的管线动作名
// （config.PipelineStep.Kind，共 10 种），不新造动作（plan §4.1）。
type apiPipelineStep struct {
	Op      string            `json:"op"`
	Args    map[string]string `json:"args,omitempty"`
	OnError string            `json:"on_error,omitempty"`
	Line    int               `json:"line,omitempty"`
}

// mergedPipelines 合并两条来源：DB 行（source=ui）在前，模板行（source=template，只读）在后；
// 与 DB 同名的模板条目被 DB 覆盖 —— 这就是 plan §4.1 的「DB 定义优先于模板」。
// 未注入 subManager 时只返回模板来源（不报错），Convert/Pipelines 页在纯 CLI 部署下仍可用。
func (s *Server) mergedPipelines(ctx context.Context, configRef string) ([]apiPipeline, error) {
	params, err := s.engine.ResolveTemplates(ctx, configRef)
	if err != nil {
		return nil, err
	}
	out := []apiPipeline{}
	taken := make(map[string]bool)
	if s.subManager != nil {
		rows, err := s.subManager.ListPipelines(ctx)
		if err != nil {
			return nil, err
		}
		for _, p := range rows {
			out = append(out, apiPipeline{Pipeline: p})
			taken[p.Name] = true
		}
	}
	names := make([]string, 0, len(params.Pipelines))
	for name := range params.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if taken[name] {
			continue
		}
		spec := params.Pipelines[name]
		steps := make([]apiPipelineStep, 0, len(spec.Steps))
		for _, st := range spec.Steps {
			steps = append(steps, apiPipelineStep{Op: st.Kind, Args: st.Args, OnError: st.OnError, Line: st.Line})
		}
		raw, err := json.Marshal(steps)
		if err != nil {
			return nil, err
		}
		out = append(out, apiPipeline{
			Pipeline: store.Pipeline{
				Name:      name,
				StepsJSON: string(raw),
				Enabled:   true,
				Source:    "template",
			},
			ReadOnly: true,
		})
	}
	return out, nil
}

// uiPipelineSpecs 把控制台维护的管线（source=ui）翻译成引擎认的命名管线定义。
//
// steps_json 里的 op 就是 config.PipelineStep.Kind（apiPipelineStep 的注释已约定
// 两者同名），所以这里只做反序列化，不重新解释语义：编译与报错交给
// PipelineSpec.Compile，保证界面建的管线和 INI 里写的管线走同一段校验。
//
// 停用的行直接跳过。界面上「停用」是「别拿它跑转换」的意思，让它在引擎里继续
// 生效会让开关形同虚设。
func (s *Server) uiPipelineSpecs(ctx context.Context) (map[string]config.PipelineSpec, error) {
	if s.subManager == nil {
		return nil, nil
	}
	rows, err := s.subManager.ListPipelines(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]config.PipelineSpec, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		var steps []apiPipelineStep
		if raw := strings.TrimSpace(row.StepsJSON); raw != "" {
			if err := json.Unmarshal([]byte(raw), &steps); err != nil {
				return nil, fmt.Errorf("控制台管线 %q 的步骤不是合法 JSON: %w", row.Name, err)
			}
		}
		spec := config.PipelineSpec{Name: row.Name, Steps: make([]config.PipelineStep, 0, len(steps))}
		for _, st := range steps {
			spec.Steps = append(spec.Steps, config.PipelineStep{
				Kind: st.Op, Args: st.Args, OnError: st.OnError, Line: st.Line,
			})
		}
		out[row.Name] = spec
	}
	return out, nil
}

// handleAPIPipelinesMerged 是 GET /api/pipelines 的实现；web.go 的 handleAPIPipelines 委托到这里
// （函数名与路由表保持不动，避免与既有 BUG-3 回归用例的调用点冲突）。
func (s *Server) handleAPIPipelinesMerged(w http.ResponseWriter, r *http.Request) {
	out, err := s.mergedPipelines(r.Context(), r.URL.Query().Get("config"))
	if err != nil {
		// 模板加载失败必须报出来（BUG-3）：502，而不是伪装成空模板。
		s.fail(w, r, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, out)
}

// apiPipelineReq 是 POST/PUT 的请求体。steps 传数组（推荐）或 steps_json 传原样字符串。
type apiPipelineReq struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Steps       json.RawMessage `json:"steps"`
	StepsJSON   string          `json:"steps_json"`
	Enabled     *bool           `json:"enabled"`
}

// stepsJSON 把请求里的步骤归一成入库字符串：优先 steps（数组），其次 steps_json（原样），
// 都没有则 "[]"。两者都不合法时报错，交给 HTTP 层回 400。
func (req apiPipelineReq) stepsJSON() (string, error) {
	if len(req.Steps) > 0 && string(req.Steps) != "null" {
		var arr []json.RawMessage
		if err := json.Unmarshal(req.Steps, &arr); err != nil {
			return "", errors.New("steps 必须是 JSON 数组")
		}
		b, err := json.Marshal(arr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if s := strings.TrimSpace(req.StepsJSON); s != "" {
		if !json.Valid([]byte(s)) {
			return "", errors.New("steps_json 不是合法 JSON")
		}
		return s, nil
	}
	return "[]", nil
}

// isPipelineNotFound 判断 Manager 的中文「不存在」错误（口径与 rule_sources.go:96 一致）。
func isPipelineNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "不存在")
}

// isPipelineConflict 判断 name 唯一索引冲突：modernc.org/sqlite 的报错文本是
// "constraint failed: UNIQUE constraint failed: pipelines.name (2067)"。
func isPipelineConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// writePipelineErr 把 Manager 错误映射成 404（不存在）或 500（真故障）。
func (s *Server) writePipelineErr(w http.ResponseWriter, r *http.Request, err error) {
	if isPipelineNotFound(err) {
		s.writeAPIError(w, r, http.StatusNotFound, err)
		return
	}
	s.writeAPIError(w, r, http.StatusInternalServerError, err)
}

// handleAPIPipelinesCreate 处理 POST /api/pipelines：成功 201 {"pipeline":{...}}。
func (s *Server) handleAPIPipelinesCreate(w http.ResponseWriter, r *http.Request) {
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	var req apiPipelineReq
	if !s.decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("流水线名称不能为空"))
		return
	}
	steps, err := req.stepsJSON()
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p, err := m.CreatePipeline(r.Context(), req.Name, req.Description, steps, enabled)
	if err != nil {
		if isPipelineConflict(err) {
			s.writeAPIError(w, r, http.StatusConflict, errors.New("流水线名称已存在"))
			return
		}
		s.writePipelineErr(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]any{"pipeline": p})
}

// handleAPIPipeline 处理 GET/PUT/DELETE /api/pipelines/{id}。
func (s *Server) handleAPIPipeline(w http.ResponseWriter, r *http.Request) {
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
		p, err := m.GetPipeline(r.Context(), id)
		if err != nil {
			s.writePipelineErr(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"pipeline": p})
	case http.MethodPut:
		cur, err := m.GetPipeline(r.Context(), id)
		if err != nil {
			s.writePipelineErr(w, r, err)
			return
		}
		var req apiPipelineReq
		if !s.decodeBody(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			s.writeAPIError(w, r, http.StatusBadRequest, errors.New("流水线名称不能为空"))
			return
		}
		steps, err := req.stepsJSON()
		if err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		upd := cur
		upd.Name = strings.TrimSpace(req.Name)
		upd.Description = req.Description
		upd.StepsJSON = steps
		if req.Enabled != nil {
			upd.Enabled = *req.Enabled
		}
		if err := m.UpdatePipeline(r.Context(), upd); err != nil {
			s.writePipelineErr(w, r, err)
			return
		}
		got, err := m.GetPipeline(r.Context(), id)
		if err != nil {
			s.writePipelineErr(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"pipeline": got})
	case http.MethodDelete:
		if err := m.DeletePipeline(r.Context(), id); err != nil {
			s.writePipelineErr(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 GET/PUT/DELETE"))
	}
}
