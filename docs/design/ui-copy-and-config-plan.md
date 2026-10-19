# SubForge 界面文案与配置兜底计划

> 状态：定稿待施工　｜　依据：用户反馈（m21587）三问题 + 现有代码契约实测
> 关联：`docs/design/web-ui-rewrite-plan.md` §5.2/§5.5、`docs/design/vite-vuetify-coverage-plan.md`（技术栈与门禁权威）

---

## §0 三个问题：现象、根因、验收

| # | 用户原话 | 现象 | 根因（实测） | 验收信号 |
| --- | --- | --- | --- | --- |
| 1 | 「配置文件什么的路径，连默认配置都没有吗」 | 设置页不暴露 `rules.manifest_path`；规则源页一打开就报 500 | ① `Manager` 无 dataDir，`rules.manifest_path` 未配置时 `ListRuleSources` 直接返回 error → 500 `未配置规则源清单路径（rules.manifest_path）`；② `settingDefs` 里 4 项设置前端一个都没渲染 | 未配置时 `GET /api/rule-sources` 返回 **200** + 默认路径 + 空列表 + 引导；设置页可见并落盘 4 项设置；默认路径随 `-data-dir` 走 |
| 2 | 「界面太简单了，你要设置足够解释或者引导的文案」 | 13 页里有 3 页只有 21 行（纯 CRUD 壳）、4 页只有只读列表；全站无一条字段说明 | 文案从未被当作交付物：`ResourcePage` 只有 `emptyText` 一行字，`PageHeader` 只有 title/subtitle，无 hint/示例/空状态三段式 | 每页有「这页做什么 / 什么时候用 / 概念 / FAQ / 相关页面」；每个表单字段有 hint + 可照抄示例；空状态给出下一步按钮 |
| 3 | 「脚本管线到底怎么操作也不知道」 | 脚本页只有裸 textarea；管线页只有只读列表；沙箱语义（filter 是表达式、operate 是语句、无网络无文件）无处可查 | 缺「概念解释 + 可运行示例 + 试运行反馈 + 与订阅的关系」四件套 | 脚本页内置 4 个示例 + 试运行；管线页可增删排序步骤、可引用脚本、保存报错可见 |

---

## §1 问题一：配置路径默认兜底

### 1.1 后端：默认路径解析

**新增能力（`internal/subforge`）**

| 符号 | 签名 | 语义 |
| --- | --- | --- |
| `Manager.dataDir` | 字段 | 数据根目录 |
| `NewManagerWithDataDir` | `func NewManagerWithDataDir(dsn, dataDir string) (*Manager, error)` | 显式指定数据根目录 |
| `NewManager` | 保持 `func NewManager(dsn string) (*Manager, error)` | 内部调 `NewManagerWithDataDir(dsn, defaultDataDirFromDSN(dsn))`，**签名不变，存量测试零改动** |
| `defaultDataDirFromDSN` | `func defaultDataDirFromDSN(dsn string) string` | `filepath.Dir(dsn)`；dsn 为 `:memory:`/空/含 `?` 或目录为 `.` 时返回 `""` |
| `Manager.DataDir` | `func (m *Manager) DataDir() string` | 空表示「未指定，落 cwd」 |
| `Manager.ResolveRuleManifestPath` | `func (m *Manager) ResolveRuleManifestPath(ctx context.Context) (string, error)` | ① config `rules.manifest_path` 非空 → 用之；② `dataDir` 非空 → `filepath.Join(dataDir,"rules","manifest.yaml")`；③ 否则 `filepath.Join(os.Getwd(),"rules","manifest.yaml")` |
| `Manager.ResolveRulesOutputDir` | `func (m *Manager) ResolveRulesOutputDir(ctx context.Context) (string, error)` | 同上，key `rules.output_dir`，默认 `<dataDir>/rules`（`os.Getwd()/rules` 兜底） |
| `Manager.RuleManifestTemplate` | `func RuleManifestTemplate() string`（包级） | 返回可直接落盘的 manifest YAML 模板（含注释） |
| `Manager.EnsureRuleManifest` | `func (m *Manager) EnsureRuleManifest(ctx context.Context, force bool) (path string, created bool, err error)` | 文件不存在则写入模板；已存在且 `!force` → `created=false`；`force` 时先备份 `<path>.bak` 再覆盖 |

**改造点**

- `LoadRuleManifest(ctx)` 改为先 `ResolveRuleManifestPath`，**不再因为「未配置」报错**；只有「路径存在但读不到（非 fs.ErrNotExist）」才返回 error。
- `ListRuleSources(ctx)` 改为先 `ResolveRuleManifestPath`；`manifest.LoadManifest` 返回 `errors.Is(err, fs.ErrNotExist)` 时返回 `([]RuleSourceStatus{}, nil)` + `exists=false`。
- `cmd/subforge/serve.go`：`subforge.NewManagerWithDataDir(dbPath, dataDir)`（dataDir 已有变量）。

### 1.2 后端：/api/rule-sources 契约（改造后）

```text
GET /api/rule-sources
  200 {"manifest_path":"<解析后绝对路径>","default_manifest_path":"<同值>","output_dir":"<解析后>",
       "exists":true|false,"sources":[RuleSourceStatus...],"hint":"<仅 exists=false 时给出>",
       "manifest_template":"<模板 YAML 文本>"}
  500 {"error":"..."}          // 仅「路径存在但读取/解析失败」或存储层故障
POST /api/rule-sources/init    // body 可空；?force=1 覆盖
  200 {"path":"...","created":true|false}
  500 {"error":"..."}
POST /api/rule-sources/update   // 不变
POST /api/rule-sources/rollback // 不变
POST /api/rule-sources/mrs      // 不变
```

`hint` 文案（冻结）：`未找到规则源清单文件，可点击「生成默认清单」在本机创建一份模板，或在上方设置里改路径。`

### 1.3 后端：设置项可发现性

`SettingItem` 增两个字段（json 小写）：

```go
type SettingItem struct {
    Key         string `json:"key"`
    Value       string `json:"value"`
    Kind        string `json:"kind"`
    Label       string `json:"label"`
    Description string `json:"description"`
    Placeholder string `json:"placeholder,omitempty"` // path 类型：解析后的默认路径；bool 类型：空
    IsDefault   bool   `json:"is_default"`             // true = 当前用的是默认值（未显式配置）
}
```

- `ListSettings` 对 `path` 类型填 `Placeholder`（`Resolve*` 结果）、`IsDefault` = 配置为空；bool 类型 `Placeholder` 留空、`IsDefault` = 未配置。
- **`SetSettings` 语义微调**：值为空串时**删除该配置键**（回到默认），而不是写入空串。未知键仍报 `未知设置项 %q`。

### 1.4 前端表现

| 页面 | 改造 |
| --- | --- |
| `SettingsPage.vue` | 重写：从 `GET /api/settings` 渲染 `settings[]`；bool → `v-switch`；path/string → `v-text-field` + `placeholder=item.placeholder` + `persistent-hint`；每项显示 `label`/`description`；改动即 `PUT /api/settings`（回显刷新）；`is_default` 显示「使用默认值」chip |
| `RuleSourcesPage.vue` | 重写：空态（`exists=false`）显示 `hint` + 「生成默认清单」按钮（`POST /api/rule-sources/init`）+ 可展开的模板预览；非空态显示路径/输出目录/来源表（名称/URL/格式/最新版本/历史版本数）+ 更新/回滚/生成 MRS 行内操作 |

