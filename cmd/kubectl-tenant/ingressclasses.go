package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var ingressClassesLong = `List ingress classes permitted for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.ingressClasses.allowed
3. Displays the list of allowed ingress classes`

func newListIngressClassesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingressclasses <tenant-name>",
		Short: "List ingress classes permitted for a Tenant",
		Long:  ingressClassesLong,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]

			// Initialize logger
			logger, err := zap.NewProduction()
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer logger.Sync()

			// Load feature flags
			ff := featureflags.NewConfig()
			ff.Enable(featureflags.FeatureIngressClasses)

			if !ff.IsEnabled(featureflags.FeatureIngressClasses) {
				return fmt.Errorf("feature 'ingressClasses' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get ingress classes
			ingressClasses, err := tenantClient.GetTenantIngressClasses(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print ingress classes
			if len(ingressClasses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No ingress classes configured.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Allowed Ingress Classes:")
			for _, ic := range ingressClasses {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", ic)
			}

			return nil
		},
	}

	return cmd
}

func init() {
	ListCmd.AddCommand(newListIngressClassesCmd())
}
