# SubForge Web UI 重写 · 指导性 Plan

> 状态：待评审　|　读者：实现者（照着写代码，不需要再追问设计）　|　需求来源：m15025 / m15121 / m15610
> 重写判据：**每个数据实体有且只有一个归属页面；订阅页只做“生成 + 交付”。**

---

## 0. 问题定义（为什么重写）

现状 5 页（index / convert / rules / subscription / logs），其中：

| 文件 | 体积 | 问题 |
|---|---|---|
| `web/subscription.html` | 4.6 KB | 只承载“机场/分组/规则/令牌/生成”全部入口 |
| `web/js/subscription.js` | **23.6 KB** | 占全部前端 JS 的 70%，是唯一的大杂烩 |

具体重合：

1. **规则两处可改** — `/rules` 和 `/subscriptions` 都能编辑规则行。
2. **机场两处可填** — `/convert` 的 URL 输入和 `/subscriptions` 的机场列表语义混同。
3. **分组两处可见** — `/subscriptions` 内嵌分组表，与独立分组概念打架。
4. **管线无处管理** — 管线只在 INI 模板 `[pipeline:<name>]` 里，UI 只拿到名字列表，用户改不了。
5. **流量余额断链** — `Airport` 无流量字段，`/subscribe` 不写 `subscription-userinfo`，客户端看不到余额与到期。

---

## 1. 模块边界（9 页，一页一实体）

| # | 页面 | 路由 | 唯一职责 | 拥有的实体 | 明确不做 |
|---|---|---|---|---|---|
| 1 | 概览 | `/` | 只读状态台：节点/规则/分组计数、最近转换、来源健康 | — | 任何写操作 |
| 2 | 转换 | `/convert` | 一次性临时转换（URL/文本/文件 → 产物） | —（无状态，不落库） | 持久化、引用命名实体 |
| 3 | 管线 | `/pipelines` | 命名处理链 CRUD（filter → operate 有序步骤） | `pipeline` 定义 | 选择订阅源 |
| 4 | 机场 | `/airports` | 上游订阅源 CRUD + 流量/配额/到期 + 手动刷新 | `airports` | 分组、规则 |
| 5 | 自建节点 | `/self-nodes` | 本地节点分享链接 CRUD | `self_nodes` | 上游拉取 |
| 6 | 规则 | `/rules` | 规则集（`rule_providers`）+ 规则行（`rules`） | `rules`, `rule_providers` | 代理组 |
| 7 | 分组 | `/groups` | 代理组定义 CRUD | `proxy_groups` | 规则 |
| 8 | 订阅 | `/subscriptions` | 令牌管理 + 生成订阅 URL + **流量聚合策略** | `sub_tokens`, traffic 策略 | 其他实体 CRUD（只读引用 + 跳转链接） |
| 9 | 日志 | `/logs` | 系统日志 + 拉取审计（`fetch_audit`） | —（只读） | 配置 |

### 页面间跳转契约（避免再次重合）

- `/subscriptions` 里出现的机场/分组/规则，**一律渲染为只读 chip + 跳到对应页的链接**，不就地编辑。
- `/convert` 的 URL 输入是**临时值**，不写入 `airports`；若要留存，页面提供“另存为机场”显式动作（POST `/api/airports`）。
- `/airports` 是流量数据的**唯一**写入点（刷新/编辑）。

---

## 2. 数据模型变更

### 2.1 `internal/store/store.go`

`migrate()` 目前是幂等 `CREATE TABLE IF NOT EXISTS`。SQLite 没有 `ADD COLUMN IF NOT EXISTS`，新增：

```go
// addColumnIfMissing 幂等加列：SQLite 无 ADD COLUMN IF NOT EXISTS，
// 用 PRAGMA table_info 判定，避免重复迁移报 duplicate column。
func addColumnIfMissing(db *sql.DB, table, column, decl string) error
```

对 `airports` 追加 7 列：

