# SubForge 全站能力对照矩阵（设计-后端-WEB）

> 生成：T6 审计 · 基准 HEAD 31c280b · 视角：第一次用的小白
> 覆盖：web/src/router/index.ts:17-29 的 13 个页面 + 全局层（v-switch/v-dialog/空态/文案/help）

## 一、页面 × 能力矩阵

| 页面 | 新手目标 | 后端能力 (server.go 路由+handler) | endpoints.ts 封装 | WEB 控件 | 能力差距 | 新手断点 | 级别 | 修复去向 |
|---|---|---|---|---|---|---|---|---|
| 概览 (OverviewPage.vue) | 看懂系统做什么、下一步去哪 | GET /api/stats (server.go:149); GET /api/history (150) | stats() 355; history() 356 | 指标卡 + 历史表 + 空态 + 首次使用路径链接 | 后端仅只读统计；无写入口（设计如此） | 历史上限 20 条且进程重启清零 (OverviewPage.vue:71) | P2 | 暂不修 |
| 转换 (ConvertPage.vue) | 不配来源直接贴链接转出 | POST /api/convert (159); GET /api/targets (156); GET /api/pipelines (151); POST /api/diff (160) | convert 364; targets 361; pipelines 357; diff 365 | target 下拉、URL/文本框、types/include/exclude、pipeline 下拉、list 开关、sort 下拉、转换/复制/下载、diff | ConvertRequest.config 字段存在 (endpoints.ts:148) 但 UI 硬编码 '' (ConvertPage.vue:117) → 手动转换不能选 config；pipeline 已下拉 (234) | types/include/exclude 逗号手填 (231-233) 可接受；config 缺失 | P2 | 新卡：转换页补 config 选择 |
| 管线 (PipelinesPage.vue) | 创建/编辑处理步骤 | GET/POST /api/pipelines (151-152); GET/PUT/DELETE /api/pipelines/{id} (153-155) | pipelines 357; createPipeline 358; updatePipeline 359; deletePipeline 360 | 表格、新建/编辑 dialog、步骤 builder、op 下拉、by 下拉、ref combobox、on_error 下拉 | 步骤参数除 by/ref 外均为 text-field (283-290)；ref 已下拉 (272-282) | 模板管线只读 (237-238) | P2 | 新卡：步骤参数下拉化 |
| 脚本 (ScriptsPage.vue) | 编写/试运行 JS 脚本 | GET/POST /api/scripts (220-221); POST /api/scripts/test (223); GET/PUT/DELETE /api/scripts/{id} (224-226) | listScripts 393; create 394; get 395; update 396; delete 397; test 398 | 列表、新建/编辑 dialog、代码框、类型下拉、试运行 dialog | 无新增入口缺口；沙箱提示 (157) | kind 仅 filter/operate (208) | P2 | 暂不修 |
| 机场 (AirportsPage.vue) | 新增机场→刷新→导入规则 | /api/airports CRUD (164-167); refresh/import/check (167-175) | list/create/update/delete 367-370; refreshAll 423; refreshOne 425; importRules 427; check 429; lastCheck 431 | 表格、新增/编辑 dialog、刷新全部、测活全部、行级刷新/导入/测活 | withRules 开关 (252) 仅影响 refreshAll (187)；行级 refresh/import 独立 (142-159) → 语义不自明 (F6) | 启用状态用圆点而非开关 (265-268) | P1 | F6：文案/布局澄清（建议新卡） |
| 自建节点 (SelfNodesPage.vue) | 手填分享链接作为节点 | /api/self-nodes CRUD (205-213) | list/create/update/delete 388-391 | ResourcePage 通用表单 | 后端无链接校验 → 无效链接静默不出现 (SelfNodesPage.vue:37-41) | 无报错提示，需去概览看节点数 | P1 | 新卡：链接校验/错误反馈 |
| 规则 (RulesPage.vue) | 规则行/规则集/候选/建议 | /api/rules CRUD+bulk (187-196); /api/rule-providers GET (236); /api/suggestions (237-240) | listRules 377; create 378; update 379; delete 380; bulk 405; listRuleProviders 403; suggestions 407-410 | tabs、ResourcePage、规则集列表、候选勾选、建议卡片 | rule_config 填分组 id 而非下拉 (RulesPage.vue:168)；候选仅前端筛选 (206) | 目标分组 id 与分组页 id 对不上手 | P2 | 新卡：rule_config 改下拉 |
| 规则源 (RuleSourcesPage.vue) | 导入外部 INI 规则源 | GET (241); POST init (242); POST update (243); POST rollback (244); POST mrs (245)；**无 POST/DELETE/单源 update** (F1) | list 411; updateRuleSources 413; rollback 415; generateMRS 417; init 455；**无 create/delete/single update** (F1) | manifest 卡、源表格、行内"更新" (199)、init/rollback/mrs dialog | F1 双缺；F2 行内更新调全量 (199)；F7 暴露 manifest 路径 (162-164) | INI 源需手编 manifest.yaml | P0 | T1/T3 修复中 |
| 分组 (GroupsPage.vue) | 定义策略组与归组 | /api/groups CRUD (178-186) | list/create/update/delete 372-375 | ResourcePage + 配方折叠 | 无缺口 | rule_config 依赖分组 id (GroupsPage.vue:40) | P2 | 暂不修 |
| 订阅 (SubscriptionsPage.vue) | 创建令牌→拿到订阅 URL | /api/tokens CRUD (197-204); /api/targets (156); /api/traffic/* (214-219); /api/airports/refresh (423) | listTokens 382; create 383; update 384; delete 386; targets 361; strategy 434/436; preview 438; refreshAll 423 | 令牌选择/URL、令牌表、创建/编辑 dialog、流量策略、余额刷新 | F3 pipeline/config 手填 (361-362)；target 已下拉 (347-356) | 高级选项折叠 (357-367) | P0 | T4/T5 修复中 |
| 任务 (TasksPage.vue) | 查看调度执行记录 | GET /api/tasks (228-230) | listTasks 401 | 任务表、调度器不可用态 | 仅内存态、重启清空；cron 不可 UI 编辑 (72-73)；无手动触发 | 失败自动重试 3 次 (65) | P2 | 暂不修 |
| 设置 (SettingsPage.vue) | 调整全局设置/主题 | GET/PUT /api/settings (246-249); GET /api/targets (156); GET /api/stats (149) | listSettings 420; updateSettings 421; targets 361; stats 355 | 设置行 (switch/text)、保存/重置、目标格式列表、历史条数、主题按钮 | 暴露内部 setting key (82) 与 manifest 相关概念 (F7) | 清空=恢复默认 (105) | P2 | F7：隐藏内部 key/manifest |
| 日志 (LogsPage.vue) | 排查错误 | GET /api/logs (158) | logs 363 | 日志列表、自动刷新、清屏、滚动 | 无级别过滤、200 行上限 (93-95)；自动刷新需手动关 (95) | 关键字高亮仅前端 (25-28) | P2 | 暂不修 |

### 全局层

- v-switch 未选中态 track 与背景无对比：VSwitch.css:25-27（未选中 track-true opacity 0）+ VSwitch.css:36-48（track 背景 surface-variant opacity .6）；tokens.css:31 --sf-bg #F0F2E6 近同色；vuetify-overrides.css 无 v-switch 覆写 → F4 → T2 修复中。
- v-dialog 高内容顶部裁切：VDialog.css:1-5 居中布局 (align-items: center; margin: auto)；内容超视口时顶部被裁；VDialog.css:7 已有 max-height calc(100%-48px) 但仍因居中裁切顶部 → F5 → T2 修复中（证据较计划 §3 有修正：line 6/7 已设 max-height，真正根因是居中）。
- 空态：各页 AsyncSection empty-state 教学文案（如 LogsPage.vue:116、OverviewPage.vue:120-124）符合 progressive disclosure；Airport 启用状态用圆点 (AirportsPage.vue:265-268) 非开关，语义弱。
- 文案/help：PageHeader.vue:20 HelpPanel 入口；help.ts 提供 pageHelp/fieldHelp；ScriptsPage.vue:207 fieldHelp('script.name') 示例。

## 二、新手关键链路走查 L1-L7

- L1 首次打开概览：畅通。OverviewPage.vue:55-64 "首次使用路径" 四步链接；空态 history 提示 (120-124)。断点：无。
- L2 转换页无来源直接贴链接：畅通。ConvertPage.vue:100-127 submit 校验 URL/文本非空 (101)；target 默认 '' (24) 由后端 /api/targets 提供 (92)。断点：config 未暴露 (117)。
- L3 机场新增→刷新→"刷新时同时导入规则"：卡点 (F6)。AirportsPage.vue:252 开关仅驱动 refreshAll (187)；行级"导入规则" (298) 与"刷新流量" (297) 分离；新手分不清开关影响范围。
- L4 规则源从零到可用：卡点 (F1)。RuleSourcesPage.vue 无新增/删除入口；需手编 manifest.yaml；init 生成默认清单 (59-78)。修复中 T1/T3。
- L5 令牌创建→订阅 URL 可用：部分畅通。SubscriptionsPage.vue:323 新建令牌；target 下拉 (347-356)；pipeline/config 手填 (361-362) (F3)。修复中 T4/T5。
- L6 输出验证闭环：部分畅通。Overview 历史展开 failures/warnings (127-147)；Tasks 页内存态重启清空 (TasksPage.vue:64)。断点：无持久化历史。
- L7 失败路径可见性：部分畅通。Convert 失败列表 (259-264)；Logs 关键字高亮 (25-28)；subscheck 结果 (332-340)。断点：日志无级别过滤 (LogsPage.vue:93)。

## 三、Findings 全量清单

- F1 规则源无新增/删除入口（前后端双缺）：server.go:241-245 仅 GET/init/update/rollback/mrs；endpoints.ts:411-418 无 create/delete/single update；RuleSourcesPage.vue 无入口。状态：修复中 T1/T3。
- F2 RuleSourcesPage 行内"更新"调全量接口：RuleSourcesPage.vue:199 @click="updateAll"。状态：修复中 T3。
- F3 令牌 dialog pipeline/config 手填：SubscriptionsPage.vue:361-362 v-text-field；ConvertPage.vue:92,234 已有下拉模式 (endpoints.pipelines(''))。状态：修复中 T4/T5。
- F4 v-switch 未选中态与背景无对比：VSwitch.css:25-27 + :36-48；tokens.css:31 --sf-bg #F0F2E6；vuetify-overrides.css 无覆写。状态：修复中 T2。
- F5 v-dialog 高内容顶部裁切：VDialog.css:1-5 居中布局（证据较计划 §3 修正：line 7 已有 max-height，根因是居中）。状态：修复中 T2。
- F6 机场页 withRules 开关与行内导入规则关系不自明：AirportsPage.vue:252, 187, 298。状态：建议新卡（P1）。
- F7 设置页暴露内部概念：SettingsPage.vue:82 setting-key code；RuleSourcesPage.vue:162-164 manifest 路径。状态：建议新卡（P2）。
- F8 自建节点无效链接静默不出现：SelfNodesPage.vue:37-41。状态：建议新卡（P1）。
- F9 规则集 providers 只读无启停：server.go:236 仅 GET /api/rule-providers；endpoints.ts:403。状态：建议新卡（P2）。
- F10 规则 rule_config 填分组 id 非下拉：RulesPage.vue:168。状态：建议新卡（P2）。
- F11 手动转换 config 未暴露：ConvertPage.vue:117。状态：建议新卡（P2）。
- F12 管线步骤参数 text-only：PipelinesPage.vue:283-290（by/ref 已下拉 272-282）。状态：建议新卡（P2）。
- F13 日志无级别过滤与 200 行上限：LogsPage.vue:93-95。状态：建议新卡（P2）。
- F14 任务页 cron 不可 UI 编辑/无手动触发（设计如此）：TasksPage.vue:72-73。状态：暂不修。
- F15 机场启用状态用圆点非开关：AirportsPage.vue:265-268。状态：建议新卡（P3）。

## 四、修复中标注（避免重复派工）

- F1/F2 → T1/T3（后端写 API + 前端 UI + 单源更新）
- F3 → T4/T5（后端 GET /api/configs + 前端下拉化）
- F4/F5 → T2（CSS 对比度 + dialog 溢出）
- F6/F8/F10/F11/F12/F13/F7/F15 → 建议新增卡片（P1/P2），由 Lead 决定派工

## 五、修复建议清单（卡片级，不写代码）

1. 标题：机场页"刷新时同时导入规则"语义澄清 · 写范围：web/src/pages/AirportsPage.vue · 验收：开关文案注明仅影响"刷新全部流量"；行级导入按钮独立；vitest 绿。
2. 标题：自建节点链接校验与错误反馈 · 写范围：web/src/pages/SelfNodesPage.vue + internal/api/self_nodes*.go · 验收：无效链接返回 400 并前端显示；单测覆盖。
3. 标题：规则 rule_config 改为分组下拉 · 写范围：web/src/pages/RulesPage.vue + endpoints.ts · 验收：下拉取 /api/groups name；失败退化手填。
4. 标题：转换页暴露 config 选择 · 写范围：web/src/pages/ConvertPage.vue · 验收：config 下拉取 /api/configs（T5）。
5. 标题：管线步骤参数下拉化 · 写范围：web/src/pages/PipelinesPage.vue · 验收：filter.regex 等参数提供常用选项；保留手输。
6. 标题：日志级别/过滤与导出 · 写范围：web/src/pages/LogsPage.vue · 验收：前端关键字过滤；导出按钮。
7. 标题：设置页隐藏内部 key/manifest · 写范围：web/src/pages/SettingsPage.vue + RuleSourcesPage.vue · 验收：setting key 默认折叠；manifest 路径移入高级区。
