# M2 验收项核查：Aethersailor 8 模板「零修改跑通」

> 核查人：compat-auditor（共享任务板 task-2）　核查日期：2026-10-17（Asia/Shanghai）
> 核查对象：`Aethersailor/Custom_OpenClash_Rules` `cfg/` 下的 8 个订阅转换模板 + 1 个兼容文件
> 上游基线：`main@f90155ba7d012bbaad1b91d4abde0900194b157d`（2026-10-17T08:29:45Z）
> 夹具：`testdata/fixtures/aethersailor/`（逐字节与上游一致，见该目录 `SOURCES.md`）

---

## 0. 结论（先看这里）

**可以宣称「8 模板零修改跑通」，但必须写清口径。**

| 口径 | 结论 | 依据 |
|---|---|---|
| 模板文件零修改、`subforge convert` 正常退出、产物结构完整 | ✅ **通过** | 9 个文件（8 模板 + 1 兼容文件）全部 `EXIT=0`，无解析错误，无悬空引用，无空分组，`MATCH` 位于规则末行 |
| 产物语义与 **subconverter v0.9.0** 一致 | ✅ **通过** | 抽 4 个模板与参考实现逐行对拍：规则行**完全一致**（5/5、93/93、5/5、57/57），组名/类型/顺序完全一致 |
| 产物语义与 **SubConverter-Extended（Aethersailor 官方推荐后端）** 一致 | ❌ **不通过** | 见 G1：`clash-domain:*.mrs` 远程规则集未被展开，每个模板丢 2~3 条规则集（即丢 `DOMAIN`/`DOMAIN-SUFFIX` 类规则） |

- **缺口只有 1 条（G1）**，且已在 `docs/compatibility.md` 登记为 N9（非本次新发现），但 N9 的表述是「对齐 subconverter」，没有点明「相对 SCE 的功能缺口」。
- 另发现 2 个**真实 bug**（P1、P2），**均未被这 8 个模板触发**，但都有最小复现，建议排进 M2 收尾或 M3。
- 建议验收措辞：**「Aethersailor 8 模板零修改跑通（基线：subconverter v0.9.0 语义；SCE 的 MRS 规则集展开记为已知差异 N9）」**。若要求与 SCE 等价，则本项不通过，缺口为 G1。

---

## 1. 核查方法

### 1.1 环境

| 项 | 值 |
|---|---|
| 工作目录 | `E:\Projects\subforge` |
| Go | go1.26.3 windows/amd64 |
| 订阅夹具 | `testdata/fixtures/mixed-subscription.txt`（12 节点：vmess/vless×2/hysteria2/hysteria/tuic/ss/ssr/trojan/anytls/socks5/http） |
| 参考实现 | `.compat/bin/subconverter/subconverter.exe`（SubConverter **v0.9.0**，HTTP `127.0.0.1:25501`） |
| 上游模板获取 | GitHub Contents API + raw.githubusercontent.com |

### 1.2 命令形态（逐模板）

```powershell
go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt `
  -config testdata/fixtures/aethersailor/<模板>.ini `
  -out dist/aethersailor-<模板>.yaml
```

模板一律**未做任何修改**（完整性由 git blob SHA-1 与上游 API 的 `sha` 逐一比对确认）。

### 1.3 对拍方法

参考实现以 HTTP 方式取产物（subconverter 要求 `url=` 为 base64 订阅）：

```powershell
python -m http.server 8899 --bind 127.0.0.1 --directory testdata/fixtures      # 提供订阅与模板
.\.compat\bin\subconverter\subconverter.exe                                   # 参考后端
curl.exe "http://127.0.0.1:25501/sub?target=clash&url=<b64订阅>&config=<模板URL>" -o dist/scv-<模板>.yaml
```

### 1.4 关于并发修改的说明（重要）

核查期间工作树被另一条 M2 工作流（Web UI / `internal/api`、`internal/render/registry`）并发修改，
期间出现过**一次** `go build` 失败：

```
internal\render\registry\registry.go:136:31: cannot use render.ClashJSONTargets (variable of type map[string]bool) as map[string]string value in argument to sortedKeys
internal\render\registry\registry.go:157:2: undefined: sort
```

