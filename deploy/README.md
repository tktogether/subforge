# 交付

四个入口：静态二进制、systemd 单元、容器镜像、OpenWrt 安装包。产物都是同一个
`./cmd/subforge` 的单二进制，没有外部依赖。

## 1. 静态二进制

```
make cross    # 交叉编译七个目标，产物写入 bin/，文件名形如 subforge-linux-amd64
make release  # 在 cross 之后把 bin/ 下的产物打包成 dist/*.tar.gz
```

发行版 v0.1.0 的 GitHub Release 资产实测体积（逐个取自 Release API 的 `size`
字段，非估算）：

| 目标 | 字节 | MiB | 形态 |
| --- | --- | --- | --- |
| linux/amd64 | 40,046,754 | 38.2 | ELF 64-bit x86-64，静态 |
| linux/arm64 | 37,945,506 | 36.2 | ELF 64-bit ARM aarch64，静态 |
| linux/arm | 38,273,186 | 36.5 | ELF 32-bit ARM，静态 |
| windows/amd64 | 38,848,512 | 37.0 | PE32+ console x86-64 |
| darwin/amd64 | 39,011,280 | 37.2 | Mach-O 64-bit x86-64 |
| darwin/arm64 | 37,127,426 | 35.4 | Mach-O 64-bit ARM64 |
| freebsd/amd64 | 38,088,876 | 36.3 | ELF 64-bit x86-64，静态 |

七个目标落在 35.4–38.2 MiB 区间，**不存在 15–25 MB 这一档**——任何按 20 MB 量级
规划的镜像、分区或下载预算都需要按 40 MB 重新核算。交叉编译在 linux/amd64 主机上
完成，无需目标平台工具链。

**测量口径**：上表是 Release 页面上的原始可执行文件本身，不含压缩。构建命令为
`.github/workflows/release.yml:86-97`（与 `Makefile:29-36` 的 `make cross` 同参数），
参数是 `-trimpath -mod=vendor -ldflags "-s -w"`、`CGO_ENABLED=0`，并额外注入
`-X .../internal/buildinfo.{Version,Commit,Date}`（`Version` 取标签名 `v0.1.0`）。
Release 资产是用 **Go 1.26.0** 构建的：`release.yml:74-77` 的 `actions/setup-go@v5`
配的是 `go-version-file: go.mod`，而 `go.mod:3` 写的是 `go 1.26.0`。

**本机复现与体积差异的成因（已查明）**：用同一组参数在本机（Go 1.26.3）复现 linux/amd64
得到 **40,075,426 B**，比 Release 资产大 28,672 B。差异**全部来自 Go 工具链的补丁版本**：
把本机工具链换成 1.26.0（`docker run golang:1.26.0-bookworm`）用同一命令构建，体积
**恰好等于 Release 资产的 40,046,754 B**（差值 0），构建信息中的 `go1.26.0`、`GOAMD64=v1`、
`vcs.revision=7a2738b3…` 也逐项吻合。已排除的其它可能原因：注入 `buildinfo` 三个变量
（实测体积不变）、`-buildvcs`、`-s` 与 `-w` 的取舍、提交号长度。sha256 无法逐字节对齐的
原因是 Release 由 CI 在干净检出上构建（`vcs.modified=false`），本机是在有未提交改动的
工作树上构建（`vcs.modified=true`），且构建时间戳不同；体积口径不受影响。

## 2. systemd 单元

`deploy/subforge.service` 已在 Ubuntu 22.04 上实测：

- `systemd-analyze verify` 零警告（`StartLimitIntervalSec` / `StartLimitBurst`
  属于 `[Unit]` 而不是 `[Service]`，systemd ≥229 起移到 `[Unit]`；放错节会报
  `Unknown key name ... ignoring`）
- `systemctl start subforge` → `active (running)`
- 空载 RSS **27,436 KB（26.8 MiB）**，9 个线程，稳态 CPU 0.0%
- 只监听 `127.0.0.1:25500`，未开额外端口

安装：

```sh
sudo install -o root -g root -m 0755 dist/subforge-linux-amd64 /usr/local/bin/subforge
sudo install -o root -g root -m 0640 deploy/subforge.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now subforge
```

单元里没写 `--token`。默认监听本机时不设令牌是安全的；一旦要把 `--listen`
改成 `0.0.0.0`，必须先设令牌——程序会在启动时对"绑到所有网卡且无令牌"这组
组合打印警告（只警告、不拒绝启动），暴露到局域网前请自行确认令牌已设置。
改 INI 配置走 mtime 轮询热重载，不需要重启。

