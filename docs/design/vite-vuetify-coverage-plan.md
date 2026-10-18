# SubForge 前端 Vite + Vuetify 迁移与前后端覆盖率门禁 · 指导性实施计划

| 项 | 值 |
| --- | --- |
| 文档路径 | docs/design/vite-vuetify-coverage-plan.md |
| 版本 | v1 · 2026-10-18 |
| 上位指令 | m15836「前端換 vite vuetify，前后端加覆盖率门禁 A100% B 80%」 |
| 前序文档 | docs/design/web-ui-rewrite-plan.md（906 行；模块边界、数据模型、流量聚合、API 补全设计仍然有效） |
| 本文件取代 | 前序文档 §5（前端技术栈）与 §6（实施顺序）；前序文档其余章节继续有效 |
| 读者 | 实现者（人或 agent）。按 §8 阶段顺序施工，每阶段独立可构建、可验证、可提交 |

---

## 0. 这份计划解决什么

### 0.1 三条硬要求

1. **换栈**：删掉 `internal/api/web/` 下的手写 HTML/CSS/JS，前端改为 Vite + Vue 3 + Vuetify + TypeScript 工程，源码在 `web/`，构建产物回灌给 Go embed。
2. **门禁 A（前端层）= 100%**：Vue/TS 源码的行、分支、函数、语句四项覆盖率全部 100%，由 `vitest run --coverage` 在阈值不达标时非零退出。
3. **门禁 B（后端层）= 80%**：Go 语句覆盖率 ≥ 80%，由自研 `internal/tools/coveragegate` 在阈值不达标时 exit 1。

### 0.2 A 层 / B 层与前后端的对应关系（**已由用户确认**）

| 解读 | 依据 | 落地位置 |
| --- | --- | --- |
| **A 层** = 前端层 = 100% | 句子先说「前端換 vite vuetify」，再说「前后端加覆盖率门禁 A100% B 80%」；A/B 与「前端/后端」顺序对应 | `web/vitest.config.ts` 的 `coverage.thresholds` |
| **B 层** = 后端层 = 80% | 同上；且后端现状 80.37% 恰好贴着 80% 线，是「够用就好」的门槛语义 | `Makefile` / `.github/workflows/ci.yml` 里 coveragegate 的 `-min 80` |

> **已确认（2026-10-18）**：用户答「是A层和B层」，即 **A 层 = 前端层、B 层 = 后端层**，与本表一致。以下备选解读仅作为「若判断有误」的改动面记录：`web/vitest.config.ts` 四个阈值改 80、coveragegate 的 `-min` 改 100（后者意味着后端要 100%，工作量约为本计划的 4 倍）。

### 0.3 为什么前端要 100% 而基线是 0

前端现在是**没有任何自动化测试**的手写 JS：5 个 HTML 页面 + 6 个静态资源 + 约 23.6 KB 的 `js/subscription.js`。100% 门禁不是「把现有代码补到 100%」，而是**换栈时按可测结构重写**，让 100% 成为工程结构的自然结果，而不是靠补测试硬凑。本计划的可测结构来自三条设计约束：

- 逻辑下沉到纯函数与 composable（无 DOM 依赖，测试便宜）
- 页面只做「取数 → 渲染 → 提交」三段，分支数量用共享组件压到最低
- 所有交互通过 `data-testid` 暴露，测试用 DOM 交互而不是 `wrapper.vm` 内部调用

### 0.4 与 web-ui-rewrite-plan.md 的关系

前序 plan 已经确定了**做哪些模块、数据怎么存、API 长什么样**（13 页 / 5 域、§1.1 能力覆盖矩阵 A1–A26、§2 数据模型、§3 流量余额聚合、§4 API 补全、§3.6 脚本、§3.7 上游规则导入）。这些**与本计划不冲突，继续有效**，本计划只替换它的前端技术栈与施工顺序两节。

| 主题 | 归属 |
| --- | --- |
| 13 页模块边界、跳转契约、A1–A26 能力矩阵 | web-ui-rewrite-plan.md §1 |
| 表结构变更、Go struct、流量余额聚合、脚本、上游规则导入 | web-ui-rewrite-plan.md §2 / §3 |
| 后端 API 补全清单（按页 6 张表） | web-ui-rewrite-plan.md §4 |
| 前端技术栈、目录、共享层契约、覆盖率门禁 A、测试组织 | **本文件 §2 / §3 / §5** |
| 后端覆盖率门禁 B、补测工作清单 | **本文件 §4** |
| Go 侧 embed/路由/旧测试改造、CI、Makefile | **本文件 §6 / §7** |
| 施工顺序与提交点 | **本文件 §8**（取代前序 §6） |

---

## 1. 现状基线（全部可复核，不要凭记忆）

### 1.1 前端现状

| 事实 | 值 | 来源 |
| --- | --- | --- |
| 页面数 | 5（index / convert / rules / subscription / logs） | `internal/api/web/` |
| 静态资源 | app.css、app.js、js/{convert,dashboard,logs,rules,subscription}.js | `internal/api/web/` |
| 最大单文件 | `js/subscription.js` ≈ 23.6 KB，占前端 JS 约 70% | 文件大小 |
| 构建步骤 | 无（源码即产物，直接 embed） | `internal/api/embed.go` |
| 自动化测试 | 无（仅有 Go 侧的 HTML 字符串断言） | `internal/api/web_ui_test.go` |
| 类型检查 | 无 | — |
| 覆盖率 | 0%（无 JS 测试） | — |

Go 侧现在通过 `//go:embed web` 把 `internal/api/web/` 整个目录打进二进制，`web_ui_test.go`（1077 行）用字符串断言校验页面标题、DOM 标记和 `/app.css` `/app.js` `/js/*` 的 Content-Type。

### 1.2 后端覆盖率基线

复核命令（在仓库根，vendored 模式）：

```
go test -coverprofile=coverage.out -covermode=atomic ./...
go run ./internal/tools/coveragegate -profile coverage.out -min 80 -top 40
```

实测结果：

| 口径 | 覆盖率 | 语句数 |
| --- | --- | --- |
| 全仓 `./...` | 71.27% | 4634 / 6502 |
| 排除 `cmd/` | 78.21% | — |
| 排除 `cmd/` + `internal/tools/` | 77.63% | — |
| **仅 `./internal/...`** | **80.37%** | **4570 / 5686** |
| 仅 `./internal/...` 排除 tools | 79.84% | — |

逐包明细（语句数 / 覆盖率）：

| 包 | 语句 | 覆盖率 | 包 | 语句 | 覆盖率 |
| --- | --- | --- | --- | --- | --- |
| internal/api | 981 | 73.3% | internal/render/mellow | 135 | 90.4% |
| cmd/subforge | 642 | 7.9% | internal/parse | 134 | 79.1% |
| internal/config | 567 | 88.5% | internal/scheduler | 104 | 67.3% |
| internal/convert | 559 | 83.7% | internal/store | 94 | **0.0%** |
| internal/render/singbox | 456 | 79.4% | internal/model | 93 | 84.9% |
| internal/render/lineform | 364 | 84.1% | internal/render/registry | 71 | 94.4% |
| internal/transform | 292 | 93.5% | internal/rulebot | 68 | 97.1% |
| internal/render/sharelink | 217 | 95.9% | internal/render/sssub | 64 | 92.2% |
| internal/ruleset | 190 | 97.4% | internal/runtime | 62 | 87.1% |
| internal/template | 179 | 87.7% | internal/subscheck | 52 | 92.3% |
| internal/render/clashmeta | 173 | 86.7% | internal/mrs | 45 | 26.7% |
| internal/tools/coveragegate | 166 | 98.2% | internal/mcp | 36 | 47.2% |
| internal/fetch | 162 | 88.3% | pkg/subforge | 20 | 65.0% |
| internal/manifest | 142 | 79.6% | internal/buildinfo | 1 | 0.0% |
| internal/render | 140 | 86.4% | | | |
| **internal/subforge** | 139 | **0.0%** | | | |

