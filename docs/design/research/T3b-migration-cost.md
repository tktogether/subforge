# T3b · 迁移成本 A/B/C 三档 + v-data-table 难改点（带行号）

- 统计对象：`E:\Projects\subforge\web\src` 下全部 `*.vue`（DSH glob `web/src/**/*.vue` 实测 **23 个**）+ `web/package.json`
- 统计工具：DSH `glob` / `read` / `grep`（未使用 shell 的 grep/find）
- 现状基线：Vuetify **^3.13.7**、Vue **^3.5.44**（`E:\Projects\subforge\web\package.json:22` 与 `:24`）
- 组件注册方式：`import * as components from 'vuetify/components'` + 全量注册（`E:\Projects\subforge\web\src\plugins\vuetify.ts:4`、`:8`）
- 所有数字均为本轮实测；未查到的一律写「未查到」，不估算。

---

## 1. 全仓构造盘点（逐条带绝对路径:行号）

### 1.1 `<v-data-table`：4 处 / 4 个文件

| # | 绝对路径:行号 | 原文（截断） |
|---|---|---|
| 1 | `E:\Projects\subforge\web\src\components\ResourcePage.vue:147` | `<v-data-table :items="items" :headers="headers" density="comfortable" hover data-testid="table">` |
| 2 | `E:\Projects\subforge\web\src\pages\OverviewPage.vue:126` | `<v-data-table v-else :items="history" density="comfortable" hover show-expand :expanded="expanded" @update:expanded=...` |
| 3 | `E:\Projects\subforge\web\src\pages\AirportsPage.vue:243` | `<v-data-table :headers="headers" :items="rows" data-testid="table">` |
| 4 | `E:\Projects\subforge\web\src\pages\RulesPage.vue:211` | `<v-data-table :items="candidates" show-select v-model="selectedIds" item-value="id" density="comfortable">` |

闭合标签 4 处：`ResourcePage.vue:179`、`OverviewPage.vue:149`、`AirportsPage.vue:274`、`RulesPage.vue:219`。

### 1.2 `:headers` / `:items` / 列定义数组

- `:headers` 在 `<v-data-table` 上 **2 处**：`ResourcePage.vue:147`、`AirportsPage.vue:243`。
- `:items` 在 `<v-data-table` 上 **4 处**（即 §1.1 的 4 行本身）；其余 `:items` 命中全部是 `<v-select>` 的选项列表（`ConvertPage.vue:221/227/232`、`SubscriptionsPage.vue:236/306/316/317/318/319`、`ScriptsPage.vue:208/235`、`RuleSourcesPage.vue:275`、`PipelinesPage.vue:238/249/265`）。
- 列定义数组（`{ key, title }` 对象数组，喂给 `:headers`）**5 处**：
  1. `E:\Projects\subforge\web\src\components\ResourcePage.vue:44-49` —— `const headers = computed(() => { ... return props.columns.concat([{ key: 'actions', title: '操作' }]) })`
  2. `E:\Projects\subforge\web\src\pages\AirportsPage.vue:37-47` —— 9 列硬编码数组
  3. `E:\Projects\subforge\web\src\pages\GroupsPage.vue:8-15` —— `const groupColumns = [...]`
  4. `E:\Projects\subforge\web\src\pages\SelfNodesPage.vue:8-13` —— `const selfNodeColumns = [...]`
  5. `E:\Projects\subforge\web\src\pages\RulesPage.vue:167` —— 模板内联数组（`:columns="[...]"`）
- **注意**：`OverviewPage.vue:126` 的表格**没有** `:headers`，列由 `:items="history"` 自动推导 —— 这一处的列定义数组「未查到」，换库后列序/空值/格式化需重新实现。

### 1.3 `#item.` 具名插槽：15 处 / 4 个文件

| 文件（绝对路径） | 行号 | 插槽名 |
|---|---|---|
| `...\components\ResourcePage.vue` | 148 / 153 / 156 / 159 / 162 | `#item.enabled` / `#item.subscription_url` / `#item.link` / `#item.token` / `#item.actions` |
| `...\pages\AirportsPage.vue` | 244 / 247 / 256 / 259 / 263 / 267 | `#item.enabled` / `#item.traffic` / `#item.expire` / `#item.updated` / `#item.status` / `#item.actions` |
| `...\pages\RulesPage.vue` | 212 / 217 / 218 | `#item.rule_config` / `#item.source` / `#item.rule_type` |
| `...\pages\OverviewPage.vue` | 127 | `#item.at` |

