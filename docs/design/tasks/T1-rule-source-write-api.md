# T1 · 规则源「新增 / 删除 / 单源更新」后端写 API

## 0. 你是谁、红线是什么
你是 SubForge 后端实施队员。三条红线：
1. 只允许修改下面「改动清单」列出的文件，其他一律只读。
2. 不许 git commit / git add / git stash，工作区留给 Lead 验收。
3. 不许新建任何任务卡；只做本卡。卡状态由 Lead 操作，你不碰。

## 1. 进度心跳（必须执行，免费模型纪律）
用 send_message 发给 lead，共 4 次，每次一行即可：
- 心跳1：读完任务书后（说「T1 book read, starting」）
- 心跳2：三个新文件骨架落盘后（说「T1 skeleton written: <文件列表>」）
- 心跳3：开始自测时（说「T1 self-test starting」）
- 心跳4：全部自测通过后，贴完整命令输出原文（说「T1 done」+ 输出）

## 2. WHY（为什么做）
规则源现在只能手改 manifest.yaml 才能新增/删除源（internal/api/server.go:241-245 只有 GET/init/update/rollback/mrs 五个只读或全量接口）。本任务补三个写接口，前端（另一队员）后续接上。

## 3. 改动清单（只许这些文件）
- 新建 internal/manifest/save.go
- 新建 internal/subforge/rulesources_write.go
- 修改 internal/api/rule_sources.go（文件末尾追加 3 个 handler + import 一行）
- 修改 internal/api/server.go（路由注册追加 3 行）
- 新建 internal/api/rule_sources_write_test.go

## 4. 现状契约（照抄不改）
- internal/manifest/manifest.go:25-43：Manifest{Sources []Source; Output Output}，Source{Name,URL,Type,UpdateInterval,InputFormat string}（yaml tag 已有）。
- internal/manifest/manifest.go:46-66 LoadManifest 会 validate+applyDefaults+把 output.path 转绝对路径 —— 所以写回路径【禁止】用 LoadManifest 的返回值写回（会固化绝对路径），必须用下面新写的 LoadManifestRaw。
- internal/manifest/manifest.go:68-87 m.validate()（≥1 source / URL 非空 / 名称唯一）；:144 urlBasename（包内私有）。
- internal/subforge/rulesources.go:75-84 ResolveRuleManifestPath(ctx) (string, error)；:149-162 LoadRuleManifest；:203-214 UpdateRuleSources 的写法（先 LoadRuleManifest 再设 Output.Path 再 mf.Update(ctx, manifest.WithRetention(manifest.DefaultRetention))）。
- internal/api/rule_sources.go:113-144 rollback handler 是你要照抄的错误映射风格（writeAPIError / writeJSON / strings.Contains(err.Error(),"不存在")→404）。
- internal/api/server.go:241-245 路由注册风格。
- 测试助手：internal/api/settings_test.go:15-25 newCfgTestServer(t) 返回 (srv, mgr)；internal/api/template_api_test.go:17-25 seedConfig(t, mgr, key, val) 直接写 config 表。

## 5. 逐步实施（代码可整段照抄）

### 步骤 1：新建 internal/manifest/save.go（完整文件）
```go
package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadManifestRaw 只做字节级反序列化：不校验、不补默认值、不改写 output.path。
// 专供「读原始结构 → 修改 → 写回」使用，避免 LoadManifest 的绝对路径固化。
func LoadManifestRaw(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest %s: %w", path, err)
	}
	return &m, nil
}

// SaveManifest 把 manifest 写回 path；已存在旧文件时先备份 <path>.bak。
func SaveManifest(path string, m *Manifest) error {
	if m == nil {
		return fmt.Errorf("manifest 为空")
	}
	if old, rerr := os.ReadFile(path); rerr == nil {
		_ = os.WriteFile(path+".bak", old, 0o644)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// DeriveSourceName 从 URL 推导缺省名称，与 applyDefaults 的规则一致。
func DeriveSourceName(u string) string {
	return urlBasename(u)
}

// ValidateForWrite 复用读取侧校验（≥1 source / URL 非空 / 名称唯一）。
func (m *Manifest) ValidateForWrite() error {
	return m.validate()
}
```

