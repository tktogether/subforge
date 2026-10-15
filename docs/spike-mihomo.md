# I1 Spike：mihomo v1.19.31 嵌入可行性

- 日期：2026-10-14
- 结论：**闸门通过**。mihomo 可作为库嵌入，全协议解析与内核校验零错误；关键设计约束已确证。
- 复现命令：`go test ./internal/parse/ -v`、`go test -bench . -run '^$' -benchtime=20x ./internal/parse/`

## 0. 环境

| 项 | 实测值 |
|---|---|
| 本机工具链 | Go 1.26.3（go.mod 指令 `go 1.24`）、git 2.50.0、node 24.21.0 |
| CPU / GOOS | 13th Gen Intel Core i5-13490F（16 逻辑核）/ windows/amd64 |
| GOPROXY | `https://proxy.golang.org,direct`（可达，HTTP 200） |
| 目标版本 | `github.com/metacubex/mihomo v1.19.31`（与 subs-check 生产验证同版） |
| 依赖规模 | 主模块 `go list -deps` 共 **70 个包**；mihomo 自身约 122 条 require |
| Docker | **未安装**（金测基线改用 subconverter 便携版二进制，见 I8） |

## 1. 已确证的 API 面（v1.19.31 源码实读）

| 能力 | 签名 | 位置 |
|---|---|---|
| 分享链接/Base64 订阅 → Clash 配置 map | `func ConvertsV2Ray(buf []byte) ([]map[string]any, error)` | `common/convert/converter.go` |
| 配置 map → 运行期代理（校验/测活） | `func ParseProxy(mapping map[string]any, options ...ProxyOption) (constant.Proxy, error)` | `adapter/parser.go` |
| 延迟测试（M2 测活用） | `(*adapter.Proxy).URLTest(ctx, url, expectedStatus)` / `DelayHistory()` | `adapter/adapter.go` |
| 域名/IP 匹配 trie（F1/F4） | 目录存在 | `component/trie/` |
| GeoIP mmdb（F5） | 目录存在 | `component/mmdb/` |

`ConvertsV2Ray` 覆盖：vmess / vless(含 reality) / hysteria / hysteria2 / tuic / trojan /
ss / ssr / socks(5) / http(s) / anytls / mierus。

## 2. 核心功能验证（`testdata/fixtures/mixed-subscription.txt`，12 条合成链接）

12 条链接 → **12 个节点全部解析成功、全部通过 `ParseProxy` 校验，零错误**：

| # | 协议 | 名称 |
|---|---|---|
| 0 | vmess | vmess-node |
| 1 | vless（reality + tcp） | vless-reality |
| 2 | vless（tls + ws） | vless-ws |
| 3 | hysteria2 | hy2-node |
| 4 | hysteria | hy1-node |
| 5 | tuic | tuic-node |
| 6 | ss | ss-node |
| 7 | ssr | ssr-node |
| 8 | trojan | trojan-node |
| 9 | anytls | anytls-node |
| 10 | socks5 | socks-node |
| 11 | http | http-node |

Base64 整体编码的订阅（`mixed-subscription.b64`）产物与明文**逐节点一致**；
空输入与纯文本垃圾输入均返回可辨认错误、不 panic。

每个节点都含 `name`/`type`/`server`/`port` 四个必需字段。

## 3. 两个决定性发现（已写入 ADR-0001）

1. **outbound 适配器没有 Marshal 方法**（`adapter/outbound/vless.go` 实读无 `MarshalYAML`/`MarshalJSON`）。
   → 内部节点模型以**原始 config map 为唯一事实源**，渲染走 raw map 往返；
   `ParseProxy` 只用于校验与 M2 测活。自研结构体与内核字段的漂移风险被结构消除。
2. **`ParseProxy` 不修改输入 map**（`TestValidateDoesNotMutateMapping`：校验前后 JSON 字节级相等）。
   → 校验与渲染可安全共用同一份 map，无需副本。

其他解析细节：reality 的 `pbk`/`sid` 被折叠进嵌套 `reality-opts`；
hysteria2 同时产生 `password` 与 `obfs-password`；vmess 的 `alterId` 等字段原样保留。

## 4. 性能与体积度量

### 4.1 转换成本（1000 节点，i5-13490F，-benchtime=20x）

