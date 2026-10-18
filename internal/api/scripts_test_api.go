package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"time"

	"github.com/tktogether/subforge/internal/model"
	"github.com/tktogether/subforge/internal/transform"
)

const scriptTestMaxTimeout = 30 * time.Second

var defaultScriptSampleNodes = []map[string]any{
	{"name": "ss-node", "type": "ss", "server": "1.2.3.4", "port": 8388},
	{"name": "vmess-node", "type": "vmess", "server": "5.6.7.8", "port": 443},
	{"name": "trojan-node", "type": "trojan", "server": "9.10.11.12", "port": 443},
}

type scriptTestReq struct {
	Code    string         `json:"code"`
	Kind    string         `json:"kind"`
	Node    map[string]any `json:"node"`
	Timeout int            `json:"timeout"`
}

type scriptTestResp struct {
	OK         bool           `json:"ok"`
	Kept       bool           `json:"kept"`
	Before     map[string]any `json:"before"`
	After      map[string]any `json:"after"`
	DurationMs int64          `json:"duration_ms"`
	Error      string         `json:"error"`
}

func (s *Server) handleAPIScriptsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, errors.New("仅支持 POST"))
		return
	}
	var req scriptTestReq
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
	}
	if req.Code == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("缺少 code"))
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "filter"
	}
	if kind != "filter" && kind != "operate" {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("未知的脚本类型: "+kind))
		return
	}
	if req.Timeout > 30000 {
		s.writeAPIError(w, r, http.StatusBadRequest, errors.New("timeout 超过 30000ms 上限"))
		return
	}
	timeout := time.Duration(req.Timeout) * time.Millisecond

	nodeMap := req.Node
	if nodeMap == nil {
		nodeMap = defaultScriptSampleNodes[0]
	}
	n := model.NodeFromRaw(cloneMap(nodeMap), "")

	start := time.Now()
	if kind == "filter" {
		sf, err := transform.NewScriptFilter("script-test", req.Code, timeout)
		if err != nil {
			s.writeJSON(w, http.StatusOK, scriptTestResp{
				OK: false, Kept: false, Before: map[string]any{}, After: map[string]any{},
				DurationMs: time.Since(start).Milliseconds(), Error: err.Error(),
			})
			return
		}
		out, err := sf.Apply(model.NodeList{n})
		dur := time.Since(start).Milliseconds()
		if err != nil {
			s.writeJSON(w, http.StatusOK, scriptTestResp{
				OK: false, Kept: false, Before: map[string]any{}, After: map[string]any{},
				DurationMs: dur, Error: err.Error(),
			})
			return
		}
		s.writeJSON(w, http.StatusOK, scriptTestResp{
			OK: true, Kept: len(out) > 0, Before: map[string]any{}, After: map[string]any{},
			DurationMs: dur, Error: "",
		})
		return
	}
	before := scriptTestBefore(nodeMap)
	so, err := transform.NewScriptOperate("script-test", req.Code, timeout)
	if err != nil {
		s.writeJSON(w, http.StatusOK, scriptTestResp{
			OK: false, Kept: false, Before: before, After: map[string]any{},
			DurationMs: time.Since(start).Milliseconds(), Error: err.Error(),
		})
		return
	}
	out, err := so.Apply(model.NodeList{n})
	dur := time.Since(start).Milliseconds()
	if err != nil {
		s.writeJSON(w, http.StatusOK, scriptTestResp{
			OK: false, Kept: false, Before: before, After: map[string]any{},
			DurationMs: dur, Error: err.Error(),
		})
		return
	}
	afterRaw := map[string]any{}
	if len(out) > 0 {
		afterRaw = out[0].Raw
	}
	diff := map[string]any{}
	for k, v := range afterRaw {
		if !reflect.DeepEqual(v, before[k]) {
			diff[k] = v
		}
	}
	for k := range before {
		if _, ok := afterRaw[k]; !ok {
			diff[k] = nil
		}
	}
	s.writeJSON(w, http.StatusOK, scriptTestResp{
		OK: true, Kept: false, Before: before, After: diff,
		DurationMs: dur, Error: "",
	})
}

// scriptTestBefore 返回脚本可见的节点快照：与 transform 的节点投影一致，
// 把 name/type/server/port 归一化为字符串，避免 JSON 数字与派生字符串造成假差异。
func scriptTestBefore(nodeMap map[string]any) map[string]any {
	out := cloneMap(nodeMap)
	n := model.NodeFromRaw(nodeMap, "")
	for k, v := range map[string]string{"name": n.Name, "type": n.Type, "server": n.Server, "port": n.Port} {
		if _, ok := out[k]; ok {
			out[k] = v
		}
	}
	return out
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
