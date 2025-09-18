package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Tenant resources in the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger, err := zap.NewProduction()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer logger.Sync()

		ff := featureflags.NewConfig()
		ff.Enable(featureflags.FeatureHibernation)
		ff.Enable(featureflags.FeatureHostValidation)
		ff.Enable(featureflags.FeaturePodPriority)
		ff.Enable(featureflags.FeatureServiceAccounts)
		ff.Enable(featureflags.FeatureImageRegistries)
		ff.Enable(featureflags.FeatureIngressClasses)
		ff.Enable(featureflags.FeatureNamespaces)
		ff.Enable(featureflags.FeatureAccessControl)

		tc, err := client.NewTenantClient(ff, logger)
		if err != nil {
			return err
		}

		tenants, err := tc.ListAllTenants(context.Background())
		if err != nil {
			return err
		}

		if len(tenants) == 0 {
			fmt.Println("No Tenant resources found.")
			return nil
		}

		fmt.Printf("Found %d Tenant(s):\n\n", len(tenants))

		for _, t := range tenants {
			fmt.Printf("Name: %s\n", t.GetName())
			fmt.Printf("Namespace: %s\n", t.GetNamespace())
			fmt.Printf("API Version: %s\n", t.GetAPIVersion())
			fmt.Printf("Kind: %s\n", t.GetKind())

			spec, exists, err := unstructured.NestedMap(t.Object, "spec")
			if err != nil {
				fmt.Printf("⚠️ Error reading spec: %v\n", err)
				continue
			}
			if !exists {
				fmt.Println("⚠️ Spec not found")
				continue
			}

			fmt.Println("Spec:")
			client.PrintTenantSpec(spec, "  ")
			fmt.Println(strings.Repeat("-", 60) + "\n")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
