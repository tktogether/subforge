# T3c：测试与门禁的真实约束（事实盘点）

> 任务书：`C:\Users\橙先生\AppData\Local\Temp\subforge-taskbooks\T3c-gates.md`
> 写范围：本文件（唯一）。
> 调查时间：2026-10-19，工作目录 `E:\Projects\subforge`。
> 本文件仅记录事实与结论，未修改任何源码。

## ① 文件清单（本报告引用的配置/源文件，绝对路径 + 字节数）

| 文件 | 字节数 |
|---|---|
| `E:\Projects\subforge\web\package.json` | 954 |
| `E:\Projects\subforge\web\vitest.config.ts` | 791 |
| `E:\Projects\subforge\web\vitest.setup.ts` | 1096 |
| `E:\Projects\subforge\web\tests\setup.ts` | 1383 |
| `E:\Projects\subforge\.github\workflows\ci.yml` | 5167 |
| `E:\Projects\subforge\scripts\verify.cmd` | 1851 |
| `E:\Projects\subforge\Makefile` | 2702 |

前端测试源文件：`E:\Projects\subforge\web\src\**\*.spec.ts`，共 35 个文件（PowerShell `Get-ChildItem -Recurse -Filter *.spec.ts -Path src` 实测，与 `web/vitest.config.ts:15` 的 `include: ['src/**/*.spec.ts']` 一致）。

---

## ② 命令 + 原始输出（原文粘贴）

### 2.1 配置原文引用

#### A. `E:\Projects\subforge\web\package.json` — scripts（原文，L10-L18）

```json
"scripts": {
  "dev": "vite",
  "build": "vue-tsc --noEmit && vite build && node scripts/ensure-dist-placeholder.mjs",
  "build:only": "vite build && node scripts/ensure-dist-placeholder.mjs",
  "preview": "vite preview",
  "typecheck": "vue-tsc --noEmit",
  "test": "vitest run",
  "test:watch": "vitest",
  "coverage": "vitest run --coverage"
},
```

#### B. `E:\Projects\subforge\web\vitest.config.ts`（原文，全文 34 行）

```ts
import { fileURLToPath } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts', './vitest.setup.ts'],
    include: ['src/**/*.spec.ts'],
    server: {
      deps: {
        inline: ['vuetify'],
      },
    },
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary'],
      include: ['src/**/*.ts', 'src/**/*.vue'],
      exclude: ['src/**/*.spec.ts', 'src/env.d.ts'],
      thresholds: {
        lines: 100,
        functions: 100,
        branches: 80,
        statements: 100,
      },
    },
  },
})
```

#### C. `E:\Projects\subforge\.github\workflows\ci.yml` — web job（原文，L17-L41）

```yaml
jobs:
  web:
    name: web
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 11
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - name: Install
        run: pnpm install --frozen-lockfile
      - name: Typecheck
        run: pnpm run typecheck
      - name: Test + coverage gate
        run: pnpm run coverage
      - name: Build
        run: pnpm run build
```

#### D. `E:\Projects\subforge\scripts\verify.cmd` — 第 9 步前端覆盖率门禁（原文，L49-L55）

```bat
echo [9/10] frontend coverage gate (vitest run --coverage; thresholds in web/vitest.config.ts)
if defined HAVE_PNPM (
  pnpm -C web run coverage
) else (
  cmd /c "cd web && node_modules\.bin\vitest.cmd run --coverage"
)
if errorlevel 1 exit /b 1
```

#### E. `E:\Projects\subforge\Makefile` — verify 目标（原文，L90-L94）

```makefile
# --- Verify (one-shot local green) ---
# 本机一把过：go build/vet/test + 门禁A + 门禁B快层 + 前端 typecheck/test/build。
verify: build vet test coverage-gate web-typecheck web-coverage web-build
	@echo "verify: all green"
```

#### F. `E:\Projects\subforge\web\vitest.setup.ts` — 全局桩（原文，L14-L17，L44）

```ts
config.global.stubs = {
  ...config.global.stubs,
  RouterLink: defineComponent({ ... }),
}
```

#### G. `E:\Projects\subforge\web\tests\setup.ts` — jsdom 桩（原文，L1-L2）

