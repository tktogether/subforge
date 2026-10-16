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

## 当前状态：M1 内核里程碑开发中

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

M1 之外的功能（Web UI、测活、goja 脚本、Rule-Bot 闭环、MRS 离线生成等）见设计书路线图。

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
cmd/subforge       CLI 入口（serve | convert | version | doctor）
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
