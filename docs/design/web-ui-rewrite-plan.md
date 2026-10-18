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

## 1. 模块边界（13 页 / 5 域，一页一实体）

> **为什么从 9 页扩到 13 页**：初版 9 页只覆盖了「现在已经有 HTTP 面」的能力，把后端**已经写好、但只挂在 CLI 上**的 5 项（manifest 聚合 / MRS 生成 / 测活 / 定时告警 / Rule-Bot）和**完全没有出口**的 2 项（goja 脚本 / 上游规则导入）静默漏掉了。漏掉的代价不是"少个页面"，而是这些能力永远只有命令行用户能用。本节按**实体**而不是按"现在有没有 API"划页。

| # | 域 | 页面 | 路由 | 唯一职责 | 拥有的实体 | 明确不做 |
|---|---|---|---|---|---|---|
| 1 | 转换 | 概览 | `/` | 只读状态台：计数、来源健康、最近转换 | — | 任何写操作 |
| 2 | 转换 | 转换 | `/convert` | 一次性临时转换（URL/文本/文件 → 产物） | —（无状态，不落库） | 持久化、引用命名实体 |
| 3 | 转换 | 管线 | `/pipelines` | 命名处理链 CRUD（有序步骤） | `pipelines` | 脚本源码、订阅源 |
| 4 | 转换 | 脚本 | `/scripts` | **goja 脚本库 CRUD + 试运行** | `scripts` | 管线步骤顺序、线上执行开关 |
| 5 | 来源 | 机场 | `/airports` | 上游订阅源 CRUD + 流量/配额/到期 + 刷新 + **测活** | `airports` | 分组、规则、节点编辑 |
| 6 | 来源 | 自建节点 | `/self-nodes` | 本地节点分享链接 CRUD | `self_nodes` | 上游拉取 |
| 7 | 规则 | 规则 | `/rules` | 规则行 + 规则集 + **上游导入候选** + **Rule-Bot 建议** | `rules`, `rule_providers` | 规则源文件生命周期、代理组 |
| 8 | 规则 | 规则源 | `/rulesets` | **manifest 聚合 + MRS 生成 + 版本/回滚 + 来源健康** | 规则源清单 + 版本目录 | 规则行增删（在 `/rules`） |
| 9 | 输出 | 分组 | `/groups` | 代理组定义 CRUD | `proxy_groups` | 规则 |
| 10 | 输出 | 订阅 | `/subscriptions` | 令牌 + 订阅 URL + **流量聚合策略** | `sub_tokens` + traffic 策略 | 其他实体 CRUD（只读引用 + 跳转） |
| 11 | 运维 | 任务 | `/tasks` | **定时任务总览 + 告警通道 + 执行历史** | 告警通道配置、任务执行记录 | 各任务的业务参数（属各自页面） |
| 12 | 运维 | 设置 | `/settings` | **INI 模板 + 分层覆盖 + node_pref + 全局开关** | 模板/覆盖文本、开关项 | 规则行、分组、机场 |
| 13 | 运维 | 日志 | `/logs` | 系统日志 + 拉取审计 | —（只读） | 配置 |

### 页面间跳转契约（避免再次重合）

1. `/subscriptions` 与 `/tasks` 里出现的机场/分组/规则/脚本，**一律渲染为只读 chip + 跳到对应页的链接**，不就地编辑。
2. `/convert` 的 URL 输入是**临时值**，不写入 `airports`；要留存必须走显式"另存为机场"（POST `/api/airports`）。
3. `/airports` 是流量数据与测活结果的**唯一**写入点；`/rulesets` 是规则源版本与 MRS 缓存的**唯一**写入点；`/scripts` 是脚本源码的**唯一**写入点。
4. `/rules` 里的导入候选与 Rule-Bot 建议都是**队列**：确认后才成为规则行；未确认的候选**不属于**渲染输入。
5. 全局模板与覆盖只在 `/settings` 编辑；`/rules` **不再**提供覆盖编辑器（初版把覆盖放在 `/rules` 是既存重合点之一）。
6. 脚本开关只在 `/settings` 改（安全项集中放置），`/scripts` 只读显示状态 + 跳转。

### 1.1 能力覆盖矩阵（防再次静默漏项）

**用法**：下表每一行是仓库里一个**已存在的能力**（含只有 CLI 的）。右列必须是恰好一个页面编号；出现"无"即视为本 plan 不完整，必须先补设计再动手。