（`...` = `E:\Projects\subforge\web\src`。上面每一行都由 DSH `read` 逐行确认。）

### 1.4 `#expanded-row` / 展开行 / `v-model:expanded`

- `#expanded-row` **1 处**：`E:\Projects\subforge\web\src\pages\OverviewPage.vue:128`，实现体为手写的 `<tr><td :colspan="columns.length">...`（`:129-147`）。
- 展开状态与事件：`OverviewPage.vue:15` `const expanded = ref<string[]>([])`；`OverviewPage.vue:126` `show-expand :expanded="expanded" @update:expanded="(v: readonly string[]) => (expanded = [...v])"`。
- `v-model:expanded` 写法 **未查到**（0 命中）；本仓用的是 `:expanded` + `@update:expanded` 拆分写法。

### 1.5 `item-value` / `items-per-page`

- `item-value` **3 处**：`RulesPage.vue:211`（表格选择协议）、`PipelinesPage.vue:240`、`PipelinesPage.vue:267`（后两处是 `<v-select>` 的值字段）。
- `items-per-page` **未查到**（0 命中）；`v-data-table-server`、`:page`、`:sort-by` 同样未查到 —— 本仓 4 个表格**全是客户端全量数据**，无服务端分页/排序。

### 1.6 `<v-form` / `rules=` / `validate()`：全部未查到

- `v-form`（含 `<v-form`）：**0 命中**（全仓）。
- `rules=`：`web/src` 下 **0 命中**；全仓 46 处命中全部落在 Go 后端与文档里（例如 `internal\config\custom.go:11`、`docs\compatibility.md:26`），与表单校验无关。
- `.validate(`：`web/src` 下 **0 命中**；全仓 18 处命中全在 `internal/` 与 `vendor/` 的 Go 代码（例如 `internal\config\override.go:99`）。
- 模板内 `required` 属性：`*.vue` 下 **0 命中**。
- 结论：**本仓不存在 Vuetify 表单校验协议**，表单是 `v-dialog` + `v-text-field`/`v-select`/`v-switch` 的裸表单；所谓「校验」只有 `:disabled` 与前端条件渲染（例：`RulesPage.vue:208` 的 `:disabled="selectedIds.length === 0"`）。因此任务书 C 档中的「表单校验协议」在本仓**命中 0 处**。

### 1.7 `<v-dialog` 的 `v-model`：12 处 / 6 个文件

`ResourcePage.vue:182`、`ResourcePage.vue:230`、`SubscriptionsPage.vue:285`、`ScriptsPage.vue:203`、`ScriptsPage.vue:230`、`RuleSourcesPage.vue:241`、`RuleSourcesPage.vue:256`、`RuleSourcesPage.vue:271`、`PipelinesPage.vue:227`、`PipelinesPage.vue:299`、`AirportsPage.vue:318`、`AirportsPage.vue:335`。

其中 `ResourcePage.vue:182` 与 `:230` 是共享组件，被 `GroupsPage.vue:45`、`SelfNodesPage.vue:45`、`RulesPage.vue:163` 三页复用 —— 即「12 处」中有 2 处服务 3 个业务页。

### 1.8 表格外的 Vuetify 属性面（B 档弹药）

- `variant=`：`*.vue` 下 **109 处**（`v-alert`/`v-btn`/`v-card`/`v-chip`/`v-expansion-panels`/`v-tabs` 等）。
- `density=`：**13 处**，全在 `<v-data-table`/`<v-list>`/`<v-app-bar>` 上（`ResourcePage.vue:147`、`OverviewPage.vue:126`、`RulesPage.vue:211`、`ConvertPage.vue` 等）。
- `persistent-hint`：**22 处**（`AirportsPage.vue:322/323/324`、`ConvertPage.vue` 8 处、`SubscriptionsPage.vue` 9 处、`ScriptsPage.vue:207`）。
- `hide-details`：**2 处**（`ResourcePage.vue:192`、`LogsPage.vue:104`）。
- `<v-expansion-panels>`：**7 处 / 5 个文件**（`HelpPanel.vue:20/29/36`、`GroupsPage.vue:55`、`RuleSourcesPage.vue:205`、`RulesPage.vue:189`、`SelfNodesPage.vue:55`）。
- `<v-tabs>`：**1 处**（`RulesPage.vue:149`）。
- `v-slot:` 旧式写法：**0 命中**（本仓全用 `#` 简写）。

