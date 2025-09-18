
# BINARY_NAME=kubectl-tenant
# OUTPUT_DIR ?= $(shell pwd)/bin
# LOCALBIN ?= $(OUTPUT_DIR)
# GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
# GOLANGCI_LINT_VERSION ?= v1.64.8

# # go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# # $1 - target path with name of binary (ideally with version)
# # $2 - package url which can be installed
# # $3 - specific version of package
# define go-install-tool
# @[ -f "$(1)-$(3)" ] || { \
# set -e; \
# package=$(2)@$(3) ;\
# echo "Downloading $${package}" ;\
# rm -f $(1) || true ;\
# GOBIN=$(LOCALBIN) go install $${package} ;\
# mv $(1) $(1)-$(3) ;\
# } ;\
# ln -sf $(1)-$(3) $(1)
# endef

# $(LOCALBIN):
# 	mkdir -p $(LOCALBIN)

# .PHONY: golangci-lint
# golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.

# $(GOLANGCI_LINT): $(LOCALBIN)
# 	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# .PHONY: lint
# lint: golangci-lint ## Run golangci-lint linter
# 	$(GOLANGCI_LINT) run

# .PHONY: test
# test:
# 	go test ./...


# build: $(OUTPUT_DIR)
# 	go build -o $(OUTPUT_DIR)/$(BINARY_NAME) main.go

# clean:
# 	rm -f $(OUTPUT_DIR)/$(BINARY_NAME)


# Makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.1-dev")
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X 'main.Version=$(VERSION)' \
	-X 'main.BuildDate=$(BUILD_DATE)' \
	-X 'main.GitCommit=$(GIT_COMMIT)'

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/kubectl-tenant ./cmd/kubectl-tenant

.PHONY: test
test:
	go test -v ./test/unit/...

.PHONY: run
run: build
	./bin/kubectl-tenant

.PHONY: clean
clean:
	rm -rf bin/