| 列 | 类型 | 默认 | 语义 |
|---|---|---|---|
| `upload` | INTEGER | 0 | 已用上行字节（上游上报） |
| `download` | INTEGER | 0 | 已用下行字节 |
| `total` | INTEGER | 0 | 配额总字节，0 = 无限/未知 |
| `expire` | INTEGER | 0 | 到期 unix 秒，0 = 未知 |
| `node_count` | INTEGER | 0 | 该源解析出的节点数（“按节点加权”策略用） |
| `traffic_at` | INTEGER | 0 | 上次**成功**采集的 unix 秒 |
| `traffic_err` | TEXT | `''` | 上次采集失败原因（成功时清空） |

新建审计表：

```sql
CREATE TABLE IF NOT EXISTS fetch_audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  airport_id  INTEGER NOT NULL,
  url         TEXT    NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  bytes       INTEGER NOT NULL DEFAULT 0,
  ok          INTEGER NOT NULL DEFAULT 0,
  error       TEXT    NOT NULL DEFAULT '',
  duration_ms INTEGER NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fetch_audit_created ON fetch_audit(created_at DESC);
```

### 2.2 Go struct

```go
// internal/store/store.go
type Airport struct {
    ID              int64
    Name            string
    SubscriptionURL string
    Remarks         string
    Enabled         bool
    SortOrder       int
    // —— M3.7 流量元数据 ——
    Upload     int64
    Download   int64
    Total      int64
    Expire     int64
    NodeCount  int
    TrafficAt  int64  // unix 秒，0 = 从未成功采集
    TrafficErr string
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

type FetchAudit struct {
    ID         int64
    AirportID  int64
    URL        string
    StatusCode int
    Bytes      int64
    OK         bool
    Error      string
    DurationMS int64
    CreatedAt  time.Time
}
```

> ⚠️ `Airport` 结构体改动会波及所有 `SELECT ... FROM airports` 的手写列清单与 `Scan` 调用。实现时必须逐个补齐（`ListAirports` / `GetAirport` / `AddAirport` / `UpdateAirport`），漏一个就是运行时 `sql: expected N destination arguments`。

---

## 3. 后端核心：流量余额采集与聚合

### 3.1 采集响应头（`internal/fetch/fetcher.go`）

`Result` 增加一个字段：

```go
type Result struct {
    ... // 现有字段不动
    // SubscriptionUserinfo 上游 subscription-userinfo 响应头原文。
    // 缓存命中时为空：缓存没存响应头，调用方不得用空值覆盖已存的流量。
    SubscriptionUserinfo string
}
```

`finish()` 里补一行：`SubscriptionUserinfo: resp.Header.Get("Subscription-Userinfo")`。

**验收**：`go test ./internal/fetch/...` 新增一条，httptest 返回该头，断言 `Result.SubscriptionUserinfo` 等于原文。

### 3.2 新包 `internal/traffic`（解析 + 聚合）

```go
package traffic

// Usage 一次订阅流量快照。零值合法（未知）。
type Usage struct {
    Upload   int64 `json:"upload"`
    Download int64 `json:"download"`
    Total    int64 `json:"total"`   // 0 = 无限
    Expire   int64 `json:"expire"`  // unix 秒，0 = 未知
}

// ParseSubscriptionUserinfo 解析 "upload=1; download=2; total=3; expire=4"。
// 带 "Subscription-Userinfo: " 前缀也接受。返回 ok=false 表示这条头不可用。
// 未出现的键保持零值；单个键语法错误只跳过该键，不整体失败。
func ParseSubscriptionUserinfo(s string) (Usage, bool)

// FormatSubscriptionUserinfo 生成可直接写响应头的原文。
func FormatSubscriptionUserinfo(u Usage) string

// —— 第二大块：字段独立策略 ——
type FieldStrategy string

const (
    FieldSum   FieldStrategy = "sum"
    FieldMin   FieldStrategy = "min"
    FieldMax   FieldStrategy = "max"
    FieldAvg   FieldStrategy = "avg"
    FieldFirst FieldStrategy = "first"
    FieldLast  FieldStrategy = "last"
    FieldNone  FieldStrategy = "none" // 该字段不透传
)

// Strategy 四个字段各一个策略，加一个只用于回显的 Preset 名。
type Strategy struct {
    Preset   string        `json:"preset,omitempty"`
    Upload   FieldStrategy `json:"upload"`
    Download FieldStrategy `json:"download"`
    Total    FieldStrategy `json:"total"`
    Expire   FieldStrategy `json:"expire"`
}

// Normalize 填空为默认：upload/download/total = sum，expire = min。
func (s Strategy) Normalize() Strategy

// —— 第一大块：10 种预设（一键铺满四个字段）——
// 前 6 种是纯字段策略；后 4 种必须整体作用于同一机场，所以用 Preset 短路。
func PresetStrategy(name string) (Strategy, bool)

// Aggregate 聚合多机场快照。
// 返回 ok=false 表示按策略不应输出该响应头（全部 none / 无输入）。
func Aggregate(usages []Usage, nodeCounts []int, s Strategy) (Usage, bool)

// ToHeader 把聚合结果写成响应头原文（内部复用 FormatSubscriptionUserinfo）。
func ToHeader(u Usage) string
```