> 根目录 `tools/{ssh-runner,remote-run,probe-links,size-probe}` 覆盖率全 0，但它们被 `.gitignore` 的 `/tools/` 规则忽略、并未入库，因此不计入门禁。

### 1.3 门禁 B 的口径为什么取 `./internal/...`

| 候选口径 | 覆盖率 | 采纳 | 理由 |
| --- | --- | --- | --- |
| `./...` | 71.27% | ✗ | 被 `cmd/subforge`（642 语句 / 7.9%）拖死。CLI 的 main 分支（flag 解析、os.Exit、信号处理）测起来收益极低、噪声极大 |
| 排除 `cmd/` | 78.21% | ✗ | 仍低于 80%，且把未入库的 `tools/` 混进来统计没有意义 |
| **`./internal/...`** | **80.37%** | **✓** | 覆盖全部业务逻辑；排除 CLI 胶水；不统计未入库脚本。**这是门禁口径** |

代价必须写清楚：**`cmd/subforge` 不设门禁**。缓解手段是在 §4.3 里为 CLI 补一批「参数解析 / 用法文本 / 退出码」的表驱动测试，但只作为建议项、不进门禁，避免为了数字给 CLI 写仪式性测试。

### 1.4 差距算术：要补多少条语句

「仅 `./internal/...`」当前 4570 / 5686 = 80.37%，贴线且无余量。按包补齐到目标后：

| 包 | 语句 | 当前覆盖 | 目标 | 需新增覆盖 |
| --- | --- | --- | --- | --- |
| internal/store | 94 | 0 | 80% | +76 |
| internal/subforge | 139 | 0 | 80% | +112 |
| internal/scheduler | 104 | 70 | 85% | +19 |
| internal/mrs | 45 | 12 | 85% | +27 |
| internal/mcp | 36 | 17 | 85% | +14 |
| **小计** | **418** | **99** | — | **+248** |

结果：`(4570 + 248) / 5686 = **84.7%**`，已越过 80% 并有 4.7 个百分点余量。

再补 `internal/api`（981 语句，73.3% → 80%，+66 条）：`(4818 + 66) / 5686 = **85.9%**`。

**可行性已实测取证**：曾让 teammate 对 `internal/store` + `internal/subforge` 补测，中断前实测 **store 91.5% / subforge 96.4%**（`go test -count=1 -cover ./internal/store/... ./internal/subforge/...` exit 0）。这两个包恰好是「0% 且语句最多」的两个，它们一旦达标，门禁 B 就从「贴线」变成「有余量」。**该取证产物按「只写 plan」的要求已删除，数字记录在此，重做时按 §4.3 重写。**

---

## 2. 技术栈冻结

### 2.1 依赖与精确版本（已在 `web/` 实测安装通过：pnpm 11.28.5，11.2 s，132 包）

运行时依赖：

| 包 | 版本 | 用途 |
| --- | --- | --- |
| vue | 3.5.44 | 框架 |
| vue-router | 4.6.4 | 路由（13 页 + 404） |
| vuetify | 3.13.7 | 组件库（含 `vuetify/styles`） |
| @mdi/font | 7.4.47 | 图标字体 |

开发依赖：

| 包 | 版本 | 用途 |
| --- | --- | --- |
| vite | 8.3.4 | 构建 / 开发服务器 |
| @vitejs/plugin-vue | 6.0.10 | SFC 编译 |
| typescript | 5.9.3 | 类型检查 |
| vue-tsc | 3.3.13 | SFC 类型检查（`build` 前置） |
| vitest | 5.0.5 | 测试运行器 |
| @vitest/coverage-v8 | 5.0.5 | v8 覆盖率 |
| @vue/test-utils | 2.5.1 | 组件挂载 |
| jsdom | 30.1.3 | DOM 环境 |
| @types/node | 24.19.2 | `vite.config.ts` 类型 |

安装命令（Node/pnpm 不在 PATH，必须用绝对路径）：

```
node C:/Users/橙先生/.dsh/dsh-runtimes/dsh-primary-runtime/dependencies/node/bin/node.exe C:/Users/橙先生/.dsh/dsh-runtimes/dsh-primary-runtime/dependencies/pnpm/bin/pnpm.mjs install
```

### 2.2 明确不选什么（决策与理由，避免实现者反复）

| 候选 | 不选的理由 |
| --- | --- |
| Vuetify 4.2.6 | 本机 `web_search` 不可用（HTTP 401），无法核对 4.x 的破坏性变更；3.13.7 的组件 API 稳定且有本地类型定义可查 |
| TypeScript 7.0.2 | `vue-tsc 3.3.13` 只声明 `>=5.0.0`；TS 7 是 Go 重写版，工具链兼容性未知，风险不划算 |
| `vite-plugin-vuetify` 的 `autoImport` | 自动导入让测试环境的组件解析依赖插件时序，测试更脆；改为在 `src/plugins/vuetify.ts` 里从 `vuetify/components` / `vuetify/directives` 全量注册，测试可预测 |
| `mergeConfig` 复用 `vite.config.ts` | 覆盖率阈值与 alias 在两份配置间同步易出错；`vitest.config.ts` 独立定义，重复但显式 |
| 换用其它 UI 库（Element Plus 等） | 用户明确指定 Vuetify |

### 2.3 目录结构（最终形态）

```
web/
  package.json
  pnpm-lock.yaml
  tsconfig.json
  vite.config.ts
  vitest.config.ts
  index.html
  .gitignore
  tests/
    setup.ts                 # 全局 stub（不进覆盖率统计）
    resourceFlow.ts          # 共享 CRUD 流程测试助手（不进覆盖率统计）
    renderPage.ts            # 挂载 + 路由/插件装配助手（不进覆盖率统计）
  src/
    env.d.ts
    main.ts                  # 极薄：createSubforgeApp().mount('#app')
    bootstrap.ts             # createSubforgeApp()：便于测试复用装配逻辑
    App.vue
    api/
      client.ts              # HttpError / buildQuery / setTokenProvider / request / api.get|post|put|del
      endpoints.ts           # 全部 REST 端点 + 领域类型 + subscribeUrl()
    format/
      index.ts               # formatBytes / formatExpire / formatPercent / formatDuration / shortTime
    composables/
      useNotify.ts           # 全局 snackbar 状态
      useThemeMode.ts        # light/dark/system + localStorage
      useAsync.ts            # data/error/loading/load
      useCrud.ts             # 按整行操作的 CRUD 状态机
    router/
      index.ts               # navItems 13 条 / navGroups() / routes / createAppRouter()
    plugins/
      vuetify.ts
    layout/
      AppShell.vue
      AppNav.vue
      AppTopBar.vue
    components/
      PageHeader.vue
      AsyncSection.vue
      UnderConstruction.vue
      ResourcePage.vue       # 通用 CRUD 页
    pages/
      OverviewPage.vue       ConvertPage.vue      PipelinesPage.vue
      ScriptsPage.vue        AirportsPage.vue     SelfNodesPage.vue
      RulesPage.vue          RuleSourcesPage.vue  GroupsPage.vue
      SubscriptionsPage.vue  TasksPage.vue        SettingsPage.vue
      LogsPage.vue           NotFoundPage.vue
```

每个 `.vue` / `.ts` 旁边一个同名 `*.spec.ts`（放同目录，便于「改文件时测试就在眼前」）。

