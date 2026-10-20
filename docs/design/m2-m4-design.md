# SubForge M2–M4 设计方案（个人自用收敛版）

> 本文档是 [subforge-design.md](subforge-design.md) 的延续，聚焦 M1 之后剩余里程碑的具体设计。
> 设计目标：**个人自用**——单用户、单实例、无多租户/计费/审计合规负担。
> 所有决策闭环，无悬而未决项。

---

## 0. 个人自用约束（Scope Lock）

| 维度 | 决策 |
|------|------|
| 用户数 | 1（本人） |
| 部署形态 | 本地开发机 / NAS / VPS，单实例 |
| 鉴权 | 本机免鉴权；LAN/VPS 通过 scope-token + IP allowlist |
| 多租户 | 不做 |
| 计费/配额 | 不做 |
| 审计日志 | 仅本地结构化日志，无合规审计 |
| 上游数量 | ≤ 20 订阅 + ≤ 10 规则集（个人场景足够） |
| 并发 | ≤ 8 并发转换请求 |
| 内存预算 | 常驻 < 60MB（M1 已达标 <30MB，M2+ 增量 <30MB） |

---

## 1. M2 定制化（4 周）

### 1.1 管线引擎（Pipeline Engine）

**目标**：Parse → Filter/Operate 链 → Render，链式可组合、可复用、可调试。

**Stage 流程**：

```
Input (订阅/文件/URL)
  ↓
[Parse] → 内部模型 (Node/Group/Rule)
  ↓
[Filter Chain] → 筛选节点（正则/地区/类型/脚本）
  ↓
[Operate Chain] → 属性设置/改名/排序/脚本
  ↓
[Render] → 目标格式输出
```

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 链式组合 | 有序 slice，每步独立可测试 | 简单、可组合、易调试 |
| 错误处理 | 单步失败可 skip/abort/warn 三策略 | 个人场景容错优先 |
| 脚本引擎 | goja（ES5.1+） | 嵌入简单、无 cgo、社区成熟 |
| 脚本隔离 | 无网络/无文件系统/无 os 调用 | 安全 |
| 脚本超时 | 5s 默认，可配置 | 防死循环 |
| 脚本 panic | recover + 返回错误 | 不 crash 主进程 |
| 链式复用 | 命名 pipeline 模板（INI `[pipeline]`） | 复用常见组合 |

**数据结构**：

```go
type Pipeline struct {
    Name     string   `yaml:"name"`
    Filters  []Filter `yaml:"filters"`
    Operates []Operate `yaml:"operates"`
}

type Filter struct {
    Type    string `yaml:"type"`    // regex / region / type / script
    Pattern string `yaml:"pattern"` // 正则或参数
    Script  string `yaml:"script"`  // goja 脚本路径
    OnError string `yaml:"on_error"` // skip / abort / warn
}

type Operate struct {
    Type    string `yaml:"type"`    // set / rename / sort / script / resolve
    Params  map[string]any `yaml:"params"`
    OnError string `yaml:"on_error"`
}
```

**验收**：
- 单步失败三策略行为正确
- goja 脚本可访问节点字段（name/region/type/server）
- 命名模板可复用

### 1.2 分层覆盖 DSL（Override DSL）

**目标**：超高度定制化——group/rule/proxy 三级覆盖链。

**语法**（YAML/INI 双支持）：

```yaml
# 基础层（远程模板）
groups:
  - name: "AI"
    type: select
    proxies: [HK, JP, SG]

# 覆盖层 1（用户本地）
override:
  groups:
    - name: "AI"
      proxies: [HK, JP, SG, US]  # 追加 US
  rules:
    - prepend:
        - "DOMAIN-SUFFIX,openai.com,AI"
    - append:
        - "DOMAIN-SUFFIX,claude.ai,AI"
  proxies:
    - name: "HK"
      server: "new-hk.example.com"  # 覆盖 server
```

**覆盖优先级链**（高 → 低）：

