package resources

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/stakater/kubectl-tenant/internal/featureflags"
	e "github.com/stakater/kubectl-tenant/pkg/extractors"
	"go.uber.org/zap"
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
	ExtractTenantResourcesFn func(tenantObj *unstructured.Unstructured, logger *zap.Logger) ([]string, error)

	getOptions struct {
		GVR        schema.GroupVersionResource
		GVK        schema.GroupVersionKind
		Namespaced bool
		Extract    ExtractTenantResourcesFn
		Feature    featureflags.Feature
	}
)

func Keys(m map[string]getOptions) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

var (
	SupportedResources = map[string]getOptions{
		"storageclasses": {
			GVR:        schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
			GVK:        schema.GroupVersion{Group: "storage.k8s.io", Version: "v1"}.WithKind("StorageClass"),
			Namespaced: false,
			Extract:    e.ExtractStorageClassNames,
			Feature:    featureflags.FeatureStorageClasses,
		},
		"ingressclasses": {
			GVR:        schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"},
			GVK:        schema.GroupVersion{Group: "networking.k8s.io", Version: "v1"}.WithKind("IngressClass"),
			Namespaced: false,
			Extract:    e.ExtractIngressClassNames,
			Feature:    featureflags.FeatureIngressClasses,
		},
		"priorityclasses": {
			GVR:        schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
			GVK:        schema.GroupVersion{Group: "scheduling.k8s.io", Version: "v1"}.WithKind("PriorityClass"),
			Namespaced: false,
			Extract:    e.ExtractPriorityClassNames,
			Feature:    featureflags.FeaturePodPriority,
		},
		"resourcequotas": {
			GVR:        schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"},
			GVK:        schema.GroupVersion{Group: "", Version: "v1"}.WithKind("ResourceQuota"),
			Namespaced: true,
			Extract:    e.ExtractResourceQuotaNames,
			Feature:    featureflags.FeatureQuota,
		},
	}

	tenantGVR = schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com",
		Version:  "v1beta3",
		Resource: "tenants",
	}
)

func ListResources(
	ctx context.Context,
	cfg *rest.Config,
	tenantNamespace, tenantName, resourceKey string,
	opts getOptions,
	printFlags *get.PrintFlags,
	io genericclioptions.IOStreams,
	logger *zap.Logger,
) error {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	// 1) Read the Tenant object (both spec and status)
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
	names, err := opts.Extract(tenant, logger)
	if err != nil {
		return err
	}
	// dedupe + sort for stable output
	names = e.UniqSorted(names)

	logger.Info("Extracted resource names from tenant",
		zap.String("tenant", tenantName),
		zap.String("resource", resourceKey),
		zap.Strings("names", names),
		zap.Int("count", len(names)))

	// 3) Fetch those objects (cluster- or namespace-scoped as declared in getOptions)
	items := make([]unstructured.Unstructured, 0, len(names))
	for _, name := range names {
		var obj *unstructured.Unstructured
		if opts.Namespaced {
			// For namespaced resources, determine which namespace to use
			ns := tenantNamespace
			if ns == "" {
				// For namespaced resources without explicit namespace,
				// you might want to search across tenant's allowed namespaces
				// For now, refuse to guess
				return errors.New("namespaced resource requested but no namespace provided; pass -n")
			}
			obj, err = dc.Resource(opts.GVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		} else {
			obj, err = dc.Resource(opts.GVR).Get(ctx, name, metav1.GetOptions{})
		}
		if err != nil {
			// Skip missing/forbidden items to keep UX tolerant
			logger.Warn("Resource not found or forbidden, skipping",
				zap.String("name", name),
				zap.Error(err))
			continue
		}
		items = append(items, *obj)
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