| # | 后端能力 | 代码位置 | 当前出口 | 归属页 |
|---|---|---|---|---|
| A1 | 全协议解析/渲染、`/sub` `/render` `/profile` `/inspect` `/api/convert` `/api/diff` | `internal/convert`, `internal/render`, `internal/api/handlers.go` | HTTP ✅ | 2 |
| A2 | 规则集转换 `/getruleset` | `internal/api/handlers.go:119` | HTTP ✅ | 2 |
| A3 | target 家族 + auto-UA | `internal/api/auto.go`, `internal/api/web.go:147` | HTTP ✅ | 2 |
| A4 | INI 管线解析（`[pipeline:<name>]`，10 种步骤） | `internal/config/pipeline.go` | HTTP 只读名字 | 3 |
| A5 | **goja 脚本**（`filter.script` / `op.script`，内联 `src=`） | `internal/transform/script.go`, `internal/config/pipeline.go:88,114` | **无** | **4** |
| A6 | 机场 CRUD + 订阅 URL | `internal/api/subscription.go:68` | HTTP ✅ | 5 |
| A7 | **subscription-userinfo** 流量采集/聚合 | 待建 `internal/traffic`（§3.1–3.5） | **无** | 5 + 10 |
| A8 | **测活**（并发探测，默认关闭） | `internal/subscheck/subscheck.go` + `cmd/subforge/subscheck.go` | **仅 CLI** | 5 |
| A9 | 自建节点 CRUD | `internal/api/subscription.go:413` | HTTP ✅ | 6 |
| A10 | 规则行 CRUD | `internal/api/subscription.go:240` | HTTP ✅ | 7 |
| A11 | **上游订阅 `rules` 导入**（已解析但被丢弃） | `internal/parse/parse.go:114-116` 产出；消费点 `internal/convert/engine.go:146-167` 只取 Nodes/Providers | **无（丢弃）** | 7 |
| A12 | **上游 `rule-providers:` 段** | **完全未解析**（非 vendor 目录搜不到该字符串） | **无** | 7 |
| A13 | `rule_providers` 表 | `internal/store/store.go:303` | **死表**（全仓仅此一条建表语句） | 7 |
| A14 | **Rule-Bot 建议队列**（Detector + PendingQueue 确认/拒绝） | `internal/rulebot/rulebot.go` | **仅 CLI** | 7 |
| A15 | **manifest 规则源聚合**（拉取/版本/retention/回滚） | `internal/manifest/manifest.go` | **仅 CLI** | 8 |
| A16 | **MRS 离线生成** | `internal/mrs/mrs.go` | **仅 CLI** | 8 |
| A17 | proxy-providers 透传 | `internal/parse/parse.go:113`, `internal/convert/engine.go` | 转换路径内 ✅ | 2（无感透传，不在 UI 编辑） |
| A18 | 分组 CRUD | `internal/api/subscription.go:151` | HTTP ✅ | 9 |
| A19 | 令牌 CRUD + `/subscribe` | `internal/api/subscription.go:329,499` | HTTP ✅ | 10 |
| A20 | **定时任务 + 告警通道**（log/telegram/dingtalk；smtp 是桩） | `internal/scheduler/scheduler.go`, `alert_channels.go` | **仅 CLI** | 11 |
| A21 | INI 模板加载 + `[custom]`/`[ruleset]`/`[node_pref]` | `internal/template/ini.go`, `internal/config/{template,custom}.go` | 部分（`/api/rulesets` 只读） | 12 |
| A22 | 分层覆盖 DSL（L1–L4） | `internal/config/override.go` | 仅 `/rules` 的 textarea | 12 |
| A23 | 转换历史 | `internal/api/web.go:123` | HTTP ✅ | 1 |
| A24 | 日志缓冲 + 拉取审计 | `internal/api/logbuf.go` | HTTP ✅ | 13 |
| A25 | **MCP 服务器** | `internal/mcp/mcp.go` | **死代码**（全仓无引用） | 无页面（机器接口）；**必须接线 `subforge mcp`**，见附录 3 |
| A26 | pprof + `/api/metrics` | `internal/api/pprof.go` | HTTP ✅ | 13（只放入口链接，不自建面板） |

**断链诊断**：A5/A11/A12 是"后端已解析、下游没消费"；A13 是"建了表没人用"；A25 是"写了包没接线"——四类都是**链路断在半途**，共同原因是没有一张"能力 → 出口"的对照表。§1.1 就是那张表，后续任何新能力都必须先在这里登记。

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

### 2.1b 本轮补全新增/变更的表

初版只加了 `airports` 的流量列与 `fetch_audit`；补全脚本与上游规则导入后，还需要下面这些。

**① `scripts`（新增）—— goja 脚本库**

