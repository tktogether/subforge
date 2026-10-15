# ADR-0001：技术栈与嵌入策略（M1 落地）

- 状态：已采纳（2026-10-14）
- 依据：主设计书 §5.1 / §9；本次实施前对 mihomo v1.19.31 源码与本地工具链的实测核对

## 背景

SubForge 的全部架构建立在一个关键假设上：mihomo（MIT）可作为 Go 库嵌入，
协议解析/测活/规则编译复用上游。该假设在设计阶段已由 subs-check 生产验证，
但 mihomo 并非为库设计（内部 API 无稳定性承诺），落地前必须冻结可依赖的 API 面。

## 实测核对结果（2026-10-14，mihomo v1.19.31 源码）

| 能力 | API（已核实的精确签名） | 位置 |
|---|---|---|
| 分享链接/Base64 订阅 → Clash 配置 map | `ConvertsV2Ray(buf []byte) ([]map[string]any, error)` | `common/convert/converter.go` |
| 配置 map → 运行期代理（校验/测活用） | `ParseProxy(mapping map[string]any, options ...ProxyOption) (constant.Proxy, error)` | `adapter/parser.go` |
| 延迟测试 | `(*adapter.Proxy).URLTest(ctx, url, expectedStatus)` / `DelayHistory()` | `adapter/adapter.go` |
| 域名/IP 匹配 trie（F1/F4 用） | 目录存在 | `component/trie/` |
| GeoIP mmdb（F5 用） | 目录存在 | `component/mmdb/` |

`ConvertsV2Ray` 覆盖协议：vmess / vless(含 reality) / hysteria / hysteria2 / tuic /
trojan / ss / ssr / socks(5) / http(s) / anytls / mierus。

**关键结论（影响内部模型设计）**：outbound 适配器（如 `Vless`）**没有** `MarshalYAML`/`MarshalJSON`
方法（v1.19.31 实读），无法从适配器结构体还原配置。因此：

> 节点内部模型以**原始 config map**（`map[string]any`）为唯一事实源，全部渲染走 raw map 往返；
> `ParseProxy` 只用于校验与运行期适配器（M2 测活）。这从结构上杜绝了自研结构体与内核字段漂移。

其他实测事实：

- mihomo v1.19.31 `go.mod`：`go 1.20` 指令、约 122 个依赖、1 条 replace（protobuf fork）。
- 本机工具链：Go 1.26.3 / git 2.50 / node 24；GOPROXY 可达；无 Docker（金测基线改用 subconverter 便携版二进制）。

## 决策

1. **主语言 Go**，go.mod 指令 `go 1.24`（本机 1.26.3 构建）。
2. **锁定 mihomo v1.19.31**（与 subs-check 生产验证同版），`go mod vendor` 全量锁定；
   升级走 CI 回归 + `internal/parse` 适配层隔离。
3. **`internal/parse` 是全仓库唯一 import mihomo 的包**；其他代码只依赖自有模型。
4. 节点模型 = 原始 config map（见上），字段派生（Name/Type/Server/Port）为只读视图。
5. 直接依赖保持最小：mihomo + `gopkg.in/yaml.v3` + `chi`；其余用标准库。
6. YAML 渲染用 `yaml.v3` 手工构造节点（保序、可控缩进），不用 map 序列化。

## 后果

- 每次 mihomo 升级：适配层可能需改动 → 由 `internal/parse` 单点承担 + CI 金测兜底（对齐 R1）。
- 无法使用内核结构体做序列化输出（本来也不需要：渲染自研，设计书 §5.1 已如此规划）。
- 二进制体积与编译时间由 mihomo 依赖树主导（15~25MB 预期，I1 spike 实测确认）。