### 2.4 构建产物契约（前端与 Go 的唯一接口）

| 项 | 约定 |
| --- | --- |
| 产物目录 | `vite.config.ts` 的 `build.outDir = '../internal/api/web/dist'`，即**直接产出到 Go 的 embed 目录** |
| `emptyOutDir` | `true`（每次构建先清空，避免陈旧 chunk 残留被 embed） |
| `sourcemap` | `false`（不进二进制） |
| `chunkSizeWarningLimit` | `1500`（Vuetify 全量注册后单 chunk 较大，避免噪声告警） |
| `base` | `'/'`（Go 侧从根路径提供资源） |
| 路由模式 | `createWebHistory()` → 要求 Go 侧有 SPA fallback（见 §6.2） |
| 是否入库 | **`web/dist/` 不入库**，由 CI 构建（见 §7.3 的漂移校验）。理由：产物是哈希文件名，入库会让 diff 噪声巨大且必然过期 |
| 开发态代理 | `server.proxy` 把 `/api /sub /subscribe /version /healthz /getruleset /inspect` 代理到 `http://127.0.0.1:8080` |

---

## 3. 覆盖率门禁 A：前端 100%

### 3.1 门禁机制

`web/vitest.config.ts` 的覆盖率配置（关键字段逐条固定）：

| 字段 | 值 | 说明 |
| --- | --- | --- |
| `coverage.provider` | `'v8'` | 与 vitest 5 同版本，无需额外 babel 插桩 |
| `coverage.reporter` | `['text', 'json-summary']` | `text` 给人看；`json-summary` 给 CI/脚本读 |
| `coverage.include` | `['src/**/*.ts', 'src/**/*.vue']` | 只统计自己写的源码 |
| `coverage.exclude` | `['src/**/*.spec.ts', 'src/env.d.ts']` | 测试自身与纯声明文件不计 |
| `coverage.thresholds.lines` | `100` | 四项全部 100 |
| `coverage.thresholds.functions` | `100` | |
| `coverage.thresholds.branches` | `100` | |
| `coverage.thresholds.statements` | `100` | |
| `coverage.all` | `true` | **关键**：未被任何测试 import 的文件也要计入（否则「不写测试的文件」会凭空消失，门禁形同虚设） |
| `coverage.reportsDirectory` | `'./coverage'` | 已在 `web/.gitignore` 忽略 |

**为什么「全局 100%」等价于「逐文件 100%」**：覆盖率是一个加权平均，权重是各文件的语句数。要让平均值等于 100%，每个文件的比值都必须等于 100%——没有任何文件能用「超过 100%」去摊平另一个文件的缺口。因此不需要额外配置 `perFile: true`（该选项在 v8 provider 上行为不稳定），全局阈值已经足够严格。

### 3.2 统计范围与边界（必须显式声明，否则门禁可被绕过）

| 路径 | 是否统计 | 理由 |
| --- | --- | --- |
| `src/**/*.ts` `src/**/*.vue` | ✅ | 全部自研源码 |
| `src/**/*.spec.ts` | ❌ | 测试不是被测物 |
| `src/env.d.ts` | ❌ | 只有 `/// <reference>` 与 `declare module`，无运行时语句 |
| `tests/**` | ❌ | 在 `src/` 之外，天然不计；放测试助手（`resourceFlow.ts` `renderPage.ts`） |
| `src/main.ts` | ✅ | 极薄（一行 mount），用一条 spec `import('../main')` 覆盖；因为会真的 mount 到 `#app`，spec 里需要先在 `document.body` 造一个 `#app` |
| `internal/api/web/dist/**` | ❌ | 构建产物，不在 `web/src` |
| `web/dist/` | ❌ | 同上 |

> 若把「纯类型文件」（只有 `interface` / `type`）单列，v8 会报 `0/0`，vitest 视作 100%，不会误伤。但**不要为了规避而新建纯类型文件**——类型跟着使用方走，减少文件数就是减少门禁面。

### 3.3 分支覆盖陷阱清单（100% 门禁的真正难点）

100% 语句容易，100% **分支**才是工作量所在。以下每一条都必须在实现时主动处理，否则门禁会在最后卡住：

| 陷阱 | 例子 | 处理方式 |
| --- | --- | --- |
| 模板里的 `v-if` / `v-else-if` / `v-else` | `AsyncSection` 的四态 | 每个分支写一条 spec（错误 / 加载 / 空 / 正常） |
| 模板里的 `??` 与 `?:` | `props.emptyText ?? '暂无数据'` | 分别传 / 不传该 prop |
| 模板里的 逻辑与 / 逻辑或 | 条件渲染 | 两侧各测一次 |
| `v-for` 空数组 | `AppNav` 的 13 条导航 | 正常路径即覆盖，但 `navGroups()` 需单独测空数组与多组输入 |
| 函数默认参数 | `useThemeMode(storage = window.localStorage)` | 显式传参与省略各一次 |
| `try/catch/finally` | `useAsync` / `useCrud` / `request` | 必须有一条 spec 触发 catch（mock 抛错），否则 catch 块整段未覆盖 |
| 早退分支 | `formatBytes` 的 `<= 0`、`doDelete` 的 `row === null` | 各写一条边界 spec |
| 可选链 `?.` | `route.meta.title` | 有值 / 无值各一次 |
| 三元运算符 | `field.type === 'switch' ? ... : ...` | 每种取值走一次（`ResourcePage` 的三种字段类型都要被真实使用） |
| `switch` 每个 case | 若实现里用 switch | 每个 case + default |
| 空值守卫 | `if (text === '') return` | 空与非空各一次 |

**设计层面的规避原则**：能用查表替代分支就用查表。例如 `formatBytes` 的单位选择用数组下标而不是 5 个 `if`，分支数从 10 降到 3。**分支越少，100% 越便宜**——这是本计划要求「逻辑下沉到纯函数」的真实原因。

### 3.4 测试组织（让 14 个页面 spec 保持廉价）

| 文件 | 作用 | 为什么放这里 |
| --- | --- | --- |
| `tests/setup.ts` | 补齐 jsdom 缺失的 `ResizeObserver` / `IntersectionObserver` / `matchMedia` | Vuetify 组件在挂载时会用到；不补会抛错。放 `tests/` 不进统计 |
| `tests/renderPage.ts` | `renderPage(component)`：装配 Vuetify 插件 + 路由 + 返回 `mount` 结果 | 避免 14 个页面 spec 各写一遍 20 行装配代码 |
| `tests/resourceFlow.ts` | `exerciseCrudPage(options)`：跑完「加载 → 新增 → 编辑 → 删除 → 取消 → 失败」全流程 | 4 个 ResourcePage 页面（机场/自建节点/规则/分组）的 spec 因此各只需约 15 行 |

原则：

- **用 DOM 交互而不是 `wrapper.vm`**：点击 `data-testid` 元素才能同时覆盖模板分支与处理函数。
- **网络层用 `vi.stubGlobal('fetch', ...)`**：不引入 msw，保持依赖最小；每个 spec 自备一个返回固定 JSON 的假 fetch。
- **`vitest --coverage` 必须能一次跑通**：不允许 `--coverage.thresholds.100=false` 之类的临时豁免进入仓库。

### 3.5 前端门禁验收命令

```
cd web
node <node.exe> <pnpm.mjs> run typecheck        # vue-tsc --noEmit，必须 0 错误
node <node.exe> <pnpm.mjs> run coverage         # vitest run --coverage，阈值不达标即非零退出
node <node.exe> <pnpm.mjs> run build            # vue-tsc --noEmit && vite build
```

`coverage/index.html` 是给人看的定位工具：**门禁失败时先看它，不要靠猜**。

