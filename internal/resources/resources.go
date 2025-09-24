package resources

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/stakater/kubectl-tenant/internal/featureflags"
	e "github.com/stakater/kubectl-tenant/pkg/extractors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
)

type (
	ExtractTenantResourcesFn func(tenantObj *unstructured.Unstructured) ([]string, error)

	GetOptions struct {
		GVR        schema.GroupVersionResource
		GVK        schema.GroupVersionKind
		Namespaced bool
		Extract    ExtractTenantResourcesFn
		Feature    featureflags.Feature
	}
)

func Keys(m map[string]GetOptions) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var (
	// ---- Register all cluster-scoped resources you want to support here ----
	ClusterResources = map[string]GetOptions{
		"storageclasses": {
			GVR:        schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
			GVK:        schema.GroupVersion{Group: "storage.k8s.io", Version: "v1"}.WithKind("StorageClass"),
			Namespaced: false,
			Extract:    e.ExtractStorageClassNames,
		},
		"ingressclasses": {
			GVR:        schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"},
			GVK:        schema.GroupVersion{Group: "networking.k8s.io", Version: "v1"}.WithKind("IngressClass"),
			Namespaced: false,
			Extract:    e.ExtractIngressClassNames,
		},
		"priorityclasses": {
			GVR:        schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
			GVK:        schema.GroupVersion{Group: "scheduling.k8s.io", Version: "v1"}.WithKind("PriorityClass"),
			Namespaced: false,
			Extract:    e.ExtractPriorityClassNames,
		},
		"clusterroles": {
			GVR:        schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
			GVK:        schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("ClusterRole"),
			Namespaced: false,
			Extract:    e.ExtractClusterRoleNames,
		},
	}

	// ---- Register all namespaced resources you want to support here ----
	NamespacedResources = map[string]GetOptions{
		"resourcequotas": {
			GVR:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"},
			GVK:        schema.GroupVersion{Group: "", Version: "v1"}.WithKind("ResourceQuota"),
			Namespaced: true,
			Extract:    e.ExtractResourceQuotaNames,
		},
		"namespaces": {
			GVR:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			GVK:        schema.GroupVersion{Group: "", Version: "v1"}.WithKind("Namespace"),
			Namespaced: false, // namespaces themselves are cluster-scoped
			Extract:    e.ExtractNamespaceNames,
		},
		"serviceaccounts": {
			GVR:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
			GVK:        schema.GroupVersion{Group: "", Version: "v1"}.WithKind("ServiceAccount"),
			Namespaced: true,
			Extract:    e.ExtractServiceAccountNames,
		},
		"roles": {
			GVR:        schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
			GVK:        schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("Role"),
			Namespaced: true,
			Extract:    e.ExtractRoleNames,
		},
		"rolebindings": {
			GVR:        schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
			GVK:        schema.GroupVersion{Group: "rbac.authorization.k8s.io", Version: "v1"}.WithKind("RoleBinding"),
			Namespaced: true,
			Extract:    e.ExtractRoleBindingNames,
		},
	}

	tenantGVR = schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com",
		Version:  "v1beta3",
		Resource: "tenants",
	}
)

func ListAllResources(ctx context.Context, flags *genericclioptions.ConfigFlags, tenantNamespace, tenantName string, io genericclioptions.IOStreams) error {
	fmt.Fprintf(io.Out, "Resources for tenant %q:\n\n", tenantName)

	// Show cluster resources
	fmt.Fprintf(io.Out, "CLUSTER-SCOPED RESOURCES:\n")
	for resourceName := range ClusterResources {
		fmt.Fprintf(io.Out, "  %s\n", resourceName)
	}

	// Show namespaced resources
	fmt.Fprintf(io.Out, "\nNAMESPACED RESOURCES:\n")
	for resourceName := range NamespacedResources {
		fmt.Fprintf(io.Out, "  %s\n", resourceName)
	}

	fmt.Fprintf(io.Out, "\nUse 'kubectl tenant get %s <resource-type>' to view specific resources.\n", tenantName)
	return nil
}

func ListResources(
	ctx context.Context,
	cfg *rest.Config,
	tenantNamespace, tenantName, resourceKey string,
	opts GetOptions,
	printFlags *get.PrintFlags,
	io genericclioptions.IOStreams,
) error {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// 1) Read the Tenant object
	var tenant *unstructured.Unstructured
	if tenantNamespace == "" {
		tenant, err = dc.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{})
	} else {
		tenant, err = dc.Resource(tenantGVR).Namespace(tenantNamespace).Get(ctx, tenantName, metav1.GetOptions{})
	}
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	// 2) Extract allowed names for this resource from the Tenant
	names, err := opts.Extract(tenant)
	if err != nil {
		return err
	}
	// dedupe + sort for stable output
	names = e.UniqSorted(names)

	if len(names) == 0 {
		fmt.Fprintf(io.ErrOut, "No %s resources found for tenant %q\n", resourceKey, tenantName)
		return nil
	}

	// 3) Fetch those objects (cluster- or namespace-scoped as declared in GetOptions)
	items := make([]unstructured.Unstructured, 0, len(names))
	for _, name := range names {
		var obj *unstructured.Unstructured
		if opts.Namespaced {
			// If a resource is namespaced, you might need to search across tenant's allowed namespaces
			ns := tenantNamespace
			if ns == "" {
				// For resources like namespaces themselves, we don't need a namespace
				if resourceKey == "namespaces" {
					obj, err = dc.Resource(opts.GVR).Get(ctx, name, metav1.GetOptions{})
				} else {
					return errors.New("namespaced resource requested but no namespace provided; pass -n")
				}
			} else {
				obj, err = dc.Resource(opts.GVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			}
		} else {
			obj, err = dc.Resource(opts.GVR).Get(ctx, name, metav1.GetOptions{})
		}
		if err != nil {
			// Skip missing/forbidden items to keep UX tolerant
			fmt.Fprintf(io.ErrOut, "Warning: %s %q not found or access denied, skipping\n", resourceKey, name)
			continue
		}
		items = append(items, *obj)
	}

	if len(items) == 0 {
		fmt.Fprintf(io.ErrOut, "No accessible %s resources found for tenant %q\n", resourceKey, tenantName)
		return nil
	}

	// 4) Build a list for printing that looks like kubectl get <kind>
	listGVK := schema.GroupVersion{Group: opts.GVK.Group, Version: opts.GVK.Version}.WithKind(opts.GVK.Kind + "List")
	ul := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": listGVK.GroupVersion().String(),
			"kind":       listGVK.Kind,
		},
		Items: items,
	}

	// 5) Print via kubectl's printers (-o json|yaml|wide|name etc.)
	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}
	var obj runtime.Object = ul
	return p.PrintObj(obj, io.Out)
}
