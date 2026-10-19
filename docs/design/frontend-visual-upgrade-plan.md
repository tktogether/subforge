# 前端视觉升级计划（SubForge Web UI）

> 状态：**已完工（W1–W6 全部落地，最终门禁 35 files / 618 tests 全绿）**。用户已拍板 D1（Instrument）方向、不换框架；W1 令牌层 → W2 样式层 → W3/W4 页面与表格 → W5 集成接线 → W6 修复波（深色首帧 / 表格 / 帮助面板）逐波验收；Lead 真机截图对照见附录 D。所有交付**尚未 git commit**（等用户拍板）。（T1 / T2 / T3a / T3b / T3c / T6 / T7 / T9 / T11 / T12 / T13 / T14 全部验收通过；D1 / D2 / D3 三份样板在 1600 与 1280 双视口下 render-check + cell-fit-check 双门禁均 exit 0；cell-fit-check.ps1 已升级为三判据 + `-SelfTest` 自证；Lead 目视三份样板通过）
>
> 卡片改名记录：原 T3 卡（单卡覆盖迁移面+门禁）由 free subagent 执行时**上下文耗尽、零产出**，已删除并拆成 T3a/T3b/T3c 三张窄卡重派；T4/T5/T6/T7 因 teammate 无法指定模型（见附录 A.1），改由 `subagent(provider=llmio, model=free)` 派发。
> 创建：2026-10-19 · Owner：lead
> 触发：用户反馈「现在前端框架太丑了，去找找好看的」

## 0. 背景与问题陈述

SubForge Web 端当前是 **Vue 3.5 + Vuetify 3.13 + vue-router 4**，共 23 个 `.vue`（约 2,770 行）、13 个业务页 + 404 页、35 个 spec 文件 / 594 条测试。

丑的**根因不是 Vuetify 本身**，而是主题层几乎为空：

- `web/src/plugins/vuetify.ts:12-22` 只定义了 4 个颜色（primary `#1976d2` 是 Vuetify 默认蓝、secondary `#00897b`、background `#f4f6f8`、surface `#ffffff`），**没有** dark 主题、**没有** 排版/圆角/阴影/密度令牌、**没有** CSS 变量层。
- 全站 93 个 `<v-btn>`、41 个 `<v-card>`、4 个 `<v-data-table>` 全部裸用默认外观，没有任何统一的视觉语言。
- 仓库根已有 `.impeccable.md` 设计上下文（"calm · precise · dense"、OKLCH、禁止 Bootstrap admin 观感 / 毛玻璃 / 渐变文字），但**从未落到代码里**。

结论：**先做调研与选型**（用户原话是「去找找好看的」），产出可对比的候选方向 + 可直接在浏览器打开的静态样板，再由用户拍板是否迁移。不先斩后奏做 13 页迁移。

## 1. 验收信号（Lead 用原始命令独立复跑）

工作目录 `E:\Projects\subforge\web`，node/pnpm 用 DSH 自带：

```
NODE = C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\node\bin\node.exe
PNPM = C:\Users\橙先生\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\pnpm\bin\pnpm.mjs
```

| # | 命令 | 期望（基线已实测） |
|---|---|---|
| G1 | `& $NODE $PNPM typecheck` | exit 0，无输出 |
| G2 | `& $NODE $PNPM test` | `Test Files 35 passed (35)` / `Tests 594 passed (594)`，exit 0 |
| G3 | `& $NODE $PNPM build` | exit 0，产物落到 `internal/api/web/dist` |
| G4 | `& $NODE $PNPM coverage` | 覆盖率门禁通过 |

**基线（2026-10-19 12:22 实测，Lead 跑）**：G2 → `Test Files 35 passed (35)`，`Tests 594 passed (594)`，`Duration 15.40s`，exit 0。

> 纪律：调研 Wave 只允许**新建** `docs/design/research/**` 与 `docs/design/mockups/**`，**不许动** `web/src/**`。这样 Wave 1 结束时 G1–G4 必然仍是绿的，无需回归。

## 2. 工作拆解表

### Wave 1 —— 调研与样板（并行，全部免费模型）

| 卡号 | 内容 | Owner | 写范围（互斥） | 依赖 |
|---|---|---|---|---|
| T1 | Vue 3 UI 框架美学横评 | free-scout-frameworks | `docs/design/research/T1-vue-ui-frameworks.md` | — |
| T2 | "好看的后台"视觉配方库 | free-scout-recipes | `docs/design/research/T2-visual-recipes.md` | — |
| T3a | 组件使用面清单 + 只改主题覆盖率 | free subagent | `docs/design/research/T3a-component-inventory.md` | — |
| T3b | 迁移成本 A/B/C 三档 + `v-data-table` 难改点 | free subagent | `docs/design/research/T3b-migration-cost.md` | — |
| T3c | 测试与门禁真实约束（逐字引用） | free subagent | `docs/design/research/T3c-gates.md` | — |
| T4 | 样板 D1「Instrument」 | implementer-mock-d1 | `docs/design/mockups/d1-instrument.html` | — |
| T5 | 样板 D2「Editorial」 | implementer-mock-d2 | `docs/design/mockups/d2-editorial.html` | — |
| T6 | 样板 D3「Console」 | free subagent | `docs/design/mockups/d3-console.html` | — |
| T7 | 样板表格横向溢出修复 + `render-check.ps1` | free subagent | `docs/design/mockups/render-check.ps1` | T6 |

### Wave 2 —— 决策（Lead）

- Lead 汇总 T1–T3 → 写本文件第 4 节「选型结论」
- 把 T4–T6 三个样板交给用户**目视**拍板
- 产出 `docs/design/frontend-visual-upgrade-decision.md`（选定方向 + 迁移工作量 + 回滚）

### Wave 3 —— 落地（用户拍板后另开）

预留拆法（写范围互斥）：
- `implementer-tokens`：`web/src/design/**` + `web/src/plugins/vuetify.ts`
- `implementer-styles`：`web/src/styles/**`
- `implementer-shell`：`web/src/layout/**` + `web/src/components/PageHeader.vue` + `web/src/components/AsyncSection.vue`
- **串行点**：`web/src/main.ts` 由 Lead 改（只加 import 行），避免三人抢同一文件。

## 3. 已知契约（源码原文，不许猜）

### 3.1 当前主题（唯一视觉入口）
`web/src/plugins/vuetify.ts:1-24`
```ts
import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'

export const vuetify = createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'subforge',
    themes: {
      subforge: {
        dark: false,
        colors: {
          primary: '#1976d2',
          secondary: '#00897b',
          background: '#f4f6f8',
          surface: '#ffffff',
        },
      },
    },
  },
})
```

### 3.2 依赖版本（`web/package.json`）
```
vue ^3.5.44 / vue-router ^4.5.0 / vuetify ^3.13.7 / @mdi/font ^7.4.47
devDeps: vite ^8.3.4 / vitest ^5.0.5 / @vue/test-utils ^2.5.1 / jsdom ^30.1.3 / vue-tsc ^3.3.13 / typescript ~5.9.3
packageManager: pnpm@11.28.5  engines.node: >=20.19.0
```

### 3.3 脚本（`web/package.json` scripts）
```
dev=vite  build=vue-tsc --noEmit && vite build && node scripts/ensure-dist-placeholder.mjs
build:only=vite build && node scripts/ensure-dist-placeholder.mjs  preview=vite preview
typecheck=vue-tsc --noEmit  test=vitest run  test:watch=vitest  coverage=vitest run --coverage
```

### 3.4 构建产物去向（`web/vite.config.ts`）
```
outDir = ../internal/api/web/dist   （Go embed 目录，不要改成不要 emptyOutDir=false）
alias '@' -> ./src
dev proxy: /api /sub /subscribe /version /healthz /getruleset /inspect -> http://127.0.0.1:8080
```

### 3.5 导航结构（`web/src/router/index.ts:17-31`，13 项 5 组）
```
转换: 概览(/) 转换(/convert) 管线(/pipelines) 脚本(/scripts)
来源: 机场(/airports) 自建节点(/self-nodes)
规则: 规则(/rules) 规则源(/rule-sources) 分组(/groups)
输出: 订阅(/subscriptions)
运维: 任务(/tasks) 设置(/settings) 日志(/logs)
```

### 3.6 现有 Vuetify 组件使用频次（Lead 实测，决定"改主题"能覆盖多少）
```
v-btn 93 | v-card 41 | v-card-text 24 | v-card-title 23 | v-text-field 17 | v-select 15
v-spacer 14 | v-card-actions 13 | v-dialog 12 | v-alert 9 | v-switch 8 | v-textarea 8
v-expansion-panel* 7/7/7/5 | v-chip 7 | v-list-item 6 | v-table 4 | v-list 4 | v-tab 4
v-data-table 4 | v-row 2 | v-divider 1 | v-navigation-drawer 1 | v-tabs 1 | v-snackbar 1
v-app 1 | v-app-bar 1 | v-main 1 | v-container 1 | v-skeleton-loader 1 | v-col 4
```

### 3.7 页面清单与行数（Lead 实测）
```
pages/: SubscriptionsPage 342, AirportsPage 314, PipelinesPage 283, ConvertPage 274,
        RuleSourcesPage 253, ScriptsPage 240, RulesPage 228, OverviewPage 146,
        LogsPage 111, SettingsPage 108, TasksPage 107, SelfNodesPage 63, GroupsPage 61,
        NotFoundPage 12
components/: ResourcePage 219, AsyncSection 54, HelpPanel 44, PageHeader 18, UnderConstruction 14
layout/: AppNav 38, AppShell 33, AppTopBar 15
```

## 4. 选型结论（Wave 2，Lead 起草；**视觉方向仍待用户拍板**）

### 4.1 框架层结论：**不换框架**，留在 Vuetify 3.13 上做「令牌 + 全局样式 + 密度重写」

证据链（每条都能复跑）：