该状态在数分钟内自行恢复（`go build ./...` 退出码 0）。本报告所有数据均取自**编译通过**的工作树，
并在末轮（run2）完整复跑确认：除「生成时间戳」与「随机 User-Agent」外，产物与首轮**逐行相同**。
建议 Lead 在最终验收前确认工作树编译状态并重跑本核查命令。

---

## 2. 模板清单

来源目录：<https://github.com/Aethersailor/Custom_OpenClash_Rules/tree/main/cfg>　抓取日期：2026-10-17　许可：CC-BY-SA-4.0

上游 `cfg/README.md` 定义「4 个普通版 + 4 个故障转移版 = **8 个模板**」，另有 1 个由生成流程同步的兼容文件。

| # | 文件 | 定位 | 上游 blob SHA-1 | 字节 | 本地 sha256(16) |
|---|---|---|---|---|---|
| 1 | `Custom_Clash.ini` | 标准版 | `6a749623…08a2a` | 12707 | `1bb4f64365a4a0f3` |
| 2 | `Custom_Clash_Fallback.ini` | 标准故障转移版 | `eeb10e5b…6508c` | 37611 | `b2c757347792c46c` |
| 3 | `Custom_Clash_Lite.ini` | 轻量版 | `17ce6211…eacf4` | 7832 | `23753309f3a5f473` |
| 4 | `Custom_Clash_Lite_Fallback.ini` | 轻量故障转移版 | `adb67e3f…aaa37` | 16904 | `6ffce47f708e0107` |
| 5 | `Custom_Clash_GFW.ini` | 极简 GFW 版 | `f138a918…cb256` | 1269 | `e6327808584da128` |
| 6 | `Custom_Clash_GFW_Fallback.ini` | 极简 GFW 故障转移版 | `955e7bb0…a77ae` | 1201 | `fe96bcd1e468ca4a` |
| 7 | `Custom_Clash_Full.ini` | 重度分流版 | `315c2ca2…58711` | 30324 | `aeea0542a03d31c5` |
| 8 | `Custom_Clash_Full_Fallback.ini` | 重度分流故障转移版 | `84eaf62e…855d` | 29459 | `7f725715f2726dc9` |
| — | `Custom_Clash_Mainland.ini` | 兼容文件（**与 #1 逐字节相同**） | `6a749623…08a2a` | 12707 | `1bb4f64365a4a0f3` |

完整哈希与下载地址见 [SOURCES.md](../testdata/fixtures/aethersailor/SOURCES.md)。

> 注意：#1 与兼容文件字节相同，但两次转换的产物字节数相差 1 字节（22528 vs 22527），
> 原因是产物里的 ws `User-Agent` 取自随机池——见 §7 观察项 O3。

---

## 3. INI 解析器现状（源码口径）

来源：`internal/template/ini.go`、`internal/config/template.go`、`internal/config/custom.go`、
`internal/config/pipeline.go`、`internal/config/override.go`、`internal/config/settings.go`。

### 3.1 词法层（`ini.go`）

| 语法 | 行为 |
|---|---|
| `[段落名]` | 行首行尾方括号，名字 trim；空名不算段落头 |
| `key = value` / `key: value` | 按**第一个** `=` 或 `:` 拆；`=` 优先；值只去首部空格（`[node_pref]` 的前缀依赖尾部空格） |
| `#` | **任意位置**起注释 |
| `;` | **仅行首**起注释（值里合法出现分号，rename/emoji 用分号分隔多条） |
| `---` | 仅在 `[clash_proxy_group]` 内作为「代理组分隔符」；其它段落里是语法错误 |
| 可重复键 | `custom_proxy_group` `ruleset` `rules` `step` `proxy` `prepend` `append`（追加，保序）；其余键后者覆盖前者 |
| 无 `=` 的非空行 | 硬错误：`无法解析的行（应为 key = value）` |

### 3.2 `!!import` / `!include` 行为

