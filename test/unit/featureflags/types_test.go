package featureflags_test

import (
	"testing"

	"github.com/stakater/kubectl-tenant/internal/featureflags"
	"github.com/stretchr/testify/assert"
)

func TestFeatureFlags_IsEnabled(t *testing.T) {
	ff := featureflags.NewConfig()

	// Test defaults
	assert.True(t, ff.IsEnabled(featureflags.FeatureHibernation))
	assert.True(t, ff.IsEnabled(featureflags.FeatureNamespaces))
	assert.False(t, ff.IsEnabled(featureflags.FeatureHostValidation)) // experimental

	// Test override
	ff.Enable(featureflags.FeatureHostValidation)
	assert.True(t, ff.IsEnabled(featureflags.FeatureHostValidation))

	ff.Disable(featureflags.FeatureHibernation)
	assert.False(t, ff.IsEnabled(featureflags.FeatureHibernation))
}
