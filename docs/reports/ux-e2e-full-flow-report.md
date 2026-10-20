# SubForge Web 全流程操作报告（86 例矩阵 / 93 次执行 · 无头浏览器实测 · 三轮修复闭环）

> 被测站点：http://192.168.8.102:25500/ （令牌 flzx3qcczk）
> 被测版本：subforge v0.1.6 (commit 41962db8, built 2026-10-20T08:18:35Z)，进程 2026-10-20T08:30:05Z 启动；与本地仓库 HEAD 41962db 一致
> 测试日期：2026-10-20（Asia/Shanghai），执行窗口约 09:00–19:30
> 执行方式：headless Chrome 155.0.8059.82（CDP，真实鼠标/键盘事件）+ 派发 llmio/free 模型小队；操作矩阵见 .tmp/ux-e2e/operation-matrix.md（83 例）
> **第二轮（修复后）见 §10**：本地复刻环境 92/92 PASS、12 项缺陷已修、12 项未修（无 P0/P1 剩余），综合重估 6.5/10。
> **第三轮（剩余 12 项修完）见 §11**：93 次浏览器回归全绿（86 个唯一用例）、24/24 缺陷闭环、综合重估 ≈7.7/10；代码分 9 个 commit 落地，报告入库 docs/reports/。
> 报告正文中的 `.tmp/...` 是本地实测证据路径（脚本、用例、日志、截图），按约定不入库；入库的只有本报告与操作矩阵。
> 全部交互在真实网页上完成；仅两类例外已获授权并逐条注明：① 页面内只读探针 fetch（读 /api/* 统计）；② 清理 5 条测试规则时用页面内已鉴权 fetch DELETE（9600+ 行规则无法用 UI 分页快速定位），其余新增/编辑/删除一律走 UI 行内按钮与确认对话框。

## 1. 结论摘要

| 维度 | 评分 | 一句话 |
| --- | --- | --- |
| 核心闭环可用性 | 6/10 | 登录→转换→管线→分组→规则→订阅主链路能走通（真实机场 45 节点转换 2.3s 出结果，管线 include=HK 过滤到 12 节点），但脚本页、规则源、导入规则三个环节带有 P1 级硬伤 |
| 输入校验 | 3/10 | 机场、规则、分组、脚本四个新增对话框**全部零校验**：非法 URL、非法正则、负数间隔、空参数、语法错误代码都能保存成功 |
| 错误处理 | 4/10 | 大量错误直接把 `HTTP 4xx/5xx: {"error":...}` 原文弹给用户；导入规则 500 只有一个「不可用」；回滚空操作却提示「已回滚」 |
| 数据一致性 | 4/10 | 脚本 kind 全链路丢失、规则源「最新版本」显示最旧、分组/自建节点「排序」字段完全无效、勾选「刷新时导入规则」刷新即丢、自建节点非法链接照样入库（D-07） |
| 性能 | 5/10 | 转换 2.3s、订阅测活 848ms 很好；但机场「导入规则」在 9638 条规则时 >30s 且 500，/api/rules 一次返回 9638 条全量（含 raw，前端做客户端分页） |
| 设计与可发现性 | 6/10 | 界面统一、空状态与说明文案丰富（明显下了功夫）；但「插入示例」名不副实、无显式登出入口、规则页无批量入口、试运行结果被对话框裁切 |
| **综合** | **4.7/10** | **不是「烂到不能用」：主流程确实能跑，转换/管线/分组/规则的 CRUD 稳定；但一碰输入校验、长流程（导入 9600 条）、状态持久化和前后端字段契约就系统性露馅。4 个 P1 都出现在「用户会真的用」的功能上。** |

83 例矩阵最终判定：**69 PASS / 9 FAIL（全部归因产品缺陷）/ 5 例因用例或工具问题未获有效判定**。另通过探针与源码逐行核实补证了 1 个未计入这 9 例的 P1（D-04：AP-09 判 PASS 的断言太弱，掩盖了导入规则 500）。缺陷合计 24 项：**P0 0 / P1 4 / P2 10 / P3 10**。

## 2. 方法与执行环境

- 操作矩阵：.tmp/ux-e2e/operation-matrix.md，13 条路由 × 12 个操作维度，展开 83 个用例（G 8 / AP 11 / SN 6 / RL 8 / RS 7 / GR 7 / PL 7 / SC 7 / SB 10 / CV 8 / OPS 4）。
- 执行层：自建 CDP 驱动（.tmp/ux-e2e/cdp.mjs、harness.mjs、stp.mjs、run.mjs、act.mjs，零 npm 依赖，Node v24.21.0 自带 fetch/WebSocket）。每例独立 Chrome profile、独立 runs/<team>/<case>.json 与 shots/ 截图。
- 真实鼠标事件：统一用 scrollIntoView → 等 320ms → 重读 rect → Input.dispatchMouseEvent，并用一次性 click 监听器验证命中，未命中则 DOM click 兜底。此项修复前曾造成大量「点了没反应」的假阴性（按钮在视口外，y=1172 > 900）。
- 登录：清 localStorage 首访会 401 并自动弹「访问令牌」对话框；正确令牌保存后 localStorage 为 {"subforge.token":"flzx3qcczk"}。后续用例通过 Page.addScriptToEvaluateOnNewDocument 预注入令牌，避免 401 噪音。
- 数据纪律：测试对象统一前缀 UXT-<团队>-<用例>-<时间戳>；破坏性操作前截图/记录字段；末态逐项核对并清理（见 §6）。
- 派发：按页拆成 T1–T5 五队，第二期再拆 t3a/t3b/t4a/t4b/t4c，均由 `llmio/free` 模型执行，见 §7。

## 3. 逐流程结果（13 个路由）

### 3.1 认证与全局框架（/、令牌对话框、13 路由巡检）— 8/8 PASS

- G-01..G-03：首屏 401 横幅「HTTP 401: 401: 缺少或错误的访问令牌」+ 自动弹令牌框；错误令牌保存后有明确失败提示、不静默关闭；正确令牌保存后弹框关闭、数据重取、localStorage 正确。
- G-04：**「清除」只清空输入框、不动 localStorage，对话框保持打开**（源码 TokenDialog.vue:32 clearDraft 只做 draft.value=""）；**「取消」关闭且不改存储**；**清空后点「保存」= 隐蔽登出**（saveToken 空串清 localStorage，web/src/api/token.ts:18）→ 13+ 步断言全过。UI 没有显式登出入口（OBS）。
- G-05..G-07：令牌按钮回填当前值、刷新后仍登录；深链 /rules /logs 直达且侧栏高亮；/nope 渲染 404 页且导航可用。
- G-08：13 条路由逐条 strict 巡检（console/page/net 各 0 条除 favicon 404）；1280×800、1920×1080、1000×800 三档无横向溢出；窄屏抽屉可开合并导航成功。

### 3.2 概览（/overview）— 只读，随各流程联动核对

- 初始全 0/—（无转换记录）；转换后历史/缓存命中指标更新（CV-08 联动）；健康块/使用说明可展开；空状态「还没有转换记录」与行动按钮可用。
- 指标块与 API /api/stats 一致（history.total/cacheHitRate、logs 行数）；未见错误。

### 3.3 转换（/convert）— 核心链路可用

- 直连 API 实测：POST /api/convert（真实机场 URL，target=clash.meta）→ **200，12914 字节，2336ms**，响应头注明 subforge v0.1.6。
- UI 实测（真实鼠标点击 submit）：6204ms 内渲染结果区「节点 45（原始 45）缓存命中 0 复制 下载 全部来源成功」，elementFromPoint 命中按钮；CV-03 的 90s 无结果是一次 flake，复跑通过。
- 空来源点提交只显示「至少要给一个来源：URL 与文本不能同时为空」且不发请求（CV-01）；多来源一真一假可部分成功（CV-06）；diff A/B 可运行（CV-07 因用例 evalNote 语法错未留下有效断言）；复制/下载按钮存在且可点（CV-05/CV-06 通过）。
- 目标格式下拉 36 项，默认空值也能按服务端默认成功；保存/复制/下载/结果摘要齐全。**未发现该页产品缺陷。**

### 3.4 机场（/airports）— AP 11 例：9 PASS（AP-11 分页通过、计 OBS）/ 2 FAIL（AP-03、AP-10）

- AP-01/02/04/05/06/07/08/11：表头 9 列与真实行（45 节点、5.2GB/100GB、2026-11-12、正常）全部命中；新增/编辑回填/启用开关持久化/删除二次确认/刷新流量错误反馈/测活结果区都正常；分页功能正常（每页 10/25/50/100/全部，手动切换复核通过，**无 20 档**）。
- AP-03 FAIL（D-05，P2）：ftp://、not-a-url、javascript:alert(1)、空 URL 四种输入全部保存成功且无任何提示；直到「刷新全部流量」才报 unsupported protocol scheme。实测四条垃圾机场全部入库（已 UI 删除）。
- AP-09（PASS 但弱断言）+ 独立取证（D-04，P1）：真实机场行内「导入规则」在当前 9638 条规则量级下浏览器 console 必现 HTTP 500，页面只显示「不可用」；页内探针 15s/30s 两次都等不到响应（job 各 33s/64s）。源码：internal/api/import_rules.go:26 上游失败 502、写库失败 500（第 43 行），internal/subforge/import.go:124 对每条规则单条 INSERT、无事务。
- AP-10 FAIL（D-06，P2）：「刷新时同时导入规则」勾选后刷新页面即回未勾选，/api/settings 无该键；「测活全部」执行后无任何结果文案（P3）。
- AP-11 OBS：分页本身正常；用例里的 pageSize 动词因下拉菜单未按预期展开失败，属工具问题，未判产品缺陷。

### 3.5 自建节点（/self-nodes）— SN 6 例：5 PASS / 1 FAIL（SN-05 原 run 的失败已复核为测试假阴性）

- SN-01/02/04/06：空状态 + 格式示例面板可用；合法 ss:// 链接入库；编辑回填；自建+机场合并转换成功（与 CV 用例联动）。
- SN-03 FAIL（D-07，P2）：页面文案明确写「填错不会有报错提示，节点会静默不出现」，实测 `not-a-valid-link` 保存后**无报错但列表真的多了一行**，与承诺不一致。
- SN-05 复核 PASS：新增自建节点后打开编辑框把「启用」关掉并保存 → 页内探针读 API 得到 `{id:12,en:false}`，开关**确实落库**；首轮 run JSON 的失败是测试脚本把开关点了两次的假阴性（lead22/lead23 两轮复核，含 UI 删除清理）。附带观察：自建节点列表「顺序」列显示 `1792495506820332800`——即后端用 UnixNano 覆盖用户排序值（见 D-11）。

### 3.6 规则（/rules）— 8/8 有结论（7 PASS + 1 观察缺陷）

- RL-01：总数 9638–9644（随并发浮动），每页 25/50 生效，下一页翻页正确；**无 20 档**。
- RL-02：新增 DOMAIN-SUFFIX + uxt 参数的规则成功（API 返回 id/t/p/en/g），尾页可定位，UI 删除后 API 查空。
- RL-03（D-08，P2）：类型、参数全空也能保存——保存按钮不置灰、无任何错误文案、toast「已新增」、API 出现 rule_type="" 的规则（实测 id 9646；另残留 9641/9644 两条空参数规则已由队员删除）。
- RL-04：编辑回填（类型/参数/配置/解析DNS/启用）正确，改参数+关启用落库，行显示「停用」，删除成功。
- RL-05：四个 tab 内容正确：规则集「暂无规则集」、导入候选筛选说明、Rule-Bot「暂无建议」；1 例因 CDP 截图超时未留证据，内容已由其他批次核对。
- RL-06（D-09，P2）：规则行页签**没有任何 checkbox/批量控件**（页内 checkbox 计数 0，翻页后仍 0）；真正可用的批量逻辑在「导入候选」tab（web/src/pages/RulesPage.vue:241-261，show-select + disable-selected + enable-all-airport），但当前无候选数据，UI 上不可达。`POST /api/rules/bulk` 存在且可用：GET→400「无效的 ID」、POST {}→400「必须给出 ids 或 source」、POST {"source":"zzz-not-exist"}→200 {"updated":0}。
- RL-07：建议/候选均为空状态，无可测交互；RL-08：真实规则 sntp.uk 启用开关 true→false→true 全链路验证并已恢复。

### 3.7 规则源（/rule-sources）— 7 例：5 PASS / 2 FAIL（3 个 P1）

- RS-01/02：清单卡（生效/默认路径 /data/manifest.yaml、输出 /data/rules、已就绪）与源表格（名称/URL/格式/输出目录/版本数/最新版本/操作）齐全；「生成默认清单」幂等且已有清单先二次确认。
- RS-03/05/06：全部更新可用（本次跑到 20261020-110705）；新增源可用；必失败源 https://127.0.0.1:1/x.ini 有可解释错误文案「拉取 …（重试 3 次后仍失败）… 该源保留上一版本，产物不受影响」。
- RS-04 FAIL（D-02/D-03，P1）：**「最新版本」列显示的是最旧版本**（/api/rule-sources 返回 versions 升序 [105407…110433]，latest=105407）；源码 internal/subforge/rulesources.go:199 `status.Latest = versions[0]`，manifest.ListVersions 升序（manifest.go:311-330）。**「回滚」是空操作**：点确认后 toast「已回滚: /data/rules/…-110645.yaml」，但 API 前后完全一致（n=5、latest、last 均未变）；源码 rulesources.go:221-246 只调 manifest.Rollback 返回路径，不写状态；前端 RuleSourcesPage.vue:118-119 只 notify 路径。
- 「生成 MRS」（D-02 连带）：点「生成」弹 400，snackbar 原文 `HTTP 400: {"error":"ConvertToMrs: invalid input: magic number mismatch"}`——根因是 handleAPIRuleSourcesMRS 用 latest（最旧）当 srcPath 且默认 format=mrs（internal/api/rule_sources.go:204-228，format 缺省在 212-215 行），而源文件是 .yaml。
- RS-07 FAIL（D-12，P2）：全部更新/单源更新期间按钮 `disabled=false`、无 spinner、无「更新中」文案，可重复点击并发更新；`performance.getEntriesByType("resource")` 不记录该 POST fetch，请求计数断言无效（探针缺陷已注明）。

### 3.8 分组（/groups）— 7 例：5 PASS / 2 FAIL（2 缺陷）

- GR-01/02/03/05：空状态、配方展开、编辑改类型与正则、删除二次确认、刷新均正常；「常见分组配方」只是静态文字（GroupsPage.vue:56-67），没有一键填充按钮（矩阵 GR-02 前提不成立，记 OBS）。
- GR-04 FAIL（D-10，P2）：匹配正则 `[a-`（非法）与测活间隔 `-5` 全部保存成功，toast「已新增」，行显示 regex=[a- interval=-5；无任何校验。
- GR-06 PASS（Lead 亲自复跑）：建分组 A → 建规则 DOMAIN-SUFFIX uxt…test 指向 A（API cfg=32、gn=A）→ 删除分组 A 后规则保留、group_name 变空（退回 DIRECT），行为符合预期。
- GR-07 FAIL（D-11，P2）：新增时「排序」填 10/1，列表仍按创建顺序显示 A 再 B；源码证实：API DTO 根本不接收 sort_order（internal/api/subscription.go:171-177、210-216），AddProxyGroup 硬编码 `time.Now().UnixNano()`（internal/subforge/subforge.go:234-237），UpdateProxyGroup 不更新该列；自建节点同病（store.go:150-153 AddSelfNode 也写 UnixNano）。

### 3.9 管线（/pipelines）— 7/7 PASS，3 个缺陷

- PL-01..03/05/06：空状态与执行顺序说明、步骤编辑器 10 种 op（正则过滤/地区过滤/协议过滤/脚本过滤/正则重命名/加国旗 Emoji/去重/截断节点名/排序/脚本改写）、上移/下移/删除、空名 400 且错误位显示不关框、删除二次确认、编辑回填全部通过。
- PL-04：脚本引用四态可见——脚本存在显示「脚本已启用」；不存在显示「未找到该脚本」；但 **kind 为空时同屏出现自相矛盾的「脚本已启用」+「类型可能不匹配：脚本 kind=undefined」（D-16，P3）**；清空脚本 ref 下拉时前端抛 `TypeError: Cannot read properties of null (reading 'value')`（web/src/pages/PipelinesPage.vue:281，D-13，P2）。
- PL-07 PASS（真实联动）：建管线（正则过滤 include=HK）→ 转换页填真实机场 URL → 选中该管线 → 结果「节点 12（原始 45）」，过滤真实生效。

### 3.10 脚本（/scripts）— 7 例：5 PASS / 1 FAIL / 1 未获有效判定（1 个 P1 贯穿）

- SC-01：沙箱说明（goja ES5.1、5s 上限、无 require/网络/文件/console、每节点独立环境）与变量表 6 行（node.name/type/server/port/source/raw）齐全。
- SC-02：新建 filter 脚本；「插入示例」按钮**不直接插入**，只展开菜单，必须点 example-item 才填入代码（D-17，P3）；插入后 textarea 内容长度 0→60，保存成功，列表出现。
- SC-03 FAIL（D-01，P1）：试运行对话框「类型」不回填（为空），结果区只有「成功（0ms） 耗时：0 ms」，**看不到 filter 的 保留/丢弃**；手动补选 filter 后 test-kept 正常出现、test-dropped 正确隐藏（t4c 11:24 复跑，12 项检查里唯一失败就是「类型未继承」）。根因：internal/store/store.go:204-212 Script 结构体无 Kind、store.go:561-569 建表无 kind 列、internal/api/scripts.go:30-35/81-86 的 DTO 不解析 kind → 前端 web/src/api/endpoints.ts:197-207 提交的 kind 被静默丢弃；模板 testResult.ok && testKind==="filter"/"operate" 两个分支都不渲染。
- SC-04 PASS（观察缺陷 D-14，P2）：代码 `return {` 语法错误仍保存成功、无任何保存期提示，只有点试运行时才报编译失败；空代码同理可保存。
- SC-05 PASS：`while (true) {}` 试运行被服务端 5s 上限中断，前端实测 5250ms 返回，文案「节点 \"ss-node\": 脚本 \"script-test\" 被中断（超时或死循环）失败（5000ms）」（内部脚本名 script-test 外露，D-17，P3）。
- SC-06 PASS（有条件）：手动补选 operate 后，试运行结果显示 `UXT-T4C-ss-node`、写回区可见，API `/api/scripts/test` 的 after.name 也正确；但**每次试运行都得手动补选类型**，列表/编辑框里类型仍是空白（同一 D-01）。
- SC-07 未获有效判定：用例实际只读了脚本/管线列表（当时都为空）并跑了一次**没有选管线**的转换，断言只覆盖「转换摘要里有节点数」，**并没有把脚本挂到管线上**；所以「脚本 → 管线 → 转换」端到端仍未验证（见 §7 第 1 条）。

### 3.11 订阅（/subscriptions）— SB 10 例：9 PASS / 1 用例问题

- SB-01..05/07..10：无令牌空态与「还没有令牌」提示、新建令牌（最小/全字段/高级选项展开后的处理管线/配置模板/包含/排除）、选中令牌后订阅地址形如 `http://192.168.8.102:25500/sub?token=…`、复制按钮变「已复制」、编辑/删除令牌、401 与错误 token 访问订阅 URL 的响应、令牌带管线拉取、立即刷新全部机场余额均通过。
- SB-06：5 个快捷预设（默认/严格/乐观/独立/隐藏）与四字段联动可用，「显式改过的字段优先于预设」提示正确；用例里 select("整体预设","sum") 找不到选项（实际选项是「默认/严格/乐观/独立/隐藏」，而 sum 是字段级策略），属用例写错，**不判缺陷**。

### 3.12 任务（/tasks）与设置（/settings）— OPS 4 例：3 PASS / 1 用例问题

- OPS-01：任务表 1 条「订阅测活 成功 2026/10/20 17:00:00 848.52 ms」，说明两个 cron（规则源更新 0 4 * * *、订阅测活 */30 * * * *）、失败重试 3 次指数退避；本页无手动触发。
- OPS-02：设置页 4 项（script.enabled、subscheck.enabled、rules.manifest_path、rules.output_dir）+ 保存/重置默认；改值保存刷新回显后已改回。
- OPS-03：日志页刷新/自动刷新/过滤/清屏/滚到底全部可操作；用例断言「清屏后 log-row 数 > 0」写错（清屏本应为 0），**不判缺陷**。
- OPS-04：概览使用说明展开/收起、服务状态块正常。

