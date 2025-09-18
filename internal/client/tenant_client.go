package client

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

const (
	Group   = "tenantoperator.stakater.com"
	Version = "v1beta3"
	Kind    = "Tenant"
	Plural  = "tenants"
)

type TenantClient struct {
	dynClient    dynamic.Interface
	gvr          schema.GroupVersionResource
	FeatureFlags *featureflags.Config
	Logger       *zap.Logger
	timeout      time.Duration
}

func NewTenantClient(ff *featureflags.Config, logger *zap.Logger) (*TenantClient, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &TenantClient{
		dynClient:    dynClient,
		gvr:          schema.GroupVersionResource{Group: Group, Version: Version, Resource: Plural},
		FeatureFlags: ff,
		Logger:       logger,
		timeout:      30 * time.Second,
	}, nil
}

func (tc *TenantClient) ListAllTenants(ctx context.Context) ([]*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(ctx, tc.timeout)
	defer cancel()

	tenantClient := tc.dynClient.Resource(tc.gvr)

	tenantList, err := tenantClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}

	tc.Logger.Info("Fetched tenants", zap.Int("count", len(tenantList.Items)))

	var filtered []*unstructured.Unstructured
	for i := range tenantList.Items {
		spec, exists, _ := unstructured.NestedMap(tenantList.Items[i].Object, "spec")
		if exists {
			filteredSpec := tc.FilterSpecByFeatureFlags(spec)
			tenantList.Items[i].Object["spec"] = filteredSpec
		}
		filtered = append(filtered, &tenantList.Items[i])
	}

	return filtered, nil
}

func (tc *TenantClient) FilterSpecByFeatureFlags(spec interface{}) interface{} {
	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return spec
	}

	filtered := make(map[string]interface{})

	for key, value := range specMap {
		feature := tc.fieldToFeature(key)
		if feature != "" && !tc.FeatureFlags.IsEnabled(feature) {
			tc.Logger.Debug("Field filtered out by feature flag", zap.String("field", key), zap.String("feature", string(feature)))
			continue
		}
		filtered[key] = value
	}

	return filtered
}

func (tc *TenantClient) fieldToFeature(field string) featureflags.Feature {
	switch field {
	case "hibernation":
		return featureflags.FeatureHibernation
	case "hostValidationConfig":
		return featureflags.FeatureHostValidation
	case "podPriorityClasses":
		return featureflags.FeaturePodPriority
	case "serviceAccounts":
		return featureflags.FeatureServiceAccounts
	case "imageRegistries":
		return featureflags.FeatureImageRegistries
	case "ingressClasses":
		return featureflags.FeatureIngressClasses
	case "namespaces":
		return featureflags.FeatureNamespaces
	case "accessControl":
		return featureflags.FeatureAccessControl
	default:
		return ""
	}
}

func PrintTenantSpec(spec map[string]interface{}, indent string) {
	for k, v := range spec {
		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, k)
			PrintTenantSpec(val, indent+"  ")
		case []interface{}:
			fmt.Printf("%s%s:\n", indent, k)
			for _, item := range val {
				switch it := item.(type) {
				case map[string]interface{}:
					fmt.Printf("%s  -\n", indent)
					PrintTenantSpec(it, indent+"    ")
				case string:
					fmt.Printf("%s  - %s\n", indent, it)
				default:
					fmt.Printf("%s  - %v\n", indent, it)
				}
			}
		case string, bool, int, float64:
			fmt.Printf("%s%s: %v\n", indent, k, val)
		default:
			fmt.Printf("%s%s: %v (type: %T)\n", indent, k, val, val)
		}
	}
}