| 指令 | 生效层级 | 行为 |
|---|---|---|
| `!!import <路径>` | 任意层级 | 读文件并展开 |
| `!import <路径>` | 任意层级 | 同上（无 `!include` 的键名冲突） |
| `!include <路径>` | **仅顶层**（任何段落之前） | 段落内不认——`[common]` 里的 `!include`/`!exclude` 是节点名过滤的合法键名 |

- 路径相对**当前文件所在目录**解析；递归展开并做**循环引用检测**（`inStack`）。
- include 只能引用**根模板目录内**的文件：绝对路径、`../` 越界与指向目录外的符号链接均被拒绝；远程模板没有本地基准目录，其中的 include 一律拒绝。
- **顶层** include：整份配置 `Overlay` 合并。
- **段内** include：只把被引文件里**同名段落**的条目 `Set` 进当前段（覆盖语义）。
- 展开后条目的 `Source`/`Line` 指向**实际定义处**（可追溯性）。
- 已知边界：`!!import` 只支持本地路径，无 HTTP 拉取（Aethersailor 的 8 个模板**均未使用** `!!import`，只用一个 `[custom]` 段）。

### 3.3 语义层：段落与语法

| 段落 | 支持情况 |
|---|---|
| `[common]` | ✅ 单次转换参数透传；`node_pref`/`local`/`remote`/`token`/`emoji_type`/`node_order`/`rename`/`emoji`/`rules` 为控制键不透传 |
| `[node_pref]` | ✅ `include`/`exclude`/`sort`/`prefix`/`suffix`/`remove`，前缀按协议键值对写 |
| `[rename]` / `[emoji]` | ✅ 段落与 `[common]` 内联两种写法都收口（`正则,替换` 按 `;` 分条） |
| `[clash_proxy_group]` | ✅ `name/type` 块式 + `---` 分隔；`group = select,手动选择,.*` 旧式写法 |
| `[ruleset]` | ✅ 产出 `ruleset:<名>:<类型>` 参数 |
| `[custom]` | ✅ `custom_proxy_group=` 反引号内联 DSL、`ruleset=` 内联（`[]GEOSITE` / `[]GEOIP` / `[]FINAL` / `clash-domain:` / `clash-classic:`）、`rules=` 直映行；`enable_rule_generator` / `overwrite_original_rules` 为控制键 |
| `[pipeline:<名>]` | ✅ `step = filter.regex …` / `op.rename …` / `on_error = skip\|abort\|warn` |
| `[override.rules]` / `[override.group:<名>]` / `[override.proxy:<名>]` | ✅ 三级覆盖；未知 `[override.*]` 段落报错 |
| 其它 `custom_*` 指令 | ⚠️ 未实现的 `custom_*` 键产出告警（`当前未实现，该条定义被忽略`），不静默 |

### 3.4 `custom_proxy_group` 内联 DSL（本次核查的核心）

语法（反引号分隔各段）：