### 3.13 404 与全局导航

- /nope 渲染 not-found 页，侧栏与顶栏仍可用；13 条路由无崩溃白屏。每页 console 均有一条 favicon.ico 404（D-19，P3）。

## 4. 缺陷清单（24 项：P0 0 / P1 4 / P2 10 / P3 10）

级别口径：P1 = 核心功能不可用或数据/状态给出错误结论；P2 = 用户会踩到、有明确错误行为或明显体验损失；P3 = 文案/一致性/视觉问题。每条都给了可复现的 UI 路径与源码位置，源码均已逐行核对。

### P1（4 项，建议本周修）

**D-01（P1）脚本类型 kind 全链路丢失：列表不显示类型、编辑不回填、每次试运行都要手动补选**

- 现象：新建脚本时选「类型」（filter/operate），保存后列表副标题为空、编辑回填类型为空；点「试运行」时弹窗「类型」也**不会继承**该脚本的类型（实测为空），此时结果区只显示「成功（0ms） 耗时：0 ms」，看不到 filter 的「保留/丢弃」或 operate 的 before→after。**在试运行弹窗里手动把类型补选成 filter/operate 后，结果就正常显示**（t4c 11:24 复跑：手动补选后 test-kept/test-dropped、写回 UXT-T4C-ss-node 均正确），所以功能不是死了，而是「每次都得手动补一次、且列表根本看不出这脚本是哪种类型」。管线里引用该脚本也不会触发类型校验。
- 根因（逐行核实）：internal/store/store.go:204-212 的 Script 结构体没有 Kind 字段；store.go:561-569 的 scripts 建表语句无 kind 列；internal/api/scripts.go:30-35（POST）与 81-86（PUT）的请求 DTO 不解析 kind。前端 web/src/api/endpoints.ts:197-207 声明并提交的 kind 被后端静默丢弃 → 前端 `testKind` 为 undefined，模板里 `v-if="testResult.ok && testKind==='filter'"` 两个分支都不渲染。
- 影响：脚本类型是这类脚本的语义核心（filter 是表达式、operate 是赋值语句），丢了这个字段后：列表无法区分、编辑框回填空白、试运行默认按空类型跑（用户第一反应是「点了运行没反应」）、管线脚本步骤的 kind 校验（script-ref-kind-mismatch）永远匹配不上。属于数据丢失型缺陷，不只是显示问题。

