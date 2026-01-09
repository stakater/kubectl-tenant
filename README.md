# kubectl-tenant (Multi Tenant Operator client plugin)

> ⚠️ **Disclaimer:** This tool is under active development. Features are limited, APIs may change, and code may undergo drastic revisions.

The **kubectl-tenant** plugin extends `kubectl` with the `tenant` command group, enabling secure, close-to-native interactions with [Stakater's Multi Tenant Operator](https://www.stakater.com/mto).
It provides tenant-scoped Kubernetes operations to simplify cluster multi-tenancy and improve security by filtering results according to tenant context.

---

## Features

* Adds a `kubectl tenant` subcommand set.
* Current supported resources:
  * Storage Classes `kubectl tenant get storageclasses <tenant>`
  * Namespaces `kubectl tenant get namespaces <tenant>`
* Functions like `kubectl get <resource>` but **filters output for the specified tenant**.
* Ensures tenants can only discover their own resources (storageclasses/namespaces) instead of all resources available in the cluster (limitation of native RBAC on `list`).

---

## Quickstart

### Prerequisites

  * A running cluster with [Multi Tenant Operator](https://docs.stakater.com/mto/latest/installation/overview.html) installed.
* `kubectl` (or `oc` on OpenShift).

### Installation

Download the prebuilt binary for your platform from the [GitHub Releases](https://github.com/stakater/kubectl-tenant/releases), place it somewhere on your system, and make su(storageclasses/namespaces)re that directory is in your `$PATH`.

Example:

```bash
# Download for your OS/Arch
curl -L https://github.com/stakater/kubectl-tenant/releases/download/v0.0.1/kubectl-tenant-linux-amd64 -o kubectl-tenant
chmod +x kubectl-tenant
mv kubectl-tenant ~/.local/bin/   # ensure this path is in your $PATH
```

Alternatively, you can build from source:

```bash
# Clone the repository
git clone https://github.com/stakater/kubectl-tenant.git
cd kubectl-tenant

# Build and install the plugin
go build -o kubectl-tenant
mv kubectl-tenant ~/.local/bin/
```

### Usage

```bash
# Get tenant-scoped resources
kubectl tenant get <resource> <tenant>
```
### Examples

- StorageClasses
  ```bash
  # Get tenant-scoped storageclasses
  kubectl tenant get storageclasses my-tenant
  ```
  Example output:
  
  ```bash
  NAME                  PROVISIONER                    AGE
  my-tenant-sc          kubernetes.io/no-provisioner   5d
  ```
- Namespaces
  ```bash
  # Get tenant-scoped namespaces
  kubectl tenant get namesapces my-tenant
  ```
  Example output:
  
  ```bash
  NAME                        AGE
  my-tenant-prod              5d
  my-tenant-sandbox           7d
  ```

---

## Documentation & References

* **Multi Tenant Operator:** [https://www.stakater.com/multi-tenant-operator](https://www.stakater.com/mto)
* **Tenant Operator Docs:** [https://docs.stakater.com/mto/latest/index.html](https://docs.stakater.com/mto/latest/index.html)
* **kubectl Plugin Mechanism:** [https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)
* **OpenShift `oc` Plugin Docs:** [https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/cli_tools/openshift-cli-oc#cli-extend-plugins](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/cli_tools/openshift-cli-oc#cli-extend-plugins)

---

## Roadmap

* Additional tenant-scoped resources (Namespaces, IngressClasses, etc.).

---

## License

Apache 2.0