```ts
// jsdom 缺失的浏览器 API 桩：Vuetify 的 useDisplay / VResizeObserver 需要它们。
// 本文件位于 tests/，不在覆盖率统计范围内（vitest.config.ts 的 coverage.include 只含 src/）。
```

---

### 2.2 现有门禁数字（实测 / 配置读取）

| 指标 | 数值 | 来源 |
|---|---|---|
| 测试文件数 | 35 | 实测（PowerShell `Get-ChildItem -Recurse -Filter *.spec.ts -Path src`） |
| 测试条数 | 594 | 实测（vitest run，末行 `Tests 594 passed (594)`） |
| 覆盖率阈值 lines | 100 | 配置读取（`web/vitest.config.ts:27`） |
| 覆盖率阈值 functions | 100 | 配置读取（`web/vitest.config.ts:28`） |
| 覆盖率阈值 branches | 80 | 配置读取（`web/vitest.config.ts:29`） |
| 覆盖率阈值 statements | 100 | 配置读取（`web/vitest.config.ts:30`） |
| 实测覆盖率（All files） | Stmts 100 / Branch 97.73 / Funcs 100 / Lines 100 | 实测（vitest run --coverage，exitCode 0） |
| Go 覆盖率门禁 | `-min 80 -min-func 100` | 配置读取（`.github/workflows/ci.yml:155`、`scripts/verify.cmd:28`） |
| Go 条件覆盖门禁 | `-min 80`（profile 快层；gobco 仅 nightly） | 配置读取（`ci.yml:157,183`、`Makefile:87-88`） |
| 环境 | jsdom；setupFiles `./tests/setup.ts`, `./vitest.setup.ts` | 配置读取（`web/vitest.config.ts:13-14`） |
| 覆盖率 include | `src/**/*.ts`, `src/**/*.vue`（exclude spec.ts, env.d.ts） | 配置读取（`web/vitest.config.ts:24-25`） |

### 2.3 测试断言模式 grep（web/src/**/*.spec.ts）

命令形态（DSH grep，等价于 `grep -rn <pattern> web/src --include=*.spec.ts`）：

| 模式 | 命中数 | 说明 |
|---|---|---|
| `class=` | 47 | 多数为测试内 stub 模板字面量；直接断言 CSS 类的见 2.4 |
| `data-test` | 326 | `find('[data-testid="..."]')` / `findAll`，项目主定位手段 |
| `getByText` / `getByRole` | 0 | 项目未使用 testing-library，全部用 @vue/test-utils |
| `\.find(` | 312 | DOM/组件查询入口，含 `.find('[data-testid=...]')`、`.findAll('button').find(...)` |
| `\.html(` | 2 | `LogsPage.spec.ts:121`、`SubscriptionsPage.spec.ts:226` |
| `toMatchSnapshot` | 0 | 无快照测试 → 视觉重构无快照回退风险 |

### 2.4 vitest run 基线（末 15 行原文）

命令：`cd web && node_modules\.bin\vitest.cmd run`（本机 pnpm 不在 PATH，采用 `scripts/verify.cmd:53` 的文档化 fallback；exitCode 0，耗时 18.05s）。

```
   ✓ AirportsPage (51)
     ✓ onMounted 加载 listAirports 渲染表格与 testids 598ms
     ✓ 加载失败显示错误，点击重试重新加载 385ms
     ✓ traffic-err 红色渲染 492ms
     ✓ 新增机场对话框提交 createAirport 694ms
     ✓ 编辑机场对话框提交 updateAirport 414ms
     ✓ 删除确认对话框确认后调用 deleteAirport 326ms
     ✓ 刷新单个机场流量 356ms
     ✓ checkOne 调用 checkAirports([id]) 369ms
 ✓ src/composables/useNotify.spec.ts (4 tests) 4ms

 Test Files  35 passed (35)
      Tests  594 passed (594)
   Start at  12:56:30
   Duration  18.05s (tests 39%, environment 39%, transform 11%, import 8%, setup 3%)
```

### 2.5 vitest coverage 末段（原文节选）

命令：`cd web && node_modules\.bin\vitest.cmd run --coverage`，exitCode 0。

