# syntax=docker/dockerfile:1
#
# SubForge 单二进制容器：两阶段构建，最终镜像基于 scratch，只含静态二进制，
# 不依赖任何运行时（CGO_ENABLED=0、静态链接）。
#
# 构建：
#   docker build -t subforge:latest .
#   docker build --build-arg VERSION=1.0.0 --build-arg COMMIT=$(git rev-parse --short HEAD) -t subforge:latest .
#   docker build --platform linux/arm64 -t subforge:arm64 .
# 运行：
#   docker run -d --name subforge -p 25500:25500 -v subforge-data:/data subforge:latest
#   docker run --rm -v subforge-data:/data subforge:latest \
#       convert -file /data/sub.txt -format sing-box -out /data/cfg.json
#
# 环境限制：本仓库的两台可用机器都打不出镜像——本机未安装 Docker，已用的远端
# （Ubuntu 22.04）内核没有 overlayfs 挂载点也没有 overlay 模块，dockerd 起不来。
# 因此本文件的构建配方由等价命令实测验证（同一份 ldflags、同一 set of flags 在
# linux/amd64 与 linux/arm64 上 `go build` 成功，产物 `file` 判定为静态链接、
# `ldd` 报 not a dynamic executable），但镜像层本身尚未在任何机器上构建过。
# 拿到可用 Docker 后需补一次 `docker build` 实测。

FROM golang:1.26-bookworm AS builder

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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -mod=vendor \
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