## 3. 容器镜像

容器镜像有两条路线：**直接拉取公开镜像**（最快，适合先跑起来）与**自行构建**（推荐
用于实际部署：可以带上版本信息，也可以选带根证书的运行层）。两者差别集中在运行层与平台
支持上：

| 路线 | 运行层 | 能否抓 HTTPS 上游 | 镜像平台 |
| --- | --- | --- | --- |
| 拉取公开镜像 `ghcr.io/tktogether/subforge` | `scratch` | **不能**（镜像内无根证书，见下文实测） | 仅 `linux/amd64` |
| 自行构建 `Dockerfile.alpine` | `alpine:3.20` + `ca-certificates` + `tzdata` | 能 | 随构建机架构，也可用 buildx 出多架构 |
| 自行构建 `Dockerfile` | `scratch` | **不能**（上游必须是 `http://` 或本地文件） | 随构建机架构，也可用 buildx 出多架构 |

两个 Dockerfile 都已参数化目标平台：构建阶段固定在 `$BUILDPLATFORM` 上原生执行，只把目标
平台交给 Go 交叉编译，`GOOS`/`GOARCH` 取自 buildx 注入的 `TARGETOS`/`TARGETARCH`
（`Dockerfile:27-29`、`Dockerfile:51-53`；`Dockerfile.alpine:23-25`、`Dockerfile.alpine:44-49`），
默认值 `linux`/`amd64`。因此 `docker build` 出的是构建机架构，`docker buildx build
--platform linux/amd64,linux/arm64` 能一次出两个架构，且编译过程不需要指令级模拟。

> **已发布镜像与仓库当前工作区不是同一份东西。** 下面路线一里关于"只有 amd64 / 自报
> `dev (commit none, built unknown)` / 用 `Dockerfile` 构建"的结论，测的是**已推送到 GHCR 的
> 那份镜像**，它由 `.github/workflows/release.yml` 的**已提交版本**产出。仓库工作区里已经有一份
> 未提交的流水线修正（见"已修正但尚未发布"一节），修正落地并重新发版后，公开镜像的这三个
> 特征都会变。引用时请写清是哪一份。

### 路线一：拉取公开镜像

```sh
docker pull ghcr.io/tktogether/subforge:latest
docker run -d --name subforge -p 25500:25500 -v subforge-data:/data \
    ghcr.io/tktogether/subforge:latest serve -listen 0.0.0.0:25500 -token <YOUR_TOKEN>
```

- 镜像仓库公开，**匿名即可拉取**（实测）。可用 tag 为 `v0.1.0` 与 `latest`；两者指向同一
  个 `linux/amd64` manifest。`v0.1.0` 的 digest 是
  `sha256:418f7ef051dae9d1323f3be52c099c09a4d7d0cc752f4d35ddf6aef3c942909c`，
  压缩后 15,964,766 B，构建于 2026-10-19T17:21:57Z。
- **只有 `linux/amd64` 一个平台**：镜像 manifest 是 OCI image index，除一个 attestation
  条目外只含 amd64 一个平台（`docker manifest inspect` 实测）。这是**已发布的那一份**的限制，
  原因是产出它的流水线版本没有写 `platforms:`；工作区里未提交的修正已经补上，见下文。
- **镜像自报版本为 `subforge dev (commit none, built unknown)`**：产出它的流水线版本没有传
  `build-args`，`VERSION`、`COMMIT`、`DATE` 都停在 Dockerfile 的默认值
  （`Dockerfile:32-34`、`Dockerfile.alpine:28-30`、`internal/buildinfo/buildinfo.go:8-10`）。
- **该镜像是 `scratch` 运行层，镜像内没有任何根证书，抓 HTTPS 订阅源会直接失败。** 实测用一个
  与 `Dockerfile:55-69` 运行层同构的 scratch 镜像抓 HTTPS 上游，报
  `tls: failed to verify certificate: x509: certificate signed by unknown authority`（退出码 1）；
  换成与 `Dockerfile.alpine:51-54` 同构的 alpine 运行层抓同一个 URL 则成功（12 个节点）。
  因此公开镜像只适合 `http://` 上游或本地文件输入，**要抓 HTTPS 订阅源请走路线二构建 alpine 版**。

### 路线二：自行构建