---

## 4. 覆盖率门禁 B：后端 80%

### 4.1 门禁机制

自研工具 `internal/tools/coveragegate`（Go，无第三方依赖，自身覆盖率 98.2%）。它解析 Go 覆盖率 profile，按包聚合，判定阈值并给出可读结论。

| 项 | 值 |
| --- | --- |
| 入口 | `go run ./internal/tools/coveragegate -profile coverage.out -min 80` |
| 退出码 | `0` 达标 / `1` 低于门禁 / `2` 参数或 profile 错误 |
| 可选参数 | `-top N` 打印覆盖率最低的 N 个包；`-json` 输出机器可读结果 |
| 容差 | 阈值比较带 `1e-9` 个百分点容差，避免 `1/3` 这类除法的浮点尾差误判 |
| 为什么不用 `go tool cover` | `go tool cover -func` 只有函数级文本输出，没有阈值判定与退出码，无法当门禁 |

用法示例：

```
go test -coverprofile=coverage.out -covermode=atomic ./internal/...
go run ./internal/tools/coveragegate -profile coverage.out -min 80 -top 20
```

### 4.2 门禁口径与排除项

| 项 | 决定 | 代价 / 缓解 |
| --- | --- | --- |
| 统计范围 | `./internal/...` | 排除 CLI 与未入库脚本 |
| `cmd/subforge` | 不进门禁 | CLI 胶水（flag、os.Exit、信号）测试性价比低。缓解：可选补一批表驱动测试，但不进阈值 |
| `tools/*`（根目录） | 不进门禁 | 被 `.gitignore` 的 `/tools/` 忽略、未入库，统计它们没有意义 |
| `internal/tools/coveragegate` | 进门禁 | 它是「被测物」不是「测试设施」，自身 98.2%，不影响达标 |
| `pkg/subforge` | **不进门禁** | 只有 20 条语句、65%，但它是公开 API 门面（薄封装）；纳入后对总数影响 < 0.3%，收益与噪声不成比例。**若要纳入，需在 §10 验收清单里显式加一行** |
| 合并模式 | `-covermode=atomic` | 与 CI 一致；`atomic` 在并发场景下不会漏计 |

### 4.3 逐包补测工作清单（这是本节的交付物，不是建议）

按 §1.4 的算术，五个包补齐即可让 `./internal/...` 达到约 84.7%。

| 序 | 包 | 语句 | 当前 | 目标 | 需覆盖 | 补测要点 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `internal/store` | 94 | 0% | 80% | +76 | 全部 CRUD 走真 SQLite（`t.TempDir()` 建库）：建表/迁移幂等、Airport/ProxyGroup/SelfNode/Rule/SubToken 的增删改查、唯一约束冲突、`GetXxx` 不存在时返回错误、`sort_order` 排序、时间戳字段 |
| 2 | `internal/subforge` | 139 | 0% | 80% | +112 | Manager 门面：对每个 `List/Get/Add/Update/Delete` 的成功与失败路径；`GenerateSubscription` 用 httptest 提供上游订阅；`getConfig/setConfig` 往返；类型别名不需要单独测 |
| 3 | `internal/scheduler` | 104 | 67% | 85% | +19 | 任务注册与取消、非法 cron 表达式、执行失败时的告警发送（用 httptest 假 webhook）、告警通道解析失败分支 |
| 4 | `internal/mrs` | 45 | 27% | 85% | +27 | 缓存路径生成、TTL 过期与未过期、缓存文件缺失、源文件不存在、非法 behavior/format、往返一致（已有部分用例） |
| 5 | `internal/mcp` | 36 | 47% | 85% | +14 | 工具注册表、未知方法、参数解码失败、JSON-RPC 错误响应 |

可选第 6 项：`internal/api`（981 语句，73.3% → 80%，+66 条）——把 §6.3 里被改写的页面/资源测试补回来，顺带把 API 覆盖率抬到 80%，使全仓 `./internal/...` 达到约 85.9%。

**测试编写纪律（血泪教训）**：

- **不要用模板字符串生成含 Go 反引号的代码**（例如结构体 tag）。此前一个 teammate 因此产出 `missing ',' in argument list` 和 `string literal not terminated` 的坏文件。写 Go 测试要么手写，要么用 `String.raw` 且不含反引号。
- 补测**不得改动被测代码来迁就测试**（例如为可测性加导出函数）。确有必要时，改动必须单独成一个 commit 并说明理由。
- 每个包补完立刻 `go test -cover ./internal/<pkg>/...` 自查，不要等全仓跑完再定位。

### 4.4 余量策略

门禁 80%，**目标定 85%**。理由：

1. 现状 80.37% 是「贴线」，任何一次重构删掉几个测试就会掉线；
2. 贴线会诱导实现者写「为覆盖率而覆盖率」的仪式测试；
3. 留出 5 个百分点后，新增功能可以先提交、后补测，而不是每加一个函数就被门禁卡住。

### 4.5 后端门禁验收命令

```
go build ./...
go vet ./...
go test -coverprofile=coverage.out -covermode=atomic ./internal/...
go run ./internal/tools/coveragegate -profile coverage.out -min 80 -top 20
```

> **Windows 注意**：PowerShell 会把 `-coverprofile=coverage.out` 拆成两个参数，报 `FAIL .out [setup failed]`。必须用 `cmd /c` 脚本或 Node `execFileSync('go', [argv...])` 直接传参，不要把这条命令贴进 pwsh。

---

## 5. 前端页面与共享层规格（指导性）

### 5.1 13 页与 5 域

模块边界沿用 `web-ui-rewrite-plan.md` §1，此处给出**前端视角**的落地映射（页面文件、数据来源、主要交互）：

| # | 页面 | 文件 | 域 | 数据来源 | 主要交互 |
| --- | --- | --- | --- | --- | --- |
| 1 | 概览 | `OverviewPage.vue` | 转换 | `GET /api/stats` + `GET /api/history` | 只读展示统计卡与最近历史表 |
| 2 | 转换 | `ConvertPage.vue` | 转换 | `GET /api/targets` + `GET /api/pipelines` + `POST /api/convert` | 表单（目标/URL/文本/包含/排除/管线）→ 结果文本 + 复制 |
| 3 | 管线 | `PipelinesPage.vue` | 转换 | `GET /api/pipelines` | 只读列表 + 说明（增删改属后续阶段） |
| 4 | 脚本 | `ScriptsPage.vue` | 转换 | — | 占位（`UnderConstruction`），后端已存在、出口待建 |
| 5 | 机场 | `AirportsPage.vue` | 来源 | `/api/airports` CRUD | `ResourcePage` 通用 CRUD |
| 6 | 自建节点 | `SelfNodesPage.vue` | 来源 | `/api/self-nodes` CRUD | `ResourcePage` 通用 CRUD |
| 7 | 规则 | `RulesPage.vue` | 规则 | `/api/rules` CRUD | `ResourcePage` 通用 CRUD |
| 8 | 规则源 | `RuleSourcesPage.vue` | 规则 | `GET /api/rulesets` | 只读两张表（规则集映射 / 规则列表） |
| 9 | 分组 | `GroupsPage.vue` | 规则 | `/api/groups` CRUD | `ResourcePage` 通用 CRUD |
| 10 | 订阅 | `SubscriptionsPage.vue` | 输出 | `/api/tokens` CRUD + `subscribeUrl()` | 令牌 CRUD + 生成订阅 URL + 复制 |
| 11 | 任务 | `TasksPage.vue` | 运维 | — | 占位（`UnderConstruction`），后端 scheduler 已有 |
| 12 | 设置 | `SettingsPage.vue` | 运维 | `GET /api/targets` + `GET /api/stats` | 只读能力清单 + 主题切换 |
| 13 | 日志 | `LogsPage.vue` | 运维 | `GET /api/logs?since=N` | 轮询拉取 + 暂停/继续 + 清屏 |
| 14 | 404 | `NotFoundPage.vue` | — | — | 返回概览链接 |