---

## §2 问题二：文案体系

### 2.1 写作规范（12 条硬规则，评审以此为准）

1. 中文；术语固定为「节点 / 订阅 / 机场 / 规则 / 规则集 / 管线 / 转换 / 令牌」。
2. 每页第一句「这页做什么」，第二句「什么时候用它」。
3. 空状态三段式：为什么是空的 → 点哪里（写出按钮名）→ 之后会看到什么。
4. 错误 actionable：`什么错了` + `怎么修`，不 blaming、不带「请」「麻烦」。
5. 每个表单字段必须有 `hint`（≤40 字，一行）与 `example`（可直接照抄的值）。
6. 危险操作按钮写明后果（删除会移除什么、回滚会切到哪个版本）。
7. 禁口语化：不出现「搞定/一键起飞/超简单/轻松」。
8. 出现协议术语（MRS、ETag、SNI、sing-box）时同句给一句解释。
9. 数值带单位与范围（例：「保留 5 个历史版本，范围 1–50」）。
10. 文案集中放在 `web/src/content/help.ts`，**不允许散落在模板字符串里**。
11. 每段关键文案对应一个 `data-testid`，便于 spec 断言与覆盖率统计。
12. 同一概念在全站只有一份定义（`GLOSSARY`），其他页面引用不重写。

### 2.2 `web/src/content/help.ts`（冻结接口）

```ts
export interface HelpLink { label: string; to: string; note?: string }
export interface HelpStep { title: string; detail: string }
export interface HelpConcept { term: string; explain: string }
export interface HelpFaq { q: string; a: string }
export interface EmptyStateHelp { title: string; detail: string; action?: { label: string; to?: string } }

export interface PageHelp {
  page: string
  title: string
  oneLiner: string
  whenToUse: string
  steps?: HelpStep[]
  concepts?: HelpConcept[]
  faq?: HelpFaq[]
  emptyState?: EmptyStateHelp
  links?: HelpLink[]
}

export const PAGE_HELP: Record<string, PageHelp>
export function pageHelp(key: string): PageHelp            // 未知 key 返回通用兜底对象，不抛
export const GLOSSARY: HelpConcept[]
export function glossaryFor(term: string): string          // 命中返回 explain，未命中返回 ''
export const FIELD_HELP: Record<string, { hint: string; example?: string }>
export function fieldHelp(key: string): { hint: string; example?: string } | undefined
export interface ScriptVariable { name: string; type: string; writable: boolean; explain: string }
export interface ScriptReturnRule {
  kind: 'filter' | 'operate'
  body: string   // 脚本体应该长什么样
  ret: string    // 返回值怎么被处理
  how: string    // 怎么生效
  good: string   // 正确示例
  bad: string    // 错误示例 + 为什么错
}
export const SCRIPTS_HELP: {
  intro: string
  sandbox: string[]                 // ≥4 条能力边界
  variables: ScriptVariable[]       // 变量表，见 §3.1 ①
  returnRules: ScriptReturnRule[]   // filter / operate 各一条，见 §3.1 ②
  writeback: string[]               // ≥3 条写回规则，见 §3.1 ③
  examples: { name: string; kind: 'filter' | 'operate'; code: string; why: string; caution?: string }[]
}
export const PIPELINE_HELP: {
  intro: string
  order: string[]
  steps: { kind: string; label: string; hint: string; example: string }[]
  scriptStep: string                // 脚本步骤的 ref=<脚本名> 写法说明
}
```

### 2.3 组件契约（共享文件，单独任务）

| 组件 | 变化 | 兼容性 |
| --- | --- | --- |
| `HelpPanel.vue`（新增） | props `{ help: PageHelp; dense?: boolean }`；渲染 `oneLiner`/`whenToUse`，`steps`/`concepts`/`faq`/`links` 折叠区；根节点 `data-testid="help-panel"`，子块 `help-steps`/`help-concepts`/`help-faq`/`help-links` | 新增，无破坏 |
| `PageHeader.vue` | props 增 `helpKey?: string`；给了就渲染 `<HelpPanel :help="pageHelp(helpKey)">` | `title`/`subtitle` 保持必填不变；现有调用零改动 |
| `ResourcePage.vue` | `fields` 元素增 `hint?: string; example?: string`（缺省从 `fieldHelp(field.key)` 取）；`emptyText?: string` 保留，新增 `empty?: EmptyStateHelp`（优先）；渲染 `data-testid="field-hint"` 与 `empty-state` | 全部新增可选 props |
| `AsyncSection.vue` | 新增可选 `hint?: string`，加载失败时展示 | 新增可选 prop |

### 2.4 逐页必备内容清单

| 页面 | 现状（行数） | 本次必须补 |
| --- | --- | --- |
| OverviewPage | 60 | 「这页做什么」+ 指标释义（订阅数/机场数/规则数/成功率各自口径）+ 首次使用路径 |
| ConvertPage | 101 | target 取值含义表、url 参数说明、diff 结果读法 |
| PipelinesPage | 50 | **重写**：概念（filter 链→operate 链）、步骤可视化、增删排序、引用脚本、编译错误展示 |
| AirportsPage | 21 | **重写**：订阅地址含义、流量字段口径（上传/下载/总量/到期/节点数/更新时间）、刷新与导入规则按钮、测活结果、空状态 |
| SelfNodesPage | 21 | 自建节点与机场节点的区别、各字段 hint、示例 |
| GroupsPage | 21 | 分组如何影响订阅输出、排序、示例 |
| RulesPage | 228 | 规则来源（手工/导入）、规则集与规则的区别、批量启用说明、Rule-Bot 建议是什么 |
| SubscriptionsPage | 67 | **重写**：令牌与订阅 URL 关系、流量聚合策略（10 种预设 + 4 字段独立策略）、策略试算、风险提示 |
| RuleSourcesPage | 104 | **重写**：空态引导 + 生成模板、版本保留策略、回滚后果、MRS 格式说明 |
| ScriptsPage | 169 | **重写**：沙箱能力边界、filter/operate 差异、4 个示例、试运行反馈读法 |
| TasksPage | 66 | 任务含义、cron 说明、手动触发后果 |
| LogsPage | 59 | 日志级别、时间范围、跟到底怎么用 |
| SettingsPage | 41 | **重写**：4 项设置逐条解释 + 默认值展示 + 落盘提示 |

### 2.5 验收

- 每个 `*Page.spec.ts` 至少一条断言 `[data-testid="help-panel"]` 文本包含 `oneLiner` 片段。
- `web/src/content/help.ts` 与 `HelpPanel.vue` 函数/行/语句覆盖 100%（vitest 阈值）。
- 人工评审：13 页任意页面，不看源码能回答「这页做什么 / 第一步点哪」。

---

## §3 问题三：脚本与管线引导

### 3.1 脚本语义（实测冻结，示例脚本必须遵守）

> **用户明确要求（m21707）**：「脚本应该写清楚我能拿到什么变量，应该处理返回什么」。
> 因此脚本页必须**逐字**给出下面两张表——变量表 + 返回值表——不能只给一句「写 JS」。

**① 我能拿到什么变量（作用域里只有 `node`，没有别的）**