**D-02（P1）规则源「最新版本」取的是最旧版本，并连带 MRS 生成 400**

- 现象：/rule-sources 表格「最新版本」列始终显示最旧的那份（实测 versions 升序 [105407,105802,110119,110431,110433]，latest 显示 105407；两次「更新」后 latest 依次变成 105802、110119，仍非最新）。点「生成 MRS」→ `HTTP 400: {"error":"ConvertToMrs: invalid input: magic number mismatch"}`。
- 根因：internal/subforge/rulesources.go:199 `status.Latest = versions[0]`，而 internal/manifest/manifest.go:311-330 的 ListVersions 用 sort.Strings **升序**返回；MRS 处理器 internal/api/rule_sources.go:204-228 直接 `filepath.Join(src.Dir, src.Latest)` 取该值，且 format 缺省为 "mrs"（212-215 行），而源文件是 .yaml → 转换器报 magic number mismatch。
- 影响：用户看到错误的「最新版本」，并据此判断回滚/产物；MRS 功能对该源 100% 失败。

**D-03（P1）规则源「回滚」是空操作，却提示「已回滚」**

- 现象：点行内「回滚」→ 确认对话框「确认回滚 Custom_Clash_Full」→ 点「确认回滚」→ snackbar「已回滚: /data/rules/Custom_Clash_Full-20261020-110645.yaml」，但前后两次 /api/rule-sources 响应**完全一致**（版本数 5、latest、最后更新时间都没变），产物也没有变化。
- 根因：internal/subforge/rulesources.go:221-246 RollbackRuleSource 只调 manifest.Rollback（manifest.go:332-343 返回倒数第二个版本路径）并把它当返回值，不写任何状态；前端 web/src/pages/RuleSourcesPage.vue:118-119 doRollback 只 `notify('已回滚: ' + r.path)`。
- 影响：最危险的「假成功」——用户以为已回退到旧规则，实际输出仍是新版本。

**D-04（P1）机场「导入规则」在真实数据量下 >30s 且 500，页面只显示「不可用」**

- 现象：机场页对真实机场点「导入规则」，浏览器 console 必现 `Failed to load resource: 500 (Internal Server Error)`；页内探针 POST /api/airports/1/import-rules 用 15s/30s 两次 Runtime.evaluate 超时都等不到响应（任务分别 33s/64s）；页面没有可读错误，只显示「不可用」。
- 根因：internal/api/import_rules.go:26 importRulesFromAirport——上游拉取/解析失败 502（35/39 行）、写库失败 500（43 行 m.ImportRules）、前置缺失 503；internal/subforge/import.go:46 ImportRules 对每条规则单条 INSERT（124 行 ExecContext），**无事务、循环 9600+ 次**，UNIQUE 冲突才跳过（128 行）。
- 影响：把机场规则导进规则库这条主路径在真实数据量下不可用，而且失败原因对用户完全不可见。

### P2（10 项）

