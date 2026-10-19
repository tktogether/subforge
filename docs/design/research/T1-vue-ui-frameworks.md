# T1 · Vue 3 UI 框架美学横评（事实底表）

> 卡片 task-1 ｜ Owner `free-scout-frameworks` ｜ 数据采集日 **2026-10-19**（Asia/Shanghai）
> 写范围：本文件 `docs/design/research/T1-vue-ui-frameworks.md`（本轮只新建，未改动任何既有文件）
> 定位：只做**事实收集与结构化**，不下最终选型结论（选型结论由 Lead 在 `docs/design/frontend-visual-upgrade-plan.md` 第 4 节下）
> 采集方式：DSH `run_code` 内 Node `fetch`（浏览器 UA、follow redirect）；GitHub 数字取自 `ungh.cc` 代理与 `github.com` 页面 HTML 的 `stargazerCount` 字段（原因见 §6）

---

## 0. 一页摘要

- 现状是 **Vuetify 3.13.7**，而本仓视觉入口只有 4 个颜色：`web/src/plugins/vuetify.ts:15-20`。Vuetify 会把你写的主题与 Material 默认**深合并**（`web/node_modules/vuetify/lib/composables/theme.js:107-110`），所以**另外 10 个语义色 + 全部 16 个 variables 仍是框架默认值**。"丑"的主因在令牌层缺位，不在组件层。
- 12 个候选里，**Vue 3 原生 + 官方暗色 + 可覆盖令牌 + 维护活跃**四项全中的有：Vuetify 3/4、shadcn-vue（+Reka UI）、Naive UI、PrimeVue、Element Plus、DaisyUI+Tailwind。
- **停维护风险最硬的两家**：Ant Design Vue（npm 与 GitHub Releases 最新都停在 `4.2.6 @2024-11-11`，近 12 个月 **0** 次发布）、Arco Design Vue（最近发布 `2.58.0 @2026-04-16`、最近提交 `2026-06-24`，近 12 月仅 2 次发布）。
- **原生 OKLCH 的两家**：DaisyUI（实测 `--color-primary:oklch(45% .24 277.023)`）与 Tailwind CSS v4（实测 `--color-red-500: oklch(63.7% 0.237 25.331)`）——直接命中 `.impeccable.md:24` 的"所有颜色 OKLCH"。
- **无障碍文档实测**：Element Plus、DaisyUI、Flowbite Vue 的 a11y 文档 URL 全部 **404**（真死链）；PrimeVue 与 Reka UI 的 a11y 文档位于**非 SPA 站点**且 200（可信）；Vuetify / Ant Design Vue / Arco 的 a11y URL 返回 200，但站点是 **SPA catch-all**，状态码不能证明页面存在（§3.2）。
- 迁移成本：本仓 89 个 `<v-btn>`、41 个 `<v-card>`、4 个 `<v-data-table>`（口径见 §0.1）。除"留在 Vuetify 改主题"外，其余候选均为**高**成本。

### 0.1 本仓组件用量复核（与计划文档口径对齐）

| 口径 | 命令（DSH grep 工具，path=web/src，include=*.vue） | 命中行数 | 计划文档值 |
|---|---|---|---|
| `<v-btn>` 精确开标签 | 正则 `<v-btn[\s>]` | **89** | 93 |
| `<v-btn` 前缀（多含 `<v-btn-toggle>`） | 正则 `<v-btn` | **93** | — |
| `<v-card>` 精确开标签 | 正则 `<v-card[\s>]` | **41** | 41 |
| `<v-card` 前缀（含 `-text/-title/-actions`） | 正则 `<v-card` | 101 | — |
| `<v-data-table>` | 正则 `<v-data-table` | **4** | 4 |
| `<v-table>` | 正则 `<v-table[\s>]` | 4 | 4 |

结论：计划文档 `frontend-visual-upgrade-plan.md:14` 的「93 个 `<v-btn>`"」用的是**前缀口径**（把 `<v-btn-toggle>` 一并计入）；精确开标签为 89。按行内出现次数求和复核后仍是 89 / 41，即不存在"一行多个组件"的漏计。此差异不影响任何结论。

---

## 1. 候选横评总表（15 列 × 12 行）

> 列名与任务书一致。Stars 一律为 **2026-10-19 实测值**。
> 体积 = bundlephobia `https://bundlephobia.com/api/size?package=<pkg>@<ver>` 的 `size` / `gzip`（字节），**barrel 入口值**，非实际产物体积。
> 「默认观感打分」为 1–5，评的是**框架出厂默认外观**（不含本仓现状配置）。