**10 种预设的精确语义**（实现时逐条对照，不许含糊）：

| 预设 | upload | download | total | expire | 说明 |
|---|---|---|---|---|---|
| 求和 `sum` | sum | sum | sum | min | **默认**。到期取最早，避免整体过量承诺 |
| 最小 `min` | min | min | min | min | 保守 |
| 最大 `max` | max | max | max | max | 乐观 |
| 平均 `avg` | avg | avg | avg | avg | 均值 |
| 仅首机场 `first` | first | first | first | first | 按 `sort_order,id` 的启用机场顺序 |
| 仅末机场 `last` | last | last | last | last | 顺序最后 |
| 最低配额 `min_quota` | ← 取 `total` 最小那台机场的全部四字段 | | | | total=0 视为无限，排在最后 |
| 最高配额 `max_quota` | ← 取 `total` 最大那台机场的全部四字段 | | | | |
| 按节点加权 `weighted` | weighted | weighted | weighted | min | 权重 = `nodeCount[i] / ΣnodeCount`，求和后取整 |
| 不透传 `off` | none | none | none | none | 不写响应头 |

**字段级聚合规则**（`Aggregate` 对每个字段应用 `FieldStrategy`）：

- `sum`/`min`/`max`/`avg`：**只在非零输入上计算**。`total=0` 表示无限，参与 min 时应视为 +∞（否则“无限”会被判成“最少”）。
- `avg` = Σ / 非零项数，向下取整。
- `first`/`last`：按输入顺序（调用方保证 = `sort_order, id`）取首个/末个**非零**值；全零则 0。
- `none`：结果字段置 0，并在 `ToHeader` 里**省略该键**。
- `expire` 出现 0（未知）时：`min` 忽略 0；若全为 0 则输出 0。
- 结果 `total > 0` 但 `upload + download > total`：**不裁剪**，如实上报（裁剪会让用户以为上游没超量）。

**预设 → 字段策略的映射**（`PresetStrategy` 返回值）：

```
sum       → {sum, sum, sum, min}
min       → {min, min, min, min}
max       → {max, max, max, max}
avg       → {avg, avg, avg, avg}
first     → {first, first, first, first}
last      → {last, last, last, last}
min_quota → Preset="min_quota"（Aggregate 内部短路，字段值仅回显）
max_quota → Preset="max_quota"
weighted  → Preset="weighted"
off       → {none, none, none, none}
```

**预设组合**（UI 下拉的快捷项，落到 `Strategy`）：

| 组合名 | Strategy |
|---|---|
| 默认 | `{sum, sum, sum, min}` |
| 严格 | `{sum, sum, min, min}` |
| 乐观 | `{sum, sum, max, max}` |
| 独立（仅首机场） | `{first, first, first, first}` |
| 隐藏 | `{none, none, none, none}` |

**测试清单**（`internal/traffic/traffic_test.go`）：
- 解析：正常、缺键、非法值跳过、带前缀、空串。
- 每字段 6 种策略各一条。
- 10 种预设各一条（含 `min_quota` / `max_quota` / `weighted` 短路）。
- 边界：`total=0` 无限参与 min；`expire=0` 被忽略；全 0 输入 → `ok=false`；`avg` 取整。

### 3.3 刷新路径（`internal/subforge/traffic.go` 新建）

