# SubForge 文档索引

本文件是 SubForge 仓库全部文档的入口。无论你是使用者、运维人员还是贡献者，
都可以从这里定位到需要的资料。

阅读本索引前，请先明确一条总原则：

> **源码是唯一事实来源。** 任何文档与实现不一致时，以源码为准，并欢迎提交 Issue
> 或 Pull Request 修正文档。设计类文档记录的是当时的决策过程，不保证与当前实现同步；
> 实现期对设计的落地与修订以 `docs/adr/` 中的架构决策记录为准。

---

## 一、按角色选择阅读路径

### 1.1 使用者（只想把订阅转成能用的配置）

1. 仓库根目录 [README.md](../README.md) — 项目定位、快速开始、命令行与 API 摘要。
2. [Wiki 使用手册](wiki/Home.md) — 完整教程与原理讲解，建议按首页给出的学习路径阅读。
3. 遇到问题查 Wiki 的 [故障排查与常见问题](wiki/故障排查与常见问题.md)。

### 1.2 运维人员（要长期维护一套转换服务）

1. Wiki [安装与部署](wiki/安装与部署.md) — Docker、systemd、OpenWrt 三种落地方式。
2. Wiki [Web-控制台使用指南](wiki/Web-控制台使用指南.md) — 日常管理入口。
3. Wiki [定时任务与告警](wiki/定时任务与告警.md) 与 [流量策略与用量统计](wiki/流量策略与用量统计.md)。
4. [docs/reports/](reports/) — 质量核查与审计记录，用于评估版本可靠性。

### 1.3 贡献者（要改代码或写文档）

1. 仓库根目录 [CONTRIBUTING.md](../CONTRIBUTING.md) — 环境准备、提交规范、CI 检查项。
2. Wiki [系统架构总览](wiki/系统架构总览.md) 与 [转换流水线原理](wiki/转换流水线原理.md)。
3. Wiki [开发与测试指南](wiki/开发与测试指南.md) — 构建、测试与覆盖率门禁。
4. [docs/design/](design/) 与 [docs/adr/](adr/) — 设计背景与关键决策。
5. [docs/style-guide.md](style-guide.md) — 编写或修改文档时必须遵循的写作规范。

---

## 二、目录结构

```text
docs/
├── README.md                 # 本文件：文档总索引
├── style-guide.md            # 文档写作规范（中英文风格、术语、Mermaid 约束）
├── compatibility.md          # 与 subconverter 的已知行为差异表（金测副产品）
├── m2-aethersailor-compat.md # 与 Aethersailor 规则集的兼容性分析
├── spike-mihomo.md           # mihomo 配置校验的技术预研记录
├── adr/                      # 架构决策记录（ADR）
│   └── 0001-stack-decisions.md
├── design/                   # 设计文档与设计期产物
│   ├── README.md             # 设计文档目录说明
│   ├── subforge-design.md    # 总体设计书
│   ├── feature-exploration-v2.md
│   ├── m2-m4-design.md
│   ├── web-ui-rewrite-plan.md
│   ├── vite-vuetify-coverage-plan.md
│   ├── frontend-visual-upgrade-plan.md
│   ├── frontend-visual-upgrade-decision.md
│   ├── ui-copy-and-config-plan.md
│   ├── ux-audit-plan.md
│   ├── ux-capability-matrix.md
│   ├── cicd-github-actions-plan.md
│   ├── mockups/              # 静态视觉稿与校验脚本
│   ├── research/             # 技术选型调研
│   ├── screenshots/          # 控制台界面截图
│   └── tasks/                # 设计期任务分解（T1–T12）
├── reports/                  # 质量核查与审计报告
│   ├── README.md
│   └── quality-audit.md
└── wiki/                     # GitHub Wiki 的源文件
    ├── Home.md               # Wiki 首页
    ├── _Sidebar.md           # Wiki 侧边导航
    ├── _Footer.md            # Wiki 页脚
    └── ...                   # 其余 21 个正文页面
```

---

## 三、各目录职责

| 目录 | 职责 | 是否与实现同步 | 维护要求 |
| --- | --- | --- | --- |
| `docs/wiki/` | 面向使用者的完整手册，发布到 GitHub Wiki | 必须同步 | 改行为必须同步改 Wiki |
| `docs/design/` | 设计期的方案、计划、调研与视觉稿 | 不保证同步 | 仅作历史记录，不回填 |
| `docs/adr/` | 架构决策记录，记录"为什么这样选" | 决策变更时新增记录 | 不修改已有记录，只追加 |
| `docs/reports/` | 质量核查、审计与验收结论 | 按次归档 | 每次核查新增一份 |
| `docs/compatibility.md` | 与 subconverter 的行为差异对照 | 必须同步 | 金测发现差异时更新 |

---

## 四、仓库根目录文档

