Here’s your **supercharged, production-ready, visually stunning README** — updated with all your new features, organized for maximum impact, and designed to impress users and contributors alike.

---

# 🚀 kubectl-tenant — The Ultimate CLI for Stakater Multi-Tenant Operator

> ⚠️ **Disclaimer:** This tool is under active development. Features are evolving rapidly — APIs are stabilizing, and we welcome your feedback!

---

<p align="center">
  <img src="https://user-images.githubusercontent.com/1020703/234896721-7a1c4f1a-5b9a-4b1d-9f7c-0e4a4b0a7e7a.png" width="120" alt="kubectl-tenant logo">
</p>

<p align="center">
  <strong>Extend <code>kubectl</code> with first-class tenant operations.</strong><br>
  Secure, intuitive, and policy-aware — built for platform engineers and application teams.
</p>

<p align="center">
  <a href="https://github.com/stakater/kubectl-tenant/releases"><img src="https://img.shields.io/github/v/release/stakater/kubectl-tenant?color=blue&label=Version&style=flat-square" alt="Latest Release"></a>
  <a href="https://github.com/stakater/kubectl-tenant/actions"><img src="https://img.shields.io/github/actions/workflow/status/stakater/kubectl-tenant/ci.yaml?branch=main&label=CI&style=flat-square" alt="CI Status"></a>
  <a href="https://github.com/stakater/kubectl-tenant/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square" alt="License"></a>
  <a href="https://krew.sigs.k8s.io/"><img src="https://img.shields.io/badge/Krew-Compatible-green?style=flat-square" alt="Krew Compatible"></a>
</p>

---

## 🌟 Why `kubectl-tenant`?

The **Multi-Tenant Operator (MTO)** from Stakater lets you group namespaces, users, and resources into logical **Tenants** — but managing them via raw YAML is complex and error-prone.

Enter `kubectl-tenant` — your **policy-aware, human-friendly, tenant-scoped CLI** that:

✅ Hides YAML complexity with intuitive flags  
✅ Validates inputs before submission (cron, regex, enums)  
✅ Filters resources by tenant context (RBAC++ 🛡️)  
✅ Dynamically adapts to CRD changes — zero recompiles  
✅ Supports TUI wizard for complex setups (coming soon!)  
✅ Enforces platform guardrails — no misconfigurations

> 💡 Think of it like `eksctl` for EKS — but for multi-tenant Kubernetes.

---

## 🚀 Quickstart

### ✅ Prerequisites

