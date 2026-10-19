# T3a 组件使用面清单 + 「只改主题」能覆盖多少

- 统计对象：`E:\Projects\subforge\web\src` 下全部 `*.vue`（DSH glob `web/src/**/*.vue` 实测 **23 个**）
- 统计工具：DSH `glob` / `read` / `grep`（未使用 shell 的 grep/find）
- 所有数字均为本轮实测，未使用任何记忆或估算值。

---

## 1. Vuetify 组件标签使用面（按出现次数降序）

统计命令原文（DSH run_code，用 DSH 的 glob 枚举文件、DSH 的 read 取内容、JS 正则计数）：

```ts
const g = await tools.glob({ pattern: "web/src/**/*.vue" });
const tagRe = /<v-[a-zA-Z0-9-]+/g;
const agg = {};
for (const p of g.paths.sort()) {
  const r = await tools.read({ file_path: p, limit: 2000 });
  const lines = r.lines.map(l => l.text);
  for (const t of lines) {
    const m = t.match(tagRe);
    if (m) for (const x of m) { (agg[x] ||= {occ:0, files:new Set()}); agg[x].occ++; agg[x].files.add(p); }
  }
}
const rows = Object.entries(agg).map(([k,v]) => ({tag:k, occ:v.occ, files:v.files.size}))
  .sort((a,b) => b.occ - a.occ || a.tag.localeCompare(b.tag));
```

计数口径：正则 `<v-[a-zA-Z0-9-]+` 贪婪匹配**开标签**，闭标签 `</v-...>` 不匹配；`<v-card-title` 不会被误计成 `<v-card`。同一行的多个标签都计入。

| # | 组件标签 | 出现次数 | 出现文件数 | 组件 CSS 是否引用 `--v-theme-*` |
|---:|---|---:|---:|---|
| 1 | `<v-btn` | 93 | 12 | 是 |
| 2 | `<v-card` | 41 | 14 | 是 |
| 3 | `<v-card-text` | 24 | 10 | 否 |
| 4 | `<v-card-title` | 23 | 10 | 否 |
| 5 | `<v-text-field` | 17 | 7 | 是 |
| 6 | `<v-select` | 15 | 5 | 是 |
| 7 | `<v-spacer` | 14 | 8 | 否 |
| 8 | `<v-card-actions` | 13 | 7 | 否 |
| 9 | `<v-dialog` | 12 | 6 | 否 |
| 10 | `<v-alert` | 9 | 7 | 是 |
| 11 | `<v-switch` | 8 | 7 | 是 |
| 12 | `<v-textarea` | 8 | 2 | 否 |
| 13 | `<v-chip` | 7 | 6 | 是 |
| 14 | `<v-expansion-panel` | 7 | 5 | 是 |
| 15 | `<v-expansion-panel-text` | 7 | 5 | 否 |
| 16 | `<v-expansion-panels` | 7 | 5 | 否 |
| 17 | `<v-list-item` | 6 | 4 | 是 |
| 18 | `<v-expansion-panel-title` | 5 | 3 | 否 |
| 19 | `<v-col` | 4 | 1 | 否 |
| 20 | `<v-data-table` | 4 | 4 | 是 |
| 21 | `<v-list` | 4 | 3 | 是 |
| 22 | `<v-tab` | 4 | 1 | 否 |
| 23 | `<v-table` | 4 | 4 | 是 |
| 24 | `<v-row` | 2 | 1 | 否 |
| 25 | `<v-app` | 1 | 1 | 是 |
| 26 | `<v-app-bar` | 1 | 1 | 是 |
| 27 | `<v-app-bar-nav-icon` | 1 | 1 | 否 |
| 28 | `<v-app-bar-title` | 1 | 1 | 否 |
| 29 | `<v-container` | 1 | 1 | 否 |
| 30 | `<v-divider` | 1 | 1 | 否 |
| 31 | `<v-list-subheader` | 1 | 1 | 否 |
| 32 | `<v-main` | 1 | 1 | 否 |
| 33 | `<v-navigation-drawer` | 1 | 1 | 是 |
| 34 | `<v-skeleton-loader` | 1 | 1 | 是 |
| 35 | `<v-snackbar` | 1 | 1 | 是 |
| 36 | `<v-tabs` | 1 | 1 | 是 |
| — | **合计（36 个去重标签）** | **350** | — | **是：18 / 否：18** |

