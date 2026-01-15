BINARY_NAME=kubectl-tenant
OUTPUT_DIR ?= $(shell pwd)/bin
LOCALBIN ?= $(OUTPUT_DIR)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.64.8

# E2E config
K3D_CLUSTER_NAME ?= kubectl-tenant-e2e
MTO_NAMESPACE ?= multi-tenant-operator
MTO_CHART_VERSION ?= 1.4.2

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
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

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: build
build: $(OUTPUT_DIR) ## Build the plugin binary
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME) main.go

.PHONY: clean
clean: ## Clean build artifacts
	rm -f $(OUTPUT_DIR)/$(BINARY_NAME)

##@ E2E Testing

.PHONY: e2e
e2e: ## Run e2e tests (requires cluster with MTO)
	cd e2e && go test -tags=e2e -v -timeout=10m .

.PHONY: e2e-setup
e2e-setup: e2e-create-cluster e2e-install-cert-manager e2e-install-mto e2e-create-quota ## Create Kind cluster and install MTO

.PHONY: e2e-create-cluster
e2e-create-cluster: ## Create k3d cluster
	@echo "Creating k3d cluster..."
	k3d cluster create $(K3D_CLUSTER_NAME) --wait
	@echo "Waiting for cluster to be ready..."
	kubectl wait --for=condition=Ready nodes --all --timeout=120s
	kubectl cluster-info

.PHONY: e2e-install-cert-manager
e2e-install-cert-manager: ## Install cert-manager (required by MTO)
	@echo "Installing cert-manager..."
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.5/cert-manager.yaml
	@echo "Waiting for cert-manager namespace and pods..."
	@sleep 10
	kubectl wait --for=condition=Ready pods --all -n cert-manager --timeout=300s
	@echo "Waiting for cert-manager webhook to be ready..."
	kubectl wait --for=condition=Available deployment/cert-manager -n cert-manager --timeout=180s
	kubectl wait --for=condition=Available deployment/cert-manager-webhook -n cert-manager --timeout=180s
	kubectl wait --for=condition=Available deployment/cert-manager-cainjector -n cert-manager --timeout=180s
	@echo "Verifying cert-manager webhook is responding..."
	@sleep 15

.PHONY: e2e-install-mto
e2e-install-mto: ## Install MTO via Helm
	@echo "Installing MTO..."
	helm install tenant-operator oci://ghcr.io/stakater/public/charts/tenant-operator \
		--namespace $(MTO_NAMESPACE) \
		--create-namespace \
		--version $(MTO_CHART_VERSION) \
		--set 'webhook.manager.env.bypassedGroups=system:masters' \
		--wait --timeout 5m
	@echo "Waiting for MTO pods..."
	kubectl wait --for=condition=Ready pods --all -n $(MTO_NAMESPACE) --timeout=180s

.PHONY: e2e-create-quota
e2e-create-quota: ## Create TenantQuota required for tenant creation
	@echo "Creating TenantQuota..."
	kubectl apply -f e2e/testdata/quota.yaml

.PHONY: e2e-cleanup
e2e-cleanup: ## Delete k3d cluster
	@echo "Deleting k3d cluster..."
	k3d cluster delete $(K3D_CLUSTER_NAME)

.PHONY: e2e-full
e2e-full: e2e-setup e2e e2e-cleanup ## Full e2e: setup + test + cleanup

##@ Help

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)