| 文件 | 基础镜像 | 说明 |
| --- | --- | --- |
| `Dockerfile.alpine` | 构建阶段 `golang:1.26-bookworm`，运行阶段 `alpine:3.20` | 额外安装 `ca-certificates` 与 `tzdata`，**实际部署推荐** |
| `Dockerfile` | 构建阶段 `golang:1.26-bookworm`，运行阶段 `scratch` | 最小镜像，只含静态二进制与许可证；**无根证书** |

```sh
# 推荐：带根证书的 alpine 运行层，并把版本信息编进二进制
docker build -f Dockerfile.alpine \
    --build-arg VERSION=v0.1.0 \
    --build-arg COMMIT=$(git rev-parse --short HEAD) \
    --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
    -t subforge:alpine .

# 可选：最小体积的 scratch 运行层
docker build -f Dockerfile -t subforge:scratch .

# 需要 arm64 时用 buildx 一次出两个架构（无需指令级模拟）
docker buildx build --platform linux/amd64,linux/arm64 -f Dockerfile.alpine -t subforge:alpine .

docker run -d --name subforge -p 25500:25500 -v subforge-data:/data \
    subforge:alpine serve -listen 0.0.0.0:25500 -token <YOUR_TOKEN>
docker run --rm -v subforge-data:/data subforge:alpine convert -file /data/nodes.txt -format sing-box -out /data/cfg.json
```

容器默认命令是 `serve`（`Dockerfile:67-69`、`Dockerfile.alpine:66-68`）；覆盖命令即可执行
`convert`、`version`、`doctor` 等其他子命令。`-listen 0.0.0.0:25500` 不能省：`serve` 默认只监听
`127.0.0.1:25500`（`cmd/subforge/serve.go:100`），在容器里就是容器自己的回环网卡，端口发布
对它无效（见下文实测）。不传 `--build-arg` 时自报版本同样是 `dev (commit none, built
unknown)`，这正是公开镜像的现状。

上面两条 `docker build` 都把镜像打成了 `subforge:*` 这样的本地名字，**不要用 `subforge:latest`
这个本地名去覆盖公开镜像的标签语义**——`ghcr.io/tktogether/subforge:latest` 是仓库名带 registry
前缀的另一个引用，两者不会互相覆盖，但同一个 shell 里混用容易看错，建议本地名统一带后缀。

### 实测结果

**镜像层已于 2026-10-20 在 WSL2 Ubuntu 22.04 + Docker 29.1.3（overlayfs 后端）上实测
构建成功**，不再是"配方未验证"状态：

| 镜像 | 构建命令 | 退出码 | 耗时 | 体积 |
| --- | --- | --- | --- | --- |
| `subforge:verify-scratch` | `docker build -f Dockerfile .` | 0 | 40s | 56.1 MB |
| `subforge:verify-alpine` | `docker build -f Dockerfile.alpine .` | 0 | 60s | 71.7 MB |

两个镜像内的二进制均为 `ELF 64-bit LSB executable, x86-64, statically linked,
stripped`，各 40,091,810 B，`ldd` 报 `not a dynamic executable`；`version` 退出码 0；
`convert` 实测成功（12 节点 → `clash.meta` 4845 B / 4870 B，`sing-box` 5546 B 且
JSON 合法）。scratch 镜像跑 `serve` 时端口映射为 `0.0.0.0:25500->25500/tcp`，
设 `-token` 后无令牌访问 `/healthz`、`/version`、`/` 均返回 401。

`docker-compose.yml` 已改为构建 `Dockerfile.alpine`（`docker-compose.yml:9`，原因就是根证书），
并在 `container_name` 之后显式补了 `command: [serve, -listen, 0.0.0.0:25500]`
（`docker-compose.yml:23-26`），使端口发布真正可达。**没有这段 `command` 时宿主机访问已发布
端口打不通**：`curl http://127.0.0.1:25500/healthz` 得到 `HTTP=000`（`Recv failure:
Connection reset by peer`），而容器内回环访问是 `HTTP=200`——端口发布转发到的是容器内回环。
补上后宿主机访问恢复 `HTTP=200`（`{"status":"ok"}`）。详见 `docs/wiki/安装与部署.md` 第 5.4 节。

**产出已发布镜像的那一版流水线有三个缺陷**（它们解释了路线一里的三个限制；详见
`docs/wiki/安装与部署.md` 第五节）：

1. 那一版的 build-push 步骤**没有传 `build-args`**，因此已发布镜像里的版本号是 Dockerfile 的
   ARG 默认值 `dev (commit none, built unknown)`（`Dockerfile:32-34`、
   `internal/buildinfo/buildinfo.go:8-10`），与同一 Release 中带 `VERSION/COMMIT/DATE`
   的二进制产物不一致。
