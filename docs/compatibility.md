# 兼容性已知差异表

与 subconverter / SubConverter-Extended（SCE）的**已知行为差异**记录处。
金测集（`testdata/golden/`）通过与它们对拍维护；无法对齐或有意不对齐的行为记录于此。

原则（对齐设计书 R8 的残余代价）：

- 差异必须**可机器复现**（fixture + 参数 + 双方输出片段）。
- 每项标注：影响面、首见版本、处置（对齐 / 明示差异 / 待定）。

## 参数与 API

| # | 差异点 | subconverter/SCE 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| — | （暂无，M1-I8 金测开始时填充） | | | |

## INI 模板语法

| # | 差异点 | subconverter 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| — | （暂无） | | | |

## 各 target 渲染

| # | target | 状态 |
|---|---|---|
| — | `clash.meta` | 已落地，经 `mihomo -t` 实机校验 |
| — | `list` / `mixed` / `ss` / `ssd` / `ssr` / `trojan` / `v2ray` | 已落地（差异见 S4 / S5） |
| — | `surge` / `surge2` / `surge3` / `surge4` / `suri` | 已落地（差异见下节 C1~C5） |
| — | `quan` / `quantumult` | 已落地 |
| — | `quanx` / `quantumultx` | 已落地 |
| — | `loon` | 已落地 |
| — | `surfboard` | 已落地 |
| — | `ss-sub` / `ss-sub-json` / `ssjson` / `ss-sub-b64` | 已落地（SS-android 节点配置） |
| — | `sing-box` | 未落地（进程隔离调用，独立设计项，见 R2） |
| — | `mellow` / `sstable` | 未落地（各自有独立的分段格式，待实现） |
| — | `clash-json` / `clashmeta-json` | 未落地（I7e 待补） |

## SS-android 节点配置（ss-sub）

SS-android 吃的是 **JSON 数组**而不是分享链接，每个节点一组
`server` / `server_port` / `method` / `password` 字段，外加一套与节点无关的
应用级默认值（路由预设、远程 DNS、代理应用白名单）。
字段顺序按 subconverter 的实机输出固定——SS-android 反序列化时不挑顺序，
但 golden 要字节级稳定，所以不交给 `encoding/json` 的 map 排序，而是手工编码
（`internal/render/sssub`）。

只支持 SS 与 SSR：这两个协议才有 `server` / `method` 的语义，vmess / vless /
trojan 等直接跳过并逐节点告警。节点数与告警照常在 `X-Node-Count` /
`X-Convert-Skipped` 里上报。

target 别名归一：`ss-sub`、`ss-sub-json`、`ssjson` 输出裸 JSON（与
subconverter 一致）；`ss-sub-b64` 输出 base64 包裹，那是 SS-android 的
订阅入口格式。四者渲染器名都是 `ss-sub`——`X-Convert-Format` 与落盘名
才保持一致。

SS-android 没有代理组与分流规则的概念，`Groups` 与 `Rules` 对该形态不生效，
引擎照常计算但渲染器不消费。

## 一行一节点配置形态（surge / quan / quanx / loon / surfboard）

Surge、Quantumult classic、Quantumult X、Loon、Surfboard 共享同一套骨架
（节点段落 + 代理组段落 + 规则段落），差异只在字段顺序、引号约定、节点名放置
位置与规则词汇，因此只写一份组装逻辑（`internal/render/lineform`），每个格式
贡献自己的节点 / 组 / 规则映射。

段落名对照（`[ ]` 包裹，QX 系列除外）：

| 格式 | 节点段 | 组段 | 本地规则段 | 远程规则段 |
|---|---|---|---|---|
| surge | `[Proxy]` | `[Proxy Group]` | `[Rule]` | — |
| quan | `[SERVER]` | `[POLICY]` | `[TCP]` | — |
| quanx | `[server_local]` | `[policy]` | `[filter_local]` | `[filter_remote]` |
| loon | `[Proxy]` | `[Proxy Group]` | `[Rule]` | `[Remote Rule]` |
| surfboard | `[Proxy]` | `[Proxy Group]` | `[Rule]` | — |

节点名放置：Surge / QX classic / Loon / Surfboard 写在等号左边
（`名字 = 协议字段`），Quantumult X 写在 `tag=` 字段里（`协议 = host:port, ..., tag=名字`）。

协议支持面（不支持的协议跳过并逐节点告警，不硬塞）：

