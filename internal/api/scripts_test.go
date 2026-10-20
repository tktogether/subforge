package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestScriptsListEmpty(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiGet(t, ts, "/api/scripts")
	if code != http.StatusOK {
		t.Fatalf("GET 空库状态码 = %d，应为 200；body=%s", code, body)
	}
	if !strings.Contains(body, "\"scripts\":[]") {
		t.Fatalf("空库应输出 scripts 空数组，body=%s", body)
	}
}

func TestScriptsPostCreated(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1","description":"d","code":"foo","enabled":true}`)
	if code != http.StatusCreated {
		t.Fatalf("POST 合法状态码 = %d，应为 201；body=%s", code, body)
	}
	var resp struct {
		Script struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Code    string `json:"code"`
			Enabled bool   `json:"enabled"`
		} `json:"script"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 JSON 失败: %v; body=%s", err, body)
	}
	if resp.Script.ID <= 0 {
		t.Fatalf("ID 应 > 0，got=%d", resp.Script.ID)
	}
	if resp.Script.Name != "s1" || resp.Script.Code != "foo" || !resp.Script.Enabled {
		t.Fatalf("字段不匹配: %+v", resp.Script)
	}
}

func TestScriptsPostBlankName(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":""}`)
	if code != http.StatusBadRequest {
		t.Fatalf("POST 空 name 应 400，got=%d; body=%s", code, body)
	}
}

func TestScriptsPostBadJSON(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", "{not json")
	if code != http.StatusBadRequest {
		t.Fatalf("POST 非法 JSON 应 400，got=%d; body=%s", code, body)
	}
}

func TestScriptsPostInvalidMethod(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, _ := uiDoJSON(t, ts, http.MethodPatch, "/api/scripts", `{"name":"x"}`)
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /api/scripts 应 405，got=%d", code)
	}
}

func TestScriptsListNonEmpty(t *testing.T) {
	ts, _ := uiSubServer(t)
	uiPostJSON(t, ts, "/api/scripts", `{"name":"s1"}`)
	code, _, body := uiGet(t, ts, "/api/scripts")
	if code != http.StatusOK {
		t.Fatalf("GET 状态码 = %d；body=%s", code, body)
	}
	if !strings.Contains(body, "\"s1\"") {
		t.Fatalf("应包含 s1，body=%s", body)
	}
}

func TestScriptGetOK(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1","code":"c"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, gbody := uiGet(t, ts, "/api/scripts/"+strconv.FormatInt(id, 10))
	if code != http.StatusOK {
		t.Fatalf("GET 详情应 200，got=%d; body=%s", code, gbody)
	}
	if !strings.Contains(gbody, "\"s1\"") {
		t.Fatalf("应含 s1，body=%s", gbody)
	}
}

func TestScriptGetNotFound(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiGet(t, ts, "/api/scripts/9999")
	if code != http.StatusNotFound {
		t.Fatalf("GET 不存在应 404，got=%d; body=%s", code, body)
	}
}

func TestScriptGetInvalidID(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiGet(t, ts, "/api/scripts/abc")
	if code != http.StatusBadRequest {
		t.Fatalf("GET 无效 ID 应 400，got=%d; body=%s", code, body)
	}
}

func TestScriptPutOK(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1","code":"c"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, pbody := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10), `{"name":"s2","code":"c2","enabled":true}`)
	if code != http.StatusOK {
		t.Fatalf("PUT 应 200，got=%d; body=%s", code, pbody)
	}
	if !strings.Contains(pbody, "\"s2\"") {
		t.Fatalf("应更新为 s2，body=%s", pbody)
	}
}

func TestScriptPutNotFound(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/9999", `{"name":"s2"}`)
	if code != http.StatusNotFound {
		t.Fatalf("PUT 不存在应 404，got=%d; body=%s", code, body)
	}
}

func TestScriptPutBlankName(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, b := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10), `{"name":""}`)
	if code != http.StatusBadRequest {
		t.Fatalf("PUT 空 name 应 400，got=%d; body=%s", code, b)
	}
}

func TestScriptPutBadJSON(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, b := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10), "{not json")
	if code != http.StatusBadRequest {
		t.Fatalf("PUT 非法 JSON 应 400，got=%d; body=%s", code, b)
	}
}

func TestScriptDeleteOK(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, b := uiDoJSON(t, ts, http.MethodDelete, "/api/scripts/"+strconv.FormatInt(id, 10), "")
	if code != http.StatusOK {
		t.Fatalf("DELETE 应 200，got=%d; body=%s", code, b)
	}
	if !strings.Contains(b, "\"ok\":true") {
		t.Fatalf("应 ok:true，body=%s", b)
	}
	code, _, _ = uiGet(t, ts, "/api/scripts/"+strconv.FormatInt(id, 10))
	if code != http.StatusNotFound {
		t.Fatalf("删除后 GET 应 404，got=%d", code)
	}
}

func TestScriptDeleteNotFound(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiDoJSON(t, ts, http.MethodDelete, "/api/scripts/9999", "")
	if code != http.StatusNotFound {
		t.Fatalf("DELETE 不存在应 404，got=%d; body=%s", code, body)
	}
}

func TestScriptDeleteInvalidID(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiDoJSON(t, ts, http.MethodDelete, "/api/scripts/abc", "")
	if code != http.StatusBadRequest {
		t.Fatalf("DELETE 无效 ID 应 400，got=%d; body=%s", code, body)
	}
}

func TestScriptMethodNotAllowed(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s1"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, _ := uiDoJSON(t, ts, http.MethodPatch, "/api/scripts/"+strconv.FormatInt(id, 10), "")
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /api/scripts/{id} 应 405，got=%d", code)
	}
}

