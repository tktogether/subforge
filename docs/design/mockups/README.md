# 样板目录（Wave 1 · 视觉方向对照）

本目录存放三个**可直接用浏览器打开的静态样板**，供用户目视拍板；它们不是产品代码，
不参与构建，也不被 `web/**` 引用。三份样板只做视觉方向对照，功能与数据均为示意。

| 样板 | 方向 | 文件 | 交付卡 |
|---|---|---|---|
| D1「Instrument」 | 仪表感（精密、刻度化） | `d1-instrument.html` | T4 |
| D2「Editorial」 | 编辑感（暖中性、阅读优先） | `d2-editorial.html` | T5 |
| D3「Console」 | 控制台（高密度、表格优先、状态语义化） | `d3-console.html` | T6 |

> D1 / D2 由另外两张卡（T4 / T5）交付，可能尚未落盘；本目录的核对工具对三者同样适用。

## 渲染核对：`render-check.ps1`

用途：用无头 Chrome 在**指定视口**下量出页面内每个 `.table-wrap` 的横向溢出，并可顺手出图。

```powershell
# 只量溢出（默认 1600 宽）
& .\render-check.ps1 -Html (Resolve-Path .\d3-console.html).Path

# 指定视口 + 出图
& .\render-check.ps1 -Html (Resolve-Path .\d3-console.html).Path -Width 1280 -OutPng "$env:TEMP\d3-1280.png"
```

- 参数：`-Html`（必填，绝对路径）、`-Width`（默认 1600，最小 320）、`-OutPng`（可选，绝对路径）。
- 输出：每个 `.table-wrap` 一行 `scrollWidth / clientWidth / over`，末行 `PASS`（全 0，exit 0）或 `FAIL`（exit 1）。
- 量测必须带 `--window-size=<W>,1000`：漏掉它会得到 `over=0` 的**假结果**（Chrome 视口默认宽度不同）。
- 依赖：硬编码 `C:\Program Files\Google\Chrome\Application\chrome.exe`，不存在则报错退出（exit 1）。
- 临时文件只写 `$env:TEMP`（脚本结束即删），**不会**在本目录留下残留文件。
- 本机实测无 `pwsh`（PowerShell 7）在 PATH 上；用 Windows PowerShell 5.1 以 `&` 调用即可（脚本自身以 UTF-8 BOM 保存，5.1 才能正确解析其中的中文）。

## 契约来源（以这些文件为准，不要凭样板猜）

- 验收协议与红线：[frontend-visual-upgrade-plan.md](../frontend-visual-upgrade-plan.md) §1、§2、附录 B.4（样板渲染核对协议）、B.5（红线第 7 条）
- 设计令牌与语义：[subforge-design.md](../subforge-design.md)；D3 的 19 个 `--sf-*` 令牌同时列在 `d3-console.html` 页头注释与页脚令牌表
- 视觉配方来源：[research/T2-visual-recipes.md](../research/T2-visual-recipes.md)（配方 C「Graphite Console」对应 D3 方向）
- 落地阶段（Wave 3）才动 `web/**`；Wave 1 只允许新建 `docs/design/research/**` 与 `docs/design/mockups/**`

## 单元格门禁：`cell-fit-check.ps1`

用无头 Chrome 量出每张表**每个 th/td** 的三判据，任一命中即 BAD：

- **SPILL**：任一后代的 getBoundingClientRect 超出单元格 rect +0.5px，且中途无 overflow hidden/clip 祖先（真溢出）。
- **NOTITLE**：td.scrollWidth > clientWidth 但单元格/后代均无 title（截断无 title）。
- **STRICT**：表头含 `名称/节点数/流量/到期时间/更新时间` 的列必须 scrollWidth <= clientWidth。

CLI：`-Html`（必填绝对路径）、`-Width`（默认 1600）、`-OutPng`（可选）、`-SelfTest`（无 Html，四夹具自证门禁未被改松）。

退出码：全过 0，否则 1；末行 `PASS: N cells fit` 或 `FAIL: k/N cells bad`，每条 BAD 行写明 SPILL/NOTITLE/STRICT。

```powershell
& .\cell-fit-check.ps1 -SelfTest
& .\cell-fit-check.ps1 -Html (Resolve-Path .\d3-console.html).Path -Width 1600
```

（UTF-8 带 BOM 保存，PS 5.1 下才能正确解析中文。）
