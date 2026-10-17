# 兼容性已知差异表

与 subconverter / SubConverter-Extended（SCE）的**已知行为差异**记录处。
金测集（`testdata/golden/`）通过与它们对拍维护；无法对齐或有意不对齐的行为记录于此。

原则（对齐设计书 R8 的残余代价）：

- 差异必须**可机器复现**（fixture + 参数 + 双方输出片段）。
- 每项标注：影响面、首见版本、处置（对齐 / 明示差异 / 待定）。

## 参数与 API

| # | 差异点 | subconverter/SCE 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| A1 | 订阅地址自带查询串（`url=http://…?token=…`） | 按 `?` 拆分多来源，查询串作为该来源的一部分 | 已修复：`url` 不再按 `?` 拆，只认 `;` 和重复参数（`TestParseQuery_URLWithQueryString`）。此前按 `?` 拆会丢掉 `?token=`，上游收不到凭证直接 404，并多出一个 `token=…` 的假来源 | 已对齐 |
| A2 | 未实现的 target | 返回错误体 | 501 Not Implemented，且在**任何上游拉取之前**判定（`TestConvert_UnsupportedTargetSkipsFetch` 断言上游请求次数为 0） | 已对齐 |
| A3 | `config` 参数指向远程 URL | 拉取并应用 | 已支持：CLI `-config` 与 HTTP `config` 参数同语义，带 `://` 即按远程模板拉取 | 已对齐 |
| A4 | INI `[common] url=` 里写订阅地址 | 生效，作为默认订阅来源 | **不支持**：`url` 只从 HTTP query 参数 / CLI `-url` 取，INI 里的 `url=` 被当作普通转换参数透传，不会成为来源 | 待定（M2） |

## INI 模板语法

| # | 差异点 | subconverter 行为 | SubForge 行为 | 处置 |
|---|---|---|---|---|
| N1 | `[custom]` 里的 `custom_proxy_group=` 内联分组 | 生效：反引号 DSL（`` `select`` / `` `url-test``、成员前缀 `[]`、url-test 参数 `测试URL`间隔`容忍度,最大失败`） | 已对齐：同套 DSL；同一段落里重复的同名键**全部保留**（公开模板里同一个键写 56 次，压成一个等于丢弃 55 个分组） | 已对齐 |
| N2 | `[custom]` 里的 `ruleset=` 内联规则 | 生效：`ruleset=<组>,<类型>,<值>[,间隔]`，类型含 `[]GEOSITE` / `[]GEOIP` / `clash-domain:` / `clash-classic:` / `[]FINAL` | 已对齐：直映类型按声明顺序成行，远程类型拉取后在**原位**展开（`MATCH` 之前的规则集不能被挪到 `MATCH` 之后，否则整段规则失效） | 已对齐 |
| N3 | `enable_rule_generator` / `overwrite_original_rules` | `enable_rule_generator=true` 把远程规则集展开成内联 `GEOSITE,` / `IP-CIDR,` / `DST-PORT,` 行；`overwrite_original_rules=true` 用生成结果替换原有规则 | 效果等价：远程规则集一律展开成内联规则行，这两个开关不再需要判断（mihomo 的 `rules:` provider 只吃本地文件路径，不吃 URL，所以没法原样写成 `!!import`） | 已对齐 |
| N4 | 远程规则集（`clash-domain:` / `clash-classic:`） | 拉取并在本地展开或写成 `!!import` | 已对齐：拉取后在规则流原位展开；拉取失败或内容异常（非文本字节，多为上游 CDN 返回压缩壳）时跳过该规则集并逐条告警，不阻断整次转换 | 已对齐 |
| N5 | `[common]` 里的 `!!import` | 拉取并合并远程模板 | 已支持，走 `LoadTemplate` 递归加载 | 已对齐 |
| N6 | 成员正则里的 PCRE 环视断言（`(?<=…)` / `(?<!…)` / `(?=…)` / `(?!…)`） | 生效：subconverter 用回溯正则引擎（`regexp2`），断言按真值求值 | 部分对齐：`^(?!.*X).*` 这种整段取反翻译成"不匹配 X"（语义等价）；其余断言直接剥掉，匹配范围只会放宽不会收紧 | 有意差异（Go RE2 无回溯，见下节） |
| N7 | 分组成员正则命中不了任何节点 | 落 `DIRECT` | 已对齐：落 `DIRECT`。留空或保留正则串都会让 mihomo 报 `proxy group[i]: … not found` 拒绝加载整份配置 | 已对齐 |
| N8 | 顶层默认项 | `port: 7890` / `socks-port: 7891` / `allow-lan: true` | `mixed-port: 7890` / `allow-lan: false`，另加 `ipv6: false` / `unify-ip: true` / `tcp-concurrent: true` / 内置最小 `dns:` | 有意差异：混合端口是 mihomo 的现代默认写法；`allow-lan: false` 对一台订阅转换服务更安全 |
| N9 | 上游 `.mrs`（mihomo 二进制规则集） | 静默丢弃：产物里 0 条 `DOMAIN` 规则，不留痕迹 | **已对齐**：识别 zstd 魔数后走 MRS 解码（`internal/parse/mrs.go`），域名集合展开成 `DOMAIN` / `DOMAIN-SUFFIX` / `DOMAIN-REGEX`，网段集合展开成 `IP-CIDR` / `IP-CIDR6`；解码结果与 `mihomo convert-ruleset` 逐行一致。解码失败或行为不支持时仍逐条告警 | 已对齐 |

