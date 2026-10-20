# 部署

四种交付形态：静态二进制、systemd 单元、容器镜像、OpenWrt 安装包。产物都是同一个
`./cmd/subforge` 单二进制，无外部依赖、无需运行时。

本页只写"读者据此操作"的内容：命令、前置条件、失败时会看到的报错。各项验证的原始数据
（字节数、sha256、镜像层大小、构建耗时）归档在
[docs/reports/](../docs/reports/README.md)，不在此重复。

## 1. 静态二进制

```sh
make build    # 本机架构，产物 bin/subforge
make cross    # 交叉编译七个目标到 bin/，文件名形如 subforge-linux-amd64
make release  # 在 cross 之后把 bin/ 打包成 dist/*.tar.gz
```

交叉编译在任意一台 amd64 主机上完成，不需要目标平台工具链（`CGO_ENABLED=0`，纯 Go）。
Release 页面提供的就是裸可执行文件，不是压缩包，下载后 `chmod +x` 即可运行：

```sh
curl -fsSLO https://github.com/tktogether/subforge/releases/latest/download/subforge-linux-amd64
chmod +x subforge-linux-amd64
./subforge-linux-amd64 version
```

单二进制在 36～38 MiB 量级（见[发布验证报告](../docs/reports/v0.1.2-release-verification.md)），
按 40 MB 规划镜像、分区或下载预算。同一个 tag 的资产体积随 Go 工具链补丁版本浮动，
`make cross` 与 CI 的差异属正常。

## 2. systemd 单元

`deploy/subforge.service` 适用于 systemd ≥ 229 的发行版（Ubuntu 22.04 起）。它有两项前置
条件，都在下面的命令里备好了：以 `User=subforge` 运行（用户不存在会启动失败），以及
`--config /etc/subforge/subforge.ini` 指向的文件必须存在（见后文）。

```sh
make cross    # 先产出 bin/subforge-linux-amd64（make build 只出 bin/subforge）
sudo useradd --system --no-create-home --shell /usr/sbin/nologin subforge
sudo install -d -o subforge -g subforge -m 0750 /etc/subforge
sudo install -o subforge -g subforge -m 0640 /dev/null /etc/subforge/subforge.ini
sudo install -o root -g root -m 0755 bin/subforge-linux-amd64 /usr/local/bin/subforge
sudo install -o root -g root -m 0640 deploy/subforge.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now subforge
systemctl status subforge
```

`ExecStart` 里的 `--config /etc/subforge/subforge.ini` **指向的文件必须先存在**：指向不存在的
路径时 `serve` 立刻退出，报
`subforge: 读取配置 <path>: open <path>: no such file or directory`（退出码 1）；空文件可以正常
启动，所有配置项都有内置默认值，按需往里面填即可。缓存目录 `/var/lib/subforge/cache` 由单元里的
`StateDirectory=subforge` 自动创建，不需要手工 `mkdir`。

单元默认只监听 `127.0.0.1:25500`，没有写 `--token`。默认监听本机时不设令牌是安全的；
一旦要把监听地址改成 `0.0.0.0`，**必须先设令牌**——程序对"绑到所有网卡且无令牌"这组组合
只打印警告、不拒绝启动，暴露到局域网前请自行确认。INI 配置走 mtime 轮询热重载，改完
不需要重启。

`StartLimitIntervalSec` / `StartLimitBurst` 必须写在 `[Unit]` 而不是 `[Service]`，放错节
systemd 会报 `Unknown key name ... ignoring` 并静默忽略限流配置。

## 3. 容器镜像

两条路线：**拉取公开镜像**（最快）与**自行构建**（实际部署推荐，可带版本信息、可选运行层）。
差别只在运行层与平台支持：

| 路线 | 运行层 | 能否抓 HTTPS 上游 | 平台 |
| --- | --- | --- | --- |
| 拉取 `ghcr.io/tktogether/subforge:latest` | `alpine:3.20` + `ca-certificates` + `tzdata` | 能 | `linux/amd64` + `linux/arm64` |
| 拉取 `ghcr.io/tktogether/subforge:latest-scratch` | `scratch` | **不能** | `linux/amd64` + `linux/arm64` |
| 构建 `Dockerfile.alpine` | `alpine:3.20` + `ca-certificates` + `tzdata` | 能 | 随构建机，可用 buildx 出多架构 |
| 构建 `Dockerfile` | `scratch` | **不能** | 随构建机，可用 buildx 出多架构 |

> **`scratch` 运行层里没有根证书，抓 HTTPS 订阅源必然失败**，报错是
> `tls: failed to verify certificate: x509: certificate signed by unknown authority`。
> `scratch` 镜像里连 `/bin/sh` 都不存在，进去排查也做不到。它只适合 `http://` 上游或
> 本地文件输入。需要 HTTPS 上游就用默认标签或 `Dockerfile.alpine`。