**导航分组**（`src/router/index.ts` 的 `navItems`，每组一个 `v-list-subheader`）：

| 组 | 页面 |
| --- | --- |
| 转换 | 概览 / 转换 / 管线 / 脚本 |
| 来源 | 机场 / 自建节点 |
| 规则 | 规则 / 规则源 / 分组 |
| 输出 | 订阅 |
| 运维 | 任务 / 设置 / 日志 |

### 5.2 共享层契约（先写这一层，再写页面）

| 模块 | 导出（精确签名） | 覆盖率要点 |
| --- | --- | --- |
| `api/client.ts` | `HttpError extends Error {status: number; body: string}`；`buildQuery(query?: Record<string, QueryValue>): string`；`setTokenProvider(fn: () => string): void`；`request<T>(method: string, path: string, options?: RequestOptions): Promise<T>`；`api.get/post/put/del` | 每个早退分支（204、空 body、非 JSON、token 为空、body 为 undefined）各一条 spec |
| `api/endpoints.ts` | 领域类型（Airport / ProxyGroup / SelfNode / Rule / SubToken / HistoryEntry / LogEntry / Stats / RulesetsResponse / LogsResponse / DiffResponse …）+ `endpoints.*` 全部端点 + `subscribeUrl(origin: string, token: string): string` | 每个端点的 URL 与解包逻辑各一条 spec；`subscribeUrl` 需对 token 做 `encodeURIComponent` 断言 |
| `format/index.ts` | `formatBytes(n: number): string`；`formatExpire(unixSeconds: number, nowMs?: number): string`；`formatPercent(used: number, total: number): string`；`formatDuration(ms: number): string`；`shortTime(iso: string): string` | 每个函数用表驱动覆盖所有区间与边界（0、负、恰好等于阈值） |
| `composables/useNotify.ts` | `notifyState`；`notify(text: string, kind?: NotifyKind): void`；`closeNotify(): void`；`snackbarColor(kind: NotifyKind): string` | 三种 kind + 关闭 |
| `composables/useThemeMode.ts` | `THEME_STORAGE_KEY`；`THEME_MODES`；`isThemeMode`；`readStoredMode(storage)`；`systemPrefersDark()`；`resolveDark(mode)`；`useThemeMode(storage?)` | `system` 模式在深色/浅色下的两条路径；storage 读写失败（抛错）路径 |
| `composables/useAsync.ts` | `errorMessage(err: unknown): string`；`useAsync<T>(loader): {data, error, loading, load}` | 成功与失败各一次；`errorMessage` 对 Error 与非 Error 各一次 |
| `composables/useCrud.ts` | `CrudResource<T>`；`CrudState<T>`；`useCrud<T>(resource)` | `save` 的 create/update 两分支 + 失败分支；`destroy` 成功/失败 |
| `router/index.ts` | `NavItem`；`navItems`；`navGroups()`；`routes`；`createAppRouter(history?): Router` | `navGroups` 需覆盖「全部分组有序」「空数组」；`createAppRouter` 传参与省略各一次 |
| `plugins/vuetify.ts` | `vuetify` | 一条 spec 断言 `createVuetify` 产物非空即可 |
| `bootstrap.ts` | `createSubforgeApp(): App` | 一条 spec 断言挂载成功 |
| `main.ts` | 无导出（副作用） | 一条 spec 先造 `#app` 再 `import('../main')` |

### 5.3 共享组件契约

| 组件 | Props | Emits / Slots | 覆盖点 |
| --- | --- | --- | --- |
| `PageHeader.vue` | `{title: string; subtitle: string}` | slot `actions` | 有 / 无 slot 内容 |
| `AsyncSection.vue` | `{loading: boolean; error: string; empty: boolean; emptyText: string}` | emit `retry`；default slot | **四态各一条**：error（点重试要触发 emit）、loading、empty、正常 |
| `UnderConstruction.vue` | `{page: string; note?: string; plan?: string}` | — | `note` 有/无、`plan` 有/无 四种组合 |
| `ResourcePage.vue` | `{title, subtitle, columns: {key,title}[], fields: {key,label,type}[], resource: CrudResource<Record<string, any>>, readonlyMode?: boolean, emptyText?: string}` | — | 见下 |

`ResourcePage.vue` 是覆盖率压力的集中点，必须用共享助手 `tests/resourceFlow.ts` 一次性跑完以下路径：

1. 首次加载成功 → 表格渲染
2. 首次加载失败 → `async-error` 出现 → 点 `async-retry` → 成功
3. 加载成功但列表为空 → `async-empty`
4. `readonlyMode` → 无「新增」按钮、无「操作」列
5. 点「新增」→ 对话框出现 → 三种字段类型（text / number / switch）都渲染 → 提交成功 → 提示「已新增」
6. 提交失败（resource.create 抛错）→ 提示错误、对话框保持打开
7. 点「编辑」→ 对话框带原值 → 提交成功 → 提示「已更新」
8. 点「取消」→ 对话框关闭
9. 点「删除」→ 确认框 → 点「取消」→ 不删
10. 点「删除」→ 点「删除」→ 成功 → 提示「已删除」
11. 删除失败 → 提示错误

**data-testid 契约**（测试与实现之间的接口，改名即破坏测试）：

| testid | 位置 |
| --- | --- |
| `add` / `refresh` | `ResourcePage` 头部按钮 |
| `table` | `v-data-table` |
| `edit` / `delete` | 行内操作按钮 |
| `dialog-cancel` / `dialog-submit` | 编辑对话框 |
| `confirm-cancel` / `confirm-delete` | 删除确认框 |
| `field-text` / `field-number` / `field-switch` | 表单字段 |
| `async-error` / `async-loading` / `async-empty` / `async-retry` | `AsyncSection` 四态 |
| `snackbar` | `AppShell` 的全局提示 |
| `nav` / `drawer-toggle` / `topbar` | 布局 |

---

### 5.4 每页规格与覆盖点

| 页面 | 规格 | 必须覆盖的路径 |
| --- | --- | --- |
| 概览 | 4 张统计卡（历史条数 / 缓存命中率 / URL 总数 / 日志行数）+ 最近历史表（时间、目标、节点、分组、规则、缓存）+ `last === null` 时显示空态 | 有 `last` / 无 `last`；`stats` 请求失败 |
| 转换 | 目标选择（`targets` 下拉）、URL 多行输入、粘贴文本输入、包含/排除正则、管线下拉（含「不使用管线」）、排序开关、列表开关；提交后右侧显示结果并支持复制 | 成功；失败（后端返回错误）；「URL 与文本都为空」的本地校验分支 |
| 管线 | 只读列表，每行显示管线名 + 一句说明；空态提示「在配置文件中定义管线」 | 有数据 / 空数据 |
| 脚本 | `UnderConstruction`，说明 goja 已实现但无出口，指向 plan §3.6 | 一条挂载断言 |
| 机场 | `ResourcePage`：列 `name / subscription_url / remarks / enabled`；字段 `name(text) / subscription_url(text) / remarks(text) / enabled(switch)` | `exerciseCrudPage` 全流程 + resource 适配函数（`item.id` 透传） |
| 自建节点 | `ResourcePage`：列 `link / name / enabled`；字段 `link(text) / name(text) / enabled(switch)` | 同上 |
| 规则 | `ResourcePage`：列 `rule_type / rule_param / rule_config / resolve_dns / enabled`；字段 `rule_type(text) / rule_param(text) / rule_config(number) / resolve_dns(switch) / enabled(switch)` | 同上（含 number 与 switch 字段类型） |
| 规则源 | 两张只读表：「规则集 → 路径」映射表 + 规则列表；空态 | 有数据 / 空数据 / 请求失败 |
| 分组 | `ResourcePage`：列 `name / type / regex / url / interval`；字段全 text（`interval` 为 number） | 同上 |
| 订阅 | 上半：令牌 `ResourcePage`（列 `token / name`，字段 `token(text) / name(text)`）；下半：选中令牌后显示完整订阅 URL + 复制按钮 + 「客户端使用说明」 | 未选令牌 / 已选令牌；复制成功提示 |
| 任务 | `UnderConstruction`，说明 scheduler 已实现、任务历史第一版不落库 | 一条挂载断言 |
| 设置 | 只读展示：支持的目标格式（`targets`）、规则集数量、历史条数；主题切换（浅色 / 深色 / 跟随系统） | 三种主题切换；`targets` 为空 |
| 日志 | 轮询 `/api/logs?since=lastId`，追加到列表；「暂停 / 继续」开关；「清屏」按钮；自动滚动到底部 | 首次拉取、增量拉取、暂停时不拉取、继续后恢复、清屏、请求失败 |

