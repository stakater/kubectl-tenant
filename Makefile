# ==============================
# VARIABLES
# ==============================

BINARY_NAME      = kubectl-tenant
OUTPUT_DIR       ?= $(shell pwd)/bin
LOCALBIN         ?= $(OUTPUT_DIR)
INSTALL_DIR      ?= $(HOME)/.local/bin

# Version info (auto-detected)
VERSION          ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
BUILD_DATE       ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT       ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Linting
GOLANGCI_LINT    = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.64.8

# Linker flags for version info
LDFLAGS = -X 'main.Version=$(VERSION)' \
          -X 'main.BuildDate=$(BUILD_DATE)' \
          -X 'main.GitCommit=$(GIT_COMMIT)'

# Platforms for release
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# ==============================
# HELP TARGET
# ==============================

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ==============================
# TOOL INSTALLATION
# ==============================

define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "📥 Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.

$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# ==============================
# BUILD TARGETS
# ==============================

.PHONY: build
build: $(OUTPUT_DIR) ## Build binary for current platform
	go build -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd/kubectl-tenant

.PHONY: build-all
build-all: ## Build binaries for all supported platforms
	@mkdir -p $(OUTPUT_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output="$(OUTPUT_DIR)/$(BINARY_NAME)-$$os-$$arch"; \
		if [ "$$os" = "windows" ]; then output="$$output.exe"; fi; \
		echo "🏗️  Building $$os/$$arch → $$output"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o $$output ./cmd/kubectl-tenant; \
	done

.PHONY: install
install: build ## Install binary to $(INSTALL_DIR)
	mkdir -p $(INSTALL_DIR)
	cp $(OUTPUT_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@echo "✅ Installed to $(INSTALL_DIR)/$(BINARY_NAME)"

# ==============================
# TEST & LINT
# ==============================

.PHONY: test
test: ## Run unit tests
	go test -v ./test/unit/...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: test-lint
test-lint: test lint ## Run tests and linter

.PHONY: test-unit
test-unit: ## Run unit tests
	go test -v ./test/unit/... -cover

.PHONY: test-commands
test-commands: ## Run command tests
	go test -v ./test/unit/commands/... -cover

.PHONY: test-all
test-all: test-unit test-commands ## Run all tests

.PHONY: cover
cover: ## Generate coverage report
	go test -coverprofile=$(LOCALBIN)/coverage.out ./test/unit/...
	go tool cover -html=$(LOCALBIN)/coverage.out -o $(LOCALBIN)/coverage.html
	@echo "Coverage report generated: $(LOCALBIN)/coverage.html"

# ==============================
# CLEAN & RELEASE
# ==============================

.PHONY: clean
clean: ## Remove built binaries
	rm -rf $(OUTPUT_DIR)

.PHONY: release
release: clean build-all ## Build release binaries for all platforms
	@echo "📦 Release built in $(OUTPUT_DIR)"
	@ls -la $(OUTPUT_DIR)/$(BINARY_NAME)*

# ==============================
# DEVELOPMENT
# ==============================

.PHONY: run
run: build ## Build and run
	$(OUTPUT_DIR)/$(BINARY_NAME)

.PHONY: version
version: ## Show version info
	@echo "Version: $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Git Commit: $(GIT_COMMIT)"