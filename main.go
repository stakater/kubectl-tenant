package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/spf13/cobra"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	PluginName = "kubectl-tenant"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		// Match kubectl's non-verbose error reporting style
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	flags := genericclioptions.NewConfigFlags(true)
	ioStreams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	root := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant-related helpers for kubectl",
	}

	// Extensible: add more groups later; for now we only support list storageclasses
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List tenant resources",
	}
	listCmd.AddCommand(newListStorageClassesCmd(flags, ioStreams))

	flags.AddFlags(root.PersistentFlags())
	root.AddCommand(listCmd)
	return root
}

func newListStorageClassesCmd(configFlags *genericclioptions.ConfigFlags,
	ioStreams genericiooptions.IOStreams) *cobra.Command {
	printFlags := get.NewGetPrintFlags()

	cmd := &cobra.Command{
		Use:   "storageclasses",
		Short: "List StorageClasses permitted for a Tenant",
		Long: `List StorageClasses permitted for a Tenant.

This behaves like 'kubectl get storageclasses', but filtered to those listed in
.status.storageClasses[].available[].name of the Tenant CR
(tenant.tenantoperator.stakater.com).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]

			cfg, err := configFlags.ToRESTConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			return runListStorageClasses(ctx, cfg, tenantName, printFlags, ioStreams)
		},
	}

	printFlags.AddFlags(cmd)
	// mimic kubectl defaults (human-readable table if no -o provided)
	// _ = printFlags.EnsureWithNamespace()
	return cmd
}

func runListStorageClasses(
	ctx context.Context,
	cfg *rest.Config,
	tenantName string,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	// dynamic client to read the Tenant (unstructured)
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// typed client to fetch StorageClasses
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	tenantGVR := schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com", // CRD group
		Version:  "v1beta3",                     // adjust if your CRD version differs
		Resource: "tenants",
	}

	var tenant *unstructured.Unstructured
	tenant, err = dyn.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{}, "status")
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	// Extract names from status.storageClass[].available[].name
	names := extractStorageClassNames(tenant)
	if len(names) == 0 {
		// Return an *empty* list to keep behavior close to `kubectl get storageclasses` with zero matches
		return printStorageClassList(
			&storagev1.StorageClassList{Items: []storagev1.StorageClass{}},
			printFlags,
			ioStreams)
	}

	// Fetch those StorageClasses
	scList := &storagev1.StorageClassList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "StorageClassList",
		},
	}
	for _, n := range names {
		sc, err := kc.StorageV1().StorageClasses().Get(ctx, n, metav1.GetOptions{})
		if err != nil {
			// If a name listed in the Tenant doesn't exist, skip it but keep going
			// (You could gate this behind a --strict flag if desired).
			continue
		}
		scList.Items = append(scList.Items, *sc)
	}

	// Sort by name to keep stable output (like kubectl)
	sort.Slice(scList.Items, func(i, j int) bool {
		return scList.Items[i].Name < scList.Items[j].Name
	})

	return printStorageClassList(scList, printFlags, ioStreams)
}

func extractStorageClassNames(u *unstructured.Unstructured) []string {
	// status.storageClass: { available: []{ name: string } }
	scList, found, err := unstructured.NestedSlice(u.Object, "status", "storageClasses", "available")
	if err != nil || !found {
		return nil
	}

	seen := map[string]struct{}{}
	var out []string

	for _, scEntry := range scList {
		sc, ok := scEntry.(map[string]interface{})
		if !ok {
			continue
		}

		var raw any
		if v, ok := sc["name"]; ok {
			raw = v
		}
		n, ok := raw.(string)
		if !ok || strings.TrimSpace(n) == "" {
			continue
		}
		if _, dup := seen[n]; !dup {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func printStorageClassList(scList *storagev1.StorageClassList, printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams) error {
	// Make sure storage API is in the scheme for printers
	_ = storagev1.AddToScheme(scheme.Scheme)

	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	// If human-readable (no -o), use the table printer like kubectl does
	// Otherwise, ToPrinter() already handles json|yaml|name|custom-columns, etc.
	// The cli-runtime TablePrinter needs a runtime.Object and a scheme that knows the type.
	var obj runtime.Object = scList
	return p.PrintObj(obj, ioStreams.Out)
}
