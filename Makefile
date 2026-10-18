# SubForge Makefile

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
    -X github.com/tktogether/subforge/internal/buildinfo.Version=$(VERSION) \
    -X github.com/tktogether/subforge/internal/buildinfo.Commit=$(COMMIT) \
    -X github.com/tktogether/subforge/internal/buildinfo.Date=$(DATE)

GOFLAGS := -trimpath -mod=vendor -ldflags "$(LDFLAGS)"

COVERAGE_OUT ?= coverage.out

.PHONY: build test vet cross docker release clean \
        web-install web-dev web-typecheck web-test web-coverage web-build \
        coverage coverage-gate condition-gate verify

build:
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/subforge ./cmd/subforge

test:
	go test ./...

vet:
	go vet ./...

cross:
	@for target in linux/amd64 linux/arm64 linux/arm \
	    windows/amd64 darwin/amd64 darwin/arm64 \
	    freebsd/amd64; do \
	    GOOS=$${target%/*} GOARCH=$${target#*/} CGO_ENABLED=0 \
	      go build $(GOFLAGS) -o bin/subforge-$${target//\//-} ./cmd/subforge && \
	    echo "ok: $$target"; \
	done
	@for target in linux/mipsle linux/mips; do \
	    GOOS=$${target%/*} GOARCH=$${target#*/} GOMIPS=softfloat CGO_ENABLED=0 \
	      go build $(GOFLAGS) -o bin/subforge-$${target//\//-}-softfloat ./cmd/subforge && \
	    echo "ok: $$target (softfloat)"; \
	done

docker:
	docker build -t subforge:$(VERSION) .
	docker build -f Dockerfile.alpine -t subforge:alpine-$(VERSION) .

release: cross
	@mkdir -p dist
	@for f in bin/subforge-*; do \
	    tar -czf dist/$$(basename $$f).tar.gz -C bin $$(basename $$f); \
	done
	@echo "Release artifacts in dist/"

clean:
	rm -rf bin/ dist/

# --- Frontend (web/) targets ---

web-install:
	pnpm -C web install --frozen-lockfile

web-dev:
	pnpm -C web run dev

web-typecheck:
	pnpm -C web run typecheck

web-test:
	pnpm -C web run test

web-coverage:
	pnpm -C web run coverage

web-build:
	pnpm -C web run build

# --- Coverage targets ---

coverage:
	go test -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./internal/... -count=1

coverage-gate: coverage
	go run ./internal/tools/coveragegate -profile $(COVERAGE_OUT) -min 80 -min-func 100
	go run ./internal/tools/condgate -engine=profile -profile $(COVERAGE_OUT) -min 80

# 准层（gobco 真条件覆盖）：仅 nightly / workflow_dispatch 跑，不进 verify。
condition-gate: coverage
	go run ./internal/tools/condgate -engine=gobco -packages ./internal/... -min 80

# --- Verify (one-shot local green) ---

# 本机一把过：go build/vet/test + 门禁A + 门禁B快层 + 前端 typecheck/test/build。
verify: build vet test coverage-gate web-typecheck web-test web-build
	@echo "verify: all green"
