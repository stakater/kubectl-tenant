package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stakater/kubectl-tenant/internal/config"
	"github.com/stakater/kubectl-tenant/internal/featureflags"
	"github.com/stretchr/testify/assert"
)

func TestConfigManager(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load or create config
	ff, err := config.LoadOrCreateConfig()
	assert.NoError(t, err)
	assert.NotNil(t, ff)

	// Modify and save
	ff.Enable(featureflags.FeatureHibernation)
	err = config.SaveConfig(ff)
	assert.NoError(t, err)

	// Reload
	ff2, err := config.LoadOrCreateConfig()
	assert.NoError(t, err)
	assert.True(t, ff2.IsEnabled(featureflags.FeatureHibernation))

	// Check file exists
	configPath := filepath.Join(tmpDir, ".kube", "tenant-config.yaml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}
