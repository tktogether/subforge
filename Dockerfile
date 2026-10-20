# syntax=docker/dockerfile:1
#
# SubForge 单二进制容器：两阶段构建，最终镜像基于 scratch，只含静态二进制，
# 不依赖任何运行时（CGO_ENABLED=0、静态链接）。
#
# 构建：
#   docker build -t subforge:latest .
#   docker build --build-arg VERSION=1.0.0 --build-arg COMMIT=$(git rev-parse --short HEAD) -t subforge:latest .
#   docker buildx build --platform linux/amd64,linux/arm64 -t subforge:latest .
# 运行：
#   docker run -d --name subforge -p 25500:25500 -v subforge-data:/data subforge:latest
#   docker run --rm -v subforge-data:/data subforge:latest \
#       convert -file /data/sub.txt -format sing-box -out /data/cfg.json
#
# 开箱即用：下面三个 ENV 是 `serve` 真正读取的环境变量，镜像因此无需额外参数即可
# 从宿主机访问、并默认启用订阅管理（SQLite 落在 /data）。命令行参数优先级更高，
# 例如 `docker run ... subforge:latest serve -listen 127.0.0.1:25500` 会覆盖 SUBFORGE_LISTEN。
#   SUBFORGE_LISTEN    0.0.0.0:25500  容器内必须监听所有网卡，端口映射才通
#   SUBFORGE_DATA_DIR  /data          订阅管理数据库与规则源落盘目录（配合 VOLUME 持久化）
#   SUBFORGE_TOKEN     未设置         需要鉴权时用 -e SUBFORGE_TOKEN=... 传入
#
# 多架构：构建阶段固定在 BUILDPLATFORM 上原生执行，只把目标平台交给 Go 交叉编译
# （GOOS/GOARCH 取自 buildx 注入的 TARGETOS/TARGETARCH），因此多架构构建不需要为编译
# 过程做指令级模拟。运行阶段基于 scratch，不需要执行任何程序，同样不依赖模拟。
#
# 构建实测（2026-10-20，WSL2 Ubuntu 22.04 + Docker 29.1.3 + overlayfs 存储驱动）：
# 本文件与 Dockerfile.alpine 均已成功构建并启动，镜像内二进制经 `file` 与 `ldd` 判定为
# 静态链接（not a dynamic executable），容器内 `version`、`serve`、`convert` 三个子命令
# 全部跑通。注意：scratch 运行层不含根证书，抓 HTTPS 订阅源会直接失败（实测报
# `x509: certificate signed by unknown authority`）；需要 HTTPS 上游请改用 Dockerfile.alpine。

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS builder

# 目标平台由 buildx 注入；直接用 `docker build` 时回落到 linux/amd64。
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# 版本信息，与 Makefile 的 LDFLAGS 保持同一套变量名。
# 注意：这三个变量必须在下面的 go build 里**直接引用**，不能再套一层
# `ARG LDFLAGS=...${VERSION}...`。Docker 在声明 ARG 时就把默认值里的 ${VERSION} 展开成
# 声明处的默认值，`--build-arg VERSION=...` 覆盖不到它，结果是镜像始终自报 dev/none/unknown
# （2026-10-20 实测：本文件传 build-args 后仍报 `subforge dev (commit none, built unknown)`，
# 而 Dockerfile.alpine 因为直接在 RUN 里引用而正常）。
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

# 先拷模块清单与 vendor/，让这一层在源码变动时仍可复用缓存。vendor/ 已全量
# 锁定依赖，构建过程不需要访问网络。
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
RUN go mod verify

COPY . .

# CGO_ENABLED=0：纯静态二进制，可跑在 scratch。-mod=vendor 保证不联网解析。
# GOOS/GOARCH 取自 TARGETOS/TARGETARCH：多架构构建时即为目标平台。
# -X 注入的版本信息直接引用上面的 ARG，见 ARG 处的说明。
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -mod=vendor \
        -ldflags="-s -w \
        -X github.com/tktogether/subforge/internal/buildinfo.Version=${VERSION} \
        -X github.com/tktogether/subforge/internal/buildinfo.Commit=${COMMIT} \
        -X github.com/tktogether/subforge/internal/buildinfo.Date=${DATE}" \
        -o /out/subforge ./cmd/subforge

FROM scratch

COPY --from=builder /out/subforge /usr/local/bin/subforge
COPY LICENSE /LICENSE

# 运行时数据目录：订阅管理数据库（subforge.db）与规则源缓存的落盘位置，挂卷持久化。
# 变量名必须是 SUBFORGE_DATA_DIR —— serve 只认这个；写错名字不会报错，只会静默地
# 退回「未启用订阅管理」，控制台相关接口全部 503。
ENV SUBFORGE_DATA_DIR=/data

# 容器内必须监听所有网卡：只绑 127.0.0.1 的话，端口映射到宿主机后连接会被重置。
ENV SUBFORGE_LISTEN=0.0.0.0:25500

VOLUME ["/data"]

# subconverter 习惯端口，对齐默认配置。
EXPOSE 25500

# 默认起服务；覆盖 CMD 可执行 convert / version / doctor。
ENTRYPOINT ["/usr/local/bin/subforge"]
CMD ["serve"]
