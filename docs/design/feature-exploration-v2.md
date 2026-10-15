# SubForge 新功能发散探索 v2

> **v2.1 修订**：修正 F2 隐含的"后端可见流量"错误假设——SubForge 不在数据面上，对用户真实访问没有天然视角。观测能力现显式建模为三种部署拓扑 + Beacon 上报代理（见 F2 修订节与主设计书 §4.1，新增风险 R11）。F1 本为纯静态模拟，已补充说明以免误读。

> 本文是《SubForge：订阅分流一体化方案设计书》的功能扩展篇。方法论：先无限制发散（24 个候选），再按统一标准收敛（采纳 / 远期 / 拒绝），**每项均标注实现依据、成本与风险，无悬而未决项**。
> 发散过程中对 4 个关键依赖做了源码级实测验证（见各功能"验证"栏，2026-10-13）。
> 成本单位为人日（1 名熟悉 Go 的开发者）。"复用"列指向上游可复用资产。

---

## 0. 收敛结果总览

| 收敛桶 | 数量 | 判定标准 |
|---|---|---|
| 旗舰功能（重点展开） | 6 | 生态空白 + 与"超高度定制分流"主轴强相关 + 依赖已验证 |
| 快速采纳（并入路线图） | 8 | 低成本（≤5 人日）+ 明确价值 + 低风险 |
| 远期观察（给出触发条件） | 6 | 有价值但依赖未成熟 / ABI 设计成本高 / 性能语义待实测 |
| 明确不做（附理由） | 5 | 违背性能预算 / 职责越界 / 不可行 |

**漏斗**：24 个候选 → 6 旗舰 + 8 采纳 + 6 远期（其中 1 项并入旗舰）+ 5 拒绝。

---

## 1. 旗舰功能

### F1 分流追踪器 `trace`（分流 dry-run 模拟器）

**一句话**：给定五元组（域名 / IP / 端口 / 进程名 / UA），静态跑一遍最终生成的配置，返回"命中哪条规则 → 走哪个策略组 → 落到哪个节点 → 是否 Fake-IP"，不产生真实流量。

> **v2.1 澄清**：F1 是**纯静态模拟**——解析自己刚生成的配置并按序求值，与任何流量观测无关，T1/T2/T3 三种拓扑下均可用。其输入域名/IP 来自用户手工查询（CLI/HTTP），可选预填自 T1/T2 观测到的历史连接记录，但后者仅为便利性、非功能前提。"落到哪个节点"同样只读配置内的节点列表与测活缓存，不发起连接。

- **生态空白**：mihomo 无 dry-run 能力；Clash Verge 的"连接"页只能看已发生连接。排障"为什么这条域名走了代理"目前只能靠人肉读规则。社区无人做静态模拟。
- **设计**：实现一个只读匹配引擎副本，按 mihomo 语义顺序执行（domain 后缀精确/关键字 → IP-CIDR（DNS 解析开关）→ GEOIP → RULE-SET → FINAL）；policy group 解析为普通函数求值；`trace` 同时支持 CLI（`subforge trace --domain openai.com --profile home`）与 HTTP API（`GET /trace?domain=&ip=&process=`）。
- **复用**（已验证）：`MetaCubeX/mihomo component/trie`（MIT，实测存在 `domain.go/domain_set.go/ipcidr_trie.go`）——直接复用其 domain trie 与 ipcidr trie，保证与内核**同款匹配语义**（避免自研引擎语义漂移）。
- **成本**：8 人日。**风险**：mihomo 规则语义随版本演进 → 匹配引擎锁同版本 + golden test 对拍（每版本 CI 跑 500 条真实域名样本比对 trace 结论与内核实测命中一致）。
- **依赖验证**：✅ trie 组件实测存在；✅ `/connections` 返回 `rule`/`rulePayload` 字段（见 F2）可提供"真实命中"对照样本。

### F2 规则命中率回流与"死规则"瘦身

**一句话**：通过 mihomo external controller 实时采集每条连接命中的规则与规则集，统计长期命中率，自动生成个人化精简规则集（"你的规则集 62% 从未命中"）与未匹配域名补全建议。