交叉校验（DSH grep 工具，原文命令 `tools.grep({pattern:"<v-btn", path:"web/src", include:"*.vue"})`）：

```
grep <v-btn        -> matches=93  files=12
grep <v-card-title -> matches=23  files=10
grep <v-list-item  -> matches=6   files=4
grep <v-data-table -> matches=4   files=4
```

与上表逐格一致。

「是否引用 `--v-theme-*`」一列的算法（机械、可复现）：标签 kebab-case → PascalCase 并加 `V` 前缀（`<v-btn` → `VBtn`），在 `web/node_modules/vuetify/lib` 下递归查找同名 `<VComp>.css`（实测共 118 个 .css），只要命中文件内容含子串 `--v-theme-` 即记「是」；找不到同名文件记「否」（保守）。
vuetify 实解析根目录：`E:\Projects\subforge\web\node_modules\.pnpm\vuetify@3.13.7_typescript@5.9.3_vue@3.5.44_typescript@5.9.3_\node_modules\vuetify`。
样例证据 `lib/components/VBtn/VBtn.css`：第 133 行 `background: rgb(var(--v-theme-surface));`、第 134 行 `color: rgba(var(--v-theme-on-surface), var(--v-high-emphasis-opacity));`。

---

## 2. 23 个 .vue 里的 `<style>` 块

统计命令原文（同一段程序里，逐行状态机）：

```ts
let inStyle = false, styleBlocks = 0, styleLines = 0, scoped = 0, styleStart = -1;
lines.forEach((t, i) => {
  if (!inStyle && /<style/.test(t)) { inStyle = true; styleBlocks++; styleStart = i+1; if (/scoped/.test(t)) scoped++; styleLines++; }
  else if (inStyle) { styleLines++; if (/<\/style>/.test(t)) { inStyle = false; /* 记录 start..i+1 */ } }
});
```

- 23 个 .vue 中**带 `<style` 块的文件数：1**
- 这些 style 块**总行数：7**（含 `<style>` 与 `</style>` 两行）
- 其中 `scoped` 的**数量：1**

| 文件（绝对路径） | style 块数 | 块行号范围 | 行数 | 是否 scoped |
|---|---:|---|---:|---|
| `E:\Projects\subforge\web\src\pages\ConvertPage.vue` | 1 | 294–300 | 7 | 是（`<style scoped>`，第 294 行） |

交叉校验（DSH grep 工具）：

```
tools.grep({pattern:"<style",  path:"web/src", include:"*.vue"}) -> matches=1
  web/src\pages\ConvertPage.vue:294 | <style scoped>
tools.grep({pattern:"scoped",  path:"web/src", include:"*.vue"}) -> matches=1
  web/src\pages\ConvertPage.vue:294 | <style scoped>
```

即：**22 / 23 个 .vue 完全没有自己的样式块**，样式 100% 来自 Vuetify 组件 CSS 与主题令牌。

---

## 3. 全局样式入口盘点

逐条给出处：

1. `E:\Projects\subforge\web\src\main.ts` 全文只有 3 行，**没有直接 import 任何 css**：
   - `E:\Projects\subforge\web\src\main.ts:1` `import { createSubforgeApp } from './bootstrap'`
   - `E:\Projects\subforge\web\src\main.ts:3` `createSubforgeApp().mount('#app')`