2. 那一版的注释写"scratch + alpine，推 GHCR"，但 `file:` 只指向 `./Dockerfile`
   ——**alpine 镜像从未被构建或推送**。
3. 那一步没有 `platforms:`，**已发布镜像只有 linux/amd64 一个架构**；仓库内也没有任何
   Docker Hub 的登录与推送配置。

### 已修正但尚未发布

仓库工作区里已经有一份**尚未提交**的 `release.yml` 改动，把上面三条都修掉了：

- 默认标签改指 `Dockerfile.alpine`，scratch 版走 `-scratch` 后缀的标签
  （`.github/workflows/release.yml:126-153`）；
- 两个镜像都写了 `platforms: linux/amd64,linux/arm64`
  （`.github/workflows/release.yml:131`、`:145`）；
- 两个镜像都显式传 `build-args`，取值来自统一的 `Compute build metadata` 步骤
  （`.github/workflows/release.yml:79-85`、`:133-136`、`:147-150`）；
- 新增 `Set up QEMU`（`.github/workflows/release.yml:116-117`）。

**因此 GHCR 上的公开镜像目前仍是旧的那一份**（tag 只有 `v0.1.0` 与 `latest`，没有
`latest-scratch`；实测 `GET https://ghcr.io/v2/tktogether/subforge/tags/list` 只返回这两个）。
这些改动要等提交并重新发版后才会反映到公开镜像上；在那之前，路线一里的三个限制依然成立。

**注意什么**：`docker-compose.yml` 与两个 Dockerfile 里的 `SUBFORGE_DATA`、`SUBFORGE_CONFIG`
只用于声明路径，`serve` **不读取**这两个变量；非测试代码里读取环境变量的位置只有四处：
`SUBFORGE_DATA_DIR`（`cmd/subforge/doctor.go:62`）、`PATH`（`cmd/subforge/doctor.go:126`）、
`TELEGRAM_CHAT_ID`（`cmd/subforge/scheduler.go:47`）、`SUBFORGE_DEBUG`
（`internal/api/pprof.go:22`）。监听地址、令牌等参数仍以命令行参数与 INI 配置文件为准。

## 4. OpenWrt

打包定义在 `deploy/openwrt/`：`Makefile` 是 SDK 包定义，`files/subforge.init` 是
procd 启动脚本，`files/subforge.conf` 是 UCI 配置。

**前置事实：修复前打不出包。** 原样（未修改）的 `deploy/openwrt/` 在官方
OpenWrt SDK 24.10.4 下执行 `make package/subforge/compile V=s` **失败，退出码 2**：

```
make[3]: *** No targets specified and no makefile found.  Stop.
make[2]: *** [Makefile:49: .../subforge-1.0.0/.built] Error 2
    ERROR: package/subforge failed to build.
```

根因在 SDK 侧：`include/package-defaults.mk:63-69` 的 `Build/Prepare/Default` 被
`ifneq ($(strip $(PKG_UNPACK)),)` 整段包住，而本包没有 `PKG_SOURCE`（`PKG_UNPACK`
为空），于是 `Build/Prepare` 根本没有定义，`$(PKG_BUILD_DIR)` 只是个空目录；
`Package/subforge/install` 里的 `./bin/subforge`、`./files/*` 因此全部找不到，
默认的 `Build/Compile` 也会在空目录里跑 `make` 而报上面的错。修复方式是在
`deploy/openwrt/Makefile` 中显式定义 `Build/Prepare`（把 `bin/`、`files/` 拷进
`$(PKG_BUILD_DIR)`）与空的 `Build/Compile`，并删掉对本包无意义的
`PKG_BUILD_DEPENDS:=golang/host`。

**唯一被实测走通的打包路径**（x86/64 与 aarch64_cortex-a72 两个目标都验证过）：