```go
// RefreshAirportTraffic 拉取所有启用机场，采集 subscription-userinfo + 节点数，
// 写回 airports，并记一条 fetch_audit。单个机场失败不影响其余。
// 返回每个机场的结果，供 API 回显。
func (m *Manager) RefreshAirportTraffic(ctx context.Context, f *fetch.Fetcher) ([]TrafficRefreshResult, error)

type TrafficRefreshResult struct {
    AirportID int64
    Name      string
    OK        bool
    Usage     traffic.Usage
    NodeCount int
    Err       string
    Duration  time.Duration
}
```

规则：
- 顺序或 `fetch.DefaultConfig().Parallel` 并发（≤4），与 `internal/manifest` 同样节流。
- **缓存命中（`Result.FromCache`）时 `SubscriptionUserinfo` 为空 → 不覆盖已有流量值**，只更新 `TrafficErr=""`。
- 拉取成功但无该响应头 → 流量保持原值，`TrafficErr` 记“上游未提供 subscription-userinfo”。
- 节点数用 `len(parse.DecodeSubscription(body))`（internal/parse/parse.go:121，明文/Base64/Clash YAML 三形态通吃）；解析失败不阻断流量写入，记录 `node_count = 0`。
- 审计表只保留最近 N 条（`const auditRetention = 500`），超出按 `created_at` 倒序裁剪，避免无限增长。

### 3.4 `/subscribe` 写响应头（`internal/api/subscription.go`）

在 `handleSubscribe` 里，`engine.Run` **之前**：

1. 读 `m.ListAirports`，筛 `Enabled && SubscriptionURL != ""`（顺序即聚合顺序）。
2. `s.loadTrafficStrategy(ctx)` 从 config 表读 `traffic.strategy`（JSON），解析失败回落默认。
3. 组装 `[]traffic.Usage` + `[]int`（nodeCounts）。
4. `u, ok := traffic.Aggregate(...)`；`ok` 时 `w.Header().Set("Subscription-Userinfo", traffic.ToHeader(u))`。

响应头名大小写不敏感，写 `Subscription-Userinfo` 即可（Go 会规范化）。

同时补齐 `/subscribe` 现在的三个缺口（顺手做，同一函数）：
- 接 `pipeline` query（默认取 config 里 `subscribe.pipeline`）；
- 接 `config` query（模板名）；
- 接 `include` / `exclude`。

**验收**：新增 `internal/api/subscription_test.go`，httptest 起两个假机场返回不同 `subscription-userinfo`，断言 `/subscribe` 响应头按每种预设正确；再加一条“全部 none → 无该响应头”。

### 3.5 策略持久化

存 config 表，key = `traffic.strategy`，value = `Strategy` 的 JSON。Manager 加两个方法（与现有 `getConfig`/`setConfig` 同风格）：

```go
func (m *Manager) GetTrafficStrategy(ctx context.Context) (traffic.Strategy, error) // 无记录/解析失败 → 默认
func (m *Manager) SetTrafficStrategy(ctx context.Context, s traffic.Strategy) error
```

---

## 4. 后端 API 补全

`internal/api/server.go` 的 `Routes()` 新增：

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| GET | `/api/airports` | 现有 | 增补流量字段 |
| POST | `/api/airports/refresh` | `handleAPIAirportsRefresh` | 拉全部启用机场，返回 `[]TrafficRefreshResult` |
| POST | `/api/airports/{id}/refresh` | 同上（单源） | |
| GET | `/api/traffic/strategy` | `handleAPITrafficStrategy` | 返回当前 Strategy + 预设清单 |
| PUT | `/api/traffic/strategy` | 同上 | 保存 Strategy |
| GET | `/api/traffic/preview` | `handleAPITrafficPreview` | `?strategy=<json>` → 用当前启用机场的**已存**数据试算，返回 Usage + 响应头原文（不落库、不重拉） |
| GET | `/api/audit` | `handleAPIAudit` | `?limit=` 拉取审计 |
| GET/POST/PUT/DELETE | `/api/pipelines...` | `handleAPIPipelines` 扩展 | 管线的**数据库** CRUD（见 §5 管线存储） |

页面路由（`Routes()` 里 `webAssets` 部分）新增：`/airports` `/pipelines` `/groups` `/self-nodes`，对应 html。

### 4.1 管线的存储决策

