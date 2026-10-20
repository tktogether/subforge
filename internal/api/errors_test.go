package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// errProbeReq 是 D-22 文案治理的探针请求体：interval 是整数，喂字符串就该被翻译。
type errProbeReq struct {
	Interval int    `json:"interval"`
	Name     string `json:"name"`
}

// errBody 把错误响应体解成 map，方便断言字段。
func errBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatalf("错误响应不是 JSON: %q", rr.Body.String())
	}
	return m
}

// TestWriteErrorDecodeTypeMessage：字段类型不对时，用户看到中文文案而不是 Go 报文。
func TestWriteErrorDecodeTypeMessage(t *testing.T) {
	s := &Server{logger: quietLogger()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/probe", strings.NewReader(`{"interval":"abc"}`))
	var dst errProbeReq
	if s.decodeBody(rr, req, &dst) {
		t.Fatal("decodeBody 不该成功")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d want 400", rr.Code)
	}
	body := errBody(t, rr)
	if body["error"] != "请求参数格式不正确：字段 interval 需要整数" {
		t.Fatalf("error=%v", body["error"])
	}
	if !strings.Contains(fmt.Sprint(body["detail"]), "cannot unmarshal") {
		t.Fatalf("detail 应保留原始报文: %v", body["detail"])
	}
}

// TestWriteErrorDecodeSyntaxMessage：不是合法 JSON 时也给中文文案。
func TestWriteErrorDecodeSyntaxMessage(t *testing.T) {
	s := &Server{logger: quietLogger()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/probe", strings.NewReader(`{"interval":`))
	var dst errProbeReq
	if s.decodeBody(rr, req, &dst) {
		t.Fatal("decodeBody 不该成功")
	}
	body := errBody(t, rr)
	if body["error"] != "请求体不是合法的 JSON" {
		t.Fatalf("error=%v", body["error"])
	}
}

// TestWriteErrorInternalServerError：500 统一文案，原始报文只进 detail。
func TestWriteErrorInternalServerError(t *testing.T) {
	s := &Server{logger: quietLogger()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/probe", nil)
	s.fail(rr, req, http.StatusInternalServerError, errors.New("boom: 内部细节"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d want 500", rr.Code)
	}
	body := errBody(t, rr)
	msg := fmt.Sprint(body["error"])
	if !strings.HasPrefix(msg, "服务端内部错误") {
		t.Fatalf("error=%q", msg)
	}
	if strings.Contains(msg, "boom") {
		t.Fatalf("内部报文不该出现在 error 里: %q", msg)
	}
	if body["detail"] != "boom: 内部细节" {
		t.Fatalf("detail=%v", body["detail"])
	}
}

// TestWriteErrorInternalServerErrorRequestID：真实请求带 request id 时，文案里带 ID。
func TestWriteErrorInternalServerErrorRequestID(t *testing.T) {
	s := &Server{logger: quietLogger()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/probe", nil)
	h := chimw.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.fail(w, r, http.StatusInternalServerError, errors.New("boom"))
	}))
	h.ServeHTTP(rr, req)
	body := errBody(t, rr)
	msg := fmt.Sprint(body["error"])
	if !strings.HasPrefix(msg, "服务端内部错误（请求 ID ") {
		t.Fatalf("error=%q", msg)
	}
	if id, _ := body["request_id"].(string); id == "" {
		t.Fatalf("request_id 不该为空: %v", body)
	}
}

// TestWriteErrorOtherStatusUnchanged：非 500 的中文文案与报文形状保持原样。
func TestWriteErrorOtherStatusUnchanged(t *testing.T) {
	s := &Server{logger: quietLogger()}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/probe", nil)
	s.writeAPIError(rr, req, http.StatusNotFound, errors.New("规则源 x 不存在"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code=%d want 404", rr.Code)
	}
	body := errBody(t, rr)
	if body["error"] != "规则源 x 不存在" {
		t.Fatalf("error=%v", body["error"])
	}
	if _, ok := body["detail"]; ok {
		t.Fatalf("普通错误不该带 detail: %v", body)
	}
	if _, ok := body["request_id"]; ok {
		t.Fatalf("普通错误不该带 request_id: %v", body)
	}
}

// TestJSONDecodeErrorText：文案翻译的纯函数分支。
func TestJSONDecodeErrorText(t *testing.T) {
	if _, ok := jsonDecodeErrorText(nil); ok {
		t.Fatal("nil 不该被翻译")
	}
	if _, ok := jsonDecodeErrorText(errors.New("普通错误")); ok {
		t.Fatal("普通错误不该被翻译")
	}
	typeErr := &json.UnmarshalTypeError{Value: "string", Type: reflect.TypeOf(0), Field: "interval"}
	msg, ok := jsonDecodeErrorText(typeErr)
	if !ok || msg != "请求参数格式不正确：字段 interval 需要整数" {
		t.Fatalf("msg=%q ok=%v", msg, ok)
	}
	// 字段名为空时兜底成「请求体」
	emptyField := &json.UnmarshalTypeError{Value: "string", Type: reflect.TypeOf(0)}
	msg, ok = jsonDecodeErrorText(emptyField)
	if !ok || msg != "请求参数格式不正确：请求体需要整数" {
		t.Fatalf("msg=%q ok=%v", msg, ok)
	}
	if got := goTypeName("chan int"); got != "chan int" {
		t.Fatalf("goTypeName=%q", got)
	}
	if got := goTypeName("string"); got != "字符串" {
		t.Fatalf("goTypeName=%q", got)
	}
}
