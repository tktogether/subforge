# CI/CD GitHub Actions 计划

> 2026-10-19 · 随第六次审计后立项。前置事实：仓库当前无 remote，推送 GitHub 前凭据治理已收口（见 §4）。

## 1. CI 现状（.github/workflows/ci.yml，已在库）

| job | 触发 | 内容 |
| --- | --- | --- |
| web | push main / PR / 手动 / 每日 18:00 UTC | pnpm install --frozen-lockfile → typecheck → coverage 门禁 → build → 上传 dist artifact |
| test (ubuntu/windows 矩阵) | web 之后 | 下载 dist → go build / vet / `go test ./... -count=1` / gofmt（Linux） |
| cross-build | web 之后 | `make cross` 多平台交付矩阵 |
| bench | web 之后 | `-bench=. -benchmem` 结果存 artifact |
| docker-build | web 之后 | scratch + alpine 双镜像构建 + 容器内 `version` 冒烟 |
| coverage | web 之后 | coveragegate -min 80 -min-func 100 + condgate -min 80 |
| condition-nightly | 仅 schedule/手动 | gobco 条件覆盖 ≥80% |

结论：CI 覆盖已达标，**唯一缺口是 CD**。

## 2. CD 设计（.github/workflows/deploy.yml，本次新增）

- **触发**：仅 `workflow_dispatch`，且要求输入 `confirm=DEPLOY` 双重确认——测试机部署不出自动门（D.11 的 MZ 二进制事故 + SSH 通道中断事故的教训：部署必须可控、可确认、可回滚）。
- **流程**（逐条对应 D.11–D.13 人工规程）：
  1. web build → 交叉编译 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`，ldflags 注入 `buildinfo.{Version=dev,Commit=<8位sha>,Date=<UTC>}`；
  2. **上传前 ELF 魔数校验**（`7f 45 4c 46`，防 PE 误传）；
  3. scp 上传 `/tmp/subforge.new<sha>`；
  4. 远端脚本文件 `/tmp/switch-ci.sh`（heredoc 落盘后执行，**不依赖 SSH 通道存活**）：记旧 pid → 备份 `/tmp/subforge.bak-<ts>` + `subforge.db.pre-<sha>` → kill（10s 宽限 → kill -9）→ mv 换二进制 → `setsid nohup` 同参数重启 → 打印新旧 pid 与备份路径；
  5. 验收：`/healthz` 200 + `/version` 必须含新 commit（不匹配即 fail）；
  6. 失败时打印回滚提示（最新 `/tmp/subforge.bak-*` 路径）。
- **并发**：`concurrency: deploy-210` 串行，防两次部署互踩。
- **已知取舍**：SSH HostKey 校验暂不启用（与现人工流程一致）；后续可加 `DEPLOY_HOSTKEY` secret 走严格校验。建议中期把密码认证升级为 ed25519 key（见 §3 注）。

## 3. 需要配置的环境变量（GitHub → Settings → Secrets and variables → Actions）

| 名称 | 类型 | 值 | 说明 |
| --- | --- | --- | --- |
| `DEPLOY_HOST` | Secret | `210.us1.firadio.net` | 测试机地址 |
| `DEPLOY_PORT` | Secret | `21005` | SSH 端口 |
| `DEPLOY_USER` | Secret | `root` | SSH 用户 |
| `DEPLOY_PASSWORD` | Secret | **轮换后的新密码**（勿用旧值） | SSH 密码；旧值已按泄露处理（§4） |
| （可选）`DEPLOY_HOSTKEY` | Secret | 服务器 host key | 启用严格 HostKey 校验时再加 |

- 服务端口 21009、远端目录 `/tmp`、数据目录 `/tmp/subforge-data` 为非敏感拓扑常量，已硬编码在 workflow env，不入 Secrets。
- **推荐升级**：服务器加 `authorized_keys`（ed25519），workflow 换 `appleboy/ssh-action` 的 `key:` 字段读 `DEPLOY_SSH_PRIVATEKEY`，随后关闭密码登录——一次性消灭密码轮换问题。

## 4. 凭据治理记录（2026-10-19 审计）

- **审计范围**：全历史 124 commit × 全部 blob，模式库 = `.creds210` 实际值（host/port/user/password）+ 通用凭据正则（PRIVATE KEY / ghp_ / AKIA / xox / Bearer / api_key 等）。
- **结论（唯一泄露）**：`tools/docker-build/main.go` 明文硬编码测试机 root 密码，存在于 63 个 commit 的 blob（从引入 commit 直至审计 HEAD）；该文件因先于 `/tools/` ignore 规则被追踪而滞留。早期 ssh-runner 已在 c52d509 清理为环境变量读取，`msg.txt` 仅含 env 变量说明；`tools/.creds210` 从未入库。
- **处置**：`git filter-repo --invert-paths --path tools/docker-build --force` 全历史抹除（重写后 123 commit，全历史密码 grep = **0 命中**）；清理前备份 `E:\Projects\subforge-backup-pre-filter.bundle`；仓库无 remote 无协作者，改写零协调成本。历史 sha 全变（`78f3507 → c21acf9` 等）。
- **遗留动作（用户）**：测试机 root 密码必须轮换——旧密码在本地历史 blob 与审计输出中出现过，按已泄露处理；轮换后新值才配 `DEPLOY_PASSWORD`。
- **防再犯**：`tools/` 整目录已 ignore；后续新增部署工具一律走环境变量（c52d509 先例），入库前跑一次 §4 模式扫描。
