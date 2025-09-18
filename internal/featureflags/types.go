package featureflags

import "sync"

type Feature string

const (
	FeatureHibernation     Feature = "hibernation"
	FeatureHostValidation  Feature = "hostValidation"
	FeaturePodPriority     Feature = "podPriorityClasses"
	FeatureServiceAccounts Feature = "serviceAccounts"
	FeatureImageRegistries Feature = "imageRegistries"
	FeatureIngressClasses  Feature = "ingressClasses"
	FeatureNamespaces      Feature = "namespaces"
	FeatureAccessControl   Feature = "accessControl"
	FeatureStorageClasses  Feature = "storageClasses"
	FeatureQuota           Feature = "quota"
)

type FeatureFlag struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Source  string `json:"source,omitempty" yaml:"source,omitempty"`
}

type Config struct {
	Flags map[Feature]FeatureFlag `json:"featureFlags" yaml:"featureFlags"`
	mu    sync.RWMutex
}

func NewConfig() *Config {
	return &Config{
		Flags: make(map[Feature]FeatureFlag),
	}
}

func (c *Config) IsEnabled(feature Feature) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if flag, exists := c.Flags[feature]; exists {
		return flag.Enabled
	}

	switch feature {
	case FeatureStorageClasses, FeatureQuota, FeatureImageRegistries, FeatureIngressClasses, FeatureServiceAccounts, FeatureNamespaces:
		return true
	default:
		return false
	}
}

func (c *Config) Enable(feature Feature) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Flags[feature] = FeatureFlag{Enabled: true, Source: "runtime"}
}

func (c *Config) Disable(feature Feature) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Flags[feature] = FeatureFlag{Enabled: false, Source: "runtime"}
}
