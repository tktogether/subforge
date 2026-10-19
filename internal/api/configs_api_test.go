package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
)

// [common] 内 config= 只有最后一次生效（template/ini.go:371-373：repeatable 白名单不含 config，
// 非可重复键走 Section.Set，同名键原地覆盖），所以两个候选分别放在 [common] 与 [custom]：
// Semantics 合并这两段（config/template.go:199-213）且 customControl 不含 config
// （config/custom.go:48-55），得到 [a.ini, b.ini]，顺序 common 在前 —— 正好覆盖去重/保序/default 取第一个。
const configsTestTemplate = `[common]
config = http://example.com/a.ini
target = clash.meta

[custom]
config = http://example.com/b.ini
`

func TestAPIConfigs405(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/configs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestAPIConfigsNoBaseTemplate200Empty(t *testing.T) {
	srv := New(Config{Engine: &convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	req := httptest.NewRequest(http.MethodGet, "/api/configs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAPIConfigsFromTemplateCommonConfig(t *testing.T) {
	srv, _ := newCfgTestServer(t)
	tpl, err := config.LoadBody([]byte(configsTestTemplate), "test.ini")
	if err != nil {
		t.Fatalf("LoadBody: %v", err)
	}
	srv.engine.BaseTemplate = func() *config.Template { return tpl }
	req := httptest.NewRequest(http.MethodGet, "/api/configs", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Configs []string `json:"configs"`
		Default string   `json:"default"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(got.Configs) != 2 || got.Default != "http://example.com/a.ini" {
		t.Fatalf("got %+v", got)
	}
}
