# T7 规则集（rule_providers）启用/停用 API

## WHY（为什么）
能力矩阵 F9：规则集只读无启停。规则页能看到 providers 列表，但不能停用误导入的规则集。
本卡只做后端。前端开关是另一张卡（T8）。

## 心跳（必须执行，4 时点 send_message 给 lead）
1. 任务书读完 + 锚点校对完
2. 骨架落盘（store 方法 + handler 空壳）
3. 自测开始（跑 go test 前）
4. 全部完成 + 全量命令输出原文

## 红线（违反=返工）
- 只许动这些文件：internal/store/store.go、internal/store/store_test.go、internal/api/ruleproviders.go、internal/api/server.go、internal/api/ruleproviders_test.go、internal/api/routes_test.go（如需）
- 不许 git commit / 不许建卡 / 不许改其他文件
- 不许编造 API；每个写法必须有下面给出的源码锚点撑腰

## 第 0 步：锚点校对（先读再说）
用 Read 工具核对以下行号内容对得上，对不上就停下报告 lead（说明行号偏移）：
- internal/store/store.go:962  func (d *DB) UpsertRuleProvider(ctx context.Context, p RuleProvider) error {   ← 学习 ExecContext 风格
- internal/store/store_test.go:19  func newTestDB(t *testing.T) (*DB, context.Context) {
- internal/api/subscription.go:52  id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)   ← 取路径参数的既有先例（就用 r.PathValue，不要用 chi.URLParam）
- internal/api/ruleproviders.go:20  providers, err := m.DB().ListRuleProviders(r.Context())   ← m.DB() 用法
- internal/api/settings.go:21  s.writeAPIError(w, r, http.StatusInternalServerError, err)   ← 错误返回用法
- internal/api/server.go:236  r.Get("/api/rule-providers", s.handleAPIRuleProviders)   ← 路由注册落点

## 第 1 步：internal/store/store.go 加方法
放在 UpsertRuleProvider（:962 起）函数结束之后：

```go
// SetRuleProviderEnabled 按 id 更新规则集启用状态；行不存在返回 (false, nil)。
func (d *DB) SetRuleProviderEnabled(ctx context.Context, id int64, enabled bool) (bool, error) {
	now := time.Now().UTC()
	res, err := d.db.ExecContext(ctx,
		"UPDATE rule_providers SET enabled = ?, updated_at = ? WHERE id = ?",
		enabled, now, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
```

## 第 2 步：internal/api/ruleproviders.go 加 handler（文件末尾）
文件头 import 若缺 fmt/strconv/strings 就补上：

```go
// handleAPIRuleProviderSetEnabled 处理 PUT /api/rule-providers/{id}/enabled，body {"enabled": true|false}。
func (s *Server) handleAPIRuleProviderSetEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		s.writeAPIError(w, r, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 PUT"))
		return
	}
	m, ok := s.subManagerOr503(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("非法的规则集 id %q", r.PathValue("id")))
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		s.writeAPIError(w, r, http.StatusBadRequest, fmt.Errorf("body 必须是 {\"enabled\": true|false}"))
		return
	}
	found, err := m.DB().SetRuleProviderEnabled(r.Context(), id, *body.Enabled)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, err)
		return
	}
	if !found {
		s.writeAPIError(w, r, http.StatusNotFound, fmt.Errorf("规则集 %d 不存在", id))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
```

## 第 3 步：internal/api/server.go :236 旁加路由
紧跟在 r.Get("/api/rule-providers", ...) 那行下面加一行：

```go
		r.Put("/api/rule-providers/{id}/enabled", s.handleAPIRuleProviderSetEnabled)
```

（缩进对齐 :236 那行——先读上下文再抄。）

## 第 4 步：测试
### internal/store/store_test.go 追加（文件末尾）：
```go
// TestSetRuleProviderEnabled 覆盖启用切换与不存在 id 的 (false, nil) 语义。
func TestSetRuleProviderEnabled(t *testing.T) {
	d, ctx := newTestDB(t)
	if err := d.UpsertRuleProvider(ctx, RuleProvider{Name: "rp1", RawJSON: "{}", Source: "airport:HK", Enabled: true, SortOrder: 1}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	list, err := d.ListRuleProviders(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("seed list: %v len=%d", err, len(list))
	}
	id := list[0].ID

	found, err := d.SetRuleProviderEnabled(ctx, id, false)
	if err != nil || !found {
		t.Fatalf("SetRuleProviderEnabled(existing) = (%v, %v)，应为 (true, nil)", found, err)
	}
	list, _ = d.ListRuleProviders(ctx)
	if list[0].Enabled {
		t.Error("禁用后 Enabled 应为 false")
	}

	found, err = d.SetRuleProviderEnabled(ctx, 9999, true)
	if err != nil || found {
		t.Fatalf("SetRuleProviderEnabled(missing) = (%v, %v)，应为 (false, nil)", found, err)
	}
}
```

### internal/api/ruleproviders_test.go 追加：
- 405：仿 :14 TestRuleProviders_MethodNotAllowed，PUT 换成 POST 打 /api/rule-providers/1/enabled，期望 405。
- 400：合法 id 但 body 不是 JSON（如 "not-json"），期望 400。
- 404：seed 一个 provider（仿 TestRuleProviders_EnabledFilter :62 的构造方式），PUT /api/rule-providers/9999/enabled body {"enabled":true}，期望 404。
- 成功：seed 后 PUT 真实 id body {"enabled":false}，期望 200 且 GET /api/rule-providers?enabled=1 列表不再含它。
（构造 Server 的方式照抄本文件已有用例——先读再写。）

## 第 5 步：自测（原样贴输出给 lead）
```
gofmt -l internal/store/store.go internal/store/store_test.go internal/api/ruleproviders.go internal/api/ruleproviders_test.go
go vet ./...
go test ./internal/store/ ./internal/api/ -count=1
```
预期：gofmt 无输出；vet 无输出；两个包 ok。
注意：go 命令若报 GOCACHE 环境错误，设 GOCACHE=%TEMP%\gocache 再跑。

## 完成定义
- 上述三条命令全绿（输出原文贴给 lead）
- 没动红线外的文件（git status 里只许出现第 0 步列出的文件）
