// internal/config/manager.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/stakater/kubectl-tenant/internal/featureflags"
	"gopkg.in/yaml.v3"
)

const configFileName = "tenant-config.yaml"

func GetConfigPath() string {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "kubectl-tenant", configFileName)
	}
	return filepath.Join(os.Getenv("HOME"), ".kube", configFileName)
}

func LoadOrCreateConfig() (*featureflags.Config, error) {
	path := GetConfigPath()

	// Create config dir if not exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Try to load existing config
	ff := featureflags.NewConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Create default config
		if err := SaveConfig(ff); err != nil {
			return nil, err
		}
		fmt.Printf("✨ Created default config: %s\n", path)
		return ff, nil
	}

	if err := yaml.Unmarshal(data, ff); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return ff, nil
}

func SaveConfig(ff *featureflags.Config) error {
	path := GetConfigPath()
	data, err := yaml.Marshal(ff)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
