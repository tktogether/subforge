# 订阅产物预生成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** /subscribe 默认组合拉取命中预生成产物（毫秒级返回），改动后下一次拉取语义与今天完全一致。

**Architecture:** 全局产物版本号（config 表计数器，store 写路径 bump）判定产物新旧；后台重建器轮询版本、按令牌预编译产物入库；拉取路径先查产物命中（版本一致即返回，附 ETag/304），未命中共用同一构建函数实时编译并回填；/subscribe 加 gzip。

**Tech Stack:** Go 1.26、chi、SQLite（store 层直写 SQL）、现有 fetch/convert/render 管线（不动引擎）。

**Spec:** docs/superpowers/specs/2026-11-15-subscribe-pregeneration-design.md

## Global Constraints

- 现有实时编译路径语义逐字节保留：失败策略、部分降级、最终组改写、响应头全部不变（spec 反目标）。
- 不修改 `internal/convert/engine.go`、`internal/fetch`、`internal/render`（预生成只是「提前做的实时编译」）。
- 机场 traffic 列更新（upload/download/total/expire/traffic_at/node_count/traffic_err）**不 bump**；`PutSourceCache` 不 bump。
- bump 自身写 config 表但必须用独立 UPSERT 语句，不得经由 config 写方法（防递归）。
- 版本守卫 upsert：`excluded.version >= sub_artifacts.version` 才覆盖。
- 本机验证命令用 `go test ./...`（本机无 make；Go 可执行文件在 PATH 中）。
- 每个任务独立提交，提交信息按仓库惯例 `feat:/test:/refactor: 主题`（中文一句话）。

## Review Focus

- **上游故障窗口的失败语义**：failure_policy=error 机场失败时，产物路径必须在「重建一轮」内回落到 502（删产物）；测试要覆盖「重建后产物已删除 → 拉取 502」。
- **改动后第一次拉取的正确性**：任何 DB 写（含 traffic 除外全部）后拉取必须看到新配置——靠版本比对 miss；测试要覆盖「改规则 → 拉取 → 新规则在产物/实时输出中」。
- **令牌删除后的孤儿产物**：删令牌必须连产物一起删；测试覆盖「建令牌→拉取回填→删令牌→产物表无该行」。
- **温度=0 的既有语义**：`fetch.refresh_minutes=0` 时产物命中禁用、重建跳过，行为与今天一致；测试覆盖。
- **304 与 gzip 并存**：带 Accept-Encoding: gzip 的 If-None-Match 请求回 304（无 body）；不带 gzip 的客户端回 200 明文。测试覆盖。

---

### Task 1: store 层产物表、版本号原语与产物 CRUD

**Files:**
- Create: `internal/store/artifacts.go`
- Modify: `internal/store/store.go`（migrate() 加表；DeleteSubToken 级联删产物）
- Test: `internal/store/artifacts_test.go`

**Interfaces:**
- Produces（Task 2、3 依赖）:
  - `type SubArtifact struct { Token string; ProfileID int64; Version int64; Body []byte; ETag string; GeneratedAt time.Time }`
  - `func (d *DB) ArtifactVersion(ctx context.Context) (int64, error)` — config 键 `subscribe.artifact_version`，键缺失返回 0
  - `func (d *DB) BumpArtifactVersion(ctx context.Context) (int64, error)` — 原子 +1，返回新值（首贴为 1）
  - `func (d *DB) GetSubArtifact(ctx context.Context, token string) (*SubArtifact, error)` — 不存在返回 (nil, nil)
  - `func (d *DB) SaveSubArtifact(ctx context.Context, a SubArtifact) error` — 版本守卫 upsert
  - `func (d *DB) DeleteSubArtifact(ctx context.Context, token string) error`
- 常量：`ArtifactVersionConfigKey = "subscribe.artifact_version"`（放 artifacts.go）

