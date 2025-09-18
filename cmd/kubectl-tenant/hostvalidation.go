package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var hostValidationLong = `Display host validation configuration for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.hostValidationConfig configuration
3. Displays allowed hosts, regex pattern, and wildcard policy`

func newHostValidationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host-validation <tenant-name>",
		Short: "Display host validation configuration for a Tenant",
		Long:  hostValidationLong,
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
			ff.Enable(featureflags.FeatureHostValidation)

			if !ff.IsEnabled(featureflags.FeatureHostValidation) {
				return fmt.Errorf("feature 'hostValidation' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get host validation config
			hostValidationConfig, err := tenantClient.GetTenantHostValidationConfig(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print config
			printHostValidationConfig(hostValidationConfig, cmd.OutOrStdout())

			return nil
		},
	}

	return cmd
}

func printHostValidationConfig(config map[string]interface{}, out io.Writer) {
	if len(config) == 0 {
		fmt.Fprintln(out, "No host validation configuration defined.")
		return
	}

	// Allowed Hosts
	if allowed, ok := config["allowed"].([]interface{}); ok && len(allowed) > 0 {
		fmt.Fprintln(out, "Allowed Hosts:")
		for _, host := range allowed {
			if h, ok := host.(string); ok {
				fmt.Fprintf(out, "  - %s\n", h)
			}
		}
	}

	// Allowed Regex
	if regex, ok := config["allowedRegex"].(string); ok {
		fmt.Fprintf(out, "\nAllowed Regex Pattern: %s\n", regex)
	}

	// Deny Wildcards
	if deny, ok := config["denyWildcards"].(bool); ok {
		fmt.Fprintf(out, "\nDeny Wildcards: %v\n", deny)
	}
}

func init() {
	rootCmd.AddCommand(newHostValidationCmd())
}
