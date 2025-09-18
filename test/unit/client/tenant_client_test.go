// test/unit/client/tenant_client_test.go
package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
	"github.com/stakater/kubectl-tenant/test/unit/client/mocks"
)

func TestTenantClient_ListAllTenants(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ff := featureflags.NewConfig()

	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface) // ← Only create if you use it

	// Mock tenant list
	tenantList := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{"name": "tenant-1"},
					"spec": map[string]interface{}{
						"quota": "small",
						"hibernation": map[string]interface{}{
							"sleepSchedule": "0 20 * * *",
						},
					},
				},
			},
		},
	}

	mockClient.On("Resource", mock.Anything).Return(mockResource)
	mockResource.On("Namespace", "").Return(mockInterface)
	mockInterface.On("List", mock.Anything, mock.Anything).Return(tenantList, nil)

	tc := &client.TenantClient{
		DynClient:    mockClient,                                                                                                 // ✅ Exported field
		Gvr:          schema.GroupVersionResource{Group: "tenantoperator.stakater.com", Version: "v1beta3", Resource: "tenants"}, // ✅ Exported field
		FeatureFlags: ff,
		Logger:       logger,
		Timeout:      30 * time.Second, // ✅ Exported field
	}

	tenants, err := tc.ListAllTenants(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tenants, 1)
	assert.Equal(t, "tenant-1", tenants[0].GetName())
}

func TestTenantClient_FilterSpecByFeatureFlags(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ff := featureflags.NewConfig()
	ff.Disable(featureflags.FeatureHibernation)

	tc := &client.TenantClient{
		FeatureFlags: ff,
		Logger:       logger,
	}

	spec := map[string]interface{}{
		"quota":       "small",
		"hibernation": map[string]interface{}{"sleepSchedule": "0 20 * * *"},
		"namespaces":  map[string]interface{}{"enabled": true},
	}

	filtered := tc.FilterSpecByFeatureFlags(spec).(map[string]interface{})
	assert.Contains(t, filtered, "quota")
	assert.Contains(t, filtered, "namespaces")
	assert.NotContains(t, filtered, "hibernation")
}
