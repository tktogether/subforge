GO      ?= go
BIN_DIR := dist
PKG     := github.com/tktogether/subforge

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(PKG)/internal/buildinfo.Version=$(VERSION) \
	-X $(PKG)/internal/buildinfo.Commit=$(COMMIT) \
	-X $(PKG)/internal/buildinfo.Date=$(DATE)

.PHONY: build test test-race bench vet vendor tidy mihomo-check cross clean

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge ./cmd/subforge

test:
	$(GO) test ./...

# -race 依赖 cgo：Windows 上用户目录含非 ASCII 字符（中文用户名）时 mingw 链接器会因
# 路径编码失败而无法链接；此时请在 CI（Linux runner）执行，或改用 ASCII 路径的 TEMP。
test-race:
	$(GO) test -race ./...

bench:
	$(GO) test -bench . -benchmem -run '^$$' ./internal/...

vet:
	$(GO) vet ./...

# 用 mihomo 实机校验 clash.meta 产物；SUBFORGE_MIHOMO 指向 mihomo 二进制路径。
mihomo-check:
	bash scripts/validate-mihomo.sh

vendor:
	$(GO) mod vendor

tidy:
	$(GO) mod tidy

# 交付矩阵：Linux 服务端 / NAS / 软路由（mipsle、mips 用 softfloat）、Windows 开发机、macOS、FreeBSD。
# 九个目标都已本地实测通过（CGO_ENABLED=0，纯 Go 无 cgo 依赖，交叉编译不碎）。
# mipsle/mips 必须带 GOMIPS=softfloat：硬浮点变体在多数家用路由固件上跑不起来。
cross:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-linux-amd64        ./cmd/subforge
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-linux-arm64        ./cmd/subforge
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm   $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-linux-arm          ./cmd/subforge
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-windows-amd64.exe  ./cmd/subforge
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-darwin-amd64       ./cmd/subforge
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-darwin-arm64       ./cmd/subforge
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-freebsd-amd64      ./cmd/subforge
	CGO_ENABLED=0 GOMIPS=softfloat GOOS=linux GOARCH=mipsle $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-linux-mipsle ./cmd/subforge
	CGO_ENABLED=0 GOMIPS=softfloat GOOS=linux GOARCH=mips   $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/subforge-linux-mips   ./cmd/subforge

clean:
	rm -rf $(BIN_DIR)