```sh
# 1) 先交叉编译出待打包的静态二进制（这一步在源码仓库里做）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -mod=vendor -ldflags "-s -w" \
  -o deploy/openwrt/bin/subforge ./cmd/subforge
# 树莓派 4 等 arm64 目标把 GOARCH 换成 arm64

# 2) 解压 SDK（官方 24.10.4，x86/64 目标）
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

产物落在 **`<sdk>/bin/packages/<PKGARCH>/base/`**（不是 `bin/targets/...`）：

| 目标 | 产物 | 字节 | sha256 |
| --- | --- | --- | --- |
| x86/64 | `bin/packages/x86_64/base/subforge_1.0.0-r1_x86_64.ipk` | 15,451,470 | `5ba5d0f87946a21c666678fea50207a0f8be54f07656c9d13b96463bd9488fce` |
| aarch64_cortex-a72 | `bin/packages/aarch64_cortex-a72/base/subforge_1.0.0-r1_aarch64_cortex-a72.ipk` | 14,264,239 | `78050022c2b0c8ca191174468eb613ae8834625233260e7e906381f2392844cc` |

上表两个哈希是**当时那次构建**的实测值。`.ipk` 里装的是预编译二进制，因此哈希随
“用哪个 Go 工具链编那个二进制”而变（同第 1 节查明的 28,672 B 工具链差异）。按上面步骤重打时
哈希对不上属正常，只要 `control`、`data.tar.gz` 内容与包内二进制可运行即可。

**包内二进制比你放进去的那个小 7,264 字节，这是 SDK 的正常行为。** 打包时 SDK 会对安装树跑
`scripts/rstrip.sh`，它对 ELF 可执行文件调用 `staging_dir/host/bin/sstrip -z`，把节头表等
打包后不再需要的数据削掉。实测：源二进制 40,075,426 B（sha256 `35f019c3…`）经
`sstrip -z` 后变成 40,068,162 B（sha256 `a9fbb4f99e77345c…`），与 `.ipk` 里解出来的二进制
**逐字节相同**；同一输入跑两次 `sstrip` 结果一致，所以这个变换是可复现的。被处理过的二进制
`file` 会报 `too large section header offset`（节头表已被削），但仍然可以正常运行。

**改了 `bin/subforge` 之后，直接重跑 `make package/subforge/compile V=s` 就够了**，不需要先 `clean`。
实测：换入一个内容完全不同的二进制后重跑 compile，`.ipk` 的 sha256 立即变化，解包出来的
`usr/bin/subforge` 与换入的文件 sha256 一致（三种起始状态都验过——删掉整个 `build_dir`、
保留 `build_dir`、只删 `.prepared_*` 标记，每次都正确换新）。

唯一需要留意的是**不要手工删 `build_dir` 里的东西**。SDK 把 `.prepared_<hash>` 标记和
`.pkgdir/` 都放在 `$(PKG_BUILD_DIR)` 内部（实测路径
`build_dir/target-x86_64_musl/subforge-1.0.0/.prepared_767b93f7…`）；用 `rm -rf` 手工删掉这个目录
会连带删掉标记，SDK 就会重跑 `Build/Prepare`——本身无害，但这是 SDK 不保证的状态。要清理就用：

```sh
make package/subforge/clean V=s      # 清掉 build_dir 与标记
make package/subforge/compile V=s
```

验证是否真的换了二进制，最直接的办法是解开新包比对（见下面的 `gunzip` 步骤），而不是只看
`.ipk` 的 sha256——只要输入没变，`.ipk` 的 sha256 每次都一样（本页上表那个 x86_64 哈希在
“同一个源二进制 + 同一个工具链”下重打多次完全一致，`clean` 与否都一样）。

**这个 SDK 产出的 `.ipk` 外层是 gzip 包一层 tar，不是 `ar` 归档**（`ar t` 会报
`file format not recognized`），要看内容得先 `gunzip`：

```sh
gunzip -c subforge_1.0.0-r1_x86_64.ipk > pkg.tar
tar -tf pkg.tar        # ./debian-binary  ./data.tar.gz  ./control.tar.gz
tar -xzf data.tar.gz   # ./usr/bin/subforge  ./etc/init.d/subforge  ./etc/config/subforge
```

`control` 与 Makefile 声明逐项一致（`Package: subforge`、`Version: 1.0.0-r1`、
`Depends: libc, ca-bundle`、`Architecture: x86_64`、`License: MIT`、`Section: net`）。
把 `data.tar.gz` 解出来后，x86/64 包里的二进制可以直接运行：`./usr/bin/subforge
version` 退出码 0，`ldd` 报 `not a dynamic executable`。

**尚未验证**：没有在真实 OpenWrt 设备或 rootfs 上执行过 `opkg install`，因此
`postinst`、`procd` 拉起、`ca-bundle` 依赖解析、设备端 `opkg` 是否接受上述 gzip
外层格式都还没有实测。另外 `files/subforge.conf` 声明的 UCI 配置项当前不会被 init
脚本读取（脚本里没有任何 `config_load`），详见 `docs/wiki/安装与部署.md` 第 7.3 节。

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
