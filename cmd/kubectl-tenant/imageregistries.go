package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var imageRegistriesLong = `List image registries permitted for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.imageRegistries.allowed
3. Displays the list of allowed image registries`

func newListImageRegistriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "imageregistries <tenant-name>",
		Short: "List image registries permitted for a Tenant",
		Long:  imageRegistriesLong,
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
			ff.Enable(featureflags.FeatureImageRegistries)

			if !ff.IsEnabled(featureflags.FeatureImageRegistries) {
				return fmt.Errorf("feature 'imageRegistries' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get image registries
			registries, err := tenantClient.GetTenantImageRegistries(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print registries
			if len(registries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No image registries configured.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Allowed Image Registries:")
			for _, reg := range registries {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", reg)
			}

			return nil
		},
	}

	return cmd
}

func init() {
	ListCmd.AddCommand(newListImageRegistriesCmd())
}
