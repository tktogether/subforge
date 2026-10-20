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
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG LDFLAGS=-s -w \
    -X github.com/tktogether/subforge/internal/buildinfo.Version=${VERSION} \
    -X github.com/tktogether/subforge/internal/buildinfo.Commit=${COMMIT} \
    -X github.com/tktogether/subforge/internal/buildinfo.Date=${DATE}

WORKDIR /src

# 先拷模块清单与 vendor/，让这一层在源码变动时仍可复用缓存。vendor/ 已全量
# 锁定依赖，构建过程不需要访问网络。
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
RUN go mod verify

COPY . .

# CGO_ENABLED=0：纯静态二进制，可跑在 scratch。-mod=vendor 保证不联网解析。
# GOOS/GOARCH 取自 TARGETOS/TARGETARCH：多架构构建时即为目标平台。
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -mod=vendor \
        -ldflags "${LDFLAGS}" -o /out/subforge ./cmd/subforge

FROM scratch

COPY --from=builder /out/subforge /usr/local/bin/subforge
COPY LICENSE /LICENSE

# 运行时数据目录（HTTP 缓存等），挂卷持久化。
ENV SUBFORGE_DATA=/data
VOLUME ["/data"]

# subconverter 习惯端口，对齐默认配置。
EXPOSE 25500

# 默认起服务；覆盖 CMD 可执行 convert / version / doctor。
ENTRYPOINT ["/usr/local/bin/subforge"]
CMD ["serve"]
