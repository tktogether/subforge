# 交付

三个入口：静态二进制、systemd 单元、容器镜像。产物都是同一个 `./cmd/subforge`
的单二进制，没有外部依赖。

## 1. 静态二进制

```
make cross    # 产物在 dist/
```

实测体积（`-trimpath -ldflags="-s -w"`，`CGO_ENABLED=0`，Go 1.26.3）：

| 目标 | 体积 | 形态 |
| --- | --- | --- |
| linux/amd64 | 23,154,850 B（22.1 MiB） | ELF 64-bit x86-64，`ldd` 报 not a dynamic executable |
| linux/arm64 | 21,495,970 B（20.5 MiB） | ELF 64-bit ARM aarch64，静态 |
| windows/amd64 | 22,009,856 B（21.0 MiB） | PE32+ console x86-64 |

三个目标都在 15–25 MB 预算内。交叉编译在 linux/amd64 主机上完成，无需目标平台工具链。

## 2. systemd 单元

`deploy/subforge.service` 已在 Ubuntu 22.04 上实测：

- `systemd-analyze verify` 零警告（`StartLimitIntervalSec` / `StartLimitBurst`
  属于 `[Unit]` 而不是 `[Service]`，systemd ≥229 起移到 `[Unit]`；放错节会报
  `Unknown key name ... ignoring`）
- `systemctl start subforge` → `active (running)`
- 空载 RSS **27,436 KB（26.8 MiB）**，9 个线程，稳态 CPU 0.0% —— 在 30 MB
  预算内
- 只监听 `127.0.0.1:25500`，未开额外端口

安装：

```sh
sudo install -o root -g root -m 0755 dist/subforge-linux-amd64 /usr/local/bin/subforge
sudo install -o root -g root -m 0640 deploy/subforge.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now subforge
```

单元里没写 `--token`。默认监听本机时不设令牌是安全的；一旦要把 `--listen`
改成 `0.0.0.0`，必须先设令牌——程序会在启动时拦下"绑到所有网卡且无令牌"这组
组合。改 INI 配置走 mtime 轮询热重载，不需要重启。

## 3. 容器镜像

`Dockerfile` 是两阶段：`golang:1.26-bookworm` 构建、`scratch` 运行。

```
docker build -t subforge:latest .
docker build --platform linux/arm64 -t subforge:arm64 .
docker run -d --name subforge -p 25500:25500 -v subforge-data:/data subforge:latest
```

**已知限制**：镜像层本身尚未在任何机器上构建过。本地未安装 Docker；已用的远端
主机内核没有 overlayfs 挂载点、`lsmod` 里没有 overlay 模块，`dockerd` /
`containerd` 起不来。Dockerfile 里的构建配方（同一份 ldflags、`CGO_ENABLED=0`、
`-mod=vendor`、`-trimpath`）已由等价命令行在 linux/amd64 与 linux/arm64 上实测，
产物经 `file` 与 `ldd` 确认是纯静态；拿到可用 Docker 后需补一次 `docker build`。

## 已完成的实机验证

远端 Ubuntu 22.04 / Go 1.26.3，sing-box v1.14.3 官方二进制：

- `go build ./...`、`go vet`、全量 `go test ./...` 全绿
- 两份 sing-box 金测 + CLI 转换产物 + **HTTP API 返回的产物**，全部过
  `sing-box check -c`
- 端到端链路：`python3 -m http.server` 当上游订阅 → `GET /sub?target=sing-box&url=...`
  → 200 / 5546 B / 合法 JSON / 13 出站 → 该产物过 `sing-box check`
- `/healthz` 200、`/version` 200 带 uptime、缺参 `/sub` 返回 400 而不是 500
- `/getruleset` 六种形态（1~6 与命名别名）全部 200，`type=99` 与缺 `name`
  返回带说明的 400
