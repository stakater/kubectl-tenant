package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
)

const (
	PluginName = "kubectl-tenant"
)

type getOptions struct {
	resource               schema.GroupVersionResource
	listKind               string
	extractTenantResources func(*unstructured.Unstructured) []string
}

var ClusterResources = map[string]getOptions{
	"storageclasses": {
		resource: schema.GroupVersionResource{
			Group:    "storage.k8s.io",
			Version:  "v1",
			Resource: "storageclasses",
		},
		listKind:               "StorageClassList",
		extractTenantResources: extractStorageClassNames,
	},
	"namespaces": {
		resource: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "namespaces",
		},
		listKind:               "NamespaceList",
		extractTenantResources: extractNamespaceNames,
	},
}

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
		Long: `kubectl-tenant extends kubectl with tenant-scoped resource operations.

It works with Stakater's Multi Tenant Operator to provide filtered views of 
cluster-scoped resources based on tenant permissions.`,
	}

	getCmd := newGetCmd(flags, ioStreams)
	docsCmd := newDocsCmd(root)

	flags.AddFlags(root.PersistentFlags())
	root.AddCommand(getCmd)
	root.AddCommand(docsCmd)
	return root
}

func newGetCmd(configFlags *genericclioptions.ConfigFlags, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get tenant-scoped resources",
		Long: `Get cluster-scoped Kubernetes resources filtered by tenant permissions.

This behaves like 'kubectl get <resource>', but filtered to those resources
permitted for the specified tenant according to the Tenant CR status.`,
	}

	for resourceName, opts := range ClusterResources {
		cmd.AddCommand(newGetResourceCmd(resourceName, opts, configFlags, ioStreams))
	}

	return cmd
}

func newDocsCmd(root *cobra.Command) *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate documentation for kubectl-tenant",
		Long:   `Generate Markdown documentation for all kubectl-tenant commands.`,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			if err := doc.GenMarkdownTree(root, outputDir); err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}

			absPath, _ := filepath.Abs(outputDir)
			fmt.Printf("Documentation generated successfully in: %s\n", absPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "./docs", "Output directory for generated documentation")

	return cmd
}

func newGetResourceCmd(resourceName string, opts getOptions, configFlags *genericclioptions.ConfigFlags,
	ioStreams genericiooptions.IOStreams) *cobra.Command {
	printFlags := get.NewGetPrintFlags()

	cmd := &cobra.Command{
		Use:   resourceName + " <tenant> [resource-name]",
		Short: fmt.Sprintf("List %s permitted for a Tenant", resourceName),
		Long: fmt.Sprintf(`List %s permitted for a Tenant.

This behaves like 'kubectl get %s', but filtered to those listed in
the Tenant CR status (tenant.tenantoperator.stakater.com).

When a specific resource name is provided, the command validates tenant access
and passes through to kubectl for native output.`, resourceName, resourceName),
		Example: fmt.Sprintf(`  # List %s for my-tenant
  kubectl tenant get %s my-tenant

  # Get a specific %s
  kubectl tenant get %s my-tenant specific-resource`,
			resourceName, resourceName, resourceName, resourceName),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]

			cfg, err := configFlags.ToRESTConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// If a specific resource name is provided, validate and get it
			if len(args) > 1 {
				resourceToGet := args[1]
				return handleSpecificResource(ctx, cfg, tenantName, resourceName, resourceToGet, opts, printFlags, ioStreams)
			}

			return listResources(ctx, cfg, tenantName, opts, printFlags, ioStreams)
		},
	}

	printFlags.AddFlags(cmd)
	return cmd
}

func handleSpecificResource(
	ctx context.Context,
	cfg *rest.Config,
	tenantName string,
	resourceType string,
	resourceName string,
	opts getOptions,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	tenantGVR := schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com",
		Version:  "v1beta3",
		Resource: "tenants",
	}

	tenant, err := dyn.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	allowedResources := opts.extractTenantResources(tenant)

	allowed := false
	for _, name := range allowedResources {
		if name == resourceName {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("%s %q is not permitted for tenant %q", resourceType, resourceName, tenantName)
	}

	obj, err := dyn.Resource(opts.resource).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}
	return p.PrintObj(obj, ioStreams.Out)
}

func listResources(
	ctx context.Context,
	cfg *rest.Config,
	tenantName string,
	opts getOptions,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	// dynamic client to read the Tenant (unstructured)
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	tenantGVR := schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com", // CRD group
		Version:  "v1beta3",                     // adjust if your CRD version differs
		Resource: "tenants",
	}

	var tenant *unstructured.Unstructured
	tenant, err = dyn.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	names := opts.extractTenantResources(tenant)
	if len(names) == 0 {
		return printResourceList(opts, []*unstructured.Unstructured{}, printFlags, ioStreams)
	}

	items := make([]*unstructured.Unstructured, 0, len(names))
	for _, name := range names {
		obj, err := dyn.Resource(opts.resource).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// If a name listed in the Tenant doesn't exist, skip it but keep going
			continue
		}
		items = append(items, obj)
	}
	// Sort by name to keep stable output (like kubectl)
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})

	return printResourceList(opts, items, printFlags, ioStreams)
}

func extractStorageClassNames(u *unstructured.Unstructured) []string {
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

func extractNamespaceNames(u *unstructured.Unstructured) []string {
	seen := map[string]struct{}{}
	var out []string

	deployedNs, found, err := unstructured.NestedStringSlice(u.Object, "status", "deployedNamespaces")
	if err == nil && found {
		for _, ns := range deployedNs {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				if _, dup := seen[ns]; !dup {
					seen[ns] = struct{}{}
					out = append(out, ns)
				}
			}
		}
	}

	sandboxes, found, err := unstructured.NestedMap(u.Object, "status", "deployedSandboxes")
	if err == nil && found {
		for _, val := range sandboxes {
			ns, ok := val.(string)
			if !ok {
				continue
			}
			ns = strings.TrimSpace(ns)
			if ns != "" {
				if _, dup := seen[ns]; !dup {
					seen[ns] = struct{}{}
					out = append(out, ns)
				}
			}
		}
	}

	sort.Strings(out)
	return out
}

func printResourceList(
	opts getOptions,
	items []*unstructured.Unstructured,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": opts.resource.GroupVersion().String(),
			"kind":       opts.listKind,
		},
	}

	for _, item := range items {
		list.Items = append(list.Items, *item)
	}

	return p.PrintObj(list, ioStreams.Out)
}
