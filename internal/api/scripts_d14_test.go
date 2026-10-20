package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// TestScriptsPostSyntaxError 覆盖 D-14：保存前用 goja 编译校验，语法错 400 且文案含「语法」与行号。
func TestScriptsPostSyntaxError(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"bad","code":"if ("}`)
	if code != http.StatusBadRequest {
		t.Fatalf("POST 语法错应 400，got=%d; body=%s", code, body)
	}
	if !strings.Contains(body, "语法") {
		t.Fatalf("400 文案应含「语法」，body=%s", body)
	}
	if !strings.Contains(body, "Line 1") {
		t.Fatalf("400 文案应带 goja 行号（Line 1），body=%s", body)
	}
	// 语法错的脚本不应落库。
	code, _, lbody := uiGet(t, ts, "/api/scripts")
	if code != http.StatusOK {
		t.Fatalf("GET 列表应 200，got=%d; body=%s", code, lbody)
	}
	if strings.Contains(lbody, "bad") {
		t.Fatalf("语法错的脚本不应落库，body=%s", lbody)
	}
}

// TestScriptsPostEmptyCode 覆盖 D-14：空代码（含纯空白）400。
func TestScriptsPostEmptyCode(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"empty","code":"   "}`)
	if code != http.StatusBadRequest {
		t.Fatalf("POST 空代码应 400，got=%d; body=%s", code, body)
	}
	if !strings.Contains(body, "语法") {
		t.Fatalf("400 文案应含「语法」，body=%s", body)
	}
}

// TestScriptsPutSyntaxError 覆盖 D-14：PUT 语法错 400，且库里保留旧代码。
func TestScriptsPutSyntaxError(t *testing.T) {
	ts, _ := uiSubServer(t)
	_, _, body := uiPostJSON(t, ts, "/api/scripts", `{"name":"keep","code":"true"}`)
	var resp struct {
		Script struct {
			ID int64 `json:"id"`
		} `json:"script"`
	}
	_ = json.Unmarshal([]byte(body), &resp)
	id := resp.Script.ID
	code, _, pbody := uiDoJSON(t, ts, http.MethodPut, "/api/scripts/"+strconv.FormatInt(id, 10), `{"name":"keep","code":"if ("}`)
	if code != http.StatusBadRequest {
		t.Fatalf("PUT 语法错应 400，got=%d; body=%s", code, pbody)
	}
	if !strings.Contains(pbody, "语法") {
		t.Fatalf("400 文案应含「语法」，body=%s", pbody)
	}
	code, _, gbody := uiGet(t, ts, "/api/scripts/"+strconv.FormatInt(id, 10))
	if code != http.StatusOK {
		t.Fatalf("GET 应 200，got=%d; body=%s", code, gbody)
	}
	if !strings.Contains(gbody, `"code":"true"`) {
		t.Fatalf("语法错不应覆盖旧代码，body=%s", gbody)
	}
}

// TestScriptsPostValidOperateStillSaves 覆盖 D-14：合法 operate 脚本仍正常保存（校验不误报）。
func TestScriptsPostValidOperateStillSaves(t *testing.T) {
	ts, _ := uiSubServer(t)
	code, _, body := uiPostJSON(t, ts, "/api/scripts",
		`{"name":"op","kind":"operate","code":"node.name = \"[\" + node.type + \"] \" + node.name","enabled":true}`)
	if code != http.StatusCreated {
		t.Fatalf("合法 operate 脚本应 201，got=%d; body=%s", code, body)
	}
	if !strings.Contains(body, `"kind":"operate"`) {
		t.Fatalf("响应应带 kind=operate，body=%s", body)
	}
}