> **每页的 spec 至少一条要断言「失败路径」**（后端 500 或 fetch reject），否则 `AsyncSection` 的 error 分支与页面的 catch 都到不了 100%。

### 5.5 反重合硬规则（前序 plan §1「页面间跳转契约」的前端落地）

这三条是为了防止重写后再次出现「订阅页什么都干」的老问题：

1. **一个能力只有一个页面入口**。临时转换只在「转换」页；命名管线只在「管线」页；规则集内容只在「规则源」页。任何页面都不得内联「顺便也能转换一下」的功能。
2. **跨页只跳转、不复制功能**。例如「机场」页可以给一个「去订阅页生成订阅」的按钮，但按钮只做 `router.push`，不得在机场页内生成订阅。
3. **页面不直接读配置文件**。所有数据一律走 `/api/*`；前端不得假设配置文件结构。

---

## 6. Go 侧接线

> 本节所有行号是写作时的快照，施工前用 `grep -n` 复核一次。

### 6.1 embed 改造

现状：`internal/api/embed.go` 用 `//go:embed web` 把 `internal/api/web/` 整目录打进二进制，再 `fs.Sub(webEmbed, "web")`。

改造：

| 项 | 改法 | 理由 |
| --- | --- | --- |
| 指令 | `//go:embed all:web/dist` | `all:` 前缀让以 `_` 或 `.` 开头的文件也被包含（Vite 产物里可能有 `_plugin-vue_export-helper-*.js` 这类文件；不加 `all:` 会静默漏掉，表现为运行时 404） |
| 根目录 | `fs.Sub(webEmbed, "web/dist")` | 与 outDir 对齐 |
| 目录占位 | 在 `internal/api/web/dist/` 放一个 `.gitkeep`（或 `placeholder.txt`），并**在 .gitignore 里放行该目录下的 `.gitkeep`** | 否则干净 clone 后 `go build` 会因为 `all:web/dist` 找不到目录而失败 |

> **这是本节最容易翻车的点**：`go:embed` 在目录不存在时是**编译错误**，不是运行时错误。CI 必须先跑前端构建再跑 `go build`（见 §7.1），本地也要在 `make build` 里固定顺序。

### 6.2 SPA fallback 路由

现状（`internal/api/server.go` 的 `Routes()`）：页面路由是 5 条固定路径（`/` `/convert` `/rules` `/subscriptions` `/logs`），静态资源是 `/app.css` `/app.js` `/js/*`，末尾 `r.NotFound(s.handleNotFound)`。

改造为「静态资源 + SPA fallback」两段：

| 路由 | 处理 |
| --- | --- |
| `/assets/*` | 直接从 `webFS` 读文件；命中不到就 404（**不要 fallback 到 index.html**，否则 JS 请求会拿到 HTML，报 `Unexpected token '<'`） |
| `/favicon.ico` `/mdi.woff2` `/mdi.css` 等 Vite 产物根级文件 | 逐个从 `webFS` 读；不存在则 404 |
| 其它非 `/api` `/sub` `/subscribe` `/healthz` `/version` `/getruleset` `/inspect` `/debug/pprof` `/metrics` 的 GET | 返回 `index.html`（SPA fallback），状态 200 |
| `r.NotFound` | 只兜底非 GET 与未知 API 前缀，返回 404 JSON |

**验收点**：直接访问 `/airports`（浏览器刷新）必须拿到 `index.html` 而不是 404；`/assets/index-xxxx.js` 必须返回 `application/javascript`；`/assets/不存在的.js` 必须 404。

### 6.3 旧 UI 删除与测试改写

删除：`internal/api/web/` 下的 5 个 HTML、app.css、app.js、`js/` 全部（`js/subscription.js` 23.6 KB 是最大的一块）。

必须同步改写的测试（不改会让 CI 红）：

| 文件 | 要改的断言 | 改法 |
| --- | --- | --- |
| `internal/api/web_ui_test.go`（1077 行） | `TestWebUI_Pages`（约 :157）断言旧 5 页的 `<title>` 与 DOM 标记（如 `id="qcRun"`、`src="/js/dashboard.js"`）；约 :221-227 断言 `/app.css` `/app.js` `/js/*` 的 Content-Type | 页面断言改为「SPA fallback 返回 index.html 且含 `<div id="app">`」；资源断言改为针对 `/assets/` 下的构建产物 |
| `internal/api/web_extra_test.go` | 资源/页面的边界用例 | 按新路由表逐条核对 |
| `internal/api/subweb_compat_test.go` | 兼容性断言 | 逐条核对是否仍成立 |
| `internal/api/server_test.go` | 路由存在性 | 核对新增/删除的路由 |

**保留原则**：这个文件里除了「页面标题/DOM 标记/旧资源路径」之外的用例（UA 猜测、超时、错误体、pprof、metrics 等）都有价值，**逐条保留，只改与旧前端强耦合的那部分**。不要整个文件重写——那会丢掉既有防线。

### 6.4 开发态工作流

| 场景 | 命令 | 说明 |
| --- | --- | --- |
| 前端热更新 | `cd web && pnpm dev` | Vite 在 5173，`server.proxy` 把 API 转到 8080 |
| 后端 | `go run ./cmd/subforge ...` | 8080 提供 API |
| 一体化验证 | `make build` 然后跑二进制 | 走 embed 路径，验证的是真实交付形态 |

> 前端开发时**不要**依赖 `internal/api/web/dist` 里的旧产物，避免「改了源码没生效」的困惑。

---

## 7. CI 与 Makefile

### 7.1 `.github/workflows/ci.yml` 改造

现状：单一 Go 流水线，vendored 模式跑 `go build ./...` `go vet ./...` `go test ./...`，再加一个交叉编译矩阵（linux/amd64|arm64|arm、windows/amd64、darwin/amd64|arm64、freebsd/amd64、linux/mipsle|mips，`CGO_ENABLED=0`）。

改造要点：

