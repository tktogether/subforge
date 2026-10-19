# CI/CD GitHub Actions 计划

> 2026-10-19 · 随第六次审计后立项。定位修正：**打包流水线为主**（产物分发），不搞 SSH 部署 workflow——测试机部署走本地工具链（tools/rsh210.py + switchN.sh 规程），GitHub runner 没有必要持有测试机凭据。

## 1. CI 现状（.github/workflows/ci.yml，已在库）

| job | 触发 | 内容 |
| --- | --- | --- |
| web | push main / PR / 手动 / 每日 18:00 UTC | pnpm install --frozen-lockfile → typecheck → coverage 门禁 → build → 上传 dist artifact |
| test (ubuntu/windows 矩阵) | web 之后 | 下载 dist → go build / vet / `go test ./... -count=1` / gofmt（Linux） |
| cross-build | web 之后 | `make cross` 多平台交付矩阵 |
| bench | web 之后 | `-bench=. -benchmem` 结果存 artifact |
| docker-build | web 之后 | scratch + alpine 双镜像构建 + 容器内 `version` 冒烟 |
| coverage | web 之后 | coveragegate -min 80 -min-func 100 + condgate -min 80 |
| condition-audit (manual) | 仅 workflow_dispatch | gobco 条件覆盖 ≥80%（纯手动审计，无定时） |

## 2. 打包流水线（.github/workflows/release.yml，本次新增）

- **触发**：push tag `v*`（正式发布）或 workflow_dispatch（手动打包快照）。
- **产物**（`dist/`）：
  - 二进制矩阵与 `make cross` 逐字一致：linux/amd64 arm64 arm、windows/amd64（.exe）、darwin/amd64 arm64、freebsd/amd64，共 7 个（mips/mipsle 已移除：modernc.org/sqlite 不提供该平台构建，交叉编译必炸 build constraints exclude——2026-10-19 CI cross job 实测，本地 GOOS=linux GOARCH=mips 复现 13 包 exclude）；
  - `sha256sums.txt` 校验清单；
  - docker 镜像 scratch（Dockerfile）→ `ghcr.io/<owner>/subforge:<tag>` + `:latest`（仅 tag push 推送）。
- **质量门**（打包前自动拦）：ELF 魔数抽查（D.11 规程）+ `./subforge-linux-amd64 version` 冒烟（buildinfo 非 dev、commit 可追）。
- **交付**：tag 触发 → GitHub Release 自动建包（`generate_release_notes`）；手动触发 → Actions artifact 下载。
- **版本号**：tag 名进 `buildinfo.Version`（不再是 dev），commit/Date 照常注入——`version` 命令自证来源。

## 3. 需要配置的环境变量 / Secrets

**打包流水线：零 Secrets。** 推 ghcr 用的是 Actions 自动注入的 `GITHUB_TOKEN`（由 `permissions: packages: write` 授权），不需要手动建任何凭据。仓库推上 GitHub 后 workflow 直接可用。

| 名称 | 类型 | 说明 |
| --- | --- | --- |
| `GITHUB_TOKEN` | 自动注入 | Release 创建 + GHCR 推镜像，无需配置 |
| （可选）`DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` | Secret | 仅当要同时推 Docker Hub 时才配；默认只用 GHCR |
| （可选）SSH 测试机凭据 | — | **不进 GitHub**。测试机部署维持本地工具链（rsh210.py），凭据留在 `tools/.creds210`（gitignore） |

## 4. 凭据治理记录（2026-10-19 审计）

- **审计范围**：全历史 124 commit × 全部 blob，模式库 = `.creds210` 实际值（host/port/user/password）+ 通用凭据正则（PRIVATE KEY / ghp_ / AKIA / xox / Bearer / api_key 等）。
- **结论（唯一泄露）**：`tools/docker-build/main.go` 明文硬编码测试机 root 密码，存在于 63 个 commit 的 blob（从引入 commit 直至审计 HEAD）；该文件因先于 `/tools/` ignore 规则被追踪而滞留。早期 ssh-runner 已在 c52d509 清理为环境变量读取，`msg.txt` 仅含 env 变量说明；`tools/.creds210` 从未入库。
- **处置**：`git filter-repo --invert-paths --path tools/docker-build --force` 全历史抹除（重写后 123 commit，全历史密码 grep = **0 命中**）；清理前备份 `E:\Projects\subforge-backup-pre-filter.bundle`；仓库无 remote 无协作者，改写零协调成本。历史 sha 全变（`78f3507 → c21acf9` 等）。
- **遗留动作（用户）**：测试机 root 密码必须轮换——旧密码在本地历史 blob 与审计输出中出现过，按已泄露处理。
- **防再犯**：`tools/` 整目录已 ignore；部署类凭据只留在本地 gitignore 文件/环境变量，**不进 GitHub Secrets**（打包流水线不需要它们）；入库前跑一次 §4 模式扫描。
