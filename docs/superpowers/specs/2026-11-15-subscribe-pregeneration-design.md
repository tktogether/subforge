# 订阅产物预生成设计书（/subscribe 拉取提速）

日期：2026-11-15
状态：待评审

## 意图与动机

`/subscribe?token=xxx` 当前每次拉取都完整实时编译：读 DB（机场/分组/规则）→
拉上游原文 → 解析 → 管线 → 渲染（`internal/api/subscription.go` 的
`handleSubscribe`，注释明确写着「不存在落盘的静态产物」）。

已有的提速措施：机场原文 TTL 缓存（默认 30 分钟）+ 后台每分钟预刷新 +
上游并发拉取 + 对上游的 ETag 条件请求。剩余成本有两块：

1. 缓存过期那一次的上游拉取（秒级，客户端直接感知）；
2. 每次拉取都重复执行的解析 + 管线 + 渲染（CPU 成本随节点数增长）；
3. 响应无压缩、对客户端无 304。

目标：默认组合的拉取降到毫秒级纯字节返回，同时**保持「改动后下一次拉取
即生效」的现有语义**——这是本次设计的硬约束，任何缓存都不得破坏它。

## 方案取舍

- **A 纯定时预生成**：cron 周期性渲染全部令牌产物，拉取直接返回。
  实现最简单，但控制台改动要等下一轮 cron 才生效，违背现有语义 → 否。
- **B 事件失效 + 惰性重建**：所有影响产物的写操作 bump 全局版本号；拉取时
  版本一致回缓存产物，不一致现编译并回填。语义与今天完全一致，但变更后
  第一次拉取仍然慢。
- **C 变更触发 + 后台急切重建**：在 B 的版本机制上，变更后异步重建全部
  令牌产物，拉取永远零编译。

**选定 B + C 叠加**：版本号失效是正确性准绳（B 保证「下一次拉取即生效」），
后台急切重建保性能（C 保证绝大多数拉取零编译），惰性回填兜底（急切重建
失败或竞态时，拉取现编译并回填，行为退化为 B，语义不变）。另附两条独立
措施一并纳入：gzip 响应压缩、对客户端的 ETag/304。

## 设计

### 1. 失效机制：全局产物版本号（B 的准绳）

- `config` 表新键 `subscribe.artifact_version`，整数计数器，原子自增
  （`UPDATE config SET value = value + 1`）。
- `subforge.Manager` 新增 `BumpArtifactVersion(ctx) error`，所有影响产物
  正文的写路径在成功后调用。

Bump 接线清单（应用层显式调用，不走 SQLite 触发器）：

| 类别 | 范围 |
| --- | --- |
| DB 写 | airports、self_nodes、proxy_groups、rules、rule_providers、profiles、sub_tokens、config（设置写入）、scripts、pipelines、source_pipelines 的增删改 |
| 文件写 | 模板保存（template_api）、override 保存（override_api）——落盘成功后 bump |
| INI 热重载 | `watchConfig`（serve.go）热替换基础模板/混合端口后 bump——两者都烤进产物正文，且代码注释承诺「改配置文件立即影响后续请求」 |
| 规则源 | `UpdateRuleSources` / `RollbackRuleSource` / manifest 当前版本指针变更后 |
| 后台 | source-refresh 任务**内容确实变化**时 bump（与旧缓存原文比对，304/无变化不 bump——空闲系统保持安静）；**刷新失败时也 bump 并唤醒重建**（见 §4，失败语义需要它收敛） |

- `BumpArtifactVersion` 失败只记日志、不让 API 写失败——漏一次 bump 退化为
  B 语义 + TTL 自愈，不是错误。
- 不在清单内的变更通道按 TTL 兜底：远程 URL 模板内容漂移（上游无本地钩子）、
  绕过 API 手改盘上模板/override 文件——最迟一个 TTL 内重建收敛。

- **例外**：机场 traffic 列更新（upload/download/total/expire/traffic_at/
  node_count）**不 bump**——流量只影响 `Subscription-Userinfo` 响应头，
  响应头在 serve 时现算（见 §3），正文产物不含流量数据。