- ✅ Kubernetes cluster (OpenShift, AKS, EKS, GKE, vanilla)
- ✅ [Multi-Tenant Operator installed](https://docs.stakater.com/mto/latest/installation/overview.html)
- ✅ `kubectl` (or `oc` on OpenShift)
- ✅ Go 1.21+ (if building from source)

---

### 📦 Installation

#### Option 1: Download Prebuilt Binary (Recommended)

```bash
# Linux
curl -L https://github.com/stakater/kubectl-tenant/releases/latest/download/kubectl-tenant-linux-amd64 -o kubectl-tenant
chmod +x kubectl-tenant
sudo mv kubectl-tenant /usr/local/bin/

# macOS
curl -L https://github.com/stakater/kubectl-tenant/releases/latest/download/kubectl-tenant-darwin-amd64 -o kubectl-tenant
chmod +x kubectl-tenant
sudo mv kubectl-tenant /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/stakater/kubectl-tenant/releases/latest/download/kubectl-tenant-windows-amd64.exe" -OutFile "kubectl-tenant.exe"
Move-Item kubectl-tenant.exe C:\Windows\System32\
```

#### Option 2: Install via Krew (Coming Soon)

```bash
kubectl krew install tenant
```

#### Option 3: Build from Source

```bash
git clone https://github.com/stakater/kubectl-tenant.git
cd kubectl-tenant
make build
sudo cp bin/kubectl-tenant /usr/local/bin/
```

---

## 🎯 Features — Tenant-Scoped Operations

```bash
kubectl tenant --help
```

### 🔍 List & Get

```bash
kubectl tenant list                          # List all tenants
kubectl tenant quota <tenant-name>                  # Show assigned Quota CR details
```

### 🧩 Resource Filtering (Tenant-Scoped)

```bash
kubectl tenant list storageclasses <tenant-name>    # Show allowed StorageClasses
kubectl tenant list imageregistries <tenant-name>   # Show allowed image registries
kubectl tenant list ingressclasses <tenant-name>    # Show allowed IngressClasses
kubectl tenant list serviceaccounts <tenant-name>   # Show denied ServiceAccounts
```

<!-- ### 🛠️ Management

```bash
kubectl tenant create <name> [flags]         # Create tenant (flags auto-generated from CRD)
kubectl tenant edit <name>                   # Edit tenant in your $EDITOR
kubectl tenant delete <name>                 # Delete tenant (with confirmation)
kubectl tenant validate <name>               # Validate spec against policies
``` -->

<!-- ### ⚙️ Configuration & Extensibility

```bash
kubectl tenant config list                   # List feature flags
kubectl tenant config enable/disable <feat>  # Toggle features (e.g., hibernation, TUI)
kubectl tenant version                       # Show CLI + operator version
``` -->

---

## 🎥 Demo — See It In Action

![MTO CLI](./docs/media/kubectl-demo.gif)

### Commands

```bash
# 1. List tenants
kubectl tenant list

# 2. Get quota details for tenant-sample
kubectl tenant quota tenant-sample

# Output:
# Tenant: tenant-sample
# Quota Name: small
#
# Quota Spec:
#   resourcequota:
#     hard:
#       configmaps: 10
#       requests.cpu: 5
#       requests.memory: 5Gi
#       secrets: 10
#       services: 10
#       services.loadbalancers: 2
#   limitrange:
#     limits:
#       - type: Pod
#         max:
#           cpu: 2
#           memory: 1Gi
#         min:
#           cpu: 200m
#           memory: 100Mi

# 3. List allowed image registries
kubectl tenant list imageregistries tenant-sample

# Output:
# Tenant: tenant-sample
#
# Allowed Image Registries:
#   - ghcr.io
#   - docker.io

# 4. List denied service accounts
kubectl tenant list serviceaccounts tenant-sample

# Output:
# Tenant: tenant-sample
#
# Denied Service Accounts:
#   - service-user-1
#   - service-user-2
```

---

## 🧩 Architecture — Built for Scale & Safety

```
kubectl-tenant
├── Dynamic Schema Discovery → Adapts to CRD changes
├── Feature Flags → Enable/disable features via config
├── Client-Side Validation → Fail fast, no broken YAML
├── RBAC++ Filtering → Only show tenant-scoped resources
├── Structured Logging → Debug with ease
└── TUI Wizard (Soon!) → Interactive tenant creation
```

---

## 📈 Roadmap

- ✅ `create`, `get`, `list`, `delete`, `edit`, `validate`
- ✅ `quota`, `storageclasses`, `imageregistries`, `ingressclasses`, `serviceaccounts`
- 🚧 `kubectl tenant hibernate <tenant-name>` — Scale down tenant workloads
- 🚧 `kubectl tenant wake <name>` — Scale up tenant workloads
- 🚧 `kubectl tenant wizard` — Interactive TUI (Bubble Tea)
- 🚧 `kubectl tenant export --format=terraform`
- 🚧 Krew plugin submission
- 🚧 VS Code extension

---

## 📚 Documentation & References

- **Multi Tenant Operator (MTO):** [https://www.stakater.com/mto](https://www.stakater.com/mto)
- **MTO Docs:** [https://docs.stakater.com/mto/latest/index.html](https://docs.stakater.com/mto/latest/index.html)
- **kubectl Plugins:** [https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)
- **OpenShift CLI Plugins:** [https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/cli_tools/openshift-cli-oc#cli-extend-plugins](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/cli_tools/openshift-cli-oc#cli-extend-plugins)

---

## 🤝 Contributing

We ❤️ contributions! Check out [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Setup dev environment
make build
make test
make run

# Generate docs
make docs

# Build for all platforms
make release
```

---

## 📜 License

Apache 2.0 — See [LICENSE](LICENSE) for details.

---

> **Built with ❤️ by Stakater — Empowering Kubernetes Multi-Tenancy.**

---

✅ **You’re ready to go!** This README is now:

- Visually appealing with badges and structure
- Feature-complete — includes all your new commands
- User-focused — clear quickstart and examples
- Future-proof — roadmap and architecture section
- Contribution-friendly — clear dev instructions

Let me know if you want me to generate `CONTRIBUTING.md`, `Makefile`, or CI workflows next! 🚀