| 名称 | 官网 URL | GitHub URL | Stars（2026-10-19） | 最近发布版本 | 最近提交时间 | 是否 Vue 3 原生 | 是否官方暗色主题 | 设计令牌覆盖机制（具体名） | 默认观感打分（1–5 + 理由） | 打包体积量级（min / gzip） | 无障碍 / 键盘 | 迁移成本量级 | 许可证 | 主要风险 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **Vuetify 3**（现状） | https://vuetifyjs.com/ | https://github.com/vuetifyjs/vuetify | 41032（forks 7111） | 3.13.7 @ 2026-10-14（`v3-stable`；主线 `latest` 已是 4.2.6 @ 同日） | 2026-10-19T00:02:12Z（commits.atom 2026-10-14T20:39:37Z） | 是（Vue 3 专用） | 是（内置 light / dark 两套） | `theme.themes[x].colors`（14 键）与 `.variables`（16 键）→ CSS 变量 `--v-theme-*` / `--v-border-opacity` 等；SASS `_settings.scss`；全局 `defaults` | **3** — Material 语言本身不丑，但表格 52px 行高 + 每行 1px 分隔线（`VTable.sass:30`）读起来是 admin 模板感 | 51000 / 19595（barrel，按需 tree-shake；本仓已全量注册组件） | 官方 a11y 文档 URL 200（SPA 不可证）；包内 **52 个组件 `.js` 含 `aria-`，共 137 处**（本地实grep） | 低（0，已在用；升 v4 另算中） | MIT（`node_modules/vuetify/LICENSE.md:1`） | v3 已是旧线，主线切到 v4；不装 sass 则圆角/字体/排版层摸不到 |
| **shadcn-vue**（+Tailwind v4 +Reka UI） | https://www.shadcn-vue.com/ | https://github.com/unovue/shadcn-vue | 10703（forks 685） | 2.8.2 @ 2026-08-08 | 2026-10-15T15:02:16Z | 是（Vue 3 + TS，源码复制进仓库） | 是（`.dark` class + CSS 变量；`/docs/dark-mode` 200 且站点非 SPA） | CSS 变量 `--background`/`--foreground`/`--primary`… + Tailwind v4 `@theme inline` | **5** — 出厂即 neutral 低对比 + OKLCH + 极浅边框，没有 Material 阴影墙；但只给零件不给外壳 | 非运行时库：= 实际复制的组件 + Reka UI primitive（reka-ui 832560 / 188680） | 继承 Reka UI：https://reka-ui.com/docs/overview/accessibility 200（非 SPA 可信） | 高（无 `v-data-table` 等价物需自建；89 按钮 / 41 卡片全量改写） | MIT | 源码进仓 → 上游更新需手动合并；Tailwind 与 Vuetify 双样式引擎并存会互相打架 |
| **Naive UI** | https://www.naiveui.com/（**403**，Cloudflare） | https://github.com/tusen-ai/naive-ui | 18573（forks 1861） | 2.45.3 @ 2026-08-27 | 2026-10-05T15:39:49Z | 是（纯 Vue 3，无 Vue 2 包袱） | 是（导出 `darkTheme` + `NConfigProvider`） | `n-config-provider :theme-overrides`（JS 对象）+ `useThemeVars()` + 组件级 CSS 变量 `--n-color`/`--n-text-color`/`--n-font-weight`（包内 `es/button/src/Button.mjs` 实测） | **4** — 出厂即"素"，无 Material 阴影，边框浅；但偏平、缺少层级对比 | 2199719 / 521478（**全部候选最重**，deps 18） | 官网 403 无法验证；源码 `Button.mjs` 含 `role:`/`aria-hidden`/`tabindex`/`keydown`/`keyup` | 高（组件 API 与 Vuetify 完全不同，`n-data-table` 列定义/插槽需重写） | MIT | 发布节奏慢（2.44.1 @ 2026-03-08 → 2.45.0 @ 2026-08-16，约 5 个月空窗）；体积最大；无 SASS 令牌导出体系 |
| **PrimeVue**（任务书写 4，实测主线已 5） | https://primevue.org/（302 → https://primevue.dev/） | https://github.com/primefaces/primevue | 14446（forks 1784） | 5.0.2 @ 2026-09-29（`v4-stable` = 4.5.5 @ 2026-04-08） | 2026-09-24T13:07:08Z | 是 | 是（`darkModeSelector`） | styled 模式 `definePreset` 设计令牌（primitive / semantic 双层），主题预设 Aura / Lara / Material / Nora | **3** — Aura 预设现代，但仍是典型"组件库脸"；Lara/Material 预设更像 Material | 2081403 / 444405（deps 8） | 官方 a11y 指南 https://primevue.dev/guides/accessibility/ **200（非 SPA，可信）** | 高 | MIT（npm 字段写 `SEE LICENSE IN LICENSE.md`，仓库 LICENSE.md 为 MIT） | 大版本更迭快（4 → 5 在 2026-07）；主题系统 5 与 4 不兼容；体积大 |
| **Element Plus** | https://element-plus.org/ | https://github.com/element-plus/element-plus | 27812（forks 19804） | 2.14.7 @ 2026-09-30 | 2026-10-19T02:25:04Z | 是 | 是（`html.dark` + CSS 变量；`/guide/dark-mode.html` 200 且站点非 SPA） | CSS 变量 `--el-color-primary:#409eff`、`--el-border-radius-base`、`--el-font-size-base`、`--el-bg-color`、`--el-text-color-regular`（`dist/index.css` 实测，均为十六进制非 OKLCH）+ SCSS 变量 | **3** — 出厂蓝 `#409eff` + 浅灰边框 + 卡片阴影，是 `.impeccable.md:19` 点名的 "Bootstrap admin template" 观感最接近的一家 | 1066110 / 298422（deps 15） | **无官方 a11y 文档**：`/guide/a11y.html` 与 `/guide/accessibility.html` 实测均 **404** | 高 | MIT | 缺少无障碍说明；出厂观感与品牌反例最贴近 |
| **Arco Design Vue** | https://arco.design/vue/en-US/docs/start | https://github.com/arco-design/arco-design-vue | 3111（forks 603） | 2.58.0 @ 2026-04-16 | 2026-06-24T03:21:21Z | 是 | 是（`body[arco-theme='dark']`，`dist/arco.css` 实测） | CSS 变量 `--primary-6: var(--arcoblue-6)`、`--border-radius-small: 2px`、`--color-bg-2`、`--color-text-1`（`dist/arco.css` 实测）+ Less 变量 | **3** — 紧凑克制，但 2px 圆角 + 密灰阶偏"内部系统"，缺层次 | 882030 / 220279（deps 9，12 家中等偏轻） | a11y URL 200 但站点 SPA catch-all（§3.2），不可证 | 高 | MIT | **近 12 月仅 2 次发布、最近提交 2026-06-24（≥3 个月无提交）**；stars 相对低 |
| **Ant Design Vue 4** | https://antdv.com/ | https://github.com/vueComponent/ant-design-vue | 21684（forks 4183） | 4.2.6 @ **2024-11-11**（npm 与 GitHub Releases 双停） | 2026-07-24T08:29:13Z（commits.atom） | 是 | 是（ConfigProvider `theme.darkAlgorithm`） | ConfigProvider `theme.token`（seed / map token 双层，继承 antd v5 体系） | **4** — Ant Design 语言成熟克制，密度合理；但 14px 基准 + 大量 1px 边框偏"后台" | 1451159 / 419641（deps 22） | a11y URL 200 但站点 SPA catch-all（§3.2），不可证 | 高 | MIT | **快 2 年没有新版本（最新 4.2.6 @ 2024-11-11）**；dist-tags 的 `next=4.0.0-rc.6` 与 `alpha` 是陈旧残留 → 停维护风险最高 |
| **Reka UI**（原 Radix Vue） | https://reka-ui.com/ | https://github.com/unovue/reka-ui | 6884（forks 566） | 2.11.0 @ 2026-10-05 | 2026-10-19T03:24:40Z | 是 | **否**（无样式，需自己写） | **无自带令牌**；输出 `data-*` 状态属性 + 组件级 CSS 变量供你挂样式，令牌完全由你定义 | **3** — 无默认外观 = 零"丑"也零"美"，观感 100% 取决于你的 CSS | 832560 / 188680（deps 9，按需） | 官方 a11y 总览 https://reka-ui.com/docs/overview/accessibility **200（非 SPA 可信）** | 极高（连按钮都要自己写） | MIT | 2025 年由 `radix-vue` 改名 `reka-ui`，旧包 1.9.17 冻结在 2025-02-28（@unovue/radix-vue 已 301 到 reka-ui）；无样式 = 全部 UI 由你维护 |
| **DaisyUI 5 + Tailwind v4** | https://daisyui.com/ | https://github.com/saadeghi/daisyui | 42623（forks 1710） | 5.7.50 @ 2026-10-13 | 2026-10-13T11:44:04Z | 否（纯 CSS 类，与框架无关，Vue 里当 class 用） | 是（`[data-theme=light]`/`[data-theme=dark]` 实测；官方 themes 文档 200） | CSS 变量 `--color-primary:oklch(45% .24 277.023)`、`--radius-box:.5rem`、`--radius-field`、`--size-field`（`daisyui.css` 实测）→ **原生 OKLCH** | **3** — 默认主题圆润且 primary 是高饱和紫蓝（oklch 45% .24 277），正好命中 `.impeccable.md:19` 的"紫蓝 SaaS"反例；换 neutral 主题后可控 | 306719 / 50411（deps 0；另加 tailwindcss 284070 / 70742） | **无 a11y 文档**：`/docs/accessibility/` 实测 **404** | 极高（Vuetify 组件全改写为原生 HTML + class，表格完全自建） | MIT | 默认配色与品牌反例冲突；无 Vue 组件层（无 props / 事件 / 类型） |
| **Flowbite Vue** | https://flowbite-vue.com/ | https://github.com/themesberg/flowbite-vue | 954（forks 138） | 0.4.0 @ 2026-07-27 | 2026-07-28T17:10:03Z | 是 | 是（Tailwind `dark:` 变体） | **无令牌层**；纯 Tailwind 工具类 + `dark:` 变体，令牌靠 Tailwind `@theme` | **3** — Tailwind 风格干净，但模板感强、层级靠阴影与圆角堆 | 271837 / 70017（deps 12） | **无 a11y 页面**：`/accessibility` 实测 **404** | 极高 | MIT | **仍是 0.x**、近 12 月仅 3 版、stars 954（全部候选最低）→ 生态与稳定性风险最高 |
| **Headless UI Vue** | https://headlessui.com/（Vue 文档仅 https://headlessui.com/v1/vue） | https://github.com/tailwindlabs/headlessui | 28782（forks 1212） | `@headlessui/vue` 1.7.23 @ 2024-09-09 | 2026-04-13T16:12:23Z（仓库 React 主线） | 是 | **否**（无样式，需自己写） | **无令牌**；只输出行为与 ARIA 属性，样式完全归你 | **3** — 无样式，观感取决于你的 CSS | 123066 / 32296（deps 1，最轻） | 产品核心卖点（Tailwind Labs 维护 WAI-ARIA 行为）；Vue 版文档仅 v1（`/vue/menu` 实测 **404**） | 极高 | MIT | **Vue 包停更在 2024-09**；Vue 文档停留在 v1；近 12 月 release 全是 React 线 → Vue 侧事实停维护 |
| **Tailwind CSS v4**（基础设施，非组件库） | https://tailwindcss.com/ | https://github.com/tailwindlabs/tailwindcss | 97914（forks 8776） | 4.3.3 @ 2026-07-16 | 2026-10-16T11:28:05Z | 不适用（纯 CSS 工具链，可用于任何 Vue 3 项目） | 是（`dark:` 变体 + `prefers-color-scheme`；`/docs/dark-mode` 200 非 SPA 可信） | `@theme { --color-*: oklch(...) }`；`theme.css` 实测 `--color-red-500: oklch(63.7% 0.237 25.331)`、`--font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto…`、`--spacing`、`--radius-*`、`--text-*` | 不适用（本身不提供外观；3 = 中性） | 284070 / 70742（deps 0；v4 CSS-first，JIT 后实际更小） | 不适用（非组件库，不提供键盘行为） | 中（可与 Vuetify 并存，但样式优先级与重置需调解） | MIT | 与 Vuetify 的 SASS/组件 CSS 叠加会产生两套间距与 box 语言 |


