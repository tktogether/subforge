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

## 当前状态：M3 规则工厂里程碑已完成

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
- [x] I9 性能与交付 —— bench 已固化（parse/transform 包）；Dockerfile / systemd 已落地；
      交叉编译矩阵实测通过（linux/{amd64,arm64,arm,mipsle,mips}、windows/amd64、darwin/{amd64,arm64}、freebsd/amd64），
      CI 已固化（.github/workflows/ci.yml）；二进制 25~30 MB

## 当前状态：M2 定制化开发中

M2 范围：管线引擎 + 分层覆盖 DSL + goja 沙箱 + Collection/Provider 透传 + Web UI（embed）。
设计见 [M2–M4 设计方案](docs/design/m2-m4-design.md) §1。

- [x] M2.1 管线引擎 —— 命名管线（INI `[pipeline:<名>]`）、per-step 错误策略（abort/skip/warn）、
      告警收集；CLI `-pipeline` 与 HTTP `pipeline=` 走同一条解析路径
- [x] M2.2 分层覆盖 DSL —— `groups` / `rules.prepend|append` / `proxies` 三级覆盖，
      YAML 与 INI 双解析；合并语义为"数组追加、标量替换"；`subforge diff` 预览改动
- [x] M2.3 goja 脚本沙箱 —— `filter.script` / `op.script`，无网络/无文件系统，
      5s 超时 + panic 兜底，语法错误在配置期暴露
- [x] M2.3b MRS 二进制规则集解码 —— 识别 zstd 魔数，domain/ipcidr 两种行为展开成
      `DOMAIN`/`DOMAIN-SUFFIX`/`DOMAIN-REGEX`/`IP-CIDR`/`IP-CIDR6`，与内核解码逐行一致
- [x] M2.4 Provider 透传 —— Clash YAML 的 `proxy-providers` 原样提取并按名去重透传
- [x] M2.5 Web UI（embed）—— Dashboard / Convert / Rules / Logs 四页 + `/api/*`，
      纯 HTML/CSS/原生 JS 无构建，light/dark 双主题
- [x] M2.6 Aethersailor 8 模板零修改跑通 —— 全部 `EXIT=0` 且**告警归零**；
      与 subconverter v0.9.0 对拍规则/组完全一致（5/5、93/93、5/5、57/57）；
      `.mrs` 二进制规则集已能解码展开（与 `mihomo convert-ruleset` 逐行一致），
      9 份产物全部通过 `mihomo -t`；
      **sub-web 可直连**：16 个客户端类型全部 200，`/version` 返回 subconverter 兼容纯文本，
      `sort`/`filename`/`emoji`/`list` 等 sub-web 参数全部接受；
      契约测试：`internal/api/subweb_compat_test.go`（TestSubWeb_* 全绿）；
      核查报告：[docs/m2-aethersailor-compat.md](docs/m2-aethersailor-compat.md)

M3/M4 的功能（规则工厂、MRS 离线生成、测活、Rule-Bot 闭环等）见设计书路线图。

## 当前状态：M3 规则工厂（已完成）

M3 范围：manifest 聚合 → MRS 离线生成 → cron/告警 → Rule-Bot 闭环 → 测活 → OpenWrt/Docker 交付。
设计见 [M2–M4 设计方案](docs/design/m2-m4-design.md) §2。

- [x] M3.1 manifest 聚合 —— 多上游规则集并发拉取（≤4，复用 fetch + ETag）、
      版本化文件 `<name>-<UTC timestamp>.<ext>`、retention 保留最近 N（默认 5）、
      失败保留上一版 + warn；CLI `subforge manifest update|versions|rollback`
- [x] M3.2 MRS 离线生成 —— 复用内核 `rules/provider.ConvertToMrs`，
      与 `mihomo convert-ruleset` 字节级一致（ipcidr/domain 往返对拍 committed .mrs）；
      磁盘缓存（7d TTL）；CLI `subforge mrs generate|cached`
- [x] M3.3 cron/告警 —— 内嵌 cron（robfig/cron）+ 三通道告警（Telegram/DingTalk/SMTP）+ 失败重试 3 次指数退避
- [x] M3.5 订阅测活 —— 并发探测（≤8）、默认关闭、单 URL 超时；CLI `subforge subscheck`
- [x] M3.4 Rule-Bot 闭环 —— 日志驱动异常检测（正则+阈值）、建议生成、待确认队列、确认/拒绝；CLI `subforge rulebot analyze|stats`
- [x] M3.6 OpenWrt/Docker 交付 —— Dockerfile（scratch）+ Dockerfile.alpine + docker-compose + Makefile cross/docker/release + OpenWrt ipk 包定义（init script + UCI config）+ CI Docker 构建任务

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

## HTTP API

服务默认监听 `http://127.0.0.1:25500/`（仅本机）。除标注「需要 `-data-dir`」的端点外，
其余端点为无状态转换/渲染/探针，不依赖数据目录。

> `-data-dir` 指向订阅管理的 SQLite 数据库；未传时所有订阅管理类端点返回 503 或空。

### 转换与渲染（无状态）

