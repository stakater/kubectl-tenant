package client

import (
	"context"
	"fmt"
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
