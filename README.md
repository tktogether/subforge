# SubForge

> 订阅转换 + 分流规则一体化引擎。`SubForge` 为工作名，发布前将查重改名。
> Go 单二进制，嵌入 [mihomo](https://github.com/MetaCubeX/mihomo)（MIT）解析内核；自研代码只做编排层。

## 这是什么

合并四家生态（subconverter / Sub-Store / sub-poly / Aethersailor）的全部功能点，
聚焦**分流的超高度定制化**，以极致性能开销为约束（目标：常驻 RSS < 30MB、冷启动 < 100ms、
单二进制 15~25MB；对照：subconverter C++ 依赖重、Sub-Store Node 常驻约百 MB 级）。

设计依据（决策闭环，含实测依据与明确代价）：

- [主设计书](docs/design/subforge-design.md)
- [新功能发散探索 v2](docs/design/feature-exploration-v2.md)
- [架构决策记录](docs/adr/0001-stack-decisions.md)
- [兼容性已知差异表](docs/compatibility.md)

## 当前状态：M2 定制化里程碑已完成（M3 规则工厂进行中）

M1 范围：mihomo 嵌入解析 + 全 target 渲染 + `/sub` 兼容 API + INI 解析器 + 金测集。

- [x] I0 仓库地基与 CLI 骨架
- [x] I1 mihomo 嵌入 spike（风险闸门：解析/往返/体积/RSS/交叉编译实测）
- [x] I2 内部模型 + 订阅拉取（并行/重试/ETag 缓存）
- [x] I3 transform 管线 v0（filter → operate 链）
- [x] I4 render/clash.meta 端到端 + `subforge convert`
- [x] I5 HTTP API + subconverter 参数超集 + 配置热重载
- [x] I6 INI 解析器完整（`!!import`、规则集类型、覆盖优先级链）+ `/getruleset` 六种形态
- [x] I7 渲染器铺开 —— 18 个 target 全部完成（clash.meta / clash-json / sing-box / list / mixed / ss / ssd / ssr / trojan / v2ray / surge / quan / quanx / loon / surfboard / ss-sub / ss-sub-b64 / mellow）
- [x] I8 兼容金测固化 —— subconverter 对拍 + Aethersailor 语义对拍 + mihomo 实机加载，37 项测试全部通过
- [ ] I9 性能与交付 —— bench 已固化（parse/transform 包）；Dockerfile / systemd 已落地；交叉编译矩阵待做

## 当前状态：M2 定制化开发中

M2 范围：管线引擎 + 分层覆盖 DSL + goja 沙箱 + Collection/Provider 透传 + Web UI（embed）。
设计见 [M2–M4 设计方案](docs/design/m2-m4-design.md) §1。

- [x] M2.1 管线引擎 —— 命名管线（INI `[pipeline:<名>]`）、per-step 错误策略（abort/skip/warn）、
      告警收集；CLI `-pipeline` 与 HTTP `pipeline=` 走同一条解析路径
- [x] M2.2 分层覆盖 DSL —— `groups` / `rules.prepend|append` / `proxies` 三级覆盖，
      YAML 与 INI 双解析；合并语义为"数组追加、标量替换"；`subforge diff` 预览改动
- [x] M2.3 goja 脚本沙箱 —— `filter.script` / `op.script`，无网络/无文件系统，
      5s 超时 + panic 兜底，语法错误在配置期暴露
- [x] M2.4 Provider 透传 —— Clash YAML 的 `proxy-providers` 原样提取并按名去重透传
- [x] M2.5 Web UI（embed）—— Dashboard / Convert / Rules / Logs 四页 + `/api/*`，
      纯 HTML/CSS/原生 JS 无构建，light/dark 双主题
- [x] M2.6 Aethersailor 8 模板零修改跑通 —— 全部 `EXIT=0`；
      与 subconverter v0.9.0 对拍规则/组完全一致（5/5、93/93、5/5、57/57）；
      与 SCE 的 `.mrs` 规则集展开差异记为 compatibility.md N9；
      核查报告：[docs/m2-aethersailor-compat.md](docs/m2-aethersailor-compat.md)

M3/M4 的功能（规则工厂、MRS 离线生成、测活、Rule-Bot 闭环等）见设计书路线图。

## Web UI

服务启动后浏览器打开 `http://127.0.0.1:25500/`（默认仅监听本机）：

| 页面 | 路径 | 用途 |
|------|------|------|
| Dashboard | `/` | 快速转换、状态概览、最近 10 条转换历史 |
| Convert | `/convert` | 输入 → 管线 → 输出，覆盖 DSL 编辑与 diff 预览 |
| Rules | `/rules` | 模板规则集列表与覆盖编辑器 |
| Logs | `/logs` | 结构化日志流（实时 tail + 过滤） |

前端为原生 JS，通过 `/api/targets`、`/api/convert`、`/api/diff` 等 JSON 端点工作；
LAN/VPS 部署时在页面右上角填 token（或 URL 带 `?token=`），本机默认免鉴权。

## 构建

要求 Go 1.24+。

```sh
go build ./cmd/subforge         # 本地构建
go test ./...                   # 测试
go run ./cmd/subforge version   # 版本信息
go run ./cmd/subforge doctor    # 环境与运行时自检
```

交叉编译（Linux 服务器 / NAS / 软路由）：

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/subforge
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/subforge
```

## 目录结构

```
cmd/subforge       CLI 入口（serve | convert | diff | version | doctor）
internal/parse     mihomo 适配隔离层（全仓库唯一 import 内核的包）
internal/model     内部模型（节点以"原始 config map"为唯一事实源）
internal/fetch     订阅拉取（并行 / 重试 / ETag 磁盘缓存）
internal/transform filter → operate 管线（Sub-Store 语义）
internal/template  INI 模板解析（subconverter 语义兼容）
internal/render    各 target 渲染器
internal/api       HTTP API（/sub /getruleset /render …）
internal/runtime   配置热重载（零拷贝双缓冲）
docs/              设计文档 / ADR / 兼容性差异表
testdata/          测试夹具与 golden 快照
```

## 许可证与合规

- 代码：MIT（见 [LICENSE](LICENSE)）。
- 规则/模板数据：**一律运行时拉取**，不静态打包（GPL/CC-BY-SA 上游规避，见设计书 §9）。
- 嵌入依赖：mihomo（MIT，锁定 v1.19.x）；sing-box 仅以子进程方式调用（不链接，规避 GPL 传染）。
- 测试夹具中的第三方内容注明出处，不进入发行物。
