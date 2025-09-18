// test/unit/commands/namespaces_test.go
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

func TestNamespacesCommand(t *testing.T) {
	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface)

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "tenant-sample"},
			"spec": map[string]interface{}{
				"namespaces": map[string]interface{}{
					"sandboxes": map[string]interface{}{
						"enabled": true,
						"private": true,
					},
					"withoutTenantPrefix":     []interface{}{"analytics", "marketing"},
					"withTenantPrefix":        []interface{}{"dev", "staging"},
					"onDeletePurgeNamespaces": true,
					"metadata": map[string]interface{}{
						"common": map[string]interface{}{
							"labels": map[string]interface{}{
								"common-label": "common-value",
							},
							"annotations": map[string]interface{}{
								"common-annotation": "common-value",
							},
						},
						"sandbox": map[string]interface{}{
							"labels": map[string]interface{}{
								"sandbox-label": "sandbox-value",
							},
							"annotations": map[string]interface{}{
								"sandbox-annotation": "sandbox-value",
							},
						},
						"specific": []interface{}{
							map[string]interface{}{
								"namespaces": []interface{}{"tenant-sample-dev"},
								"labels": map[string]interface{}{
									"specific-label": "specific-dev-value",
								},
								"annotations": map[string]interface{}{
									"specific-annotation": "specific-dev-value",
								},
							},
						},
					},
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

	cmd := tenant.NewListNamespacesCmd()
	cmd.SetArgs([]string{"tenant-sample"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Tenant: tenant-sample")
	assert.Contains(t, output, "Sandboxes:")
	assert.Contains(t, output, "Enabled: true")
	assert.Contains(t, output, "Private: true")
	assert.Contains(t, output, "On Delete Purge Namespaces: true")
	assert.Contains(t, output, "Namespaces Without Tenant Prefix:")
	assert.Contains(t, output, "- analytics")
	assert.Contains(t, output, "- marketing")
	assert.Contains(t, output, "Namespace Prefixes (will be prepended with tenant name):")
	assert.Contains(t, output, "- dev")
	assert.Contains(t, output, "- staging")
	assert.Contains(t, output, "Metadata Templates:")
	assert.Contains(t, output, "Common:")
	assert.Contains(t, output, "Labels:")
	assert.Contains(t, output, "common-label: common-value")
	assert.Contains(t, output, "Annotations:")
	assert.Contains(t, output, "common-annotation: common-value")
	assert.Contains(t, output, "Sandbox:")
	assert.Contains(t, output, "sandbox-label: sandbox-value")
	assert.Contains(t, output, "Specific:")
	assert.Contains(t, output, "Namespaces:")
	assert.Contains(t, output, "- tenant-sample-dev")
	assert.Contains(t, output, "specific-label: specific-dev-value")
}
