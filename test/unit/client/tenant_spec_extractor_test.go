package client_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/test/unit/client/mocks"
)

func TestTenantSpecExtractor_GetTenantQuotaName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	extractor := client.NewTenantSpecExtractor(logger)

	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface)

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "test-tenant"},
			"spec": map[string]interface{}{
				"quota": "small",
			},
		},
	}

	mockClient.On("Resource", mock.Anything).Return(mockResource)
	mockResource.On("Namespace", "").Return(mockInterface)
	mockInterface.On("Get", mock.Anything, "test-tenant", mock.Anything).Return(tenant, nil)

	quotaName, err := extractor.GetTenantQuotaName(context.Background(), mockClient, schema.GroupVersionResource{}, "test-tenant")
	assert.NoError(t, err)
	assert.Equal(t, "small", quotaName)
}

func TestTenantSpecExtractor_GetTenantImageRegistries(t *testing.T) {
	logger := zaptest.NewLogger(t)
	extractor := client.NewTenantSpecExtractor(logger)

	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface)

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "test-tenant"},
			"spec": map[string]interface{}{
				"imageRegistries": map[string]interface{}{
					"allowed": []interface{}{"docker.io", "ghcr.io", "docker.io"}, // dup
				},
			},
		},
	}

	mockClient.On("Resource", mock.Anything).Return(mockResource)
	mockResource.On("Namespace", "").Return(mockInterface)
	mockInterface.On("Get", mock.Anything, "test-tenant", mock.Anything).Return(tenant, nil)

	registries, err := extractor.GetTenantImageRegistries(context.Background(), mockClient, schema.GroupVersionResource{}, "test-tenant")
	assert.NoError(t, err)
	assert.Equal(t, []string{"docker.io", "ghcr.io"}, registries) // deduped and sorted
}