| 格式 | ss | ssr | vmess | trojan | vless | hy2 | tuic | anytls | socks/http |
|---|---|---|---|---|---|---|---|---|---|
| surge | ✅ | — | — | — | — | — | — | — | ✅ |
| quan | ✅ | ✅ | ✅ | — | — | — | — | — | — |
| quanx | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — |
| loon | ✅ | ✅ | ✅ | ✅ | — | — | — | — | — |
| surfboard | ✅ | — | ✅ | ✅ | — | — | — | — | ✅ |

SubForge 相对 subconverter 更宽的两处：Surge / Surfboard 额外导出 socks5 与
http（两者的语法都原生支持）；Quantumult classic 在 vless 上没有产出，与
subconverter 一致。

### 已知差异

| # | 差异点 | subconverter v0.9.0 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| C1 | 产物体积 | 走完整模板，含 `[DNS]` / `[REWRITE]` / `[URL-REJECTION]` / `[Script]` / `[BACKUP-SERVER]` 等十余段 | 只产出节点 + 组 + 规则三段（Surge 额外带 `[General]`） | 有意差异：模板系统（I6）能覆盖的段落应走模板配置，而不是硬编码一份第三方模板。想要 subconverter 那种全量产物需配 INI 模板 |
| C2 | 代理组数量 | 十余个预置组（媒体分流 / 微软服务 / 广告拦截…） | 未配置组时只生成两个兜底组：`节点选择`（select）、`自动测速`（url-test） | 有意差异：预置组语义属于模板内容。兜底组保证产物开箱可用，且成员只引用本次产物真实存在的节点名，不产生悬空引用 |
| C3 | 收尾规则 | 模板决定（常见 `MATCH`） | 未配置规则时生成 `GEOIP,CN,DIRECT` + `FINAL,<首个 select 组>` | 有意差异：`FINAL` 是这四个客户端的通用兜底语法，mihomo 的 `MATCH,<策略>` 写进去不识别 |
| C4 | 组成员排序 | 保持订阅里的节点顺序 | 兜底组内按节点名字典序 | 有意差异：保证 golden 字节级稳定（map 遍历无序）。带模板配置时组按模板给的顺序走，不受影响 |
| C5 | 组名 emoji | 大量 emoji 前缀（🎯 🚀 ♻️ …） | 纯文本组名（`节点选择` / `自动测速`） | 有意差异：这几个客户端对组名字符集限制比 mihomo 严，emoji 在部分版本会导致组解析失败。clash.meta 的默认组保留 emoji |
| C6 | IP-CIDR 的 `no-resolve` | QX 与 Surfboard 的 IP-CIDR 追加 `no-resolve`，Surge 不加 | 与 subconverter 一致（QX / Surfboard 加，Surge 不加） | 对齐。参考：`tools/scv/goldens2/{surge,quan,surfboard}.out` |
| C7 | `relay` / `balanced` / `sniffer` 组类型 | 无（mihomo 专属类型，subconverter 同样无法在这些格式表达） | 跳过并计入告警 | 对齐。这四个客户端只有 `select` / `url-test` / `fallback`（QX 为 `static` / `url-latency-*`）与 `direct` / `reject` 出站，没有链式组合的组类型。Quantumult X 连 `fallback` 都没有，`fallback` 组在 quanx 下被跳过 |

规则词汇映射（规则集写法各异，规则类型本身 1:1 透传）：

| 格式 | 规则集写法 | 落点 |
|---|---|---|
| surge | `RULE-SET,url,组,update-interval=N` | 本地规则段 |
| quan | `RULE-SET,url,组,update-interval=N` | 本地规则段（`[TCP]`） |
| quanx | `rule-list=url,group=组,update-interval=N` | `[filter_remote]` |
| loon | `url,组` | `[Remote Rule]` |
| surfboard | `RULE-SET,url,组,update-interval=N` | 本地规则段 |

`GEOIP` 原样透传（subconverter 在这三个格式的产物里也原样出现），`GEOSITE`
是内核专属、无法表达，丢弃并计入告警。规则无法表达时不静默：
`Warnings` 逐条列出被丢的规则与节点。

## 分享链接（`list` / `target=list`）

内核的 outbound 适配器没有 Marshal 方法，分享链接序列化是自研的（`internal/render/sharelink`）。
正确性的兜底不是文本比对，而是**往返 oracle**（`internal/parse/roundtrip_test.go`）：
产物再喂回内核 `convert.ConvertsV2Ray` 解析，比对解析出的配置 map。链接文本的格式细节
（查询参数顺序等）不参与断言。

已覆盖协议（内核规范化后的类型名）：vmess、vless、trojan、ss、ssr、hysteria、hysteria2、
tuic、anytls、socks5、http。不支持的协议（wireguard、ssh 等）不中断整批导出：跳过并记名，
通过 `X-Convert-Skipped`（HTTP）或 stderr（CLI）上报，产物里只留可序列化的节点。