### 步骤 2：新建 internal/subforge/rulesources_write.go（完整文件）
```go
package subforge

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/tktogether/subforge/internal/manifest"
)

// AddRuleSource 追加一个规则源并写回 manifest，返回 (清单路径, 是否新建, 错误)。
// name 为空时从 URL 推导；重名返回错误（消息含「已存在」）。
func (m *Manager) AddRuleSource(ctx context.Context, src manifest.Source) (string, bool, error) {
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", false, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", false, fmt.Errorf("规则源清单不存在，请先生成默认清单: %s", path)
	}
	if strings.TrimSpace(src.URL) == "" {
		return "", false, fmt.Errorf("url 不能为空")
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", false, fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	name := strings.TrimSpace(src.Name)
	if name == "" {
		name = manifest.DeriveSourceName(src.URL)
	}
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			return "", false, fmt.Errorf("规则源 %q 已存在", name)
		}
	}
	src.Name = name
	mf.Sources = append(mf.Sources, src)
	if err := mf.ValidateForWrite(); err != nil {
		return "", false, err
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// RemoveRuleSource 按名称删除规则源并写回 manifest。
// 不存在返回错误（消息含「不存在」）；删空返回错误（消息含「至少保留」）。
func (m *Manager) RemoveRuleSource(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	path, err := m.ResolveRuleManifestPath(ctx)
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return "", fmt.Errorf("规则源清单不存在，请先生成默认清单: %s", path)
	}
	mf, err := manifest.LoadManifestRaw(path)
	if err != nil {
		return "", fmt.Errorf("读取规则源清单 %s: %w", path, err)
	}
	kept := make([]manifest.Source, 0, len(mf.Sources))
	found := false
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			found = true
			continue
		}
		kept = append(kept, s)
	}
	if !found {
		return "", fmt.Errorf("规则源 %q 不存在", name)
	}
	if len(kept) == 0 {
		return "", fmt.Errorf("至少保留一个规则源")
	}
	mf.Sources = kept
	if err := mf.ValidateForWrite(); err != nil {
		return "", err
	}
	if err := manifest.SaveManifest(path, mf); err != nil {
		return "", err
	}
	return path, nil
}

// UpdateRuleSourceOne 只更新指定名称的规则源，返回该源的结果。
// name 不存在返回错误（消息含「不存在」）。
func (m *Manager) UpdateRuleSourceOne(ctx context.Context, name string) ([]manifest.UpdateResult, error) {
	name = strings.TrimSpace(name)
	mf, err := m.LoadRuleManifest(ctx)
	if err != nil {
		return nil, err
	}
	dir, derr := m.ResolveRulesOutputDir(ctx)
	if derr == nil && dir != "" {
		mf.Output.Path = dir
	}
	target := make([]manifest.Source, 0, 1)
	for _, s := range mf.Sources {
		if s.SourceName() == name {
			target = append(target, s)
		}
	}
	if len(target) == 0 {
		return nil, fmt.Errorf("规则源 %q 不存在", name)
	}
	mf.Sources = target
	return mf.Update(ctx, manifest.WithRetention(manifest.DefaultRetention))
}
```

### 步骤 3：修改 internal/api/rule_sources.go
(a) import 块（文件顶部 :3-13）追加一行（注意与其他 import 按字母序）：
```go
	"github.com/tktogether/subforge/internal/manifest"
```
(b) 文件末尾追加三个 handler（错误映射全部照抄 rollback handler :114-143 的风格）：
```go
// handleAPIRuleSourcesAdd 处理 POST /api/rule-sources（新增规则源）。
func (s *Server) handleAPIRuleSourcesAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		InputFormat string `json:"input_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("url 不能为空"))
		return
	}
	path, created, err := m.AddRuleSource(r.Context(), manifest.Source{Name: req.Name, URL: req.URL, InputFormat: req.InputFormat})
	if err != nil {
		if strings.Contains(err.Error(), "已存在") {
			s.writeAPIError(w, r, http.StatusConflict, err)
			return
		}
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"path": path, "created": created})
}

// handleAPIRuleSourcesRemove 处理 POST /api/rule-sources/remove（删除规则源）。
func (s *Server) handleAPIRuleSourcesRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("name 不能为空"))
		return
	}
	path, err := m.RemoveRuleSource(r.Context(), strings.TrimSpace(req.Name))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		if strings.Contains(err.Error(), "至少保留") {
			s.writeAPIError(w, r, http.StatusBadRequest, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
}

// handleAPIRuleSourcesUpdateOne 处理 POST /api/rule-sources/update-one（单源更新）。
func (s *Server) handleAPIRuleSourcesUpdateOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 POST"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeAPIError(w, r, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("name 不能为空"))
		return
	}
	results, err := m.UpdateRuleSourceOne(r.Context(), strings.TrimSpace(req.Name))
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			s.writeAPIError(w, r, http.StatusNotFound, err)
			return
		}
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(results))
	for _, res := range results {
		out = append(out, map[string]any{
			"source": res.Source, "version": res.Version, "path": res.Path,
			"from_cache": res.FromCache, "err": res.Err,
		})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"results": out})
}
```
注意：UpdateResult 的字段名先打开 internal/manifest 包 grep "type UpdateResult struct" 核对（Source/Version/Path/FromCache/Err），以实际字段为准，不许编造。

### 步骤 4：修改 internal/api/server.go
在 :245（r.Post("/api/rule-sources/mrs", ...) 之后）追加三行：
```go
	r.Post("/api/rule-sources", s.handleAPIRuleSourcesAdd)
	r.Post("/api/rule-sources/remove", s.handleAPIRuleSourcesRemove)
	r.Post("/api/rule-sources/update-one", s.handleAPIRuleSourcesUpdateOne)