| 变量 | 类型 | 读 | 写 | 说明 |
| --- | :---: | :---: | :---: | --- |
| `node.name` | string | ✅ | ✅ | 节点名，派生自原始配置的 `name` |
| `node.type` | string | ✅ | ✅ | 协议类型：`ss` / `ssr` / `vmess` / `vless` / `trojan` / `hysteria2` / `anytls` / `tuic` 等 |
| `node.server` | string | ✅ | ✅ | 服务器地址（域名或 IP） |
| `node.port` | **string** | ✅ | ✅ | 端口。**是字符串**（如 `'443'`），不是数字；写回时也会被转成字符串 |
| `node.source` | string | ✅ | ❌ | 节点来源标识（机场名或导入批次），只读，用于溯源 |
| `node.raw` | object | ✅ | ✅ | 该节点的**完整原始配置**。协议相关字段全在这里：`uuid` / `id` / `password` / `sni` / `servername` / `alterId` / `cipher` / `ws-opts` / `tls` / `network` / `skip-cert-verify` … |

补充事实（都要在页面上说明）：

- **没有 `require` / 网络 / 文件 / 进程 / `console`**，只有纯数据对象 `node`。
- **每个节点一个全新的执行环境**：节点之间**不能共享状态**，不能靠全局变量跨节点累加计数。
- 引擎是 goja（ES5.1+），支持 `var` / `if` / `for` / `function` / 数组对象方法；不支持 `import` / `export` / `async` / `await`。
- 执行上限 5 秒（可配置），死循环会被 `vm.Interrupt` 打断 → `脚本 %q 被中断（超时或死循环）`。

**② 我该处理/返回什么（filter 与 operate 完全不同，这是最容易踩的坑）**

| 类型 | 脚本体形态 | 返回值怎么处理 | 生效方式 | 正确示例 | 错误示例 |
| --- | --- | --- | --- | --- | --- |
| `filter` | **表达式**（一个值） | 取整个表达式的**完成值**并 `.ToBoolean()`：真 → 保留该节点，假 → 丢弃 | 返回值即结果 | `node.name.indexOf('香港') >= 0` | `return node.name.indexOf('香港') >= 0`（顶层 `return` 是语法错误，编译期就报 `脚本 %q 编译失败`） |
| `operate` | **语句块** | **返回值被完全忽略** | 必须**赋值**给 `node.*` 才生效 | `node.name = '[HK] ' + node.name` | `node.name.indexOf('HK') >= 0`（只算了个布尔值，什么都没改，节点原样输出） |

**③ `operate` 的写回规则（决定「我改了到底算不算数」）**

- 顶层字段 `node.name` / `node.type` / `node.server` / `node.port` 写回后**覆盖** `node.raw` 里的同名字段。
- 其他协议字段必须写 `node.raw.xxx`，例如 `node.raw.sni = 'example.com'`、`node.raw['ws-opts'].path = '/ws'`。
- 写回后由 `model.NodeFromRaw` 从 `raw` 重新派生只读视图，**`raw` 永远是唯一事实源**。
- 只读字段 `node.source` 改了不生效。

**④ 试运行会告诉我什么**

`POST /api/scripts/test` 的响应（`internal/api/scripts_test_api.go`）必须在前端呈现为：

| 字段 | 含义 | 前端怎么展示 |
| --- | --- | --- |
| 保留判定 | filter 脚本对该节点的真假 | filter 显示「保留 ✅」或「丢弃 ⛔」 |
| 结果节点 | operate 执行后的 `name/type/server/port` + `raw` | 左右对照：改前 / 改后，变化字段高亮 |
| `err` | 编译/运行/超时错误原文 | 原样展示错误文本 |

（**W1 的 ScriptsPage 任务书里以实际响应字段为准**；若后端返回字段与上表不一致，以实测响应为准并在交付说明里写明差异。）

**内置 4 个示例（冻结文案与代码）**

| # | 名称 | kind | 代码 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | 按地区保留节点 | filter | `node.name.indexOf('香港') >= 0 \|\| node.name.indexOf('HK') >= 0` | filter 脚本要返回布尔表达式；`\|\|` 两侧任一命中即保留 |
| 2 | 给节点名加地区前缀 | operate | `node.name = '[HK] ' + node.name` | operate 靠赋值生效，返回值被忽略 |
| 3 | 统一改端口 | operate | `node.port = 443` | 改顶层字段即可；`raw.port` 会自动同步 |
| 4 | 改写 SNI | operate | `if (node.raw.sni) { node.raw.sni = 'example.com' }` | 非顶层字段走 `node.raw`；先判存在避免给无 SNI 的节点写入 |

### 3.2 ScriptsPage 结构

1. `PageHeader helpKey="scripts"` + HelpPanel（沙箱边界、filter/operate 差异）。
2. 脚本列表：名称 / 类型 / 启用 / 更新时间 / 行内「编辑」「试运行」「删除」。
3. 编辑抽屉：名称 + 类型下拉（含差异说明）+ 代码 textarea + 「插入示例」下拉（4 个内置）+ 保存。
4. 试运行区：`kind` + 节点 JSON（缺省用内置样例）+ 超时 → 调 `POST /api/scripts/test`；展示「保留 / 丢弃」、结果节点 JSON、错误原文。

### 3.3 PipelinesPage 结构

1. HelpPanel：管线是什么（filter 链 → operate 链 → 输出）、执行顺序、与订阅的关系、脚本步骤的 `ref=` 写法。
2. 管线列表：名称 / 步骤数 / 来源（UI 覆盖 or 模板只读）。
3. 编辑：步骤有序列表，可增删上下移；每步 `kind` 下拉（值来自 `PIPELINE_HELP.steps`）+ 参数输入 + hint/example；`script` 步骤从 `listScripts()` 选 `ref`，并显示该脚本是否存在/启用。
4. 保存：调 `POST|PUT /api/pipelines`，后端 message 原样展示。

### 3.4 交叉链接（`PageHelp.links`）

ScriptsPage ↔ PipelinesPage ↔ SubscriptionsPage ↔ ConvertPage 两两互链，`note` 写清「去那边做什么」。

### 3.5 验收

- 脚本页 4 个示例可一键插入；试运行对示例 1 返回「保留」、示例 2 返回改名后的节点。
- 管线页能创建含 `ref=<脚本名>` 的步骤并保存成功；引用不存在的脚本时后端错误可见。

---

## §4 施工顺序与文件归属

**纪律**：每条任务一个写范围，两两不重叠；`internal/api/server.go` 只在 §1 任务里改一次；每个前端任务必须自带 spec 并让 `pnpm test` 阈值（100/100/100/80）通过。

### Wave 0 —— 基础设施（3 条并行）

| 任务 | 归属 | 写范围 |
| --- | --- | --- |
| W0-1 help.ts + 共享组件 | implementer-client | `web/src/content/help.ts`(+spec)、`web/src/components/HelpPanel.vue`(+spec)、`PageHeader.vue`(+spec)、`ResourcePage.vue`(+spec)、`AsyncSection.vue`(+spec) |
| W0-2 前端端点与格式化 | implementer-web | `web/src/api/endpoints.ts`(+spec)、`web/src/format/index.ts`(+spec) |
| W0-3 后端默认路径兜底 | implementer-format | `internal/subforge/{subforge,rulesources,settings}.go` + 测试、`internal/api/{rule_sources,settings}.go` + 测试、`cmd/subforge/serve.go` |
| W0-3b 规则源路径解析修正 | implementer-format | `internal/manifest/manifest.go`(+测试)、`internal/subforge/rulesources.go`(+测试)、`internal/api/rule_sources.go`(+测试) |