- **生态空白**：Aethersailor Rule-Bot 只做"未匹配域名 → 补规则"单向闭环；**反向瘦身**（死规则剔除）无人做。规则集逐年膨胀（blackmatrix7 单仓库 28k⭐ 的规则文件数以千计），死规则拖累 MRS 体积与内存。
- **设计**：
  - 采集：订阅 mihomo `GET /connections`（WebSocket，默认 1s 快照）——**已验证** `tunnel/statistic/tracker.go` 源码含 `Rule string \`json:"rule"\`` 与 `RulePayload string \`json:"rulePayload"\`` 字段，可精确归属到规则集条目；
  - 聚合：每条规则一个 `atomic.Uint64` 计数器（数组下标 = 编译期规则索引），无锁累加，定时（60s）落 SQLite WAL，内存增量 O(规则数)（千级规则 ≈ 数 KB）；
  - 产物：① 命中率报表（/dashboard 可视化）；② 精简版规则集生成（保留 N 天内命中过的 + FINAL 兜底），输出为独立 MRS，用户一键替换；
  - 隐私：域名明细默认只存聚合后缀（eTLD+1），完整域名记录需显式开启并本地存储。
- **成本**：6 人日。**风险**：长尾域名被误删（新业务首次访问前规则已瘦掉）→ 瘦身版默认"保留模式"（仅标记死规则不删除）+ 用户显式确认；追踪回环（SubForge 自身域名被采集）→ 进程名单过滤。
- **依赖验证**：✅ `/connections` 快照含 `rule`/`rulePayload`（源码实测）；✅ `/logs`、`/traffic`、`/memory`、`/rules`、`/providers/rules` 端点实测挂载于 `hub/route/server.go`。
- **观测点与部署拓扑（v2.1 修订——本功能不适用于"后端天然全知"的假设）**：SubForge 不在数据面上（刻意设计：转换器碰流量 = 延迟与隐私双重灾难），唯一合法观测通道是 mihomo external controller。采集器为同一模块 `internal/observe` 的两种编译形态，按部署拓扑落位：
  | 拓扑 | 部署形态 | 采集方式 | 反馈功能可用性 |
  |---|---|---|---|
  | T1 同机/同网（推荐主路径） | SubForge 与 mihomo 同设备或同 LAN（软路由/NAS 常见形态） | 直连 `http://127.0.0.1:9090` 或 LAN IP + secret，WebSocket 订阅 | ✅ 全功能；域名数据不出内网 |
  | T2 远程后端 | SubForge 在公网 VPS，客户端在家庭出口 | **Beacon**：约 3MB Go 单文件 agent 部署在 mihomo 旁（OpenWrt/PC），本地聚合后 POST `/api/v1/telemetry`（设备 token 沿用 F5 体系） | ✅ 全功能；默认仅上报规则命中计数 + eTLD+1 聚类，原始域名 opt-in |
  | T3 无观测 | 任意 | 无 | ⛔ F2 自动禁用（UI 明示）；F1/F4 静态分析与 F5/F6 不受影响 |
  先例背书：Aethersailor Rule-Bot + Rule-Bot-Client 即"客户端侧采集 → 外送汇总"的已验证形态（跑在 OpenWrt 侧收集 mihomo MATCH 日志后开 GitHub Issue）；subs-check 亦为贴近数据面运行的独立 agent。
- **视角偏差注记**：测活/测速从 SubForge 主机视角发起，T2 部署下延迟数值 ≠ 用户侧体感（仅可作相对比较）；T1 无此问题。Beacon 不做客户端侧测活（范围控制，防 agent 膨胀）。

### F3 Rule DSL v1（声明式规则编译语言 + Lint）

**一句话**：把"模板继承 + 条件覆盖 + 规则集组织"从散落在 URL 参数和 INI 里的 30+ 个开关，收敛为一门可校验、可继承、可版本管理的小型声明式语言。