| 文件 | 面向对象 | 内容 |
| --- | --- | --- |
| [README.md](../README.md) | 全部读者 | 中文主文档：定位、特性、快速开始、命令行、API、配置、部署 |
| [README_en.md](../README_en.md) | 英文读者 | 英文主文档，与中文版结构一致 |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | 贡献者 | 开发环境、分支与提交规范、测试要求、CI 检查项、发布流程 |
| [SECURITY.md](../SECURITY.md) | 全部读者 | 安全政策、漏洞报告流程、部署安全建议 |
| [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) | 全部参与者 | 社区行为准则 |
| [LICENSE](../LICENSE) | 全部读者 | MIT 许可证 |
| [docs/style-guide.md](style-guide.md) | 文档作者 | 文案风格、术语表、图表与示例规范 |

`.github/` 目录下另有 Issue 与 Pull Request 模板，用于收集结构化的反馈信息。

---

## 五、文档维护规则

1. **新增文档先选目录。** 面向使用者的操作与原理说明放 `docs/wiki/`；方案与调研放
   `docs/design/`；不可逆的技术决策放 `docs/adr/`；核查结论放 `docs/reports/`。
2. **命名使用小写连字符或中文标题。** 英文文件名用小写与连字符（`kebab-case`），
   Wiki 页面名使用中文标题，且必须与 `_Sidebar.md` 中的链接完全一致。
3. **Wiki 不在网页端编辑。** Wiki 的源文件是 `docs/wiki/` 下的 Markdown，
   网页端修改会在下次发布时被覆盖。发布方式见 `scripts/publish-wiki.ps1`。

   GitHub 的 Wiki 仓库（`<owner>/<repo>.wiki.git`）**在网页端创建首个页面之前并不存在**，
   此时 `git ls-remote` 与 `git push` 都会报 `Repository not found`。因此首次发布前需要
   人工完成一次引导：打开 `https://github.com/tktogether/subforge/wiki`，点击
   `Create the first page` 建立任意首页（内容可以是占位文本），之后执行
   `pwsh -File scripts/publish-wiki.ps1` 即可用仓库中的 `Home.md` 覆盖该占位页。
   引导完成后不再需要网页端操作，后续所有页面均由脚本同步。

   日常发布与预览：

   ```powershell
   pwsh -File scripts/publish-wiki.ps1 -DryRun                     # 预览将新增/更新/删除的页面
   pwsh -File scripts/publish-wiki.ps1 -Message "docs(wiki): ..."  # 正式发布
   ```

   脚本会保留 `_Sidebar.md` 与 `_Footer.md` 这两个 GitHub Wiki 专用页面，
   并默认删除 Wiki 上已不存在于 `docs/wiki/` 的多余页面（用 `-NoPrune` 关闭）。
   脚本同时兼容 Windows PowerShell 5.1 与 PowerShell 7；若环境中没有 `pwsh`，
   把示例中的 `pwsh` 换成 `powershell` 即可。
4. **行为变更必须同步文档。** 修改命令行参数、HTTP 端点、INI 段落、默认值时，
   必须在同一个 Pull Request 中更新受影响的 README 与 Wiki 页面。
5. **示例必须可执行。** 文档中的命令与配置片段应当可以直接复制执行；
   无法直接执行的片段要明确标注为示意。
6. **写作遵循规范。** 中英文文案风格、术语、图表与自检清单见
   [docs/style-guide.md](style-guide.md)。

---

## 六、文档质量自检

提交文档改动前，至少确认以下各项：

- [ ] 文档中出现的路径、命令、参数、端点均能在仓库中找到对应实现。
- [ ] 相对链接指向的文件真实存在（Wiki 页面之间的链接使用页面名）。
- [ ] Mermaid 代码块使用 ` ```mermaid ` 标注，且语法可被 GitHub 正常渲染。
- [ ] 术语与 [docs/style-guide.md](style-guide.md) 的术语表一致。
- [ ] 未实现或已废弃的能力已明确标注状态，未使用"即将支持""敬请期待"等模糊表述。

其中前四项可以交给仓库自带的文档自检脚本自动完成：

```bash
node scripts/check-docs.mjs
```

该脚本为零依赖的 Node.js 程序，会遍历仓库根目录与 `docs/` 下的全部 Markdown 文件，
检查代码围栏闭合性、Mermaid 图类型与高危写法、链接可达性、Wiki 页面标题一致性，
并在发现问题时以退出码 1 结束。加 `--quiet` 可只输出统计与问题清单。
脚本的完整说明见 [CONTRIBUTING.md 的文档贡献一节](../CONTRIBUTING.md)。

### 6.1 可选：用 Mermaid 官方解析器做真解析校验

`check-docs.mjs` 只能做结构层面的检查，无法证明 Mermaid 语法真的能被渲染。
图表改动较多时（例如新增或重构流程图），建议再用 Mermaid 官方解析器逐张真解析一次。
该依赖不进入仓库，只在本地临时安装：

```bash
mkdir -p .tmp/mmcheck && cd .tmp/mmcheck
npm install mermaid@11 jsdom --no-save
# 再写一个 Node 脚本：用 jsdom 提供 window/document，逐个提取 Markdown 里的
# Mermaid 代码块并调用 mermaid.parse() 校验
```

判定标准：所有图块解析失败数为 0。`mermaid.parse()` 会抛出带行号的语法错误，
比人工比对更可靠。该目录已被 `.gitignore` 忽略，不会进入提交。