```
URL 参数 > 用户 override > 远程模板 > 内置默认
```

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 覆盖语义 | merge（数组 append，标量 replace） | 直觉、可预测 |
| 冲突检测 | 启动时 warn 重复定义 | 个人场景简单 |
| 回滚 | 删除 override 即回滚 | 无需版本控制 |
| 预览 | `subforge diff` 显示覆盖前后差异 | 调试友好 |

**验收**：
- 三级覆盖链行为正确
- `subforge diff` 输出可读差异
- 冲突检测 warn

### 1.3 Collection/Provider 透传

**目标**：多订阅聚合为单 URL + Provider 缓存透传。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 聚合模式 | merge（节点合并，分组按名归并） | 简单、可预测 |
| 缓存策略 | ETag + 磁盘缓存（24h TTL） | 减少上游拉取 |
| 并发拉取 | ≤ 4 并发 | 防机场限流 |
| 部分失败 | 成功节点仍输出 + warn 日志 | 容错 |
| Provider 透传 | 原样透传 provider 字段 | 兼容 mihomo |

**验收**：
- 3 订阅聚合为 1 URL
- ETag 304 短路
- 部分失败容错

### 1.4 Web UI（embed）

**目标**：嵌入式 Web UI，渐进 disclosure，instrument-grade 密度。

**技术栈**：

| 层 | 选择 | 理由 |
|----|------|------|
| 框架 | 纯 HTML + CSS + 原生 JS（无构建） | 零依赖、embed 简单 |
| 状态 | 服务端渲染 + 局部 fetch 更新 | 简单、无 SPA 复杂度 |
| 路由 | 服务端路由（`/`, `/convert`, `/rules`, `/logs`） | 无前端路由库 |
| 样式 | 手写 CSS（OKLCH 变量） | 完全控制、无框架 |

**页面结构**（渐进 disclosure）：

```
/ (Dashboard)
├── 快速转换（URL → target → 下载）
├── 状态概览（节点数/分组数/规则数/缓存命中率）
└── 最近转换历史（最近 10 条）

/convert (Convert)
├── 输入（URL/文件/文本）
├── 管线选择（命名模板 or 自定义链）
├── 输出（target 选择 + 预览 + 下载）
└── 高级（覆盖 DSL 编辑 / diff 预览）

/rules (Rules)
├── 规则集列表（远程/本地/覆盖）
├── 规则树（按 group 分组）
└── 覆盖编辑器（YAML/INI 双模式）

/logs (Logs)
├── 结构化日志流（实时 tail）
├── 过滤（level/module/keyword）
└── 导出（JSON/文本）
```

**设计 Context**（来自仓库根的本地设计上下文文件 `.impeccable.md`；该文件不入版本库，克隆后不存在，此处仅作来源说明）：

- **Typography**: `Bricolage Grotesque`（display）+ `Geist`（body）+ `Geist Mono`（mono）
- **Color**: OKLCH 变量，light/dark 双主题，accent = `oklch(0.55 0.16 45)`（warm amber）
- **Spacing**: 4pt scale（4/8/12/16/24/32/48），`gap` 优先
- **Layout**: 左对齐、非对称、无居中 hero
- **Motion**: ease-out-quint，transform/opacity only
- **Bans**: 无 side-stripe、无 gradient text、无 glassmorphism、无 nested cards

**验收**：
- 4 页面全部可用
- 渐进 disclosure 行为正确
- light/dark 主题切换
- 无 AI 模板感（impeccable 检查清单通过）

---

## 2. M3 规则工厂（3 周）

### 2.1 manifest 聚合

**目标**：多上游规则集自动跟进 + 版本管理。

**数据结构**：

