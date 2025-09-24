package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/get"

	r "github.com/stakater/kubectl-tenant/internal/resources"
)

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	flags := genericclioptions.NewConfigFlags(true)
	io := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	root := &cobra.Command{
		Use:   "kubectl-tenant",
		Short: "CLI for managing Tenant CRs from Stakater Tenant Operator",
		Long: `kubectl-tenant is a kubectl plugin for interacting with Tenant Custom Resources
from the Stakater Tenant Operator.

It provides commands to list and inspect resources that are allowed/available
for specific tenants, with output formats compatible with kubectl.`,
		Example: `  # List storage classes allowed for a tenant
  kubectl tenant get my-tenant storageclasses

  # Get resources in JSON format  
  kubectl tenant get my-tenant storageclasses -o json

  # List namespaces for a tenant
  kubectl tenant get my-tenant namespaces

  # List all resources for a tenant
  kubectl tenant get my-tenant all`,
	}

	// Create print flags that will be shared across commands
	printFlags := get.NewGetPrintFlags()

	getCmd := &cobra.Command{
		Use:   "get TENANT RESOURCE [-n NAMESPACE] [-o json|yaml|wide|name|...]",
		Short: "Get tenant-allowed resources (reads tenant spec/status)",
		Long: `Get resources that are allowed for a specific tenant.

This behaves like 'kubectl get <resource>', but filtered to those resources
that are allowed/available for the specified Tenant CR.

Supported resource types:
- Cluster-scoped: storageclasses, ingressclasses, priorityclasses, clusterroles
- Namespaced: resourcequotas, namespaces, serviceaccounts, roles, rolebindings
- Special: all (shows all supported resources)`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]
			resourceKey := strings.ToLower(args[1])
			ns := ""
			if flags.Namespace != nil && *flags.Namespace != "" {
				ns = *flags.Namespace
			}

			// Special case: "all" shows all resources
			if resourceKey == "all" {
				return r.ListAllResources(cmd.Context(), flags, ns, tenantName, io)
			}

			// Try cluster resources first, then namespaced
			var opts r.GetOptions
			var ok bool
			if opts, ok = r.ClusterResources[resourceKey]; !ok {
				if opts, ok = r.NamespacedResources[resourceKey]; !ok {
					allResources := append(r.Keys(r.ClusterResources), r.Keys(r.NamespacedResources)...)
					return fmt.Errorf("unsupported resource %q; supported: %s, all", resourceKey, strings.Join(allResources, ", "))
				}
			}

			cfg, err := flags.ToRESTConfig()
			if err != nil {
				return err
			}

			return r.ListResources(cmd.Context(), cfg, ns, tenantName, resourceKey, opts, printFlags, io)
		},
	}

	// Add flags to commands
	flags.AddFlags(root.PersistentFlags())
	printFlags.AddFlags(getCmd)
	printFlags.EnsureWithNamespace()

	root.AddCommand(getCmd)
	return root
}