---

## 2.「丑」的归因对照

针对本仓现状（Vuetify 3 + 只配了 4 个颜色，见 `web/src/plugins/vuetify.ts:12-22`）逐条回答。

### 2.1 只改主题令牌：Vuetify 3 的可配置项清单与视觉上限

**关键前提（源码事实）**：Vuetify 把你写的主题与框架默认**深合并**，不是替换。
`web/node_modules/vuetify/lib/composables/theme.js:107-110`：

`@
const defaultTheme = theme.dark || key === 'dark' ? defaults.themes?.dark : defaults.themes?.light
themes[key] = mergeDeep(defaultTheme, theme)
`@

所以本仓只写 4 个颜色 ⇒ 剩下 10 个语义色 + **全部 16 个 variables** 都还是 Material 默认（键表见 `theme.js:20-35` 与 `theme.js:36-53`）。

| 层 | 机制（具体名） | 能改什么 | 证据 |
|---|---|---|---|
| 运行时主题 · colors | `createVuetify({ theme: { themes: { x: { colors: {...} } } } })` | 14 个语义色键（`background / surface / surface-bright / surface-light / surface-variant / on-surface-variant / primary / primary-darken-1 / secondary / secondary-darken-1 / error / info / success / warning`）→ CSS 变量 `--v-theme-<key>: r,g,b` | `theme.js:20-35`（键表）、`theme.js:119-136`（CSS 变量生成） |
| 运行时主题 · variables | 同上 `.variables` | **16 个键**：`border-color / border-opacity / high-emphasis-opacity / medium-emphasis-opacity / disabled-opacity / idle-opacity / hover-opacity / focus-opacity / selected-opacity / activated-opacity / pressed-opacity / dragged-opacity / theme-kbd / theme-on-kbd / theme-code / theme-on-code` → CSS 变量 `--v-<key>` | `theme.js:36-53`、`theme.js:119-136` |
| 色阶生成 | `theme.variations = { colors: [], lighten: n, darken: n }` | 默认 `colors: []` 即**不生成任何色阶**；开启后按 lighten/darken 批量生成 | `theme.js:12-16` |
| 明暗双主题 | `theme.defaultTheme` + `themes: { dark: { dark: true, … } }` | 是否走 dark 默认由 `theme.dark || key === 'dark'` 判定 | `theme.js:107-110` |
| 全局默认 props | `createVuetify({ defaults: { global: {...}, VBtn: {...}, VCard: {...} } })` | **"不改组件只改外观"的主力**：可把 VCard 默认压成 `variant: 'outlined', elevation: 0`、VBtn 压成 `variant: 'flat'` | `lib/composables/defaults.js:61`、`:71` |
| 组件变体 / 尺寸 / 密度 | props `variant`（elevated / flat / tonal / outlined / text / plain）、`density`（default / comfortable / compact）、`size` | 单组件粒度 | 例：`lib/components/VCard/VCard.js:71-73` 默认 `variant: 'elevated'` |
| **CSS 变量直改**（不必重编译 SASS） | 覆写 `--v-table-row-height` / `--v-table-header-height` | 表格行高、表头高 | 读取处 `VTable.sass:110`（`height: var(--v-table-row-height)`）与 `:113`；赋值处 `VTable.sass:119-122` 的 density mixin（基准 56 / 52 + 0 / -2 / -4，见 `VTable/_variables.scss:9`、`:10`、`:14`） |
| **SASS 编译期变量**（圆角 / 字体 / 排版 / 阴影只能走这层） | `_settings.scss` + `css.preprocessorOptions.scss.additionalData` | `$border-radius-root`、`$body-font-family`、整个 `$typography` map、`$table-row-height` / `$table-header-height` / `$table-border` / `$table-column-padding`、`$card-elevation` | 入口 `node_modules/vuetify/_settings.scss:1-3`（`@forward './lib/styles/settings'; @forward './dist/component-variables'; @forward './dist/component-variables-labs';`）；变量 `lib/styles/settings/_variables.scss:21`、`:25`、`:217-326`；`lib/components/VTable/_variables.scss:10`、`:14`、`:17`、`:20`；`lib/components/VCard/_variables.scss:15` |
| 现状：CSS 变量层几乎是空的 | 全局只挂了 2 个变量 | `--v-theme-overlay-multiplier: 1`、`--v-scrollbar-offset: 0px` | `lib/styles/elements/_global.sass:18-20` |
| 现状：**没有字体 CSS 变量** | 字体被直接编译进 `html`：`font-family: settings.$body-font-family` | ⇒ 换字体**不能**靠改 CSS 变量 | `lib/styles/elements/_global.sass:6` |
| 现状：本仓 SASS 层未开启 | `web/vite.config.ts:1-34` 全文**没有** `css.preprocessorOptions`，依赖里也没有 sass | ⇒ 圆角 / 字体 / 排版 / 阴影目前**完全不可配** | `web/vite.config.ts:1-34` |