- [ ] **Step 1: 写失败测试** `internal/store/artifacts_test.go`（沿用 store_test.go 现有开库方式）：
  - `TestArtifactVersion_Bump`：新库 ArtifactVersion()==0；Bump 三次 → 3
  - `TestSubArtifact_CRUD`：Save → Get 返回等值结构体；Delete 后 Get 返回 (nil, nil)
  - `TestSubArtifact_VersionGuard`：存 version=5 后存 version=3 → 仍读到 version=5 的旧行；存 version=6 → 覆盖
  - `TestDeleteSubToken_CascadesArtifact`：建令牌、存产物、DeleteSubToken → 产物行消失
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/store/ -run TestArtifact -v` → 编译失败（类型未定义）
- [ ] **Step 3: 实现**：migrate() 的 stmts 追加 `CREATE TABLE IF NOT EXISTS sub_artifacts (token TEXT PRIMARY KEY, profile_id INTEGER NOT NULL DEFAULT 0, version INTEGER NOT NULL, body BLOB NOT NULL, etag TEXT NOT NULL, generated_at INTEGER NOT NULL)`；DeleteSubToken 方法内在删令牌的同函数里追加 `DELETE FROM sub_artifacts WHERE token = ?`；Bump 用 `INSERT INTO config(key,value,updated_at) VALUES(?,?,CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value = CAST(value AS INTEGER) + 1, updated_at = CURRENT_TIMESTAMP` 再 `SELECT value` 读回（两条语句，防递归靠独立 SQL 不走 config 写方法）
- [ ] **Step 4: 跑测试确认通过**：`go test ./internal/store/ -v`
- [ ] **Step 5: 提交** `git add internal/store/artifacts.go internal/store/artifacts_test.go internal/store/store.go && git commit -m "feat(store): 订阅产物表与版本号原语"`

### Task 2: store 层写路径 bump 接线

**Files:**
- Modify: `internal/store/` 下所有对下列表做 INSERT/UPDATE/DELETE 的 DB 方法：airports、self_nodes、proxy_groups、rules、rule_providers、profiles、sub_tokens、config（设置写）、scripts、pipelines、source_pipelines
- Test: `internal/store/artifacts_bump_test.go`

**Interfaces:**
- Consumes: Task 1 的 `BumpArtifactVersion`
- 例外清单（**不** bump）：机场 traffic 更新方法（upload/download/total/expire/node_count/traffic_at/traffic_err 列）、`PutSourceCache`、Task 1 的 bump 自身

- [ ] **Step 1: 写失败测试**（表驱动，每个变更方法一条用例）：对每个要接线的 DB 方法——先记 ArtifactVersion()，执行变更方法，断言版本 +1；外加两条负例：traffic 更新方法、PutSourceCache 执行后版本不变。用 grep 找全方法名：`grep -n "INSERT INTO\|UPDATE \|DELETE FROM" internal/store/*.go` 逐表核对（profiles.go、rules_batch.go、rules_delete.go、layers.go 都要看）
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/store/ -run TestBump -v` → FAIL
- [ ] **Step 3: 接线**：每个变更方法在主语句执行成功后调用 `_, _ = d.BumpArtifactVersion(ctx)`（bump 失败不阻断业务写入，错误吞掉只留注释说明「漏 bump 退化为 TTL 自愈」）
- [ ] **Step 4: 跑测试确认通过**：`go test ./internal/store/ -v`
- [ ] **Step 5: 提交** `git commit -am "feat(store): 产物相关表变更联动版本号"`

### Task 3: Manager 包装、artifact_mode 设置与 store 外 bump 点

**Files:**
- Create: `internal/subforge/artifacts.go`
- Modify: `internal/subforge/settings.go`（设置定义表加一行）、`internal/subforge/rulesources.go`、`internal/api/template_api.go`、`internal/api/override_api.go`
- Test: `internal/subforge/artifacts_test.go`、settings_test.go 里补一条模式校验用例

**Interfaces:**
- Consumes: Task 1 的 store 原语
- Produces（Task 5、7 依赖）:
  - `func (m *Manager) ArtifactVersion(ctx context.Context) (int64, error)`
  - `func (m *Manager) BumpArtifactVersion(ctx context.Context) error`
  - `func (m *Manager) GetSubArtifact(ctx context.Context, token string) (*store.SubArtifact, error)`
  - `func (m *Manager) SaveSubArtifact(ctx context.Context, a store.SubArtifact) error`
  - `func (m *Manager) DeleteSubArtifact(ctx context.Context, token string) error`
  - `func (m *Manager) ArtifactsEnabled(ctx context.Context) bool` — `subscribe.artifact_mode` 开（默认）**且** `SourceRefreshInterval > 0`；读配置出错按禁用处理
- 设置键：`ArtifactModeKey = "subscribe.artifact_mode"`，SettingKindBool，显示名「订阅产物缓存」，默认开（未配置 = 开）
- store 够不着的 bump 点（文件/落盘路径，Manager 层显式调 `m.db.BumpArtifactVersion`，失败只记日志不阻断）：
  - `recordCurrentVersions`（rulesources.go:298，规则源当前版本指针是文件操作）末尾 bump 一次——`UpdateRuleSources` 等全量更新都经它
  - `RollbackRuleSource` 成功 setCurrentVersion 后 bump
  - template_api.go / override_api.go 的文件保存 handler 落盘成功后调 `m.BumpArtifactVersion(ctx)`（Task 8 的 INI 热重载另算）

- [ ] **Step 1: 写失败测试**：ArtifactsEnabled 默认 true；写入 artifact_mode=0 → false；refresh_minutes=0 → false；Bump/Get/Save/Delete 透传等值
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/subforge/ -run Artifact -v` → 编译失败
- [ ] **Step 3: 实现**：Manager 方法透传 m.db；ArtifactsEnabled 里 bool 设置读取沿用现有 bool 设置解析（settings.go 的 bool 路径），未配置 = true；加上面三处 store 外 bump 点
- [ ] **Step 4: 跑测试确认通过**：`go test ./internal/subforge/ ./internal/api/ -v`
- [ ] **Step 5: 提交** `git commit -am "feat(subforge): 产物缓存 Manager 门面、开关设置与文件路径 bump"`

### Task 4: handleSubscribe 抽取 buildSubscription（纯重构）

**Files:**
- Modify: `internal/api/subscription.go`（handleSubscribe 整个前段）

**Interfaces:**
- Produces（Task 5、7 依赖）:
  - `type subBuild struct { Name string; DefaultCombo bool; Params convert.Params; UsedAirports []subforge.Airport }`
  - `func (s *Server) buildSubscription(ctx context.Context, token string, query url.Values) (b *subBuild, cleanup func(), status int, err error)` — status==0 表示成功；**令牌查找 404、方案停用 400、参数归并、来源/分组/规则/规则源收集、临时文件、Params 组装全部并入**（实时与预生成共用同一前段，防两处漂移）；cleanup 关闭自建节点临时目录，调用方必须 defer
- DefaultCombo 判定：query 的 target/pipeline/config/include/exclude 五个参数全部为空

- [ ] **Step 1: 先补一条特征测试**（防重构变形）到 subscribe 相关测试文件：同一 token 两次拉取默认组合，响应头 profile-title 与 body 相同（`go test ./internal/api/ -run TestSubscribe -v` 先确认现状全绿）
- [ ] **Step 2: 抽取**：handleSubscribe 从 token 查找起至 Params 组装结束整体移入 buildSubscription；错误出口从 s.writeAPIError 改为返回 (status, err)——沿用现有各分支的状态码；`defer os.RemoveAll(tmpDir)` 改为 cleanup 闭包由调用方 defer；handler 侧剩余：build → engine.Run → 失败策略 → 响应头 → 写出
- [ ] **Step 3: 跑全部 api 测试确认通过**：`go test ./internal/api/ -v`（现有用例必须全绿、无行为变化）
- [ ] **Step 4: 提交** `git commit -am "refactor(api): 抽取订阅构建函数 buildSubscription"`

### Task 5: 拉取命中路径、惰性回填与 ETag/304

**Files:**
- Create: `internal/api/artifact.go`
- Modify: `internal/api/subscription.go`（handleSubscribe 尾段）
- Test: `internal/api/artifact_test.go`

**Interfaces:**
- Consumes: Task 3 的 Manager 门面、Task 4 的 buildSubscription
- Produces（Task 7 依赖）:
  - `func computeArtifactETag(version int64, token string) string` — 带引号的 `"<hex>"`，hex=sha256(version + "\n" + token)（版本号已捕获全部正文输入：bump 清单 + 内容变化才 bump + 失败 bump；不同 query 组合按 URL 各自缓存，不串）
  - `func etagMatch(ifNoneMatch, etag string) bool` — 逗号分隔列表与 `*`
  - `func (s *Server) trafficHeaderValue(ctx context.Context, usedAirports []subforge.Airport) string` — 现 handleSubscribe 的流量聚合段原样搬入
  - `func requiredFailedSources(usedAirports []subforge.Airport, result convert.Result) []string` — 现 requiredFailed 循环搬入
- handleSubscribe 尾段新流程：build 成功后读 `ArtifactVersion`；`DefaultCombo && s.subManager.ArtifactsEnabled(ctx)` 且产物存在且 `art.Version == cur` → 现算响应头（trafficHeaderValue + profile-title + Content-Disposition）+ `ETag`，`If-None-Match` 命中回 304，否则 200 回 `art.Body`，记命中日志，return；engine.Run 成功且 `requiredFailedSources` 为空且 DefaultCombo 且 enabled → `SaveSubArtifact`（Version=cur、ETag=computeArtifactETag）
- 产物 body 存的是实时编译的原始 body（含时间戳行），命中直接回——同一版本同一输入下与实时编译除时间戳行外逐字节一致

- [ ] **Step 1: 写失败测试**（用 uiSubServer/fixtureURL 现有装配）：
  - `TestSubscribeArtifactHit`：默认组合拉取两次，第二次命中——两次 body 除「生成时间」行外相同（剥离该行再比），第二次响应带 ETag
  - `TestSubscribeArtifact304`：第一次拿 ETag，带 If-None-Match 再拉 → 304 无 body；If-None-Match: `*` → 304
  - `TestSubscribeArtifactInvalidatedOnChange`：拉取回填后改一条规则（走 mgr 方法），再拉 → miss（新规则出现在 body），且产物表版本已更新
  - `TestSubscribeArtifactQueryOverrideMiss`：带 target= 覆盖参数拉取 → 不回填产物（产物表行数不变）
  - `TestSubscribeArtifactModeOff`：artifact_mode=0 → 无 ETag 头、两次都走实时路径
  - `TestSubscribeArtifactRefreshZero`：refresh_minutes=0 → 无 ETag 头
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/api/ -run TestSubscribeArtifact -v` → FAIL
- [ ] **Step 3: 实现** artifact.go 与 handleSubscribe 改造
- [ ] **Step 4: 跑全部 api 测试**：`go test ./internal/api/ -v`。判定规则：若现有用例因「改上游 fixture 内容但未动 DB」而失败，那是测试自身没模拟真实失效流——在测试的变更步骤后补一次 mgr.BumpArtifactVersion（模拟真实链路里 refresh 任务的动作），不许为过测试削弱命中判定
- [ ] **Step 5: 提交** `git commit -am "feat(api): /subscribe 产物命中与惰性回填"`

### Task 6: /subscribe gzip

**Files:**
- Modify: `internal/api/server.go`（/subscribe 路由注册处）
- Test: `internal/api/artifact_test.go` 补用例

- [ ] **Step 1: 写失败测试** `TestSubscribeGzip`：手设 `Accept-Encoding: gzip` 请求（Go 客户端此时不自动解压）→ 响应 `Content-Encoding: gzip`，`gzip.NewReader` 能解出 YAML；不带该头 → 无 Content-Encoding
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/api/ -run TestSubscribeGzip -v` → FAIL
- [ ] **Step 3: 实现**：路由改为 `r.With(chimw.Compress).Get("/subscribe", s.handleSubscribe)`（import chi middleware）
- [ ] **Step 4: 跑测试确认通过**：`go test ./internal/api/ -run TestSubscribeGzip -v`
- [ ] **Step 5: 提交** `git commit -am "feat(api): /subscribe 响应 gzip 压缩"`

### Task 7: 后台重建器

**Files:**
- Create: `internal/api/artifact_rebuild.go`
- Modify: `cmd/subforge/serve.go`（SetSubManager 之后启动）
- Test: `internal/api/artifact_rebuild_test.go`

**Interfaces:**
- Consumes: Task 3 门面、Task 4 buildSubscription、Task 5 requiredFailedSources/computeArtifactETag
- Produces:
  - `func (s *Server) StartArtifactRebuilder(ctx context.Context)` — subManager 为 nil 时空操作；goroutine 轮询
  - `func (s *Server) rebuildArtifacts(ctx context.Context)` — 单轮，测试直接调用
  - Server 字段 `artifactPoll time.Duration`（零值 = 5s），测试可设短
- 轮询语义（设计书 wake channel 的等价简化）：每轮先看 `ArtifactsEnabled`，关则跳过；读当前版本；`ListSubTokens` 逐个：绑定停用方案跳过；产物缺失、version != cur、或年龄 > SourceRefreshInterval → `rebuildOneToken`
- `rebuildOneToken`：buildSubscription（空 query）→ status!=0 → DeleteSubArtifact + 日志；engine.Run（拉取预算 = s.cfg.SubscribeTimeout 的独立 context）→ err 或 `requiredFailedSources` 非空 → DeleteSubArtifact + 日志；成功 → SaveSubArtifact（Version=cur、ETag=computeArtifactETag(cur, token)）；defer build 的 cleanup
- 启动即跑一轮（预热），然后按 artifactPoll 轮询；serve.go 在 SetSubManager 与调度器装配之后调 `srv.StartArtifactRebuilder(ctx)`

- [ ] **Step 1: 写失败测试**：
  - `TestRebuildArtifacts_WarmsAllTokens`：两个令牌（一个绑方案一个不绑）→ rebuildArtifacts 后产物表两行、body 剥时间戳行后与实时拉取一致
  - `TestRebuildArtifacts_DeletesOnRequiredFailure`：机场 failure_policy=error 指向不可达地址 → rebuild 后该令牌无产物；再拉取 → 502
  - `TestRebuildArtifacts_PicksUpVersionBump`：先 rebuild，改分组 → rebuildArtifacts 再次执行 → 产物 version == 当前版本
  - `TestRebuildArtifacts_DisabledModeSkips`：artifact_mode=0 → 产物表仍空
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/api/ -run TestRebuild -v` → FAIL
- [ ] **Step 3: 实现** artifact_rebuild.go 与 serve.go 接线
- [ ] **Step 4: 跑测试确认通过 + 全量**：`go test ./internal/api/ ./cmd/... -v`
- [ ] **Step 5: 提交** `git commit -am "feat(api): 订阅产物后台重建器"`

### Task 8: source-refresh 任务联动版本号

**Files:**
- Modify: `internal/scheduler/jobs.go`（SourceRefreshJob.Run）
- Test: `internal/scheduler/jobs_test.go` 补用例

**Interfaces:**
- Consumes: Task 1 的 `mgr.db.BumpArtifactVersion`（经 mgr 透传或直接 m.db 调用，取 jobs.go 现有 mgr 用法）
- 语义（设计书 §1「后台」行）：刷新**成功且内容确实变化**（无旧缓存或新旧原文不等）→ bump 一次（整轮最多一次，不在并发 goroutine 里各自 bump）；**刷新失败**（res.Err != nil）→ 也 bump（失败语义收敛靠它，见 spec §4）；304/无变化/跳过的源不 bump

- [ ] **Step 1: 写失败测试**（沿用 jobs_test.go 现有的 SourceRefreshJob 装配方式；版本断言一律用相对值 +1）：
  - 首轮拉取（无缓存）→ 版本 +1
  - TTL 内二轮（跳过）→ 版本不变
  - 上游内容变化且缓存过期 → +1；内容未变且缓存过期 → 不变
  - 上游失败 → +1
- [ ] **Step 2: 跑测试确认失败**：`go test ./internal/scheduler/ -run TestSourceRefresh -v` → FAIL
- [ ] **Step 3: 实现**：Run 里取过期判断时的旧缓存内容（现在 `_` 丢弃了），轮末统计 changed/failed 数，>0 时 bump 一次
- [ ] **Step 4: 跑测试确认通过**：`go test ./internal/scheduler/ -v`
- [ ] **Step 5: 提交** `git commit -am "feat(scheduler): 机场原文变化与刷新失败联动产物版本"`

### Task 9: INI 热重载 bump

**Files:**
- Modify: `cmd/subforge/serve.go`（watchConfig）
- Test: `cmd/subforge/serve_test.go` 补用例

- [ ] **Step 1: 写失败测试**：watchConfig 回调在成功重载后调用注入的 bump 函数（解析成功但内容未变的轮询 tick 不触发——runtime.Watch 只在文件变化时回调，若实测发现启动首拍也回调，断言放宽为「至少一次」并在注释说明预热无害）
- [ ] **Step 2: 跑测试确认失败**：`go test ./cmd/subforge/ -run TestWatchConfig -v` → FAIL
- [ ] **Step 3: 实现**：watchConfig 增加非 nil 安全的 `onReload func()` 参数，`snap.Store(next)` 成功后调用；serve.go 闭包里判 mgr != nil 时调 `mgr.BumpArtifactVersion(ctx)`（错误只记 stderr）
- [ ] **Step 4: 跑测试确认通过**：`go test ./cmd/... -v`
- [ ] **Step 5: 提交** `git commit -am "feat(serve): INI 热重载联动产物版本"`

### Task 10: 全量验证

**Files:** 无新文件

- [ ] **Step 1: 全量测试**：`go test ./...` → 全绿
- [ ] **Step 2: 覆盖率自查**：`go test -cover ./internal/store/ ./internal/subforge/ ./internal/api/` → 新增逻辑（artifact.go、artifact_rebuild.go、store/artifacts.go）无显著未覆盖分支（仓库门禁要求逻辑层 100%，未覆盖的补测试）
- [ ] **Step 3: go vet**：`go vet ./...` → 无新增告警
- [ ] **Step 4: 按需提交** 若前序任务有遗漏修正：`git commit -am "test: 补产物缓存用例"`