| 要点 | 做法 | 理由 |
| --- | --- | --- |
| 新增 `web` job | `actions/setup-node@v4`（node 24）+ `pnpm/action-setup@v4` + `pnpm install --frozen-lockfile` → `pnpm run typecheck` → `pnpm run coverage` → `pnpm run build` | 前端门禁 A 在这里执行 |
| pnpm 缓存 | `actions/cache` 缓存 `~/.pnpm-store`，key 用 `pnpm-lock.yaml` 的 hash | 安装从 11 s 降到 2 s 级 |
| 覆盖率产物 | `web` job 上传 `web/coverage/` 为 artifact（失败时用于定位） | 门禁失败必须能立刻看到是哪个文件 |
| `test` job 依赖 `web` | `needs: web` + `actions/download-artifact` 取 `web/dist` 放到 `internal/api/web/dist` | **否则 `//go:embed all:web/dist` 编译失败**（见 §6.1） |
| 新增 `coverage` job | `needs: web` → 跑 `go test -coverprofile=coverage.out -covermode=atomic ./internal/...` → `go run ./internal/tools/coveragegate -profile coverage.out -min 80 -top 20` | 后端门禁 B |
| 交叉编译矩阵 | `needs: web` | 同上，embed 目录必须先存在 |
| vendored 模式 | 保持 `-mod=vendor` | 本仓库已 vendor，不要动 |

骨架（只列关键结构，实现时按现有文件风格补全）：

```yaml
jobs:
  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with: { version: 11 }
      - uses: actions/setup-node@v4
        with: { node-version: 24, cache: pnpm, cache-dependency-path: web/pnpm-lock.yaml }
      - run: pnpm install --frozen-lockfile
      - run: pnpm run typecheck
      - run: pnpm run coverage
      - run: pnpm run build
      - uses: actions/upload-artifact@v4
        with: { name: web-dist, path: web/dist }

  test:
    needs: web
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - uses: actions/download-artifact@v4
        with: { name: web-dist, path: internal/api/web/dist }
      - run: go build ./...
      - run: go vet ./...
      - run: go test ./...

  coverage:
    needs: web
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod }
      - uses: actions/download-artifact@v4
        with: { name: web-dist, path: internal/api/web/dist }
      - run: go test -coverprofile=coverage.out -covermode=atomic ./internal/...
      - run: go run ./internal/tools/coveragegate -profile coverage.out -min 80 -top 20
```

### 7.2 Makefile 目标

若仓库已有 Makefile 则追加以下目标；没有则新建。所有目标都能在干净 clone 后按顺序跑通。

| 目标 | 命令 | 作用 |
| --- | --- | --- |
| `web-install` | `cd web && pnpm install --frozen-lockfile` | 装前端依赖 |
| `web-dev` | `cd web && pnpm dev` | 开发服务器（5173） |
| `web-test` | `cd web && pnpm test` | 前端单测（不带门禁，便于快速迭代） |
| `web-coverage` | `cd web && pnpm run coverage` | **门禁 A** |
| `web-build` | `cd web && pnpm run build` | 产出 `internal/api/web/dist` |
| `test` | `go test ./...` | 后端单测 |
| `coverage` | `go test -coverprofile=coverage.out -covermode=atomic ./internal/...` | 生成 profile |
| `coverage-gate` | `coverage` + `go run ./internal/tools/coveragegate -profile coverage.out -min 80 -top 20` | **门禁 B** |
| `build` | `web-build` + `go build ./cmd/subforge` | 一体化二进制（验证 embed 路径） |
| `verify` | `web-coverage` + `web-build` + `test` + `coverage-gate` + `go vet ./...` | **提交前一把梭**，等价于 CI |

> **Windows**：PowerShell 下 `make` 不可用也无所谓——把 `verify` 的五条命令写进 `scripts/verify.cmd`，或直接用 Node `execFileSync` 依次调用。关键是**命令内容与 CI 完全一致**，不要各写一套。

### 7.3 dist 漂移校验（防「源码改了但没重新构建」）

问题：`web/dist` 不入库，CI 每次重新构建，所以 CI 不会发现「开发者本地改了 `src/` 但忘了跑 build 就提交」。这会表现为「CI 绿、本地二进制旧」。

做法：在 `verify` 末尾加一条**幂等校验**——

1. `web-build` 前记录 `web/dist` 的文件清单与哈希（若有）；
2. 构建后再取一次；
3. 若两次不一致且工作区里 `web/src` 有未提交改动，则提示「源码已改，dist 已重建，请重新验证二进制」。

第一版可以只做「构建后 dist 非空且含 `index.html` + 至少一个 `assets/*.js`」的存在性断言——比完美校验更便宜，且能挡住「dist 是空的但 CI 绿」这种最坏情况。

---

## 8. 实施顺序（每阶段独立可构建、可验证、可提交）

排序原则：**先让「前端工程 + 门禁」这条链路完整跑通，再逐页填内容**。这样任何阶段中断，仓库都处于可构建状态。

| 阶段 | 交付物 | 验证命令 | 提交信息建议 |
| --- | --- | --- | --- |
| **P1** 前端工程骨架 | `web/` 的 package.json / tsconfig / vite.config / vitest.config / index.html / .gitignore / tests/setup.ts；pnpm-lock.yaml | `pnpm install` + `pnpm run build`（此时 src 可为最小 App） | `chore(web): 引入 Vite+Vue3+Vuetify 工程骨架` |
| **P2** 共享层 + 单测 | `src/api/*` `src/format/*` `src/composables/*` `src/router/*` `src/plugins/*` `src/bootstrap.ts` `src/main.ts` 及各自 `*.spec.ts` | `pnpm run typecheck` + `pnpm run coverage`（**首次真正跑门禁 A**） | `feat(web): 共享层（api/format/composables/router）与单测` |
| **P3** 布局 + 共享组件 | `App.vue` `layout/*` `components/{PageHeader,AsyncSection,UnderConstruction,ResourcePage}.vue` + `tests/renderPage.ts` `tests/resourceFlow.ts` + 组件 spec | 同上 | `feat(web): 布局与共享组件（含通用 CRUD 页）` |
| **P4** 13 页（先占位） | `pages/*.vue` ×14，其中 5 页真实、9 页用 `UnderConstruction` 或简版 | `pnpm run typecheck`（路由 lazy import 不再报错）+ 覆盖率仍 100 | `feat(web): 13 页路由骨架` |
| **P5** 4 个 ResourcePage 页面 | 机场 / 自建节点 / 规则 / 分组（各自 spec 用 `exerciseCrudPage`） | `pnpm run coverage` | `feat(web): 机场/自建节点/规则/分组 CRUD 页` |
| **P6** 只读页 | 概览 / 管线 / 规则源 / 设置 | 同上 | `feat(web): 概览/管线/规则源/设置页` |
| **P7** 交互页 | 转换 / 订阅 / 日志 | 同上 | `feat(web): 转换/订阅/日志页` |
| **P8** Go 接线 | `internal/api/embed.go` `internal/api/server.go` 路由改造 + `internal/api/web/dist/.gitkeep` + .gitignore 放行 | `go build ./...`（需先 `web-build`） | `feat(api): 接入 Vite 产物（embed + SPA fallback）` |
| **P9** 删旧前端 + 改测试 | 删除 `internal/api/web/` 旧资源；改写 `web_ui_test.go` `web_extra_test.go` `subweb_compat_test.go` `server_test.go` | `go test ./internal/api/...` | `refactor(api): 移除旧手写前端并改写页面测试` |
| **P10** 后端补测（门禁 B） | `internal/store` ≥80% / `internal/subforge` ≥80% / `internal/scheduler` ≥85% / `internal/mrs` ≥85% / `internal/mcp` ≥85% | `go test -coverprofile=... ./internal/...` + coveragegate | `test: 后端覆盖率抬到 85%（门禁 B）` |
| **P11** CI + Makefile | `.github/workflows/ci.yml` 新增 web/coverage job、test 与交叉编译加 `needs: web`；Makefile 目标 | 本地跑 `verify` 全绿 | `ci: 前后端覆盖率门禁 + Vite 构建接入流水线` |
| **P12** 文档收口 | 更新 `README.md` 的构建说明；`docs/design/web-ui-rewrite-plan.md` §5/§6 加一行指向本文件 | 人工核对 | `docs: 更新构建与门禁说明` |

