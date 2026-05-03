# OutBalancer · Makefile
# Common development commands

BIN     := outbalancer
PKG     := ./cmd/outbalancer
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build run demo clean test fmt vet release shots help

all: build

help:
	@echo "OutBalancer Makefile"
	@echo ""
	@echo "  make build       Build the binary for current platform"
	@echo "  make run         Run with default settings"
	@echo "  make demo        Run with --demo flag (10 sample servers)"
	@echo "  make clean       Remove build artifacts"
	@echo "  make test        Run tests"
	@echo "  make fmt         Format code"
	@echo "  make vet         Run go vet"
	@echo "  make shots       Regenerate README screenshots"
	@echo "  make release     Build for all platforms (linux/darwin/windows)"
	@echo ""

build:
	@echo "==> Building $(BIN) ($(VERSION))"
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)
	@ls -lh $(BIN)

run: build
	./$(BIN)

demo: build
	./$(BIN) --demo

clean:
	@rm -rf $(BIN) $(BIN)-* dist/ build/

test:
	go test -v -race ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

shots:
	go run scripts/genshots/main.go docs/images/

release: clean
	@mkdir -p dist
	@for target in linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64 windows/amd64; do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "==> Building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath \
			-ldflags="$(LDFLAGS)" \
			-o "dist/$(BIN)-$$os-$$arch$$ext" \
			$(PKG); \
	done
	@echo ""
	@ls -lh dist/