```
 % Coverage report from v8
-------------------|---------|----------|---------|---------|-------------------
File               | % Stmts | % Branch | % Funcs | % Lines | Uncovered Line #s 
-------------------|---------|----------|---------|---------|-------------------
All files          |     100 |    97.73 |     100 |     100 |                   
...
  ScriptsPage.vue  |     100 |    97.77 |     100 |     100 | 191,250           
  ...tionsPage.vue |     100 |    97.18 |     100 |     100 | 143-146           
-------------------|---------|----------|---------|---------|-------------------
```

---

## ③ 关键结论（每条带出处）

1. **前端门禁组合** = typecheck（`vue-tsc --noEmit`）+ vitest 35 文件 / 594 条 + v8 覆盖率阈值（100/100/80/100）+ build（`web/package.json:12,15-18`；CI `ci.yml:36-41`；verify.cmd 步骤 8-10）。
2. **门禁真实状态**：vitest 35/35 文件、594/594 条全绿（实测，exitCode 0）；coverage All files 100/97.73/100/100 对阈值 100/100/80/100，全达标（实测，exitCode 0）。
3. **纯视觉重构（只改 CSS/主题/布局类名）的主要风险是"断言直接耦合到 CSS 类名/结构选择器"的少数测试**：
   - `E:\Projects\subforge\web\src\pages\SettingsPage.spec.ts:117,127`：`expect(w.find('.text-error').exists())` 依赖 Vuetify 颜色工具类 `.text-error`；
   - `E:\Projects\subforge\web\src\pages\RuleSourcesPage.spec.ts:94`：`expect(w.find('[data-testid="init-manifest"]').classes().join(' ')).toContain('primary')` 断言按钮带 `.primary` 类；
   - `E:\Projects\subforge\web\src\layout\AppShell.spec.ts:66`：`expect(w.find('.apptopbar-stub').exists())` 依赖 stub 的类名（测试内定义）；
   - `E:\Projects\subforge\web\src\pages\OverviewPage.spec.ts:175`：`w.find('td.v-data-table__td--expanded-row button')` 依赖 Vuetify BEM 类；
   - `E:\Projects\subforge\web\src\pages\GroupsPage.spec.ts:141`：`w.find('[data-testid="group-recipes"] .v-expansion-panel-title')` 依赖 Vuetify 展开面板类与容器结构。
4. **相对安全的契约**：326 处 `data-testid` 定位（grep `data-test`）与文本断言（如 `AirportsPage.spec.ts:119` `.text()).toBe('从未采集')`）不随纯 CSS 变化而失效；`.html()` 仅 2 处（`LogsPage.spec.ts:121`、`SubscriptionsPage.spec.ts:226`），断言的是文本内容而非类名。
5. **无快照测试**：`toMatchSnapshot` 命中 0（grep 全库 spec），故无"更新快照"类门禁；`getByText`/`getByRole` 命中 0，项目统一用 @vue/test-utils 的 `find`/`findAll` + `data-testid`。
6. **覆盖率门禁对纯 CSS 重构不构成阻塞**：include 为 `src/**/*.ts` 与 `src/**/*.vue`（`vitest.config.ts:24`），SFC 的 `<style>` 不计入可执行行，类名改名不改变执行行数；现有未覆盖分支仅 ScriptsPage.vue:191,250 与 SubscriptionsPage.vue:143-146（coverage 输出），重构不应触及。
7. **测试内 stub 模板**（如 `SubscriptionsPage.spec.ts:80`、`ResourcePage.spec.ts:419`、`AppShell.spec.ts:12,21,30`）含 `class="v-dialog-stub"` 等字面量，仅在 stub 契约变化时失效，纯应用 CSS 重构不涉及。

---

## ④ 未决问题

- 本机 PATH 无 pnpm（`Get-Command pnpm` 无输出），因此基线采用 `node_modules\.bin\vitest.cmd` fallback（与 `scripts/verify.cmd:50-54` 一致）；CI 环境使用 pnpm 11，未在本机复现 pnpm 路径。未跑 `pnpm test` 原生命令。
- 未运行 Go 侧门禁（`go build/vet/test`、coveragegate、condgate），本任务范围仅前端测试与门禁；Go 侧数字来自配置读取。
- 未查到 Go 覆盖率实际百分比（需跑 `go test -coverprofile`），本报告不编造。
- 未决：若视觉重构涉及 Vuetify 主题 token（如颜色变量），`.text-error`/`.primary` 等 Vuetify 颜色工具类是否仍生成，需在重构 PR 中单独验证；本报告不确认。