- 为什么选应用层 bump 而不是 SQLite 触发器：traffic 列的排除、文件落盘与
  规则源产物（数据目录文件，触发器够不着）都需要应用层参与；触发器只能
  覆盖一半场景，反而造成「触发器 bump 了、文件没 bump」的两套口径。
- **兜底**：产物年龄超过 source refresh TTL 后无条件重建（§4 的 reconcile），
  未来漏接的 bump 最迟一个 TTL 内自愈，正确性不完全依赖接线完整性。

### 2. 产物存储

- 新表 `sub_artifacts`（幂等迁移，沿用 store.go 现有加表风格）：
  - `token TEXT PRIMARY KEY`（令牌值即主键）
  - `profile_id INTEGER NOT NULL DEFAULT 0`
  - `version INTEGER NOT NULL`（生成时读到的全局版本）
  - `body BLOB NOT NULL`（未压缩渲染结果）
  - `etag TEXT NOT NULL`（输入指纹哈希，见 §3）
  - `generated_at`（时间戳，供年龄判断）
- 回填用**版本守卫 upsert**而不是裸 INSERT OR REPLACE：仅当库内 version
  小于等于本次写入的 version 时覆盖，防止慢速在途重建把新鲜的惰性回填
  压回旧版本。
- 令牌删除时级联删除产物；令牌改绑方案按普通写路径 bump，下一轮重建覆盖。
- **不做内存 L1**：serve 时读一次 SQLite BLOB（几十~几百 KB）亚毫秒级，
  内存缓存引入的一致性问题不值当。列为后续可选优化，本期不做。

### 3. 拉取路径（handleSubscribe 改造）

- 前半段照旧：token/profile 解析、target/pipeline/config/include/exclude
  归并链（query > 令牌 > 方案 > 全局配置）。
- **默认组合判定**：query 未带 target/pipeline/config/include/exclude 任一
  覆盖参数。Web UI 生成的订阅链接是纯 `?token=`（endpoints.ts），默认组合
  覆盖实际流量；带覆盖参数的拉取走实时编译，不缓存（非目标，见下）。
- **命中**：artifact_mode=on 且默认组合且产物存在且 `version == 当前版本`
  → 直接回产物字节，加 ETag（`If-None-Match` 命中回 304），响应头现算：
  `Subscription-Userinfo`（现有聚合逻辑）、`profile-title`、
  `Content-Disposition`。
- **未命中**：照旧实时编译——现有 `engine.Run` 路径一行不动，失败策略、
  部分降级、最终组改写语义全部保留；成功且是默认组合时回填产物表
  （`INSERT OR REPLACE`，最后写者胜）。
- **gzip**：/subscribe 路由挂 chi `middleware.Compress`，按 Accept-Encoding
  自动压缩；YAML 产物文本压缩率高，慢链路收益明显。
- **ETag = 输入指纹，不是 body 哈希**：产物正文烤了「生成时间」时间戳
  （engine 每次编译写 now），body 哈希永远在变，304 永远命不中。改为
  `etag = sha256(版本号 + 各来源原文内容 + 有效参数指纹)`——输入不变则
  etag 不变，即使重建重写了时间戳，客户端也能 304，不重新下载。实时编译
  路径同样算指纹 etag（编译时本来就过了原文），非默认组合也能受益于 304。
  304 时客户端保留旧正文（时间戳停在旧值）， informational 注释行，可接受。

### 4. 后台急切重建（C 的性能保证）

- serve 装配 artifact-rebuild goroutine：single-flight（atomic.Bool，同
  SourceRefreshJob 惯例）；输入两个信号：wake channel（缓冲 1，多次 bump
  防抖合并）+ 定时 tick（每分钟，与 source-refresh 同节奏）。
- 每轮：读当前版本；列出全部订阅令牌（绑定已停用方案的跳过——拉取时本来
  就 400）；对每个令牌：产物缺失、version 落后或年龄超 TTL → 重建。
