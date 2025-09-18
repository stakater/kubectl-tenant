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
	"k8s.io/client-go/dynamic"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
	"github.com/stakater/kubectl-tenant/test/unit/client/mocks"
)

type mockDynamicClient struct {
	mock.Mock
}

func (m *mockDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	args := m.Called(gvr)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

func TestTenantClient_ListAllTenants(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ff := featureflags.NewConfig()

	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	// mockList := new(mocks.MockResourceInterface)

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
	mockResource.On("List", mock.Anything, mock.Anything).Return(tenantList, nil)

	tc := &client.TenantClient{
		dynClient:    mockClient,
		gvr:          schema.GroupVersionResource{Group: "tenantoperator.stakater.com", Version: "v1beta3", Resource: "tenants"},
		FeatureFlags: ff,
		Logger:       logger,
		timeout:      30 * time.Second,
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