---

## 2. 三档判定标准 + 本仓实际命中数量

### A 档：只改 CSS/令牌，模板不动

**判定标准（机械可查）**：该页的 Vuetify 依赖**只**以「CSS 工具类 + 内联 `style`」形式出现，模板里**没有任何 Vuetify 组件标签**。此时迁移只需替换类名与主题令牌，模板字面不动。

**本仓实际命中：0 页。** 13 个业务页最少也有 3 个 Vuetify 标签（`NotFoundPage.vue:6/7/8`，但它不是业务页），业务页里最少的是 `GroupsPage.vue` 与 `SelfNodesPage.vue`（各 5 个 `<v-` 开标签）。只要出现 `<v-card>` 这类标签，就至少要改标签名 —— 不属于 A 档。

**A 档的真实形态是「共享成本」而非「某些页」**，可查实据：

- 主题对象只有 4 个颜色令牌：`E:\Projects\subforge\web\src\plugins\vuetify.ts:15-20`（`primary`/`secondary`/`background`/`surface`），外加 `:11` `defaultTheme: 'subforge'`、`:14` `dark: false`。
- `web/src` 下**没有任何** `.css`/`.scss` 文件（glob `web/src/**/*.{css,scss}` → `[]`），也没有 `styles/` 目录；23 个 `.vue` 里只有 1 个带 `<style>` 块：`E:\Projects\subforge\web\src\pages\ConvertPage.vue:294-300`（`<style scoped>`，7 行）。
- 工具类命中：`text-medium-emphasis` **44 处 / 13 个文件**；`text-error`、`mb-*`、`pa-*`、`ga-*`、`d-flex` 等合计 132 处（按 §1.8 同法统计）。
- 内联 `style="` **6 处**：`ResourcePage.vue:154`、`ResourcePage.vue:157`、`LogsPage.vue:121`、`LogsPage.vue:122`、`TasksPage.vue:112`，以及 `TasksPage.vue:119`（`max-height: 480px`）。

### B 档：改属性名/事件名/插槽名，结构不变

**判定标准**：该页用到 Vuetify 组件标签，但**不含** §2 的 C 档构造（列定义数组 / `#item.*` 插槽 / 展开行 / 表格选择协议 / 表单校验协议）。改动是「同位置换名」：`v-select :items` → 目标库选项属性、`v-text-field type="number"` → 目标库数字类型、`v-dialog max-width` → 目标库宽度属性、`@update:model-value` → 目标库变更事件、`v-expansion-panels` 的 `variant="accordion" multiple flat` → 目标库手风琴属性。

**本仓实际命中：8 页**（`SettingsPage.vue`、`ScriptsPage.vue`、`LogsPage.vue`、`RuleSourcesPage.vue`、`TasksPage.vue`、`PipelinesPage.vue`、`SubscriptionsPage.vue`、`ConvertPage.vue`）。

具体例子（≥3，全部带行号）：

1. `E:\Projects\subforge\web\src\pages\ConvertPage.vue:221` —— `<v-select v-model="selectedTarget" :items="targets" label="目标格式" ...>`；`ConvertPage.vue:227`、`:232` 同形。
2. `E:\Projects\subforge\web\src\pages\LogsPage.vue:100-106` —— `<v-switch :model-value="autoRefresh" ... @update:model-value="onAutoChange">`，显式的「值 + 更新事件」拆写。
3. `E:\Projects\subforge\web\src\pages\ScriptsPage.vue:207` —— `<v-text-field v-model="formName" label="名称" class="mb-2" :hint="fieldHelp('script.name')?.hint" persistent-hint data-testid="field-hint" />`（`hint`/`persistent-hint` 是 Vuetify 专属 prop 名）。
4. `E:\Projects\subforge\web\src\pages\PipelinesPage.vue:240` 与 `:267` —— `<v-select ... item-value="value">`，值字段名需换。
5. `E:\Projects\subforge\web\src\pages\RuleSourcesPage.vue:205` —— `<v-expansion-panels class="mt-4">` + 子标签 `v-expansion-panel-title`/`v-expansion-panel-text` 具名插槽结构。