- **生态空白**：subconverter 的 INI `[ruleset]`+URL 参数组合已被证明难维护（大量公共实例配置不可复现）；Sub-Store 的 JS 脚本算子灵活但不可静态校验。无人提供"编译期校验的声明式分流描述"。
- **设计**（严格限定为"组织语言"，**不做图灵完备**，保持可静态分析）：
  ```yaml
  # profile.yaml —— 四级继承：base → template → subscription → request
  extends: [base, aethersailor-standard]      # 模板继承链，后者覆盖前者
  when:
    ua: contains("ClashMeta")                 # 条件化输出（请求参数谓词，编译期展开）
  groups:
    AI: {filter: 'name =~ "GPT|Claude" && region in [US,SG]', test: true}
  rules:
    - use: acl4ssr@ai                         # 引用规则源清单（见主设计书 §5.3 manifest）
    - exclude: [" gambling ", "porn"]         # 例外剔除
    - include: my-custom.list                 # 本地补充
    - final: 漏网之鱼
  ```
  - **编译器产出**：最终扁平规则表 + 各 target 配置；引用不存在的组/源 → 编译失败；与 F4 联动输出冲突警告；
  - **Lint 库**（静态检查，持续积累）：FINAL 之后的规则永不命中、GEOIP CN DIRECT 前置导致国内 IP 类规则失效、Fake-IP 模式下 IP-CIDR 需 `no-resolve` 否则触发 DNS 泄漏、策略组引用闭环检测。Aethersailor Wiki 里人工总结的坑沉淀为机器可查规则。
- **成本**：DSL 语法 5 人日 + 编译器 10 人日 + lint 规则库 5 人日（可增量）。**风险**：表达力不足导致用户回退裸 INI → 兜底"逃生舱"（允许内嵌任意 subconverter INI 片段，v1 不做 100% 覆盖）；语法设计范围蔓延 → 版本化冻结 v1 特性集，扩展走提案流程。
- **复用**：YAML 解析（gopkg.in/yaml.v3）、现有 manifest 机制、F1 的编译产物即 trace 的输入。

### F4 规则冲突与重叠分析器

**一句话**：编译期对全部规则源做交集/冲突分析——同一域名被多个规则集以不同策略命中时可视化呈现，并量化各上游之间的重叠度。

- **生态空白**：上游 8 家（ACL4SSR、blackmatrix7、anti-AD、Loyalsoldier…）彼此大量重叠且优先级靠排列顺序隐式决定；Aethersailor 的取舍结论只存在于 Wiki 文字，无机器可查的量化工具。无人做规则集重叠度量。
- **设计**：全部 domain 规则载入复用的 domain trie（同一份内存，与 F1 共享），遍历一次即得每域名的命中集合（单次 O(总规则数)）；IP-CIDR 用 ipcidr trie 同理；输出：冲突矩阵（哪个域名 / 被谁命中 / 最终采用谁）、上游重叠率（如 anti-AD ∩ AWAvenue = 38.2%）、体积贡献排名（谁贡献了最多死规则，联动 F2）。
- **成本**：5 人日（trie 复用后主要工作在报表）。**风险**：keyword/generic 语义无法精确交集（正则类）→ 对此类仅做"计数归组"，不做逐条交集，报告标注置信度。
- **依赖验证**：✅ 同 F1 的 trie 组件。

### F5 订阅保险箱（per-device token 审计与防盗链）

**一句话**：每个客户端设备发放独立订阅 token，后端按 token 审计访问源（IP/UA/频率），识别"订阅链接被分享/泄露"并自动告警或轮换。