**官方文档 URL（状态码与 SPA 注意见 §3）**
主题 https://vuetifyjs.com/en/features/theme/ ｜ SASS 变量 https://vuetifyjs.com/en/features/sass-variables/ ｜ 全局默认 https://vuetifyjs.com/en/features/global-configuration/ ｜ 数据表 https://vuetifyjs.com/en/components/data-tables/basics/ ｜ 无障碍 https://vuetifyjs.com/en/features/accessibility/ ｜ 发行说明 https://vuetifyjs.com/en/getting-started/release-notes/

### 2.2 「可配置修掉」清单（逐条给证据）

| # | 现象 | 现状证据 | 修法 | 层级 |
|---|---|---|---|---|
| 1 | 卡片阴影墙 | `VCard.js:71-73` 默认 `variant: 'elevated'`；`$card-elevation: 1`、`$card-hover-elevation: 8`（`VCard/_variables.scss:15`、`:18`） | `defaults: { VCard: { variant: 'outlined' } }` 或 `elevation: 0` | 运行时 token |
| 2 | 按钮默认带阴影 | `VBtn.js:74-76` 默认 `variant: 'elevated'` | `defaults: { VBtn: { variant: 'flat' } }` | 运行时 token |
| 3 | 表格"全网格"1px 线 | 每行/每表头 `border-bottom: $table-border`（`VTable.sass:23`、`:30`），`$table-border: thin solid $table-border-color`（`VTable/_variables.scss:17`） | 把线调到几乎不可见：`variables: { 'border-opacity': 0.06 }`——因为 `$border-color-root: rgba(var(--v-border-color), var(--v-border-opacity))`（`settings/_variables.scss:24`） | 运行时 token |
| 4 | 行高 52px 太松散 | `$table-row-height: 52px`（`VTable/_variables.scss:14`）+ `height: var(--v-table-row-height)`（`VTable.sass:110`） | 覆写 `--v-table-row-height`，或 SASS 改 `$table-row-height` | CSS 变量 / SASS |
| 5 | `density="comfortable"` 只降 2px | `$table-density: ('default': 0, 'comfortable': -2, 'compact': -4)`（`VTable/_variables.scss:9`） | 想要真密度只能覆写 CSS 变量或 SASS | CSS 变量 / SASS |
| 6 | 圆角 4px 的 Material 味 | `$border-radius-root: 4px`（`settings/_variables.scss:25`） | SASS 改 `$border-radius-root` | SASS |
| 7 | 字体实际回落到系统 sans | `$body-font-family: 'Roboto', sans-serif`（`settings/_variables.scss:21`），但 `web/index.html:1-12` 无任何字体 link，且 `web/src` 与 `web/index.html` 中 grep `Roboto` **0 命中** | SASS 改 `$body-font-family`，或 CSS 覆写 `html/body` 的 font-family | SASS / CSS 覆写 |
| 8 | 按钮全大写 | `$typography` 的 button 条目 `'text-transform': uppercase`（`settings/_variables.scss:306`，条目范围 `:300-307`）；与 `.impeccable.md:20` 的 "no all-caps body" 冲突 | SASS 改 `$typography`，或 CSS 覆写 `.v-btn { text-transform: none }` | SASS / CSS 覆写 |
| 9 | 只有浅色，没有暗色 | `web/src/plugins/vuetify.ts:12-22` 只定义了 `dark: false` 的 `subforge` 一个主题 | 加 `themes.dark`（`.impeccable.md:18` 要求 both + 手动覆盖） | 运行时 token |
| 10 | 主色是 Material 蓝 | `web/src/plugins/vuetify.ts:16` `primary: '#1976d2'`（Material Blue 700），与 Vuetify 默认 light primary `#1867C0`（`theme.js:24`）同族 | 换成 OKLCH 推导的自有主色 | 运行时 token |
| 11 | 39 处卡片是 `variant="tonal"` 混搭少量默认 elevated | 例 `web/src/pages/` 与 `web/src/components/` 中 `<v-card variant="tonal"` 与裸 `<v-card` 并存 | 用 `defaults: { VCard: {...} }` 统一 | 运行时 token |