```text
<组名>`<类型>`<成员>`<成员>…[`<测活地址>`<参数>]
```

- 成员前的 `[]` 表示引用另一个组；不带的按**节点名正则**展开。
- 测活地址的识别方式：**取最后一个以 `http` 开头的段**（`groupFromInline`），其后第一段是 url、第二段按 `间隔,中段,容忍延迟` 解析（仅 `url-test`/`fallback` 生效）。
- 正则命中 0 个节点 → 该成员落 `DIRECT`（N7）；成员为空的分组 → 见 P2。

---

## 4. 逐项结果（run2，工作树编译通过）

命令：`go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt -config testdata/fixtures/aethersailor/<模板>.ini -out dist/run2-<模板>.yaml`

| # | 模板 | 退出码 | 产物字节 | 节点 | 代理组 | 规则 | 跳过项 | 结构校验 |
|---|---|---|---|---|---|---|---|---|
| 1 | `Custom_Clash` | 0 | 22528 | 12 | 35 | 78 | 3 | ✅ |
| 2 | `Custom_Clash_Fallback` | 0 | 23477 | 12 | 35 | 78 | 3 | ✅ |
| 3 | `Custom_Clash_Lite` | 0 | 13589 | 12 | 19 | 57 | 3 | ✅ |
| 4 | `Custom_Clash_Lite_Fallback` | 0 | 13599 | 12 | 19 | 57 | 3 | ✅ |
| 5 | `Custom_Clash_GFW` | 0 | 4814 | 12 | 3 | 5 | 0 | ✅ |
| 6 | `Custom_Clash_GFW_Fallback` | 0 | 4459 | 12 | 2 | 5 | 0 | ✅ |
| 7 | `Custom_Clash_Full` | 0 | 42974 | 12 | 56 | 93 | 2 | ✅ |
| 8 | `Custom_Clash_Full_Fallback` | 0 | 35861 | 12 | 56 | 93 | 2 | ✅ |
| — | `Custom_Clash_Mainland` | 0 | 22527 | 12 | 35 | 78 | 3 | ✅ |

「结构校验」= 无悬空规则目标（规则指向的组都存在）、无悬空组成员（成员都是节点或已定义的组）、无零成员分组、`MATCH` 位于规则末行。

**代理组数量与模板定义逐条吻合**（`custom_proxy_group=` 条数 = 产物组数）：

| 模板 | 模板定义 | 产物 | 类型分布 |
|---|---|---|---|
| GFW / GFW_Fallback | 3 / 2 | 3 / 2 | select+url-test / fallback+select |
| Clash / Mainland / Lite / Lite_Fallback | 35 / 35 / 19 / 19 | 同 | 28+7 / 12+7 |
| Clash_Fallback / Full_Fallback | 35 / 56 | 同 | 含 fallback 21 / 28 |
| Full | 56 | 56 | select 39 + url-test 17 |

**规则构成**（无一行丢失，除 G1 的 MRS 规则集）：

| 模板 | GEOSITE | GEOIP | IP-CIDR | DOMAIN-SUFFIX | DST-PORT | SRC-PORT | MATCH | 合计 |
|---|---|---|---|---|---|---|---|---|
| Clash / Fallback / Mainland | 31 | 7 | 34 | 0 | 4 | 1 | 1 | 78 |
| Full / Full_Fallback | 43 | 7 | 34 | 3 | 4 | 1 | 1 | 93 |
| Lite / Lite_Fallback | 14 | 3 | 34 | 0 | 4 | 1 | 1 | 57 |
| GFW / GFW_Fallback | 1 | 3 | 0 | 0 | 0 | 0 | 1 | 5 |

### 4.1 stderr 原文（有跳过项的模板，以 `Custom_Clash_Full` 为例）

```text
提示: 跳过/忽略 2 项
  规则集 https://cdn.jsdelivr.net/gh/Aethersailor/Custom_OpenClash_Rules@main/rule/Custom_Direct_Domain.mrs 内容异常（含 NUL 或非文本字节），该规则集未生效
  规则集 https://cdn.jsdelivr.net/gh/Aethersailor/Custom_OpenClash_Rules@main/rule/Custom_Proxy_Domain.mrs 内容异常（含 NUL 或非文本字节），该规则集未生效