- **生态空白**：泄露是公共转换后端头号问题（bulianglin psub 专为防盗而生；SCE 提供安全档位但无 per-device 审计维度）。将"防泄露"从部署建议升级为产品功能，生态空白。
- **设计**：token 结构 `sub/{profile}/{device-id}/{hmac-ts}`；每次访问记录 (token, IP, ASN, UA, 频率) 至 SQLite；异常模式（同 token 多 ASN 突增、来自数据中心 ASN 的首次访问）触发可配置动作：`notify`（Shoutrrr 推送）/ `stale`（继续供应旧缓存快照）/ `revoke`；轮换后旧 token 仅返回上次快照（不暴露新内容）。GEOIP 查询复用 mihomo `component/mmdb`（MIT，已随内核引入，零新增依赖）。
- **成本**：6 人日。**风险**：误伤（用户换网络）→ 置信度模型从宽松档起步（仅通知不阻断）；攻击者可伪造 UA → 不依赖 UA 做唯一判据，ASN 突变为主信号。
- **依赖验证**：✅ mmdb 组件在 mihomo component 目录实测存在。

### F6 跨内核输出扩展：sing-box srs（进程隔离）+ DNS 层全家桶

**一句话**：同一套规则源，一键产出 sing-box 的 srs 二进制规则集与 DNS 层分流配置（mosdns / AdGuard Home / dnsmasq），把 SubForge 从"mihomo 生态工具"升维成"多内核规则工厂"。

- **生态空白**：meta-rules-dat（5.3k⭐）证明 sing-box srs 有真实需求，但其生成依赖上游仓库；mosdns/AdGuard 用户目前只能各自找规则，无人提供统一编译出口。
- **设计**：
  - **srs 输出**：sing-box 许可证为 GPL 系（GitHub API 实测 `NOASSERTION`，含附加条款），**不链接其代码**；官方 CLI 文档化提供 `rule-set compile` 子命令 → 以外部进程调用（用户自备 sing-box 二进制或由安装器下载），SubForge 只负责：规则源 → sing-box source JSON（公开文档格式）→ 调进程 → 校验产物哈希。进程隔离彻底规避 GPL 传染（与主设计书对 GPL 的处理原则一致）。若未来探测到环境无 sing-box 二进制，自动降级为仅输出 source JSON 并提示。
  - **DNS 层输出**：规则源编译为 ① mosdns 配置片段（yaml 文本）② AdGuard Home 过滤规则（adblock 语法文本）③ dnsmasq server/conf 列表（`conf-file=/tmp/subforge.conf`+`server=/domain/ip` 行）。均为纯文本生成，且 F4 的冲突分析可顺带发现"上游间 DNS 劫持建议冲突"。
- **成本**：srs 4 人日 + DNS 三格式 4 人日。**风险**：srs 二进制格式随 sing-box 版本演进 → 只做"调用方"不做"格式实现方"，格式演进风险转移给上游 CLI（代价：交付包不含 srs 生成能力，需运行时依赖，已在安装器文档声明）；AdGuard 语法表达力有限（不支持 IP-CIDR 精细分流）→ 明确降级说明：DNS 层只承载"域名类"规则，IP 类留在内核层。
- **依赖验证**：✅ sing-box 许可证与星标实测；rule-set compile 为官方文档化命令（进程隔离设计使其格式细节不构成本项目依赖）。

---

## 2. 快速采纳清单（并入路线图，8 项）

