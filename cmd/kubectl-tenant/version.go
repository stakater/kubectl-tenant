// cmd/kubectl-tenant/version.go
package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// These will be set during build
var (
	Version   = "v0.1.0-dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI and operator version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("CLI Version: %s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		fmt.Printf("Git Commit: %s\n", GitCommit)

		// Try to get operator version from CRD
		if err := printOperatorVersion(); err != nil {
			fmt.Printf("Operator Version: ❌ Not detected (%v)\n", err)
		}

		return nil
	},
}

func printOperatorVersion() error {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// ✅ Use ApiextensionsV1 client — NOT core kubernetes client
	apiextClient, err := apiextclient.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	ctx := context.Background()
	crdClient := apiextClient.ApiextensionsV1().CustomResourceDefinitions()

	// Get Tenant CRD
	crd, err := crdClient.Get(ctx, "tenants.tenantoperator.stakater.com", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get CRD: %w", err)
	}

	// Get version from CRD labels or annotations
	if crd.Labels != nil {
		if version, ok := crd.Labels["app.kubernetes.io/version"]; ok {
			fmt.Printf("Operator Version: %s\n", version)
			return nil
		}
	}

	if crd.Annotations != nil {
		if version, ok := crd.Annotations["operator.version"]; ok {
			fmt.Printf("Operator Version: %s\n", version)
			return nil
		}
	}

	// Fallback: use CRD creation timestamp as indicator
	fmt.Printf("Operator Version: ✅ CRD installed (created: %s)\n", crd.CreationTimestamp.Format("2006-01-02"))
	return nil
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