## 协议收窄的 link target（`ss` / `ssd` / `ssr` / `trojan` / `v2ray` / `mixed`）

这些 target 产出的是分享链接而不是客户端配置，全部经整体 base64 包裹（与 subconverter 一致），
每条链接后补一个换行。协议收窄在引擎侧落成类型过滤（`internal/convert/engine.go` 的
`resolveTarget`），而不是丢给渲染器再切一遍——否则 `X-Node-Count` 统计的节点数与产物行数对不上。

收窄取值来自 subconverter v0.9.0 实机输出（`tools/scv/`，GPL-3.0 仅作参照）：

| target | subconverter 输出的节点集 | SubForge |
|---|---|---|
| `mixed` | 全部存活协议 | 一致 |
| `ss` / `ssd` | ss **与 ssr** 节点 | 节点集一致；差异见 T1 |
| `ssr` | 仅 ssr | 一致 |
| `trojan` | 仅 trojan | 一致 |
| `v2ray` | 仅 **vmess**（不是 V2Ray JSON） | 一致 |
| `vmess` / `base64` / `sing-box` / `list` | **400 `Invalid target!`** | 视为未支持 target |

`-type` 与 target 的隐含收窄同时出现时，`-type` 更具体、胜出。

### 类型名归一（重要）

mihomo 写进节点 `type` 字段的值是分享链接的 scheme（`converter.go` 里基本是
`x["type"] = scheme`），少数硬编码：`hysteria2`、`socks5`、`anytls`、`mieru`（mihomo 自己的拼法）。
因此 `ss` 不是 `shadowsocks`、`ssr` 不是 `shadowsocksr`。写错名的后果是**静默匹配零节点**，
请求退化成"没有可用节点"，用户无从分辨是拼错还是订阅为空。所以 `-type` 在构造期先过别名表
（`internal/transform/filter.go` 的 `kernelTypes`）：

`shadowsocks`→`ss`、`shadowsocksr`→`ssr`、`socks`/`socks4`/`socks5h`→`socks5`、
`https`→`http`、`hy2`/`hysteria2+realm`→`hysteria2`、`mierus`→`mieru`、`v2ray`→`vmess`。

别名只作用于用户输入，匹配端始终只认内核真名，这样 `X-Node-Count` 与产物口径恒等。
未收录的名字原样保留——拼错就让它暴露成空结果，不猜。

### 已知差异

| # | 差异点 | subconverter v0.9.0 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| S1 | 分享链接输入 | **完全不解析**：vmess / vless / ssr / trojan / ss / hysteria2 / tuic 等 12 条链接全部返回 `Invalid subscription`，包括 v2ray 官方测试节点 | 全部识别 | SubForge 更宽。对拍金测时不要用 subconverter 做分享链接的输入基线 |
| S2 | Clash YAML 输入的协议覆盖 | 静默丢弃 vless、hysteria、hysteria2、tuic、anytls，只保留 vmess / ss / ssr / trojan / socks5 / http | 全部保留 | SubForge 更宽。I8 对拍时按此判断，不要把 subconverter 的丢节点当成自己的转换错误 |
| S3 | 不支持协议的导出 | 视版本而定，多数实现报整份失败 | 跳过并上报计数与名单，可序列化的节点照常导出 | 有意差异：静默失败与整份失败都不如"降级 + 明示" |
| S4 | `ss` target 里的 ssr 节点 | **降级**成 `ss://` 链接、保留原密码算法 | 保留原生 `ssr://` 形态（protocol + obfs 语义无法无损压进 ss 链接） | 有意差异：SS-only 客户端读到 `ssr://` 会报错，但降级会静默产生错密码的节点。I7d 若要做纯 SS 客户端 target，应新增一个显式 target 而不是让 `ss` 悄悄降级 |
| S5 | SSR payload 形态 | `…passwd=<b64>=/P2group=<base64>/…&remarks=<b64>&obfsparam=&protoparam=` | `host:port:protocol:method:obfs:<urlSafe密码>/?remarks=<urlSafe名>` | mihomo 的解析靠 `strings.Cut(payload, "/?")` 切分，**读不了 subconverter 的 `/P2group=` 写法**——即 mihomo 拒绝 subconverter 自己的 SSR 输出。SubForge 走内核真名，实机往返通过 |

### 内核解析器约束（v1.19.31，逐条读源码核实）

这些是序列化必须遵守的字段映射，不是设计选择。