| # | 功能 | 一句话 | 复用 | 成本 | 风险与缓解 | 排期 |
|---|---|---|---|---|---|---|
| Q1 | 订阅 diff 引擎 | 每次拉取与上次对比，事件化（新增/失效/参数变更）节点，推送 Shoutrrr 通知 | 现有解析产物直接比对 | 3 人日 | 大订阅 diff 噪音（机场改名）→ 归一化指纹（服务器+端口+协议+UUID 四元组） | M2 |
| Q2 | 流量账单仪表盘 | 解析 subscription-userinfo 头，历史曲线、到期预警、多机场聚合 | Q1 的事件存储 | 2 人日 | 机场头缺失 → 显示"未上报"，不猜 | M2 |
| Q3 | IPv4/IPv6 双栈可用性标记 | 出站探测分 v4/v6 两轮，节点名注入标记并支持按栈过滤分组 | mihomo 出站 dialer | 3 人日 | v6-only 节点探测失败噪音 → 标记"未知"而非"不可用" | M3 |
| Q4 | 多镜像源自动切换 | raw.githubusercontent/jsdelivr/ghproxy 候选链 + 熔断 + 本地缓存兜底 | Aethersailor repository-mirror 思路 | 2 人日 | 镜像投毒 → 产物哈希校验（Q8 提供） | M2 |
| Q5 | 零拷贝热重载 | 配置变更原子替换（`atomic.Pointer` 双缓冲），在途请求用旧配置完成，无锁读 | 标准库 | 2 人日 | 新旧配置引用混合 → 变更集整体替换（不做字段级合并） | M1 |
| Q6 | Prometheus 指标导出 | /metrics：转换耗时直方图、上游失败率、token 审计计数、规则命中计数（F2 复用） | prometheus/client_golang（Apache-2.0） | 2 人日 | 指标基数爆炸（per-token label）→ token 维度仅保留 Top-N | M3 |
| Q7 | 通用通知/Bot 面 | Shoutrrr 统一出站（Telegram/Discord/SMTP…）；Telegram Bot 提供查询/触发刷新 | Shoutrrr（MIT，Sub-Store 同款先例） | 3 人日 | Bot token 泄露 → token 只存哈希、命令白名单 | M3 |
| Q8 | 规则产物完整性链 | 全部产物 manifest（SHA256 + 生成时间 + 源版本），客户端可校验；Q4 镜像与 F5 轮换共同依赖 | 对齐 SCE 的 RELEASE-MANIFEST 实践 | 2 人日 | 无人校验形同虚设 → 提供一行 curl 校验脚本模板 | M2 |

## 3. 远期观察（给出触发条件，6 项）

| # | 功能 | 一句话 | 触发条件（满足即启动） | 主要代价 |
|---|---|---|---|---|
| Y1 | relay 链路组合推荐 | 在允许的组合预算内（≤50 组合/轮）自动尝试"入口→中转→出口"，输出最优 relay 链 | F2 的延迟时序数据积累满 30 天 | 探测流量对机场的扰动；组合爆炸已用预算上限封顶 |
| Y2 | wazero WASM 插件系统 | 社区以 WASM（wazero 运行时，MIT、纯 Go、零 cgo）编写自定义 filter/operator | M3 稳定后且有 ≥3 个真实社区诉求 | 插件 ABI 设计 10 人日；插件错误隔离与资源限制 |
| Y3 | 浏览器离线转换（PWA） | 前端内嵌 WASM 子集解析器，纯链接转换不过服务器 | 与主设计书 R2 的 Worker 轻模式**共用同一子集解析器**（一处实现两处复用，摊薄双解析器维护代价） | 子集解析器覆盖 ~95% 场景，长尾协议缺失需明示 |
| Y4 | 规则产物 ed25519 签名链 | 在 Q8 manifest 之上叠加私钥签名，防镜像篡改升级为可验证防篡改 | Q4 镜像源数量 ≥3 且出现真实投毒事件 | 密钥托管与轮换运维成本 |
| Y5 | 多实例联邦 | 软路由/NAS/VPS 多实例指标聚合到单一 dashboard | ≥10 个用户自述多实例部署（issue 统计） | 聚合协议设计；P2P 打洞复杂度（先只做 Pull 型） |
| Y6 | 增量规则构建 | 服务端缓存 base，上游更新仅重建 delta 后合并 | 单次 MRS 构建 p95 > 2s 的实测证据 | 一致性风险（base/delta 版本错配）→ 以构建 ID 关联 |

## 4. 明确不做清单（附理由，5 项）

