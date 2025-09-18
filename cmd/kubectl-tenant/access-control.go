package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var accessControlLong = `Display access control configuration for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.accessControl configuration
3. Displays owners, editors, and viewers (users and groups)`

func newAccessControlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access-control <tenant-name>",
		Short: "Display access control configuration for a Tenant",
		Long:  accessControlLong,
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
			ff.Enable(featureflags.FeatureAccessControl)

			if !ff.IsEnabled(featureflags.FeatureAccessControl) {
				return fmt.Errorf("feature 'accessControl' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get access control config
			accessControl, err := tenantClient.GetTenantAccessControl(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print config
			printAccessControl(accessControl, cmd.OutOrStdout())

			return nil
		},
	}

	return cmd
}

func printAccessControl(config map[string]interface{}, out io.Writer) {
	if len(config) == 0 {
		fmt.Fprintln(out, "No access control configuration defined.")
		return
	}

	roles := []string{"owners", "editors", "viewers"}
	for _, role := range roles {
		if roleConfig, ok := config[role].(map[string]interface{}); ok {
			fmt.Fprintf(out, "%s:\n", strings.Title(role))

			// Users
			if users, ok := roleConfig["users"].([]interface{}); ok && len(users) > 0 {
				fmt.Fprintln(out, "  Users:")
				for _, user := range users {
					if u, ok := user.(string); ok {
						fmt.Fprintf(out, "    - %s\n", u)
					}
				}
			}

			// Groups
			if groups, ok := roleConfig["groups"].([]interface{}); ok && len(groups) > 0 {
				fmt.Fprintln(out, "  Groups:")
				for _, group := range groups {
					if g, ok := group.(string); ok {
						fmt.Fprintf(out, "    - %s\n", g)
					}
				}
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(newAccessControlCmd())
}
