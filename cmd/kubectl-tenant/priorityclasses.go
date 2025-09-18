package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var priorityClassesLong = `List pod priority classes permitted for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.podPriorityClasses.allowed
3. Displays the list of allowed pod priority classes`

func newListPriorityClassesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "priorityclasses <tenant-name>",
		Short: "List pod priority classes permitted for a Tenant",
		Long:  priorityClassesLong,
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
			ff.Enable(featureflags.FeaturePodPriority)

			if !ff.IsEnabled(featureflags.FeaturePodPriority) {
				return fmt.Errorf("feature 'podPriorityClasses' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get priority classes
			priorityClasses, err := tenantClient.GetTenantPodPriorityClasses(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print priority classes
			if len(priorityClasses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No pod priority classes configured.")
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Allowed Pod Priority Classes:")
			for _, pc := range priorityClasses {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", pc)
			}

			return nil
		},
	}

	return cmd
}

func init() {
	ListCmd.AddCommand(newListPriorityClassesCmd())
}
