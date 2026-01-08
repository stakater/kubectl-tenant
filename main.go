package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
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

type getOptions struct {
	resource               schema.GroupVersionResource
	extractTenantResources func(*unstructured.Unstructured) []string
}

var ClusterResources = map[string]getOptions{
	"storageclasses": {
		resource: schema.GroupVersionResource{
			Group:    "storage.k8s.io",
			Version:  "v1",
			Resource: "storageclasses",
		},
		extractTenantResources: extractStorageClassNames,
	},
	"namespaces": {
		resource: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "namespaces",
		},
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

func init() {
	// Make sure storage API is in the scheme for printers
	_ = storagev1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

func newRootCmd() *cobra.Command {
	flags := genericclioptions.NewConfigFlags(true)
	ioStreams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	root := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant-related helpers for kubectl",
	}

	getCmd := newGetCmd(flags, ioStreams)
	flags.AddFlags(root.PersistentFlags())
	root.AddCommand(getCmd)
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

func newGetResourceCmd(resourceName string, opts getOptions, configFlags *genericclioptions.ConfigFlags,
	ioStreams genericiooptions.IOStreams) *cobra.Command {
	printFlags := get.NewGetPrintFlags()

	cmd := &cobra.Command{
		Use:   resourceName + " <tenant>",
		Short: fmt.Sprintf("List %s permitted for a Tenant", resourceName),
		Long: fmt.Sprintf(`List %s permitted for a Tenant.

This behaves like 'kubectl get %s', but filtered to those listed in
the Tenant CR status (tenant.tenantoperator.stakater.com).`, resourceName, resourceName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]

			cfg, err := configFlags.ToRESTConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			return listResources(ctx, cfg, tenantName, resourceName, opts, printFlags, ioStreams)
		},
	}

	printFlags.AddFlags(cmd)
	return cmd
}

func listResources(
	ctx context.Context,
	cfg *rest.Config,
	tenantName string,
	resourceName string,
	opts getOptions,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	// dynamic client to read the Tenant (unstructured)
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// typed client to fetch the resource
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
	tenant, err = dyn.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	names := opts.extractTenantResources(tenant)
	if len(names) == 0 {
		return printResourceList(resourceName, []runtime.Object{}, printFlags, ioStreams)
	}

	items := make([]runtime.Object, 0, len(names))
	for _, name := range names {
		obj, err := fetchResources(ctx, kc, opts.resource, name)
		if err != nil {
			// If a name listed in the Tenant doesn't exist, skip it but keep going
			continue
		}
		items = append(items, obj)
	}
	// Sort by name to keep stable output (like kubectl)
	sort.Slice(items, func(i, j int) bool {
		return getObjectName(items[i]) < getObjectName(items[j])
	})
	return printResourceList(resourceName, items, printFlags, ioStreams)
}

func fetchResources(
	ctx context.Context,
	kc *kubernetes.Clientset,
	gvr schema.GroupVersionResource,
	name string) (runtime.Object, error) {

	switch gvr.Resource {
	case "storageclasses":
		return kc.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	case "namespaces":
		return kc.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", gvr.Resource)
	}
}

func getObjectName(obj runtime.Object) string {
	switch v := obj.(type) {
	case *storagev1.StorageClass:
		return v.Name
	case *corev1.Namespace:
		return v.Name
	default:
		panic(fmt.Sprintf("unsupported type: %T", obj))
	}
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
	resourceName string,
	items []runtime.Object,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	// Create the appropriate list object based on resource type
	var listObj runtime.Object
	switch resourceName {
	case "storageclasses":
		scList := &storagev1.StorageClassList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "storage.k8s.io/v1",
				Kind:       "StorageClassList",
			},
		}
		for _, item := range items {
			if sc, ok := item.(*storagev1.StorageClass); ok {
				scList.Items = append(scList.Items, *sc)
			}
		}
		listObj = scList

	case "namespaces":
		nsList := &corev1.NamespaceList{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "NamespaceList",
			},
		}
		for _, item := range items {
			if ns, ok := item.(*corev1.Namespace); ok {
				nsList.Items = append(nsList.Items, *ns)
			}
		}
		listObj = nsList

	default:
		return fmt.Errorf("unsupported resource type for printing: %s", resourceName)
	}

	return p.PrintObj(listObj, ioStreams.Out)
}