**现状**：管线只在 INI 模板 `[pipeline:<name>]`，`/api/pipelines` 只返回名字。

**决策**：新增 `pipelines` 表，DB 定义优先于模板（同名时 DB 覆盖模板，UI 标注“来自模板/来自界面”）。

```sql
CREATE TABLE IF NOT EXISTS pipelines (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  steps_json  TEXT NOT NULL DEFAULT '[]',  -- []Step，有序
  enabled     INTEGER NOT NULL DEFAULT 1,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  source      TEXT NOT NULL DEFAULT 'ui',  -- 'ui' | 'template'
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
```

`Step` 复用现有 convert 的 filter/operate 语义（实现时对照 `internal/transform` + `internal/convert` 里管线已支持的动作枚举，**不新造动作**；`steps_json` 里 `{"op":"...","args":{...}}`）。

`/api/pipelines` 合并返回 DB + 模板两条来源；`GET /api/pipelines/{id}` 返回完整步骤；模板来源只读。

> 若实现时发现 `convert` 的管线动作没有可枚举的 op 名（是位置参数式），则 `Step` 改为 `{"raw":"<原始行>"}` 透传，UI 只做有序行编辑 —— 先保证 UI 能管理，再谈结构化。**这一条留给实现者按代码实况二选一，plan 不预设。**

---

## 5. 前端（按 `impeccable` skill 执行）

### 5.1 技术栈（不改动构建方式）

- 纯静态 HTML/CSS/JS，`go:embed` 进二进制（现架构，不引入 npm/打包）。
- 每页一个 html + 一个 js，共享 `app.css` + `app.js`。
- **不引入框架**：现有 5 页是原生 DOM，改成 Vue/React 会带来构建链和体积，与 `.impeccable.md` 的 “instrument-grade、不浪费你时间” 相悖。

### 5.2 设计基调（来自 `.impeccable.md`，实现时逐条遵守）

- 关键词：**calm · precise · dense**；用户 = 个人基础设施 operator。
- 主题：both（跟随系统）+ 手动切换，OKLCH 调色板，surface 60 / text 30 / accent 10。
- 禁令：无侧边条纹 >1px、无渐变文字、无 glassmorphism、无嵌套卡片、无 hero metric、正文不全大写、无紫蓝渐变。
- 4pt 间距刻度；信息密度靠分组间距节奏，不靠装饰。
- 渐进披露：默认只露最常用路径（3 条），详情/原因/diff 由用户展开。
- 空状态 teach，不 scold；错误消息 actionable、不 blaming。
- 优先 inline dialog / sheet，不用 modal（不可避免时才用）。

### 5.3 目录结构（重写后）

```
internal/api/web/
  app.css                # 设计令牌 + 布局 + 组件（重写）
  app.js                 # 侧边栏导航 + 主题 + token 门控 + fetch 封装（重写）
  index.html             # 概览
  convert.html           # 转换
  pipelines.html         # 管线          [新]
  airports.html          # 机场          [新]
  self-nodes.html        # 自建节点      [新]
  rules.html             # 规则（重写）
  groups.html            # 分组          [新]
  subscriptions.html     # 订阅（重写，瘦身）
  logs.html              # 日志（重写，加审计）
  js/
    dashboard.js
    convert.js
    pipelines.js         [新]
    airports.js          [新]
    self-nodes.js        [新]
    rules.js              # 重写：合并原 subscription.js 的规则部分
    groups.js            [新]
    subscriptions.js      # 重写：只有令牌 + 生成 + 策略；删除 23.6KB 大杂烩
    logs.js
  partials/
    dialog.js            # 共享：inline dialog / sheet / toast / confirm（新）
    table.js             # 共享：空状态、加载、错误行（新）
    chips.js             # 共享：只读引用 chip + 跳转（新）
```

**删除**：`js/subscription.js`（内容拆到 rules.js / groups.js / airports.js / subscriptions.js）。

### 5.4 共享组件契约（先写这个，再写页面）