- **重建参数与拉取路径共用同一构建函数**：把 handleSubscribe 里「收集
  sources/groups/rules → 组 convert.Params」的部分抽成可复用函数，实时
  编译与预生成走同一份代码，防止两处漂移。
- 重建用后台 context + 与拉取相同的拉取阶段预算（s.cfg.SubscribeTimeout）。
- **编译失败分两类处理**：
  - 降级成功（默认 ignore 策略，部分来源失败仍产出）：照常写产物——上游
    故障期间今天的实时拉取也是产出同样的降级内容，等价。
  - 整体失败（failure_policy=error 的机场失败、全部来源失败、无来源——
    即实时路径会回 502/400 的情形）：**删除该令牌的产物**并记日志。拉取
    命中判定失败 → 回落实时编译 → 复现与今天逐字节相同的失败响应。
- **失败语义的收敛窗口**：上游故障只在机场缓存过期后才被感知（TTL 内
  今天也是用缓存原文，两边行为一致）；缓存过期后，source-refresh 每
  分钟的 tick 会发现刷新失败 → bump + 唤醒重建 → 产物删除/降级。从
  「今天会开始失败」到「产物路径同样失败」的窗口 ≤ 一个调度 tick。
- 启动时 wake 一次（预热），首个拉取前产物已就绪。

### 5. 模式开关与边界语义

- 设置键 `subscribe.artifact_mode`：`on`（默认）/ `off`。
- off 时拉取与重建都忽略产物表，行为与今天完全一致——排障逃生门，也保
  「出问题一键回到旧路径」。
- **`fetch.refresh_minutes = 0` 视为自动 off**：该设置的既有语义是「关闭
  缓存与后台刷新」，产物缓存同样是缓存，尊重这个显式选择——refresh=0 时
  命中判定直接失败、重建任务跳过，拉取照旧每次实时拉上游编译，行为不变。

### 6. 观测

- 命中/回填/重建各计一条日志（现有 logger 风格，带 rid）。
- 概览页展示产物状态与数量：本期不做，列为 follow-up。

## 目标 / 非目标 / 反目标

- **目标**：默认组合拉取从「秒级（缓存过期时）/几十毫秒（缓存温热）」降到
  毫秒级；改动后下一次拉取语义与今天完全一致。
- **非目标**：query 覆盖参数组合不预生成（回落实时编译）；CLI、
  `/api/convert`、preview 不缓存（交互式语义要求实时）；不做多实例一致性；
  不做内存 L1。
- **反目标**：绝不让预生成改变产物内容或失败语义——预生成只是「提前做的
  实时编译」，任何命中都必须等价于**同一份来源缓存状态下**的实时编译结果
  （来源缓存的 TTL 语义本身就是今天实时路径的一部分）。

## 约束

- 单进程单 SQLite，无集群场景。
- 迁移幂等：新表 + 新 config 键，老库升级无损（沿用现有迁移风格）。
- 测试遵循项目惯例（表驱动 + httptest），逻辑层覆盖率门禁维持 100%。
- 本机 Go 在 /tmp、无 make，验证在远端 192.168.8.112 或本地直接 go test。

## 留给实现者

- `sub_artifacts` 迁移与 store 层 CRUD 的具体写法。
- handleSubscribe 构建逻辑抽函数的具体切面（保持既有行为逐字节等价）。
- bump 接线核对：实现时 grep store 层 INSERT/UPDATE/DELETE 复核清单完整性。
- 测试切分与用例设计。

## 风险

- **bump 漏接** → TTL reconcile 兜底，最迟一个 TTL（默认 30 分钟）自愈；
  排障时 artifact_mode=off 一键回旧行为。
- **上游故障期间的失败语义**：整体失败类编译会删除产物，让拉取回落实时
  编译复现 502/400；收敛窗口 ≤ 一个调度 tick（见 §4）。这是评审发现的
  最关键语义点，实现时必须有对应用例覆盖。
- **预生成与拉取回填竞态** → 版本守卫 upsert（§2），不会回退到旧版本产物。
- **令牌多、节点多时重建 CPU** → 后台 single-flight + 防抖，与拉取路径
  隔离，不影响拉取延迟。