| 方法 | 路径 | 说明 | 需要 `-data-dir` |
|------|------|------|------------------|
| GET | `/sub` | 订阅转换（subconverter 兼容） | — |
| GET | `/profile` | 订阅转换（profile 形态） | — |
| GET | `/render` | 按 target 渲染 | — |
| GET | `/getruleset` | 规则集（六种形态） | — |
| GET | `/inspect` | 回显解析后参数（排查用） | — |
| GET | `/subscribe` | 订阅拉取 | — |
| POST | `/api/convert` | 转换（JSON） | — |
| POST | `/api/diff` | 覆盖 DSL diff 预览 | — |

### 探针与运行时

| 方法 | 路径 | 说明 | 需要 `-data-dir` |
|------|------|------|------------------|
| GET | `/version` | subconverter 兼容纯文本版本 | — |
| GET | `/healthz` | 存活探针 | — |
| GET | `/api/version` | 结构化版本与运行时长 | — |
| GET | `/api/metrics` | 运行时指标（内存/GC/协程） | — |
| GET | `/debug/pprof/` | pprof 索引 | — |
| GET | `/debug/pprof/cmdline` | pprof cmdline | — |
| GET | `/debug/pprof/profile` | pprof profile | — |
| GET | `/debug/pprof/symbol` | pprof symbol | — |
| GET | `/debug/pprof/trace` | pprof trace | — |
| GET | `/debug/pprof/heap` | pprof heap | — |
| GET | `/debug/pprof/goroutine` | pprof goroutine | — |
| GET | `/debug/pprof/allocs` | pprof allocs | — |
| GET | `/debug/pprof/block` | pprof block | — |
| GET | `/debug/pprof/mutex` | pprof mutex | — |
| GET | `/debug/pprof/threadcreate` | pprof threadcreate | — |

### 订阅管理（需要 `-data-dir`）

| 方法 | 路径 | 说明 | 需要 `-data-dir` |
|------|------|------|------------------|
| GET | `/api/stats` | 统计概览 | ✓ |
| GET | `/api/history` | 转换历史 | ✓ |
| GET | `/api/pipelines` | 命名管线列表（DB + 模板合并） | ✓ |
| POST | `/api/pipelines` | 新增命名管线 | ✓ |
| GET | `/api/pipelines/{id}` | 单个命名管线（含 steps） | ✓ |
| PUT | `/api/pipelines/{id}` | 更新命名管线 | ✓ |
| DELETE | `/api/pipelines/{id}` | 删除命名管线 | ✓ |
| GET | `/api/targets` | 渲染 target 列表 | ✓ |
| GET | `/api/rulesets` | 规则集列表 | ✓ |
| GET | `/api/logs` | 结构化日志流 | ✓ |
| GET | `/api/airports` | 机场列表 | ✓ |
| POST | `/api/airports` | 新增机场 | ✓ |
| POST | `/api/airports/refresh` | 刷新全部启用机场流量（`?with_rules=1` 同步导入规则） | ✓ |
| POST | `/api/airports/check` | 订阅测活 | ✓ |
| GET | `/api/airports/check/last` | 最近测活结果 | ✓ |
| GET | `/api/airports/{id}` | 单个机场 | ✓ |
| PUT | `/api/airports/{id}` | 更新机场 | ✓ |
| DELETE | `/api/airports/{id}` | 删除机场 | ✓ |
| POST | `/api/airports/{id}/refresh` | 刷新单个机场流量 | ✓ |
| POST | `/api/airports/{id}/import-rules` | 导入规则 | ✓ |
| GET | `/api/groups` | 分组列表 | ✓ |
| POST | `/api/groups` | 新增分组 | ✓ |
| GET | `/api/groups/{id}` | 单个分组 | ✓ |
| PUT | `/api/groups/{id}` | 更新分组 | ✓ |
| DELETE | `/api/groups/{id}` | 删除分组 | ✓ |
| GET | `/api/rules` | 规则列表 | ✓ |
| POST | `/api/rules` | 新增规则 | ✓ |
| POST | `/api/rules/bulk` | 批量启用/停用 | ✓ |
| GET | `/api/rules/{id}` | 单条规则 | ✓ |
| PUT | `/api/rules/{id}` | 更新规则 | ✓ |
| DELETE | `/api/rules/{id}` | 删除规则 | ✓ |
| GET | `/api/tokens` | token 列表 | ✓ |
| POST | `/api/tokens` | 新增 token | ✓ |
| DELETE | `/api/tokens/{token}` | 删除 token | ✓ |
| GET | `/api/self-nodes` | 自测节点列表 | ✓ |
| POST | `/api/self-nodes` | 新增自测节点 | ✓ |
| GET | `/api/self-nodes/{id}` | 单个自测节点 | ✓ |
| PUT | `/api/self-nodes/{id}` | 更新自测节点 | ✓ |
| DELETE | `/api/self-nodes/{id}` | 删除自测节点 | ✓ |
| GET | `/api/traffic/strategy` | 流量策略 | ✓ |
| PUT | `/api/traffic/strategy` | 更新流量策略 | ✓ |
| POST | `/api/traffic/refresh` | 刷新全部流量 | ✓ |
| GET | `/api/traffic/preview` | 流量试算 | ✓ |
| GET | `/api/scripts` | 脚本列表 | ✓ |
| POST | `/api/scripts` | 新增脚本 | ✓ |
| POST | `/api/scripts/test` | 脚本试运行 | ✓ |
| GET | `/api/scripts/{id}` | 单个脚本 | ✓ |
| PUT | `/api/scripts/{id}` | 更新脚本 | ✓ |
| DELETE | `/api/scripts/{id}` | 删除脚本 | ✓ |
| GET | `/api/tasks` | 定时任务执行历史 | ✓ |
| GET | `/api/alert-channels` | 告警通道列表（脱敏） | ✓ |
| PUT | `/api/alert-channels` | 整体替换告警通道 | ✓ |
| POST | `/api/alert-channels/test` | 告警通道连通性测试 | ✓ |
| GET | `/api/rule-providers` | 规则提供者列表 | ✓ |
| GET | `/api/suggestions` | Rule-Bot 建议队列 | ✓ |
| POST | `/api/suggestions/apply` | 应用 Rule-Bot 建议 | ✓ |
| POST | `/api/suggestions/{index}/confirm` | 确认建议 | ✓ |
| POST | `/api/suggestions/{index}/reject` | 拒绝建议 | ✓ |
| GET | `/api/rule-sources` | 规则源列表 | ✓ |
| POST | `/api/rule-sources/update` | 更新规则源 | ✓ |
| POST | `/api/rule-sources/rollback` | 回滚规则源 | ✓ |
| POST | `/api/rule-sources/mrs` | MRS 离线生成 | ✓ |
| GET | `/api/settings` | 设置 | ✓ |
| PUT | `/api/settings` | 更新设置 | ✓ |
| GET | `/api/template` | 模板 | ✓ |
| PUT | `/api/template` | 更新模板 | ✓ |
| GET | `/api/override` | 覆盖 DSL | ✓ |
| PUT | `/api/override` | 更新覆盖 DSL | ✓ |

