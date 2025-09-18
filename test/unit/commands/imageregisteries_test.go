package commands_test

import (
	"bytes"
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

func TestImageRegistriesCommand(t *testing.T) {
	// Mock client
	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface)

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "tenant-sample"},
			"spec": map[string]interface{}{
				"imageRegistries": map[string]interface{}{
					"allowed": []interface{}{"docker.io", "ghcr.io"},
				},
			},
		},
	}

	mockClient.On("Resource", mock.Anything).Return(mockResource)
	mockResource.On("Namespace", "").Return(mockInterface)
	mockInterface.On("Get", mock.Anything, "tenant-sample", mock.Anything).Return(tenant, nil)

	// Override client creation
	originalNewTenantClient := client.NewTenantClient
	client.NewTenantClient = func(ff *featureflags.Config, logger *zaptest.Logger) (*client.TenantClient, error) {
		return &client.TenantClient{
			dynClient:    mockClient,
			gvr:          schema.GroupVersionResource{Group: "tenantoperator.stakater.com", Version: "v1beta3", Resource: "tenants"},
			FeatureFlags: ff,
			Logger:       logger,
			timeout:      30 * time.Second,
		}, nil
	}
	defer func() { client.NewTenantClient = originalNewTenantClient }()

	// Create command
	cmd := tenant.NewListImageRegistriesCmd()
	cmd.SetArgs([]string{"tenant-sample"})

	// Capture output
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Execute
	err := cmd.Execute()
	assert.NoError(t, err)

	// Check output
	output := out.String()
	assert.Contains(t, output, "Tenant: tenant-sample")
	assert.Contains(t, output, "Allowed Image Registries:")
	assert.Contains(t, output, "- docker.io")
	assert.Contains(t, output, "- ghcr.io")
}