```sql
CREATE TABLE IF NOT EXISTS scripts (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL UNIQUE,          -- 管线段用 ref=<name> 引用
  description TEXT NOT NULL DEFAULT '',
  code        TEXT NOT NULL,                 -- 内联 JS 源码（goja.Compile 的输入）
  enabled     INTEGER NOT NULL DEFAULT 1,
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**② `rules` 加 3 列（上游导入需要溯源与幂等）**

现有列：`id / rule_type / rule_param / rule_config / resolve_dns / enabled / sort_order / created_at / updated_at`（`internal/store/store.go:291-301`）——**没有来源字段，也没有原始行**。上游规则是 `DOMAIN-SUFFIX,example.com,DIRECT` 这样的**整行文本**：`rule_type`/`rule_param` 能拆出来，但第 3 段是分组**名字**，而 `rule_config` 存的是分组 **id**，需要一个映射步骤；映射失败的行不能丢。

```sql
ALTER TABLE rules ADD COLUMN source     TEXT NOT NULL DEFAULT '';  -- 'manual' | 'airport:<名>' | 'rulebot'
ALTER TABLE rules ADD COLUMN raw        TEXT NOT NULL DEFAULT '';  -- 原始规则行，映射失败时的兜底
ALTER TABLE rules ADD COLUMN origin_key TEXT NOT NULL DEFAULT '';  -- 去重键：规范化后的行
CREATE UNIQUE INDEX IF NOT EXISTS idx_rules_origin ON rules(source, origin_key);
```

**③ `rule_providers` 加唯一索引**

列（`name / raw_json / source / enabled / sort_order / created_at / updated_at`）已经够用，只需 `source` 写成 `airport:<名>`，并在 `(source, name)` 加唯一索引，避免反复导入同一机场时重复。

**④ 规则源（manifest）不落库**

`internal/manifest` 的实现是**文件版本化**（`<name>-<UTC 时间戳>.<ext>` + retention + `Rollback`），不要为了 UI 把它改成数据库。`/rulesets` 读 config 表里的 `rules.manifest_path` 指向的 YAML + 扫描输出目录得到版本列表。落库会丢掉"文件即事实源、可人工替换"这个性质。

**⑤ 告警通道落 config 表**

key = `alert.channels`，value = `[]ChannelConfig` 的 JSON：

```go
type ChannelConfig struct {
    Kind     string `json:"kind"`      // log | telegram | dingtalk | smtp
    Enabled  bool   `json:"enabled"`
    BotToken string `json:"bot_token,omitempty"`
    ChatID   string `json:"chat_id,omitempty"`
    Webhook  string `json:"webhook,omitempty"`
    Host     string `json:"host,omitempty"`
    Port     int    `json:"port,omitempty"`
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`
    From     string `json:"from,omitempty"`
    To       string `json:"to,omitempty"`
}
```

**读取脱敏**：`GET /api/alert-channels` 对 `bot_token`/`webhook`/`password` 一律回 `"***"` + `configured: true`；写入时收到 `"***"` 表示"保持原值不变"。这一条是安全约定，不许省。

**⑥ 任务执行历史不落库（第一版）**

`scheduler.Scheduler.Results()` 已经返回内存里最近的任务结果（`JobResult{JobName, Success, Err, StartedAt, Duration}`）。第一版 `/tasks` 直接读它，进程重启即清空——**这是有意的**：落库要引入保留策略、迁移、清理三个新问题，而任务历史不是资产。等确有需要再加表。

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

### 3.6 脚本子系统（补齐 A5）

> **本节已实现**（提交：`7adce6b` feat(config) 管线脚本步骤支持 `ref=` 脚本名与 `script.enabled` 安全开关；`0d70381` feat(scripts) 脚本库/设置/告警/规则源存储层与对应 API；`c4738a6` feat(web) 脚本库与任务页落地、管线对象化适配、前端端点补全；`fb896df` pipelines 表启用）。概念映射：`internal/scripts` 薄包思想落成 `internal/subforge/scripts.go`（ScriptResolver 鸭子类型）+ `internal/api/scripts.go`（CRUD）+ `internal/api/scripts_test.go`（`POST /api/scripts/test`）；`ref=<脚本名>` 与 `script.enabled` 开关在 `internal/config/pipeline.go` 的 ScriptResolver 接口实现；前端 `ScriptsPage.vue` 已从占位改为真实页面。关键文件：`internal/subforge/scripts.go`、`internal/api/scripts.go`、`internal/api/server.go`（`/api/scripts` 路由）、`web/src/pages/ScriptsPage.vue`。


**现状（已核对）**：`internal/transform/script.go` 提供 goja 沙箱；`internal/config/pipeline.go:88` 的 `filter.script` 与 `:114` 的 `op.script` 把**内联脚本源码**直接编译执行。

- 参数：`src`（**是 JS 源码本身，不是文件路径**——`newScriptRunner` 直接 `goja.Compile(name, src, false)`，见 `internal/transform/script.go:33-45`）、`name`（仅用于错误定位）、`timeout`（缺省 5s，`defaultScriptTimeout = 5 * time.Second`）。
- 沙箱：只注入纯数据对象 `node`（`name/type/server/port/source` + 完整 `raw`），无 require/fetch/文件系统/进程；超时用 `vm.Interrupt`，panic 被 recover。
- **问题**：脚本站位只能塞在 INI 模板的一行里。源码里带逗号、带等号、带换行都会把 `splitStepTokens` 解析弄坏；用户也没法复用、没法试跑、没法版本管理。

**新增 `internal/scripts` 包（薄，只做存取）**

```go
package scripts

type Script struct { ID int64; Name, Description, Code string; Enabled bool }

// List / Get / GetByName / Create / Update / Delete：操作 scripts 表。
// GetByName 是管线解析的热路径，未命中返回 ErrNotFound。
```

**管线引用语义（关键决策）**

`filter.script` / `op.script` 增加 `ref=<脚本名>`。解析顺序：

1. `ref` 命中 `scripts` 表 → 用库里源码；
2. `ref` 给了但库里没有 → **报错**（不要静默回落成空 `src`，否则用户以为脚本生效了）；
3. 只有 `src` → 沿用现状（存量 INI 模板零迁移）；
4. `ref` 与 `src` 同时给 → 报错（歧义不猜）。

`ref` 解析必须发生在 `internal/config/pipeline.go` 的 `PipelineStep.build()` 里，但 `internal/config` **不能**反向依赖 SQLite。做法：给 `PipelineSpec.Compile()` 增加一个可选的解析器参数

```go
type ScriptResolver interface {
    Resolve(code string) (source string, err error) // code 是 ref 名
}
```

`subforge convert` 不传（保持纯文件语义，只支持 `src`）；serve 路径传 `scripts` 包的实现。nil 解析器 + `ref` = 明确报错。

**安全开关（对齐设计书 §2"token 保护，默认关闭"）**

config 表 key = `script.enabled`，默认 `false`。

- 关闭时：脚本步骤在 `Compile()` 阶段就报 `脚本步骤已禁用（在「设置」里开启 script.enabled）`，**不执行任何 goja 代码**。
- 开启时：`/scripts` 顶部常驻风险提示——"脚本在内嵌 goja 沙箱执行，只注入节点数据；但脚本仍然是代码，只运行你自己写的脚本。"
- 开关只在 `/settings` 改；`/scripts` 只读显示状态 + 跳转链接（避免两处可改）。

**试运行 API（脚本库真正的价值点）**

`POST /api/scripts/test`，body：

```json
{ "code": "<JS 源码>", "kind": "filter", "node": { "...一个节点的原始 map..." }, "timeout": "5s" }
```

返回：

```json
{ "ok": true, "kept": true, "before": {}, "after": {},
  "duration_ms": 3, "error": "" }
```

- `kind=filter`：返回 `kept`（脚本真值）；`kind=operate`：返回 `before`/`after` 字段 diff（只回变化的键，避免大 `raw` 刷屏）。
- 不给 `node` 时用内置 3 个样例节点（ss / vmess / trojan）。
- 编译错误原样回传（goja 的 `*goja.Exception` 带行列号），超时按 `timeout` 截断。
- **绝不落库**：试运行不写 `scripts` 表。
- 关闭 `script.enabled` 时试运行**仍可用**（试运行就在用户眼皮底下、拿到的是样例节点，风险可控），但要在面板上标注"线上脚本已禁用，试运行不生效"。若实现者认为口径不一致，可改为一并禁用——**这一条留给实现者按安全评审结论定**。

---

### 3.7 上游订阅规则导入（补齐 A11–A13）

> **本节已实现**（提交：`6bea4da` feat(api) GET /api/rules 补 source/raw/origin_key 与 group_name；`bbf71c3` feat(api) 补全规则导入/规则集/批量启用/建议/设置/模板/override/规则源/任务/测活/脚本试运行端点并收口路由；`214fa76` feat(api) POST /api/airports/refresh 支持 `?with_rules=1` 顺带导入规则；`9cd4339` feat(rulebot) 建议应用闭环；`6f54f47` feat(web) 规则页四区块与规则源页操作按钮）。① 解析层 `internal/model` 的 `ParseClashRuleProviders` + `parse.Decoded.RuleProviders/Warnings`；② 导入语义 `internal/subforge/import.go` 的 `ImportRules`/`NormalizeRuleOriginKey`（候选队列 enabled=0、`origin_key` 幂等、分组名映射不上不丢行）；③ 入口 `POST /api/airports/{id}/import-rules` 与 `POST /api/airports/refresh?with_rules=1`；④ 前端 `RulesPage.vue` 四区块 + `RuleSourcesPage.vue` 操作按钮。关键文件：`internal/model/model.go`、`internal/parse/parse.go`、`internal/subforge/import.go`、`internal/api/server.go`（`/api/airports/{id}/import-rules`、`/api/rule-sources/*`）、`web/src/pages/RulesPage.vue`、`web/src/pages/RuleSourcesPage.vue`。


**现状（已核对，链路断在半途）**

- `internal/parse/parse.go:114-116` 的 `Decoded.Rules` 注释写得非常明确：**"供后续 SQLite 持久化导入扫描复用，不在转换路径上隐式合并"**——作者有意不在转换时自动并规则，留给"导入"场景消费。**但消费方从未写出来**：`internal/convert/engine.go:146` 拿到 `dec` 后只用 `dec.Nodes`（`:151`）和 `dec.Providers`（`:152`），`dec.Rules` 被丢弃；`Files` 分支（`:161-167`）同样。
- `rule-providers:` 段**完全没解析**（非 vendor 目录搜不到该字符串）。注意 `Decoded.Providers` 是 **proxy**-providers（`internal/model/model.go:73-81`），不是 rule-providers。
- `internal/store/store.go:303` 建了 `rule_providers` 表，全仓引用**只有这一条建表语句**——死表。
- `rules` 表没有 `source` / `raw` / `origin_key` 列（见 §2.1b②），导入无法溯源也无法幂等。

**① 解析层：`Decoded` 加 `RuleProviders`**

```go
type Decoded struct {
    Nodes         []map[string]any
    Providers     []model.Provider     // proxy-providers（已有，原样透传）
    Rules         []string             // rules 段（已有，之前没人用）
    RuleProviders []model.RuleProvider // 新增：rule-providers 段
    Warnings      []string             // 新增：解析告警（如分组名映射失败）
}

// model 新增（与 Provider 同构：Raw 是渲染事实源）
type RuleProvider struct {
    Name string
    Raw  map[string]any
}
```

`DecodeSource` 里把 `rule-providers:` 按 "name → map" 读进 `RuleProviders`（Clash 的 rule-providers 是 **map** 形式，不是 list）。

**② 导入语义：候选队列，不自动生效**

最重要的决策，直接继承 `parse.go:115` 的原意：

- 机场导入时把 `Rules` 与 `RuleProviders` 写成**候选**：
  - 规则行 → `rules` 表，`source='airport:<机场名>'`，**`enabled=0`**（默认不生效）；
  - 规则集 → `rule_providers` 表，`source='airport:<机场名>'`，**`enabled=0`**。
- 幂等：`origin_key` = 规范化后的整行（去多余空白、统一 `,` 分隔），唯一索引挡重复；重复导入只更新 `updated_at`。
- 分组名映射：上游第 3 段是分组**名**，`rules.rule_config` 存分组 **id**。映射不上时**不丢行**——写 `raw` 原样保留 + `rule_config=0`，并在 `/rules` 候选区红色标注"目标分组不存在"。**不要自动新建分组**，那是替用户做决定。
- 生效条件：`enabled=1` 且 `rule_config` 能解析到分组，才进渲染输入。

**③ 入口**

- `POST /api/airports/{id}/import-rules`：单机场导入，返回 `{rules_added, rules_skipped, providers_added, warnings[]}`。
- `POST /api/airports/refresh` 增加可选 `?with_rules=1`：刷新时顺带导入。默认**不带**，保持刷新是纯流量操作、职责单一。
- `/getruleset` 保持不动（它是 subconverter 兼容的临时转换面，与导入是两码事）。

**④ 与 `/rules` 页面的关系**

`/rules` 变成四个区块（详见 §5.5 第 7 条）：**规则行**（手动，可编辑）、**规则集**（`rule_providers`）、**导入候选**（只读列表 + 批量启用/禁用 + "启用本机场全部"）、**Rule-Bot 建议**（确认/拒绝）。

---

## 4. 后端 API 补全

> **实现状态（2026-10）**：§4 的端点清单已实现（对应提交 `bbf71c3` 与 `0d70381`）；§3.6 脚本子系统、§3.7 规则导入已实现。所有端点均注册在 `internal/api/server.go` 的 `Routes()`，全量清单见 README 「HTTP API」章节。
>
> 尚未实现：`/api/pipelines` 的 DB 持久化（§4.1 `pipelines` 表）仍未落地——当前 `/api/pipelines` 仅返回名字（模板来源），`pipelines` 表尚未创建；`/api/airports/{id}/import-rules` 的 handler 已落地，但 Rule-Bot 闭环仍依赖 `SetSuggestionQueue` 注入（默认 nil，POST 返回 501）。

`internal/api/server.go` 的 `Routes()` 新增（按页面归属分组）：

**流量（页 5 / 10）**

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| GET | `/api/airports` | 现有 | 增补流量字段（upload/download/total/expire/node_count/traffic_at/traffic_err） |
| POST | `/api/airports/refresh` | `handleAPIAirportsRefresh` | 拉全部启用机场，返回 `[]TrafficRefreshResult` |
| POST | `/api/airports/{id}/refresh` | 同上（单源） | |
| GET | `/api/traffic/strategy` | `handleAPITrafficStrategy` | 返回当前 Strategy + 预设清单 |
| PUT | `/api/traffic/strategy` | 同上 | 保存 Strategy |
| GET | `/api/traffic/preview` | `handleAPITrafficPreview` | `?strategy=<json>` → 用当前启用机场的**已存**数据试算，返回 Usage + 响应头原文（不落库、不重拉） |

**测活（页 5）**

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| POST | `/api/airports/check` | `handleAPIAirportsCheck` | body `{ids?: []int64}`，缺省=全部启用；并发上限取 `subscheck.Options.normalize()` 的 8、超时 15s；返回 `[]subscheck.Result`（URL/OK/StatusCode/Latency/Bytes/Err） |
| GET | `/api/airports/check/last` | 同上 | 最近一次测活结果（内存缓存，进程内，不落库，与 §2.1b⑥ 同一口径） |

**脚本（页 4）**

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| GET/POST | `/api/scripts` | `handleAPIScripts` | 列表 / 新建 |
| GET/PUT/DELETE | `/api/scripts/{id}` | `handleAPIScript` | 单条 CRUD |
| POST | `/api/scripts/test` | `handleAPIScriptTest` | 试运行，**不落库**（§3.6） |

**规则与来源（页 7 / 8）**

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| GET | `/api/rules` | 现有 | 增补 `source` / `raw` / `origin_key` / 分组名解析结果 |
| POST | `/api/airports/{id}/import-rules` | `handleAPIAirportImportRules` | 上游 `rules` + `rule-providers` → 候选（`enabled=0`） |
| POST | `/api/rules/bulk` | `handleAPIRulesBulk` | body `{ids|source, enabled}`：批量启用/禁用候选（含"启用本机场全部"） |
| GET | `/api/rule-providers` | `handleAPIRuleProviders` | 规则集列表（当前 **无任何 API**，表是死的） |
| GET | `/api/suggestions` | `handleAPISuggestions` | Rule-Bot `PendingQueue.Pending()` |
| POST | `/api/suggestions/{index}/confirm` | `handleAPISuggestionAction` | 确认 → 写成 `rules` 行（`source='rulebot'`） |
| POST | `/api/suggestions/{index}/reject` | 同上 | 拒绝 |
| GET | `/api/rule-sources` | `handleAPIRuleSources` | 读 `rules.manifest_path`，返回来源清单 + 每个来源的版本列表与健康 |
| POST | `/api/rule-sources/update` | `handleAPIRuleSourcesUpdate` | 触发 `manifest.Update(ctx, manifest.WithRetention(n))`，返回 `[]UpdateResult` |
| POST | `/api/rule-sources/rollback` | `handleAPIRuleSourcesRollback` | `{name, ext}` → `manifest.Rollback(dir, name, ext)` |
| POST | `/api/rule-sources/mrs` | `handleAPIRuleSourcesMRS` | `{src, behavior, format}` → `mrs.GenerateCached(...)`，返回缓存路径与是否命中 |

**任务与告警（页 11）**

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| GET | `/api/tasks` | `handleAPITasks` | `scheduler.Results()` + 已注册 Job 名 + 下次运行时间 |
| GET/PUT | `/api/alert-channels` | `handleAPIAlertChannels` | 读写告警通道，**读脱敏**（见 §2.1b⑤） |
| POST | `/api/alert-channels/test` | `handleAPIAlertChannelTest` | 发一条测试消息 |

**设置（页 12）**

| 方法 | 路径 | 处理函数 | 说明 |
|---|---|---|---|
| GET/PUT | `/api/settings` | `handleAPISettings` | 读写全局开关与路径：`script.enabled`、`rules.manifest_path`、`rules.output_dir`、`subscheck.enabled` |
| GET/PUT | `/api/template` | `handleAPITemplate` | INI 模板全文（含 `[custom]`/`[ruleset]`/`[node_pref]`）；PUT 前做**语法校验**（复用 `internal/template` + `internal/config` 的解析器），校验失败回 400 + 行号 |
| GET/PUT | `/api/override` | `handleAPIOverride` | 分层覆盖文本（原在 `/rules` 的 textarea 迁到这里） |

**页面路由**新增：`/pipelines` `/scripts` `/airports` `/self-nodes` `/rulesets` `/groups` `/tasks` `/settings`，各对应一个 html（`/rules` `/subscriptions` `/logs` `/convert` `/` 保留现有路由名）。

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

> 本节的「前端技术栈与施工顺序」已被 `docs/design/vite-vuetify-coverage-plan.md` 取代，以该文件为准。
> 本文件中仍然有效的是：§1 模块边界与能力矩阵、§2 数据模型、§3 流量聚合与脚本、§4 后端 API 补全清单。

> ⚠️ **本节已被取代**：前端技术栈改为 **Vite + Vue 3 + Vuetify 3.13.7 + TypeScript**，并新增前端覆盖率门禁 100%。
> 见 `docs/design/vite-vuetify-coverage-plan.md` §2（技术栈）、§3（门禁 A）、§5（页面与共享层规格）。
> 本节保留，仅作为「13 页要做哪些事」的设计意图记录。

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
  index.html             # 1  概览
  convert.html           # 2  转换
  pipelines.html         # 3  管线            [新]
  scripts.html           # 4  脚本            [新]
  airports.html          # 5  机场            [新]
  self-nodes.html        # 6  自建节点        [新]
  rules.html             # 7  规则（重写：四区块）
  rulesets.html          # 8  规则源          [新]
  groups.html            # 9  分组            [新]
  subscriptions.html     # 10 订阅（重写，瘦身）
  tasks.html             # 11 任务            [新]
  settings.html          # 12 设置            [新]
  logs.html              # 13 日志（重写，加审计）
  js/
    dashboard.js
    convert.js
    pipelines.js         [新]
    scripts.js           [新]
    airports.js          [新]
    self-nodes.js        [新]
    rules.js              # 重写：规则行 + 规则集 + 导入候选 + 建议
    rulesets.js          [新]
    groups.js            [新]
    subscriptions.js      # 重写：只有令牌 + 生成 + 策略；删除 23.6KB 大杂烩
    tasks.js             [新]
    settings.js          [新]
    logs.js
  partials/
    dialog.js            # 共享：inline dialog / sheet / toast / confirm（新）
    table.js             # 共享：空状态、加载、错误行（新）
    chips.js             # 共享：只读引用 chip + 跳转（新）
    editor.js            # 共享：带行号的等宽编辑器 + Tab 缩进 + 语法校验提示（新，脚本/模板/覆盖三处复用）
```

**删除**：`js/subscription.js`（内容拆到 rules.js / groups.js / airports.js / subscriptions.js）。

`partials/editor.js` 是本次**唯一新增的基础组件**，因为脚本源码、INI 模板、覆盖文本三处都需要"等宽 + 行号 + 不把手感做坏"的输入框。**不要引入 CodeMirror/Monaco**——项目零构建、单二进制内嵌，一个 `<textarea>` + 行号槽 + Tab 处理就够了。

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
- 列表列：名字、步骤数、来源（`api`=界面建的 / `template`=INI 模板里的）、启用开关。
- 编辑：**有序步骤列表**，可增删拖排序。每步的结构化表单：
  - `kind` 下拉，选项就是 `internal/config/pipeline.go 的 PipelineStep.build()` 支持的 **10 种**：
    `filter.regex` / `filter.region` / `filter.type` / `filter.script` / `op.rename` / `op.emoji` / `op.dedupe` / `op.split` / `op.sort` / `op.script`
  - 选中 kind 后表单**只显示该 kind 的合法参数**（`include`/`exclude`、`pattern`/`repl`、`sep`、`by`……），不允许自由填 `Args`。
  - 两个 script 步骤的 `src` 字段旁边给 **脚本选择器**：从 `/api/scripts` 拉列表，选一个 → 写 `ref=<名>`（见 §3.6）。也允许内联 `src`（转义/多行），此时显示“内联脚本，不参与脚本库复用”的提示。
  - 每步可用 `onError`（inherit/continue/abort）。
- 模板来源的管线**只读**，标 `template` badge，并给“复制为可编辑管线”按钮（一次性 fork 进 DB）。
- 保存时先调 `PipelineSpec.Compile()` 做**后端校验**，错误回 400 + 具体步骤序号 + 行号文案，前端在对应步骤上打红标。
- 验收：保存后 `/convert` 与 `/subscribe` 的 `pipeline=` 都能选到；`filter.script`/`op.script` 在 `script.enabled=false` 时保存直接报错，不进 goja。

**4) 脚本 `/scripts`** [新]
- 列表列：名称、类型（`filter`/`operate`）、被几条管线引用、更新时间、启用。
- 编辑：`partials/editor.js`（等宽 + 行号），**右侧试运行面板**：
  - 输入一个样例 `node`（JSON，给默认骨架：name/type/server/port/source）；
  - 调 `POST /api/scripts/test`，展示 `before` / `after` 节点对比、`kept` 真值、`duration_ms`、`error`；
  - 超时/异常原样回显（goja 的 `asInterrupted` 文案）。
- 安全提示条：**脚本默认关闭**（`script.enabled=false`），开关只在设置页改；关闭时本页只能读写代码，试运行返回 403。
- 验收：新增脚本 → 管线页 `ref` 下拉里出现；删除被引用的脚本时后端拒绝并列出引用它的管线名。

**5) 机场 `/airports`** ← 流量数据的唯一归属页
- 列表列：启用、名称、URL（截断 + 悬浮全量）、备注、**流量**（used/total 进度条）、**到期**（日期 + 剩余天数）、**测活**（延迟 + 状态点）、上次采集、错误。
- 行内操作：编辑、刷新（单源）、**测活**、删除。
- 顶部操作：新增、**刷新全部**、**全部测活**。
- 编辑表单：名称、订阅 URL、备注、启用。
- 刷新中：按钮转 loading，行内显示 spinner；完成后 toast + 就地更新。
- 测活结果来自 `POST /api/airports/check`（`subscheck`，并发 8 / 超时 15s）；结果只在内存，刷新页面后从 `/api/airports/check/last` 恢复。
- 验收：`POST /api/airports/refresh` 后列表数值变化；失败源显示红色错误文本且**不丢旧值**；测活失败只影响测活列，不影响流量列。

**6) 自建节点 `/self-nodes`**
- 列表：启用、名称（无则显示链接摘要）、链接。
- CRUD 同机场模式。
- 验收：新增后 `/subscribe` 产物含该节点。

**7) 规则 `/rules`** ← 四个区块，不再是一坨
- 区块 A **规则行**（`rules` 表）：启用、类型、参数、命中分组（下拉选分组名）、`no-resolve`、**来源**（`manual` / `airport:<名>` / `rulebot`）。
  - 支持按来源筛选；候选行（`enabled=0` 且来自机场）用**灰底“候选”**样式，行首给勾选框。
  - 批量操作条：全选 / 启用所选 / 禁用所选 / 删除所选 → `POST /api/rules/bulk`。
- 区块 B **上游导入**：
  - 每个机场一个“导入规则”按钮 → `POST /api/airports/{id}/import-rules`。
  - 结果是**候选队列**：`source='airport:<名>'`、`enabled=0`、`origin_key` 幂等（重复导入不产生重复行）。
  - 分组名映射不上的行：写 `raw` 保留原始内容、`rule_config=0`、打红标“分组未匹配”，**不自动建分组**。
  - 从 `rule-providers:` 解析出来的规则集进区块 C。
  - 文案必须写清：“导入 = 进候选区，不会自动生效。”
- 区块 C **规则集**（`rule_providers`）：名称、`source`、`enabled`、`sort_order`、`raw_json` 查看（只读抽屉）。
- 区块 D **Rule-Bot 建议**：拉 `GET /api/suggestions`，每条显示 异常类型 / 建议动作 / 规则 / 理由；行内「采纳 →」调 `confirm`（写成 `rules` 行）或「忽略」调 `reject`。
- 验收：规则行改完立即影响 `/subscribe` 产物；导入两次不产生重复行；采纳建议后规则行区块多一行且来源为 `rulebot`。

**8) 规则源 `/rulesets`** [新] ← manifest 聚合 + MRS 离线生成
- 顶部：`rules.manifest_path` 显示 + 编辑入口（跳设置页）；一个 **“更新全部”** 按钮 → `POST /api/rule-sources/update`。
- 来源表：来源名、URL、格式（yaml/text/mrs）、**当前版本文件**、大小、更新时间、状态（ok / 失败但保留旧版本）。
- 每行展开：**版本列表**（`manifest.ListVersions`，默认保留 5 个）→ 每行一个“回滚到此版本”按钮 → `POST /api/rule-sources/rollback`。
- 底部 **MRS 转换工具**：选源文件 + `behavior`（domain/ipcidr）+ `format` → `POST /api/rule-sources/mrs`，显示缓存路径与命中状态。
  - 注意文案：`classical` 没有二进制形式，选它时报错“classical 无法生成 MRS”。
- 验收：点“更新全部”后版本列表变长；回滚后当前版本指针回到旧文件；生成 MRS 后文件真实落盘且能被 `/getruleset` 路径引用。

**9) 分组 `/groups`**
- 列表：名称、类型、成员模式（regex）、测速 URL、间隔。
- CRUD inline dialog。
- 验收：分组名出现在规则页的下拉；组变更影响产物。

**10) 订阅 `/subscriptions`** ← 瘦身后的核心页
- 区块 A：**令牌**（生成/删除/复制），显示完整订阅 URL（带 token）。
- 区块 B：**流量聚合策略**
  - 第一大块：10 个预设的按钮/下拉。
  - 第二大块：4 个字段各自的下拉（sum/min/max/avg/first/last/none）。
  - 选预设时铺满四字段；手动改任一字段 → Preset 变 “自定义”。
  - **实时预览**：调 `/api/traffic/preview`，显示 “upload=X download=Y total=Z expire=D” 与生成的响应头原文。
- 区块 C：**引用概览**（只读 chips + 跳转）：启用的机场、生效的分组、生效的规则数。**这里只读，改去各自页面**。
- 区块 D：生成按钮 → 显示产物状态（节点数、格式、warnings），并提供“复制订阅地址”。
- 验收：改策略 → 预览即时变；用 `curl -I` 拉真实订阅，响应头与预览一致。

**11) 任务 `/tasks`** [新] ← scheduler + 告警
- 上半区 **已注册任务**：任务名、来源（内置：规则源更新 / 流量刷新 / 规则源健康检查）、是否启用、上次结果（成功/失败 + 耗时，来自 `GET /api/tasks` ← `scheduler.Results()`）。
- 下半区 **告警通道**：`LogAlertChannel` / `TelegramChannel` / `DingTalkChannel` / `SMTPChannel`。
  - 表单字段：enabled、token/webhook/邮箱等。**读回时 secrets 一律显示 `***` 且 `configured:true`**；用户不改这一格就沿用旧值（不能把 `***` 存回去）。
  - `SMTPChannel` 明确标注“尚未实现”（后端是桩）；选了它保存要拦或给黄条警告。
  - 每通道一个“发送测试”按钮 → `POST /api/alert-channels/test`$。
- 说明条：任务执行历史第一版**不落库**，进程重启即清空；需要长期留痕走日志页。
- 验收：改通道配置后刷新页面，secret 仍显示 `***`；发送测试在通道未配置时给明确错误。

**12) 设置 `/settings`** [新] ← 全局开关 + 模板 + 覆盖
- 区块 A **全局开关**：`script.enabled`（默认关，开了才允许 goja）、`subscheck.enabled`、`rules.manifest_path`、`rules.output_dir`。
- 区块 B **INI 模板**：`partials/editor.js` 全屏编辑，含 `[custom]` / `[ruleset]` / `[node_pref]` / `[pipeline:<名>]`。
  - 保存前必须**后端语法校验**（复用 `internal/template` + `internal/config` 解析器）；失败回 400 + 行号 + 原文，前端跳到出错行。
  - ⚠️ 保存模板**不覆盖** DB 里已有同名管线/规则/分组——这是 §4.1 的存储决策，页面上要写明优先级。
- 区块 C **分层覆盖**：原来的 YAML/INI 覆盖 textarea 从规则页迁到这里（调用不变，仅换位置）。
- 验收：故意写坏模板 → 保存被拒 + 定位到行；改 `script.enabled` 后脚本页试运行立刻从 403 变可用。

**13) 日志 `/logs`**
- Tab 1 系统日志（现有 `/api/logs`）。
- Tab 2 拉取审计（`/api/audit`）：时间、机场、状态码、字节、耗时、结果。
- Tab 3 任务日志（可选）：同 `/api/logs` 但过滤 `scheduler` / `rulebot` 前缀。
- 验收：刷新机场后审计新记录出现。

---

## 6. 实施顺序与提交点

> 本节的「前端技术栈与施工顺序」已被 `docs/design/vite-vuetify-coverage-plan.md` 取代，以该文件为准。
> 本文件中仍然有效的是：§1 模块边界与能力矩阵、§2 数据模型、§3 流量聚合与脚本、§4 后端 API 补全清单。

> ⚠️ **本节已被取代**：施工顺序见 `docs/design/vite-vuetify-coverage-plan.md` §8（P1–P12）。
> 本节的 P1–P15 仍覆盖数据模型与 API 补全，那些工作**继续有效**，只是不再与前端换栈混在同一条序列里。

每个阶段**独立可构建、可验证、可提交**。阶段间用提交点隔开，不攒大提交。

**后端（P1–P6，先做，前端才有 API 可调）**

| 阶段 | 内容 | 验证 | 提交信息 |
|---|---|---|---|
| **P1** | traffic 包（解析 + 聚合 + 预设） | `go test ./internal/traffic/...` 全绿 | `feat(traffic): 流量元数据解析与聚合策略` |
| **P2** | fetch 采集响应头 | `go test ./internal/fetch/...` | `feat(fetch): 采集 subscription-userinfo 响应头` |
| **P3** | store schema 迁移 + Airport struct + 审计表 + `scripts` 表 + `rules` 新列 | `go test ./internal/store/...`；旧库能打开 | `feat(store): 机场流量字段、脚本表与规则来源列` |
| **P4** | Manager 刷新路径 + 策略读写 + config key | `go test ./internal/subforge/...` | `feat(subforge): 机场流量刷新与策略持久化` |
| **P5** | /subscribe 写响应头 + 补 pipeline/config/include/exclude | `go test ./internal/api/...` | `feat(api): 订阅响应头透传流量余额` |
| **P6** | pipelines 表 + API CRUD | `go test ./internal/api/...` | `feat(api): 管线数据库化管理` |

**补齐 CLI-only 能力（P7–P11）** ← 本轮新增，把"能力存在但没出口"的断层接上

| 阶段 | 内容 | 验证 | 提交信息 |
|---|---|---|---|
| **P7** | `internal/scripts` 包 + `PipelineSpec.Compile()` 的 ScriptResolver 参数 + `script.enabled` 开关 + `/api/scripts` CRUD + `/api/scripts/test` | `go test ./internal/scripts/... ./internal/config/...`；开关关闭时 Compile 报错 | `feat(scripts): 命名脚本库与管线 ref 引用` |
| **P8** | 上游规则导入：`DecodeSource` 解析 `rules`/{BT}rule-providers` + `model.RuleProvider` + `import-rules` + `/api/rules/bulk` + `/api/rule-providers` | `go test ./internal/parse/... ./internal/api/...`；重复导入幂等 | `feat(rules): 上游订阅规则导入候选队列` |
| **P9** | Rule-Bot 接线：`/api/suggestions` 三端点，确认后写成 `rules` 行（`source='rulebot'`） | `go test ./internal/api/...` | `feat(rulebot): 建议队列 API 与采纳落库` |
| **P10** | 规则源：`/api/rule-sources` + update / rollback / mrs 三端点 | `go test ./internal/api/...`；回滚后当前版本指针变化 | `feat(ruleset): 规则源聚合、回滚与 MRS 生成 API` |
| **P11** | 任务与告警：`/api/tasks` + `/api/alert-channels`（读脱敏）+ test + 设置三端点（`/api/settings` / `/api/template` / `/api/override`） | `go test ./internal/api/...`；secret 回读为 `***` | `feat(api): 任务告警与设置端点` |

**前端（P12–P15）**

| 阶段 | 内容 | 验证 | 提交信息 |
|---|---|---|---|
| **P12** | 前端共享层（app.css / app.js / `partials/` 四件） | 手工过 **13 页**骨架 | `feat(web): 重写共享样式与组件层` |
| **P13** | 逐页实现（按 1→13 顺序，每页一提交或两页一提交） | 每页验收点 | `feat(web): <页名> 页面` |
| **P14** | 删除 `js/subscription.js`，清理死代码；未交付能力要在页面上留白而不是静默 | `go build ./...` + 全页回归 | `refactor(web): 移除订阅页大杂烩脚本` |
| **P15** | 全量回归 + 文档 + MCP 决策落地（接线 `subforge mcp` 或明确标注不交付） | `go build/vet/test ./...` | `docs: Web UI 重写与流量聚合说明` |

**硬性要求**（每阶段都适用）：
- `go build ./...`、`go vet ./...`、`go test ./...` 三条全过才算完成（CI 就是这三条，vendored 模式）。
- 提交信息文件以 `.txt` 结尾，不用 `--no-verify`。
- 每个后端阶段带单元测试；每个前端页面带手工验收记录。
- 不许为了"页面能点"去改后端测试或放宽校验；能力不够就在页面上写清"未实现"。

---

## 7. 全局验收清单

**结构**
- [ ] **13 个页面路由可达**，无 404：`/` `/convert` `/pipelines` `/scripts` `/airports` `/self-nodes` `/rules` `/rulesets` `/groups` `/subscriptions` `/tasks` `/settings` `/logs`；侧边栏高亮当前页正确。
- [ ] **无重合**：规则行/规则集/上游候选只在 `/rules` 改；机场只在 `/airports` 改；自建节点只在 `/self-nodes` 改；分组只在 `/groups` 改；管线只在 `/pipelines` 改；脚本只在 `/scripts` 改；模板与覆盖只在 `/settings` 改。`/subscriptions` 里这些**全部是只读 chip + 跳转**。
- [ ] `js/subscription.js` 已删除，其功能无遗漏地落到 rules/groups/airports/subscriptions 四页。

**流量**
- [ ] `curl -I "$SUB_URL"` 返回 `Subscription-Userinfo`；10 种预设 + 4 字段独立策略 + 5 个组合全部可选且语义符合 §3.2 表。
- [ ] 流量边界：`total=0` 无限不被判成最少；全 none 时不输出响应头；`expire=0` 被忽略。
- [ ] 缓存命中不覆盖已存流量值。
- [ ] 拉取审计：每次刷新有记录，超 500 条自动裁剪。

**补齐的断层（本轮新增的验收）**
- [ ] **脚本**：`script.enabled=false` 时管线保存 `ref=` 直接报错、试运行 403，goja 一次都没跑；打开开关后 `/api/scripts/test` 返回 kept/before/after/duration_ms；删除被引用脚本被拒且列出引用者。
- [ ] **上游规则导入**：导入两次不产生重复行（`origin_key` 幂等）；导入结果全部 `enabled=0` 且带"候选"样式；分组名映射不上的行保留 `raw` + 红标；**没有自动建分组**。
- [ ] **规则源**：`/api/rule-sources/update` 后版本数增加且旧的被 prune 到 retention；rollback 后当前版本指针回退；MRS 生成对 `classical` 明确报错、对 domain/ipcidr 落盘成功。
- [ ] **任务与告警**：`/api/tasks` 返回已注册任务与上次结果；告警通道 secret **回读一律 `***`**；改其他字段不改 secret 时旧值保留；`SMTPChannel` 标注未实现。
- [ ] **设置**：模板保存前经后端语法校验，坏模板被拒并给行号；`script.enabled` 改动立即影响脚本页行为。
- [ ] **MCP**：要么 `subforge mcp` 命令可运行并在 usage 里列出，要么在文档和 UI 上明确"本轮不交付"——**不允许留零引用的死代码还不说明**。

**通用**
- [ ] 主题：light / dark / 跟随系统三种都正常，无硬编码颜色。
- [ ] 响应式：≥1280px 三栏不挤，≤768px 侧边栏收起。
- [ ] 原有功能不回退：`/sub`、`/render`、`/profile`、`/getruleset`、`/inspect`、`/api/convert`、`/api/diff` 全部行为不变。
- [ ] `go build ./... && go vet ./... && go test ./...` 全绿。
- [ ] 二进制内嵌资源齐全（`go:embed` 无遗漏），单文件可运行。

---

## 附：需要实现者二次确认的三个开放点

1. **管线步骤的结构化程度** — 见 §4.1 末尾。取决于 `internal/convert` 里管线动作是否可枚举。若不可枚举，退化为“有序原始行编辑”，不影响本 plan 其余部分。（现状：`internal/config/pipeline.go 的 PipelineStep.build()` 已枚举出 10 种，所以 §5.5 第 3 页按结构化表单写；只有 `src=` 内联脚本那一格需要退化。）
2. **`subscription-userinfo` 键名大小写** — mihomo/clash 读 `upload`/`download`/`total`/`expire` 小写；上游可能有 `Upload` 等变体。解析时统一 `strings.ToLower` 键名。
3. **MCP 服务器（`internal/mcp`）的去留** — 已实现且提交（a253aff），但**全仓零引用**：`cmd/subforge/main.go` 的 usageText 里没有 `mcp`，没有一个 `case "mcp"`。当前是死代码。实现者需要二选一：
   - **接线**：加 `cmd/subforge/mcp.go` + `case "mcp"` + usage 一行，让 `subforge mcp` 能起 stdio server；
   - **或明确不交付**：在 README / 设计书里标注，并从 `internal/mcp` 起删掉，避免"看起来有其实没有"。

   在没做决策之前，`/settings` 页不要列 MCP 相关开关。

