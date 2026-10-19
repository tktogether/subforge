# SubForge 全站 UX 能力对照审计与修复计划（U9 波次）

> 2025 · Lead 起草 · 队员按卡认领 · 本文件是唯一计划真值源，任务书均从 %TEMP% 派发

## 1. 背景：用户反馈（原文）

1. 「这个按钮的开关状态就完全没对比吗，而且与背景如此重合？？？还有上面的溢出」——v-switch 未选中态与米色背景近乎同色；v-dialog 高内容顶部被视口裁切。
2. 「INI源的导入去哪里了，还得我手动吗，你给我好好排查一下设计-后端-WEB的能力对照」——规则源无新增入口，INI 源要手编 manifest.yaml。
3. 「per-token操作就不能走下拉吗，非得人手填」——令牌对话框 pipeline/config 手填。
4. 「就完全不考虑用户小白吗，逻辑一团糟，重走整个项目逻辑」——全站按新手用户重走逻辑，产出全项目能力对照矩阵。

## 2. 审计方法：设计-后端-WEB 三列能力对照（覆盖全部 12 页 + 全局）

矩阵列（每页一行，逐页给出 file:line 证据）：

| 列 | 含义 |
| --- | --- |
| 页面 | router/index.ts:16-30 的 12 页 + 全局（开关/对话框/空态/文案/help） |
| 新手目标 | 小白在这页想完成什么（一句话） |
| 后端能力 | internal/api/server.go:137-274 路由 + handler:行号 |
| endpoints.ts 封装 | web/src/api/endpoints.ts 对应方法:行号 |
| WEB 控件 | 页面实际暴露的按钮/输入/下拉 |
| 能力差距 | 后端有 WEB 无 / WEB 有后端无 / 双缺 / 下拉可用却手填 / 状态语义矛盾 |
| 新手断点 | 卡在哪、为什么不自明 |
| 级别 | P0 阻断主链路 / P1 明显摩擦 / P2 打磨 |
| 修复去向 | 卡片号或「暂不修」 |

产出：docs/design/ux-capability-matrix.md（R6 卡，只读审计不改代码）。

## 3. 已确认 findings（Lead 逐行核实过的种子，R6 须验证并扩充）

- F1 规则源无新增/删除：前后端双缺（server.go:137-274 无 POST/DELETE；RuleSourcesPage.vue 无入口）→ INI 源手编 manifest.yaml。
- F2 RuleSourcesPage.vue:199 行内「更新」@click="updateAll" 调的是全量接口。
- F3 令牌对话框 pipeline(:361)/config(:362) v-text-field 手填；ConvertPage.vue:92,234 已有现成下拉模式（endpoints.pipelines('')）。
- F4 v-switch 未选中态 track 无可见对比（VSwitch.css:25 未选中组合选择器；tokens.css --sf-bg:#F0F2E6 与默认 track 近同色）。
- F5 v-dialog 高内容顶部裁切（VDialog.css:6 居中布局无 max-height 约束）。
- F6 机场页「刷新时同时导入规则」开关与行内「导入规则」按钮关系对新手不自明。
- F7 settings 页暴露 manifest 路径等内部概念（settings.go:99）。

## 4. 新手主链路走查清单（重走项目逻辑，R6 按此逐条给结论）

- L1 首次打开概览页：能否明白系统做什么、下一步做什么。
- L2 转换页：不配任何来源，直接贴链接能否走通（ConvertRequest.urls/text）。
- L3 机场：新增 → 刷新 → 「刷新时同时导入规则」开关语义是否自明。
- L4 规则：外部 INI 规则源导入路径（当前断点：手编 manifest.yaml）。
- L5 订阅：新建令牌 → 编辑（target 已下拉；pipeline/config 手填 = 断点）。
- L6 输出验证：转换历史/任务页反馈是否闭环。
- L7 失败路径可见性：断网 / 403 / 模板错误能否 actionable。

## 5. 工作拆解（写范围互斥）

| 卡 | 标题 | Owner | 写范围 | 依赖 | 验收 |
| --- | --- | --- | --- | --- | --- |
| R1 | 后端规则源写 API（新增/删除/单源更新） | free-implementer-go | internal/api/rule_sources*.go、internal/api/server.go、internal/subforge/rulesources*.go、internal/manifest/manifest*.go | 无 | go vet + go test 全绿 + httptest 探针 |
| R2 | 全局开关对比度 + 对话框溢出（CSS） | free-implementer-web | web/src/styles/vuetify-overrides.css + 新 spec | 无 | web typecheck/test/build 全绿 + dist CSS 探针 |
| R3 | 规则源页新增/删除/单源更新 UI | free-implementer-web | web/src/pages/RuleSourcesPage.vue(+spec)、web/src/api/endpoints.ts(+spec) | 契约锁定，可与 R1 并行 | vitest 绿 + 契约一致 |
| R4 | 令牌对话框 pipeline/config 下拉化 | free-implementer-web | web/src/pages/SubscriptionsPage.vue(+spec)、web/src/api/endpoints.ts(+spec) | blocked_by R5 | vitest 绿 + 失败退化手填 |
| R5 | 后端 GET /api/configs（config 下拉数据源） | free-implementer-go | internal/api/configs*.go、internal/api/server.go、（必要时 internal/convert 只读新增） | 无 | go vet + go test 全绿 |
| R6 | 全站能力对照矩阵（只读审计） | free-implementer-audit | docs/design/ux-capability-matrix.md | 无 | 12 页+全局全覆盖，每行 file:line 证据 |

## 6. 契约锁定（R1/R3、R4/R5 任务书逐字引用，前后端同一份）

- POST /api/rule-sources  body {name?, url, input_format?, type?} → 200 {path, name, created:true}；400 URL 空/名称重复。name 缺省取 URL basename（复用 manifest applyDefaults 语义）。
- DELETE /api/rule-sources/{name} → 200 {path, removed}；404 不存在；400 删除后 sources 为空（validate 红线）。
- POST /api/rule-sources/update/{name} → 200 {results:[{source,version,path,from_cache,err}]}（与全量 update 同构）；404 不存在。实现：manifest.Update 新增 WithSources(names ...string) UpdateOption（仿 WithRetention :275）。
- 写回纪律：**不得走 LoadManifest 的返回值写回**（Output.Path 已转绝对 :60-64）；读原始字节 → unmarshal → append/删 → validate → Marshal 2 空格缩进 → 备份 .bak → 0644 写回。
- GET /api/configs → 200 {configs:[name,...], default:第一个或空串}（R4 前端依 default 预选）；候选=当前生效模板 [common]+[custom] 段 config= 去重保序；engine nil → 503；BaseTemplate nil → 200 空集（engine.go:304 合法空态）；ResolveTemplates 失败 → 502（BUG-3 教训，pipelines.go:89-91）。

## 7. 全量验收口径（Lead 收口独立复跑）

- 后端：go vet ./... ；go test ./... -count=1；gofmt -l 为空。
- WEB：pnpm -C web run typecheck；pnpm -C web run test；pnpm -C web run build。
- 探针：R2 构建产物 dist CSS grep 新规则；R1/R5 httptest 探针；收口后 git status 查乱码文件 + 显式路径提交。