### 路线一：拉取公开镜像

```sh
docker pull ghcr.io/tktogether/subforge:latest
docker run -d --name subforge -p 25500:25500 -v subforge-data:/data \
    ghcr.io/tktogether/subforge:latest serve -listen 0.0.0.0:25500 -token <YOUR_TOKEN>
```

镜像仓库公开，匿名可拉。`:latest` 与 `:v0.1.2` 是同一份（alpine 运行层、双架构、自带版本
号），`:latest-scratch` 与 `:v0.1.2-scratch` 是同一份（scratch 运行层）。`:v0.1.0` 与
`:v0.1.1` 仍在，但 `v0.1.0` 只有 amd64 且不带版本号，别再用。

### 路线二：自行构建

```sh
# 推荐：带根证书的 alpine 运行层，并把版本信息编进二进制
docker build -f Dockerfile.alpine \
    --build-arg VERSION=v0.1.2 \
    --build-arg COMMIT=$(git rev-parse --short HEAD) \
    --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
    -t subforge:alpine .

# 可选：最小体积的 scratch 运行层
docker build -f Dockerfile -t subforge:scratch .

# 需要 arm64 时用 buildx 一次出两个架构
docker buildx build --platform linux/amd64,linux/arm64 -f Dockerfile.alpine -t subforge:alpine .
```

不传 `--build-arg` 时镜像自报 `dev (commit none, built unknown)`
（`Dockerfile:37-39`、`Dockerfile.alpine:28-30`）。两个 Dockerfile 的构建阶段都固定在
`$BUILDPLATFORM` 上原生执行，只把目标平台交给 Go 交叉编译（`GOOS`/`GOARCH` 取自 buildx 注入的
`TARGETOS`/`TARGETARCH`），所以多架构构建不需要指令级模拟。

> **不要在 `Dockerfile` 里写 `ARG LDFLAGS=...${VERSION}...`。** Docker 在声明 ARG 时就把
> `${VERSION}` 展开成声明处的默认值，`--build-arg VERSION=...` 覆盖不到它，镜像会永远自报
> `dev (commit none, built unknown)`。要在 `go build` 的 `-ldflags` 里直接引用 ARG
> （`Dockerfile:54-59`、`Dockerfile.alpine:44-49`）。

### 运行

容器默认命令是 `serve`（`Dockerfile:74-75`、`Dockerfile.alpine:67-68`），覆盖命令即可执行
其他子命令：

```sh
docker run --rm -v subforge-data:/data subforge:alpine convert \
    -file /data/nodes.txt -format sing-box -out /data/cfg.json
```

`-listen 0.0.0.0:25500` **不能省**：`serve` 默认只监听 `127.0.0.1:25500`
（`cmd/subforge/serve.go:100`），在容器里就是容器自己的回环网卡，`-p` 发布出去的端口打不通
（宿主机 `curl` 得到 `HTTP=000` / `Recv failure: Connection reset by peer`，容器内回环访问
却是 200）。

### docker compose

`docker-compose.yml` 构建 `Dockerfile.alpine`（`docker-compose.yml:9`），并在
`container_name` 之后显式补了 `command: [serve, -listen, 0.0.0.0:25500]`
（`docker-compose.yml:23`）——**没有这段 `command` 时端口发布同样打不通**。

```sh
docker compose up -d --build
curl http://127.0.0.1:25500/healthz    # {"status":"ok"}
```

`docker-compose.yml` 与两个 Dockerfile 里的 `SUBFORGE_DATA`、`SUBFORGE_CONFIG` 只用于声明
路径，`serve` **不读取**这两个变量；非测试代码里读取环境变量的只有 `SUBFORGE_DATA_DIR`
（`cmd/subforge/doctor.go:62`）、`PATH`（`cmd/subforge/doctor.go:126`）、
`TELEGRAM_CHAT_ID`（`cmd/subforge/scheduler.go:47`）、`SUBFORGE_DEBUG`
（`internal/api/pprof.go:22`）。监听地址、令牌等参数以命令行参数与 INI 配置文件为准。

## 4. OpenWrt

打包定义在 `deploy/openwrt/`：`Makefile` 是 SDK 包定义，`files/subforge.init` 是 procd
启动脚本，`files/subforge.conf` 是 UCI 配置。

**前提：先把交叉编译好的二进制放进 `deploy/openwrt/bin/subforge`**（arm64 目标放
`bin/subforge-arm64` 并改名）。这一步不做，打包会直接停在
`ERROR: deploy/openwrt/bin/subforge 不存在。`——这是 `deploy/openwrt/Makefile:23-30` 的
显式检查。