```js
// dbg(app.js) —— 所有页面用同一套，不许各页自己造
ui.dialog({ title, fields, submit })   // inline dialog，返回提交结果
ui.confirm({ title, body, danger })    // 破坏性操作二次确认
ui.toast({ kind: 'ok'|'warn'|'err', text })
ui.table({ columns, rows, empty, loading })
ui.chip({ text, href })                // 只读引用 + 跳转
api.get(path) / api.post / api.put / api.del   // 统一错误 → toast
fmt.bytes(n)                           // 0 → "无限"，否则 GiB/MiB
fmt.expire(unix)                       // 0 → "未知"，否则 YYYY-MM-DD + 剩余天数
```

### 5.5 每页规格（验收点）

**1) 概览 `/`**
- 顶部 4 个 stat：节点数 / 规则行数 / 分组数 / 启用机场数。**不做 hero 大数字**，做紧凑的 label + value 行。
- 来源健康表：机场名、上次采集时间、流量进度条（used = upload+download，total>0 时显示百分比）、到期剩余天数、最近错误。
- 最近转换历史（现有 `/api/history`）。
- 空状态：没有任何机场时，给“去添加第一个机场 →”的链接。
- 验收：所有数字来自 `/api/stats`；流量列在无数据时显示“—”，不显示 0%。

**2) 转换 `/convert`**
- 唯一临时转换入口。输入：URL 列表 / 粘贴文本 / 上传文件；目标格式；可选管线。
- 输出：预览、下载、节点数、warnings（现有响应头已提供）。
- “另存为机场”按钮：POST `/api/airports`。
- 验收：不写库（除显式“另存为机场”）；切换目标格式即时重新请求。

**3) 管线 `/pipelines`**
- 列表：名字、步骤数、来源（界面/模板）、启用开关。
- 编辑：有序步骤列表，可增删排序；每步选 op + 参数。
- 模板来源只读，标 badge。
- 验收：保存后 `/convert` 与 `/subscribe` 的 `pipeline=` 都能选到。

**4) 机场 `/airports`** ← 流量数据的唯一归属页
- 列表列：启用、名称、URL（截断 + 悬浮全量）、备注、**流量**（used/total 进度条）、**到期**（日期 + 剩余天数）、上次采集、错误。
- 行内操作：编辑、刷新（单源）、删除。
- 顶部操作：新增、**刷新全部**。
- 编辑表单：名称、订阅 URL、备注、启用。
- 刷新中：按钮转 loading，行内显示 spinner；完成后 toast + 就地更新。
- 验收：POST `/api/airports/refresh` 后列表数值变化；失败源显示红色错误文本且不丢旧值。

**5) 自建节点 `/self-nodes`**
- 列表：启用、名称（无则显示链接摘要）、链接。
- CRUD 同机场模式。
- 验收：新增后 `/subscribe` 产物含该节点。

**6) 规则 `/rules`**
- 两个区域：**规则集**（`rule_providers`）与**规则行**（`rules`）。
- 规则行列表：启用、类型、参数、命中分组（下拉选分组名）、no-resolve。
- 从订阅导入（现有能力）。
- 验收：规则行改完立即影响 `/subscribe` 产物；不再需要去订阅页。

**7) 分组 `/groups`**
- 列表：名称、类型、成员模式（regex）、测速 URL、间隔。
- CRUD inline dialog。
- 验收：分组名出现在规则页的下拉；组变更影响产物。

**8) 订阅 `/subscriptions`** ← 瘦身后的核心页
- 区块 A：**令牌**（生成/删除/复制），显示完整订阅 URL（带 token）。
- 区块 B：**流量聚合策略**
  - 第一大块：10 个预设的按钮/下拉。
  - 第二大块：4 个字段各自的下拉（sum/min/max/avg/first/last/none）。
  - 选预设时铺满四字段；手动改任一字段 → Preset 变 “自定义”。
  - **实时预览**：调 `/api/traffic/preview`，显示 “upload=X download=Y total=Z expire=D” 与生成的响应头原文。
- 区块 C：**引用概览**（只读 chips + 跳转）：启用的机场、生效的分组、生效的规则数。
- 区块 D：生成按钮 → 显示产物状态（节点数、格式、warnings），并提供“复制订阅地址”。
- 验收：改策略 → 预览即时变；用 `curl -I` 拉真实订阅，响应头与预览一致。

