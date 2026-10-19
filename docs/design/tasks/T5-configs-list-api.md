# T5 · GET /api/configs：令牌可选配置模板列表 API

## 0. 你是谁、红线是什么
你是 SubForge 后端实施队员。三条红线：
1. 只允许修改下面「改动清单」列出的文件，其他一律只读。
2. 不许 git commit / git add / git stash。
3. 不许新建任何任务卡；卡状态由 Lead 操作。

## 1. 进度心跳（必须执行）
send_message 发给 lead，共 4 次：
- 心跳1「T5 book read, starting」
- 心跳2「T5 skeleton written」
- 心跳3「T5 self-test starting」
- 心跳4「T5 done」+ 自测输出原文

## 2. WHY
订阅令牌对话框的 config 字段现在是手填文本框（用户原话：per-token 操作不能走下拉吗）。前端要做成下拉/combobox，需要一个列表 API。pipeline 列表已有 GET /api/pipelines，config 没有列表来源——本任务补 GET /api/configs。

## 3. config 的真实语义（已核实，不许另编）
- internal/convert/engine.go:318-327：请求参数 config=<ref> 会经 e.LoadTemplate(ctx, ref) 加载模板；serve 模式下 cmd/subforge/serve.go:143-145 用 config.LoadSource 支持 URL/路径。
- 模板 [common] 段的 config= 键值对会进入 Semantics().Params["config"]（internal/config/template.go:199-212：「键名即参数名」）。
- 所以 config 候选 = 当前生效模板 [common] 里 config= 的值（去重），这是模板作者给出的推荐远程模板。default = 第一个。

## 4. 改动清单（只许这些文件）
- 新建 internal/api/configs_api.go
- 修改 internal/api/server.go（路由注册追加 1 行）
- 新建 internal/api/configs_api_test.go

## 5. 现状契约（照抄不改）
- internal/api/pipelines.go 的 handler 风格（s.engine 判空 → 503「转换引擎未启用」）。
- internal/convert/engine.go:42 `BaseTemplate func() *config.Template`；internal/config/template.go:191 `func (t *Template) Semantics() (Semantics, error)`；Semantics.Params map[string][]string。
- internal/api/subscription.go:37 writeAPIError / server.go:345 writeJSON。
- 测试助手 newCfgTestServer（settings_test.go:15）；给 BaseTemplate 挂模板的写法照抄 internal/convert/named_pipeline_test.go:41-48（config.LoadBody + e.BaseTemplate = func() *config.Template { return tpl }）。

## 6. 逐步实施（代码可整段照抄）

### 步骤 1：新建 internal/api/configs_api.go（完整文件）
```go
package api

import (
	"fmt"
	"net/http"
	"strings"
)

// handleAPIConfigs 处理 GET /api/configs：返回令牌/转换可用的 config 候选。
// 数据源：当前生效模板 [common] 段的 config= 值（模板作者推荐的远程模板）。
func (s *Server) handleAPIConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 GET"))
		return
	}
	if s.engine == nil || s.engine.BaseTemplate == nil {
		s.writeAPIError(w, r, http.StatusServiceUnavailable, fmt.Errorf("转换引擎未启用"))
		return
	}
	tpl := s.engine.BaseTemplate()
	if tpl == nil {
		s.writeJSON(w, http.StatusOK, map[string]any{"configs": []string{}, "default": ""})
		return
	}
	sem, err := tpl.Semantics()
	if err != nil {
		s.writeAPIError(w, r, http.StatusBadGateway, fmt.Errorf("解释模板失败: %w", err))
		return
	}
	seen := map[string]bool{}
	configs := make([]string, 0, 4)
	for _, v := range sem.Params["config"] {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		configs = append(configs, v)
	}
	def := ""
	if len(configs) > 0 {
		def = configs[0]
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"configs": configs, "default": def})
}
```

### 步骤 2：修改 internal/api/server.go
在 r.Get("/api/rule-sources", ...)（:241）附近追加：
```go
	r.Get("/api/configs", s.handleAPIConfigs)
```

### 步骤 3：新建 internal/api/configs_api_test.go（完整文件）
```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tktogether/subforge/internal/config"
	"github.com/tktogether/subforge/internal/convert"
)

const configsTestTemplate = `[common]
config = http://example.com/a.ini
config = http://example.com/b.ini
target = clash.meta
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
```
校对（写完后必做）：grep "func LoadBody" internal/config 核对签名（named_pipeline_test.go:43 用法是 config.LoadBody([]byte(body), "test.ini")）；若 503 分支判定与 pipelines.go 实际写法不一致，以 pipelines.go 为准。

## 7. 自测命令（repo 根逐条跑，贴原文）
```
go vet ./...
go test ./internal/api/ -count=1
```

## 8. 完成报告格式（心跳4 一起发）
- 改动文件列表 + 自测输出原文 + 「已核对」LoadBody 签名确认