# Binaries land in bin/ rather than the repo root, because katra's own katra
# lives in ./katra -- `go build -o katra` would be trying to write over a
# directory. bin/ is gitignored.
BINDIR  := bin
BINARY  := $(BINDIR)/katra
MCP     := $(BINDIR)/katra-mcp
PKG     := ./cmd/katra
MCPPKG  := ./cmd/katra-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all build build-mcp test lint fmt tidy clean release-check snapshot install

all: build build-mcp

build:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

build-mcp:
	@mkdir -p $(BINDIR)
	go build -ldflags "$(LDFLAGS)" -o $(MCP) $(MCPPKG)

# Both binaries onto your PATH via GOBIN. This is what the hub launchd agent
# and every project's git hook resolve, so it is the one that matters locally.
install:
	go install -ldflags "$(LDFLAGS)" $(PKG) $(MCPPKG)

test:
	go test ./...

# Use golangci-lint when it is installed; otherwise fall back to go vet.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint run"; \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; falling back to go vet"; \
		go vet ./...; \
	fi

fmt:
	go fmt ./...

tidy:
	go mod tidy

# Validate .goreleaser.yml without building anything.
release-check:
	HOMEBREW_TAP_GITHUB_TOKEN="$$HOMEBREW_TAP_GITHUB_TOKEN" goreleaser check

# Build the full release locally -- every target, archives, checksums, Homebrew
# cask and the registry-only OCI image -- without publishing or signing. This
# needs Docker Buildx (and QEMU for the arm64 image). Output lands in ./dist.
snapshot:
	HOMEBREW_TAP_GITHUB_TOKEN="$$HOMEBREW_TAP_GITHUB_TOKEN" \
		goreleaser release --snapshot --clean --skip=publish,sign,announce

clean:
	rm -rf $(BINDIR) dist
