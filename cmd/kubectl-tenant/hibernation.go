package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var hibernationLong = `Display hibernation schedule for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.hibernation configuration
3. Displays sleep and wake schedules (UTC)`

func newHibernationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hibernation <tenant-name>",
		Short: "Display hibernation schedule for a Tenant",
		Long:  hibernationLong,
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
			ff.Enable(featureflags.FeatureHibernation)

			if !ff.IsEnabled(featureflags.FeatureHibernation) {
				return fmt.Errorf("feature 'hibernation' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get hibernation config
			hibernationConfig, err := tenantClient.GetTenantHibernationConfig(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print config
			printHibernationConfig(hibernationConfig, cmd.OutOrStdout())

			return nil
		},
	}

	return cmd
}

func printHibernationConfig(config map[string]interface{}, out io.Writer) {
	if len(config) == 0 {
		fmt.Fprintln(out, "No hibernation configuration defined.")
		return
	}

	// Sleep Schedule
	if sleep, ok := config["sleepSchedule"].(string); ok {
		fmt.Fprintf(out, "Sleep Schedule (UTC): %s\n", sleep)
	}

	// Wake Schedule
	if wake, ok := config["wakeSchedule"].(string); ok {
		fmt.Fprintf(out, "Wake Schedule (UTC): %s\n", wake)
	}
}

func init() {
	rootCmd.AddCommand(newHibernationCmd())
}
