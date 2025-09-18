package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/stakater/kubectl-tenant/internal/client"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
)

func TestFilterSpecByFeatureFlags(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ff := featureflags.NewConfig()
	ff.Disable(featureflags.FeatureHostValidation)
	ff.Disable(featureflags.FeaturePodPriority)

	tc := &client.TenantClient{
		FeatureFlags: ff,
		Logger:       logger,
	}

	inputSpec := map[string]interface{}{
		"quota": "small",
		"hostValidationConfig": map[string]interface{}{
			"allowed": []interface{}{"*.example.com"},
		},
		"podPriorityClasses": map[string]interface{}{
			"allowed": []interface{}{"high-priority"},
		},
		"accessControl": "present",
	}

	filtered := tc.FilterSpecByFeatureFlags(inputSpec).(map[string]interface{})

	assert.Contains(t, filtered, "quota")
	assert.Contains(t, filtered, "accessControl")
	assert.NotContains(t, filtered, "hostValidationConfig")
	assert.NotContains(t, filtered, "podPriorityClasses")
}