| 基准 | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkConvert1000`（仅 ConvertsV2Ray） | **2,662,140（2.66ms）** | 2,589,974 | 31,238 |
| `BenchmarkConvertAndValidate1000`（+ 逐节点 ParseProxy） | **27,161,945（27.16ms）** | 23,946,806 | 193,727 |

**设计约束（新增，M1 后续必须遵守）**：逐节点内核校验成本约为纯转换的 **10 倍**（且分配多 10 倍），
**不得进入默认转换热路径**。默认模式只转换 + 用必需字段（name/type/server/port）做轻量断言；
`ParseProxy` 校验仅限显式开启（CLI `--check` / `doctor`），并在 UI/文档中说明其代价。

对照设计书 §8 预算：1000 节点转换 P99 < 50ms —— 纯转换 2.66ms 已留出 18 倍以上余量；
若误把校验放热路径（27ms），预算只剩不到一半。

### 4.2 体积与启动（`-trimpath -ldflags="-s -w"`，CGO_ENABLED=0）

**mihomo 未链接**（当前 `cmd/subforge` 仅含 version/doctor）：linux-amd64 1.80MB /
linux-arm64 1.77MB / mipsle 2.10MB / windows-amd64 1.79MB。

**mihomo 已链接**（临时探针 `tools/size-probe`，gitignore 排除）：

| 目标 | 体积 |
|---|---|
| linux/amd64 | **19,521,698（18.6MB）** |
| linux/arm64 | 18,284,706（17.4MB） |
| windows/amd64 | 18,286,592（17.4MB） |
| linux/mipsle（softfloat，OpenWrt 常见） | 20,775,105（19.8MB） |

- 全部四个交叉编译目标**一次通过**，mipsle softfloat 也可用 → 软路由交付可行。
- 体积落在设计书 15~25MB 预期内。**注**：探针仅链接 parse 路径（未含 fetch/render/api/http 栈），
  I4 接入完整管线后预计小幅上升，需重测。

启动与内存（windows/amd64）：

| 指标 | 实测 | 预算 |
|---|---|---|
| 冷启动（含解析 12 节点 + 打印） | **5 次合计 0.287s ≈ 57ms/次** | <100ms ✅ |
| 空闲 Sys（探针） | 10.52MB | <30MB ✅ |
| 空闲 Sys（无 mihomo 的 version 二进制） | 6.02MB | — |
| HeapAlloc / HeapSys（探针） | 1.02MB / 7.81MB | — |

> 稳定 RSS 属运行态指标，需常驻服务进程；I5（HTTP 服务）起用 `doctor` + 24h 挂测复测。

### 4.3 构建与依赖

| 项 | 实测 |
|---|---|
| `go get` 首次拉取 mihomo 依赖树 | 73s |
| 首次编译（含 70 包） | 42.7s |
| `go mod vendor` | 62.4s，**vendor 82MB / 5733 个 Go 文件** |
| 主模块依赖包总数 | 70 |

**上游 replace 指令不会传递**：mihomo 自身有一条 `replace google.golang.org/protobuf =>
github.com/metacubex/protobuf-go ...`，但该 replace 只在主模块生效；
实测本仓库无需该替换即可编译通过（protobuf 未被 import 路径触及）。
后续 mihomo 升级需回归确认此点（记入 ADR-0001 的升级清单）。

## 5. 闸门判定

| 检查项 | 结果 |
|---|---|
| 全协议解析（12 条 / 11 类） | ✅ 全通过 |
| `ParseProxy` 内核校验 | ✅ 全通过，零错误 |
| 明文与 Base64 订阅 | ✅ 产物一致 |
| 异常输入 | ✅ 明确报错，不 panic |
| 校验不改写事实源 map | ✅ 字节级相等 |
| 二进制体积 | ✅ 18.6MB（预算 15~25MB） |
| 冷启动 | ✅ 57ms（预算 <100ms） |
| linux/mipsle softfloat 交叉编译 | ✅ OpenWrt 场景可行 |

**无阻塞项。进入 I2（拉取 + 内部模型接线）。**

## 6. 遗留观察项（不阻塞）

1. **vendor 82MB 是否入库**：入库 = 构建可复现、离线可编译、避免被替换的 fork 版本漂移（设计书 §5.1 已如此要求）；
   代价 = 仓库克隆体积。替代方案是不入库、仅靠 `go.sum` 锁版本。倾向入库，**待你确认**。
2. 逐节点校验成本 27ms/1000 节点 —— 默认热路径禁用，已作为硬约束记录（§4.1）。
3. 完整链路（含 render/http）的二进制体积与运行态 RSS 需在 I4/I5 重测并回填本节。
4. `ParseProxy` 未做真实网络连接；测活（M2）需另行验证出站池行为与机场压力（设计书 R5）。