```yaml
# manifest.yaml
sources:
  - name: "ACL4SSR"
    url: "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/..."
    type: "clash"
    update_interval: "24h"
  - name: "Loyalsoldier"
    url: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/..."
    type: "clash"
    update_interval: "24h"

output:
  format: "clash-meta"
  path: "/etc/subforge/rules/"
```

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 更新策略 | 定时拉取 + ETag 缓存 | 简单、可预测 |
| 版本管理 | 文件名含版本号（`acl4ssr-20261016.yaml`） | 可回滚 |
| 失败处理 | 保留上一版本 + warn | 容错 |
| 并发 | ≤ 4 并发 | 防限流 |

**验收**：
- 8 家上游自动跟进
- 版本回滚可行
- 失败容错

### 2.2 MRS 离线生成

**目标**：mihomo rule-set 二进制（.mrs）离线生成。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 生成时机 | 规则集更新后自动触发 | 实时性 |
| 缓存 | 磁盘缓存（7d TTL） | 减少重复生成 |
| 校验 | mihomo 加载测试 | 兼容性 |

**验收**：
- MRS 与 Actions 产出字节级兼容抽检通过
- mihomo 加载成功

### 2.3 定时任务/告警

**目标**：定时转换 + 异常告警。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 调度器 | 内嵌 cron（robfig/cron） | 无外部依赖 |
| 告警通道 | Telegram / DingTalk / SMTP | 个人场景足够 |
| 失败重试 | 3 次指数退避 | 容错 |
| 告警内容 | 简洁（时间/类型/错误/链接） | 不 spam |

**验收**：
- 定时转换执行
- 告警发送成功
- 失败重试行为正确

### 2.4 Rule-Bot 闭环

**目标**：日志驱动的规则反馈闭环。

**流程**：

```
转换日志 → 异常检测（节点不可达/规则冲突/格式错误）
  ↓
生成规则建议（prepend/append/replace）
  ↓
人工确认 → 自动提交 override
```

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 异常检测 | 规则引擎（正则 + 阈值） | 简单 |
| 建议生成 | 模板化（基于异常类型） | 可预测 |
| 人工确认 | Web UI 确认面板 | 安全 |
| 自动提交 | 确认后写入 override | 闭环 |

**验收**：
- 异常检测准确率 > 90%
- 建议生成可读
- 人工确认后自动提交

### 2.5 测活（subs-check）

**目标**：节点可用性检测 + 流媒体解锁检测。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 默认状态 | **关闭** | 防机场封号 |
| 并发 | ≤ 8 | 防限流 |
| 频率 | 每节点 ≥ 5min 间隔 | 防封号 |
| 流媒体检测 | 可选（Netflix/Disney+/ChatGPT） | 个人场景 |
| 结果格式 | JSON + 标记节点 | 兼容 mihomo |

**验收**：
- 默认关闭
- 开启后并发/频率限制生效
- 结果标记正确

### 2.6 OpenWrt/Docker 交付

**目标**：多平台镜像 + OpenWrt 包。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 镜像 | scratch + alpine 双版本 | 体积/兼容性 |
| 交叉编译 | linux/amd64, linux/arm64, linux/mipsle | 覆盖主流 |
| OpenWrt | ipk 包 | 路由器场景 |
| 更新 | 手动触发 + 版本号 | 简单 |

**验收**：
- 多平台镜像构建成功
- OpenWrt 包安装成功

---

## 3. M4 打磨（2 周）

### 3.1 诊断面

**目标**：可观测性 + 快速排查。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| pprof | `/debug/pprof/` | Go 标准 |
| trace | `/debug/trace` | 请求级追踪 |
| doctor CLI | `subforge doctor` | 网络/配置/规则诊断 |
| 日志 | 结构化 JSON + level | 可 grep |

**验收**：
- pprof 可用
- doctor 输出可读

### 3.2 性能基准 CI 化

**目标**：benchmark 回归检测。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 基准 | parse/transform/convert 三组 | 覆盖核心路径 |
| 阈值 | 回归 > 10% 报警 | 合理 |
| 平台 | ubuntu-latest | CI 标准 |

**验收**：
- CI 自动跑 benchmark
- 回归报警