```

### 步骤 5：新建 internal/api/rule_sources_write_test.go（完整文件）
```go
package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tktogether/subforge/internal/convert"
	"github.com/tktogether/subforge/internal/subforge"
)

const seedManifestYAML = `sources:
  - name: base
    url: http://example.com/base.yaml
output:
  format: clash-meta
  path: ./rules
`

func seedManifest(t *testing.T, mgr *subforge.Manager) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	seedConfig(t, mgr, subforge.RuleManifestPathKey, path)
	if err := os.WriteFile(path, []byte(seedManifestYAML), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return path
}

func postJSON(t *testing.T, srv *Server, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestRuleSourcesAdd503(t *testing.T) {
	srv := New(Config{Engine: &&convert.Engine{}, Logger: quietLogger(), Timeout: 5})
	rec := postJSON(t, srv, "/api/rule-sources", `{"url":"http://x/a.yaml"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
}

func TestRuleSourcesAdd405(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	seedManifest(t, mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/rule-sources", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestRuleSourcesAddRemoveRoundTrip(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	path := seedManifest(t, mgr)

	rec := postJSON(t, srv, "/api/rule-sources", `{"name":"foo","url":"http://example.com/foo.yaml"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("add code = %d, body=%s", rec.Code, rec.Body.String())
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "name: foo") {
		t.Fatalf("manifest missing foo: %s", string(b))
	}

	rec = postJSON(t, srv, "/api/rule-sources", `{"name":"foo","url":"http://example.com/foo2.yaml"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup code = %d, want 409", rec.Code)
	}

	rec = postJSON(t, srv, "/api/rule-sources/remove", `{"name":"foo"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove code = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("missing .bak backup: %v", err)
	}

	rec = postJSON(t, srv, "/api/rule-sources/remove", `{"name":"foo"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("remove-missing code = %d, want 404", rec.Code)
	}

	rec = postJSON(t, srv, "/api/rule-sources/remove", `{"name":"base"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("remove-last code = %d, want 400", rec.Code)
	}
}

func TestRuleSourcesUpdateOneRoundTrip(t *testing.T) {
	srv, mgr := newCfgTestServer(t)
	path := seedManifest(t, mgr)

	upd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("DOMAIN-SUFFIX,example.com\nDOMAIN,b.com\n"))
	}))
	defer upd.Close()

	// 把 base 的 URL 改成测试服务器地址（直接改 manifest 文件内容）
	b, _ := os.ReadFile(path)
	nb := strings.Replace(string(b), "http://example.com/base.yaml", upd.URL+"/base.yaml", 1)
	if err := os.WriteFile(path, []byte(nb), 0o644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}

	rec := postJSON(t, srv, "/api/rule-sources/update-one", `{"name":"base"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update-one code = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"results\"") {
		t.Fatalf("update-one body missing results: %s", rec.Body.String())
	}

	rec = postJSON(t, srv, "/api/rule-sources/update-one", `{"name":"nope"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update-one missing code = %d, want 404", rec.Code)
	}
}
```
重要校对（写完后必须逐条做，不许跳过）：
1. grep "type UpdateResult struct" internal/manifest 核对字段名（应为 Source/Version/Path/FromCache/Err），handler 映射和测试断言以实际为准。
2. postJSON 走 srv.Routes() 是为了测真实路由（含新加的三行注册）。newCfgTestServer 没有配 Token，即不鉴权；若仍被中间件拦（401），看 internal/api 里已有测试怎么绕，照抄它们的做法。
3. seedConfig（template_api_test.go:17）与 RuleManifestPathKey（internal/subforge/rulesources.go:18，值 "rules.manifest_path"）已核对存在，直接用。

## 6. 自测命令（在 repo 根 E:\Projects\subforge 下逐条跑，全部贴原文）
```
go vet ./...
go test ./internal/manifest/ ./internal/subforge/ ./internal/api/ -count=1
```
预期：vet 无输出；tests 全 ok。任何 FAIL 都修到过为止，不许跳过用例。

## 7. 完成报告格式（心跳4 一起发）
- 改动文件列表（完整相对路径）
- 自测两条命令的完整输出原文
- 「已核对」清单：UpdateResult 字段 / SetConfig 或 seedConfig / RuleManifestPathKey 各自的实际签名