> **W0-3b 缘由（复核 W0-3 时发现）**：① `manifest.Update` 按**进程 cwd** 解析相对 `output.path`，而默认清单落在 `<dataDir>/rules/`，导致「写」与「读」都跑出 dataDir（serve 的 cwd 是 `/`）；② `default_manifest_path` 恒等于 `manifest_path`，前端无法展示默认位置。修正为：`LoadManifest` 把相对 `output.path` 相对 manifest 文件所在目录解析成绝对路径；新增包级 `DefaultRuleManifestPath()` 与 `Manager.ResolveRuleManifestPath(ctx)` / `Manager.ResolveRulesOutputDir(ctx)`（配置优先 → 默认位置兜底）。
>
> **W0-1 补丁**：`ResourcePage` 需补 `helpKey?: string` 透传给 `PageHeader`，否则 GroupsPage / SelfNodesPage 这两个壳页永远没有 HelpPanel（plan §2.5 要求每页 spec 断言 `[data-testid="help-panel"]`）。

### Wave 1 —— 核心页（5 条并行，每页一文件）

| 任务 | 页面 | 归属 | 共享任务 |
| --- | --- | --- | --- |
| W1-1 | ScriptsPage | implementer-client | task-135 |
| W1-2 | PipelinesPage | implementer-web | task-136 |
| W1-3 | SettingsPage | implementer-cov | task-137 |
| W1-4 | RuleSourcesPage | implementer-gate | task-138 |
| W1-5 | AirportsPage | implementer-format | task-139 |

任务号对照：W0-1 = task-131、W0-2 = task-132、W0-3 = task-133、W0-3b = task-134。

### Wave 2 —— 次要页（4 条并行，每页一文件）

| 任务 | 页面 | 归属 | 共享任务 |
| --- | --- | --- | --- |
| W2-1 | SubscriptionsPage（含流量聚合策略 UI） | implementer-cov | task-140 |
| W2-2 | ConvertPage（**含结果框真缺陷修复**） | implementer-web | task-141 |
| W2-3 | OverviewPage（含指标口径） | implementer-cov | task-142 |
| W2-4 | RulesPage（四区块 + 修「启用本机场全部」缺陷） | implementer-client | task-143 |

> **W2-2 真缺陷**：`endpoints.ts` 原声明 `ConvertResponse{output,groups,rules,...}`，而后端 `handleAPIConvert`（`internal/api/web.go:281-289`）返回的是 `{body,format,nodes,rawNodes,failures,warnings,cached}`。`ConvertPage.vue` 读 `result.output` → 恒 undefined → **结果框永远空白**。已修类型，本任务负责让页面改用 `result.body`。
>
> **W2-4 真缺陷**：`RulesPage.vue` 的「启用本机场全部」调 `bulkRules({source:'airport:'})`，但后端 `internal/api/rules_bulk.go:52-56` 是**精确相等**匹配（`rule.Source == body.Source`），而导入规则的 source 形如 `airport:香港 IEPL` → `updated` 恒为 0。修法：客户端按前缀取 ids 再 bulk（不改后端）。

### Wave 3 —— 收尾页（2 条并行，一任务两文件）

| 任务 | 页面 | 归属 | 共享任务 |
| --- | --- | --- | --- |
| W3-1 | TasksPage + LogsPage | implementer-format | task-144 |
| W3-2 | GroupsPage + SelfNodesPage | implementer-gate | task-145 |

> **W3-2 前提**：`ResourcePage` 的 `helpKey` 透传（W0-1 补丁）必须先落地，否则这两个壳页没有 HelpPanel。

---


### Wave 4 —— 失败与降级语义（F0–F4）+ 返工与门禁（FIX/CI-3）

| 任务 | 内容 | 归属 | 共享任务 |
| --- | --- | --- | --- |
| F0 | help.ts 失败语义文案（FAILURE_HELP / ON_ERROR_HELP / FIELD_HELP 增补） | implementer-client | task-147 |
| F1 | AirportsPage 流量失败态 + TasksPage 重试说明 | implementer-format | task-148 |
| F2 | SubscriptionsPage 余额刷新逐行 err | implementer-cov | task-149 |
| F3 | RuleSourcesPage 逐行 err + 保留上一版本 + 回滚后果 | implementer-gate | task-138 |
| F4 | PipelinesPage on_error 三策略 + ConvertPage failures/warnings 分区 | implementer-web | task-136 / task-141 |

> **F4 拆解**：on_error 部分随 W1-2（task-136）落地；ConvertPage 的 failures/warnings 分区随 W2-2 落地，但 W2-2 当时**只交付了这一个分区**，其余 6 项（help-key / target 含义表 / url 说明 / types / diff 读法 / 下载）缺失 → 由 task-151 补齐。

### Wave 4b —— 返工与门禁（Lead 发现，2026-10-19）

| 任务 | 内容 | 归属 | 共享任务 |
| --- | --- | --- | --- |
| FIX-2 | 修 RouterLink stub 的 inject() 回归（NotFoundPage push 失效） | implementer-cov | task-150 |
| W2-2 补齐 | ConvertPage 重写（111 → 约 300 行，补 6 项缺失） | implementer-format | task-151 |
| W2-1b | 订阅页补两条流量真语义（预设忽略字段级 / 字段策略跳 0 值） | implementer-cov | task-152 |
| FIX-3 | help.ts 三处事实性错误（默认 target 写成 sing-box、MRS 归属写成 sing-box）+ 全文通读纠错 | implementer-client | task-153 |
| FIX-4 | 去掉 PageHeader 上多余的 data-testid="help-panel"（Vue 丢弃该属性并告警） | implementer-format | task-154 |
| CI-3 | 前端覆盖率门禁真正进 CI / Makefile / verify.cmd | implementer-format | task-155 |

> **FIX-3 的三处事实错误（Lead 读源码核对）**：① 默认 target 是 `clash.meta`（`internal/api/auto.go:16`），help.ts 三处写成 sing-box；② MRS 是 mihomo/Clash.Meta 的二进制规则集（`internal/mrs/mrs.go` 包注释），help.ts 两处写成 sing-box 的格式；③ 其余表述要求逐行对照源码复核。
>
> **CI-3 的根因**：`package.json` 的 `test` 脚本不带 `--coverage`，三条执行路径全用它 → 阈值从未生效。


## §5 验收与部署

### 5.1 前端

```text
node <pnpm.mjs> run typecheck      # vue-tsc --noEmit，0 错误
node <pnpm.mjs> run coverage       # vitest run --coverage：全绿 + 阈值 100/100/100/80
```
> **门禁口径纠正（2026-10-19）**：`package.json` 里 `test` 是 `vitest run`（**不带 --coverage，阈值不生效**），带覆盖率的脚本叫 `coverage`。CI 的 web job、`make verify`、`scripts/verify.cmd` 原先跑的都是 `test`，等于前端门禁从未真正生效——由 CI-3 修正。
```
node <pnpm.mjs> run build          # vite build → ../internal/api/web/dist
```

### 5.2 后端