func TestScriptsNoManager(t *testing.T) {
	ts := newTestServer(t, nil)
	code, _, _ := uiGet(t, ts, "/api/scripts")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("无 manager GET 应 503，got=%d", code)
	}
	code, _, _ = uiPostJSON(t, ts, "/api/scripts", `{"name":"x"}`)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("无 manager POST 应 503，got=%d", code)
	}
	code, _, _ = uiGet(t, ts, "/api/scripts/1")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("无 manager GET/{id} 应 503，got=%d", code)
	}
	code, _, _ = uiDoJSON(t, ts, http.MethodPut, "/api/scripts/1", `{"name":"x"}`)
	if code != http.StatusServiceUnavailable {
		t.Fatalf("无 manager PUT 应 503，got=%d", code)
	}
	code, _, _ = uiDoJSON(t, ts, http.MethodDelete, "/api/scripts/1", "")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("无 manager DELETE 应 503，got=%d", code)
	}
}

// TestScriptsKindCreateAndList 覆盖 D-01：POST kind=operate 落库、201 响应带 kind、列表接口带 kind。
func TestScriptsKindCreateAndList(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s-oper","kind":"operate","code":"node.port = 443","enabled":true}`)
	if code != http.StatusCreated {
		t.Fatalf("POST kind=operate 状态码 = %d，应为 201；body=%s", code, body)
	}
	var resp struct {
		Script struct {
			ID   int64  `json:"id"`
			Kind string `json:"kind"`
		} `json:"script"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析 POST 响应失败: %v; body=%s", err, body)
	}
	if resp.Script.Kind != "operate" {
		t.Fatalf("POST 响应 script.kind = %q，期望 operate；body=%s", resp.Script.Kind, body)
	}
	if !strings.Contains(body, `"kind":"operate"`) {
		t.Fatalf("POST 响应体缺少 kind 字段，body=%s", body)
	}

	// 列表接口必须回传 kind（前端 PipelinesPage 靠它做类型校验）。
	code, _, lbody := uiGet(t, ts, "/api/scripts")
	if code != http.StatusOK {
		t.Fatalf("GET 列表状态码 = %d；body=%s", code, lbody)
	}
	if !strings.Contains(lbody, `"kind":"operate"`) {
		t.Fatalf("列表响应缺少 kind，body=%s", lbody)
	}
	var lresp struct {
		Scripts []struct {
			ID   int64  `json:"id"`
			Kind string `json:"kind"`
		} `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(lbody), &lresp); err != nil {
		t.Fatalf("解析列表响应失败: %v; body=%s", err, lbody)
	}
	found := false
	for _, s := range lresp.Scripts {
		if s.ID == resp.Script.ID {
			found = true
			if s.Kind != "operate" {
				t.Errorf("列表 kind = %q，期望 operate", s.Kind)
			}
		}
	}
	if !found {
		t.Fatalf("列表未包含 id=%d，body=%s", resp.Script.ID, lbody)
	}
}

// TestScriptsPostKindDefaultFilter 覆盖 kind 缺省时默认 filter（与 /api/scripts/test 同语义）。
func TestScriptsPostKindDefaultFilter(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s-default","code":"c"}`)
	if code != http.StatusCreated {
		t.Fatalf("POST 缺省 kind 状态码 = %d，应为 201；body=%s", code, body)
	}
	if !strings.Contains(body, `"kind":"filter"`) {
		t.Fatalf("缺省 kind 应默认 filter，body=%s", body)
	}
}

// TestScriptsPostUnknownKind 覆盖非法 kind → 400 且文案与 /api/scripts/test 一致。
func TestScriptsPostUnknownKind(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s-bad","kind":"bogus","code":"c"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("POST kind=bogus 应 400，got=%d; body=%s", code, body)
	}
	if !strings.Contains(body, "未知的脚本类型") {
		t.Fatalf("400 文案应含「未知的脚本类型」，body=%s", body)
	}
}

// TestScriptPutKindChanges 覆盖 PUT 改 kind → 200 且回读为 filter。
func TestScriptPutKindChanges(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s-oper","kind":"operate","code":"node.port = 443"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, pbody := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10),
		`{"name":"s-oper","kind":"filter","code":"node.type === 'ss'","enabled":true}`)
	if code != http.StatusOK {
		t.Fatalf("PUT 改 kind 应 200，got=%d; body=%s", code, pbody)
	}
	if !strings.Contains(pbody, `"kind":"filter"`) {
		t.Fatalf("PUT 响应 kind 应为 filter，body=%s", pbody)
	}
}

// TestScriptPutEmptyKindKeepsOld 覆盖 PUT 不带 kind 时保留库里的旧值（老脚本不被改成 filter）。
func TestScriptPutEmptyKindKeepsOld(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s-keep","kind":"operate","code":"node.port = 443"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, pbody := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10),
		`{"name":"s-keep","code":"node.port = 8443","enabled":true}`)
	if code != http.StatusOK {
		t.Fatalf("PUT 缺省 kind 应 200，got=%d; body=%s", code, pbody)
	}
	if !strings.Contains(pbody, `"kind":"operate"`) {
		t.Fatalf("PUT 缺省 kind 应保留 operate，body=%s", pbody)
	}
}

// TestScriptPutUnknownKind 覆盖 PUT 非法 kind → 400。
func TestScriptPutUnknownKind(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"s-bad2","code":"c"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, pbody := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10), `{"name":"s-bad2","kind":"bogus"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("PUT kind=bogus 应 400，got=%d; body=%s", code, pbody)
	}
	if !strings.Contains(pbody, "未知的脚本类型") {
		t.Fatalf("400 文案应含「未知的脚本类型」，body=%s", pbody)
	}
}
