.PHONY: build clean run test help install appstore templates templates-clean

# Binary name
BINARY := delivr

# Build directory
BUILD_DIR := build

# Version (can be overridden)
VERSION ?= 1.0.0

# Go build flags
LDFLAGS := -ldflags "-X github.com/philipparndt/delivr/version.Version=$(VERSION) -X github.com/philipparndt/delivr/version.GitCommit=$(shell git rev-parse --short HEAD) -X github.com/philipparndt/delivr/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

# Directories
ROTATO_INPUT := ../rotato
FRAMES_OUTPUT := frames

# Default target
all: build

## build: Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/delivr

## build-all: Build for multiple platforms
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/delivr

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/delivr

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/delivr

## install: Install binary to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/delivr

## run: Run with example config
run: build
	./$(BINARY) --config configs/example.yaml --output ./test-output --verbose

# ============================================================
# Device Template Generation
# ============================================================

## templates: Generate device templates from .rotato files
templates: build
	@echo "Generating device templates from .rotato files..."
	./$(BINARY) rotato --input $(ROTATO_INPUT) --output $(FRAMES_OUTPUT) --verbose

## templates-clean: Remove generated device templates
templates-clean:
	rm -rf $(FRAMES_OUTPUT)

# ============================================================
# App Store Image Generation
# ============================================================

## appstore: Generate all App Store screenshots (10 per device)
appstore: build templates
	@echo "Generating App Store screenshots..."
	mkdir -p output/appstore
	./$(BINARY) --config configs/appstore.yaml --output ./output/appstore --verbose
	@echo ""
	@echo "Generated screenshots in output/appstore/"
	@ls -la output/appstore/

## appstore-clean: Remove App Store output
appstore-clean:
	rm -rf output/appstore

# ============================================================
# Development / Testing
# ============================================================

## run-rotato: Run with Rotato example config (requires templates generated first)
run-rotato: build
	./$(BINARY) --config configs/example-rotato.yaml --output ./rotato-output --verbose

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	rm -rf test-output
	rm -rf output

## clean-all: Remove all generated files including device templates
clean-all: clean templates-clean

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

## fmt: Format code
fmt:
	go fmt ./...

## lint: Run linter
lint:
	go vet ./...

## test: Run tests
test:
	go test -v ./...

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Main targets:"
	@echo "  templates      Generate device templates from .rotato files"
	@echo "  appstore       Generate App Store screenshots (10 per device)"
	@echo "  build          Build the binary"
	@echo ""
	@echo "Cleaning:"
	@echo "  clean          Remove build artifacts"
	@echo "  templates-clean Remove device templates"
	@echo "  clean-all      Remove everything including templates"
	@echo ""
	@echo "All targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