### 2.3 「框架换不掉」清单（逐条给证据）

| # | 换不掉的点 | 证据 | 为什么主题层解决不了 |
|---|---|---|---|
| 1 | `v-data-table` 的**边框语言** | 每个 `td` 都有 `border-bottom: $table-border`（`VTable.sass:30`）；`VDataTable.sass:96`、`:99` 还给固定列加 `1px solid rgba(var(--v-border-color), var(--v-border-opacity))` | 主题层只能把不透明度压到 0 让线"看不见"，**换不成 Grafana 那种分组表头 / 无网格语言**；线是组件模板结构 |
| 2 | 行高 / 表头高是**固定 px** | `$table-header-height: 56px`（`VTable/_variables.scss:10`）、`$table-row-height: 52px`（`:14`） | 改的是"数值"，不是"密度语言"；且 `density` 只有 4px 档位差（`:9`） |
| 3 | 列内边距写死 | `$table-column-padding: 0 16px`（`VTable/_variables.scss:20`），消费点 `VTable.sass:104` | 不受 4pt 间距令牌控制，只能覆写 CSS 或改 SASS |
| 4 | 运行时主题**只能改颜色与透明度** | `theme.js:20-53` 的键表就是全部可配项 | 圆角 / 字体 / 排版 / 阴影 / 间距全在 SASS **编译期**；不引 sass 就摸不到（本仓未引，见 §2.1 末行） |
| 5 | **没有字体令牌变量** | `_global.sass:6` 直接 `font-family: settings.$body-font-family` | 不存在 `--v-font-family` 这类变量，换字体只能 SASS 或裸 CSS 覆写 |
| 6 | 排版是 **Material 的 9 级制** | `$typography` 覆盖 h1–h6 / subtitle-1-2 / body-1-2 / caption / button / overline（`settings/_variables.scss:217-326`） | 即使逐项改数值，节奏仍是 Material 骨架；Linear / Grafana 是 4–5 级 + tabular numerals，属于另一套排版语言 |
| 7 | 状态层是 **overlay 叠色** | `tools/_states.sass` 用 `opacity: calc(... * var(--v-theme-overlay-multiplier))` 实现 hover/focus/selected | 这是 Vuetify 的交互质感来源（Material state layer），换成 Linear 那种"背景直接换色"需要重写组件样式 |
| 8 | shell 是经典 Material 三段式 | `web/src/layout/AppShell.vue:20-36` = `v-app > AppNav + AppTopBar + v-main > v-container fluid pa-6`；`AppTopBar.vue:11` 是 `<v-app-bar flat density="comfortable" color="surface">` | 这是页面骨架而非令牌，Linear 式的紧凑 chrome 必须重写 layout |

### 2.4 只做主题升级能做到几分像 Linear / Grafana？

**结论：形似约 4–5 成，神似不到 3 成。**

能做到（证据齐全）：
- **暗色双主题**：`theme.js:20-90` 给出明暗两套 14 色 + 16 变量键表，可整体替换；本仓缺的只是没写（`web/src/plugins/vuetify.ts:12-22`）。
- **阴影墙消除**：`defaults` 机制（`defaults.js:61`、`:71`）可全局压平 VCard / VBtn 的默认 elevated（`VCard.js:71-73`、`VBtn.js:74-76`）——这是"从 Bootstrap admin 观感 → Grafana panel 观感"性价比最高的一步。
- **边框噪声下降**：`variables: { 'border-opacity': … }` 直接作用于 `$border-color-root`（`settings/_variables.scss:24`），表格 1px 全网格会显著变淡。
- **密度接近**：覆写 `--v-table-row-height`（`VTable.sass:110` 读取）可把 52px 压到接近 Grafana 的行高。
- **主色/中性色重排**：14 个语义色键可任意换成 OKLCH 派生值，满足 `.impeccable.md:24` 的 OKLCH 要求（Vuetify 本身接受任意合法 CSS 颜色字符串）。

