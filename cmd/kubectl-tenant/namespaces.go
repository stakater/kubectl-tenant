package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var namespacesLong = `Display namespace configuration for a Tenant.

This command:
1. Fetches the Tenant CR
2. Extracts .spec.namespaces configuration
3. Displays sandbox settings, prefix rules, and metadata templates`

func newListNamespacesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "namespaces <tenant-name>",
		Short: "Display namespace configuration for a Tenant",
		Long:  namespacesLong,
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
			ff.Enable(featureflags.FeatureNamespaces)

			if !ff.IsEnabled(featureflags.FeatureNamespaces) {
				return fmt.Errorf("feature 'namespaces' is disabled")
			}

			// Create tenant client
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get namespaces config
			namespacesConfig, err := tenantClient.GetTenantNamespacesConfig(ctx, tenantName)
			if err != nil {
				return err
			}

			// Print header
			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintln(cmd.OutOrStdout())

			// Print config
			printNamespacesConfig(namespacesConfig, cmd.OutOrStdout())

			return nil
		},
	}

	return cmd
}

func printNamespacesConfig(config map[string]interface{}, out io.Writer) {
	if len(config) == 0 {
		fmt.Fprintln(out, "No namespace configuration defined.")
		return
	}

	// Sandboxes
	if sandboxes, ok := config["sandboxes"].(map[string]interface{}); ok {
		fmt.Fprintln(out, "Sandboxes:")
		if enabled, ok := sandboxes["enabled"].(bool); ok {
			fmt.Fprintf(out, "  Enabled: %v\n", enabled)
		}
		if private, ok := sandboxes["private"].(bool); ok {
			fmt.Fprintf(out, "  Private: %v\n", private)
		}
	}

	// OnDeletePurge
	if purge, ok := config["onDeletePurgeNamespaces"].(bool); ok {
		fmt.Fprintf(out, "\nOn Delete Purge Namespaces: %v\n", purge)
	}

	// Without Tenant Prefix
	if withoutPrefix, ok := config["withoutTenantPrefix"].([]interface{}); ok && len(withoutPrefix) > 0 {
		fmt.Fprintln(out, "\nNamespaces Without Tenant Prefix:")
		for _, ns := range withoutPrefix {
			if name, ok := ns.(string); ok {
				fmt.Fprintf(out, "  - %s\n", name)
			}
		}
	}

	// With Tenant Prefix
	if withPrefix, ok := config["withTenantPrefix"].([]interface{}); ok && len(withPrefix) > 0 {
		fmt.Fprintln(out, "\nNamespace Prefixes (will be prepended with tenant name):")
		for _, prefix := range withPrefix {
			if p, ok := prefix.(string); ok {
				fmt.Fprintf(out, "  - %s\n", p)
			}
		}
	}

	// Metadata
	if metadata, ok := config["metadata"].(map[string]interface{}); ok {
		fmt.Fprintln(out, "\nMetadata Templates:")

		// Common
		if common, ok := metadata["common"].(map[string]interface{}); ok {
			fmt.Fprintln(out, "  Common:")
			printLabelsAnnotations(common, "    ", out)
		}

		// Sandbox
		if sandbox, ok := metadata["sandbox"].(map[string]interface{}); ok {
			fmt.Fprintln(out, "  Sandbox:")
			printLabelsAnnotations(sandbox, "    ", out)
		}

		// Specific
		if specific, ok := metadata["specific"].([]interface{}); ok && len(specific) > 0 {
			fmt.Fprintln(out, "  Specific:")
			for i, spec := range specific {
				if s, ok := spec.(map[string]interface{}); ok {
					fmt.Fprintf(out, "    #%d:\n", i+1)
					if namespaces, ok := s["namespaces"].([]interface{}); ok {
						fmt.Fprintln(out, "      Namespaces:")
						for _, ns := range namespaces {
							if name, ok := ns.(string); ok {
								fmt.Fprintf(out, "        - %s\n", name)
							}
						}
					}
					printLabelsAnnotations(s, "      ", out)
				}
			}
		}
	}
}

func printLabelsAnnotations(m map[string]interface{}, indent string, out io.Writer) {
	// Labels
	if labels, ok := m["labels"].(map[string]interface{}); ok && len(labels) > 0 {
		fmt.Fprintf(out, "%sLabels:\n", indent)
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := labels[k].(string); ok {
				fmt.Fprintf(out, "%s  %s: %s\n", indent, k, v)
			}
		}
	}

	// Annotations
	if annotations, ok := m["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
		fmt.Fprintf(out, "%sAnnotations:\n", indent)
		keys := make([]string, 0, len(annotations))
		for k := range annotations {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := annotations[k].(string); ok {
				fmt.Fprintf(out, "%s  %s: %s\n", indent, k, v)
			}
		}
	}
}

func init() {
	ListCmd.AddCommand(newListNamespacesCmd())
}
