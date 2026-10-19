# T6 · 全站能力对照矩阵（设计-后端-WEB，新手视角走查）

## 0. 你是谁、红线是什么
你是 SubForge 审计队员（只读审计）。三条红线：
1. 唯一可写文件：docs/design/ux-capability-matrix.md（新建）。repo 其他文件一律只读，一个字符都不许改。
2. 不许 git commit / git add；不许新建任务卡。
3. 每条 finding 必须带证据（文件:行号 + 一句话），不许凭感觉。

## 1. 进度心跳（必须执行）
send_message 发给 lead，共 4 次：
- 心跳1「T6 book read, starting」
- 心跳2「T6 pages 1-6 walked」
- 心跳3「T6 matrix draft written」
- 心跳4「T6 done」+ 矩阵文件绝对路径

## 2. WHY
用户原话：「就完全不考虑用户小白吗，逻辑一团糟，重走整个项目逻辑」「给我好好排查一下设计-后端-WEB 的能力对照」。需要一份全站 12 页的能力对照矩阵，把「后端有但前端没入口」「前端入口但后端不支持」「小白看不懂」三类缺口全部列出来。

## 3. 背景材料（先读）
- docs/design/ux-audit-plan.md（已落盘：矩阵列定义、种子 findings F1-F7、新手链路 L1-L7）。
- .impeccable.md（设计红线：progressive disclosure、空状态 teach、错误消息 actionable）。
- web/src/router/index.ts:16-30（12 页清单与分组）。

## 4. 已确认的种子 findings（在矩阵里要出现，逐条复核证据行号是否仍准确）
- F1 规则源无新增/删除入口（前后端双缺；注意另一队员正在补 T1/T3——矩阵里标注「修复中(T1/T3)」）
- F2 RuleSourcesPage 行内更新按钮接的是全量接口（标注「修复中(T3)」）
- F3 令牌 pipeline/config 手填（标注「修复中(T4/T5)」）
- F4 v-switch 未选中态与背景无对比（标注「修复中(T2)」）
- F5 v-dialog 高内容顶部裁切（标注「修复中(T2)」）
- F6 机场页「刷新时同时导入规则」开关与行内「导入规则」的关系对新手不自明
- F7 设置页暴露 manifest 路径等内部概念

## 5. 产出物结构（docs/design/ux-capability-matrix.md）
```markdown
# SubForge 全站能力对照矩阵（设计-后端-WEB）
> 生成：T6 审计 · 基准 HEAD 31c280b · 视角：第一次用的小白

## 一、页面 × 能力矩阵
| 页面 | 核心能力 | 后端 API | WEB 入口 | 小白可用性(1-5) | 缺口(证据) |
|---|---|---|---|---|---|
| 概览 | … | … | … | … | … |
（12 页全覆盖：概览/转换/管线/脚本/机场/自建节点/规则/规则源/分组/订阅/任务/设置/日志——以 router/index.ts 为准）

## 二、新手关键链路走查 L1-L7
L1 首次进入→加第一个机场；L2 机场→节点产出；L3 节点→规则；L4 规则源从零到可用；L5 令牌创建→订阅 URL 可用；L6 转换页手动转换；L7 日常维护（更新/回滚/排错）。
每条链路：步骤 → 每步是否自明 → 断点(证据 文件:行号) → 建议。

## 三、Findings 全量清单
F1…F7 复核 + 新发现 F8+（每条：现象/证据 文件:行号/影响/修复方向/状态）

## 四、修复中标注
正在由 T1-T5 修复的项列出对应任务号，避免重复派工。
```

## 6. 走查方法（不许跳）
- 每页：读 web/src/pages/<Page>.vue 的 template（入口、按钮、空态、提示）+ endpoints.ts 对应函数 + internal/api/server.go 路由对照。
- 后端有没有但前端没有入口的：grep server.go 的 r.Get/r.Post 全列表 vs endpoints.ts 全列表对齐。
- 小白视角：每页问三个问题——「我第一步该点哪？」「空状态告诉我要做什么了吗？」「出错提示告诉我下一步吗？」

## 7. 自测（心跳4 前）
- 矩阵 12 页齐全、每行有证据 file:line
- F1-F7 全部出现且标注修复任务号
- 文件能正常渲染（markdown 表格管道符完整）

## 8. 完成报告格式（心跳4 一起发）
- docs/design/ux-capability-matrix.md 绝对路径 + 新发现 findings 数量 + 最严重的 3 条一句话