### 3.3 Worker 轻模式（R2 决策）

**目标**：Cloudflare Workers 子集解析器。

**决策门**（M3 结束后二选一）：

| 选项 | 条件 | 代价 |
|------|------|------|
| 交付子集解析器 | 纯 Go 解析器 < 100KB + 内存 < 30MB | 双解析器维护 200~300 行/年 |
| 不交付 | 无 | 失去 Worker 场景 |

**建议**：M3 结束后评估，若 M2/M3 进度顺利则交付。

### 3.4 包版本 matrix

**目标**：多平台包版本管理。

**设计决策**：

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 版本号 | semver | 标准 |
| 包格式 | deb/rpm/ipk/tar.gz | 覆盖主流 |
| 签名 | GPG | 安全 |

**验收**：
- 多平台包构建成功
- GPG 签名验证

---

## 4. 个人自用发散功能（M5+ optional）

> 以下为 M2-M4 之外的延展功能，按优先级排序。

### 4.1 自保项（P2，建议 M2 前置）

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| 结构化日志 | JSON 格式 + level + req-id | 低 |
| `/metrics` | Prometheus 指标端点 | 低 |
| `/healthz` | 健康检查 + 就绪探针 | 低 |
| 令牌范围化 | read-only / convert / admin 三级 | 中 |
| 请求限流 | 令牌桶 + IP 维度 | 低 |
| SSRF 防护 | 上游 URL 域名白名单 | 中 |
| govulncheck | 依赖 CVE 扫描 | 低 |

### 4.2 数据/缓存增强

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| 分布式缓存 | Redis/Memcached 可插 | 中 |
| 持久化 Collection | 用户订阅组持久化 | 中 |
| 缓存预热 | 定时预热常用订阅 | 低 |
| 缓存清理 CLI | `subforge cache gc` | 低 |

### 4.3 协议/兼容增强

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| AnyTLS/WireGuard/TUIC v5 | 全链路 import/export | 中 |
| Shadowrocket/Stash/Egern | 独立 target 模板 | 中 |
| MRS 二进制 `/getruleset` | 补充二进制形态 | 低 |

### 4.4 DX/集成

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| Plugin 机制 | WASM/WASI 插件 | 高 |
| CLI 高级命令 | `diff` / `doctor` / `export --template` | 低 |
| 库模式 | `pkg/subforge` 可 import | 中 |
| MCP server | AI agent 工具接口 | 中 |

### 4.5 AI-native（后置）

| 功能 | 描述 | 复杂度 |
|------|------|--------|
| 自然语言分流规则 | "AI 流量走 HK" → 生成 ruleset | 中 |
| 订阅异常检测 | 节点质量滑动/证书异常 | 中 |

---

## 5. 验收标准汇总

| 里程碑 | 验收标准 |
|--------|----------|
| M2 | Aethersailor 8 模板零修改跑通；sub-web 可直连；管线/覆盖 DSL/goja 测试通过 |
| M3 | 8 家上游自动跟进；MRS 字节级兼容；测活默认关闭；OpenWrt/Docker 交付 |
| M4 | §8 全指标 CI 化；pprof/doctor 可用；Worker 轻模式决策完成 |

---

## 6. 风险与缓解

| # | 风险 | 概率 | 影响 | 缓解 |
|---|------|------|------|------|
| R6 | goja 脚本能力不足 | 中 | 低 | 仅非热路径 + 文档明示 ES5.1+ 范围 |
| R7 | 测活并发冲击机场 | 低 | 高 | 默认关闭 + 并发 8 + 频率限制 |
| R8 | mihomo 内部 API 变动 | 高 | 中 | adapter 层隔离 + 锁版本 + CI 回归 |
| R9 | 规则数据许可证传染 | 确认 | 中 | 运行时拉取 + 缓存，不静态打包 |

---

**设计人**：SubForge 项目组
**设计日期**：2026-10-16
**状态**：设计完成，待实施