做不到（证据齐全）：
- **排版节奏**：`$typography` 是 Material 9 级制（`settings/_variables.scss:217-326`），需 SASS 且改完仍是 Material 骨架。
- **表格语言**：1px 全网格（`VTable.sass:23`、`:30`）只能"调透明"，换不成 Grafana 的分组/无网格。
- **按钮/输入形态**：Vuetify 的 variant 体系与 Material state layer（`tools/_states.sass`）决定了 hover/focus 的质感。
- **shell**：`AppShell.vue:20-36` 的三段式是页面骨架，主题层改不到。

**估计（非实测，标注为估计）**：本仓现状出厂观感约 **2.5 / 5**；只做主题升级（含 `defaults` + `variables` + 暗色 + CSS 变量直改）可到约 **3.5 / 5**；要到 **4.5 / 5** 必须同时动 shell（`AppShell.vue` / `AppTopBar.vue` / `AppNav.vue`）与表格呈现。

---

## 3. URL 实测 HTTP 状态码表

全部于 **2026-10-19** 用 Node `fetch`（浏览器 UA、`redirect: 'follow'`）逐条实测。**未使用 HEAD**（部分站点对 HEAD 返回不一致），统一 GET 后记录状态码与最终 URL。

### 3.1 候选与文档 URL

| URL | 状态码 | 最终 URL | 可信度备注 |
|---|---|---|---|
| https://vuetifyjs.com/ | 200 | 同 | **SPA 外壳**（见 §3.2） |
| https://vuetifyjs.com/en/features/theme/ | 200 | 同 | SPA 外壳，不可证内容 |
| https://vuetifyjs.com/en/features/sass-variables/ | 200 | 同 | SPA 外壳，不可证内容 |
| https://vuetifyjs.com/en/features/global-configuration/ | 200 | 同 | SPA 外壳，不可证内容 |
| https://vuetifyjs.com/en/components/data-tables/basics/ | 200 | 同 | SPA 外壳，不可证内容 |
| https://vuetifyjs.com/en/features/accessibility/ | 200 | 同 | SPA 外壳，不可证内容 |
| https://vuetifyjs.com/en/getting-started/release-notes/ | 200 | 同 | SPA 外壳，不可证内容 |
| https://github.com/vuetifyjs/vuetify | 200 | 同 | 可信 |
| https://www.shadcn-vue.com/ | 200 | 同 | 可信（站点对乱路径 404） |
| https://www.shadcn-vue.com/docs/theming | 200 | 同 | 可信（`.html` 后缀会 301 到无后缀） |
| https://www.shadcn-vue.com/docs/dark-mode | 200 | 同 | 可信 |
| https://www.shadcn-vue.com/docs/components/button | 200 | 同 | 可信 |
| https://github.com/unovue/shadcn-vue | 200 | 同 | 可信 |
| https://www.naiveui.com/ | **403** | 同 | Cloudflare 人机校验（响应体 `<title>Just a moment...</title>`），**非死链** |
| https://www.naiveui.com/en-US/os-theme/docs/customize-theme | **403** | 同 | 同上 |
| https://www.naiveui.com/en-US/os-theme/docs/dark-theme | **403** | 同 | 同上 |
| https://www.naiveui.com/en-US/os-theme/docs/accessibility | **403** | 同 | 同上 |
| https://github.com/tusen-ai/naive-ui | 200 | 同 | 可信 |
| https://primevue.org/ | 200 | https://primevue.dev/ | 302 跳转，可信（跳转后站点对乱路径 404） |
| https://primevue.org/theming/styled/ | 200 | https://primevue.dev/theming/styled/ | 可信 |
| https://primevue.org/guides/accessibility/ | 200 | https://primevue.dev/guides/accessibility/ | 可信 |
| https://github.com/primefaces/primevue | 200 | 同 | 可信 |
| https://element-plus.org/ | 200 | 同 | 可信（站点对乱路径 404） |
| https://element-plus.org/en-US/guide/theming.html | 200 | 同 | 可信 |
| https://element-plus.org/en-US/guide/dark-mode.html | 200 | 同 | 可信 |
| https://github.com/element-plus/element-plus | 200 | 同 | 可信 |
| https://arco.design/vue/en-US/docs/start | 200 | 同 | **SPA 外壳，不可证内容** |
| https://arco.design/vue/en-US/docs/token | 200 | 同 | **SPA 外壳，不可证内容** |
| https://arco.design/vue/en-US/docs/accessibility | 200 | 同 | **SPA 外壳，不可证内容** |
| https://github.com/arco-design/arco-design-vue | 200 | 同 | 可信 |
| https://antdv.com/ | 200 | 同 | **SPA 外壳，不可证内容** |
| https://antdv.com/docs/vue/customize-theme | 200 | 同 | **SPA 外壳，不可证内容** |
| https://antdv.com/docs/vue/accessibility | 200 | 同 | **SPA 外壳，不可证内容** |
| https://github.com/vueComponent/ant-design-vue | 200 | 同 | 可信 |
| https://reka-ui.com/ | 200 | 同 | 可信（站点对乱路径 404） |
| https://reka-ui.com/docs/overview/accessibility | 200 | 同 | 可信 |
| https://reka-ui.com/docs/components/dialog | 200 | 同 | 可信 |
| https://github.com/unovue/reka-ui | 200 | 同 | 可信（`unovue/radix-vue` 301 到此，证明是同一仓库改名） |
| https://daisyui.com/ | 200 | 同 | 可信（站点对乱路径 404） |
| https://daisyui.com/docs/themes/ | 200 | 同 | 可信 |
| https://daisyui.com/docs/colors/ | 200 | 同 | 可信 |
| https://github.com/saadeghi/daisyui | 200 | 同 | 可信 |
| https://flowbite-vue.com/ | 200 | 同 | 可信（站点对乱路径 404）；**独立二次复验时同一 URL 返回 429（限流），非死链**，见 §3.1.1 |
| https://github.com/themesberg/flowbite-vue | 200 | 同 | 可信 |
| https://headlessui.com/ | 200 | 同 | 可信（站点对乱路径 404） |
| https://headlessui.com/v1/vue | 200 | 同 | 可信（Vue 只有 v1 文档） |
| https://headlessui.com/v1/vue/menu | 200 | 同 | 可信 |
| https://github.com/tailwindlabs/headlessui | 200 | 同 | 可信 |
| https://tailwindcss.com/ | 200 | 同 | 可信（站点对乱路径 404） |
| https://tailwindcss.com/docs/theme | 200 | 同 | 可信 |
| https://tailwindcss.com/docs/dark-mode | 200 | 同 | 可信 |
| https://github.com/tailwindlabs/tailwindcss | 200 | 同 | 可信 |

