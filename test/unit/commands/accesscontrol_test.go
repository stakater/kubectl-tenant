// test/unit/commands/access_control_test.go
package commands_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ktcmd "github.com/stakater/kubectl-tenant/cmd/kubectl-tenant" // 👈 Import cmd package
	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
	"github.com/stakater/kubectl-tenant/test/unit/client/mocks"
)

func TestAccessControlCommand(t *testing.T) {
	mockClient := new(mocks.MockDynamicClient)
	mockResource := new(mocks.MockNamespaceableResourceInterface)
	mockInterface := new(mocks.MockResourceInterface)

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "tenant-sample"},
			"spec": map[string]interface{}{
				"accessControl": map[string]interface{}{
					"owners": map[string]interface{}{
						"users":  []interface{}{"kubeadmin"},
						"groups": []interface{}{"admin-group"},
					},
					"editors": map[string]interface{}{
						"users":  []interface{}{"devuser1", "devuser2"},
						"groups": []interface{}{"dev-group"},
					},
					"viewers": map[string]interface{}{
						"users":  []interface{}{"viewuser"},
						"groups": []interface{}{"view-group"},
					},
				},
			},
		},
	}

	mockClient.On("Resource", mock.Anything).Return(mockResource)
	mockResource.On("Namespace", "").Return(mockInterface)
	mockInterface.On("Get", mock.Anything, "tenant-sample", mock.Anything).Return(tenant, nil)

	// ✅ Use exported fields
	tc := &client.TenantClient{
		DynClient:    mockClient,
		Gvr:          schema.GroupVersionResource{Group: "tenantoperator.stakater.com", Version: "v1beta3", Resource: "tenants"},
		FeatureFlags: featureflags.NewConfig(),
		Logger:       zaptest.NewLogger(t), // ✅ Returns *zap.Logger
		Timeout:      30 * time.Second,
	}

	// ✅ Override function for test
	originalNewTenantClient := client.NewTenantClient
	client.NewTenantClient = func(ff *featureflags.Config, logger *zap.Logger) (*client.TenantClient, error) {
		return tc, nil
	}
	defer func() { client.NewTenantClient = originalNewTenantClient }()

	// ✅ Use correct package
	cmd := ktcmd.NewAccessControlCmd()
	cmd.SetArgs([]string{"tenant-sample"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Tenant: tenant-sample")
	assert.Contains(t, output, "Owners:")
	assert.Contains(t, output, "Users:")
	assert.Contains(t, output, "- kubeadmin")
	assert.Contains(t, output, "Groups:")
	assert.Contains(t, output, "- admin-group")
	assert.Contains(t, output, "Editors:")
	assert.Contains(t, output, "- devuser1")
	assert.Contains(t, output, "- devuser2")
	assert.Contains(t, output, "Viewers:")
	assert.Contains(t, output, "- viewuser")
	assert.Contains(t, output, "- view-group")
}
