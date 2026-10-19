# 前端视觉升级 · Wave 2 决策记录

> 状态：**已拍板，进入 Wave 3 落地**
> 日期：2026-10-19 · Owner：lead
> 上游：`docs/design/frontend-visual-upgrade-plan.md`（Wave 1 调研与验收总账）

## 1. 用户决策（原文）

- User said (m00748): "D1 确认"

即：**选定方向 D1 Instrument（冷静 · 精确 · 密度优先）**，样板为 `docs/design/mockups/d1-instrument.html`（52,364 字节，1600/1280 双视口下 render-check 与 cell-fit-check 双门禁 exit 0）。

## 2. 框架决策：不换库，留在 Vuetify 3.13

依据（详见 plan §4.1，7 条证据）：

1. 「丑」的根因是 `web/src/plugins/vuetify.ts:15-20` 只配了 4 个颜色，不是 Vuetify 本身。
2. 换库体积全亏：Vuetify barrel 51KB min / 19.6KB gzip vs Naive 2.20MB、PrimeVue 2.08MB、Element Plus 1.07MB、Arco 882KB。
3. Ant Design Vue 4 已双停在 2024-11（npm 4.2.6），不可选。
4. Element Plus 令牌是 hex（`--el-color-primary:#409eff`），与 `.impeccable.md` 的 OKLCH 要求冲突。
5. 换库模板改动面大（A 档 0 页 / B 档 8 页 / C 档 5 页，C 档含列定义数组 5 处、`#item.*` 插槽 15 处、展开行手写 tr 1 处、表格选择协议 1 处，跨库不可平移）。
6. 门禁不挡换肤，但有 6 处断言耦合 CSS/DOM（见 plan §B.8），换库这些断言必然重写。
7. Vuetify 表格行高可达标：`VTable.sass:109-113` 使用 `var(--v-table-row-height)`/`var(--v-table-header-height)`，运行时可用 CSS 覆写把 52px 压到 D1 的 ~40px（Wave 3 第一件事实测）。

> 说明：用户本轮只答复了方向选择。Lead 按上述证据裁定保持 Vuetify（D1 是令牌/样式层方向，与框架选型解耦）。若用户后续要求换库，本决策可推翻，且令牌层（`web/src/design/tokens.css`）与全局样式层（`web/src/styles/**`）可原样迁移。

## 3. 视觉契约（Wave 3 冻结，实现方必须逐条遵守）

- **令牌**：19 个必需 `--sf-*` + 3 个扩展（`--sf-font-sans`、`--sf-ease`、`--sf-shell`）；浅色为默认（`:root`），深色在 `[data-theme="dark"]` 与 `.v-theme--dark` 下覆盖。值逐字取自 `docs/design/mockups/d1-instrument.html:53-102`（浅色 :53-78、深色 :79-102），实现方**不得自行改色**。
- **圆角/边框**：`--sf-radius: 6px`，面板用 1px 细边框分层，**不使用阴影**表达层级。
- **字号/排版**：正文 13–14px、行高 1.5；数字列 `font-variant-numeric: tabular-nums`；URL/令牌用 `--sf-font-mono`；正文容器 ≤ 1280px；段落 ≤ 65ch。
- **密度**：表格行高 ≈ 40px（缓冲区 ≤ 42px），表头 40px；间距一律 4px 的倍数（`--sf-space: 16px` 为基数）。
- **动效**：`--sf-ease: cubic-bezier(0.16, 1, 0.3, 1)`；微交互 150ms、页面过渡 400ms；`prefers-reduced-motion: reduce` 下降级为 0。禁止 bounce/elastic、禁止 >600ms。
- **可访问性**：`focus-visible` 2px 实线 + 2px offset，任何交互元素不得 `outline: none`；正文对比度 ≥ 4.5:1，控件边框 ≥ 3:1。
- **反模式红线**（`.impeccable.md` + frontend-design-pro）：禁 `#000000`、禁 Arial/Inter/system-ui 作为首选字体、禁毛玻璃（backdrop-filter）、禁渐变文字、禁 >1px 侧边色条、禁 hero 大指标、禁全大写正文、禁嵌套卡片。
- **双主题**：浅色默认；深色由 `useThemeMode().isDark` 驱动（`SettingsPage.vue:111-113` 已有 浅色/深色/跟随系统 三个按钮，Wave 1 前它们只写 localStorage、并未真正切换主题，Wave 3 补上接线）。

## 4. Wave 3 卡片与写范围（互斥）

| 卡 | 内容 | 写范围 |
|---|---|---|
| W1 | 令牌层 + Vuetify 双主题 + 入口导入 | `web/src/design/**`、`web/src/plugins/vuetify.ts`(+spec)、`web/src/main.ts`(+spec) |
| W2 | 全局样式层（基础排版 / 密度 / Vuetify 覆盖） | `web/src/styles/**`（新目录，暂不由 main.ts 引用） |
| W3 | 外壳层（导航 / 顶栏 / 页头 / 异步壳 + 深色接线） | `web/src/layout/**`、`web/src/App.vue`(+spec)、`web/src/components/PageHeader.vue`、`AsyncSection.vue`(+spec) |
| W4 | 表格与页面（ResourcePage / AirportsPage / OverviewPage） | `web/src/components/ResourcePage.vue`(+spec)、`web/src/pages/AirportsPage.vue`(+spec)、`web/src/pages/OverviewPage.vue`(+spec) |
| W5 | 集成收口与全量回归（依赖 W1–W4） | `web/src/main.ts`（补 styles 导入）、`web/src/styles/**`（接手修）、必要时的 spec 断言修正 |

纪律：一文件一任务；不许删断言；不许改覆盖率阈值（`web/vitest.config.ts:26-31` lines 100 / functions 100 / branches 80 / statements 100）；改断言必须在汇报里逐条列出旧断言与新断言及理由。

## 5. 验收信号（每张卡都必须复跑）

- **G1** `pnpm typecheck` exit 0
- **G2** `pnpm test`：基线 **35 files / 594 tests** 全绿 exit 0（用例数只许增不许减）
- **G3** `pnpm build` exit 0（产物写 `internal/api/web/dist`）
- **G4** `pnpm coverage` exit 0（阈值 lines 100 / functions 100 / branches 80 / statements 100）
- **V1** Lead 在真机上用无头 Chrome 截图与 `docs/design/mockups/d1-instrument.html` 对照目视

## 6. 回滚

- 代码层：整体回退到决策前的 commit（Wave 3 前 HEAD = `780ec04` 之后的最新提交），再重新 `pnpm build`。
- 文档层：`docs/design/**` 全部留档，不回滚。

## 7. 执行记录（随进度追加）

- （待填）