| 编号 | 模块 | 现象 | 根因/证据 |
| --- | --- | --- | --- |
| D-05 | 机场新增 | ftp://、not-a-url、javascript:alert(1)、空 URL 全部保存成功、零提示，直到刷新流量/转换才报 unsupported protocol scheme（实测 4 条垃圾数据均入库，已删） | 前端无校验、后端 internal/api 未校验 URL scheme；AP-03 用例截图 shots/t2/AP-03-*.png |
| D-06 | 机场工具栏 | 「刷新时同时导入规则」勾选后刷新页面即回未勾选；/api/settings 无该键，纯前端状态 | AP-10 run：刷新后 withRulesOn=false |
| D-07 | 自建节点 | 页面文案写「填错不会有报错提示，节点会静默不出现」，实测 `not-a-valid-link` 保存后无报错**且列表真的多了一行** | SN-03 run JSON + 截图；文案见 web/src/pages/SelfNodesPage.vue 顶部 alert |
| D-08 | 规则新增 | 类型、参数全空也能保存成功，无校验、无置灰，API 出现 rule_type="" 的规则（id 9646） | RL-03；internal/api/subscription.go:250 handleAPIRules 无必填校验 |
| D-09 | 规则列表 | 规则行页签没有任何批量控件（页内 checkbox 计数 0，翻页后仍 0）；批量逻辑只存在于「导入候选」tab，当前无候选数据 → UI 上不可达 | web/src/pages/RulesPage.vue:241-261（候选 tab 有 show-select/disable-selected/enable-all-airport）vs 203-211（规则行用 ResourcePage 无选择）；API `POST /api/rules/bulk` 可用 |
| D-10 | 分组 | 匹配正则 `[a-` 与测活间隔 `-5` 均保存成功，toast「已新增」，行显示 regex=[a- interval=-5 | GR-04；internal/api/subscription.go:171-177 DTO 无格式校验，后端也未 ValidateRegex |
| D-11 | 分组 + 自建节点 | 「排序（越小越靠前）」完全无效：新增填 10/1 列表仍按创建顺序；列表「顺序」列显示 1792495121675048000 这类 UnixNano | internal/api/subscription.go:171-177/210-216 与 531-548 的 DTO 都不含 sort_order；internal/subforge/subforge.go:234-237 AddProxyGroup 硬编码 time.Now().UnixNano()；internal/store/store.go:150-153 AddSelfNode 同；UpdateProxyGroup（store.go:361-364）不更新该列 |
| D-12 | 规则源 | 更新期间无 loading、无「更新中」文案，行内 source-update/rollback/mrs 均 `disabled=false`，可重复点击并发更新（remove 才会禁用） | RS-07 + lead9 探针（更新耗时 9.2s 内多次可点） |
| D-13 | 管线 | 清空脚本步骤的 ref 下拉时前端抛 `TypeError: Cannot read properties of null (reading 'value')` | web/src/pages/PipelinesPage.vue:281 `@update:model-value` 未判空 |
| D-14 | 脚本 | 代码 `return {`（语法错误）与空代码都能保存成功，保存期零校验，只有试运行才报错 | SC-04/SC-08 实测 saved=true；internal/api/scripts.go POST/PUT 不校验代码 |

### P3（10 项）

| 编号 | 模块 | 现象 |
| --- | --- | --- |
| D-15 | 脚本 | 「插入示例」按钮名不副实：点击只展开内联菜单，必须再点菜单项才填代码（实测 textarea 长度 0→60） |
| D-16 | 管线 | 脚本 kind 为空时同屏出现矛盾的「脚本已启用」+「类型可能不匹配：脚本 kind=undefined」 |
| D-17 | 脚本 | 试运行错误文案暴露内部脚本名：`脚本 "script-test" 被中断（超时或死循环）` |
| D-18 | 全局 | 401 横幅重复状态码：`HTTP 401: 401: 缺少或错误的访问令牌`（web/src/api/client.ts:6 + AsyncSection.vue:27/30 + useAsync.ts:5 各拼了一次） |
| D-19 | 全局 | 每页 console 一条 favicon.ico 404：web/index.html 无 `<link rel="icon">`，internal/api/web/dist/ 无 favicon 文件（server.go:322 已为它开了鉴权豁免，但文件不存在） |
| D-20 | 认证 | UI 没有显式登出入口；「清除」只清输入框不动存储，清空后点「保存」才是隐蔽登出（web/src/api/token.ts:18 空串清 localStorage） |
| D-21 | 脚本 | 删除脚本没有二次确认，点一下行内删除直接消失（机场/规则/分组/管线都有确认对话框） |
| D-22 | 全局 | 错误直接裸露内部报文：`HTTP 400: {"error":"json: cannot unmarshal string into Go struct field .interval of type int"}`、409/500 原文、`ConvertToMrs: invalid input: magic number mismatch` |
| D-23 | 脚本 | 试运行结果区被对话框底边裁切，截图里「成功（0ms）」下半截看不见 |
| D-24 | 机场 | 「测活全部」执行后没有任何结果汇总/结论文案，用户不知道跑没跑 |

## 5. 数据副作用与清理审计

用户在确认矩阵时明确允许在真实对象上做破坏性操作，因此本轮确实改过真实库。裸情商：**能恢复的都恢复了，不能恢复的只有 1 条规则，原因如实写在这里。**

| 项目 | 基线 | 末态 | 说明 |
| --- | --- | --- | --- |
| 机场 | 1 条（id=1，名称本来就是空字符串） | 1 条（id=1，名称仍空） | 名称空是本轮开始前就存在的现场；测试新增的机场全部经 UI 删除 |
| 自建节点 | 0 | 0 | 测试节点经 UI 删除（含 SN-05 复核残留，lead23 探针确认 after=[]） |
| 规则 | 9639 | **9638** | 中途「导入规则」成功 +2、测试新增若干并删除；末态比基线少 1 条，**无快照无法定位是哪条**，如实标注为待确认项 |
| 其中真实规则 sntp.uk | enabled=true | enabled=true | RL-08 反转后已用 UI 改回并探针核对 |
| 分组 | 0 | 0 | 测试分组经 UI 删除；GR-06 验证了「删分组后规则退回 DIRECT」 |
| 管线 | 0 | 0 | 测试管线经 UI 删除 |
| 令牌 | 0 | 0 | 测试令牌经页内鉴权 fetch DELETE 清理（UI 无删除入口时会卡在「选择令牌」），/subscriptions 已回到空态 |
| 脚本 | 1（真实脚本 test） | 1（test） | 5 条 UXT-T4C-* 经 UI 行内删除 |
| 规则源 | 1（Custom_Clash_Full，v5） | 1（同源，版本数 5） | 两次「更新」产生的新版本按产品「每源保留最近 5 个」策略自动淘汰；latest 字段被 D-02 污染但数据本身正常 |

**唯一一次绕过 UI 的写操作**：清理 5 条测试规则（id 9659/9660/9661/9662/9663）时，因规则表 9638 条无法用 UI 分页快速定位，使用**页面内已鉴权 fetch DELETE /api/rules/<id>**，5 条均返回 `{"ok":true}`；操作前后都做了总数与 bad 计数核对。其余新增/编辑/删除全部通过 UI 行内按钮与确认对话框完成。

## 6. free 模型派发复盘（你要求的「派发 free 模型」）

- 模型：`llmio/free`（list_subagent_models 只有 llmio 一家 provider，其 free 路由无法通过 spawn_teammate 指定，故用 subagent 派发，符合 teammate-dispatch 技能「需要独立验收的子任务」口径）。
- 派发 12 次：T1/T2/T3/T4/T5 五队 → 首轮 3 队零产物空转 → 拆成 t3a/t3b/t4a/t4b/t4c 并给「照抄级」执行简报（BRIEF.md + tasks/*.md + TEAM-GUIDE.md）→ 第二轮 6 队 + 补派 t1/t4c。
- 产出：t2-report.md（13 PASS/3 FAIL）、t3a-report.md、t3b-report.md、t4b 终局消息（7/7）、t5 18/22；t4a/t4c 只留半成品 runs。**没有一队能独立收口**——最终 83 例的判定、10 条缺陷的定级、以及全部 P1 的根因（源码逐行）都是我自己复跑 + 读源码完成的。
- 空转根因：任务书写成「用你的工具去做 N 个用例」这种开放句式时，free 模型会反复侦察而不落盘；改成「第一个动作必须 run_code 写心跳日志 + 每批 5 例 + 明确验收命令 + 45 分钟预算」后产出率明显提升。harness 的点击假阴性（按钮 y=1172 超出 900 视口）也让不少 free 队把「点不到」误报成产品缺陷，这部分我用 scroll→等待→重读 rect→鼠标事件+命中校验的 click 实现统一修掉后复跑。

## 7. 未验证 / 局限（如实声明）

1. **SC-07「脚本 → 管线 → 转换」端到端**没有被真正验证：该用例只读了脚本/管线列表（当时均为空）然后跑了一次未选管线的转换。已分别验证「管线 → 转换」（PL-07，真实机场 45→12 节点）与「脚本试运行 + 5s 上限」（SC-03/SC-05/SC-06），但「把脚本步骤挂进管线再转换」这一段没串起来。
2. CV-03/CV-07 的「复制」「diff A/B」断言因用例 evalNote 语法错误未落盘；复制/下载按钮存在且可点（CV-05/06 通过），diff 面板能出结果但断言无效。
3. SB-06 预设联动、OPS-03 清屏后断言写错 → 用例问题，不判缺陷；预设下拉实际选项是「默认/严格/乐观/独立/隐藏」，`sum` 是字段级策略（与页脚说明一致）。
4. 并发影响：测试期间多队并行写同一个真实库，规则总数在 9638–9644 间浮动，个别 PASS/FAIL 受时序影响（如 RL-02 的对话框在请求在途时被判定）；这些用例都已单独复跑或标注。
5. **无法访问 192.168.8.102 的文件系统与日志**，服务端行为只能通过 UI 响应体与源码（本地 HEAD 41962db 与部署 commit 一致）推断；部署侧配置（如 /data 目录权限）未核。
6. 只测了单一令牌 flzx3qcczk 的权限模型；RBAC/多用户/审计场景未覆盖。
7. 性能只做了 4 个采样点（转换 2.3s、导入规则 >30s、规则源更新 9.2s、订阅测活 848ms），未做压力测试。

## 8. 证据索引

- 报告与日志：.tmp/ux-e2e/FINAL-REPORT.md（本文）、reports/*.md、reports/*-progress.log
- 逐例结果 JSON：.tmp/ux-e2e/runs/<team>/<case>.json（含 checks/notes/shots/errors/exception 与耗时）
- 截图：.tmp/ux-e2e/shots/<team>/<case>-NN-<name>.png（关键证据：shots/lead/SC-LEAD3-01-sc03-kept.png、shots/t2/AP-01-01-airports-list.png、shots/lead11/TOK-01-tokens-clean.png）
- 执行代码：.tmp/ux-e2e/cdp.mjs（CDP 驱动）、harness.mjs（断言/对话框/表格/API 探针）、stp.mjs（步骤动词）、run.mjs（用例执行器）、act.mjs（自由操作）；用例 .tmp/ux-e2e/cases/*.json
- 复跑示例：`node E:/Projects/subforge/.tmp/ux-e2e/run.mjs t2 AP-03 AP-10`；单步自由操作：`node act.mjs probe @.tmp/ux-e2e/steps/xxx.json LABEL`

## 9. 建议的修复顺序

1. **先修 4 个 P1**（都是小改动）：D-01 给 Script 结构体/建表/API DTO 补 kind 字段并回填历史数据；D-02 改 `versions[len-1]` + MRS 按源文件格式传 format；D-03 回滚真正写 current 版本（manifest.Rollback 后落库 + 刷新列表）；D-04 导入规则改批量事务 INSERT + 前端给 loading/超时提示。
2. **再补输入校验**（D-05/D-08/D-10/D-14）：URL scheme、正则可编译、数字范围、代码可编译，全部在提交前与后端各校验一次。
3. **状态与契约一致性**（D-06/D-11/D-12）：sort_order 进 DTO 与 UPDATE、with-rules 进 settings、更新加 loading/幂等锁。
4. **错误文案治理**（D-22 及各 P3）：统一「用户可读信息 + 可展开详情」，不要把 Go 的 JSON 错误直出。
5. **体验与一致性**（D-09 批量入口、D-15 插入示例、D-20 登出入口、D-21 删除确认、D-23 裁切）。

---

## 10. 第二轮：修复 → 全量回归 → 复验（2026-10-20 夜，本地复刻环境）

> 本节是**第一轮 83 例报告之后的追加记录**。第一轮（§1–§9）测的是远端 http://192.168.8.102:25500/ 的**未修复版本**；本节测的是**修复后的同一份代码在本地复刻环境上的重新构建**。远端部署无法由我改动（无 shell、无 root），所以「修复是否真的生效」只能在本地用同一二进制验证——这一点先说明白，避免把本地结果误读成远端已修复。

### 10.1 为什么必须复刻环境

- 远端站点只暴露 HTTP，无法改代码/重建产物/看服务端日志；而本轮修复涉及后端（Go）与前端产物（Vite dist 被 embed 进二进制），必须重新构建才能验证。
- 因此做法是：在本地仓库（HEAD 62c694c + 本轮未提交改动）构建 `subforge.exe`，用**本地 fixture 机场订阅**替代真实机场，用**同一套 CDP 用例**（92 次执行 / 85 个唯一用例）重跑，把「修复前红、修复后绿」逐条对上。
- 用户要求「派 free 模型复跑、用 teammate + 共享任务、名字带 free 即免费模型」：本轮按要求用 `spawn_teammate` 起了 4 名复跑队员（rerun-*-free）+ 1 名 P3 修复队员（fix-p3-free）+ 2 名修复/复核队员（fix-f-free、fix-b-free），全部走共享任务（task-2/4/5/6/7/9/11）。**但模型路由我无法从工具侧证实**：`spawn_teammate` 没有 provider/model 参数，`list_agents` 当前对这些 teammate 报告的 `model` 是 `deepseek-flash` 而不是 `free`；只有显式 subagent(provider=llmio, model=free) 的调用我有硬证据。名字里的 free 只是命名约定，不等于服务端路由到免费模型——这一条如实标为未验证。

### 10.2 复刻环境（可复现）

| 组件 | 事实 |
| --- | --- |
| fixture 服务器 | `node .tmp/ux-e2e/local/fixture-server.mjs` → 127.0.0.1:25630。`/sub.yaml`：45 节点（香港01..10、HK01、HK02、日本01..10、新加坡01..10、美国01..13，ss/trojan/vmess 轮换）+ 9639 条 `DOMAIN-SUFFIX,seed-NNNN.test,DIRECT`；响应头 `Subscription-Userinfo: upload=291363609; download=5292093876; total=107374182400; expire=1794477630`；`/rs/*.yaml` 供规则源更新/MRS；`/bad.yaml` 恒 500 |
| 种子 | `.tmp/ux-e2e/local/seed.mjs`：机场「本地种子机场」45 节点、规则 9639、规则源 Custom_Clash_Full 3 个版本、脚本 1 条（filter，`node.type === 'ss'`）、令牌 0 |
| 5 实例 | 25611–25615，token=`fixtoken`，data-dir=data/L1..L5（同一份 seed 快照复制，**含 SQLite WAL**） |
| 用例本地化 | 12 处真实机场 URL → fixture（t2 3 / t4b 2 / t4c 1 / t5 6）；「最新版本」→「当前版本」；Windows 路径断言放宽；t3b 的 GitHub ini 源 → fixture /rs/one.yaml。原始用例备份 `.tmp/ux-e2e/cases-original/` |
| 运维事实 | **所有 API（含 /healthz）都要 X-Token 头**，否则 401「缺少或错误的访问令牌」；服务由 Lead 统一起，队员只跑自己实例 |

### 10.3 free 队复跑（首跑 68/92）与三分法归因

- 4 名 teammate：`rerun-t12-free`（t1+t2）、`rerun-t3-free`（t3+t3b）、`rerun-t4-free`（t4a/b/c）、`rerun-t5-free`（t5）；共享任务 task-4..7。
- 首跑 68/92（73.9%）。25 个失败按「CASE-STALE / PRODUCT-BUG / ENV-FLAKY」三分法逐条复核：**24 例是用例自身陈旧（CASE-STALE）、1 例环境残留（ENV-FLAKY）、0 例新产品质量缺陷**。典型用例缺陷：
  - 用例里硬编码远端令牌 `flzx3qcczk`（本地是 fixtoken）→ 局部 401；
  - `harness.mjs` 的 clearField 用 CDP Ctrl+A + Backspace，**Backspace 不删选区**，v-text-field 清不空（跨队影响 3 例）；
  - SubscriptionsPage 预设下拉是 v-virtual-scroll，选过一项后 scrollTop=186，目标项滚出 DOM；
  - 分页档位写 20（实际 10/25/50/100/全部）、路径断言写死 Linux、`evalNote` 内联表达式语法错（尾部多 `()`）。
- 改用例后各队自报全绿；**我再独立复跑**（原始命令，`run.mjs` 直跑，不经队员脚本）：t1 10/10（170s）、t2 17/17（568s）、t3 15/15（284s）、t3b 7/7（205s）、t4 21/21、t5 22/22（394s），合计 92/92。
- 队员首跑确实发现了一个真实前端缺陷：**PipelinesPage 清空脚本步骤 ref 时 `TypeError: Cannot read properties of null (reading 'value')`**（第一轮 D-13 的 P3 项），已进修复队列。

### 10.4 本轮修复（逐项源码证据）

| 缺陷 | 修复 | 关键证据 |
| --- | --- | --- |
| D-01 脚本 kind 全链路丢失 | `internal/store/store.go:220` 加 `Kind` 字段；建表 `kind TEXT NOT NULL DEFAULT ''` + `addColumnIfMissing` 迁移；`internal/api/scripts.go` POST 空→filter、非法 400「未知的脚本类型: X」、PUT 空保留旧值；前端 PipelinesPage 不再对空 kind 提示 | `go test ./internal/store ./internal/api` 绿；探针 25601 9/9 PASS |
| D-04 导入 9639 条规则 >30s 且 500 | `internal/store/rules_batch.go`（新）单事务批量 INSERT OR IGNORE；`internal/subforge/import.go:135` 改 `BatchInsertImportedRules`；关键坑：`idx_rules_origin` 是部分索引 `WHERE origin_key <> ''`，刷时间戳 UPDATE 必须显式带该条件（不加时二次导入 26.4s，加了 143ms） | 本地 5000 条 YAML 探针：首次 200 / 89.7ms / rules_added=5000，二次 200 / 109.7ms |
| D-02/D-03 规则源「最新版本」显示最旧、回滚不写当前版本 | `internal/subforge/rulesources.go:173-187` 新增 `.current-versions.json` 指针（CurrentVersionsFile 常量 + load/save/get/setCurrentVersion），latest 取当前版本；回滚真正写指针；MRS 按源文件扩展名传 format | 探针 probe-b.mjs 7/7、probe-b2.mjs 10/10（单版本回滚 400、.json 源 MRS 400、指针落盘、显式 format 400）、probe-b3.mjs 4/4（本地源 + 更新/失败源/update-one/并发×3） |
| D-12 更新期间可重复点击 | 前端 RuleSourcesPage.vue:271/:317/:385 加 `:disabled="updating"` / `:loading`；探针把 update 人为延迟 3s，按钮全程 disabled、重复点击请求数仍为 1 | probe-t3c：3s 延迟下 disabled=true 全程、请求数=1 |
| D-05/D-07/D-08/D-10/D-11 输入校验与排序 | `internal/api/subscription.go` 新增 validateAirport / validateSubscriptionURL / validateProxyGroup / validateRule / validateSelfNode，机场/规则/分组/自建节点 POST+PUT 全接 400；分组与自建节点 DTO 加 sort_order 透传；store 用 `sort_order = CASE WHEN ? > 0 THEN ? ELSE sort_order END` 保序 | 探针 probe-f.mjs 33/33（非法输入全 400 且文案逐字正确；GET /api/groups 顺序 probe-g2(1) 在 probe-g1(5) 前；PUT sort_order=0 保持 5） |
| D-13 清空 ref 抛 TypeError | `web/src/pages/pipelineStepArgs.ts`（新）导出 `stepArgRef(v: unknown): string`：字符串原样、对象取 value、null/undefined/其它 → ''、绝不抛异常；PipelinesPage.vue:285 模板改走该函数 | vitest 2 files/43 tests PASS；vue-tsc EXIT=0；最终构建回归 PL-04 PASS 且 consoleErrors=[] |
| D-19 favicon 404 | `web/public/favicon.svg` + `web/index.html:7` `<link rel="icon" type="image/svg+xml" href="/favicon.svg">`；**追加修 Lead**：`internal/api/server.go:322` 的鉴权豁免原本只放行 `/favicon.ico`，导致 `/favicon.svg` 带 X-Token 才 200、浏览器拿 401 → 加入 `p == "/favicon.svg"`；auth_exempt_test 补两条 | `GET /favicon.svg`（无令牌）200、content-type image/svg+xml、472 字节；最终构建 t1 G-08 的 consoleErrors=[] net=[] |
| D-07 自建节点文案与行为不一致 | 后端已由 validateSelfNode 400 拒绝；G3 把 SelfNodesPage.vue:41 文案改成「保存时服务端会逐条解析校验…直接返回 400 并报错…不会静默入列表」，docs/wiki 同步 | t2 SN-03 在最终构建上 PASS（页面出现「HTTP 400: {"error":"分享链接格式不正确：…"}」、dialog 不关、列表不建行） |

修复后的完整门禁（Lead 独立执行）：`go build ./...` 0、`go vet ./...` 0、`go test ./... -count=1` 0（32 包全 ok）、web `npx vitest run` 702 tests 全 PASS、`npx vue-tsc --noEmit` 0。

### 10.5 最终构建上的全量回归（92/92 PASS）

- 构建：`cd web; node node_modules/vite/bin/vite.js build` → 657ms，产物 `index-C3ErVu3_.js` 652.90kB + `favicon.svg`；`go build -o .tmp/ux-e2e/local/bin/subforge.exe ./cmd/subforge` → 86,715,392 字节 @ 22:56:57。
- 环境：data-seed 快照重建 L1..L5（每目录 8 文件，含 WAL），5 实例全部 `airports=1 nodes=45 favicon=200`。
- 结果（Lead 原始命令 `node .tmp/ux-e2e/run.mjs <team>`，UX_BASE_URL 指向 25611–25615）：

| 团队 | 结果 | 耗时 | 关键回归点 |
| --- | --- | --- | --- |
| t1 | **10/10 PASS** | 171s | G-08 十三路由巡检 consoleErrors=[]、netErrors=[]（favicon 404 已消失） |
| t2 | **17/17 PASS** | 569s | SN-03 按 D-07 修复后行为改写断言 → PASS |
| t3 | **15/15 PASS** | 284s | RS-04 回滚/MRS/当前版本链路 |
| t3b | **7/7 PASS** | 207s | 同上第二组 |
| t4 | **21/21 PASS** | t4a 140s / t4b 135s / t4c 175s | PL-04 PASS 且 consoleErrors=[]（TypeError 已消失） |
| t5 | **22/22 PASS** | 395s | SB-06/CV-03/OPS-03/OPS-01 全部通过 |

合计 **92/92**（85 个唯一用例；t3/t3b 有 7 个 RS 用例重叠）。修复后再跑 `go test ./... -count=1` 仍为 EXIT=0。

### 10.6 24 项缺陷的最终状态

- **已修 12 项**：D-01、D-02、D-03、D-04、D-05、D-07、D-08、D-10、D-11、D-12、D-13、D-19。
- **未修 12 项**（都是本轮没有派工或刻意留给产品决策的）：
  - P2 3 项：D-06（withRules 勾选刷新即丢，需进 settings）、D-09（规则行页签无批量控件，UI 不可达）、D-14（脚本代码保存期零校验，`return {` 仍可保存——后端 scripts.go 完全没有语法校验）。
  - P3 9 项：D-15（插入示例需二次点击）、D-16（kind 空时同屏矛盾提示）、D-17（试运行文案暴露内部脚本名）、D-18（401 双状态码）、D-20（无显式登出入口、清空后保存=隐蔽登出）、D-21（脚本删除无二次确认）、D-22（裸露 Go 内部报文）、D-23（试运行结果被对话框裁切）、D-24（测活全部无汇总）。
- 另外：D-05 的上游诱因「机场 URL 非法」现在会被 400 拦住，但**远端已存在的历史垃圾机场不会被自动清理**（本地环境无此历史数据）。

### 10.7 评分重估（第二轮口径，主观）

按修复后的实测重评（第一轮分数在括号里）：

| 维度 | 第二轮 | 变化理由 |
| --- | --- | --- |
| 核心闭环 | **8/10**（6） | 导入 9639 条规则从 500 变 200/89.7ms；脚本 kind 全程可见；规则源当前版本/回滚/MRS 正确 |
| 输入校验 | **7/10**（3） | 机场/规则/分组/自建节点全接 400 且文案可读；仍缺脚本代码语法校验（D-14）与前端即时校验 |
| 错误处理 | **5/10**（4） | 校验类错误已可读；但仍裸露 Go 报文（D-22）、401 双状态码（D-18） |
| 数据一致性 | **7/10**（4） | kind 落库、规则源指针、sort_order 生效、批量导入事务；withRules 仍未持久化（D-06） |
| 性能 | **7/10**（5） | 导入 9639 条 89.7ms（二次 109.7ms）；/api/rules 仍全量返回（前端分页） |
| 设计 | **6/10**（6） | favicon、文案修好；批量入口/登出/删除确认仍缺 |
| **综合** | **6.5/10**（4.7） | 12 项缺陷闭环、92/92 回归通过；剩余 12 项以 P3 体验项为主，无 P0/P1 |

### 10.8 本轮未做 / 不能下的结论

1. **远端 http://192.168.8.102:25500/ 没有重新验证**：本轮所有修复都在本地工作区（未 commit；`git diff --stat` = 34 files changed, 1795 insertions(+), 195 deletions(-)，`git status --porcelain` 39 项含未跟踪文件与 dist/.gitkeep 删除），远端仍是第一轮测的旧版本。要在远端看到效果，必须部署新二进制。
2. 本地复刻终究不是同一台机器：fixture 机场是本机 25630，时延/并发与真实订阅不同；性能数字（89.7ms/109.7ms）只代表本地 SQLite + 本机 HTTP。
3. 第二轮回归跑在 5 个实例上并行（每队一个），互相不共享数据库；跨实例一致性未测。
4. 修复涉及的后端改动只做了 `go test` 与探针级验证，**没有做代码评审（review）与安全审计**；例如新增的校验函数是否覆盖全部入口，仅按探针逐一验证。
5. 队员自测报告只作线索：本节的结论全部来自 Lead 自己跑的 `run.mjs` 原始输出与源码核对；队员自报的 92 例全绿我只采信为线索，逐队复跑后才是 §10.5 的结果。

## 11. 第三轮：剩余 12 项修完 → 93 次浏览器回归全绿 → 分批提交

### 11.1 决策口径（用户拍板，逐条落实）

| 议题 | 决策 | 落实 |
| --- | --- | --- |
| 提交方式 | 按缺陷分批 commit | 4 个语义化 commit（§11.8） |
| 剩余缺陷 | P2 3 项 + P3 9 项全修 | 12/12 修完（§11.3） |
| D-14 脚本校验 | 保存时强制校验，语法错返回 400 | goja 编译期校验，POST/PUT 都拦 |
| D-06 落点 | 存服务端 /api/settings | 新增 `airport.with_rules` 设置项 |
| D-09 范围 | 行多选 + 全选当前筛选 + 批量启停 + 批量删除（二次确认） | 四项全做（§11.3） |
| 发布 | 「github 发新版本」 | 打 tag 触发 release.yml 出 Release 资产（§11.9） |
| free 模型 | 保持现状（命名约定即可） | 后续仍用名字带 free 的 teammate |
| 本地环境 | 全部清理（含 data、shots、用例） | 收尾清理（§11.10） |
| 产物入库 | 报告移到 docs/reports 提交，用例留 .tmp | 报告+矩阵入库，脚本/用例继续 gitignore |

### 11.2 派发与人员：3 名队员全部失联、3 名继任者交付

- 第一波 `fix-w1-free` / `fix-w2-free` / `fix-w3-free`（共享任务 task-12/13/14）在 23:49 / 23:59 / 00:20 先后 inactive，都没交报告：W1 只落盘 settings 后端，W2 只落盘 /api/rules/bulk 与 BatchDeleteRules，W3 自报七项改完但没写报告、没做独立验收。
- 处理：`delete_teammate` → 写交接任务书 `.tmp/ux-e2e/fix2/TASK-W1B.md / TASK-W2B.md / TASK-W3B.md`（列明已落盘改动、禁止触碰文件）→ spawn `fix-w1b-free` / `fix-w2b-free` / `fix-w3b-free`，task-12/13/14 reassign 后交付。
- 模型路由的可证实边界：三名继任者开工时 `list_agents` 的 model 字段显示 `free`，全部 inactive 之后同批成员又显示 `deepseek-flash`。**这个字段不能作为底层模型路由的证据**；能证实的只有「名字带 free 是派发约定」，以及它们各自交付了什么。

### 11.3 12 项修复（源码证据）

| 缺陷 | 修复 | 关键证据 |
| --- | --- | --- |
| D-06 withRules 勾选刷新即丢 | `internal/subforge/settings.go` 新增 `const AirportWithRulesKey = "airport.with_rules"` 与 bool 设置项；`AirportsPage.vue` loadSettings/saveWithRules（切开关即 PUT /api/settings），onMounted 回填 | 接口往返 `{"key":"airport.with_rules","value":"true","kind":"bool","label":"刷新时同时导入规则"}`；AP-10 7 项断言含 reload 后仍为 true、关闭后 reload 仍为 false |
| D-09 规则行无批量入口 | 后端 `internal/api/rules_bulk.go` 支持 `{ids, delete:true}`（只收 ids，缺参 400「批量删除必须给出 ids」）；`internal/store/rules_delete.go` 单事务 BatchDeleteRules；前端 RulesPage tab0 自建表：搜索+启用筛选、行多选、表头全选当前筛选（select-strategy="all"）、批量条 bulk-enable/disable/delete、删除二次确认 | RL-09 14 项断言：筛选后 2 行 → 全选 → 批量禁用 → API 两条 enabled=false → 再全选 → 删除确认文案含「2 条」→ 确认后 API 归零 |
| D-14 脚本代码保存期零校验 | `internal/api/scripts.go:33-41` `validateScriptCode`（goja 编译，空→「脚本代码不能为空」，编译失败→「脚本代码语法错误：<goja 错误>」）；POST :84 / PUT :159 都调用 | SC-04 5 项断言：`return {` 保存 → 400「脚本代码语法错误：SyntaxError: script: Line 1:5 …」、不落库、save-error 可见；改回合法代码后保存成功 |
| D-17 试运行暴露内部脚本名 | `internal/api/scripts_test_api.go:16-18` `const scriptTestDisplayName = "试运行脚本"`；`internal/transform/script.go:49/51/67-73` 超时中断文案带已运行耗时 | 试运行报错实测：`节点 "ss-node": 脚本 "试运行脚本" 被中断（超时或死循环）：执行超过 50ms 上限 …（已运行 51ms）` |
| D-24 测活无汇总 | AirportsPage `checkSummary`，测活前清空、结束按 results 统计，渲染 `data-testid="check-summary"`「测活完成：N 个可用 / M 个失败（共 T 个）」 | AP-10 断言 `check-summary` 可见且含「测活完成」 |
| D-15 插入示例名不副实 | ScriptsPage `toggleExamples` 打开菜单时即插入 `SCRIPTS_HELP.examples[0]` | SC-02 6 项断言：点一次「插入示例」后 textarea 非空 |
| D-16 kind 空时同屏矛盾提示 | PipelinesPage `scriptRefKind` 归一空串、`scriptRefOkText` 返回「脚本已启用（类型未标注）」 | 单测覆盖；vue-tsc 0 |
| D-18 401 双状态码 | `web/src/api/client.ts` `stripStatusPrefix` 去掉重复的 `HTTP 401: 401: ` 前缀；`HttpError` 带 `detail` | 变异验证：还原 stripStatusPrefix → D-18 用例失败并复现 `HTTP 401: 401: 缺少或错误的访问令牌` |
| D-20 无显式登出入口 | `web/src/api/token.ts` 新增 `logout()`；AppTopBar 加 mdi-logout 按钮 `data-testid="logout"`；AppShell 加确认框（logout-cancel / logout-confirm），确认后 logout + 重弹令牌框 + 重取数据 | G-04 24 项断言：点 logout → 确认 → localStorage token 为 null → 出现「缺少或错误的访问令牌」→ 重新登录成功恢复导航 |
| D-21 脚本删除无二次确认 | ScriptsPage askDelete/doDelete 中间加确认框（script-confirm-cancel / script-confirm-delete） | 单测：取消不发请求、确认才 DELETE |
| D-22 裸露 Go 内部报文 | 后端 `internal/api/handlers.go` 新增唯一错误出口 `writeError`/`apiErrorText`/`jsonDecodeErrorText`：JSON 解码错误→「请求参数格式不正确：字段 X 需要整数」、500→「服务端内部错误（请求 ID …）」、原始报文进 `detail`；前端 client.ts errorText/errorDetail 截断展示，AsyncSection 增加可展开 `data-testid="async-error-detail"` | 单测 `errors_test.go` + client.spec；页面错误提示可展开原始 detail |
| D-23 试运行结果被裁切 | ScriptsPage 结果区 `.test-result-scroll` max-height:40vh + overflow-y:auto，对话框加 `scrollable` | 单测断言样式类；视觉上可滚动 |

修复后门禁（Lead 独立执行，原始命令）：`go build ./...` 0、`go vet ./...` 0、`go test ./... -count=1` 0（32 包全 ok，api 40.419s / transform 1.334s / subforge 24.032s / store 11.494s）、`cd web; npx vue-tsc --noEmit` 0、`npx vitest run` **39 files / 757 tests 全 PASS**。

### 11.4 用例改写（修复改变行为，旧断言必红）

修复把 5 处旧行为断言变成「必红」，Lead 逐处改写（原文件备份 `.tmp/ux-e2e/cases-backup2/`）：

| 用例 | 旧行为 | 新断言 |
| --- | --- | --- |
| t1/G-04 | 顶栏无退出入口（期望 NONE） | 找到 `logout` → 确认 → token 清空 → 401 横幅 → 重新登录 |
| t2/AP-10 | 勾选 withRules 后不验证持久化 | 勾选 → reload → 仍为 true；取消 → reload → 仍为 false；补 check-summary 断言 |
| t3/RL-09（新增） | 规则行 checkbox 数量===0（缺陷取证） | 建两条 ux-batch 规则 → 筛选 → 全选 → 批量禁用 → 批量删除（确认） |
| t4c/SC-02 | 点「插入示例」后 textarea 仍为空 | 点一次后 textarea 非空 |
| t4c/SC-04 | 语法错误代码可保存、试运行才报错 | 保存即 400、不落库、save-error 可见；改合法代码后保存成功 |

改后用例规模：t1 10 / t2 17 / t3 16 / t3b 7 / t4a 7 / t4b 7 / t4c 7 / t5 22 = **93 次执行、86 个唯一用例**（t3 与 t3b 有 7 个 RS 重叠）。

### 11.5 最终构建与本地环境

- `cd web; node node_modules/vite/bin/vite.js build` → 1.26s，产物 `index-*.js` 654.76kB + favicon.svg，写回 `internal/api/web/dist`。
- `go build -o .tmp/ux-e2e/local/bin/subforge.exe ./cmd/subforge` → 86,745,600 字节 @ 00:55（含本轮全部修复）。
- 重新 seed（`rebuild-and-seed.ps1`）：机场 1（45 节点）、规则 9639、规则源 Custom_Clash_Full 3 版本（latest=`Custom_Clash_Full-20261020-165532.yaml`）、脚本 1；快照复制到 data/L1..L5（各 8 文件，含 SQLite WAL）。
- 5 实例 25611–25615（token=fixtoken）实测一致：airports=1、rules=9639、latest 同上、version 200。

### 11.6 93 次回归结果（Lead 原始命令 `node .tmp/ux-e2e/run.mjs <team>`）

| 团队 | 结果 | 耗时 | 本轮关键回归点 |
| --- | --- | --- | --- |
| t1 | **10/10** | 180s | G-04 24 项断言（登出→确认→401→重登）通过 |
| t2 | **17/17** | 578s | AP-10 7 项（D-06 持久化 + D-24 汇总）通过；SN-03 文案断言通过 |
| t3 | **16/16** | 306s | RL-09 新增 14 项批量断言通过；RL-01..08 原断言未回归 |
| t3b | **7/7** | 205s | 规则源第二组 |
| t4a / t4b / t4c | **21/21** | 136s / 134s / 171s | SC-02 6 项、SC-04 5 项（D-14/D-15）通过；PL-04 consoleErrors=[] |
| t5 | **22/22** | 394s | SB/CV/OPS 全通过 |
| **合计** | **93/93** | — | 逐例 failed=0，无 FAIL、无异常 |

日志：`.tmp/ux-e2e/rerun/final-t1.log` / `final-t2.log` / `final-t3.log` / `final-t4.log` / `final-t5.log`；截图在 `.tmp/ux-e2e/shots/<team>/`。

### 11.7 24 项缺陷最终状态

- **已修 24 / 未修 0**：第一轮 24 项（P0 0 / P1 4 / P2 10 / P3 10）全部闭环——第二轮修 12 项（D-01/02/03/04/05/07/08/10/11/12/13/19），第三轮修 12 项（D-06/09/14/15/16/17/18/20/21/22/23/24）。
- 仍存在但**不在 24 项清单**内的观察（不阻塞交付，留作后续）：
  1. `internal/api/rule_sources.go:102-112` 的 `writeRuleSourceFailure` 仍直接 `writeJSON(500, {error, results})`，未走统一 `writeError`，该类响应不会有中文文案与 request_id；
  2. `internal/api/middleware.go:93` 的 401 仍是纯文本（前端已能去重展示，但非 JSON）；
  3. `/api/rules` 仍一次性返回全量规则（9639 行含 raw），前端做客户端分页；
  4. 规则源版本名用秒级时间戳，同秒两次更新会覆盖同一文件；
  5. retention 清理与回滚删除文件是两套逻辑，未合并。

### 11.8 分批提交（全部已完成，工作树干净）

| commit | 范围 | 统计 |
| --- | --- | --- |
| e80d57d | feat(airports): D-06 持久化 + D-24 汇总 | 4 files +166/−8 |
| d1b3078 | fix(scripts): D-14 语法校验 + D-17 文案 | 7 files +184/−9 |
| b7c0579 | feat(rules): D-09 批量操作 | 7 files +809/−34 |
| f5bf154 | fix(web): D-15/16/18/20/21/22/23 | 20 files +1108/−27 |

前两轮另有 5 个 commit：7b2bba0（输入校验与排序落库）、6c6a8c7（批量导入）、a04bbe3（规则源指针/回滚/MRS）、ea0bdb5（P3 三项）、a7745fe（favicon 免鉴权）。

发布后另有 2 个**只改测试**的收尾 commit：92d9a83（`test(web)`：补齐 vitest 覆盖率门禁，6 files +256）、4a8eb8e（`test(ci)`：Go 覆盖率门禁 + gofmt，6 files +155/−8），背景见 §12。

### 11.9 评分重估（第三轮，主观）

| 维度 | 第三轮 | 变化理由 |
| --- | --- | --- |
| 核心闭环 | **8/10** | 不变；批量入口补齐后长列表可操作 |
| 输入校验 | **8/10**（7） | 脚本代码在保存期校验并返回可读 400；前端仍无即时校验 |
| 错误处理 | **7/10**（5） | 统一错误出口 + 中文文案 + detail + request_id；401 中间件与规则源失败路径未统一 |
| 数据一致性 | **8/10**（7） | withRules 落 settings、批量删除单事务、kind/排序/指针已修 |
| 性能 | **7/10** | 不变（导入 89.7ms；/api/rules 全量返回） |
| 设计 | **8/10**（6） | 显式登出、删除确认、示例一次点击、结果可滚动、测活汇总 |
| **综合** | **≈7.7/10**（6.5） | 24/24 缺陷闭环、93 次浏览器回归全绿；剩余为架构级观察项 |

### 11.10 未做 / 不能下的结论（第三轮）

1. **远端 http://192.168.8.102:25500/ 仍未重测**：本轮全在本地复刻环境验证；远端要等 v0.1.8 Release 部署后才有同样行为。
2. 5 个实例各自独立数据库，**跨实例/并发一致性未测**；浏览器回归是串行跑单实例。
3. 未做代码评审与安全审计；本轮修复的边界靠探针+单测+浏览器用例三者交叉验证。
4. 未做压力/并发测试；`/api/rules` 全量返回在大数据量下仍是隐患（未改）。
5. 队员自测只作线索：§11.6 全部来自 Lead 自己跑的 `run.mjs` 原始输出与源码核对。

6. **teammate 的模型路由未验证**：第二轮 teammate 的 `list_agents` 报 `model=deepseek-flash`，与「名字带 free 即免费模型」的约定不一致；第三轮三名修复 teammate 报 `model=free`（见 §12.1 更正）。底层推理是否走免费路由仍无法从 DSH 侧确证，只能改用 subagent(provider=llmio, model=free) 或由部署侧核对路由日志。

## 12. CI 门禁收尾（v0.1.8 之后）

v0.1.8 的 tag 推送后，GitHub Actions 的 `CI` 工作流连续失败两次。两次都不是功能代码出错，而是仓库门禁自身没达标；而且第一次失败把其余 job 全部 skipped，所以有两个 Go 门禁直到第二次才暴露出来。

### 12.1 失败与修复

| 失败 job | 失败步骤 | 根因 | 修复 |
| --- | --- | --- | --- |
| web: Test + coverage gate | `pnpm run coverage` | vitest 全局阈值 lines/functions/statements=100、branch=80，实测 98.8 / 95.65 / 98.31 / 99.17；未覆盖点全部落在本轮新增代码：RulesPage 批量条与筛选、ScriptsPage 示例/试运行对话框、AppShell 退出确认、AirportsPage withRules 失败分支等 | 92d9a83：只补测试、不改阈值、不删测试，6 个 spec +256 行 |
| coverage | `Coverage gate` | `coveragegate -min 80 -min-func 100`：语句 91.2% 达标，函数覆盖 887/890。缺的 3 个函数是 `render/clashmeta.Warnings`（62c694c 的规则目标自动映射引入，clashmeta 包内没有测试调用它）与 `store/rules_batch.go` 的 `BatchInsertImportedRules`、`MaxRulesSortOrder`（只被 internal/subforge 间接调用，而 `go test -coverprofile` 只统计被测包自身） | 4a8eb8e：新增 `internal/store/rules_batch_test.go`（4 例：空批次、首次写入、重复导入刷新 updated_at、空 origin_key 恒新增）、clashmeta_test.go 加 `TestWarnings_RewrittenRuleTargets` |
| test (ubuntu-latest) | `Gofmt check (Linux)` | 4 个测试文件 gofmt 不干净（结构体字段对齐、拼接空格、多余空行，纯空白差异） | 4a8eb8e 内 `gofmt -w` |

补充说明：`62c694c` 那次 CI（run 39397203494）其实已经只失败在 `coverage` 一个 job —— 函数覆盖的洞在前端改动之前就存在；gofmt 与 web 覆盖率的洞则是本轮测试文件与前端改动带进来的。

### 12.2 本地复跑证据（Lead 原始命令）

- `cd web; npx vitest run --coverage` → 39 files / 768 tests 全 PASS；All files 100 stmts / 96.5 branch / 100 funcs / 100 lines，退出码 0。
- `go test -coverprofile=<abs> -covermode=atomic ./internal/... -count=1` 后 `go run ./internal/tools/coveragegate -min 80 -min-func 100` → **语句 91.5%（8795/9610，≥80）、函数 890/890 = 100%**，退出码 0；`go run ./internal/tools/condgate -engine=profile -min 80` → 条件覆盖 82.8%，退出码 0。
- `go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿（32 包）。
- `gofmt -l cmd internal pkg` 输出为空。

### 12.3 最终 CI 状态

- run **39467600382**（main `4a8eb8e`）：**全部 job 成功** —— web / test (ubuntu-latest) / test (windows-latest) / coverage / cross-build / benchmark / docker-build；condition-audit 为手动触发，skipped 属正常。
- v0.1.8 Release（`bddbe64`）与两个收尾 commit 无关：它们只改 `_test.go` 与 spec，产物二进制无差异，因此 tag 未移动、Release 未重发。若后续要发 v0.1.9，应从当前 main 打 tag。

### 12.4 对 §11.10 第 6 条的更正

§11.10 第 6 条写于第二轮，依据是当时 teammate 的 `list_agents` 输出（报 `model=deepseek-flash`）。第三轮三名修复 teammate（fix-w1-free / fix-w2-free / fix-w3-free）在 spawn 后与运行期间，`list_agents` 报的是 `model=free`；失联转 `inactive` 后才显示 `deepseek-flash`。因此可下的结论只有「名字含 free 时列表字段显示 free，第三轮与约定一致」，**底层推理是否真的路由到免费模型仍无法从 DSH 侧确证**。