1. **丑的根因已定位，且不在 Vuetify**：web/src/plugins/vuetify.ts:15-20 的主题只有 4 个颜色（primary 是 Vuetify 默认蓝 #1976d2），没有 dark 主题、没有排版/圆角/阴影/密度令牌、没有 CSS 变量层；23 个 .vue 里只有 1 个有 style 块（web/src/pages/ConvertPage.vue:294-300）。
2. **换库体积全亏**（T1 实测 npm 包体）：Vuetify barrel 51KB min / 19.6KB gzip，对手 Naive UI 2.20MB、PrimeVue 2.08MB、Element Plus 1.07MB、Arco 882KB。
3. **候选库自身有硬伤**（T1）：Ant Design Vue 4 双停在 2024-11（npm 4.2.6）→ 不可选；Element Plus 令牌是 hex 非 OKLCH（--el-color-primary:#409eff），与仓库根 .impeccable.md 的 OKLCH 要求冲突；原生 OKLCH 的只有 Tailwind v4 与 DaisyUI 5。
4. **换库的模板改动面很大**（T3b）：A 档 0 页 / B 档 8 页 / C 档 5 页。C 档 5 页全是 v-data-table 相关（列定义数组 5 处、#item.* 插槽 15 处、#expanded-row 手写 tr 1 处、选择协议 1 处），且这四类构造**跨库不可平移**；B 档 8 页还要逐处换属性名（实测 variant= 107 处、persistent-hint 21 处、density= 13 处）。
5. **门禁会挡住重写而不是挡住换肤**（T3c）：覆盖率阈值 100/100/80/100（web/vitest.config.ts:26-31），SFC 的 style 块不计入执行行 → 纯视觉重写不阻塞覆盖率；但**6 处断言直接耦合 CSS 类名/DOM 结构**（web/src/pages/OverviewPage.spec.ts:175 **与 :194**、web/src/pages/SettingsPage.spec.ts:117,127、web/src/pages/RuleSourcesPage.spec.ts:94、web/src/layout/AppShell.spec.ts:66、web/src/pages/GroupsPage.spec.ts:141），换库会让这 6 处连同 Vuetify BEM 选择器一起挂。
6. **纯主题升级的收益已经够大**（T3a + T1）：只改主题令牌 + 新增全局 CSS 层、模板不动，确定性覆盖 18/36 = 50.0% 去重标签、221/350 = 63.1% 使用量；T1 对「只换主题」的评分是 2.5 → 3.5，叠加表格/密度/卡片重写后 4.5。
7. **T1 的挑战项已被 Lead 证伪过半**：Vuetify 表格行高并非编译期硬编码 —— web/node_modules/vuetify/lib/components/VTable/VTable.sass:109-113 用的是 CSS 变量 var(--v-table-row-height) / var(--v-table-header-height)，运行时可用 CSS 覆写把 52px 压到 D3 那种 ~33px。**Wave 3 第一件事就是实测这条**，实测通过则「换框架才能变密」的理由彻底不成立。

### 4.2 视觉方向：三选一（D1 / D2 / D3），**由用户目视拍板**

三份样板同内容契约、同 19 个 CSS 变量名，只有视觉语言不同；Lead 已用无头 Chrome 在 1600×1000 与 1280×1000 实测无横向溢出，并逐张目视核对。

| 方案 | 气质 | 最关键特征 | 适合谁 |
|---|---|---|---|
| D1 Instrument | 冷静精确、仪表盘 | 细边框分层、圆角 ≤6px、tabular-nums、信息优先 | 想「专业、克制、像 Grafana」 |
| D2 Editorial | 舒展、编辑感 | 圆角 8–12px、柔和阴影、留白充裕、三档按钮 | 想「好看、松弛、不像后台」 |
| D3 Console | 控制台 / 终端 | 默认深色、行高 ≤34px、状态色点 + 文本、等宽 URL、数字右对齐 | 想「信息密度最高、像 wrangler/日志台」 |

选定后 Wave 3 只落地这一套，另外两套留档在 docs/design/mockups/。

### 4.3 落地顺序（Wave 3 预案，写范围互斥）