```sh
# 1) 产出待打包的静态二进制（在源码仓库根目录执行）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -mod=vendor -ldflags "-s -w" \
  -o deploy/openwrt/bin/subforge ./cmd/subforge

# 2) 解压官方 SDK（此处以 24.10.4 x86/64 为例）
mkdir -p /opt/sdk-x86
tar --zstd -xf openwrt-sdk-24.10.4-x86-64_gcc-13.3.0_musl.Linux-x86_64.tar.zst \
    -C /opt/sdk-x86 --strip-components=1
cd /opt/sdk-x86
export FORCE_UNSAFE_CONFIGURE=1      # 以 root 编译时需要
make defconfig

# 3) 把包放进 SDK 的本地 package 目录
mkdir -p package/subforge/bin
cp -a <repo>/deploy/openwrt/Makefile package/subforge/
cp -a <repo>/deploy/openwrt/files    package/subforge/
cp -a <repo>/deploy/openwrt/bin/subforge package/subforge/bin/

# 4) 打包
make package/subforge/compile V=s
```

产物落在 **`<sdk>/bin/packages/<PKGARCH>/base/`**（不是 `bin/targets/...`），文件名形如
`subforge_1.0.0-r1_x86_64.ipk`。x86/64 与 `aarch64_cortex-a72` 两个目标都实测成功，
完整记录见 [OpenWrt 打包验证报告](../docs/reports/openwrt-packaging-verification.md)。

### 检查 ipk 内容

这个 SDK 产出的 `.ipk` **外层是 gzip 包一层 tar，不是 `ar` 归档**（`ar t` 会报
`file format not recognized`），要看内容必须先 `gunzip`：

```sh
gunzip -c subforge_1.0.0-r1_x86_64.ipk > pkg.tar
tar -tf pkg.tar                    # ./debian-binary ./data.tar.gz ./control.tar.gz
tar -xzf data.tar.gz               # ./usr/bin/subforge ./etc/init.d/subforge ./etc/config/subforge
```

`control` 与 `Makefile` 的声明一致（`Package`、`Version`、`Depends: libc, ca-bundle`、
`Architecture`、`License: MIT`、`Section: net`）。x86/64 包里的二进制可直接运行：
`./usr/bin/subforge version` 退出码 0。

### 两个必须知道的坑

**1. `files/subforge.conf` 的 UCI 选项当前没有任何代码读取。** `files/subforge.init` 里
没有 `config_load`/`config_get`，脚本硬编码了 `SUBFORGE_DATA`、`SUBFORGE_CONFIG` 两个
环境变量，而这两个变量全仓库无人读取——`serve` 的入口是命令行 flag。所以 UCI 里改 `port`、
`data_dir`、`log_level` **不会生效**，要改行为只能编辑 init 脚本里的命令行。这是已知缺陷，
详见 `docs/wiki/安装与部署.md` 第 7.4 节。

**2. 包内二进制比放进去的源文件小几 KB，这是正常的。** SDK 打包时会对安装树跑
`scripts/rstrip.sh`，它对 ELF 可执行文件调用 `staging_dir/host/bin/sstrip -z` 削掉节头表等
运行时不需要的数据。被处理过的二进制 `file` 会报 `too large section header offset`，但仍能
正常运行。

### 改了二进制之后怎么重打

直接重跑 `make package/subforge/compile V=s` 就够了，**不需要先 `clean`**。要清理时用
`make package/subforge/clean V=s`，不要手工 `rm -rf build_dir` 里的目录——SDK 把
`.prepared_<hash>` 标记和 `.pkgdir/` 都放在 `$(PKG_BUILD_DIR)` 内部，手工删掉会让 SDK
重跑 `Build/Prepare`（本身无害，但属于 SDK 不保证的状态）。

### 未验证

没有在真实 OpenWrt 设备或 rootfs 上执行过 `opkg install`，因此 `postinst`、`procd` 拉起、
`ca-bundle` 依赖解析、设备端 `opkg` 是否接受上述 gzip 外层格式都还没有实测。

## 相关文档

- [发布验证报告 v0.1.2](../docs/reports/v0.1.2-release-verification.md)：镜像与 Release 资产的原始数据
- [OpenWrt 打包验证报告](../docs/reports/openwrt-packaging-verification.md)：ipk 结构、control 全文、复现步骤
- [质量核查报告](../docs/reports/quality-audit.md)：交叉编译矩阵与兼容性
- [安装与部署（Wiki）](../docs/wiki/安装与部署.md)：四种方式的完整教程
