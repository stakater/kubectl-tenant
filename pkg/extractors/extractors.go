package extractors

import (
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractStorageClassNames(tenant *unstructured.Unstructured, logger *zap.Logger) ([]string, error) {
	// First try status.storageClasses.available (from status)
	if names := ExtractFromStatus(tenant, []string{"status", "storageClasses", "available"}, "name", logger); len(names) > 0 {
		return names, nil
	}

	// Fallback to spec if needed (uncomment if your tenant spec has storageClasses)
	// return extractFromSpec(tenant, []string{"spec", "storageClasses", "allowed"}, logger), nil

	return []string{}, nil
}

func ExtractIngressClassNames(tenant *unstructured.Unstructured, logger *zap.Logger) ([]string, error) {
	// Extract from spec.ingressClasses.allowed
	return ExtractFromSpec(tenant, []string{"spec", "ingressClasses", "allowed"}, logger), nil
}

func ExtractPriorityClassNames(tenant *unstructured.Unstructured, logger *zap.Logger) ([]string, error) {
	// Extract from spec.podPriorityClasses.allowed
	return ExtractFromSpec(tenant, []string{"spec", "podPriorityClasses", "allowed"}, logger), nil
}

func ExtractResourceQuotaNames(tenant *unstructured.Unstructured, logger *zap.Logger) ([]string, error) {
	// Extract single quota name from spec.quota
	quotaName, found, err := unstructured.NestedString(tenant.Object, "spec", "quota")
	if err != nil {
		return nil, fmt.Errorf("reading spec.quota: %w", err)
	}
	if !found || strings.TrimSpace(quotaName) == "" {
		return []string{}, nil
	}
	return []string{quotaName}, nil
}

// -------- helper extraction functions --------

// extractFromStatus extracts names from a slice of objects in tenant status
func ExtractFromStatus(tenant *unstructured.Unstructured, path []string, nameField string, logger *zap.Logger) []string {
	slice, found, err := unstructured.NestedSlice(tenant.Object, path...)
	if err != nil || !found {
		logger.Debug("Path not found in tenant status", zap.Strings("path", path))
		return []string{}
	}

	seen := map[string]struct{}{}
	var out []string
	for _, item := range slice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Try both lowercase and capitalized field names
		var nameRaw interface{}
		if val, ok := itemMap[nameField]; ok {
			nameRaw = val
		} else if val, ok := itemMap[strings.Title(nameField)]; ok {
			nameRaw = val
		}

		name, ok := nameRaw.(string)
		if !ok {
			continue
		}

		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}

	sort.Strings(out)
	return out
}

// extractFromSpec extracts string slice from tenant spec
func ExtractFromSpec(tenant *unstructured.Unstructured, path []string, logger *zap.Logger) []string {
	slice, found, err := unstructured.NestedStringSlice(tenant.Object, path...)
	if err != nil || !found {
		logger.Debug("Path not found in tenant spec", zap.Strings("path", path))
		return []string{}
	}

	return UniqSorted(slice)
}

func UniqSorted(in []string) []string {
	seen := map[string]struct{}{}
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			seen[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