1. 令牌层：web/src/design/** + web/src/plugins/vuetify.ts（暗色主题 + 全量语义令牌）
2. 全局样式：web/src/styles/**（排版阶梯、表格/卡片密度、focus-visible、动效与 prefers-reduced-motion）
3. 外壳：web/src/layout/** + web/src/components/PageHeader.vue + AsyncSection.vue
4. 表格与卡片重写：先 web/src/components/ResourcePage.vue（一次覆盖 GroupsPage / SelfNodesPage / RulesPage 三页），再 AirportsPage、OverviewPage
5. **串行点**：web/src/main.ts 由 Lead 改（只加 import 行）
6. 每步都必须复跑 G1–G4，且先修 4.1-5 里那 6 处 CSS 耦合断言（改断言属于「改测试」，必须 Lead 批准后才动，且不许放宽覆盖）

### 4.4 回滚

Wave 3 全部改动集中在 web/src 的新增目录 + 3 个既有文件（vuetify.ts / main.ts / layout），回滚 = 回退 commit + 重新 build；调研与样板（docs/design/**）不回滚，留档。

## 5. 验收与部署

- 本轮（Wave 1）**不部署**：只新增文档与静态 HTML，`git status` 里不应出现 `web/src` 改动。
- 收口清单：`git status --porcelain --untracked-files=all` 无乱码/临时文件；G1–G4 复跑全绿。
- 部署（Wave 3 后）：`pnpm build` → 产物进 `internal/api/web/dist` → 同参数重启 `subforge serve` → 逐页验收 → 回滚 = `git revert` + 重新 build。

## 6. 红线（每个队友都必须遵守）

1. 只写自己卡上写范围内的文件；需要别的文件 → **停下来报备**，不要自己动。
2. 不许改 `web/src/**`、不许改 `web/package.json`、不许改任何 `*.spec.ts`。
3. 不许删断言、不许加 ignore、不许改阈值来"让门禁过"。
4. 调研结论必须有**出处**（URL）或**源码行号**，不许凭印象写。
5. 汇报必须贴**原始输出/原文**，不要只写"完成"。


---

## 附录 A：执行记录与模型路由裁定（Lead 记录，2026-10-19）

### A.1 队员模型路由的真相（含证据）

本机装了 `dsh-subagent-model-config`（C:\Users\橙先生\.dsh\profiles\desktop\node_modules\dsh-subagent-model-config\)，它按**队员名 glob** 给 teammate 指定模型。其 `config.json` 规则表只有三条：`*implementer*`、`plan*`、`*cheap*`（前两条路径都是 `llmio/free`）。

**两个队员没走免费模型，有两层原因：**

1. **命名没命中**（Lead 失误）：我起的名字是 `free-scout-frameworks` / `free-scout-recipes`。`free` **不是**规则表里的关键字，三条 glob 一条都不命中 → 不注入 agentOptions → 队员静默继承默认模型 deepseek-flash。（用户后续澄清：约定关键字是 **cheap**，不是 free。）
2. **更根本：插件在本版本上根本注入不了。** 证据来自插件自己的取证文件 `node_modules\dsh-subagent-model-config\.probe.jsonl`（2026-10-19 03:32–03:39 多次加载）：
   ```json
   {"phase":"patch-spawnTeammate","anyInstalled":true,"instance":{"installed":true,"viaOwn":true},"proto":{"installed":true,"viaOwn":false}}
   {"phase":"patch-startContinuable","anyInstalled":false,"instance":{"installed":false,"error":"method not found"},"proto":{"installed":false,"error":"method not found"}}
   {"phase":"post-patch-identity","instanceCarriesStart":false,"protoCarriesStart":false,"instanceCarriesSpawn":true,"protoCarriesSpawn":true}
   ```
   `spawnTeammate` 包装成功（只用来采集队员名），但真正的**注入点 `SubagentRuntime.startContinuable` 在本版本不存在**（instance 与 prototype 都 method not found）。名字命中与否都无所谓——**这条注入链是断的**。
   插件声明的实测版本是 DSH 0.1.7-alpha.2 / 0.2.0，本机 DSH 已升级，接口漂移。

**顺带一个坑**：`list_agents` 的 `model` 字段对**所有** teammate 都显示 `deepseek-flash`（provider 显示 `spawn`）。它反映的是成员描述符里的默认值，**不是**插件解析出的真实路由，所以不能用它验证路由是否生效。

### A.2 裁定

用户裁定：**低智力批处理活改用 `subagent` + `llmio/free`**（该路径实测可用：free 探针 subagent 回了 `PROBE-OK`；且 `subagent` 工具本身接受 provider/model/reasoning_effort）。

### A.3 Wave 1 实际执行人

| 卡 | 任务 | 执行人 | 模型 |
|---|---|---|---|
| task-1 | T1 框架美学横评 | teammate `free-scout-frameworks` | deepseek-flash（命名未命中且注入链已断） |
| task-2 | T2 视觉配方库 | teammate `free-scout-recipes` | deepseek-flash（同上） |
| task-3 | T3 迁移面与门禁 | subagent `1bbde830-785d-4147-b19e-4ef4ec51f57b` | **llmio/free** |
| task-4 | T4 样板 D1 Instrument | subagent `4dee93a9-d2c6-4086-8216-60eb7f9e1232` | **llmio/free** |
| task-5 | T5 样板 D2 Editorial | subagent `8957cf5b-5d6b-4a7d-a116-542d16cfc305` | **llmio/free** |
| task-6 | T6 样板 D3 Console | subagent `807862d0-0807-4c12-908d-6115093cc4be` | **llmio/free** |

### A.4 样板任务书已追加 frontend-design-pro 硬约束

`_design-pro-addendum.md` 已追加进 T4/T5/T6 三份任务书（字体白名单、OKLCH 带色调中性色、深色底 #0f0f0f 禁 #000000、4/8px 间距、`cubic-bezier(0.16, 1, 0.3, 1)` + 100–200ms 微交互 + prefers-reduced-motion、focus-visible 可见、skeleton 优于 spinner、按钮动词开头、空状态教学、反模式红线）。

## 附录 B：执行台账与验收协议（Lead 记录，2026-10-19）

### B.1 台账

| 卡 | 任务 | 执行人 | 状态 |
|---|---|---|---|
| task-1 | T1 框架美学横评 | teammate `free-scout-frameworks` | 进行中 |
| task-2 | T2 视觉配方库 | teammate `free-scout-recipes` | **已完成（rev 3）** |
| task-3 | T3 迁移面与门禁盘点 | subagent `1bbde830…f57b` | 进行中 |
| task-4 | T4 样板 D1 Instrument | subagent `4dee93a9…1232` | 进行中 |
| task-5 | T5 样板 D2 Editorial | subagent `8957cf5b…c305` | 进行中 |
| task-6 | T6 样板 D3 Console | subagent `807862d0…c4be` | 产物已出，待 T7 修完 |
| task-7 | T7 修表格溢出 + render-check.ps1 | subagent `5d98baef-dd1a-45ec-9723-5c85619d386f` | 进行中 |

### B.2 Lead 独立复核：T2（**通过，一处数字作废**）

Lead 用原始命令复跑，结果与交付一致的项：文件 72430 字节（与自报完全一致）；hex 数量 336（≥128）；去重 URL 57 条（≥15）；禁用词 0；19 个 `--sf-*` 变量名 19/19 全部出现在文件中；`web/src/styles/` 目录确实**不存在**（无 CSS 变量层这一论断成立）；`web/node_modules/vuetify/dist/vuetify.css:3644` 原文确为 `font-family: "Roboto", sans-serif;`（Roboto 无中文字形这一论断成立）。

**行数字段：Lead 一度误判，此处更正。** 交付方自报「894 行」，Lead 用 PowerShell 5.1 `(Get-Content).Count` 量得 829，据此写下「系误报，不得引用」。**该结论是错的，错在 Lead 的量法。** 改用 Node 以 UTF-8 读同一文件：`LF=893`（894 行）、`bytes=72430`，与交付方完全一致；T1 同理（Node `LF=312` = 交付方 312，而 PS 量得 258、ReadAllLines 量得 312）。

根因：**PowerShell 5.1 的 `Get-Content` 默认按 ANSI 代码页解码 UTF-8 无 BOM 文件，会吞掉换行计数**（T1 少 54、T2 少 64）；`[IO.File]::ReadAllLines` 与 Node 的 utf8 读取才是对的。

**教训（Lead 自记，比上面那条更重要）**：行数、字符数、任何与中文文本相关的计数，一律用 `node` 以 utf8 读，或显式 `-Encoding UTF8`；本机上 `Get-Content`/`Select-String` 的默认编码不可信。字节数不受影响（`(Get-Item).Length` 是对的）。**验收方自己的量法也需要被怀疑——这次差点把一位交付正确的队友误判成编数字。**

### B.3 Lead 裁定：`--sf-space` 语义

统一为 **`--sf-space: 4px`（间距基数）**。T2 提出的 `--sf-space-1..16` 刻度线是**可选扩展**，样板里无需实现；Wave 3 统一令牌时若引入刻度，必须保持 `--sf-space` 仍等于 4px，避免下游语义漂移。

### B.4 样板渲染核对协议（Wave 1 新增验收项，所有样板适用）

**触发原因**：D3 交付物 DOM 里 9 列表头齐全、文本自测全绿，但**视觉上少了 2 列**。Lead 用无头 Chrome 在页面内实测 `scrollWidth/clientWidth` 得到硬数字：

| 视口 1600×1000 | scrollWidth | clientWidth | 溢出 |
|---|---|---|---|
| D3 机场表 | 1036 | 980 | **56px** |
| D3 服务端点表 | 980 | 980 | 0 |
| D3 底部令牌表 | 1612 | 965 | **647px** |

根因：`.table-wrap { overflow:auto }` + 单元格 `white-space:nowrap` + 内容天然宽度 > 容器宽；截图时又用了 `--hide-scrollbars`，被裁的列连滚动条提示都没有。

**协议**：
1. 每份样板必须在 **1600×1000 与 1280×1000 两个视口**下，页内每一个 `.table-wrap` 的 `scrollWidth - clientWidth == 0`。
2. 量测必须走 `chrome --headless=new --window-size=<W>,1000 --dump-dom` + 页面内内联脚本读取 `scrollWidth`，**漏掉 `--window-size` 会得到 over=0 的假结果**（Lead 已踩）。
3. 工具固化为 `docs/design/mockups/render-check.ps1`（task-7 交付），Lead 用它做终验。
4. **禁止**用 `overflow:hidden` 裁掉内容、`display:none` 藏列、删内容、把字号压到 12px 以下来"通过"本条——那只是把缺陷藏起来。

### B.5 红线补充（第 7 条）

7. 交付物里**不允许存在需要滚动才能看到的关键列**；表格信息必须在一屏内可见，或用截断 + `title` 兜底全量值。

### B.6 Lead 独立复核：T1（**通过，一处引用需补全，一处结论待挑战**）

Lead 亲自逐条复跑，**全部命中**：
- 文件 39682 字节、312 行（Node utf8 口径）、`TODO|待补充` 命中 0。
- `web/src/plugins/vuetify.ts:10-23` 确为 `theme` 块，颜色**只有 4 个**（primary `#1976d2` / secondary `#00897b` / background `#f4f6f8` / surface `#ffffff`），无 dark、无 variables。
- `web/index.html` 确为 12 行；`Roboto` 在 `web/src` 与 `web/index.html` 命中 **0**。
- `web/node_modules/vuetify/lib/styles/settings/_variables.scss:21` = `$body-font-family: 'Roboto', sans-serif !default;`；`:300-307` button 条目确含 `'text-transform': uppercase` 与 `'letter-spacing': .0892857143em`。
- **表格硬编码尺寸全部为真**（Lead 找到确切文件）：`web/node_modules/vuetify/lib/components/VTable/_variables.scss:9` = `$table-density: ('default': 0, 'comfortable': -2, 'compact': -4)`；`:10` = `$table-header-height: 56px`；`:14` = `$table-row-height: 52px`；`:17` = `$table-border: thin solid $table-border-color`；`VTable.sass:109-113` = `height: var(--v-table-row-height)` / `var(--v-table-header-height)`。

**引用瑕疵（非事实错误）**：T1 报告写「`_variables.scss:14`」，实际是 `lib/components/VTable/_variables.scss`，与 `lib/styles/settings/_variables.scss` **同名不同文件**。行号全对，路径需补全；Wave 2 引用时不得省略目录。

**Lead 挑战项（列 Wave 2 必验）**：T1 结论 3 称「Grafana 那种 ~32px 行高靠 Vuetify 主题/密度达不到」。此说法**过强**：`VTable.sass:110` 用的是 CSS 变量 `var(--v-table-row-height)`，在 `styles/settings/_variables.scss` 之外完全可以在运行时用 CSS 覆写该变量把行高压到 32px。Wave 2 必须实测「只覆写 `--v-table-row-height` / `--v-table-header-height` + 令牌层」能否做到样板 D3 的密度，再决定是否需要换框架。

### B.7 T6/T7 交付验收：D3 样板（**Lead 亲验通过**）

**数字验收**（`render-check.ps1`，task-7 交付）：1600×1000 与 1280×1000 两个视口下 3 个 `.table-wrap` 全部 `scrollWidth == clientWidth`（over=0）；截图 247238 / 238002 字节；源码 `https?://` 字面量计数 0；临时文件 before=0 after=0；exit 0。

**Lead 目视验收**（free 档模型不支持图像输入，目视是不可外包的 Lead 职责）：
- `%TEMP%\d3-1600.png`（1600×2400）：机场表 **9 列全部可见**，含最右的「状态」「操作」；URL 完整；行高约 33px；底部 19 个令牌行的 OKLCH 列完整。
- `%TEMP%\d3-1280.png`（1280×2400）：**9 列仍全部可见**；URL 列按设计截断（`https://sub.example.c…` / `https://backup.exampl…`，全量值在 `title` 内）；「流量(已用/总量)」与「到期时间」在窄屏折成两行但未溢出容器；无横向滚动。
- 裁定：**B.4 协议与 B.5 红线第 7 条均达成，T6/T7 通过验收**（task-6 / task-7 已 complete，rev 3）。

**遗留（不阻塞 Wave 1）**：1280 视口下窄列折行使行高略有起伏，Wave 3 落地时可用 white-space:nowrap + 更宽的列预算收平；样板的 title 兜底在真机 Vuetify 里应换成组件化 tooltip。

### B.8 T3b / T3c 交付验收（**Lead 独立复核**）

**T3c**（docs/design/research/T3c-gates.md，11355 字节，真实 242 行）逐条复跑，**全部命中**：
- web/vitest.config.ts:26-31 阈值确为 lines 100 / functions 100 / branches 80 / statements 100；web/package.json:15-18 脚本确为 typecheck / test / test:watch / coverage；.github/workflows/ci.yml:36-41 确为 Typecheck → Test + coverage gate → Build。
- 命中数复跑一致：data-test **326**、class= **47**、getByText|getByRole **0**、toMatchSnapshot **0**、spec 文件 **35**。
- 5 处 CSS 耦合断言核对为真：web/src/pages/SettingsPage.spec.ts:117,127（.text-error）、web/src/pages/RuleSourcesPage.spec.ts:94（.toContain('primary')）、web/src/layout/AppShell.spec.ts:66（.apptopbar-stub，:30 定义）、web/src/pages/OverviewPage.spec.ts:175（td.v-data-table__td--expanded-row button）、web/src/pages/GroupsPage.spec.ts:141（[data-testid="group-recipes"] .v-expansion-panel-title）。
- **Lead 补漏**：web/src/pages/OverviewPage.spec.ts **:194** 还有一处同形断言（与 :175 同选择器），报告只写了 :175 → Wave 3 动表格 DOM 时要看的是 **6 处**，不是 5 处。
- 口径瑕疵：报告写「Lines = 210」，实际 **242 行**（LF=242）——又是 PS 5.1 Get-Content 默认编码陷阱（附录 B.2 同款）。字节数 11355 正确。

**T3b**（docs/design/research/T3b-migration-cost.md，17585 字节，真实 184 行）复跑结果：
- 命中为真：<v-data-table 4 处 / 4 文件（web/src/components/ResourcePage.vue:147、web/src/pages/OverviewPage.vue:126、web/src/pages/AirportsPage.vue:243、web/src/pages/RulesPage.vue:211）；<v-form 与 rules= 在 web/src **0 命中**（「本仓不存在表单校验协议」成立）；show-select 仅 1 处（RulesPage.vue:211）；web/src/pages/AirportsPage.vue:37-47 确为 9 列 {key,title} 数组；web/src/router/index.ts:46-58 确为 **13 条业务路由**、:59 是 catch-all；text-medium-emphasis **44**。
- **数字偏差（Lead 实测 vs 报告）**：variant= 报告 109 → 实测 **107**；persistent-hint 报告 22 → 实测 **21**。偏差 ≤2，不影响 A/B/C 结论，但**后续引用一律以 Lead 实测 107 / 21 为准**。
- 结论采纳：**A 档 0 页 / B 档 8 页 / C 档 5 页**（分母 = router/index.ts:46-58 的 13 条业务路由，NotFoundPage 不计）；C 档真实改动面 = 3 个页面文件 + ResourcePage.vue 共享组件 + 2 个自带表格页（AirportsPage、OverviewPage）。

### B.9 T4 / T5 重派记录（原 subagent **零产出**）

- 原 T4 subagent 4dee93a9-d2c6-4086-8216-60eb7f9e1232、T5 subagent 8957cf5b-5d6b-4a7d-a116-542d16cfc305 长时间**既无产出也未汇报**；list_agents / job_list 里都不出现；send_message 直接报 active teammate "..." not found。
- **工具事实**：send_message 只能触达 list_agents 列出的 team teammate，**不能** steer subagent；subagent 也没有中断/查询接口 → subagent 一旦跑偏，Lead 只能**重派**。
- **教训（写入 Wave 3 派发纪律）**：subagent 只适合「短、窄、一次落盘」的任务。长文生成类任务必须在 prompt 里强制**分阶段落盘**（先 write 完整骨架，再 edit 逐段填内容），否则失败时零产出、颗粒无收。
- 重派（已发）：D1 → 4c10c288-681b-4d4e-be54-474eed64414f；D2 → eec2565e-6f5c-4179-8f05-ab22c00a257a。两个 prompt 均新增：分阶段落盘三条纪律、上下文将尽立即停止并落盘、明确的 render-check.ps1 自测命令、「上一轮同类任务零产出」的背景与「不许硬撑」的授权。

### B.10 T4（D1 Instrument）交付验收 —— Lead 复核**通过**

- 交付：E:\Projects\subforge\docs\design\mockups\d1-instrument.html，**59024 字节 / 1064 行**；执行 subagent 4c10c288-681b-4d4e-be54-474eed64414f（已结束）。
- 数值门禁：render-check.ps1 在 1600 与 1280 下均 **exit 0**（2 个 .table-wrap over=0，tableW 1140 / 964），截图 239586 / 232917 字节。Lead 在**最终文件**（mtime 05:07:47Z）上重跑，渲染结果与首轮逐字节一致（sha256 前 12 位 7c01ea61fe46 / 69587621520e），说明后续 +7.8KB 改动不影响渲染。
- **Lead 目视**（free 档模型不支持图像输入，目视只能由 Lead 做）：浅色默认主题、细边框分层、6px 圆角、面板四角 1px 刻度角标、8 张指标卡 4×2、机场表 9 列全部可见且列间有清晰留白、错误态红字与说明、底部 19 令牌表格含 OKLCH 列。**通过**。
- 假阳性澄清：报告称 Inter 命中 3 次，实为 CSS 词 pointer 的子串（d1-instrument.html:290 cursor: pointer;、:363 pointer-events: none;、:450 cursor: pointer;），无任何 font-family 命中。

### B.11 T5（D2 Editorial）交付验收 —— 数值通过、**目视发现真实缺陷 → 开 T8**

- 交付：E:\Projects\subforge\docs\design\mockups\d2-editorial.html，**46791 字节**；执行 subagent eec2565e-6f5c-4179-8f05-ab22c00a257a（已结束）。
- 数值门禁：render-check.ps1 在 1600 → 1 个 .table-wrap over=0（1214）；1280 → over=0（950）；均 exit 0。
- **Lead 目视发现（1600 与 1280 都复现）**：机场表数值列正文**粘连**，渲染成 42.1 GB / 200 GB2027-03-14 上次成功：12 分钟前● 正常。根因：该文件自 :399 起 table-layout: fixed + 逐列定宽（合计 944px）小于 9 列内容自然宽 1291px，而这些单元格 white-space: nowrap 却**没有** overflow:hidden / text-overflow:ellipsis → 文字溢出到右邻列。
- **门禁盲区（本轮最重要的教训）**：render-check.ps1 只量 .table-wrap **容器**；容器不溢出时，**单元格级溢出量不出来** → 此前的 PASS 是**假阴性**。中间版本（45702 字节）在 1280 已可见粘连，重写后 1600 也复现。**目视核对不可由数值门禁替代。**
- 新增门禁：E:\Projects\subforge\docs\design\mockups\cell-fit-check.ps1（逐 th/td 比较 scrollWidth 与 clientWidth，任一 over>0 即 FAIL，exit 1）。
- 修复派发：team task **task-11 = T8**，任务书 C:\Users\橙先生\AppData\Local\Temp\subforge-taskbooks\T8-d2-cellfit.md（7711 字节），写范围 = d2-editorial.html + cell-fit-check.ps1（其余文件禁改）；执行 subagent **c10e98e6-1273-4284-a412-063320c0bd2f**。验收 = **两个脚本在 1600 与 1280 下都 exit 0**，且「流量(已用/总量)」「到期时间」「更新时间」三列的具体值**完整可读**（订阅地址与状态列允许省略号 + title 兜底）。
- 卡片：task-4（T4）**complete**（rev 3）；task-5（T5）保持 in_progress，**验收挂在 T8 通过之后**；task-11（T8）in_progress。
## B.12 D1 回归事故与 T9 返工（2026-10-19）

- **事实**：已在 B.10 验收通过的 d1-instrument.html（59024 字节 / 1064 行）于 05:16:31Z 被**原 T4 执行者 subagent 4dee93a9**（此前被误判为“零产出/已死”）覆盖，先降到 38641 字节，最终落定 **44203 字节 / 782 行**。
- **Lead 复核 44203 版（不通过）**：render-check.ps1 → 1600 宽 `scrollWidth=1443 clientWidth=1230 over=213`、1280 宽 `1443 / 966 over=477`，两条均 FAIL、exit 1；Lead 目视：状态与操作列被挤出可视区、名称列竖排成“主/机/场”；grep 实测：「日志尾部」0 命中、「未匹配域名」0 命中、「table-layout」0 命中 → 违反共同内容契约与红线第 7 条（不允许需要滚动才能看到的关键列）。
- **处置**：开 team task **task-12 = T9**，写范围仅 docs/design/mockups/d1-instrument.html；任务书 C:\Users\橙先生\AppData\Local\Temp\subforge-taskbooks\T9-d1-fit.md（7881 字节）；执行 subagent **8c6c90f8-86ea-4590-a056-095543f5f05f**（provider=llmio, model=free, reasoning_effort=high）。验收 = render-check 在 1600 与 1280 都 PASS 且 over=0、Lead 目视 9 列全见、三个缺失区块补齐、19 个 --sf-* 变量不回退、深色 hover 对比度 ≥4.5:1。
- **口径修正**：**B.10 的 D1 验收结论作废**（被覆盖后不成立），D1 以 T9 交付后的复跑 + 目视结论为准。用户已看到的 D1 预览图对应的是 59024 版渲染，方向不变、细节以 T9 版为准。
- **教训（并入 B.9 派发纪律）**：
  1. subagent **无法中断**：interrupt_agent 与 send_message 对它都报 active teammate not found，一旦跑偏只能等它自己结束 → 派发前必须锁死写范围，长任务必须分阶段落盘并设 checkpoint。
  2. 判断 subagent 存活**不能**看 list_agents / job_list（subagent 从不进这两个列表），只能看 C:\Users\橙先生\.dsh\sessions\--E-Projects-subforge--\<subagentId>\session.v4.jsonl.zstd 的 mtime。
  3. 被覆盖的旧版本理论可从会话记录重建（该文件是多帧 zstd：按魔数 0x28B52FFD 切片后逐帧解压），但重建成本高于返工 → 优先开窄卡返工。
## B.13 T8 执行者死亡与重派（T10，2026-10-19）

- **T8 原执行者 subagent c10e98e6-1273-4284-a412-063320c0bd2f 判定失败**：会话记录时间线显示它 step1→step5 每步 6–24 秒，step6 从 05:14:41 起**再无任何记录**；至 05:33 静默逾 18 分钟（其自身节奏的 45 倍），全程**零落盘**——cell-fit-check.ps1 未创建、d2-editorial.html 未被改动。
- **重派**：同一张卡 task-11（rev 3，owner lead），执行 subagent **e43d299e-6060-4f94-ae0a-28c4d3006f63**（provider=llmio, model=free, reasoning_effort=high）。
- **任务书追加“重派补丁”**（T8-d2-cellfit.md 由 7711 → 9382 字节）：Lead grep 实测 d2-editorial.html 缺「日志尾部」0 命中、「未匹配域名」0 命中，与 D3 及验收版 D1 不等价 → 补回这两块（含 6 行日志原文与 Rule-Bot 正文），并要求补后仍须两种宽度双脚本 PASS。
- **并行卡片**：task-12 = T9（D1 返工）执行 subagent **8c6c90f8-86ea-4590-a056-095543f5f05f**，05:31:51 仍在活跃写盘。两张卡写范围互斥（T9 只碰 d1，T10 只碰 d2 + cell-fit-check.ps1），无冲突。
- **判定死亡的依据（可复用）**：解码 <id>\session.v4.jsonl.zstd 的多帧内容，打印每条记录的时间戳增量；若最后一条长期停在 `step/start <n>` 且远超该 subagent 自身平均步时，即判定挂死。仅凭“文件不动”不够，必须看它自己此前每步的耗时基线。
## B.14 T9 交付、Lead 复核与新门禁判决口径（2026-10-19）

### T9 交付（D1 返工）
- d1-instrument.html **44203 → 52364 字节 / 898 行**；执行 subagent 8c6c90f8-86ea-4590-a056-095543f5f05f（已完成并汇报）。
- 手段：`table-layout: fixed` + `<colgroup>` 九列定宽（7/8/8/6/14/10/15/9/23 %），td 一律 overflow:hidden + text-overflow:ellipsis + white-space:nowrap（d1-instrument.html:196-238、:437-456）；截断单元格补 title（:451-495）；补回「刷新全部流量·结果 / 日志尾部 6 行 / 未匹配域名 (Rule-Bot)」三区块（:520-583）；新增 `--sf-accent-strong`（:root #004b97 :65、dark #1f4c85 :91）修深色 hover 对比度 4.35:1 → 8.40:1。
- **Lead 独立复跑**：render-check.ps1 → 1600 `over=0` PASS exit 0（截图 256750 字节）、1280 `over=0` PASS exit 0（截图 239915 字节）。

### 新门禁 cell-fit-check.ps1 首次实战（T10 产出，Lead 复跑）
- D1：1600 → `PASS: 36 cells fit`；**1280 → FAIL 2 cells**：`table#0 row=3 col=1 over=4 scrollW=72 clientW=68 text=备用机场`、`row=4 col=1 同值 测试机场` → 名称列在 1280 下被压到 68px，4 字机场名需要 72px。**render-check 完全没发现这条。**
- D3：1600 → FAIL 1（`机场列表 row=4 col=8 over=34 scrollW=189 clientW=155 text= 上游 502`）；1280 → FAIL 1（over=37 scrollW=189 clientW=152）。

### Lead 判决口径修正（**取代**“任何单元格溢出即 FAIL”的初版口径）
初版口径把「设计上刻意的省略号截断」和「内容戳进邻列」混为一谈：前者 scrollWidth>clientWidth 是必然的，后者才是事故。修正为：
1. **FAIL（真溢出）**：`scrollWidth > clientWidth` **且** 该单元格 computed `overflow-x` 不是 hidden/clip **或** `text-overflow` 不是 ellipsis → 内容既没装下也没被干净裁掉（D2 的列粘连就属这一类）。
2. **FAIL（可访问性）**：任何发生截断（scrollWidth>clientWidth）的单元格**没有** title 兜底 → 全量文本无从获取。
3. **FAIL（硬约束，不给省略号豁免）**：名称列、节点数列、流量(已用/总量)、到期时间、更新时间 **必须在 1600 与 1280 下完整可读**（scrollWidth ≤ clientWidth）。理由：D2 事故正是把 `42.1 GB / 200 GB` 与日期挤成一团，数值不可读等于没有数据。
4. 允许干净省略号 + title 的仅限：订阅地址列、状态列（消息文本可能很长）。
5. 操作列不得因换行导致行高失控（可用 white-space:normal 折行，但需在报告中给出实测行高）。
这条口径是**契约**，实现由下一张卡（T11）落地；D1 名称列 4px 与 D3 状态列 34px 都必须按新口径重新判定后再谈通过。

### T12 交付（D1 名称列 1280 下 4px 截断）
- 执行 subagent `120ab062-a282-4e3f-8439-76e1af118e27`（task-14），**只改 d1-instrument.html 三行**：`d1-instrument.html:222` 注释、`:223` `.col-name{width:7%→8%}`、`:224` `.col-url{width:8%→7%}`，九列合计仍 100%，字节数不变 **52364**。
- 让宽方选择依据：订阅地址列的长 URL 由内层 `.url` span 自行省略号截断，td 自身 `scrollWidth==clientWidth`，让出 1% 不影响该列的门禁判定。
- **Lead 独立复跑**（非执行者自述）：render-check 1600 `T0 scrollWidth=1230 clientWidth=1230 over=0` PASS exit 0；1280 `966/966 over=0` PASS exit 0；cell-fit 1600 与 1280 均 `PASS: 36 cells fit (all over<=0)` exit 0。
- 1280 下九列实测宽：name77 url68 enabled77 nodes58 traffic135 expiry97 updated145 status87 actions222（全 >0，无 display:none）。

### D2 定稿与最终验收（T8/T10 并发收口）
- 同一张 task-11 被派发过两次，两个执行者**同时在改 d2-editorial.html**（字节数轨迹 46791→47102→47108→50805→54583→50886）；subagent 既不能 steer 也不能打断（send_message/interrupt_agent 都报 active teammate not found），Lead 只能等两者自然停止后验收。
- 结局：T8（`c10e98e6-1273-4284-a412-063320c0bd2f`）在自己的 turn 结束时**未产出报告即终止**（最后写入 05:47:29 / 54583 字节）；T10（`e43d299e-6060-4f94-ae0a-28c4d3006f63`）两次报告完成，其定稿 **50886 字节 @13:49:21** 为冻结版。`cell-fit-check.ps1` 全程仍是 T10 的 7538 字节版本，未被覆盖。
- **Lead 亲跑（权威）**：render-check 1600 `over=0`（tableW=1214，截图 232562 字节）、1280 `over=0`（950，截图 225632 字节）；cell-fit 1600/1280 均 `PASS: 36 cells fit`。
- **Lead 目视（free 档模型不接受图像输入，目视不可外包）**：两视口下机场表 9 列全部可见；`42.1 GB / 200 GB`、`2027-03-14`、`上次成功：12 分钟前` 完整可读（数值列靠折行而非截断消化宽度），**首版的列粘连已消除**；第三行状态列红色「上游 502，余额仍…」属允许的省略号截断且 title 兜底；「日志尾部」6 行（12:31:58→12:28:55）与「未匹配域名 (Rule-Bot)」空状态、19 个令牌的 4×4 色块网格均在。
- 遗留（不阻塞）：1280 下更新时间列折成「上次成功：12 分／钟前」两行，边界略紧但可读；D2 令牌区是色块网格、D1/D3 是表格，属三套方向的有意差异，不视为不一致缺陷。

### T11 v2 派发：门禁升级 + D3 裁定
- task-13（rev 4，owner lead，写范围 `cell-fit-check.ps1` + `d3-console.html` + `README.md`），执行 subagent `e45cd34a-9e90-41b0-b44f-a63ab5ec85d4`，任务书 `T11-cellfit-v2.md`（6639 字节）。
- 口径从「overflow-x 属性判定」细化为**可测的几何判据**，每条 BAD 必须写明判据编号：**SPILL**（单元格内任一后代 `getBoundingClientRect` 越界，且到该单元格之间无 `overflow-x ∈ {hidden,clip}` 的祖先）／**NOTITLE**（`td.scrollWidth>clientWidth` 却无 title 承载全量文本）／**STRICT**（表头含 名称·节点数·流量·到期时间·更新时间 的列必须 `scrollWidth<=clientWidth`）。
- **防「把门禁改松」的硬指标**：新增 `-SelfTest`，自建 4 个临时夹具自证——A 装得下→PASS；B nowrap 长文本无裁切无 title→FAIL 且含 SPILL/NOTITLE；C 订阅地址列省略号带 title→PASS；D 名称列省略号截断→FAIL 且含 STRICT。
- 通过标准：`-SelfTest` + d1/d2/d3 × 1600/1280 共七条全 exit 0，且 d1/d2 必须仍是 `PASS: 36 cells fit`（证明没被弄坏）。仓库里的 cell-fit-check.ps1 在 TEMP 版自测通过前必须保持可用（先写 `%TEMP%\cellfit-v2.ps1` 再覆盖）。

### T13 交付（D1 订阅地址列 1280 下只剩 "https:"）
- task-15，执行 subagent `ab9b8b97-...`，**只改 d1-instrument.html 三行**：`:222` 注释、`:224` `.col-url{width:7%→8%}`、`:231` `.col-actions{width:23%→22%}`；名称列保持 8%（T12 的成果不回退）。九列最终 **8/8/8/6/14/10/15/9/22 = 100%**，字节数不变 **52364**。
- 1280 下订阅地址列 td `clientWidth` 68→77、内层 `.url` span 44→53，可见前缀从 `https:` 恢复为 `https://`。
- Lead 亲跑复核：render-check 1600 `T0 1230/1230 over=0`（截图 256222 字节）、1280 `966/966 over=0`（240485 字节）；cell-fit 1600/1280 均 `PASS: 36 cells fit` exit 0。Lead 目视 1600 与 1280：名称列三行完整（主机场/备用机场/测试机场），九列全见，URL 列按设计省略号截断。

### T11 v2 交付与 Lead 独立验收
- 执行 subagent `e45cd34a-9e90-41b0-b44f-a63ab5ec85d4`，`cell-fit-check.ps1` 7538 → **16920 字节**（实现 SPILL / NOTITLE / STRICT 三判据 + `-SelfTest` 四夹具）；`README.md` 2680 → **3663 字节**（追加门禁一节）。
- **D3 状态列被判定为真 spill（不是合法省略号）**：修复前探针显示内层 `.status-text` 已裁，但外层 `.status`（`display:inline-flex`、无 `overflow`/max-width）`sw=185 cw=185`、`rect.right=1307.3 > cellRight=1273.7+0.5`。修复 = `d3-console.html:429` 新增 `.grid-airports .status { max-width: 100%; overflow: hidden; }`（+66 字节）；修复后 `span sw=147 cw=147`、`rect.right=1269.7 <= cellRight`。
- **Lead 亲跑（权威）**：`cell-fit-check.ps1 -SelfTest` → `SELFTEST PASS`，A/B/C/D 退出码 0/1/0/1 全符合预期；cell-fit 矩阵 d1/d2 均 `PASS: 36 cells fit`、d3 `PASS: 136 cells fit`（1600 与 1280 各一次，全部 exit 0）。

### T14 交付（D3 令牌表被竖向裁到只剩 7 行）
- **缺陷**：`.table-wrap { max-height: 320px; overflow: auto; }`（`d3-console.html:340`）被三处复用，令牌表 19 行被裁到约 7 行即被页脚截断，看起来像渲染 bug（Lead 目视发现，非门禁发现——横向门禁对这种竖向裁剪**天然盲区**）。
- task-16（rev 3，写范围仅 `docs/design/mockups/d3-console.html`），执行 subagent `a659fa80-5e98-4872-8711-195a60699485`；手段是**作用域收窄而非放宽口径**：`:340` 一字未改，新增 `:342-343` `.table-wrap.tokens-wrap { max-height: none; }`，`:885` 令牌容器加 `tokens-wrap` 类。**57911 → 58107 字节**。
- 执行者自测（副本探针，仓库脚本未动）：1600 与 1280 均 `tokenWrap sh=871 ch=871 rows=19`；计算样式探针 `WRAP|0 maxHeight=320px sh=134 ch=134`、`WRAP|1 maxHeight=320px sh=157 ch=157`、`WRAP|2 maxHeight=none sh=871 ch=871`。
- **Lead 亲跑复核**：grep 命中 `:340` 原样、`:343` 新规则、`:885` 类名；cell-fit 1600/1280 `PASS: 136 cells fit` exit 0；render-check 1600 `T0/T1/T2 全 980/980 over=0`（截图 313399 字节）、1280 `全 964/964 over=0`（304165 字节）。Lead 目视 1600：令牌表已连续可读（--sf-bg … --sf-info 在截图范围内，其余 3 行在 2400px 截图高度之外，页面本身无裁剪、无滚动条），机场表与服务端点表外观未变。

### Wave 1 最终状态（收口清单）
- 产物目录 `docs/design/mockups/` 恰好 6 个文件：`d1-instrument.html` 52364 / `d2-editorial.html` 50886 / `d3-console.html` 58107 / `render-check.ps1` 6637 / `cell-fit-check.ps1` 16920 / `README.md` 3663；无临时文件、无截图入库。
- 门禁矩阵（Lead 亲跑，全部 exit 0）：render-check × {d1,d2,d3} × {1600,1280}；cell-fit-check × {d1,d2,d3} × {1600,1280}；cell-fit-check `-SelfTest`。
- **下一步（Wave 2 决策）**：把三份样板的 1600 像素截图与 §4.2 三方向对比表交给用户**目视拍板**，选定一套方向并确认「不换框架、留在 Vuetify」；随后写 `docs/design/frontend-visual-upgrade-decision.md`，再按 §4.3 拆 Wave 3 落地卡。

---

## 附录 C：Wave 3 执行记录（2026-10-19）

### C.1 环境重启事实
- DSH 桌面端重启两次：第一次 15:07:16（PID 22464，GUI 56910→56166）；第二次为整机重启（Windows 15:23:55 起，DSH 15:25:53 起、PID 28216，GUI 64274）。
- 子智能体并发上限已核实：C:\Users\橙先生\.dsh\profiles\desktop\cordis.patch.yml:121-124 → @deepseek-ai/dsh-subagent / config: maxActiveSubagents: 30（同时活跃上限，不是历史累计）。

### C.2 W1 令牌层验收（Lead 亲跑）
- 产物 web/src/design/tokens.css 5657 B：浅色 :root 在 :30-54、深色组在 :68-91（四个选择器共用一份值：[data-theme="dark"]、.v-theme--dark、html.v-theme--subforgeDark、.v-theme--subforgeDark）；值逐字取自 docs/design/mockups/d1-instrument.html（浅 :54-72 / 深 :80-96），每行带出处注释。
- **Lead 裁定：保留第 4 个不带 html 前缀的选择器**。证据 web/node_modules/vuetify/lib/components/VApp/VApp.js:38（class 由 _normalizeClass(['v-application', theme.themeClasses.value, ...]) 生成）与 theme.js:264（生成 v-theme--<name>）→ 主题类落在 .v-application 而不是 html，只写 html. 前缀切换主题后不命中。
- --sf-shell: 1280px 采纳：D1 样板未定义该变量（grep 0 命中），1280px 取自 d3-console.html:108，与决策文档 §3「正文容器 ≤1280px」一致。
- web/src/plugins/vuetify.ts 2874 B：defaultTheme='subforgeLight'，subforgeLight / subforgeDark 各 11 个语义色键；必须写字面 hex（Vuetify 对 var() 调 parseColor 得 NaN：theme.js:124、colorUtils.js:81-91）。
- web/src/main.ts 112 B 新增 import './design/tokens.css'；main.spec.ts 用例 1→2、vuetify.spec.ts 5→7。

### C.3 W2 样式层验收（Lead 亲跑）
- web/src/styles/ 4 文件共 3854 B：index.css 189（@import base → vuetify-overrides → utilities）、base.css 1436、vuetify-overrides.css 1961、utilities.css 268；var(--sf- 23 处、字面 hex 0、禁用词全 0、!important 5 处。
- 关键覆写：--v-table-row-height: 40px / --v-table-header-height: 40px（Vuetify 默认 52/56，web/node_modules/vuetify/lib/components/VTable/_variables.scss:10,14）；.v-card 去阴影改 1px 边框 + 6px 圆角；.v-btn 34px / text-transform: none；.v-navigation-drawer 248px（!important，宽度由 JS 内联控制）；--sf-dur-micro: 150ms / --sf-dur-page: 400ms 定义在 vuetify-overrides.css:5-6（tokens.css 无同名变量，不重复）。
- **该层尚未被 main.ts 引用**（由 W5 接入），因此当前页面处于「令牌已换、样式层未接」的中间态。
- Lead 亲跑门禁：pnpm typecheck exit 0；pnpm test → Test Files 35 passed (35) / Tests 597 passed (597) / Duration 14.71s / exit 0（基线 594 → 597，只增不减）。

### C.4 roster 插件结论：本机不可用，已放弃（用户裁定）
- 插件 dsh-agent-team-roster 0.2.0（link:E:/Projects/dsh-agent-team-roster）确实在 profile bundle 列表里（C:\Users\橙先生\.dsh\profiles\desktop\package.json:23，依赖项 :43），但重启后其三个工具（rename_teammate / delete_teammate / cleanup_team）均未注册。
- 证据一：Lead 会话工具表恒为 62 个，三者 typeof 均为 undefined。证据二：**重启后新建**的一次性探针成员（roster2-cheap-probe，15:26 之后创建）自报 COUNT 62 / HAS_ROSTER no，与 Lead 完全一致 → 排除「旧会话工具表快照」解释。
- 根因（插件 README 自述）：E:/Projects/dsh-agent-team-roster/README.md:71「仅适配 dsh 0.2.0-rc.2（依赖 sessionProjections.registrations、journal 等内部接缝）。上游结构变化时插件会在激活阶段显式报错，而不是静默降级」；本机 DSH 为 **0.2.1-rc.2**（DeepSeek Harness.exe FileVersion 0.2.1-rc.2 / ProductVersion 0.2.1.0；app.asar 内 node_modules/@deepseek-ai/dsh-experimental-agent-team 版本串同为 0.2.1-rc.2）→ 版本漂移，插件按设计拒绝激活。
- 结论：**不再修 roster**（用户裁定「不搞这个了，直接继续」）。闲置成员（free-scout-frameworks / free-scout-recipes / free-route-probe / roster-cheap-probe / roster2-cheap-probe）保留在名册中；dsh 没有会话删除 API，已结束的 subagent 会话无法物理删除（README.md:68 同结论），只能靠排序。

### C.5 W3 / W4 派发（进行中）
- task-19 = W3（rev 2，in_progress，owner lead）：写范围 web/src/layout/**、web/src/App.vue(+spec)、web/src/components/PageHeader.vue(+spec)、web/src/components/AsyncSection.vue(+spec)；执行 subagent cbbecf6c-68a5-45a2-b35c-d31a65b3ef92（llmio/free, high）；任务书 C:\Users\橙先生\AppData\Local\Temp\subforge-taskbooks\T17-w3-shell.md（6426 B）。
- task-20 = W4（rev 2，in_progress，owner lead）：写范围 web/src/components/ResourcePage.vue(+spec)、web/src/pages/AirportsPage.vue(+spec)、web/src/pages/OverviewPage.vue(+spec)；执行 subagent 3e2b545e-4946-4dbf-80bf-a92b2744e885（llmio/free, high）；任务书 T18-w4-tables.md（5676 B，同目录）。
- 两份任务书均追加「追加约束」节：基线更新为 35 files / **597 tests**、强制分阶段落盘（先 write 骨架再 edit 逐段填，防零产出）、样式层尚未接线（W5 做）、并行 vite 缓存提示、禁止 pnpm build。
- 收口口径：G1 typecheck / G2 test / G4 coverage 全绿；被改 .vue 内字面 hex 0、无 outline: none；web/src/router/index.ts 的 5 组 13 项一字未改；最后由 Lead 亲跑全量门禁。

### C.6 下一步
- W5 集成收口（写范围 web/src/main.ts 补 import './styles/index.css'、web/src/styles/** 接手修、必要的 spec 断言修正）：跑 G1–G4 全量 + 修 6 处 CSS/DOM 耦合断言（web/src/pages/OverviewPage.spec.ts:175,:194；web/src/pages/SettingsPage.spec.ts:117,:127；web/src/pages/RuleSourcesPage.spec.ts:94；web/src/layout/AppShell.spec.ts:66；web/src/pages/GroupsPage.spec.ts:141）。
- W5 完成后由 Lead 做真机目视：起 dev server + 无头 Chrome 截图，与 docs/design/mockups/d1-instrument.html 对照，确认深色切换真生效，并更新决策文档 §7。

### C.7 Lead 复核发现的真问题：W3「深色真接线」只接了一半（已由 Lead 修好并加真断言）
- W3 自测（typecheck 0 / 35 files 598 tests 0 失败 / coverage 0）**全部通过，但不足以证明「实时切换」**：它的接线只有 AppShell 侧的 `watch(isDark, applyTheme, { immediate: true })`，而 `useThemeMode()` 当时是**每次调用各造一对 ref**（旧 useThemeMode.ts:45-61），SettingsPage（pages/SettingsPage.vue:19 + :111-113 三个按钮）调用 `theme.setMode(...)` 改的是**它自己那份副本**；AppShell 的 ref 不会变 → watch 不触发 → 只有 localStorage 被写，界面直到重新挂载/刷新才变。
- 为什么 W3 的测试没抓到：`web/src/layout/AppShell.spec.ts:143-151` 只覆盖**冷启动**（先 `localStorage.setItem('subforge.theme','dark')` 再 mount，assert `document.documentElement.dataset.theme` 与 `v-theme--subforgeDark`），而 `web/src/pages/SettingsPage.spec.ts:20-25` 把 useThemeMode **整个 mock 掉**了（只断言 `mockSetMode` 被调用）→ 两头都绿，中间那段没人测。**典型假绿。**
- Lead 修法（未派新卡，Lead 直接改）：`web/src/composables/useThemeMode.ts` 引入 `WeakMap<Storage, ThemeState>`（themeStateFor 在 :45-60 附近）——同一个 storage 实例（浏览器里就是 `window.localStorage`）上的所有调用者共享同一对 ref；每次取用时再与 storage 对齐一次（storage 是真源，refs 是共享视图）。不同 storage（测试里各自 new 的 MemoryStorage）仍互相隔离。
- 新增 3 条真断言（全部为端到端/跨实例，不再依赖 mock）：`useThemeMode.spec.ts` 的「同一 storage 的两个实例共享状态：写侧 setMode，读侧同步」与「不同 storage 的实例互相隔离」；`AppShell.spec.ts` 的「深色实时切换：设置页侧 setMode("dark") 立即改动 data-theme 与 Vuetify 主题名（无需重挂载）」——挂载 AppShell 后用**另一个** useThemeMode 实例 setMode，断言 `dataset.theme` 变 dark 且 html 含 `v-theme--subforgeDark`。
- 修后 Lead 亲跑：`pnpm typecheck` exit 0；`pnpm test` → **Test Files 35 passed (35) / Tests 601 passed (601) / exit 0**；`pnpm coverage` exit 0（All files 100% Stmts / 97.58% Branch / 100% Funcs / 100% Lines）。基线由 598 → **601**。
- 教训（写进派发口径）：**「接线类」交付必须要求一条跨组件/跨实例的端到端断言**（A 组件写、B 组件读到），只断言「初始值渲染正确」或「方法被调用」都是假绿。

## 附录 D：W5 集成收口 + W6 修复波 + 最终验收（2026-10-19 晚）

### D.1 W5 集成收口（Lead 直接改，未派新卡）
- web/src/main.ts 现 5 行：:1 import { createSubforgeApp } from './bootstrap'、:2 import './design/tokens.css'、:3 **import './styles/index.css'（W5 新增）**、:5 createSubforgeApp().mount('#app')。顺序契约：令牌在前、样式层在后（样式层大量 var(--sf-*)）。
- web/src/main.spec.ts:29-42 原断言是 W1 期口径 expect(source).not.toContain("import './styles/")（注释明写「样式层不属于 W1 写范围」）→ W5 按新状态改写为顺序断言：tokensIndex > bootstrapIndex、stylesIndex > bootstrapIndex、stylesIndex > tokensIndex。这是**中间态断言的正当翻转**（每处变强不变弱），不是迁就实现。
- W5 后门禁：pnpm typecheck exit 0；pnpm coverage exit 0（35 files / 601 tests，All files 100/97.59/100/100）；pnpm build exit 0（构建到 ../internal/api/web/dist，该目录 gitignored，构建后 git status 未见 dist）。

### D.2 真机截图验收（Lead 亲跑，仓库零污染）
- 工具链：node v24.21.0（自带全局 WebSocket/fetch，裸写 CDP）；后端 go build -o %TEMP%\subforge-run\subforge.exe ./cmd/subforge（53,063,680 B）以 serve -listen 127.0.0.1:8080 -data-dir %TEMP%\subforge-run\data 后台运行；前端 vite dev（web/vite.config.ts 代理 /api /sub /subscribe /version /healthz /getruleset /inspect → 127.0.0.1:8080）。
- 临时脚本（均在 %TEMP%\subforge-run\，不进仓库）：seed.mjs（POST 3 机场/3 分组/3 规则/1 自建节点，全 201）、shoot.mjs（CDP 批量：新 target → setDeviceMetricsOverride 1600×1000 → setEmulatedMedia prefers-color-scheme → 导航并写 localStorage subforge.theme → 等 .v-skeleton-loader/.v-progress-linear 清零 → Page.captureScreenshot）、probe2.mjs（量 thead th 宽高/行高/table top/help 面板高/bar 背景）、probe3.mjs（elementFromPoint + 计算样式链）。
- 截图入库：docs/design/screenshots/ 下 w6-final-*.png 共 7 张（overview/airports 明暗 + settings/groups/rules 浅色）+ d1-instrument-mockup-light.png（D1 样机 1600×2394）；w5-real-*.png 8 张保留作为修复前对照。

### D.3 Lead 目视抓到的三个真缺陷（截图 + CDP 量测双重证据）
- **缺陷 1 · 深色首帧闪白**：web/src/plugins/vuetify.ts:19 写死 defaultTheme: 'subforgeLight'，AppShell.vue:28-29 才在挂载后切主题 → 首帧必为浅色。无头截图里固定顶栏整条卡在浅色（position:fixed + transform 的独立合成层不重绘）恰好把这个真 bug 暴露出来。旁证：AppTopBar.vue:14 的 color="surface" 与 :34-37 scoped .topbar{background:var(--sf-surface)} 是双重来源。
- **缺陷 2 · 表格毛**：1600px 下 9 列 th 高 43px 且折行（名 称 / 启 用 / 节点 数 / 流量(已\n用/总量) / 到期时\n间），tbody 行高 57px，操作列折 2 行；列宽 名称78/订阅地址412/启用79/节点数81/流量113/到期88/更新88/状态74/操作218。
- **缺陷 3 · 帮助面板抢戏 + 文案重复**：机场页 5 个帮助面板占首屏约 240px（tableTop=477）；「管理上游订阅地址（机场），供转换与订阅使用。」在 PageHeader 与帮助面板重复；概览页同一句出现 3 次；概览 9 个指标卡在 4 列网格下最后一行只有 1 个孤卡。

### D.4 W6 修复波（3 张卡，全部 subagent(provider=llmio, model=free, high)）
- 写范围互斥：W6A=useThemeMode.ts(+spec)/bootstrap.ts(+spec)/plugins/vuetify.ts(+spec)/AppTopBar.vue/AppShell.vue(+spec)/main.ts；W6B=ResourcePage.vue(+spec)/AirportsPage.vue(+spec)/styles/vuetify-overrides.css(+base.css)；W6C=HelpPanel.vue(+spec)/OverviewPage.vue(+spec)。任务书 W6-common.md / W6A-theme-firstpaint.md / W6B-tables.md / W6C-help-panel.md 在 C:\Users\橙先生\AppData\Local\Temp\subforge-taskbooks\。
- **W6A（深色首帧）** subagent 56fe4638-863c-428a-992a-dd94f7ee4667：useThemeMode.ts 3805 B 抽单一真值源（LIGHT_THEME_NAME/DARK_THEME_NAME/themeNameFor/documentThemeFor/resolveTheme/applyTheme），bootstrap.ts 989 B 新增 applyFirstPaintTheme() 并在 app.use(vuetify) **之前**写入主题名与 data-theme；AppTopBar.vue 去掉 color="surface"（背景只留一处来源）。自测 615 passed。首帧探针：{"dataTheme":"dark","barBg":"rgb(30, 33, 36)","vThemeSurface":"30,33,36"}。
- **W6B（表格）** subagent bd55e65b-bafa-44b0-9434-11a11c0cb235：根因是**选择器权重**——Vuetify 自带 .v-table > .v-table__wrapper > table > thead > tr > th（VTable.css:68-92，0,2,4），W2 写的 .v-table th（0,1,1）被静默覆盖，th 一直是 14px/padding 16px/weight 500。修法：覆写层复用同形选择器链（vuetify-overrides.css:30-68）→ th 12px/600/nowrap/padding 0 12px；AirportsPage.vue:37-57 九列 minWidth（88/72/128/96/152/72/288）+ 数值列 align:'end' + 操作列 cellProps.class='sf-table__actions'；ResourcePage.vue:44-58,147 新增 cellPropsFor()（**不改 headers 数组**，保全 ResourcePage.spec 的 toEqual）。
- **W6C（帮助面板 + 概览）** subagent 948ee843-4e1e-413b-8a49-328e2a594ad8：HelpPanel.vue 3987 B 改渐进披露——常驻入口 button[data-testid=help-toggle][aria-expanded] 内是原 oneLiner，详情包进 div[data-testid=help-body] v-if=open，文案一条未删；OverviewPage.vue 删掉与概览卡重复的 oneLiner，.metrics-grid ≥960px 由 4 列改 3 列（3×3 无孤卡），.metric-card min-height:126px。
- **Lead 亲测复核（probe2，1600×1000）**：airports helpH 321→36、tableTop 477→**192**；9 列 th 全 h=40 且单行（scrollWidth=clientWidth）、tbody 行高 40、操作列单行 297px、wrapper 无横向滚动；overview 网格 3 列（405.328×3）、9 卡等高 126px、行数 4+4+1 → 3×3。
- 4 项遗留的 Lead 裁定：①操作按钮 padding 0 6px（非 4/8 倍数）保留——改 8px 会挤爆 1230px 表宽；②订阅地址截断 380→200px 保留；③traffic_err 长文案不 clamp——错误原文完整优先；④td 字号仍 14px 暂留，作为后续可选项。

### D.5 Lead 收口（本轮 Lead 亲改，非子代理交付）
- **内建文案中文化**：plugins/vuetify.ts 加 import { zhHans } from 'vuetify/locale' 与 locale: { locale: 'zhHans', fallback: 'en', messages: { zhHans } }（package exports "./locale" → lib/locale/index.js；zh-Hans.js:29-36 定义 dataFooter.itemsPerPageText='每页数目：'、pageText='{0}-{1} 共 {2}'）。动机：表格页脚原本是整页唯一英文残留（Items per page / 1-3 of 3）。vuetify.spec.ts 加 1 例逐字比对这两个 key（该文件 8 例）。
- **去掉「页面头部 ↔ 帮助面板」重复句**：HelpPanel.vue 新增 label?: string → toggleLabel = props.label ?? props.help.oneLiner；新增 hasBody（steps/concepts/faq/links 或 non-dense 且有 whenToUse），无内容时退化成静态 .help-static 一行（保留 data-testid="help-oneliner"，未知 helpKey 的兜底不再变成点不开的空按钮）；PageHeader.vue:19 改为 <HelpPanel … label="使用说明" />。
- **断言翻转（6 个 spec 文件，每处变强）**：LogsPage / TasksPage / ConvertPage / OverviewPage / SettingsPage / SubscriptionsPage 里 panel.text()).toContain(oneLiner) 一律改为「.page-header__subtitle 含 oneLiner + 入口行=使用说明 + help-body 不存在」；HelpPanel.spec 拆 3 例（无 label 用 oneLiner / 有 label 不重复 / 无内容静态兜底）。注意其中 4 处原本断言的正是「重复」本身。

### D.6 最终门禁（Lead 亲跑，权威数字）
- pnpm -C web run typecheck → exit 0。
- pnpm -C web run coverage → exit 0：**Test Files 35 passed (35) / Tests 618 passed (618)**，All files 100% Stmts / 97.47% Branch / 100% Funcs / 100% Lines。
- pnpm -C web run build → exit 0：index-BqEBUwB6.js 648.69 kB（gzip 206.87）、PageHeader-4nZqsM2O.js 32.45 kB、AirportsPage-BOgHvZvS.js 10.62 kB、ConvertPage-CYs8VxVl.js 11.41 kB、SubscriptionsPage-BJfS115s.js 13.38 kB、ScriptsPage-CBpYc_sM.js 8.86 kB；末尾 [ensure-dist-placeholder] 已确保 E:\Projects\subforge\internal\api\web\dist\.gitkeep。
- 用例数轨迹：594（基线）→ 597（W1/W2）→ 601（W3/W4 + Lead 补断言）→ 615（W6A/W6C）→ **618**（Lead 加 locale 用例 + spec 重排）。只增不减。

### D.7 环境坑（写进后续派发口径）
- pwsh 里没有 pnpm：必须 & '<node.exe>' '<pnpm.mjs>' -C web run <script>。
- edit 会随机报 ReplaceFileW EIO (Win32 32)（Windows 共享冲突，疑 vite watcher/杀软）；**加重试退避（最多 8 次 × 2.5–4s）即可成功**，个别文件（OverviewPage.spec.ts）要等更久。诊断手段：[System.IO.File]::Open($p,'Open','ReadWrite','None') 判断是否真被锁。
- 并行卡的中间态会让别人的 typecheck 变红（例：W6C 正在改 HelpPanel.vue 时 W6A 看到 TS6133 'hasWhen'），**不能当失败证据**；全量门禁只由 Lead 在全部落地后跑一次。
- String.raw 模板里 \${f} 不会插值（会写成字面量导致 ENOENT），Windows 路径拼接要用普通字符串。

### D.8 第三次部署到测试机（2026-10-19，commit e82048b）

- 交叉编译：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -mod=vendor -ldflags "-s -w -X …buildinfo.Version=dev -X …Commit=e82048b -X …Date=2026-10-19T08:37:45Z" -o bin/subforge-linux-amd64 ./cmd/subforge` → **40,005,794 B**，sha256 `e27f4d412c994a2470b9d43bc9aa4f7f35eb373c0a17748c766882e4533e52ba`。
- 上传 → 远端 sha256 校验 → `chmod 0755` → `mv -f` 原子替换 `/tmp/subforge` → 同参数 `setsid nohup` 重启（新 pid 73227）。备份 `/tmp/subforge.bak-20261019T083843Z` + `subforge.db.pre-e82048b`。
- **数据零丢失**：8/8 端点 `curl | md5sum` 升级前后完全一致（airports/groups/rules/tokens/self-nodes/pipelines/rule-providers/traffic-strategy）。
- **前端切换**：`GET /` → `/assets/index-BqEBUwB6.js`（200 / 648,698 B）+ `index-BG5Oc447.css`（200 / 837,627 B），旧 `index-DaRdirkZ.js` 404；deep link 200；`/api/nope` 404；启动日志无 error/panic/fatal；公网端点全 200。
- 真机截图（无头 Chrome 80 公网 URL，入 `docs/design/screenshots/prod-210-*.png` 4 张）：深色主题在真实环境生效（含固定顶栏、无首帧闪白），帮助面板收成「使用说明」一行。
- 详细验收表与回滚命令见 `docs/design/ui-copy-and-config-plan.md` §5.6。
- **未改的遗留观感问题**：真实机场仅 1 条时，`traffic_err` 原文在「流量」与「状态」两列重复出现、各撑 3–5 行（行高约 130px）。限行会削弱 §6.2「错误原文完整呈现」的验收点，故保留并交用户拍板。
- 工具链教训：`tools/rsh210.py run` 打印含非 GBK 字符的远端日志会抛 `UnicodeEncodeError`（加 `tr -cd '\11\12\15\40-\176'` 过滤）；`setsid nohup … &` 会让 SSH 通道挂着直到 paramiko 超时——**部署其实已成功，必须用独立只读脚本复核**。

### D.9 W7 修复波（用户 7 点反馈，2026-10-19 晚，全部 llmio/free · high subagent 交付）

- **触发**：用户 7 点反馈（m01794）：①整体配色丑 ②启用 chip 不可见 ③目标格式含义太长 ③b INI 规则源缺失 ④长文案应悬浮 ⑤订阅 per-token 质疑 ⑥规则源看不懂 ⑦管线不能选已有脚本。
- **W7A**（57a7d4ae）：AirportsPage 启用/状态列 v-chip→.sf-status 色点+文字，traffic 列「余额未更新」短文案 + title 原文；utilities.css 增 .sf-status 5 类。
- **W7B**（bdd0adc7）：ResourcePage / RuleSourcesPage 去 chip；**W7F**（f2750023）：Rules/Tasks/AppTopBar 清零 chip + 品牌药丸→wordmark。全仓 v-chip=0。
- **W7C**（e4fcdd10）：ConvertPage 目标格式含义默认折叠（v-show + target-meanings-toggle）。
- **W7D**（54ab79f9）：PipelinesPage ref 参数 v-text-field→v-combobox（scriptNames + kind 不匹配提示）。
- **W7E**（4a478f08）：help.ts 'rule-sources' 文案重写；Lead 翻转 RuleSourcesPage.spec:319 + GLOSSARY 对齐。
- **W7G**（905ed649）：OverviewPage 9 处 tonal→白卡描边；AirportsPage 裸开关容器化。**W7H**（244fafba）：9 文件 v-card tonal 清扫（15 处），v-alert/v-btn tonal 保留——全仓 v-card tonal=0（Lead grep 复核）。
- **W7I**（a094d762）**假绿打回**：自报完成但插槽未转发+排名 Map 空占位，功能不生效；**W7I-bis**（287f07af）返工：ResourcePage 新增通用 rankKey（动态插槽+computed 排名），GroupsPage 传 rank-key="sort_order"。真机验证顺序列显示 1/2/3（原 19 位时间戳）。
- **配色**：3 套样板（palette-p1-paper/p2-slate/p3-forest.html，docs/design/mockups/）→ 用户拍板 **P3 森林柔和（Everforest 系）** → **W7J**（19374a1d）落地：tokens.css 浅深两组 + vuetify.ts 两主题全换 P3（accent #4F6E35/#A5C08B），几何不动；vuetify.spec 颜色断言逐键翻转。
- **覆盖率门禁修复**：W7K（c7c5cf06）补 ResourcePage rank 守卫分支（:56/:66/:79）；W7L（813f90b0）补 PipelinesPage combobox 分支（:69/:90/:131-132/:219/:281，纯追加 168 行 spec）。PipelinesPage:219 `?? false` 死分支裁定：接受 99.17% branch 不修。
- **终版门禁（Lead 亲跑）**：typecheck exit 0；coverage exit 0（**35 files / 628 tests**，All files 100/97.97/100/100）；build exit 0（index-C69113kr.js 648.85 kB / gzip 206.94）。
- **终版截图**：docs/design/screenshots/w7-p3-{overview,airports,settings,groups,rules}-{light,dark}.png 7 张（P3 配色真机，groups 图可见顺序列 1/2/3）。
- 用例数轨迹：594（基线）→ 601（W3/W4）→ 618（W6）→ 621（W7A-F）→ 622（W7I-bis）→ 624（W7K）→ 628（W7L）→ **631**（W7M）。
- **W7M 文案卡**（094ea706，用户裁定 ③b/⑤ 走文案路线不实现功能）：调查确认 INI 只是「转换配置模板」（internal/template/ini.go:1、internal/config/override.go:79-86），与规则源无关；规则源落盘格式由清单 output.format 决定（internal/manifest/manifest.go:112-122、internal/subforge/rulesources.go:32-43）。help.ts:179/187/190-191/329 文案与实现对齐（「不含 INI 模板」+ output.format→扩展名映射）；令牌可长期复用写进 help.ts:233/236-238 与 SubscriptionsPage.vue:233（依据 internal/api/subscription.go:515-528 无过期逻辑）。help.spec.ts:142-161 与 SubscriptionsPage.spec.ts:187-194 新增钉住断言。终版门禁：typecheck 0 / **631 tests** coverage 0（All files 100/97.97/100/100）/ build 0（index-kFHihQ-D.js 648.85 kB）。
- **遗留待用户拍板**：③b INI 规则源补实现方案、⑤订阅 per-token 定制化方案；机场页「刷新时同时导入规则」开关容器位置偏表格下方（样式已容器化，摆位未动）。

### D.10 W7 部署（第四次部署，2026-10-19）

- 用户拍板：1 提交+部署执行；2/3（③b INI、⑤ per-token）走文案路线不实现功能 → W7M 文案卡交付后随 W7 波一并提交部署。
- 提交 **3120fef**（40 files +2327/−197，后端 Go 零改动）；交叉编译 40,009,890 B / sha256 f344b399…；部署测试机 210（pid 75673），备份 bak-20261019T103234Z + db.pre-3120fef；验收：6 端点 md5 与 e82048b 一致（零丢失）、airports/traffic 两端点为用户运行期合法变更、index-kFHihQ-D.js 上线、日志零错误。证据详录 ui-copy-and-config-plan.md §5.7。
- 经验：复验脚本路径拼错（/api/traffic-strategy ≠ /api/traffic/strategy）会让 md5 对比变成 404 假象；md5 前必须确认 200。

### D.11 W8 部署（第五次部署，2026-10-19，commit d326ed5）

- **前置**：W8A 机场页开关摆位（**2ac8d4b**，web）+ W8D 订阅令牌字段级定制 + W8E INI 规则源展开（**d326ed5**，Go）。用户拍板：W8F 前端令牌弹窗派发 + commit+部署并行执行。
- **门禁（Lead 亲跑）**：Go build/vet/test ./... -count=1 全 exit 0；web typecheck 0 / **35 files 632 tests** exit 0（632 = 628 + W8A 1 + W8D 3 文件内新增）。工作区提交后干净。
- **事故与恢复（部署中断 + PE 二进制）**：①首次部署脚本因 SSH 通道中断死在 kill 之后 mv 之前，服务停机约 8 分钟；②恢复时发现第一次交叉编译**漏带 GOOS=linux GOARCH=amd64**，产出 Windows PE（MZ 头），远端 bash 报 `MZ@: not found`；③重编译显式 env 带 CGO_ENABLED=0 GOOS=linux GOARCH=amd64，本地校验 ELF 魔数（7f 45 4c 46）后再上传。**规程补充：交叉编译后必须先验 ELF 魔数再上传；远端 pgrep 用 `[s]` 类括号技巧防自匹配；多步部署脚本不可依赖 SSH 通道存活，须写成远端脚本文件执行。**
- **部署事实**：二进制 40,038,562 B，sha256 60fde837…，buildinfo Date=2026-10-19T11:31:00Z；备份 /tmp/subforge.bak-20261019T112341Z + db.pre-d326ed5；新 pid **77576**。
- **验收**：healthz ok；version → commit d326ed5；8 端点 md5 对比 7/8 SAME（零丢失），tokens CHANGED 为 W8D 预期（5 新字段 target/pipeline/config/include/exclude 空默认已出现在响应）；GET / 引 index-DjKcmap-.js（与本地构建同名）；deep link 200；/api/nope 404；日志 error/panic/fatal 0 命中（仅 1 条 PE 事故残留 bash 文本）；公网 healthz 200。
- **并行完成**：dsh-subagent-model-config 插件接缝修复（startContinuable→startActivation 17 处，0.3.2）+ 用户配置修复（规则 match:"*-free" → llmio/free/high，CONFIG_FILE 在 E:\Projects\dsh-subagent-model-config\config.json），teammate 模型路由打通；probe 取证仅 debug:true 时写，debug:false 无日志≠未触发。

## D.12 W8F 交付与第六次部署验收（2026-10-19）

### W8F 订阅令牌字段级定制（前端）
- 派发通道：任务卡 e5fd17ce（钉 llmio/free + high）——spawn_teammate 报 Team member limit 8（7 个僵尸队友占席，roster 插件版本漂移不可用），走任务卡执行通道绕过。执行会话 session-3ac91bb8，succeeded。
- 交付：c4bfd67。endpoints.ts SubToken 扩 5 字段 + created_at、SubTokenUpdate 与后端 6 个 *string 对齐、updateToken → PUT /api/tokens/{encodeURIComponent(token)} 解包 .token（subscription.go:451 响应形状 {"token": t}）；SubscriptionsPage 弹窗 target 下拉懒加载 /api/targets（失败降级「留空 = 默认 clash.meta」）+ pipeline/config/include/exclude 折叠进默认收起的「高级选项」；clearable 清空给 null 会被后端当「不改动」，提交前归一空串；新建分支只发 {token, name}。
- Lead 复核：写范围仅 3 文件 ✓；补 endpoints.spec.ts updateToken 用例 1 条（teammate 汇报的 coverage 缺口 endpoints.ts:385）。
- 门禁（Lead 亲跑）：typecheck 0；test 35 files / 642 tests（632→642）exit 0；coverage exit 0（All files 100/98.02/100/100）；build exit 0（index-6sKpCJhT.js 648.85 kB / gzip 206.94）。

### 第六次部署（c4bfd67 → 210.us1.firadio.net）
- 二进制 40,038,562 B，ELF 魔数 7f 45 4c 46 上传前已验，sha256 c6cd645b6d0bde77a0bf74aa0de753ff714677bb2fcc59a70d7cfbdc684f70ff，buildinfo Date=2026-10-19T12:30:00Z Commit=c4bfd67。
- 切换走远端脚本文件 /tmp/switch6.sh（D.11 事故规程），无通道中断：pid 77576 → 79101，备份 /tmp/subforge.bak-20261019T121302Z + subforge.db.pre-c4bfd67。
- 验收：healthz ok；version "subforge dev (commit c4bfd67, built 2026-10-19T12:30:00Z) backend"；GET / 引 index-6sKpCJhT.js（与本地构建同名）；deep link 200；/api/nope 404；8 端点 md5 与 D.11 基线逐字全同（tokens dcf00444 含新字段与 created_at，确认 W8D 迁移数据无损）；日志 error/panic/fatal = 0；公网 healthz 200。
- 环境坑：python 的 user site 是相对路径 ~\Python\Python314（pip 在仓库目录跑 --user 就装进仓库）→ paramiko 重装到固定 PYTHONUSERBASE=C:\Users\橙先生\.dsh-py-user，rsh210.py 调用需带该 env；rsh210 远端输出含非 GBK 字符（如 µ）时 sys.stdout 崩（gbk codec），探针输出加 LC_ALL 或过滤。
- 交叉编译 ldflags：-s -w -X …buildinfo.{Version=dev,Commit=c4bfd67,Date=…}；env 三件套 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 不可漏。

### 部署后状态
- 提交链：2ac8d4b → d326ed5 → 2da8597 → c4bfd67。工作区 clean。
- 待办：teammate 派发恢复需 roster 插件适配 0.2.1-rc.2（8 席满员 7 僵尸）。