管线: 12 -> 12 节点
已写入 dist/run2-Custom_Clash_Full.yaml
完成: clash.meta, 42974 字节, 12 节点, 525ms
```

各模板被跳过的规则集：

| 模板 | 被跳过的 `.mrs` 规则集 |
|---|---|
| Clash / Clash_Fallback / Mainland | `Custom_Direct_Domain.mrs`、`Custom_Proxy_Domain.mrs`、`Steam_CDN_Domain.mrs` |
| Full / Full_Fallback | `Custom_Direct_Domain.mrs`、`Custom_Proxy_Domain.mrs` |
| Lite / Lite_Fallback | `Custom_Direct_Domain.mrs`、`Custom_Proxy_Domain.mrs`、`Game_Download_CDN_Domain.mrs` |
| GFW / GFW_Fallback | 无 |

**除上述告警外，9 次运行 stderr 无任何其它输出**（模板解析零报错）。

---

## 5. 交叉验证：与 subconverter v0.9.0 对拍

用同一份 base64 订阅、同一份模板，分别取 SubForge 与 subconverter 的 `clash` 产物做结构化比对
（subconverter 仅支持 vmess/ss/ssr/trojan，故其节点数恒为 4）：

| 模板 | 规则行 | 规则数 | 组名+顺序 | 组类型 | 组数 | 节点数 |
|---|---|---|---|---|---|---|
| `Custom_Clash_GFW` | **逐行相同** | 5 = 5 | 相同 | 相同 | 3 = 3 | 12 vs 4 |
| `Custom_Clash_Full` | **逐行相同** | 93 = 93 | 相同 | 相同 | 56 = 56 | 12 vs 4 |
| `Custom_Clash_GFW_Fallback` | **逐行相同** | 5 = 5 | 相同 | 相同 | 2 = 2 | 12 vs 4 |
| `Custom_Clash_Lite` | **逐行相同** | 57 = 57 | 相同 | 相同 | 19 = 19 | 12 vs 4 |

- 组名差异仅来自 YAML 引号（SubForge 给含 emoji 的名字加引号，subconverter 不加），规范化后**完全相同**。
- 组成员差异**只有一类**：SubForge 多出 subconverter 不支持的 8 个协议节点（vless×2、hysteria2、hysteria、tuic、anytls、socks5、http）。
  **不存在任何「subconverter 有而 SubForge 没有」的成员**。
- 结论：这 8 个模板的**模板语义解析**（内联 DSL、`[]` 组引用、`[]FINAL`、远程 `clash-classic:` 规则集展开、`fallback` 组）与参考实现一致；
  `clash-domain:*.mrs` 两边都**未展开**——即 G1 是「与 SCE 的差距」，不是「与 subconverter 的差距」。

---

## 6. 不兼容点

### G1（功能缺口，影响本验收项口径）`clash-domain:*.mrs` 远程规则集未展开

**现象**：模板里 `clash-domain:<url>.mrs` 形式的规则集（MRS 是 mihomo 的二进制域名规则格式）
被判定为「内容异常」并跳过，产物因此缺少该规则集承载的 `DOMAIN`/`DOMAIN-SUFFIX` 规则。

**复现**：

```powershell
go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt `
  -config testdata/fixtures/aethersailor/Custom_Clash_Full.ini -out dist/g1.yaml
```

stderr 原文：

```text
提示: 跳过/忽略 2 项
  规则集 https://cdn.jsdelivr.net/gh/Aethersailor/Custom_OpenClash_Rules@main/rule/Custom_Direct_Domain.mrs 内容异常（含 NUL 或非文本字节），该规则集未生效
  规则集 https://cdn.jsdelivr.net/gh/Aethersailor/Custom_OpenClash_Rules@main/rule/Custom_Proxy_Domain.mrs 内容异常（含 NUL 或非文本字节），该规则集未生效
```

**影响面**：每个模板 2~3 条规则集；产物里对应域名既不直连也不走代理，最终落到 `MATCH`（漏网之鱼）。
例如 `Custom_Clash.ini` 的「本项目收录的直连/代理规则」两块整体失效。

