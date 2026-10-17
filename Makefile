# SubForge Makefile

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
    -X github.com/tktogether/subforge/internal/buildinfo.Version=$(VERSION) \
    -X github.com/tktogether/subforge/internal/buildinfo.Commit=$(COMMIT) \
    -X github.com/tktogether/subforge/internal/buildinfo.Date=$(DATE)

GOFLAGS := -trimpath -mod=vendor -ldflags "$(LDFLAGS)"

.PHONY: build test vet cross docker release clean

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
