// internal/client/tenant_spec_extractor.go
package client

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type TenantSpecExtractor struct {
	logger *zap.Logger
}

func NewTenantSpecExtractor(logger *zap.Logger) *TenantSpecExtractor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TenantSpecExtractor{logger: logger}
}

// Helper to get tenant object
func (tse *TenantSpecExtractor) getTenant(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) (*unstructured.Unstructured, error) {
	tenant, err := dynClient.Resource(gvr).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %q: %w", tenantName, err)
	}
	return tenant, nil
}

// GetTenantStatusStorageClasses extracts storageClass names from Tenant's status
func (tse *TenantSpecExtractor) GetTenantStatusStorageClasses(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) ([]string, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	scList, found, err := unstructured.NestedSlice(tenant.Object, "status", "storageClasses", "available")
	if err != nil || !found {
		tse.logger.Debug("No storageClasses found in tenant status", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	return extractStringSliceFromMapSlice(scList, "name"), nil
}

// GetTenantQuotaName extracts .spec.quota from Tenant
func (tse *TenantSpecExtractor) GetTenantQuotaName(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) (string, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return "", err
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
func (tse *TenantSpecExtractor) GetTenantImageRegistries(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) ([]string, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	allowed, found, err := unstructured.NestedStringSlice(tenant.Object, "spec", "imageRegistries", "allowed")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.imageRegistries.allowed: %w", err)
	}
	if !found {
		tse.logger.Debug("No image registries found in tenant spec", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	return dedupeAndSort(allowed), nil
}

// GetTenantIngressClasses extracts .spec.ingressClasses.allowed from Tenant
func (tse *TenantSpecExtractor) GetTenantIngressClasses(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) ([]string, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	allowed, found, err := unstructured.NestedStringSlice(tenant.Object, "spec", "ingressClasses", "allowed")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.ingressClasses.allowed: %w", err)
	}
	if !found {
		tse.logger.Debug("No ingress classes found in tenant spec", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	return dedupeAndSort(allowed), nil
}

// GetTenantServiceAccountsDenied extracts .spec.serviceAccounts.denied from Tenant
func (tse *TenantSpecExtractor) GetTenantServiceAccountsDenied(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) ([]string, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	denied, found, err := unstructured.NestedStringSlice(tenant.Object, "spec", "serviceAccounts", "denied")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.serviceAccounts.denied: %w", err)
	}
	if !found {
		tse.logger.Debug("No denied service accounts found in tenant spec", zap.String("tenant", tenantName))
		return []string{}, nil
	}

	return dedupeAndSort(denied), nil
}

// dedupeAndSort removes duplicates and sorts string slice
func dedupeAndSort(items []string) []string {
	seen := map[string]struct{}{}
	var result []string

	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}

	sort.Strings(result)
	return result
}

// extractStringSliceFromMapSlice extracts []string from []map[string]interface{} using key
func extractStringSliceFromMapSlice(slice []interface{}, key string) []string {
	seen := map[string]struct{}{}
	var result []string

	for _, item := range slice {
		if entry, ok := item.(map[string]interface{}); ok {
			if nameRaw, ok := entry[key]; ok {
				if name, ok := nameRaw.(string); ok && strings.TrimSpace(name) != "" {
					if _, exists := seen[name]; !exists {
						seen[name] = struct{}{}
						result = append(result, name)
					}
				}
			}
		}
	}

	sort.Strings(result)
	return result
}

// GetTenantNamespacesConfig extracts full .spec.namespaces from Tenant
func (tse *TenantSpecExtractor) GetTenantNamespacesConfig(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) (map[string]interface{}, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	namespaces, found, err := unstructured.NestedMap(tenant.Object, "spec", "namespaces")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.namespaces: %w", err)
	}
	if !found {
		tse.logger.Debug("No namespaces config found in tenant spec", zap.String("tenant", tenantName))
		return map[string]interface{}{}, nil
	}

	return namespaces, nil
}

// GetTenantHibernationConfig extracts .spec.hibernation from Tenant
func (tse *TenantSpecExtractor) GetTenantHibernationConfig(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) (map[string]interface{}, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	hibernation, found, err := unstructured.NestedMap(tenant.Object, "spec", "hibernation")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.hibernation: %w", err)
	}
	if !found {
		tse.logger.Debug("No hibernation config found in tenant spec", zap.String("tenant", tenantName))
		return map[string]interface{}{}, nil
	}

	return hibernation, nil
}

// GetTenantHostValidationConfig extracts .spec.hostValidationConfig from Tenant
func (tse *TenantSpecExtractor) GetTenantHostValidationConfig(ctx context.Context, dynClient dynamic.Interface, gvr schema.GroupVersionResource, tenantName string) (map[string]interface{}, error) {
	tenant, err := tse.getTenant(ctx, dynClient, gvr, tenantName)
	if err != nil {
		return nil, err
	}

	hostValidation, found, err := unstructured.NestedMap(tenant.Object, "spec", "hostValidationConfig")
	if err != nil {
		return nil, fmt.Errorf("error reading spec.hostValidationConfig: %w", err)
	}
	if !found {
		tse.logger.Debug("No host validation config found in tenant spec", zap.String("tenant", tenantName))
		return map[string]interface{}{}, nil
	}

	return hostValidation, nil
}