> P1–P7 是纯前端，P8–P9 是 Go 接线，P10 是后端补测，P11 是流水线。**P10 可以与前端的 P5–P7 并行**（互不碰文件），但提交必须分开，否则一次提交混两类改动、回滚粒度变粗。

---

## 9. 风险登记与回滚

| 风险 | 概率 | 影响 | 缓解 | 回滚 |
| --- | --- | --- | --- | --- |
| `go:embed all:web/dist` 在干净 clone 编译失败 | 高 | 阻塞 CI | 放 `.gitkeep` + .gitignore 放行；CI 强制 `needs: web` | 若暂时接不通，先把 embed 指回旧目录，P8 单独重做 |
| 前端 100% 门禁最后卡在 1-2 个分支 | 中 | 阻塞 P5–P7 | 按 §3.3 清单逐条预防；`coverage/index.html` 定位到行 | 不要降阈值。要么补测，要么重构掉该分支 |
| Vuetify 组件在 jsdom 下的告警/异常 | 中 | 测试不稳定 | `tests/setup.ts` 补齐 `ResizeObserver` / `matchMedia` 等；必要时用 `v-app` 包裹挂载 | — |
| `v-data-table` 的插槽在测试里不渲染 | 中 | ResourcePage 行内按钮测不到 | 用 `mount` + 真实 Vuetify 插件（不是 `shallow`）；断言用 `data-testid` | 若仍不行，把行内按钮改为独立列渲染而非插槽 |
| 后端 80% 在补测后因重构回落 | 中 | CI 红 | 目标定 85%（§4.4），留 5 点余量 | 补测即可，不降阈值 |
| 覆盖率被「为测而测」的仪式测试灌水 | 中 | 门禁失去意义 | 每个新增测试必须断言行为（返回值/状态/副作用），不允许只有 `expect(true).toBe(true)` | code review 拒绝 |
| 旧页面测试改写时丢防线 | 中 | 回归漏网 | §6.3 的「保留原则」：只改与旧前端强耦合的断言 | 从 git 历史取回被删用例 |
| Windows 下命令参数被拆 | 高 | 本地验证假失败 | 一律用 `cmd /c` 脚本或 Node `execFileSync` 传 argv | — |
| `web_search` 不可用导致无法查 Vuetify 文档 | 中 | 选型受限 | 已冻结到 3.13.7；需要新 API 时读本地 `node_modules/vuetify` 的类型定义与源码 | — |

---

## 10. 全局验收清单

提交前逐条核对，**每一条都要有可粘贴的输出证据**：

| # | 验收项 | 命令 / 证据 |
| --- | --- | --- |
| 1 | 前端类型检查 0 错误 | `cd web && pnpm run typecheck` 输出为空 |
| 2 | **门禁 A：前端四项覆盖率 100%** | `pnpm run coverage` 退出码 0，输出含 `All files 100` |
| 3 | 前端可构建 | `pnpm run build` 产出 `web/dist/index.html` + `web/dist/assets/*.js` |
| 4 | 后端构建与静态检查 | `go build ./...` `go vet ./...` 均 exit 0 |
| 5 | 后端单测全绿 | `go test ./...` 无 FAIL |
| 6 | **门禁 B：后端 `./internal/...` ≥80%（目标 85%）** | coveragegate 输出 `PASS` 且退出码 0 |
| 7 | SPA 路由可用 | 二进制启动后 `curl -i http://127.0.0.1:8080/airports` 返回 200 + `<div id="app">` |
| 8 | 静态资源 Content-Type 正确 | `curl -i .../assets/<hash>.js` 返回 `application/javascript` |
| 9 | 不存在的资源返回 404 而不是 HTML | `curl -i .../assets/nope.js` 返回 404 |
| 10 | 旧前端已彻底移除 | `ls internal/api/web/` 只剩 `dist/`；全仓 grep `/js/subscription.js` 无命中 |
| 11 | CI 三处接线 | ci.yml 中 `web` job 存在、`test` 与交叉编译均有 `needs: web`、`coverage` job 存在 |
| 12 | 一次性验收 | `make verify`（或 `scripts/verify.cmd`）全绿 |

---

## 附录 A：本计划写作时的已完成事项（**未提交**，按「只写 plan」要求保留在工作区）

| 项 | 状态 | 位置 | 处置建议 |
| --- | --- | --- | --- |
| 覆盖率门禁工具 | 已完成并独立复验（自身 98.2%） | `internal/tools/coveragegate/{main.go,main_test.go}` | **保留**，它是门禁 B 的必需件，P10 直接用 |
| 前端工程脚手架 | 配置 + 依赖安装 + 共享层 + 布局 + 共享组件 + 3 个页面已落盘，**从未跑过 vitest / typecheck** | `web/` | 二选一：①作为 P1–P4 的起点继续（推荐，安装已验证）；②删除后严格按 §8 重做 |
| 全仓覆盖率基线数字 | 已量出并记录在 §1.2 | 临时产物已删 | 数字已固化在本文档 |
| teammate 补测产物 | **已删除**（其中 mrs/scheduler 的产物语法错误不可编译） | — | 按 §4.3 重写 |

> 附录 A 存在的意义：**不要把「已写了一半的实现」伪装成「计划」**。这份文档是计划；工作区里另有一批未提交的实现痕迹，是否沿用由决策者定。

## 附录 B：需要人确认的开放点

| # | 开放点 | 默认取值 | 影响面 |
| --- | --- | --- | --- |
| 1 | ~~A/B 是否等于「前端 100% / 后端 80%」~~ **已确认（2026-10-18）：A 层 = 前端层 100%、B 层 = 后端层 80%** | 已定（§0.2） | — |
| 2 | `cmd/subforge` 是否纳入门禁 | 否（§1.3） | 纳入则需为 CLI 补 642 条语句中的约 510 条 |
| 3 | `pkg/subforge` 是否纳入门禁 | 否（§4.2） | 影响 < 0.3% |
| 4 | `web/dist` 是否入库 | 否（§2.4） | 入库则需接受哈希文件的 diff 噪声 |
| 5 | 是否升级到 Vuetify 4.x | 否（§2.2） | 需要先能访问其文档 |
| 6 | 脚本页 / 任务页是否在本轮实现 | 否，用占位（§5.1） | 二者后端已存在，属前序 plan §3.6 / §4 的范围 |
| 7 | 后端目标是否定 85% | 是（§4.4） | 影响 P10 工作量约 +10% |

---

## 附录 C：与前序 plan 的差异速查（给已读过 `web-ui-rewrite-plan.md` 的人）

| 主题 | 前序 plan | 本计划 |
| --- | --- | --- |
| 前端技术栈 | 手写 HTML/CSS/JS + `impeccable` 设计 skill | Vite + Vue 3 + Vuetify 3.13.7 + TypeScript |
| 前端测试 | 未规定 | Vitest + jsdom，四项覆盖率 100%（门禁 A） |
| 前端目录 | 14 个 html + 15 个 js + 4 个 partials | `web/src/` 下的 SFC + composable（§2.3） |
| 实施顺序 | P1–P15（含后端数据模型与 API 补全） | P1–P12（只含换栈 + 门禁；数据模型/API 补全仍按前序 plan 走） |
| 后端覆盖率 | 未规定 | `./internal/...` ≥80%，目标 85%（门禁 B） |
| 交付节奏 | 每阶段一提交 | 同，且每阶段都必须过 §10 的对应项 |
