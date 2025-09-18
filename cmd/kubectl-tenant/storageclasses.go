package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/get"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

var (
	storageClassesLong = `List StorageClasses permitted for a Tenant.

This behaves like 'kubectl get storageclasses', but filtered to those listed in
.status.storageClasses.available[].name of the Tenant CR
(tenant.tenantoperator.stakater.com).`
)

func newListStorageClassesCmd() *cobra.Command {
	printFlags := get.NewGetPrintFlags()

	cmd := &cobra.Command{
		Use:   "storageclasses <tenant-name>",
		Short: "List StorageClasses permitted for a Tenant",
		Long:  storageClassesLong,
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
			ff.Enable(featureflags.FeatureStorageClasses) // for demo — load from config later

			if !ff.IsEnabled(featureflags.FeatureStorageClasses) {
				return fmt.Errorf("feature 'storageClasses' is disabled")
			}

			// Load kubeconfig
			config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
			if err != nil {
				return fmt.Errorf("failed to load kubeconfig: %w", err)
			}

			// Create clients
			tenantClient, err := client.NewTenantClient(ff, logger)
			if err != nil {
				return err
			}

			storageClient, err := client.NewStorageClassClient(config, logger)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Get storage class names from Tenant status
			names, err := tenantClient.GetTenantStatusStorageClasses(ctx, tenantName)
			if err != nil {
				return err
			}

			// Fetch StorageClasses
			scList, err := storageClient.GetStorageClassesByNames(ctx, names)
			if err != nil {
				return err
			}

			// Print
			return printStorageClassList(scList, printFlags, cmd.OutOrStdout())
		},
	}

	printFlags.AddFlags(cmd)
	return cmd
}

func printStorageClassList(scList *storagev1.StorageClassList, printFlags *get.PrintFlags, out io.Writer) error {
	// Register StorageClass in scheme for printers
	_ = storagev1.AddToScheme(scheme.Scheme)

	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	// If using table printer, cast and use it
	// if printer, isTable := p.(*printers.TablePrinter); isTable {
	// 	return printer.PrintObj(scList, out)
	// }

	// Otherwise, use generic printer (json/yaml/name/custom-columns)
	return p.PrintObj(scList, out)
}

func init() {
	ListCmd.AddCommand(newListStorageClassesCmd())
}
