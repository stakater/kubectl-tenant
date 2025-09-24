package extractors

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractStorageClassNames(u *unstructured.Unstructured) ([]string, error) {
	return extractFromSpecStringSlice(u, []string{"spec", "storageClasses", "allowed"})
}

func ExtractIngressClassNames(u *unstructured.Unstructured) ([]string, error) {
	return extractFromSpecStringSlice(u, []string{"spec", "ingressClasses", "allowed"})
}

func ExtractPriorityClassNames(u *unstructured.Unstructured) ([]string, error) {
	return extractFromSpecStringSlice(u, []string{"spec", "podPriorityClasses", "allowed"})
}

func ExtractResourceQuotaNames(u *unstructured.Unstructured) ([]string, error) {
	quotaName, found, err := unstructured.NestedString(u.Object, "spec", "quota")
	if err != nil {
		return nil, fmt.Errorf("reading spec.quota: %w", err)
	}
	if !found || strings.TrimSpace(quotaName) == "" {
		return []string{}, nil
	}
	return []string{quotaName}, nil
}

func ExtractNamespaceNames(u *unstructured.Unstructured) ([]string, error) {
	var allNamespaces []string
	tenantName := u.GetName()

	// Extract from spec.namespaces.withoutTenantPrefix
	withoutPrefix, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "namespaces", "withoutTenantPrefix")
	allNamespaces = append(allNamespaces, withoutPrefix...)

	// Extract from spec.namespaces.withTenantPrefix and prepend tenant name
	withPrefix, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "namespaces", "withTenantPrefix")
	for _, ns := range withPrefix {
		allNamespaces = append(allNamespaces, tenantName+"-"+ns)
	}

	// Check for sandboxes if enabled
	sandboxEnabled, found, _ := unstructured.NestedBool(u.Object, "spec", "namespaces", "sandboxes", "enabled")
	if found && sandboxEnabled {
		// Add sandbox namespace (typically tenant-name-sandbox)
		allNamespaces = append(allNamespaces, tenantName+"-sandbox")
	}

	return UniqSorted(allNamespaces), nil
}

func ExtractServiceAccountNames(u *unstructured.Unstructured) ([]string, error) {
	denied, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "serviceAccounts", "denied")
	if len(denied) > 0 {
		return []string{}, fmt.Errorf("tenant has denied service accounts: %v. Use kubectl get serviceaccounts to see all and filter manually", denied)
	}
	return []string{}, nil
}

func ExtractClusterRoleNames(u *unstructured.Unstructured) ([]string, error) {
	// Extract roles from accessControl configuration
	var roles []string

	// This would require parsing the accessControl structure to determine
	// what ClusterRoles are granted to the tenant users/groups
	// For now, return empty list - this would need custom logic based on your RBAC setup
	return roles, nil
}

func ExtractRoleNames(u *unstructured.Unstructured) ([]string, error) {
	// Similar to ClusterRoles - would need custom logic to determine
	// which Roles are created for this tenant
	return []string{}, nil
}

func ExtractRoleBindingNames(u *unstructured.Unstructured) ([]string, error) {
	// Similar to Roles - would need custom logic to determine
	// which RoleBindings are created for this tenant
	return []string{}, nil
}

// ---- Helper functions ----

func extractFromSpecStringSlice(u *unstructured.Unstructured, path []string) ([]string, error) {
	slice, found, err := unstructured.NestedStringSlice(u.Object, path...)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", strings.Join(path, "."), err)
	}
	if !found {
		return []string{}, nil
	}
	return UniqSorted(slice), nil
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