### C 档：必须重写模板结构

**判定标准**：该页渲染路径上出现下列任一 —— (a) `<v-data-table>` + 列定义数组；(b) `<v-data-table>` + `#item.*` 具名插槽；(c) `<v-data-table>` + 展开行；(d) `<v-data-table>` + 选择协议（`show-select`/`item-value`/`v-model` 收选中项）；(e) `<v-form>` + `rules=` + `validate()` 协议。

**本仓实际命中：5 页**（`AirportsPage.vue`、`OverviewPage.vue`、`RulesPage.vue`、`GroupsPage.vue`、`SelfNodesPage.vue`）。

C 档例子（≥5，全部带行号）：

1. **列定义数组**：`E:\Projects\subforge\web\src\pages\AirportsPage.vue:37-47`（9 列 `{ key, title }` 数组），消费点 `AirportsPage.vue:243`。
2. **列定义数组（共享组件内）**：`E:\Projects\subforge\web\src\components\ResourcePage.vue:44-49`，由 `:headers` 传入 `ResourcePage.vue:147`；三个业务页各自再传一份列数组：`GroupsPage.vue:8-15`（`:49` 传入）、`SelfNodesPage.vue:8-13`（`:49` 传入）、`RulesPage.vue:167`。
3. **具名插槽**：`E:\Projects\subforge\web\src\components\ResourcePage.vue:148`（`#item.enabled`）、`:162`（`#item.actions`）、`E:\Projects\subforge\web\src\pages\AirportsPage.vue:244`、`:247`、`:256`、`:259`、`:263`、`:267`、`E:\Projects\subforge\web\src\pages\RulesPage.vue:212`、`:217`、`:218`、`E:\Projects\subforge\web\src\pages\OverviewPage.vue:127`。
4. **展开行**：`E:\Projects\subforge\web\src\pages\OverviewPage.vue:126`（`show-expand` + `:expanded` + `@update:expanded`）、`:128-148`（`#expanded-row` 手写 `<tr><td>` 展开体）、`:15`（`const expanded = ref<string[]>([])`）。
5. **选择协议**：`E:\Projects\subforge\web\src\pages\RulesPage.vue:211`（`show-select` + `v-model="selectedIds"` + `item-value="id"`）。
6. **表单校验协议**：**未查到**（见 §1.6）—— 本仓 C 档全部由 `v-data-table` 触发。

**为什么 `v-data-table` 跨库不可平移**（4 条机制）：

- 列模型是 **JS 对象数组**（`{ key, title }`）驱动 `:headers`，不是模板里的列标签。各候选库的列 API 不同名也不同形：Naive UI `n-data-table` 用 `columns: { title, key }`；PrimeVue `DataTable` 用 `columns` + `field`/`header`；Element Plus `el-table` 干脆用 `<el-table-column prop label>` 标签而非数组。数组形态本身就要重写。
- 单元格渲染靠**按列 key 命名的作用域插槽**（`#item.<key>`），15 处（§1.3）。目标库的插槽名与参数形态完全不同（PrimeVue `#body-<field>`；Naive `#<key>-cell`；Element Plus `#default="{ row }"`），且作用域字段从 `{ value, item }` 变成 `{ row, column }` 之类 —— 每个插槽体都要重写，不只是改插槽名。
- 展开行是 **`#expanded-row` + 手写 `<tr><td>` 结构**（`OverviewPage.vue:128-148`）。目标库的展开是另一套 API 与另一套 DOM（PrimeVue `rowExpansion`/`#expansion`；Naive `expanded-row`），手写表格行无法平移。
- 选择/分页是 **Vuetify 专属 prop 组合**（`show-select` + `v-model` 收数组 + `item-value`）。目标库分别是 PrimeVue `v-model:selection` + `dataKey`、Naive `row-key` + `checked-row-keys`、Element Plus `row-key` + `selection-change` —— 语义相同但 prop 名、值类型、事件名全变，且 `item-value` 在 `<v-select>` 上也要一并换（`PipelinesPage.vue:240`、`:267`）。

---

## 3. 一句话结论：13 个业务页 A/B/C 分布

> **A 档 0 页 / B 档 8 页 / C 档 5 页。**

口径（先说清，再给数）：