```text
gofmt -l cmd internal pkg          # 无输出
go build ./... ; go vet ./...      # exit 0
go test ./... -count=1             # 0 FAIL
coveragegate -profile coverage.out -min 80 -min-func 100   # 函数 100%
condgate  -engine=profile -profile coverage.out -min 80    # 条件 >= 80%
```

### 5.3 端到端手工核对

1. `subforge serve -data-dir <tmp>` 冷启动 → 设置页显示 4 项设置与默认路径占位符。
2. 规则源页首次打开：200、空列表、`hint` 可见；点「生成默认清单」→ 文件落盘 → 列表出现来源。
3. 脚本页插入示例 2 并试运行 → 节点名出现 `[HK] `。
4. 管线页建一条含 `ref` 的管线并保存 → 订阅页选该管线 → `/sub` 产物体现改动。
5. 订阅页配置流量聚合策略 → 预览显示 `subscription-userinfo` 头。

### 5.4 部署

重新交叉编译 → `tools/rsh210.py` 上传 → 原子替换 `/tmp/subforge` → 同参数重启 → 验收（沿用 2026-10-19 的部署流程与回滚路径）。



### 5.5 部署验收证据（2026-10-19 第二次部署）

把本轮改动（配置默认兜底 + 13 页文案引导 + 失败降级语义 + 前端覆盖率门禁接线）部署到测试机 `210.us1.firadio.net:21009`，沿用 5.4 的流程。