| # | 约束 | 出处 |
|---|---|---|
| L1 | vmess JSON 里 `servername` 的键是 **`sni`**，不是 `sn` | `common/convert/converter.go` vmess 分支 |
| L2 | vless 的 `sni` → `servername`；`security` 后缀 `tls` / `reality` 才置 `tls: true`；`fp` 只在 tls 时生效（默认 `chrome`） | `common/convert/v.go` |
| L3 | ssr payload 的 password / remarks 内层 base64 **不能带 `=` 填充**——`decodeUrlSafe` 用 `RawURLEncoding` 解码，带填充会静默返回空串，password 与 remarks 整段丢失 | `common/convert/base64.go` |
| L4 | userinfo 只有一侧有值时必须用 `url.User` 形态：`url.UserPassword` 会给空口令补一个冒号，而 ss / trojan / hysteria2 用 `User.Password()` 判断口令是否存在，多出的冒号会让整条链接失效 | 同 L1 各分支 |
| L5 | http / socks 分支**不读任何查询参数**：`skip-cert-verify` 恒为 `true`，`tls` 只看 scheme 是不是 `https`，`sni` 直接丢弃。序列化因此用 scheme 表达加密，不输出 `tls` / `insecure` 参数 | `converter.go:546` 起 |
| L6 | http / socks 分支会先对 userinfo 做 base64 解码、失败才回落到明文。纯 base64 字面量的用户名（如恰好 4 字符）会被内核误读——这是内核的缺陷，明文 userinfo 仍是各客户端通用约定，SubForge 沿用明文 | 同 L5 |
| L7 | vless 的 `flow` 内核不读，但序列化仍输出——真实客户端（Xray / v2rayN）需要它 | `v.go` |
| L8 | vmess JSON 同时写 `sn` 与 `sni`：内核只读 `sni`，V2RayN 等客户端只读 `sn` | L1 |

### 已知差异

| # | 差异点 | subconverter v0.9.0 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| S1 | 分享链接输入 | **完全不解析**：vmess / vless / ssr / trojan / ss / hysteria2 / tuic 等 12 条链接全部返回 `Invalid subscription`，包括 v2ray 官方测试节点 | 全部识别 | SubForge 更宽。对拍金测时不要用 subconverter 做分享链接的输入基线 |
| S2 | Clash YAML 输入的协议覆盖 | 静默丢弃 vless、hysteria、hysteria2、tuic、anytls，只保留 vmess / ss / ssr / trojan / socks5 / http | 全部保留 | SubForge 更宽。I8 对拍时按此判断，不要把 subconverter 的丢节点当成自己的转换错误 |
| S3 | 不支持协议的导出 | 视版本而定，多数实现报整份失败 | 跳过并上报计数与名单，可序列化的节点照常导出 | 有意差异：静默失败与整份失败都不如"降级 + 明示" |

## 规则集输出（/getruleset）

`type` 沿用 subconverter 的 1~6 编号，也接受命名别名
（`surge` / `qx` / `clash-domain` / `clash-ipcidr` / `surge-domain-set` / `clash-classical`）。

规则内容的取值顺序：`rules` 参数 > `list` 参数 > 模板 `[common] rules` > `url` 远程拉取。
类型的取值顺序：`type` 参数 > 模板 `[ruleset]` 段落里该名字的类型。

### 六种形态保留的规则

| 编号 | 形态 | 扩展名 | 保留的规则形态 | 丢弃 |
|---|---|---|---|---|
| 1 | Surge list | `.surge-rules` | DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / DOMAIN-REGEX | IP 网段、GEOIP、GEOSITE、MATCH |
| 2 | Quantumult X | `.qxt` | 同上 + IP-CIDR / IP-CIDR6 | GEOIP、GEOSITE、MATCH |
| 3 | Clash domain provider | `.yaml` | 四种域名形态 | IP 网段、GEOIP、GEOSITE、MATCH |
| 4 | Clash ipcidr provider | `.yaml` | IP-CIDR / IP-CIDR6 | 全部域名形态、GEOIP、GEOSITE、MATCH |
| 5 | Surge DOMAIN-SET | `.txt` | DOMAIN / DOMAIN-SUFFIX | 关键词、正则、IP 网段、GEOIP、GEOSITE、MATCH |
| 6 | Clash classical | `.yaml` | 全部（含 MATCH / FINAL） | 无 |

丢弃条数不会静默消失：响应头 `X-Ruleset-Skipped` 报实际丢弃数，
`X-Ruleset-Count` 报实际写出数。

### 已知差异

