.PHONY: build clean run test help install appstore rotato-all

# Binary name
BINARY := delivr

# Build directory
BUILD_DIR := build

# Version (can be overridden)
VERSION ?= 1.0.0

# Go build flags
LDFLAGS := -ldflags "-X github.com/philipparndt/delivr/version.Version=$(VERSION) -X github.com/philipparndt/delivr/version.GitCommit=$(shell git rev-parse --short HEAD) -X github.com/philipparndt/delivr/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

# Directories
ROTATO_TEMPLATES := ../rotato
SCREENSHOTS_LIGHT := ../screenshots/light
SCREENSHOTS_DARK := ../screenshots/dark
ROTATO_OUTPUT := rotato-output

# Marker files for caching
MARKER_DIR := .markers
MARKER_FRONT_LIGHT := $(MARKER_DIR)/front-light.done
MARKER_FRONT_DARK := $(MARKER_DIR)/front-dark.done
MARKER_LEFT_LIGHT := $(MARKER_DIR)/rotated-left-light.done
MARKER_LEFT_DARK := $(MARKER_DIR)/rotated-left-dark.done
MARKER_RIGHT_LIGHT := $(MARKER_DIR)/rotated-right-light.done
MARKER_RIGHT_DARK := $(MARKER_DIR)/rotated-right-dark.done
MARKER_Z_LIGHT := $(MARKER_DIR)/rotated-z-light.done
MARKER_Z_DARK := $(MARKER_DIR)/rotated-z-dark.done

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
# Rotato Generation (with caching)
# ============================================================

$(MARKER_DIR):
	mkdir -p $(MARKER_DIR)

# Front rotation - light theme
$(MARKER_FRONT_LIGHT): $(ROTATO_TEMPLATES)/iphone_front.rotato $(wildcard $(SCREENSHOTS_LIGHT)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato front (light)..."
	mkdir -p $(ROTATO_OUTPUT)/front/light
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_front.rotato --images $(SCREENSHOTS_LIGHT) --output $(ROTATO_OUTPUT)/front/light --verbose
	touch $@

# Front rotation - dark theme
$(MARKER_FRONT_DARK): $(ROTATO_TEMPLATES)/iphone_front.rotato $(wildcard $(SCREENSHOTS_DARK)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato front (dark)..."
	mkdir -p $(ROTATO_OUTPUT)/front/dark
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_front.rotato --images $(SCREENSHOTS_DARK) --output $(ROTATO_OUTPUT)/front/dark --verbose
	touch $@

# Rotated left - light theme
$(MARKER_LEFT_LIGHT): $(ROTATO_TEMPLATES)/iphone_rotated_left.rotato $(wildcard $(SCREENSHOTS_LIGHT)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato rotated-left (light)..."
	mkdir -p $(ROTATO_OUTPUT)/rotated-left/light
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_rotated_left.rotato --images $(SCREENSHOTS_LIGHT) --output $(ROTATO_OUTPUT)/rotated-left/light --verbose
	touch $@

# Rotated left - dark theme
$(MARKER_LEFT_DARK): $(ROTATO_TEMPLATES)/iphone_rotated_left.rotato $(wildcard $(SCREENSHOTS_DARK)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato rotated-left (dark)..."
	mkdir -p $(ROTATO_OUTPUT)/rotated-left/dark
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_rotated_left.rotato --images $(SCREENSHOTS_DARK) --output $(ROTATO_OUTPUT)/rotated-left/dark --verbose
	touch $@

# Rotated right - light theme
$(MARKER_RIGHT_LIGHT): $(ROTATO_TEMPLATES)/iphone_rotated_right.rotato $(wildcard $(SCREENSHOTS_LIGHT)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato rotated-right (light)..."
	mkdir -p $(ROTATO_OUTPUT)/rotated-right/light
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_rotated_right.rotato --images $(SCREENSHOTS_LIGHT) --output $(ROTATO_OUTPUT)/rotated-right/light --verbose
	touch $@

# Rotated right - dark theme
$(MARKER_RIGHT_DARK): $(ROTATO_TEMPLATES)/iphone_rotated_right.rotato $(wildcard $(SCREENSHOTS_DARK)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato rotated-right (dark)..."
	mkdir -p $(ROTATO_OUTPUT)/rotated-right/dark
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_rotated_right.rotato --images $(SCREENSHOTS_DARK) --output $(ROTATO_OUTPUT)/rotated-right/dark --verbose
	touch $@

# Rotated Z - light theme
$(MARKER_Z_LIGHT): $(ROTATO_TEMPLATES)/iphone_rotated_z.rotato $(wildcard $(SCREENSHOTS_LIGHT)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato rotated-z (light)..."
	mkdir -p $(ROTATO_OUTPUT)/rotated-z/light
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_rotated_z.rotato --images $(SCREENSHOTS_LIGHT) --output $(ROTATO_OUTPUT)/rotated-z/light --verbose
	touch $@

# Rotated Z - dark theme
$(MARKER_Z_DARK): $(ROTATO_TEMPLATES)/iphone_rotated_z.rotato $(wildcard $(SCREENSHOTS_DARK)/*.png) | $(MARKER_DIR)
	@echo "Generating Rotato rotated-z (dark)..."
	mkdir -p $(ROTATO_OUTPUT)/rotated-z/dark
	./$(BINARY) rotato --template $(ROTATO_TEMPLATES)/iphone_rotated_z.rotato --images $(SCREENSHOTS_DARK) --output $(ROTATO_OUTPUT)/rotated-z/dark --verbose
	touch $@

## rotato-all: Generate all Rotato images (cached)
rotato-all: build $(MARKER_FRONT_LIGHT) $(MARKER_FRONT_DARK) $(MARKER_LEFT_LIGHT) $(MARKER_RIGHT_DARK) $(MARKER_Z_DARK)

## rotato-clean: Remove Rotato cache markers (forces regeneration)
rotato-clean:
	rm -rf $(MARKER_DIR)

## rotato-full-clean: Remove all Rotato output
rotato-full-clean: rotato-clean
	rm -rf $(ROTATO_OUTPUT)

# ============================================================
# App Store Image Generation
# ============================================================

## appstore: Generate all App Store screenshots (10 per device)
appstore: build rotato-all
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

## run-rotato: Run with Rotato example config (requires Rotato app)
run-rotato: build
	./$(BINARY) --config configs/example-rotato.yaml --output ./rotato-output --verbose

## rotato-batch: Batch process screenshots through Rotato (example)
## Usage: make rotato-batch TEMPLATE=scene.rotato IMAGES=./screenshots OUTPUT=./3d-output
rotato-batch: build
	./$(BINARY) rotato --template "$(TEMPLATE)" --images "$(IMAGES)" --output "$(OUTPUT)" --verbose

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
	rm -rf test-output
	rm -rf output

## clean-all: Remove all generated files including Rotato output
clean-all: clean rotato-full-clean

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
	@echo "  appstore       Generate App Store screenshots (10 per device)"
	@echo "  rotato-all     Generate all Rotato 3D renders (cached)"
	@echo "  build          Build the binary"
	@echo ""
	@echo "Cleaning:"
	@echo "  clean          Remove build artifacts"
	@echo "  rotato-clean   Force Rotato regeneration on next run"
	@echo "  clean-all      Remove everything including Rotato output"
	@echo ""
	@echo "All targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