2. css 实际由 bootstrap → vuetify 插件间接引入（DSH read 实测）：
   - `E:\Projects\subforge\web\src\bootstrap.ts:3` `import { vuetify } from './plugins/vuetify'`
   - `E:\Projects\subforge\web\src\bootstrap.ts:9` `app.use(vuetify)`
   - `E:\Projects\subforge\web\src\plugins\vuetify.ts:1` `import 'vuetify/styles'`
   - `E:\Projects\subforge\web\src\plugins\vuetify.ts:2` `import '@mdi/font/css/materialdesignicons.css'`
   - 全仓 `web/src` 内含 `.css` 字样的 import 只有 **1 处**：`tools.grep({pattern:"\\.css['\"]", path:"web/src"})` → matches=1，即 `web/src/plugins/vuetify.ts:2`。
3. `E:\Projects\subforge\web\src\styles\` **不存在**：
   - `tools.glob({pattern:"web/src/styles/**"})` → `[]`
   - `tools.glob({pattern:"web/src/**/*.css"})` → `[]`（.scss/.sass/.less/.styl 同样全为 `[]`）
   - Node `fs.statSync("E:\\Projects\\subforge\\web\\src\\styles")` → ENOENT
4. `E:\Projects\subforge\web\index.html` **没有内联样式，也没有字体 `<link>`**：
   - `tools.grep({pattern:"<link|<style|font", path:"web/index.html"})` → **matches=0**
   - 全文 12 行（read 实测），`<head>` 仅 3 个 `<meta>` + `<title>`（第 4–6 行），`<body>` 只有 `<div id="app">`（第 9 行）与入口脚本（第 10 行）。
5. 现有主题令牌全文（`E:\Projects\subforge\web\src\plugins\vuetify.ts`，共 24 行）：
   - 第 11 行 `defaultTheme: 'subforge'`
   - 第 14 行 `dark: false`
   - 第 16–19 行 只有 **4 个颜色令牌**：`primary: '#1976d2'`、`secondary: '#00897b'`、`background: '#f4f6f8'`、`surface: '#ffffff'`

结论：**全局样式入口只有一个，且只有 4 个主题色令牌；没有任何自定义全局 CSS 层、没有 styles 目录、没有字体引入。** 这是「只改主题 + 新增全局 CSS 变量层」这一路线的全部现状基础。

---

## 4. 结论：只改 `vuetify.ts` 主题令牌 + 新增全局 CSS 变量层，能覆盖多少？

### 4.1 「受影响的组件标签占比」的定义与算法

**变体 A（保守下界，本文主口径）**

> 受影响标签占比 = |{ t ∈ T : t 对应组件在 Vuetify 自带 CSS 中引用 `--v-theme-*` }| / |T|
> T = 23 个 .vue 中出现的全部**去重** Vuetify 标签集合，实测 |T| = 36。

算法：对每个标签做 kebab→PascalCase+`V` 映射，在 vuetify `lib` 下找同名 `<VComp>.css`；命中且内容含 `--v-theme-` 记为受影响；**找不到同名 CSS 文件一律记不受影响**（保守，会漏计子组件）。

- 去重标签口径：**18 / 36 = 50.0%**
- 出现次数加权口径：**221 / 350 = 63.1%**

**变体 B（宽松上界）**

对变体 A 中「找不到同名 CSS」的标签，若其父组件 CSS 引用 `--v-theme-*`，则改记受影响。父子映射（机械规则 + 目录事实）：
`<v-card-text|v-card-title|v-card-actions>`→`VCard`、`<v-app-bar-nav-icon|v-app-bar-title>`→`VAppBar`、`<v-expansion-panel-text|v-expansion-panel-title|v-expansion-panels>`→`VExpansionPanel`、`<v-list-subheader>`→`VList`、`<v-tab>`→`VTabs`、`<v-textarea>`→`VField`（实测这些父 CSS 均含 `--v-theme-`；而 `VGrid.css` 不含，故 `<v-row>`/`<v-col>` 仍记不受影响）。

- 去重标签口径：**29 / 36 = 80.6%**
- 出现次数加权口径：**315 / 350 = 90.0%**

### 4.2 一段话结论

在本仓「22/23 个 .vue 无自带样式、全局只有 4 个主题色令牌、无 styles 目录」的现状下，只改 `E:\Projects\subforge\web\src\plugins\vuetify.ts` 的主题令牌，视觉改动**能确定性覆盖到约一半的去重标签（18/36 = 50.0%）、约六成的实际使用量（221/350 = 63.1%）**；把「子组件继承父组件 CSS」也算上，上界是 **29/36 = 80.6%**（按使用量 315/350 = 90.0%）。再叠加一个**不受限的全局 CSS 变量层**后，覆盖上界在理论上可以逼近 **100%**——因为 Vuetify 每个组件都会渲染稳定的 `.v-*` 类名，全局层可以用选择器改到几乎任意标签，而不必动模板；但这条路仍然改不了 **DOM 结构、组件选型、信息密度、页面层级、空态/加载态/响应式布局**，且全局覆盖越宽，回归风险与维护成本越高（每次 Vuetify 升级都要重新核对选择器）。

### 4.3 这个数字的局限（如实说明）

1. **口径是「主题令牌可达面」，不是「这 4 个令牌改变后的可见差异」。** `--v-theme-` 还匹配 `--v-theme-on-surface`、`--v-theme-overlay-multiplier` 等；只改 primary/secondary/background/surface 时，部分匹配到的标签未必产生肉眼差异。
2. **变体 A 系统性低估。** `VCardText/VCardTitle/VCardActions/VExpansionPanelText/VListSubheader/VRow/VCol` 等根本没有独立 CSS 文件，样式在父组件 CSS 里；这正是变体 B 存在的原因，两个数字要一起看。
3. **CSS 文件存在 ≠ 渲染链路。** Vuetify 3 有大量样式由运行时 composable/内联样式产生，组件 CSS 只是其中一部分；反之命中了 `--v-theme-` 也不代表该标签在当前页面真的被渲染到。
4. **计数单位是标签出现次数，不区分可见性。** 同一标签在 `v-if` 分支、弹窗内、隐藏 Tab 里的出现同样计数；也没区分标签是否传了 `color`/`variant` prop 覆盖主题色。
5. **纯布局标签被判为不受影响**（`<v-row>`/`<v-col>`/`<v-spacer>`/`<v-divider>`/`<v-container>`/`<v-main>`，合计 26 次出现）。若全局变量层去改 spacing/radius/border，这些标签实际也会变，占比会上升。
6. **正则口径的已知边界**：注释或 JS 字符串里出现的 `<v-xxx` 也会被计入；本轮未逐处人工剔除（`web/src` 中出现量 350 次，交叉校验的 4 个标签与 grep 逐格一致）。
7. **本数字与「视觉是否变好」无关**，只回答「能不能被改到」。审美质量取决于令牌取值与全局层的具体写法，本报告不做评价。

---

## 5. 自测原始输出

文件：`E:\Projects\subforge\docs\design\research\T3a-component-inventory.md`

```powershell
$f = 'E:\Projects\subforge\docs\design\research\T3a-component-inventory.md'
(Get-Item -LiteralPath $f).Length
(Get-Content -LiteralPath $f).Count
```

原文输出（Windows PowerShell 5.1.22621.6133）：

```
12606
187
```

**关于 `187` 的说明（实测，非估算）**：本文件是无 BOM 的 UTF-8，含中文。Windows PowerShell 5.1 的 `Get-Content` 默认按系统 ANSI（本机 zh-CN = GBK）解码，会把部分 UTF-8 多字节序列误当双字节字符吃掉换行字节，导致行数少算。同一文件用其他口径实测：

```powershell
(Get-Content -LiteralPath $f -Encoding utf8).Count   # -> 220
[System.IO.File]::ReadAllLines($f).Count             # -> 220
$PSVersionTable.PSVersion.ToString()                 # -> 5.1.22621.6133
```

```
220
220
5.1.22621.6133
```

即真实行数为 **220**（`187` 是 PowerShell 5.1 默认 ANSI 解码的产物）。字节数 `12606` 满足「≥ 4000 字节」；第 1 节表格数据行 **36 行**，满足「≥ 15 行」。