| 项 | 值 |
| --- | --- |
| 本地构建 | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -mod=vendor -ldflags "-s -w -X ...Version=dev -X ...Commit=bb851bf -X ...Date=2026-10-19T03:12:56Z" -o bin/subforge-linux-amd64 ./cmd/subforge` |
| 新二进制 | 39,989,410 B，sha256 `dfb7c6bcea3e0cfb087b7f3dd6eb7b50f65b93e8458cbcf844abb167f2e4209b`，`version` → `subforge dev (commit bb851bf, built 2026-10-19T03:12:56Z)` |
| 被替换的版本 | commit `3710d50`（39,878,818 B，sha256 `76e0cfec…`） |
| 备份 | `/tmp/subforge.bak-20261019T032001Z`、`/tmp/subforge-data/subforge.db.pre-bb851bf` |
| 重启方式 | 同参数裸进程：`cd /root && setsid nohup /tmp/subforge serve -listen 0.0.0.0:21009 -data-dir /tmp/subforge-data >> /tmp/subforge.log 2>&1 < /dev/null &`；新 pid 66069，`PPid=1`、`NSsid=66069`、`cwd=/root`、fd0→/dev/null、fd1/2→/tmp/subforge.log（真正脱离 SSH 会话） |

**数据零丢失核对**（升级前后逐端点 `cmp`）：airports / groups / rules / tokens / self-nodes / pipelines / rule-providers / traffic.strategy = **8/8 字节完全一致**。
`stats` 有差异且**只差内存态计数器**：`history.total 1→0`、`logs 366→8`——历史缓冲（上限 20）与日志缓冲（上限 200）都在进程内存里，重启即清零，**不是数据丢失**（见 §2 对「最近记录条数」的口径说明）。

**两个用户报的问题在真实环境下的前后对比**：

| 端点 | 旧版 3710d50 | 新版 bb851bf |
| --- | --- | --- |
| `GET /api/rule-sources` | **500** `{"error":"未配置规则源清单路径（rules.manifest_path）"}` | **200** `{"default_manifest_path":"/tmp/subforge-data/manifest.yaml","exists":false,"hint":"未找到规则源清单文件，可点击「生成默认清单」在本机创建一份模板，或在上方设置里改路径。","manifest_path":…,"manifest_template":…,"output_dir":"/tmp/subforge-data/rules","sources":[]}` |
| `GET /api/settings` | 638 B，只有 4 项 key/value | 800 B，path 类补 `placeholder`（`/tmp/subforge-data/manifest.yaml`、`/tmp/subforge-data/rules`）+ 四项 `is_default:true` |
| `GET /` | 旧多页前端（`/app.css`、`/app.js`、`/js/*.js`） | 新 SPA 393 B：`<div id="app">` + `/assets/index-DaRdirkZ.js`(642,956 B) + `/assets/index-7f-yQz0k.css`(832,890 B)，无 `app.css` |

**端到端核对（对应 §5.3）**：

1. 冷启动后 `GET /api/settings` 返回 4 项 + path 占位符 ✅
2. `POST /api/rule-sources/init` → `{"created":true,"path":"/tmp/subforge-data/manifest.yaml"}`；再 GET → `exists:true`，`output_dir` 仍是 `/tmp/subforge-data/rules`（**不是 `rules/rules`**，证明相对 `output.path` 按清单自身解析）✅
3. 真实转换：`GET /sub?target=clash&url=http://127.0.0.1:21999/subforge-fixture.txt` → 200 / 1627 B，产物头部即 `commit bb851bf` ✅（fixture 用远端 `python3 -m http.server 21999` 提供，hermetic）
4. `GET /api/traffic/preview` → `{"header":"","ok":false,"usage":{…全 0}}` ✅ —— 与 §7.6 第 1、4 条一致：**首次刷新前不写 `Subscription-Userinfo` 头**
5. `/sub` 响应头无 `Subscription-Userinfo` ✅（同上，符合预期）

**新端点可达性**（新版）：`/api/tasks` `/api/scripts` `/api/alert-channels` `/api/suggestions` `/api/metrics` 均 200；`/api/template` 与 `/api/override` 404（路径未配置的业务 404，非路由缺失）；SPA 兜底 `/some/deep/link` 200；`/api/nope` 404 text/plain。公网 `http://210.us1.firadio.net:21009/` 与全部上述端点均 200。
升级后日志无 `error`/`panic`/`fatal`。

**回滚命令**：

```sh
kill $(pgrep -x subforge) && sleep 1
cp -a /tmp/subforge.bak-20261019T032001Z /tmp/subforge
cp -a /tmp/subforge-data/subforge.db.pre-bb851bf /tmp/subforge-data/subforge.db
cd /root && setsid nohup /tmp/subforge serve -listen 0.0.0.0:21009 -data-dir /tmp/subforge-data >> /tmp/subforge.log 2>&1 < /dev/null &
```

---

### 5.6 部署验收证据（2026-10-19 第三次部署：前端视觉升级 e82048b）

把「D1 Instrument 设计系统落地（W1–W6）」部署到测试机 `210.us1.firadio.net:21009`，沿用 §5.4 流程。

| 项 | 值 |
| --- | --- |
| 本地构建 | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -mod=vendor -ldflags "-s -w -X github.com/tktogether/subforge/internal/buildinfo.Version=dev -X github.com/tktogether/subforge/internal/buildinfo.Commit=e82048b -X github.com/tktogether/subforge/internal/buildinfo.Date=2026-10-19T08:37:45Z" -o bin/subforge-linux-amd64 ./cmd/subforge`（前置：`pnpm -C web run build` 已把前端打进 `internal/api/web/dist` 供 go:embed） |
| 新二进制 | 40,005,794 B，sha256 `e27f4d412c994a2470b9d43bc9aa4f7f35eb373c0a17748c766882e4533e52ba`，`version` → `subforge dev (commit e82048b, built 2026-10-19T08:37:45Z)` |
| 被替换版本 | commit `bb851bf`（39,989,410 B，sha256 `dfb7c6cb…`，pid 66069）→ 新 pid **73227** |
| 备份 | `/tmp/subforge.bak-20261019T083843Z`、`/tmp/subforge-data/subforge.db.pre-e82048b` |
| 上传校验 | SFTP 上传到 `/tmp/subforge.new` 后远端 `sha256sum` 与本地一致才 `chmod 0755` + `mv -f` 原子替换 |
| 重启方式 | 同参数裸进程：`cd /root && setsid nohup /tmp/subforge serve -listen 0.0.0.0:21009 -data-dir /tmp/subforge-data >> /tmp/subforge.log 2>&1 < /dev/null &` |

**数据零丢失核对**（升级前后逐端点 `curl | md5sum`）：airports `69e1babb…` / groups `c375ce57…` / rules `c9beaa90…` / tokens `5ad10464…` / self-nodes `a79ada4b…` / pipelines `58e0494c…` / rule-providers `9a44495f…` / traffic/strategy `6eced011…` = **8/8 完全一致**。
## 5.7 第四次部署验收（commit 3120fef → 测试机，2026-10-19 18:32 本地）

W7 修复波（7 点反馈 + P3 配色 + 文案对齐，W7A–W7M）部署，流程同 §5.6。

| 项 | 值 |
| --- | --- |
| 提交 | `3120fef` feat(web): W7 修复波——7 点反馈落地 + P3 配色 + 文案对齐（W7A-W7M），40 files +2327/−197（其中后端 Go 代码零改动，仅 web/** 与 docs/**） |
| 新二进制 | 40,009,890 B，sha256 `f344b3996579483092616fb068151e92763823e6dfaca10d054a5f442c032398` |
| 被替换版本 | commit `e82048b`（40,005,794 B，pid 73227）→ 新 pid **75673** |
| 备份 | `/tmp/subforge.bak-20261019T103234Z`、`/tmp/subforge-data/subforge.db.pre-3120fef` |
| 重启方式 | 部署脚本化（/tmp/deploy-w7.sh）：sha256 校验 → chmod → 备份 → kill 旧 pid → `mv -f` 原子替换 → `(setsid nohup … &)` 子壳启动，SSH 通道不再悬挂 |

**验收结果**：`version` → commit `3120fef`；`healthz` 200；groups `c375ce57…` / rules `c9beaa90…` / tokens `5ad10464…` / self-nodes `a79ada4b…` / pipelines `58e0494c…` / rule-providers `9a44495f…` 六端点 md5 与 e82048b 时点**完全一致 = 零丢失**；airports 与 traffic/strategy 两端点 md5 变化为**运行期合法变更**（用户 08:47Z 在测试机新增第二条机场、traffic 策略被操作过，非部署丢失）；`GET /` 引 `/assets/index-kFHihQ-D.js`（本次构建产物同名）；deep link 200；`/api/nope` 404；日志 grep error/panic/fatal **0 命中**。

**教训**：复验脚本路径拼错（`/api/traffic-strategy` 不存在，真路由 `/api/traffic/strategy`）会让「md5 一致」变成 404 响应体的假象——md5 对比必须先确认路由存在且 200。


**前端切换证据（本轮的重点）**：

| 检查 | 结果 |
| --- | --- |
| `GET /`（SPA 外壳） | 200 / 393 B，引用 `/assets/index-BqEBUwB6.js` |
| `/assets/index-BqEBUwB6.js` | **200 / 648,698 B**（与本地 `pnpm build` 产物同名同大小） |
| `/assets/index-BG5Oc447.css` | 200 / 837,627 B |
| 旧资源 `/assets/index-DaRdirkZ.js` | 404（已被替换，符合预期） |
| `/some/deep/link`（SPA 兜底） | 200 |
| `/api/nope` | 404 text/plain |
| 启动日志 | `tail -200 /tmp/subforge.log | grep -Ei "error|panic|fatal"` **无输出** |
| 公网可达 | `http://210.us1.firadio.net:21009/` 与 `/healthz` `/version` `/api/airports` 全部 **200** |

**真机截图证据**：用无头 Chrome（CDP，1600×1000，写 `localStorage['subforge.theme']` + 模拟 `prefers-color-scheme`）直接拍**公网 URL**，产物入库 `docs/design/screenshots/prod-210-{airports-light,airports-dark,overview-light,groups-dark}.png`。深色主题在真实环境同样生效（含固定顶栏，无首帧闪白残留）；帮助面板在真实页面收成「使用说明」一行。

**本轮发现的遗留观感问题（未改，待拍板）**：机场页只有 1 条真实机场时，`traffic_err` 原文同时在「流量(已用/总量)」与「状态」两列出现，各占 3–5 行、把该行撑到约 130px。这与 §6.2「错误原文必须完整呈现」的既有口径冲突（限行会削弱该验收点），故保留原样并记录。

**回滚命令**：

```sh
kill $(pgrep -x subforge) && sleep 1
cp -a /tmp/subforge.bak-20261019T083843Z /tmp/subforge
cp -a /tmp/subforge-data/subforge.db.pre-e82048b /tmp/subforge-data/subforge.db
cd /root && setsid nohup /tmp/subforge serve -listen 0.0.0.0:21009 -data-dir /tmp/subforge-data >> /tmp/subforge.log 2>&1 < /dev/null &
```

> 工具链提醒：`tools/rsh210.py run` 把远端输出直接写本机 stdout，**远端日志里的非 GBK 字符会让它在 Windows 控制台抛 `UnicodeEncodeError`**（本次 tail 日志时踩到）。取远端日志要加 ASCII 过滤：`tail -8 /tmp/subforge.log | tr -cd '\11\12\15\40-\176'`。另：脚本里的 `setsid nohup … &` 会让 SSH 通道保持打开直到超时（paramiko 报 `TimeoutError`），**部署脚本已执行成功但调用方可能报超时**，验收要用独立的只读检查脚本复核，不要凭调用方返回码判断。

---

## §6 失败与降级语义（后端已定，UI 必须讲清楚）

> 依据 `docs/design/m2-m4-design.md`：§1.1「单步失败可 skip/abort/warn 三策略」、§1.3「部分失败 → 成功节点仍输出 + warn 日志」、§2.1「失败处理 → 保留上一版本 + warn」、§2.3「失败重试 3 次指数退避」、§2.5「测活默认关闭」。
> **本节的每一条都已在后端实现；缺的是 UI 把它讲出来。**

### 6.1 后端实际语义（Lead 读源码逐字核对，2026-10-19）

| 场景 | 后端行为 | 证据 |
| --- | --- | --- |
| 转换时某个机场拉取失败 | **该来源记入 failures，其余来源照常输出** | `internal/convert/engine.go:135-154`：`res.Err != nil` → `failures = append(...)` + `continue` |
| 转换时全部来源失败 | 502，正文 `所有来源都失败:\n  <每条原因>` | `internal/convert/engine.go:170-176` |
| 来源成功但过滤后无节点 | 404 `没有可用节点：检查来源内容，或放宽 include/exclude/type 条件`（`ErrNoNodes`） | `internal/convert/engine.go:27-28,188-190` |
| 转换响应的两个提示数组 | `failures` = 失败来源原因（部分成功时非空）；`warnings` = 渲染降级项 + 模板告警 + 管线告警 + 覆盖告警，**模板告警排最前** | `internal/convert/engine.go:61-65,208-228` |
| 流量余额刷新：单机场失败 | **只写 `traffic_err`，不动 upload/download/total/expire**；写 `fetch_audit`；单机场失败不炸整批 | `internal/subforge/traffic.go:114-133`（注释原文「记审计 + 只写 traffic_err（不动流量值）」「错误一律吞进结果里，不让单个机场炸掉整批」） |
| 流量刷新返回 | `{"results":[{airport_id,name,ok,usage,node_count,err}],"ok":N,"fail":M}` | `internal/subforge/traffic.go` 的 `TrafficRefreshResult` |
| 余额新鲜度 | `traffic_at` 只在成功且拿到 `subscription-userinfo` 时更新 → `now - traffic_at` 即陈旧度 | `internal/store/store.go` Airport 列；`internal/subforge/traffic.go` 成功分支 |
| 规则源更新：单源失败 | `UpdateResult.Err` 非空，**该源保留上一版本**；版本保留最近 5 个 | `internal/manifest/manifest.go` 的 `Update`/`pruneVersions`；`internal/api/rule_sources.go` 把 Err 转字符串 |
| 规则导入拉取失败 | 502 `导入失败: <err>` | `internal/api/import_rules.go` |
| 调度任务失败 | 重试 3 次（指数退避），仍失败才告警；`JobResult.Err` 非空 | `internal/scheduler` |
| 管线单步失败 | `on_error` 段落级默认（空 = abort），单步 `on_error=` 覆盖；值 ∈ `skip/abort/warn`，非法值报错 | `internal/config/pipeline.go:24-48,68-86,175-207` |
| 脚本失败 | 试运行 HTTP 200 + `ok:false` + error 原文；管线内按该步 `on_error` 处理 | `internal/api/scripts_test_api.go` |

### 6.2 各页必须呈现（验收点）

| 页面 | 必须写清 | testid |
| --- | --- | --- |
| AirportsPage | ① `traffic_err` 非空 → 错误原文 + 「余额仍是上次成功值，没有被清零」；② `traffic_at` 为空 → 「从未采集」；③ 非空 → 「上次成功：<绝对时间>（<相对时间>前）」；④ 刷新结果表逐行显示 ok/err | `traffic-error` `traffic-stale` `traffic-at` |
| ConvertPage | failures 与 warnings **分区**展示原文；一句话解释「部分来源失败不影响其他来源的节点」；全部失败时保留 502 原文并引导检查 URL | `convert-failures` `convert-warnings` |
| SubscriptionsPage | 余额刷新失败逐行 err + 「失败不会把下游余额清零，只标记错误」 | `refresh-error` |
| RuleSourcesPage | 更新结果逐行显示 err + 「该源保留上一版本，产物不受影响」；版本列表显示保留策略 | `source-update-error` |
| PipelinesPage | 每步 `on_error` 下拉（skip/abort/warn）+ 三值逐条含义 + 默认 abort | `step-on-error` |
| TasksPage | 「失败自动重试 3 次（指数退避），仍失败才告警」 | `task-retry-note` |
| ScriptsPage | 已有：试运行错误原文 | — |

### 6.3 help.ts 增补（冻结接口）

新增导出：

`export interface FailureCase { scene: string; behavior: string; ui: string }
export const FAILURE_HELP: { intro: string; cases: FailureCase[] }
export const ON_ERROR_HELP: { value: 'skip' | 'abort' | 'warn'; label: string; effect: string }[]`

`FAILURE_HELP.cases` 至少覆盖 §6.1 的 11 行场景（`scene` = 场景，`behavior` = 后端行为，`ui` = 用户该看到什么）。
`ON_ERROR_HELP` 恰好 3 条：`skip`「跳过该步，保留已处理结果，产物继续生成」/ `abort`「中止整次转换并报错」（默认）/ `warn`「记录告警，继续执行后续步骤」。
FIELD_HELP 增补：`airport.traffic_err`、`airport.traffic_at`、`pipeline.step.on_error`。

### 6.4 施工归属

| 任务 | 归属 | 文件 |
| --- | --- | --- |
| F0 help.ts 失败语义文案 | implementer-client | `web/src/content/help.ts`(+spec) |
| F1 AirportsPage 失败态 + TasksPage 重试说明 | implementer-format | `web/src/pages/AirportsPage.vue`(+spec)、`TasksPage.vue`(+spec) |
| F2 SubscriptionsPage 余额失败说明 | implementer-cov | `web/src/pages/SubscriptionsPage.vue`(+spec) |
| F3 RuleSourcesPage 单源失败保留上一版本 | implementer-gate | `web/src/pages/RuleSourcesPage.vue`(+spec) |
| F4 PipelinesPage on_error + ConvertPage failures/warnings | implementer-web | `PipelinesPage.vue`(+spec)、`ConvertPage.vue`(+spec) |

## §7 流量处理（订阅余额与到期）——从上游响应头到下游客户端

> 触发：用户问「流量处理在哪里呢？」。它横跨 5 个包，此前在本文档只有散点（§2.4 的 AirportsPage/SubscriptionsPage 行、§5.3 第 5 条、§6.1 三行）。本节把它收敛成一等章节：采集 → 存储 → 聚合 → 透传 → 呈现，外加口径与边界。

### 7.1 全链路（6 段，每段一个源码落点）

| # | 阶段 | 做什么 | 源码落点 |
| --- | --- | --- | --- |
| 1 | 采集 | 拉机场订阅时读上游响应头 `Subscription-Userinfo` | `internal/fetch/fetcher.go:268` |
| 2 | 解析 | `ParseSubscriptionUserinfo` → `Usage{Upload,Download,Total,Expire}`；ok=false 只表示「这条头不可用」，不是错误 | `internal/traffic/traffic.go:18+` |
| 3 | 存储 | Airport 表 7 列 `upload/download/total/expire/node_count/traffic_at/traffic_err`；每次刷新追加一行 `fetch_audit` 审计 | `internal/store/store.go:61` |
| 4 | 聚合 | `Strategy`（10 预设 + 4 字段独立策略）→ `Aggregate(usages, nodeCounts, s)` | `internal/traffic/traffic.go:102-356` |
| 5 | 透传 | `/subscribe` 生成产物时把聚合结果写成响应头 `Subscription-Userinfo` | `internal/api/subscription.go:659-683` |
| 6 | 呈现 | 订阅页策略块 + 余额刷新块；机场页流量列 | `web/src/pages/SubscriptionsPage.vue`、`AirportsPage.vue` |

刷新入口：`internal/subforge/traffic.go`（`RefreshAirportTraffic` / `RefreshAirportTrafficByID` / `PreviewTraffic` / `UpdateAirportTraffic`），并发 4（`trafficRefreshParallel`），审计保留 500 行（`trafficAuditRetention`）。

### 7.2 策略语义（冻结）

**4 个字段**：`upload` / `download` / `total` / `expire`，各可独立取 7 个值（`applyField`，`traffic.go:236-310`）：

| 值 | 含义 | 0 值处理 |
| --- | --- | --- |
| `sum` | 求和 | **跳过 0**（未上报的机场不计入） |
| `min` | 最小值 | **跳过 0**；全 0 → 0 |
| `max` | 最大值 | 参与比较（0 不会大于正数，效果等同跳过） |
| `avg` | 非零值平均 | **跳过 0**；全 0 → 0 |
| `first` | 第一个非零值（按机场顺序） | 跳过 0 |
| `last` | 最后一个非零值 | 跳过 0 |
| `none` | 不透传该字段（响应头里不出现这个键） | — |

**10 个预设**（`PresetStrategy`，`traffic.go:124-148`，名字大小写不敏感）：

| 预设 | 展开 | 说明 |
| --- | --- | --- |
| `sum`（默认） | upload=sum, download=sum, total=sum, expire=min | 总量求和、到期取最早 |
| `min` | 4 字段全 min | 保守 |
| `max` | 4 字段全 max | 乐观 |
| `avg` | 4 字段全 avg | |
| `first` | 4 字段全 first | 只信第一个机场 |
| `last` | 4 字段全 last | |
| `min_quota` | 只有 Preset，无字段 | 取 **Total 最小**的那个机场的整份快照（Total==0 跳过；全 0 取第一个） |
| `max_quota` | 只有 Preset，无字段 | 优先取 **Total==0**（不限量）的第一个；否则取 Total 最大 |
| `weighted` | 只有 Preset，无字段 | 按 `node_count` 加权平均 upload/download/total；expire 仍取 min；权重全 0 退化成 avg |
| `off` | 4 字段全 none | 不透传（响应头完全不写） |

⚠️ **`min_quota` / `max_quota` / `weighted` 在 `Aggregate` 里是 switch 分支，整份返回，会忽略字段级策略**（`traffic.go:160-212`）。其余 7 个预设与自定义组合才走字段级 `applyField`。

**归一化顺序**（`Normalize()`，`traffic.go:102-121`）：显式字段 > 预设展开值 > 默认 `(sum, sum, sum, min)`。

### 7.3 透传契约（`internal/api/subscription.go:659-683`）

参与聚合的机场必须同时满足：`Enabled == true` **且** `SubscriptionURL` 非空 **且** `TrafficAt > 0`。
`TrafficAt <= 0` 的机场从未采集成功、流量是零值，混进来会把 min/max/avg 拉偏 —— 所以显式排除。

- `Aggregate` 返回 `ok=false`（无可聚合数据 / 全 none）→ 不写头；
- `FormatWithStrategy` 返回空串（全 none）→ 不写头；
- 读策略失败（DB 故障）→ **静默跳过**，订阅本身仍要能生成。

### 7.4 端点契约（`/api/traffic` 4 个 + 2 个机场刷新）

| 端点 | 请求 | 响应 |
| --- | --- | --- |
| `GET /api/traffic/strategy` | — | `{"strategy":{preset,upload,download,total,expire}}` |
| `PUT /api/traffic/strategy` | 策略对象 | 同上，回显**归一化后**的结果 |
| `POST /api/traffic/refresh` | 可选 `?with_rules=1` | `{"results":[{airport_id,name,ok,usage,node_count,err}],"ok":N,"fail":M}`；带 `with_rules` 时多 `imports` 数组 |
| `GET /api/traffic/preview?strategy=<json>` | 可选策略 | `{"usage":{...},"header":"upload=...; total=...","ok":bool}`；不传用已存策略；strategy 非法 JSON → 400 `strategy 参数不是合法 JSON` |
| `POST /api/airports/refresh` | 可选 `?with_rules=1` | 同 `/api/traffic/refresh` |
| `POST /api/airports/{id}/refresh` | — | `{"result":{...}}`；机场不存在 404 |

### 7.5 前端呈现（testid 契约）

| 页面 | 块 | 必须写清 | testid |
| --- | --- | --- | --- |
| SubscriptionsPage | Block 3 流量余额聚合策略 | 10 预设逐条含义 + 4 字段下拉 7 值 + 5 个快捷组合（默认/严格/乐观/独立/隐藏）+ 试算显示响应头原文 + 「sum 在额度不叠加时会虚高」风险提示 | `block-strategy` `preset-select` `upload-field` `download-field` `total-field` `expire-field` `quick-default` `quick-strict` `quick-optimistic` `quick-independent` `quick-hidden` `save-strategy-btn` `preview-btn` `preview-header` `preview-okfalse` `strategy-risk-hint` |
| SubscriptionsPage | Block 4 余额刷新 | `{ok,fail}` 汇总 + 逐机场结果表（名称/成功/节点数/错误）+ 「失败不会把下游余额清零」 | `block-refresh` `refresh-btn` `refresh-error` `refresh-summary` `refresh-table` `refresh-row` `refresh-empty` |
| AirportsPage | 流量列 + 刷新 | 已用/总量/到期/节点数/上次成功时间；错误原文 + 「余额仍是上次成功值」 | `traffic-error` `traffic-at` `traffic-stale` `traffic-err` `traffic-status-ok` `refresh-traffic` `refresh-traffic-all` `with-rules` `refresh-summary` `refresh-error` `imports` `refresh-results` `refresh-result-row` |

工具函数：`web/src/format/index.ts:59` `formatTrafficPair(used, total)` → total<=0 时返回 `'1.0 KB / 未知'`。

### 7.6 口径与边界（必须诚实呈现）

1. `TrafficAt > 0` 才参与聚合 —— 首次刷新前，订阅响应头里**没有** `Subscription-Userinfo`。
2. 四字段全 `none`（预设 `off`）→ 不写头，下游客户端显示不出余额。
3. 刷新失败**不清零**：`UpdateAirportTraffic(u == nil)` 只写 `traffic_err`，保留上次成功值（`internal/subforge/traffic.go:114-133`）。
4. 上游不返回该头 → `Usage` 全 0、`ok=false`，界面显示「未知」而不是 0。
5. `expire` 是 unix 秒；显示走 `formatExpire`。
6. 聚合只在 `/subscribe` 生成时发生，**不落库** —— 策略改了立刻生效，无需重新刷新。
7. `min/max/avg/first/last` 都跳过 0 值，所以「某个机场没上报」不会把结果拉低，但也意味着少报一个机场时 sum 会偏小。

### 7.7 施工归属

| 任务 | 归属 | 状态 |
| --- | --- | --- |
| W2-1（task-140）SubscriptionsPage Block3+Block4 | implementer-cov | 已交付 |
| W1-5（task-139）AirportsPage 流量列 | implementer-format | 已交付 |
| F1（task-148）AirportsPage 流量失败态（traffic-error/at/stale） | implementer-format | 在飞 |
| F2（task-149）SubscriptionsPage `refresh-error` + 失败不清零说明 | implementer-cov | 已交付 |

### 7.8 待补 UI 文案（Lead 核对源码后新发现的两条语义）

§7.2 的两条语义目前**界面上没有提示**，用户会踩：

| # | 语义（源码证据） | 必须补的文案 | testid | 归属 |
| --- | --- | --- | --- | --- |
| 1 | `min_quota` / `max_quota` / `weighted` 在 `Aggregate` 里是 switch 分支整份返回，**会忽略字段级策略**（`internal/traffic/traffic.go:160-212`） | 选这三个预设时，4 个字段下拉置灰或显示「该预设不使用字段级策略」，并在旁边写明原因 | `preset-ignores-fields` | implementer-cov（SubscriptionsPage.vue） |
| 2 | `applyField` 的 `sum/min/avg/first/last` **跳过 0 值**，`max` 不跳（`internal/traffic/traffic.go:236-310`） | 字段策略 hint 补一句「跳过值为 0 的机场：未上报的机场不会拉低结果，但 sum 会少算它的额度」 | `field-zero-note` | implementer-cov（SubscriptionsPage.vue） |