- 「页」= `E:\Projects\subforge\web\src\router\index.ts:46-58` 的 13 条业务路由；`NotFoundPage.vue`（`router/index.ts:59` 的 catch-all）**不计入**。
- A 档口径：页模板零 Vuetify 标签（§2 A）。→ **0 页**：13 页每页至少 5 个 `<v-` 开标签，无一页是纯 CSS/令牌页。A 档工作量只以「共享的类名/内联样式/主题令牌」形式存在（§2 A 末尾）。
- C 档口径：页的渲染路径上命中 §2 C 的 4 类 `v-data-table` 构造之一；**经由共享组件 `ResourcePage.vue` 也算命中**（因为它真的渲染表格）。→ **5 页**。
- B 档口径：其余全部。→ **8 页**（13 − 5）。

| 档 | 页数 | 页（绝对路径） |
|---|---|---|
| A | **0** | —（A 档只有共享成本，见 §2 A） |
| B | **8** | `E:\Projects\subforge\web\src\pages\SettingsPage.vue`、`ScriptsPage.vue`、`LogsPage.vue`、`RuleSourcesPage.vue`、`TasksPage.vue`、`PipelinesPage.vue`、`SubscriptionsPage.vue`、`ConvertPage.vue` |
| C | **5** | `E:\Projects\subforge\web\src\pages\AirportsPage.vue`、`OverviewPage.vue`、`RulesPage.vue`、`GroupsPage.vue`、`SelfNodesPage.vue` |

C 档 5 页里有一个放大器：`GroupsPage.vue`、`SelfNodesPage.vue`、`RulesPage.vue` 三页共享 `E:\Projects\subforge\web\src\components\ResourcePage.vue`（`GroupsPage.vue:3`、`SelfNodesPage.vue:3`、`RulesPage.vue:9` 导入；`GroupsPage.vue:45`、`SelfNodesPage.vue:45`、`RulesPage.vue:163` 使用），所以 **C 档的实际改动面是 3 个页面文件 + 1 个共享组件文件 + 2 个自带表格的页面文件**（`AirportsPage.vue`、`OverviewPage.vue`），而不是 5 个孤立文件。`RulesPage.vue` 一页上同时有自带表格（`:211`）和共享表格（`:163`）。

---

## 4. 未决问题

1. **任务书假设的「表单校验协议」在本仓不存在**：`v-form`/`rules=`/`validate()`/`required` 全部 0 命中（§1.6）。所以 C 档的判定完全落在 `v-data-table` 上；如果后续有人给表单加上 Vuetify 校验，C 档会再涨。
2. **目标库未定**：本文的 A/B/C 是「换库」的**上界分类**，不绑定具体候选库。同一条 B 档在不同候选库下可能退化成 C（例如候选库没有 `v-expansion-panels` 等价物时，`RuleSourcesPage.vue:205` 那类折叠面板就要重写）。
3. **「属性名差异」未逐条枚举**：本文只给出 B 档的 5 个代表例子（§2 B），没有为 8 个 B 档页逐行列出 109 处 `variant=` / 22 处 `persistent-hint` 的换名对照表；那需要先在 `docs/design/research/T1-vue-ui-frameworks.md` 选定目标库。
4. **`items-per-page`/`v-data-table-server` 未查到**，所以「服务端分页表格要改」这一类风险在本仓为 0，但若将来加服务端分页会新增 C 档面。
5. **未做像素级回归口径**：本文只回答「改哪些行」，不回答「改完像不像」。

---

## 5. 自测原始输出

文件：`E:\Projects\subforge\docs\design\research\T3b-migration-cost.md`

```powershell
$f = 'E:\Projects\subforge\docs\design\research\T3b-migration-cost.md'
(Get-Item -LiteralPath $f).Length
(Get-Content -LiteralPath $f).Count
```

原文输出（Windows PowerShell 5.1.22621.6133）：
```
17585
107
```

说明：`17585` = `(Get-Item).Length`（字节）；`107` = `(Get-Content).Count` 在 PS 5.1 默认 ANSI 解码下的产物（与 T3a 报告的 `187` 是同一个解码陷阱），真实行数用 `[System.IO.File]::ReadAllLines` 实测为 **184**。字节数 ≥ 3500 满足；C 档例子 **6 个**且全部带行号（≥5）。