// GetTenantStatusStorageClasses extracts storageClass names from Tenant's status
func (tc *TenantClient) GetTenantStatusStorageClasses(ctx context.Context, tenantName string) ([]string, error) {
	tenant, err := tc.dynClient.Resource(tc.gvr).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
	}

	// Extract: status.storageClasses.available[].name
	scList, found, err := unstructured.NestedSlice(tenant.Object, "status", "storageClasses", "available")
	if err != nil || !found {
		tc.Logger.Debug("No storageClasses found in tenant status", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	seen := map[string]struct{}{}
	var names []string

	for _, item := range scList {
		if entry, ok := item.(map[string]interface{}); ok {
			if nameRaw, ok := entry["name"]; ok {
				if name, ok := nameRaw.(string); ok && strings.TrimSpace(name) != "" {
					if _, exists := seen[name]; !exists {
						seen[name] = struct{}{}
						names = append(names, name)
					}
				}
			}
		}
	}

	sort.Strings(names)
	return names, nil
}

// GetTenantQuotaName extracts .spec.quota from Tenant
func (tc *TenantClient) GetTenantQuotaName(ctx context.Context, tenantName string) (string, error) {
	tenant, err := tc.dynClient.Resource(tc.gvr).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
	}

	quotaName, found, err := unstructured.NestedString(tenant.Object, "spec", "quota")
	if err != nil {
		return "", fmt.Errorf("error reading spec.quota: %w", err)
	}
	if !found || quotaName == "" {
		return "", fmt.Errorf("tenant %q has no spec.quota defined", tenantName)
	}

	return quotaName, nil
}

// GetTenantImageRegistries extracts .spec.imageRegistries.allowed from Tenant
func (tc *TenantClient) GetTenantImageRegistries(ctx context.Context, tenantName string) ([]string, error) {
	tenant, err := tc.dynClient.Resource(tc.gvr).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
	}

	// Extract: spec.imageRegistries.allowed
	allowed, found, err := unstructured.NestedStringSlice(tenant.Object, "spec", "imageRegistries", "allowed")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.imageRegistries.allowed: %w", err)
	}
	if !found {
		tc.Logger.Debug("No image registries found in tenant spec", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	// Dedupe and sort
	seen := map[string]struct{}{}
	var registries []string
	for _, reg := range allowed {
		reg = strings.TrimSpace(reg)
		if reg == "" {
			continue
		}
		if _, exists := seen[reg]; !exists {
			seen[reg] = struct{}{}
			registries = append(registries, reg)
		}
	}

	sort.Strings(registries)
	return registries, nil
}

// GetTenantIngressClasses extracts .spec.ingressClasses.allowed from Tenant
func (tc *TenantClient) GetTenantIngressClasses(ctx context.Context, tenantName string) ([]string, error) {
	tenant, err := tc.dynClient.Resource(tc.gvr).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
	}

	// Extract: spec.ingressClasses.allowed
	allowed, found, err := unstructured.NestedStringSlice(tenant.Object, "spec", "ingressClasses", "allowed")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.ingressClasses.allowed: %w", err)
	}
	if !found {
		tc.Logger.Debug("No ingress classes found in tenant spec", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	// Dedupe and sort
	seen := map[string]struct{}{}
	var ingressClasses []string
	for _, ic := range allowed {
		ic = strings.TrimSpace(ic)
		if ic == "" {
			continue
		}
		if _, exists := seen[ic]; !exists {
			seen[ic] = struct{}{}
			ingressClasses = append(ingressClasses, ic)
		}
	}

	sort.Strings(ingressClasses)
	return ingressClasses, nil
}

// GetTenantServiceAccountsDenied extracts .spec.serviceAccounts.denied from Tenant
func (tc *TenantClient) GetTenantServiceAccountsDenied(ctx context.Context, tenantName string) ([]string, error) {
	tenant, err := tc.dynClient.Resource(tc.gvr).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
	}

	// Extract: spec.serviceAccounts.denied
	denied, found, err := unstructured.NestedStringSlice(tenant.Object, "spec", "serviceAccounts", "denied")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.serviceAccounts.denied: %w", err)
	}
	if !found {
		tc.Logger.Debug("No denied service accounts found in tenant spec", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	// Dedupe and sort
	seen := map[string]struct{}{}
	var serviceAccounts []string
	for _, sa := range denied {
		sa = strings.TrimSpace(sa)
		if sa == "" {
			continue
		}
		if _, exists := seen[sa]; !exists {
			seen[sa] = struct{}{}
			serviceAccounts = append(serviceAccounts, sa)
		}
	}

	sort.Strings(serviceAccounts)
	return serviceAccounts, nil
}