### SPA 回退

| 方法 | 路径 | 说明 | 需要 `-data-dir` |
|------|------|------|------------------|
| GET | `/*` | 静态资源 / 前端路由回退 | — |
| HEAD | `/*` | 同上 | — |

## 构建

要求 Go 1.24+。本项目**无 npm**，前端统一使用 `pnpm`。

### 纯后端构建（不需要 Node）

仓库里 `internal/api/web/dist/` 只放有一个 `.gitkeep` 占位文件（前端产物**不入库**），
`//go:embed all:web/dist` 只要求目录存在，因此**干净 clone 上 `go build ./...` 能过**（embed 会把占位目录编进二进制，SPA 路由在运行时才会 404）。

```sh
go build ./...                  # 后端编译（干净 clone 可直接跑）
go test ./...                   # 测试
go build ./cmd/subforge         # 本地构建二进制
go run ./cmd/subforge version   # 版本信息
go run ./cmd/subforge doctor    # 环境与运行时自检
```

等价的 Makefile 目标：`make build`（仅后端）、`make test`、`make vet`。

### 完整构建（含前端）

前端工程在 `web/`，使用 Vite + Vue 3 + Vuetify 3 + TypeScript，产物直接输出到 `internal/api/web/dist/`
（`emptyOutDir: true`，构建时会清空重建，脚本末尾补回 `.gitkeep`）：

```sh
cd web
pnpm install --frozen-lockfile
pnpm run build                  # = vue-tsc --noEmit && vite build && node scripts/ensure-dist-placeholder.mjs
```

`pnpm run build` 的产物落在 `internal/api/web/dist/`（`index.html` + `assets/*` + `.gitkeep`）。
构建完成后 `go build ./...` 会把前端一并嵌入二进制。

一键完整验证：`make verify`（`web-coverage` + `web-build` + `test` + `coverage-gate` + `go vet`）。

### 开发模式

```sh
cd web
pnpm run dev                    # 开发服务器（Vite HMR，端口 5173）
pnpm run test                   # 单元测试（Vitest）
pnpm run coverage               # 覆盖率门禁（函数 100% + 条件 ≥80%）
pnpm run typecheck              # 类型检查（vue-tsc --noEmit）
```

### 工具链注意

- 本机**没有 npm**，统一用 `pnpm`（`pnpm-lock.yaml` 已提交，使用 `--frozen-lockfile`）。
- 若 `pnpm` 报 `'node' 不是内部或外部命令`，把 Node 的 bin 目录放进 `PATH`：
  `C:\Users\<you>\.dsh\dsh-runtimes\dsh-primary-runtime\dependencies\node\bin`。
- `make` 在 Windows 上可能不可用；可用 `scripts/verify.cmd` 或直接按 Makefile recipe 逐条跑。

### 交叉编译（Linux 服务器 / NAS / 软路由）

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/subforge
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/subforge
```

等价的 Makefile 目标：`make cross`（交付矩阵：linux/amd64|arm64|arm、windows/amd64、darwin/amd64|arm64、freebsd/amd64、linux/mipsle|mips softfloat）。

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