### 3.1.1 独立二次复验（Windows PowerShell 5.1，换工具链交叉验证）

上表的 52 条 URL 用**另一套 HTTP 客户端**（`Invoke-WebRequest`，非 Node `fetch`）重跑了一遍，避免单一客户端的 UA / 重定向策略造成系统性误判。命令骨架（本机是 Windows PowerShell 5.1，**没有** `-SkipHttpErrorCheck` 参数，必须用 try/catch 取状态码）：

```powershell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ua = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36'
foreach ($u in $urls) {
  try {
    $r = Invoke-WebRequest -Uri $u -Method Get -UserAgent $ua -MaximumRedirection 5 -TimeoutSec 25 -UseBasicParsing
    $code = [int]$r.StatusCode; $final = $r.BaseResponse.ResponseUri.AbsoluteUri
  } catch {
    $code = [int]$_.Exception.Response.StatusCode
    $final = $_.Exception.Response.ResponseUri.AbsoluteUri
  }
  Write-Output ('{0}  {1}  <- {2}' -f $code, $final, $u)
}
```

输出末行：`TOTAL=52  HTTP200=47  OTHER=5`。与上表的一致性结论：

| 差异项 | Node fetch（第一轮） | PowerShell 5.1（第二轮） | 判定 |
|---|---|---|---|
| naiveui.com × 4 条 | 403 | 403 | 一致，Cloudflare 人机校验，**非死链** |
| flowbite-vue.com/ | 200 | **429** | 两次不一致 → 判定为**服务端限流**（同一 IP 连续请求），**非死链**；引用该 URL 时须注明 |
| primevue.org × 3 条 | 200（302 → primevue.dev） | 200（302 → primevue.dev） | 一致 |
| 其余 44 条 | 200 | 200 | 一致 |

**结论：52 条中 0 条死链，2 处非 200 均为反爬/限流而非 404。** 唯一不稳定的 URL 是 flowbite-vue.com 与 naiveui.com 四页，本文对二者的任何描述均以「命令行不可验证」为前提，不作正文取证。

**实测为 404 的 URL（真死链，本文件不引用）**：
https://www.shadcn-vue.com/docs/dark-mode/vue.html ｜ https://www.shadcn-vue.com/docs/accessibility ｜ https://headlessui.com/vue/menu ｜ https://element-plus.org/en-US/guide/a11y.html ｜ https://element-plus.org/en-US/guide/accessibility.html ｜ https://daisyui.com/docs/accessibility/ ｜ https://flowbite-vue.com/accessibility

### 3.2 SPA catch-all 陷阱（方法论警告，**必须**跟着状态码一起读）

对**不存在的路径**做对照实测，以下三家站点对乱路径也返回 **200**：

| 站点 | 乱路径 | 状态码 | 响应体大小 |
|---|---|---|---|
| https://vuetifyjs.com/en/features/zzz-not-a-page-xyz/ | 200 | 1532 B（只有 SPA 外壳） |
| https://antdv.com/docs/vue/zzz-not-a-page-xyz | 200 | 2623 B |
| https://arco.design/vue/en-US/docs/zzz-not-a-page-xyz | 200 | 1319 B |

⇒ **vuetifyjs.com / antdv.com / arco.design 的 200 不能证明页面存在**；它们的 `sitemap.xml` 同样只回 SPA 外壳（实测三站 `sitemap.xml` 均返回同一 shell，无 `<urlset>`）。

对照组：以下站点对乱路径**正确返回 404**，其 200 可信——
element-plus.org 404 ｜ flowbite-vue.com 404 ｜ www.shadcn-vue.com 404（171 B）｜ reka-ui.com 404 ｜ daisyui.com 404 ｜ headlessui.com 404 ｜ primevue.dev 404（201 B）｜ tailwindcss.com 404。

Vuetify 文档站是 SPA，HTML 里只有应用挂载点、正文由 JS 渲染；本表所有 Vuetify 行的 200 只代表**该 URL 会被站点接受**，不代表已抓到正文。Vuetify 3 的机制结论全部改用**本地已安装包源码**取证（见 §2.1 的 `文件:行号`），不依赖文档正文。

### 3.3 体积数据来源 URL（bundlephobia API，2026-10-19 实测）


| package | min (B) | gzip (B) | deps | API URL |
|---|---|---|---|---|
| `vuetify@4.2.6` | 51000 | 19595 | 1 | https://bundlephobia.com/api/size?package=vuetify@4.2.6 |
| `naive-ui@2.45.3` | 2199719 | 521478 | 18 | https://bundlephobia.com/api/size?package=naive-ui@2.45.3 |
| `primevue@5.0.2` | 2081403 | 444405 | 8 | https://bundlephobia.com/api/size?package=primevue@5.0.2 |
| `element-plus@2.14.7` | 1066110 | 298422 | 15 | https://bundlephobia.com/api/size?package=element-plus@2.14.7 |
| `@arco-design/web-vue@2.58.0` | 882030 | 220279 | 9 | https://bundlephobia.com/api/size?package=@arco-design/web-vue@2.58.0 |
| `ant-design-vue@4.2.6` | 1451159 | 419641 | 22 | https://bundlephobia.com/api/size?package=ant-design-vue@4.2.6 |
| `reka-ui@2.11.0` | 832560 | 188680 | 9 | https://bundlephobia.com/api/size?package=reka-ui@2.11.0 |
| `daisyui@5.7.50` | 306719 | 50411 | 0 | https://bundlephobia.com/api/size?package=daisyui@5.7.50 |
| `flowbite-vue@0.4.0` | 271837 | 70017 | 12 | https://bundlephobia.com/api/size?package=flowbite-vue@0.4.0 |
| `@headlessui/vue@1.7.23` | 123066 | 32296 | 1 | https://bundlephobia.com/api/size?package=@headlessui/vue@1.7.23 |
| `tailwindcss@4.3.3` | 284070 | 70742 | 0 | https://bundlephobia.com/api/size?package=tailwindcss@4.3.3 |

