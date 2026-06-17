# mello CLI build automation.

BINARY      := mello
PKG         := github.com/minhlucncc/mello-cli
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X $(PKG)/cmd.Version=$(VERSION) \
	-X $(PKG)/cmd.Commit=$(COMMIT) \
	-X $(PKG)/cmd.Date=$(DATE)
BIN_DIR     := bin

.PHONY: all build install test lint fmt vet tidy clean

all: lint test build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

lint: vet
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)