**已登记情况**：`docs/compatibility.md` N9 已记录该行为并说明「对齐 subconverter」。本次核查补充两点：
1. 上游 README 明确推荐后端为 [Aethersailor/SubConverter-Extended](https://github.com/Aethersailor/SubConverter-Extended)（SCE 支持读取 MRS），
   而这些模板正是为 SCE 编写的；「与 subconverter 对齐」不等于「与模板作者意图对齐」。
2. 建议把 N9 的「影响面」写成可验收的量化表述（每模板 2~3 条规则集 / 缺失的规则类别），否则验收方会误判为「已对齐」。

**最小修复建议（不在本次改动范围内）**，三选一：

| 方案 | 做法 | 代价 |
|---|---|---|
| A（推荐） | 在规则集展开路径接入 MRS 解码：把 `clash-domain:*.mrs` 解成域名清单再生成规则行。注意 `vendor/github.com/metacubex/mihomo/` 当前**只 vendored 了** `adapter/common/component/constant/context/dns/log/ntp/transport/tunnel`，**没有** `ruleset`/`outboundgroup`，需要新增 vendor 或自行实现 MRS 读取（MRS 是 mihomo 自有二进制格式，需按上游规范实现） | 中～大 |
| B | 对已知上游仓库，把 `.mrs` 自动回退到同名 `.yaml`/`.list`（本项目 rule/ 目录同时提供 Classical/Domain YAML） | 小，但只对 Aethersailor 这类「同名多格式」仓库有效，属启发式 |
| C | 把 `clash-domain:` 映射为 mihomo `rule-provider`（产物侧 provider 引用，不展开） | 中；会改变产物形态，且需要 provider 文件落盘/托管 |

无论选哪个，都建议**保留当前告警**（不要静默），并在 `/sub` 响应头（如 `X-Convert-Skipped`）里带上被跳过的规则集 URL。

---

### P1（真实 bug，未被 8 模板触发）成员正则以 `http` 开头时被误判为测活地址

**根因**：`internal/config/custom.go` `groupFromInline` 用「段是否以 `http` 开头」识别测活地址段
（取**最后一个**这样的段）。当成员正则本身以 `http` 开头（如 `http.*`），它会被吞掉，组直接变成零成员。

**复现**（探针文件已随夹具提交）：

```powershell
# 1) 单节点订阅，节点名故意叫 http-node
#    testdata/fixtures/aethersailor/_probe-sub-httpnode.b64
# 2) 探针模板：testdata/fixtures/aethersailor/_probe-http-member.ini
#    [custom]
#    custom_proxy_group=Test`select`http.*`[]DIRECT
#    custom_proxy_group=Ref`select`[]不存在的组`.*
go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt `
  -config testdata/fixtures/aethersailor/_probe-http-member.ini -out dist/p1.yaml
```

| 实现 | `Test` 组 | `Ref` 组 |
|---|---|---|
| SubForge | `proxies: []`（成员正则被当成测活地址吞掉） | `[DIRECT, <12 节点>]` |
| subconverter v0.9.0（同一探针 + `http-node` 订阅） | `[http-node, DIRECT]`（`http.*` 按成员正则处理） | `[不存在的组, http-node]` |

**影响**：Aethersailor 8 模板**未触发**（成员正则都是地区/关键词模式）；但任何写 `http`/`https` 开头节点名正则的模板都会命中，
且后果是「整个组没有出口」，比丢一个成员严重。次生影响：`Ref` 这种「`[]` 引用不存在的组」两边行为不同
（SubForge 落 `DIRECT`，subconverter 原样输出悬空引用）——SubForge 这边**更安全**，建议保留。

**最小修复建议**：`groupFromInline` 里把测活地址判定收窄为**绝对 URL**（`http://` 或 `https://` 前缀，
或 `net/url.Parse` 后 `Scheme`+`Host` 均非空），与 subconverter 的实测行为一致；无法判定为 URL 的段一律按成员处理。
回归用例建议直接用 `_probe-http-member.ini` + `_probe-sub-httpnode.b64`（可断言 `Test` 组含 `http-node`）。

---

### P2（真实 bug，未被 8 模板触发）零成员分组渲染成 `proxies: []`

**根因**：`internal/convert/chain.go` `resolveMembers` 在 `len(patterns) == 0` 时直接返回 `nil`，
绕过了「正则命中 0 个节点 → 落 `DIRECT`」的兜底分支（该分支只在**有**成员项时逐项生效）。

**复现**：

```powershell
go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt `
  -config testdata/fixtures/aethersailor/_probe-empty-group2.ini -out dist/p2.yaml
# 探针内容：
#   [custom]
#   custom_proxy_group=Good`select`.*
#   custom_proxy_group=Empty`select`
```

| 实现 | 产物 |
|---|---|
| SubForge | `Good`（12 节点）+ `Empty` 且 `proxies: []` |
| subconverter v0.9.0 | 只有 `Good`；**零成员组被整体丢弃** |

**影响**：8 模板未触发（每个组至少一个成员）。风险在于 `proxies: []` 会进入产物：
- 该组没有任何出口，引用它的规则实际不可用；
- 空分组的**内核可加载性本次未验证**（本机无 mihomo 二进制，且 `vendor` 未含 `adapter/outboundgroup`），
  但无论内核是否接受，产物都与参考实现不一致。

**最小修复建议**：二选一，与 subconverter 行为对齐即可——
(a) `resolveMembers` 在零成员时返回 `["DIRECT"]`；或 (b) 在渲染前丢弃零成员分组（需同步检查是否有规则引用它，避免产生悬空目标）。
建议同时加一条渲染层单测：`Good`+`Empty` 模板的产物中不得出现 `proxies: []`。

---

## 7. 其它观察项（非缺陷，供验收与测试参考）

| # | 观察 | 说明与建议 |
|---|---|---|
| O1 | `.mrs` 告警文案有两种措辞 | 同一 URL 在不同轮次出现「含控制字符，疑似压缩或编码异常」与「含 NUL 或非文本字节」两种判词（见 run1/run2 的 `Custom_Direct_Domain.mrs`）。建议归一为一条固定文案，便于日志断言 |
| O2 | 单次转换共享一个 `-timeout` | 远程规则集与订阅拉取共用一个 context 超时（默认 30s）。本次 `Full` 模板含 4 个远程 `clash-classic` 规则集，实测 0.5~3.0s；弱网或 jsdelivr 抖动时可能整体超时。建议验收脚本显式传 `-timeout`，并在 CI 里用本地规则集或缓存 |
| O3 | 产物含随机 `User-Agent` | vless/vmess 的 ws `User-Agent` 从随机池取（run1/run2 除 UA 与时间戳外逐行相同）。**字节级 golden 比对必须先归一化**该字段，否则测试会随机失败（这也是 #1 与兼容文件产物差 1 字节的原因） |
| O4 | 模板来源依赖外网 | 模板与规则集都运行时拉取（设计如此）。本次核查在能访问 `raw.githubusercontent.com` 与 `cdn.jsdelivr.net` 的网络下完成；离线环境应使用本地副本 |
| O5 | `enable_rule_generator` / `overwrite_original_rules` | 被当作控制键忽略。对 Aethersailor 模板语义等价（SubForge 无内置规则集可覆盖），但对依赖内置规则集的其它模板需另行评估 |

---

## 8. 验收建议

1. **验收措辞**建议定为：
   > 「Aethersailor 8 模板零修改跑通（基线：subconverter v0.9.0 语义）。已知差异：`clash-domain:*.mrs` 规则集不展开（N9/G1），每模板 2~3 条。」
2. 若验收方要求「与 Aethersailor 官方后端 SCE 等价」，则本项**判不通过**，唯一缺口是 **G1**；
   建议把 G1 拆成独立任务（含 MRS 解码方案选型），不要阻塞 M2 其余收尾。
3. **P1/P2 建议本轮修**：都是小改动（各 1 处判定），且已有最小复现文件可直接转成回归测试。
4. 最终验收前请 Lead 确认工作树编译状态（§1.4），并重跑 §4 的命令复现本表。

---

## 附录 A：完整复现命令

```powershell
# 0) 准备
New-Item -ItemType Directory -Force -Path dist | Out-Null

# 1) 8 模板 + 1 兼容文件逐项转换（退出码与产物见 §4）
$names = @('Custom_Clash','Custom_Clash_Fallback','Custom_Clash_Lite','Custom_Clash_Lite_Fallback',
           'Custom_Clash_GFW','Custom_Clash_GFW_Fallback','Custom_Clash_Full','Custom_Clash_Full_Fallback',
           'Custom_Clash_Mainland')
foreach ($n in $names) {
  go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt `
    -config "testdata/fixtures/aethersailor/$n.ini" -out "dist/$n.yaml"
  "$n EXIT=$LASTEXITCODE"
}

# 2) 与 subconverter v0.9.0 对拍
python -m http.server 8899 --bind 127.0.0.1 --directory testdata/fixtures
& .\.compat\bin\subconverter\subconverter.exe
curl.exe "http://127.0.0.1:25501/sub?target=clash&url=http%3A%2F%2F127.0.0.1%3A8899%2Fmixed-subscription.b64&config=http%3A%2F%2F127.0.0.1%3A8899%2Faethersailor%2FCustom_Clash_Full.ini" -o dist/scv-Full.yaml

# 3) P1 / P2 最小复现
go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt -config testdata/fixtures/aethersailor/_probe-http-member.ini -out dist/p1.yaml
go run ./cmd/subforge convert -file testdata/fixtures/mixed-subscription.txt -config testdata/fixtures/aethersailor/_probe-empty-group2.ini -out dist/p2.yaml
```

## 附录 B：夹具与产物

| 类别 | 路径 |
|---|---|
| 上游模板（9 个，逐字节同上游） | `testdata/fixtures/aethersailor/*.ini` |
| 来源与哈希 | `testdata/fixtures/aethersailor/SOURCES.md` |
| 合成探针（P1/P2） | `testdata/fixtures/aethersailor/_probe-*.ini`、`_probe-sub-httpnode.b64` |
| 本次产物（临时，可删） | `dist/run1-*.yaml`、`dist/run2-*.yaml`、`dist/scv-*.yaml`、`dist/probe*.yaml` |

---

**核查结论**：8 个模板 + 1 个兼容文件**零修改、全部转换成功**，模板语义与 subconverter v0.9.0 逐行一致；
相对 SCE 的唯一功能缺口是 **G1（`.mrs` 规则集不展开）**，需在验收口径中显式登记。
另发现 2 个未被本批模板触发的真实 bug（**P1** 成员 `http` 前缀误判、**P2** 零成员组渲染成 `proxies: []`），均附最小复现与最小修复建议。
---

## 附录 B：M2 收尾处理记录（Lead 追加，2026-10-17）

本报告是**核查时点**的记录，结论本身不改。收尾阶段对报告里列出的问题做了如下处置：

| 编号 | 报告结论 | 处置 | 证据 |
|---|---|---|---|
| G1 / N9 | `clash-domain:*.mrs` 未展开，每模板丢 2~3 条规则集 | **已修**：按方案 A 接入 MRS 二进制解码（`internal/parse/mrs.go`） | 8 模板 + 1 兼容文件全部 `EXIT=0` 且**告警归零**；产物规则数 78→1096、57→1566、93→1108 |
| P1 | 成员正则以 `http` 开头被误判成测活地址 | **已修**：`groupFromInline` 只认绝对 URL（`http://` / `https://`） | `_probe-http-member.ini` 现在产出 `Test: [http-node, DIRECT]`，与 subconverter 一致 |
| P2 | 零成员分组渲染成 `proxies: []` | **已修**：`resolveMembers` 零成员返回 `["DIRECT"]` | `_probe-empty-group2.ini` 的 `Empty` 组现在有 `DIRECT` |
| O1 | `.mrs` 告警文案有两种措辞 | 不再触发（MRS 已能解码）；真正的内容损坏仍走原告警 | — |
| O3 | 产物含随机 `User-Agent` | 保留现状；字节级 golden 比对需先归一化该字段 | — |

### G1 修复后的独立验证

1. **解码正确性**：`internal/parse/mrs_test.go` 用真实 `.mrs` 与 `mihomo convert-ruleset domain mrs`
   的输出逐行对拍（992 行），完全一致；ipcidr 行为另有 `text → mrs → text` 往返对拍。
2. **域名无缺失**：`Custom_Clash.ini` 引用的 3 份 `.mrs`（`Custom_Direct_Domain` 992 条、
   `Custom_Proxy_Domain` 17 条、`Steam_CDN_Domain` 3 条，共 1012 个域名）全部出现在产物里，
   **缺失 0 条**（与内核参考输出逐条比对）。
3. **实机可加载**：9 份产物全部通过 `mihomo -t`（mihomo v1.19.31 linux/amd64，测试机）。
4. **引入成本**：新增 vendor 仅 `mihomo/component/cidr`（2 个文件），二进制 +301.5 KB
   （37.78 MB → 38.08 MB）。

### 验收措辞（修订）

> Aethersailor 8 模板零修改跑通，产物通过 `mihomo -t`；规则集（含 `.mrs` 二进制）全部展开，
> 与 mihomo v1.19.31 的解码结果逐行一致。相对 subconverter v0.9.0 语义一致；
> 相对 SCE 的差异只剩 `.d` 后缀条目映射为 `DOMAIN-SUFFIX` 时的极轻微放宽（见 compatibility.md N9）。
