package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var quotaLong = `Display the Quota details assigned to a Tenant.

This command:
1. Fetches the Tenant CR and reads .spec.quota (e.g., "small")
2. Fetches the corresponding Quota CR (tenantoperator.stakater.com/v1beta1)
3. Displays its full spec (ResourceQuota and LimitRange settings)`

func newQuotaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota <tenant-name>",
		Short: "Show quota details assigned to a Tenant",
		Long:  quotaLong,
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
			ff.Enable(featureflags.FeatureQuota) // for demo — load from config later

			if !ff.IsEnabled(featureflags.FeatureQuota) {
				return fmt.Errorf("feature 'quota' is disabled")
			}

			// Load kubeconfig
			config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
			if err != nil {
				return fmt.Errorf("failed to load kubeconfig: %w", err)
			}

			// Create dynamic client
			dynClient, err := dynamic.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create dynamic client: %w", err)
			}

			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			quotaClient := client.NewQuotaClient(dynClient, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Step 1: Get quota name from Tenant
			quotaName, err := tenantClient.GetTenantQuotaName(ctx, tenantName)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tenant: %s\n", tenantName)
			fmt.Fprintf(cmd.OutOrStdout(), "Quota Name: %s\n\n", quotaName)

			// Step 2: Fetch Quota CR
			quotaCR, err := quotaClient.GetQuota(ctx, quotaName)
			if err != nil {
				return err
			}

			// Step 3: Print full spec
			spec, found, err := unstructured.NestedMap(quotaCR.Object, "spec")
			if err != nil {
				return fmt.Errorf("error reading quota spec: %w", err)
			}
			if !found {
				return fmt.Errorf("quota %q has no spec", quotaName)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Quota Spec:")
			printQuotaSpec(spec, "  ", cmd.OutOrStdout()) //

			return nil
		},
	}

	return cmd
}

func printQuotaSpec(spec map[string]interface{}, indent string, out io.Writer) {
	for k, v := range spec {
		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Fprintf(out, "%s%s:\n", indent, k)
			printQuotaSpec(val, indent+"  ", out)
		case []interface{}:
			fmt.Fprintf(out, "%s%s:\n", indent, k)
			for _, item := range val {
				if m, ok := item.(map[string]interface{}); ok {
					fmt.Fprintf(out, "%s  -\n", indent)
					printQuotaSpec(m, indent+"    ", out)
				} else {
					fmt.Fprintf(out, "%s  - %v\n", indent, item)
				}
			}
		case string, bool, int, float64:
			fmt.Fprintf(out, "%s%s: %v\n", indent, k, val)
		default:
			fmt.Fprintf(out, "%s%s: %v (type: %T)\n", indent, k, val, val)
		}
	}
}

func init() {
	rootCmd.AddCommand(newQuotaCmd())
}
