package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stakater/kubectl-tenant/internal/config"
	r "github.com/stakater/kubectl-tenant/internal/resources"
	"go.uber.org/zap"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/get"
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
	}

	getCmd := &cobra.Command{
		Use:   "get TENANT RESOURCE [-n NAMESPACE] [-o json|yaml|wide|name|...]",
		Short: "Get tenant-allowed resources (reads tenant spec/status)",
		Long: `Get resources that are allowed for a specific tenant.

This behaves like 'kubectl get <resource>', but filtered to those resources
that are allowed/available for the specified Tenant CR.

Examples:
  # List storage classes allowed for tenant 'my-tenant'
  kubectl tenant get my-tenant storageclasses

  # Get in JSON format
  kubectl tenant get my-tenant storageclasses -o json

  # List ingress classes for tenant in specific namespace
  kubectl tenant get my-tenant ingressclasses -n tenant-namespace`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]
			resourceKey := strings.ToLower(args[1])
			ns := ""
			if flags.Namespace != nil && *flags.Namespace != "" {
				ns = *flags.Namespace
			}

			opts, ok := r.SupportedResources[resourceKey]
			if !ok {
				return fmt.Errorf("unsupported resource %q; supported: %s", resourceKey, r.Keys(r.SupportedResources))
			}

			cfg, err := flags.ToRESTConfig()
			if err != nil {
				return err
			}

			// Load feature flags
			ff, err := config.LoadOrCreateConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Check if feature is enabled
			if !ff.IsEnabled(opts.Feature) {
				return fmt.Errorf("feature %q is disabled", opts.Feature)
			}

			// Initialize logger
			logger, err := zap.NewProduction()
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer logger.Sync()

			// Use kubectl-style printers
			printFlags := get.NewGetPrintFlags()
			printFlags.AddFlags(cmd)
			_ = printFlags.EnsureWithNamespace()

			return r.ListResources(cmd.Context(), cfg, ns, tenantName, resourceKey, opts, printFlags, io, logger)
		},
	}

	flags.AddFlags(root.PersistentFlags())
	root.AddCommand(getCmd)
	return root
}
