// test/unit/commands/host_validation_test.go
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

func TestHostValidationCommand(t *testing.T) {
	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface)

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "tenant-sample"},
			"spec": map[string]interface{}{
				"hostValidationConfig": map[string]interface{}{
					"allowed":       []interface{}{"gateway.saap.dev", "console.saap.dev", "*.saap.prod"},
					"allowedRegex":  "^[a-zA-Z0-9-]+\\.saap\\.dev$",
					"denyWildcards": false,
				},
			},
		},
	}

	mockClient.On("Resource", mock.Anything).Return(mockResource)
	mockResource.On("Namespace", "").Return(mockInterface)
	mockInterface.On("Get", mock.Anything, "tenant-sample", mock.Anything).Return(tenant, nil)

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

	cmd := tenant.NewHostValidationCmd()
	cmd.SetArgs([]string{"tenant-sample"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Tenant: tenant-sample")
	assert.Contains(t, output, "Allowed Hosts:")
	assert.Contains(t, output, "- gateway.saap.dev")
	assert.Contains(t, output, "- console.saap.dev")
	assert.Contains(t, output, "- *.saap.prod")
	assert.Contains(t, output, "Allowed Regex Pattern: ^[a-zA-Z0-9-]+\\.saap\\.dev$")
	assert.Contains(t, output, "Deny Wildcards: false")
}