| # | 提案 | 拒绝理由 | 替代方案 |
|---|---|---|---|
| N1 | eBPF 内核态分流匹配 | 职责越界（SubForge 是配置编译器/订阅面，不是转发面）；eBPF 引入内核版本耦合与特权要求，与"单二进制低依赖"目标冲突 | 分流匹配交给 mihomo 内核；SubForge 保证喂给内核的规则最精简（F2/F4） |
| N2 | AI/LLM 域名分类 | 引入 100MB+ 模型违背内存预算（<30MB RSS）；域名上云有隐私风险 | 本地启发式：公共后缀聚类 + 编辑距离分桶（F4 已覆盖归类需求） |
| N3 | P2P/IPFS 规则分发 | 需求无证据（现有镜像链 Q4 已解决可用性）；引入 NAT 穿透与种子维护负担 | Q4 多镜像 + 哈希校验 |
| N4 | 内置转发/代理能力 | 与 mihomo/sing-box 职责重叠，会把自己变成竞品且放大攻击面；SCE 的教训是"后端不碰节点流量"原则 | 保持纯编译器定位，转发面永远外置 |
| N5 | 自造私有协议/混淆支持 | 协议层已高度内卷，自研协议无互操作价值且合规风险高 | 全部协议能力复用 mihomo（MIT 嵌入），跟进上游即可 |

## 5. 路线图修订（旗舰与采纳项的落位）

| 里程碑 | 原范围（主设计书 §10） | 新增 |
|---|---|---|
| M1 内核复用（4 周） | Fetcher/Parser/Transformer/Renderer + 兼容 API | +Q5 零拷贝热重载 |
| M2 生态兼容（3 周） | INI 兼容 / Provider 透传 / Web UI | +Q1 diff、Q2 账单、Q4 镜像、Q8 manifest、**F3 DSL v1（编译器主体）** |
| M3 规则工厂（3 周） | manifest 聚合 + MRS 生成 + 定时任务 + Rule-Bot 闭环 | +F1 trace、F2 命中率回流、F4 冲突分析、Q3 双栈、Q6 指标、Q7 通知 |
| M4 打磨（2 周） | 诊断面 + 性能基准 + Worker 轻模式二选一 | +F5 保险箱、F6 srs/DNS 输出、F3 lint 规则库 |
| M5（新增，社区驱动） | — | 按 Y1~Y6 触发条件逐项启动 |

> 修订理由：F1/F2/F4 共享同一份 trie 与编译产物，落同一里程碑可摊薄集成成本（合计节省约 4 人日）；F5/F6 是防御性与生态位功能，放 M4 不阻塞核心。

## 6. 新增风险登记（接主设计书 §7，编号顺延）

| ID | 风险 | 概率 | 影响 | 缓解 | 残余代价 |
|---|---|---|---|---|---|
| R13 | trace 匹配语义与 mihomo 内核版本漂移 | 中 | 中 | 锁同版本 + golden test 对拍 CI | 每次内核升级需重跑对拍（~0.5 人日/次） |
| R14 | 命中率采集在长连接下计数膨胀 | 低 | 低 | 计数器数组 + 60s 快照落盘，内存 O(规则数) | 极端规则数（10k+）时快照延迟需实测调参 |
| R15 | DSL 语法被滥用为图灵完备脚本（用户塞循环逻辑） | 中 | 中 | v1 无循环/无变量赋值，编译期强制终止性 | 表达力天花板，逃生舱内嵌 INI |
| R16 | sing-box srs 随版本格式演进导致产物失效 | 中 | 低 | 只调用官方 CLI 不实现格式；产物哈希校验失败即降级 source JSON | 用户需自备 sing-box 二进制 |
| R17 | per-device 审计存储增长（SQLite 无限增长） | 中 | 低 | 明细表 90 天滚动清理，聚合表常驻 | 历史审计完整性上限 90 天（可配置） |

## 7. 与主设计书的衔接

- 主设计书 §6（新功能发散清单 v1）的 A1~A10 全部保留，本文 F 系列是其深化与扩展：A1→F3，A3→Q3，A4→F2，A7→Q1/Q2，其余不变。
- 性能预算不变（主设计书 §8），新增功能中 F2/F4/Q5/Q6 的内存增量均已标注（合计 <5MB），M1 基准测试时纳入同一份 pprof 报告。
- 许可证矩阵不变：本文所有新增复用项（trie/mmdb/trie/shoutrrr/client_golang/wazero）均为 MIT/Apache-2.0/ISC 族，与 MIT 主许可兼容；sing-box 仅进程隔离调用，不引入 GPL 传染。
