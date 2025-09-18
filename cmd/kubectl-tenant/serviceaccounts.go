package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var serviceAccountsLong = `List service accounts denied for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.serviceAccounts.denied
3. Displays the list of denied service accounts`

func newListServiceAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serviceaccounts <tenant-name>",
		Short: "List service accounts denied for a Tenant",
		Long:  serviceAccountsLong,
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
			ff.Enable(featureflags.FeatureServiceAccounts)

			if !ff.IsEnabled(featureflags.FeatureServiceAccounts) {
				return fmt.Errorf("feature 'serviceAccounts' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get denied service accounts
			serviceAccounts, err := tenantClient.GetTenantServiceAccountsDenied(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print service accounts
			if len(serviceAccounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No service accounts denied.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Denied Service Accounts:")
			for _, sa := range serviceAccounts {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", sa)
			}

			return nil
		},
	}

	return cmd
}

func init() {
	ListCmd.AddCommand(newListServiceAccountsCmd())
}