> 注意：`vuetify` 的 51000 / 19595 是 **barrel 入口**值，不代表整库。Vuetify 按组件 tree-shake，实际体积随用到的组件增长；本仓 `web/src/plugins/vuetify.ts:8` 用 `import * as components from 'vuetify/components'` **全量注册**，因此现网产物体积与 barrel 值同量级偏大。
> `shadcn-vue` 不是运行时库（源码复制进仓库），bundlephobia 无意义：其体积 = 你实际复制的组件源码 + 对应 Reka UI primitive。

---

## 4. 迁移成本量级的判定口径

| 量级 | 判定标准 | 本仓对应事实 |
|---|---|---|
| 低 | 不改组件调用，只改配置 | 仅 Vuetify 3 自身（改 `web/src/plugins/vuetify.ts` + 可选加 sass） |
| 中 | 可并存、逐步替换，但需调解样式层 | Tailwind CSS v4 独立引入（与 Vuetify 并存） |
| 高 | 组件 API 不同，需逐页改写模板；`v-data-table` 无等价物或需重写列定义 | Naive UI / PrimeVue / Element Plus / Arco / Ant Design Vue / shadcn-vue / DaisyUI / Flowbite Vue |
| 极高 | 连基础控件与样式都要自己写 | Reka UI / Headless UI Vue |

本仓受影响的调用点（用于估算改写量）：`<v-btn>` 89 处、`<v-card>` 41 处、`<v-card-text>` 24 处、`<v-card-title>` 23 处、`<v-text-field>` 17 处、`<v-select>` 15 处、`<v-data-table>` 4 处（组件频次表见 `docs/design/frontend-visual-upgrade-plan.md:127-134`）。

---

## 5. 未决问题 / 偏离

1. **Naive UI 官网被 Cloudflare 挡（403）**，文档正文无法用命令行验证。其令牌机制（`theme-overrides` / `useThemeVars` / `--n-*`）的旁证来自 npm 包内类型定义与源码（`naive-ui@2.45.3` 的 `es/index.d.ts` 导出 `darkTheme` / `useThemeVars` / `GlobalTheme`；`es/config-provider/src/ConfigProvider.d.ts` 含 `themeOverrides`；`es/button/src/Button.mjs` 含 `--n-color` 等）。若需官网原文佐证，须由人在浏览器打开。
2. **Vuetify / Ant Design Vue / Arco 三家站点是 SPA catch-all**，其文档页「存在性」无法用 HTTP 证明（§3.2）。本文件的这三家机制结论一律以**包内源码 / 包内 CSS** 取证，未依赖文档正文。
3. **本卡未做真机渲染验证**：没有安装任何候选库、没有跑 Lighthouse / axe。原因是安装依赖必须改 `web/package.json`，超出本卡写范围（任务书硬要求禁止）。
4. **`api.github.com` 在本机 IP 已被限流**（首次 200 后全部 403），Stars / forks 改用 `ungh.cc` 代理 + `github.com` 页面 HTML 的 `stargazerCount` 正则。若 Lead 要二次校验，请用同法，直接打 REST API 会 403。
5. **任务书写「PrimeVue 4」，实测当前主线已是 5.0.2**（`v4-stable` = 4.5.5 @ 2026-04-08）。表格按「主线 5 + v4 冻结线」双写，未把 4 当作最新版本。
6. **本仓 `<v-btn>` 计数与计划文档不一致**（我 89 / 计划 93），已定位为「精确开标签 vs 前缀（含 `<v-btn-toggle>`）」的口径差异，详见 §0.1；计划文档 `:14` 无需改动，但引用时请注明口径。
7. **本文件不含任何选型结论**（任务书要求结论由 Lead 下）；§2.4 的 2.5 / 3.5 / 4.5 分是**标注为估计**的主观刻度，不是实测数据。

---

## 6. 采集方法与工具限制

| 项 | 做法 |
|---|---|
| 外部数字（Stars / forks / pushedAt） | `https://ungh.cc/repos/<owner>/<repo>`（unjs GitHub 代理）为主，`https://github.com/<owner>/<repo>` HTML 里的 `stargazerCount":(\d+)` / `forksCount":(\d+)` 兜底 |
| 最近提交时间 | 公开无鉴权 atom feed `https://github.com/<owner>/<repo>/commits.atom` 的 `<updated>` |
| 最近发布版本 + 日期 | `https://registry.npmjs.org/<pkg>` 的 `time` 与 `dist-tags`；仓库侧用 `https://github.com/<owner>/<repo>/releases.atom` 交叉验证 |
| 近 12 个月发布数 | 数 `registry.npmjs.org` 的 `time` 中落在 2025-10-19 ~ 2026-10-19 的条目 |
| 体积 | `https://bundlephobia.com/api/size?package=<pkg>` 的 `size` / `gzip` / `dependencyCount` |
| 令牌机制 / 组件源码取证 | `https://cdn.jsdelivr.net/npm/<pkg>@<ver>/<path>`（等价于 npm 包内文件）；Vuetify 与 ARIA 统计取自本机 `web/node_modules/vuetify/**` |
| HTTP 状态码 | Node `fetch`，浏览器 UA、`redirect: 'follow'`、GET；同时对乱路径做 catch-all 对照（§3.2） |
| 未采用 | `api.github.com`（本机 IP 限流 403）；HEAD 请求（部分站点不一致）；无头浏览器渲染（本卡不引入新依赖） |

**diff 说明**：本卡执行期间**只新建本文件**，未修改、未删除任何既有文件；`web/**` 与 `web/package.json` 全程未触碰。