### N1~N4 对拍记录：Aethersailor `Custom_Clash_Full.ini`

用最流行的公开模板之一 [Aethersailor Custom_Clash_Full.ini](https://github.com/Aethersailor/Custom_OpenClash_Rules/blob/main/cfg/Custom_Clash_Full.ini) 对拍（真实 49 节点订阅，`target=clash`，双方各产出一份产物）。这份模板把全部 56 个分组和 93 条规则都写在 `[custom]` 段落的内联语法里，完全没有用 `[clash_proxy_group]` / `[ruleset]` 段落，是 N1~N4 的一次全覆盖实测。

| 层 | SubForge | subconverter v0.9.1-36573b3 |
|---|---|---|
| 节点 | 49，名称 / 顺序 / 类型逐条一致 | 49 |
| 代理组 | 56，**名称、顺序、成员清单全部一致**（合计 2,546 个成员位，逐位相同） | 56 |
| 规则 | 93，**逐行一致**；形态分布 `{GEOSITE:43, IP-CIDR:34, GEOIP:7, DST-PORT:4, DOMAIN-SUFFIX:3, SRC-PORT:1, MATCH:1}` | 93，同分布 |
| `mihomo -t` | **通过** | **失败**：`yaml: invalid trailing UTF-8 octet` |

**subconverter 自己的产物不可加载**：那份 93,774 B 的 YAML 里有非法 UTF-8 尾字节（来源是一个 `socks5` 节点的 `username` 字段含二进制垃圾），mihomo 直接 `yaml: invalid trailing UTF-8 octet` 拒绝加载。把它替换成合法字节之后又撞上 `yaml: line 52: did not find expected ',' or '}'`。SubForge 的产物是严格合法 UTF-8，能正常加载。

### N9 说明：MRS 二进制规则集的解码口径

`.mrs` 是 mihomo 的二进制规则集格式（Meta Rule Set），不是文本清单。Aethersailor 的模板里
`clash-domain:<url>.mrs` 就指向它。解码链路：

| 环节 | 做法 |
|---|---|
| 形态识别 | 只认 zstd 帧魔数 `28 b5 2f fd`（MRS 文件整体是 zstd 流），据此与文本规则集分流 |
| 解压 + 头 | zstd 解压后依次读 `MRS\x01` 魔数、行为字节、条数、扩展段长度 |
| 载荷 | 行为 0（domain）走内核 `component/trie.ReadDomainSetBin`；行为 1（ipcidr）走 `component/cidr.ReadIpCidrSet`。行为 2（classical）内核也没有二进制形态，直接报错 |
| 文本化 | 复刻内核 `domainStrategy.DumpMrs`：精确条目与后缀条目成对合并成 `+.<domain>`，再按完整文本排序 |
| 展开成规则行 | `+.d` / `.d` → `DOMAIN-SUFFIX,d`；`d` → `DOMAIN,d`；`*.d` → `DOMAIN-REGEX,^[^.]+\.d$` |

口径与验证：

- **与内核逐行一致**：`internal/parse/mrs_test.go` 拿真实 `.mrs` 与 `mihomo convert-ruleset domain mrs` 的输出做逐行对拍（992 行），任何解码偏差都会红。
- **`.d` 略微放宽**：内核语义是"仅子域"，这里映射成 `DOMAIN-SUFFIX` 会连 `d` 本身一起命中。精确表达要 `DOMAIN-REGEX`（线性扫描，几万条规则会明显变慢），而多命中的 `d` 与列表里其余条目同属一份规则集、路由到同一个组，代价可忽略。
- **`*.d` 用正则精确表达**：单标签通配在 Clash 规则里没有对应形态。
- **行为不一致仍按文件展开并告警**：模板写 `clash-domain:` 但文件内部是 ipcidr 时，按文件内容展开并告警，不静默按模板名字猜。
- **引入成本**：新增 vendor 只有 `mihomo/component/cidr`（2 个文件），二进制体积 +301.5 KB（37.78 MB → 38.08 MB）。
- **实机验证**：8 个 Aethersailor 模板 + 1 个兼容文件的产物全部通过 `mihomo -t`；`Custom_Clash.ini` 引用的 3 份 `.mrs` 共 1012 个域名全部出现在产物里（与内核参考输出逐条比对，0 缺失）。

### N6 说明：环视断言为什么只能部分对齐


Go 的正则引擎是 RE2，为保证线性时间复杂度不做回溯，因此不实现环视断言；subconverter 用的是 `dlclark/regexp2`（回溯引擎），两者对同一份正则的能力集不同。这是引擎选型差异，不是模板兼容漏项。

实测这份模板里有 4 个分组的成员正则带断言，处理方式与结果：

| 分组 | 正则写法 | 处理 | 结果 |
|---|---|---|---|
| 🇯🇵 日本节点 | `(?<!尼\|-)日` | 剥掉断言 | 6 个节点，与 subconverter 一致 |
| 🐢 低倍率节点 | `(?<![\d\.])0?\.\d+(?![0-9])(?:x\|倍)?` | 剥掉断言 | 5 个 `0.20x` 节点，一致 |
| 🌐 其他地区 | `(^(?!.*(已分类关键词)).*)` | 翻译成否定匹配 | 10 个节点，一致 |
| 🇷🇺 俄罗斯节点 | `(?<!白)俄罗斯` | 剥掉断言，0 命中 | 落 `DIRECT`，一致 |

`^(?!.*X).*` 是唯一"剥掉就反义"的写法，所以单独翻译成否定匹配而不是并到剥除逻辑里。其余断言在这类模板里都是排除性护栏（"白"前的"俄"不算俄罗斯），剥掉后匹配只会放宽；当前实测放宽没有引入错误成员，但换成节点名里真带这类字符的订阅时可能出现多归类，这是这条差异的实际代价。

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
| — | `mellow` / `mellow-ios` | 已落地（差异见下节 M1~M4） |
| — | `sstable` | 未落地（无参考产物可核实，见下节 M5） |
| — | `sing-box` / `singbox` | 已落地（JSON 配置，实机校验通过，差异见下节 B1~B7） |
| — | `clash.json` / `clash.meta.json` / `clashmeta.json` / `clash-json` / `clashmeta-json` / `mihomo.json` | 已落地，与 `clash.meta` 共用构树逻辑，只换编码 |

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

## Mellow 配置形态（mellow）

Mellow 的节点行是**三列一行**：`名字, 协议, 分享链接`。凭据不单独展开，
直接复用分享链接——`internal/render/mellow` 调 `sharelink.Render` 生成 URI，
因此 URI 的正确性只有一处实现，mellow 不会漂出去。

段落顺序与 subconverter 实机产物一致：

```
[Endpoint]      DIRECT / REJECT / Dns-Out 三个内置端点在前，节点在后
[Routing]       domainStrategy=…
[Dns]           clientIp=… / hijack=Dns-Out
[DnsServer]     可选，未配置时不写该段
[Log]           loglevel=…
[EndpointGroup] 组
[RoutingRule]   规则
```

代理组模式只有三种，从 Clash 组类型映射过去：

| Clash 组类型 | Mellow 模式 | 说明 |
| --- | --- | --- |
| `select` | `static` | 手动选择 |
| `url-test` | `latency, interval=N, timeout=N` | 测速择优 |
| `fallback` | `fallback` | 故障回退 |

**timeout 单位是秒**，Clash 侧是毫秒，必须换算（10000ms → `timeout=10`），
不换算会等到一小时才超时。亚秒超时取整到 1 秒——0 秒会被客户端当成不超时。

协议覆盖面与 `sharelink.Supported()` 一致：ss / ssr / vmess / vless / trojan /
hysteria / hysteria2 / tuic / anytls / socks5 / http 全部可导出。

### 已知差异

| # | 差异点 | subconverter v0.9.0 行为 | SubForge 行为 | 处置 |
| --- | --- | --- | --- | --- |
| M1 | 节点覆盖面 | 12 个探针节点只导出 vmess 与 ss 两个，其余协议全部丢弃 | 全部可导出协议都导出，丢不了的逐节点告警 | **有意偏离**。参考产物的节点覆盖面不够用，Mellow 本身支持这些协议 |
| M2 | 节点凭据 | 导出的两条凭据也是空的：`vmess1://@host:port?network=&tls=` 与 `ss://Og@host:port`（`Og` 是 base64 的 `:`，即 method 与 password 两段皆空） | 凭据完整，走标准分享链接编码 | **有意偏离**。空凭据的节点连不上，那是坏产物；本项是 M1 的主要动因 |
| M3 | vmess 链接 scheme | `vmess1://` | `vmess://`（标准 v2ray 分享链接） | 有意偏离。`vmess1://` 不是通用 scheme，Mellow 认标准 `vmess://`；ss 链接也补回 `#名字` 片段 |
| M4 | 规则集 | 无参考产物可核实 | `RULE-SET` 丢弃并计入告警 | 保守处置。没有可核实的语法就不猜输出，宁可丢规则并告警，也不产出无法验证的行 |
| M5 | `sstable` | 有该 target，但无实机产物可参照 | 未实现，报 501 | 无实机参考产物可核实字段，宁可报 501 也不猜格式 |

`GEOSITE` 与 surge 系 C 节同因：依赖内核 mmdb 数据，Mellow 没有对应数据源，
丢弃并告警。`MATCH` 是内核专属收尾语法，映射成 `FINAL`。IP-CIDR / IP-CIDR6
追加 `no-resolve`，与 QX / Surfboard 一致。

## sing-box 配置形态（sing-box）

产出 sing-box 自己的 JSON 配置，不是"换编码的 Clash"。顶层段落顺序与实机参考
产物一致：`log` / `dns` / `ntp` / `inbounds` / `outbounds` / `route` /
`experimental`。目标内核为 sing-box **1.14**，字段以 `sing-box check -c` 实测为
准（见 B7），因为官方 `schema.json` 在这几个点上与内核不一致。

出站按 `type` + `tag` 定位，地址用 `server` + `server_port`。前两个出站固定为
内置项：

| tag | type | 用途 |
| --- | --- | --- |
| `DIRECT` | `direct` | 直连 |
| `REJECT` | `block` | 丢弃 |

1.12 起 `dns` 出站被整体移除，所以不再有 `dns-out` 回环出站；DNS 全部走
`dns` 段内的服务器与规则。

规则映射到 `route.rules`：同字段、同出站的相邻行合并进同一个对象的数组
（`domain_suffix: [...]`），因为 sing-box 的规则对象就是"多字段各带一个数组"，
逐行拆会产出重复字段；不同字段不合并，domain 类与 `ip_cidr` 的匹配阶段不同，
合并会改变命中顺序。规则动作是**平铺字段**：路由规则用 `outbound`，DNS 规则用
`action`（字符串）+ `server`（见 B7-e）。`MATCH` / `FINAL` 不落规则行，改写
`route.final`。指向已跳过节点的规则会被丢掉并告警。

代理组映射为 outbounds 里的 `selector` / `urltest` 项（成员用 `outbounds` 数组
列 tag）。`select` / `relay` / `direct` / `pass` → `selector`，`url-test` →
`urltest`（带 `url`、`tolerance`、`interval`，`interval` 用时长字面量如 `300s`）。
`fallback` 在 1.12 起没有对应的出站类型，退化为 `selector` 并告警——失去逐个
试探的语义。成员里引用了未导出节点的 tag 会被过滤掉，成员清空后整组不输出，
指向不存在出站的引用会让内核整份配置加载失败。

### 已知差异

| # | 差异点 | subconverter v0.9.0 行为 | SubForge 行为 | 处置 |
| --- | --- | --- | --- | --- |
| B1 | 节点覆盖面 | 12 个探针节点只导出 vmess / ss / ssr / trojan 四个，其余协议全部丢弃 | vmess / vless / trojan / hysteria2 / hysteria / tuic / anytls / shadowsocks / socks / http / https 全部导出（`https` 落成 `http` + `tls`）；`ssr` 无法导出——1.12 起 `shadowsocksr` 出站被移除，该节点丢弃并逐节点告警 | **有意偏离**，与 M1 同一动因：参考产物覆盖面不够用。ssr 是内核限制，不是渲染遗漏 |
| B2 | 分类组 | 内置 13 个 emoji 分类组（🎥 NETFLIX、⛔️ 广告拦截、📲 电报信息…），来自它自带的模板 | 不自带任何分类组，完全由模板 `[clash_proxy_group]` 决定；没配就一个都不输出 | **有意偏离**。内置分类组是第三方规则数据（CC-BY-SA / GPL），按 R3 不静态打包；需要时由用户配远程源拉取 |
| B3 | 组参数 | 无 url-test 之外的组参数可对照 | Clash 的 `timeout` 在 sing-box 的 `urltest` 没有对应字段，丢弃；`fallback` → `selector` 并告警 | 保守处置。参考产物只用到 `url` 与 `interval`，无法核实其余参数 |
| B4 | 规则集 | 无实机产物可核实 | `RULE-SET` / `SCRIPT` 丢弃并计入 `X-Convert-Skipped` 告警 | 与 M4 同一处置：没有可核实的语法就不猜输出 |
| B5 | 规则覆盖 | 参考产物单条规则就含 161KB 的 `domain_suffix` 列表，`route` 段占整份体积的 99% | 规则内容全部来自模板或请求参数，不内置任何规则清单 | 与 R3 一致；产物体积因此小得多 |
| B6 | 实机校验 | 有 | 金测两份 + CLI 产物均过 sing-box 1.14.3 的 `sing-box check -c`（linux/amd64） | **已关闭**，2026-10-14。远端 Ubuntu 22.04 + Go 1.26.3 实测：`go build` / `go vet` / 全量 `go test` 全绿，两份 golden 与 CLI 转换产物全部 PASS。本地另有一层 jsonschema 校验作为快速前置门 |
| B7 | 1.14 实机校验发现 | 不适用（参考产物是旧内核） | 官方 `schema.json` 与内核行为在 9 个点上不一致，全部以实机为准；字段映射相应调整 | **已落实**。明细见下表 |

#### B7 明细：官方 schema 与 sing-box 1.14 内核的差异

以下每一项都是"照官方 schema 写能过校验、却过不了 `sing-box check`"（或反之），
逐条用真实二进制判别过。

| 子项 | 现象 | 实测报错 | 渲染器处置 |
| --- | --- | --- | --- |
| a | 规则动作被误认为嵌套对象。官方 schema 的 `Rule` 是 `allOf` 合成、`unevaluatedProperties:false`，而 `action` 被定义在 `oneOf` 的 `$ref` 里，注解不会上传到祖先，于是顶层 `action` 被判非法属性 | 写 `{"action":{"action":"route","outbound":...}}` → `json: cannot unmarshal object into Go struct field _DNSRuleAction.action of type string` | 动作平铺：路由规则用 `outbound`，DNS 规则用 `action`（字符串）+ `server`。这是 schema 生成器的产物缺陷，不是我们的写法错 |
| b | hysteria（v1）凭据字段是 `auth_str` | 写 `password` → `unknown field "password"` | 映射 `password` → `auth_str` |
| c | `up_mbps` / `down_mbps` 必须是整数。schema 把 hysteria 的 `up`/`down` 标成 `integer|string`，`up_mbps`/`down_mbps` 才是 `integer` | `json: cannot unmarshal string into Go struct field HysteriaOutboundOptions.up_mbps of type int` | 字符串形态也解析成整数 |
| d | hysteria（v1）强制要求 `up_mbps` **与** `down_mbps` 同时存在，缺一即失败；hysteria2 侧可选 | `initialize outbound[N]: missing upload speed` / `missing download speed` | 缺失的按 1000 Mbps 兜底（相当于不限速）并逐节点告警；机场链接省略速度是常态，不能丢节点 |
| e | X25519 `reality.public_key` 必须是 **base64 URL 安全、无填充**（`-_` 字母表，不带 `=`）。14 组密钥对照实测：全部 std 编码（`+/` + 填充）→ `illegal base64 data at input byte N`；全部 URL 安全无填充 → PASS | `decode public_key: illegal base64 data at input byte 23` | 渲染时 `RawStdEncoding` → `RawURLEncoding` 转换，并在 `tls.reality` 内嵌。Clash/V2Ray 生态统一输出 std 编码，不转必然失败 |
| f | DNS 服务器用 `type` 判别式，`address` 字段被移除；`address_resolver` 改名 `domain_resolver` | — | `servers[]` 每项按 15 种 type 之一组装，用 `server`/`server_port` |
| g | 主机名形式的 DNS 服务器必须声明 `domain_resolver`；缺 `route.default_domain_resolver` 在 1.12 弃用、**1.14 直接判配置非法** | `missing route.default_domain_resolver or domain_resolver` | 默认配置给主机名服务器补 `domain_resolver`，并输出 `route.default_domain_resolver`；引用了不存在的 DNS tag 会告警 |
| h | `dns` 段只允许 `cache_capacity`/`client_subnet`/`disable_cache`/`disable_expire`/`final`/`optimistic`/`reverse_mapping`/`rules`/`servers`/`strategy`/`timeout`；`default`/`resolver`/`proxy`/`block`/`fakeip`/`independent_cache` 全被移除。fakeip 移入 `dns.servers[]` 的 `{type:"fakeip", inet4_range}` | — | 兜底解析器落成一条匹配 `domain:["*"]` 的规则；fakeip 作为 4 台 DNS 服务器之一 |
| i | `geoip` / `geosite` 匹配被整体移除，`ip_cidr6` 并入 `ip_cidr`（v4/v6 通用）；`shadowsocksr`、`fallback` 出站、`dns` 出站、`udp` 字段全被移除；tuic 的 `congestion-controller` 改名 `congestion_control`；`tls.utls` 是 `{enabled, fingerprint}` 对象而非字符串，`alpn` 与 `reality` 移入 `tls` 子对象；`OutboundRealityOptions` 只剩 `{enabled, public_key, short_id}`（`spider_x` 被移除）；`mixed` inbound 用 `listen`/`listen_port`，`tun` 用 `address`（`inet4_address`/`stack`/`sniff` 移除） | — | 逐项按 1.14 形态输出。geoip/geosite 规则无法产出（需第三方规则集，按 R3 不打包），命中的规则丢弃并告警，不是静默 |

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

## Clash JSON 配置（`clash.json` 系）

`clash.json` / `clash.meta.json` / `clashmeta.json` / `clash-json` /
`clashmeta-json` / `mihomo.json` 六个别名共用同一棵构好的配置树，只换最后一步
编码，配置内容不重复实现——JSON 分支的开关由 registry 按 target 决定，不由
调用方传入，因此复用同一个 `Options` 结构体不会把 YAML 输出串成 JSON。

两个编码细节：

- **键顺序保持构树顺序**。`encoding/json` 会按字典序排 map 键，golden 就不稳定
  了，所以自己走一遍 `yaml.Node` 树做 2 空格缩进；顺带绕开 `json.Indent` 把
  字符串里的 `<` / `>` / `&` 转成 `\u` 转义的问题——域名规则里正好会出现这些字符。
- **端口保持字符串**。`"port": "443"` 而不是 `443`，与 YAML 产物字节级等价
  （`TestRender_JSONRoundTrip` 卡死这一点）；改成数字两种格式的描述就不等价了。
  空列表输出 `[]` 而不是 `null`。

JSON 不支持注释，配置头无处可放，元信息只留在 HTTP 响应头里。
同一样本下 JSON 比 YAML 大约 30%（6328 字节 vs 4893 字节），主要是每个键都要加引号。

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
| R8 | 裸规则行（无类型前缀） | 按 DOMAIN 处理 | 先按 CIDR 试：解析成网段就归 `IP-CIDR` / `IP-CIDR6`（含冒号判 v6），否则当纯域名归 DOMAIN，都不是就报 400 | 对齐并更精确。公共规则文件（`.list`）常把域名与网段混写；网段曾落进 DOMAIN 兜底分支，导致形态 4 吐 `payload: []`、形态 6 写出 `DOMAIN,10.0.0.0/8` 这种无效条目。在运行中的服务上复现后修掉，加了 `TestRender_BareCIDRNotMisreadAsDomain` 回归 |
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