**9) 日志 `/logs`**
- Tab 1 系统日志（现有 `/api/logs`）。
- Tab 2 拉取审计（`/api/audit`）：时间、机场、状态码、字节、耗时、结果。
- 验收：刷新机场后审计新记录出现。

---

## 6. 实施顺序与提交点

每个阶段**独立可构建、可验证、可提交**。阶段间用提交点隔开，不攒大提交。

| 阶段 | 内容 | 验证 | 提交信息 |
|---|---|---|---|
| **P1** | traffic 包（解析 + 聚合 + 预设） | `go test ./internal/traffic/...` 全绿 | `feat(traffic): 流量元数据解析与聚合策略` |
| **P2** | fetch 采集响应头 | `go test ./internal/fetch/...` | `feat(fetch): 采集 subscription-userinfo 响应头` |
| **P3** | store schema 迁移 + Airport struct + 审计表 | `go test ./internal/store/...`；旧库能打开 | `feat(store): 机场流量字段与拉取审计表` |
| **P4** | Manager 刷新路径 + 策略读写 + config key | `go test ./internal/subforge/...` | `feat(subforge): 机场流量刷新与策略持久化` |
| **P5** | /subscribe 写响应头 + 补 pipeline/config/include/exclude | `go test ./internal/api/...` | `feat(api): 订阅响应头透传流量余额` |
| **P6** | pipelines 表 + API CRUD | `go test ./internal/api/...` | `feat(api): 管线数据库化管理` |
| **P7** | 前端共享层（app.css / app.js / partials） | 手工过 9 页骨架 | `feat(web): 重写共享样式与组件层` |
| **P8** | 逐页实现（按 1→9 顺序，每页一提交或两页一提交） | 每页验收点 | `feat(web): <页名> 页面` |
| **P9** | 删除 `js/subscription.js`，清理死代码 | `go build ./...` + 全页回归 | `refactor(web): 移除订阅页大杂烩脚本` |
| **P10** | 全量回归 + 文档 | `go build/vet/test ./...` | `docs: Web UI 重写与流量聚合说明` |

**硬性要求**（每阶段都适用）：
- `go build ./...`、`go vet ./...`、`go test ./...` 三条全过才算完成（CI 就是这三条，vendored 模式）。
- 提交信息文件以 `.txt` 结尾，不用 `--no-verify`。
- 每个后端阶段带单元测试；每个前端页面带手工验收记录。

---

## 7. 全局验收清单

- [ ] 9 个页面路由可达，无 404；侧边栏高亮当前页正确。
- [ ] **无重合**：规则只能在 `/rules` 改；机场只能在 `/airports` 改；分组只能在 `/groups` 改；`/subscriptions` 里这三者均为只读 chip + 跳转。
- [ ] 流量：`curl -I "\$SUB_URL" ` 返回 `Subscription-Userinfo`；10 种预设 + 4 字段独立策略 + 5 个组合全部可选且语义符合 §3.2 表。
- [ ] 流量边界：`total=0` 无限不被判成最少；全 none 时不输出响应头；`expire=0` 被忽略。
- [ ] 缓存命中不覆盖已存流量值。
- [ ] 拉取审计：每次刷新有记录，超 500 条自动裁剪。
- [ ] 主题：light / dark / 跟随系统三种都正常，无硬编码颜色。
- [ ] 响应式：≥1280px 三栏不挤，≤768px 侧边栏收起。
- [ ] 原有功能不回退：`/sub`、`/render`、`/profile`、`/getruleset`、`/inspect`、`/api/convert`、`/api/diff` 全部行为不变。
- [ ] `go build ./... && go vet ./... && go test ./...` 全绿。
- [ ] 二进制内嵌资源齐全（`go:embed` 无遗漏），单文件可运行。

---

## 附：需要实现者二次确认的两个开放点

1. **管线步骤的结构化程度** — 见 §4.1 末尾。取决于 `internal/convert` 里管线动作是否可枚举。若不可枚举，退化为“有序原始行编辑”，不影响本 plan 其余部分。
2. **`subscription-userinfo` 键名大小写** — mihomo/clash 读 `upload`/`download`/`total`/`expire` 小写；上游可能有 `Upload` 等变体。解析时统一 `strings.ToLower` 键名。