| # | 差异点 | subconverter 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| R1 | 规则数据不打包 | 内置规则集可离线使用 | 一律运行时提供（`rules` / `url` / 模板）；GPL、CC-BY-SA 的规则数据不静态打包（R3） | 有意差异。GEOIP / GEOSITE 只写数据集标识（如 `GEOIP,CN`），数据由客户端自带或另配 |
| R2 | mihomo MRS 二进制规则集 | 无此项 | M1 不产出。嵌入的 mihomo 版本未包含 `rules` / `convert-ruleset` 包，且它是独立设计项 | 待定（M3） |
| R3 | 收尾规则 | 按客户端词表解释 | `MATCH` 与 `FINAL` 都原样透传 | 见下方 K2：mihomo 只认 `MATCH`，贴进 mihomo 配置前要确认 |
| R4 | 规则行第三段 | 按目标客户端解释为动作或出站 | 原样保留，不做词表转换 | 有意保留：同一份规则集喂给 Surge 是动作、喂给 Clash 是出站名，我们不改写 |
| R5 | Surge list 的 `[General]` 段 | 视模板而定 | 固定输出 `CNAME = 1` | 有意：域名规则集的标准写法，让客户端检查 CNAME 解析结果 |
| R6 | Quantumult X 的 `expires` | 由模板决定 | 固定 2592000（30 天） | 有意但保守：这是 QX 远程提供方文档的约定值。想要实时生效需自建模板套壳 |
| R7 | DOMAIN-SET 的关键词 | 部分实现会原样写进去 | 丢弃（计入 Skipped） | 有意：DOMAIN-SET 是纯域名列表，客户端按域名匹配，关键词条目无意义 |
| R8 | 裸域名行 | 按 DOMAIN 处理 | 同（仅当取值看起来是纯域名时） | 对齐 |
| R9 | 不带类型前缀的规则行 | 按 Surge 写法 `example.com,PROXY` 处理 | 报 400 | 有意：把第一段当域名会让 `DOMAIN.SUFFIX,com` 这类笔误静默变成 `DOMAIN,com`。写清楚前缀 |
| R10 | 经典形态的默认出站 | 无此项 | 内核要求每条规则都带出站（缺了 `mihomo -t` 报 `rules[i] error: format invalid` 拒载）。用 `out=<组名>` 给缺第三段的规则补默认出站；不传 `out` 时若有规则缺出站，直接报 400，不产出内核拒载的配置 | 有意。`MATCH,<组>` / `FINAL,<组>` 的第二段本身就是出站，不参与补齐 |
| R11 | 响应缓存头 | 有 | 无 | 待定（I9 的缓存与 ETag 方案落地时一并处理） |
| R12 | 模板段落名大小写 | 不敏感 | 敏感（`[COMMON]` 匹配不到 `common`） | 待定：会改已稳定的 INI 解析器核心，暂不动 |

## 内核行为（mihomo v1.19.31，实机核实）

这两条是 `mihomo -t` 实机校验直接测出来的，直接影响产物能否被内核加载。
复现：`SUBFORGE_MIHOMO=<mihomo 路径> make mihomo-check`（或直接跑
`scripts/validate-mihomo.sh`），Go 侧等价门控是 `TestRender_MihomoValid`。

| # | 内核事实 | 证据 | 影响 | 处置 |
|---|---|---|---|---|
| K1 | proxy-group 只支持 4 种类型：`select` / `url-test` / `fallback` / `load-balance`。没有 `reject` / `direct` / `pass`，`relay` 已移除。 | `adapter/outboundgroup/parser.go:187` 的类型 switch；`-t` 报 `proxy group[4]: unsupported type: reject` | "拒绝访问"这类分组必须写成 `type: select` 套内置 `REJECT` / `REJECT-DROP` 出站 | clash.meta 渲染器默认组已按此生成；渲染期用类型白名单拦截，写非法类型直接报错 |
| K2 | 兜底规则是 `MATCH`，**不是** `FINAL`。`FINAL,<组>` 会被判为 `rules[i] error: format invalid`。 | `rules/parser.go` 无 FINAL 分支；`rules/common/base.go:47` 里无 payload 的类型只有 `MATCH` 与 `NOT/OR/AND/SUB-RULE/DOMAIN-REGEX/PROCESS-*-REGEX` | 所有以 `FINAL` 结尾的老模板在 mihomo 下不可加载 | 默认规则用 `MATCH`；渲染期不做静默改写，待 INI/规则集层（I6）再决定是否为老模板做 FINAL→MATCH 归一 |

K1、K2 也意味着：subconverter 的 clashmeta 模板若含 `FINAL` 或 `reject` 类型分组，
其产物在 mihomo 下同样不可加载。M1-I8 用 subconverter 二进制对拍时按此判断，
不要把内核自身的拒绝当成 SubForge 的转换错误。
