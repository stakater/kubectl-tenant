package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const testTenant = "e2e-tenant"
const invalidTenant = "nonexistent-tenant"

// resourceTestConfig holds test fixtures for each resource type
type resourceTestConfig struct {
	cmdName   string   // command name (e.g., "storageclasses")
	allowed   []string // resources allowed for tenant
	forbidden string   // exists in cluster but not allowed
	invalid   string   // doesn't exist at all
	tenantNs1 string   // for namespaces only
	tenantNs2 string   // for namespaces only
}

var testResources = map[string]resourceTestConfig{
	"storageclasses": {
		cmdName:   "storageclasses",
		allowed:   []string{"e2e-sc-standard", "e2e-sc-fast"},
		forbidden: "e2e-sc-forbidden",
		invalid:   "nonexistent-storageclass",
	},
	"ingressclasses": {
		cmdName:   "ingressclasses",
		allowed:   []string{"e2e-ic-nginx", "e2e-ic-traefik"},
		forbidden: "e2e-ic-forbidden",
		invalid:   "nonexistent-ingressclass",
	},
	"priorityclasses": {
		cmdName:   "priorityclasses",
		allowed:   []string{"e2e-pc-low", "e2e-pc-high"},
		forbidden: "e2e-pc-forbidden",
		invalid:   "nonexistent-priorityclass",
	},
	"quotas": {
		cmdName:   "quotas",
		allowed:   []string{"small"},
		forbidden: "e2e-quota-forbidden",
		invalid:   "nonexistent-quota",
	},
	"namespaces": {
		cmdName:   "namespaces",
		allowed:   []string{"e2e-tenant-dev", "e2e-tenant-prod"},
		forbidden: "default",
		invalid:   "nonexistent-namespace",
		tenantNs1: "e2e-tenant-dev",
		tenantNs2: "e2e-tenant-prod",
	},
}

var (
	dyn dynamic.Interface

	tenantGVR = schema.GroupVersionResource{
		Group: "tenantoperator.stakater.com", Version: "v1beta3", Resource: "tenants",
	}
	storageClassGVR = schema.GroupVersionResource{
		Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses",
	}
	ingressClassGVR = schema.GroupVersionResource{
		Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses",
	}
	priorityClassGVR = schema.GroupVersionResource{
		Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses",
	}
	quotaGVR = schema.GroupVersionResource{
		Group: "tenantoperator.stakater.com", Version: "v1beta1", Resource: "quotas",
	}
)

func TestMain(m *testing.M) {
	// Build plugin
	cmd := exec.Command("go", "build", "-o", "kubectl-tenant", "..")
	if err := cmd.Run(); err != nil {
		panic("failed to build kubectl-tenant: " + err.Error())
	}

	// Init client
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic("failed to build config: " + err.Error())
	}
	dyn, err = dynamic.NewForConfig(cfg)
	if err != nil {
		panic("failed to create dynamic client: " + err.Error())
	}

	os.Exit(m.Run())
}

func runPlugin(args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command("./kubectl-tenant", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// generateResourceTests creates standard test cases for any resource type
func generateResourceTests(cfg resourceTestConfig) []struct {
	name           string
	args           []string
	wantErr        bool
	wantErrContain string
	wantOutContain string
} {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		wantErrContain string
		wantOutContain string
	}{
		{
			name:           "list permitted resources",
			args:           []string{"get", cfg.cmdName, testTenant},
			wantOutContain: cfg.allowed[0],
		},
		{
			name:           "get first permitted resource",
			args:           []string{"get", cfg.cmdName, testTenant, cfg.allowed[0]},
			wantOutContain: cfg.allowed[0],
		},
		{
			name:           "error: resource not in tenant allowed list",
			args:           []string{"get", cfg.cmdName, testTenant, cfg.forbidden},
			wantErr:        true,
			wantErrContain: "not permitted",
		},
		{
			name:           "error: resource doesn't exist",
			args:           []string{"get", cfg.cmdName, testTenant, cfg.invalid},
			wantErr:        true,
			wantErrContain: "not permitted",
		},
		{
			name:           "error: invalid tenant name",
			args:           []string{"get", cfg.cmdName, invalidTenant},
			wantErr:        true,
			wantErrContain: invalidTenant,
		},
		{
			name:           "output format: json",
			args:           []string{"get", cfg.cmdName, testTenant, "-o", "json"},
			wantOutContain: `"kind"`,
		},
		{
			name:           "output format: yaml",
			args:           []string{"get", cfg.cmdName, testTenant, "-o", "yaml"},
			wantOutContain: "kind:",
		},
	}

	// Add second resource test if available
	if len(cfg.allowed) > 1 {
		tests = append(tests, struct {
			name           string
			args           []string
			wantErr        bool
			wantErrContain string
			wantOutContain string
		}{
			name:           "get second permitted resource",
			args:           []string{"get", cfg.cmdName, testTenant, cfg.allowed[1]},
			wantOutContain: cfg.allowed[1],
		})
	}

	return tests
}

func runTestCases(t *testing.T, tests []struct {
	name           string
	args           []string
	wantErr        bool
	wantErrContain string
	wantOutContain string
}) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runPlugin(tt.args...)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got success. stdout: %s", stdout)
				}
				if tt.wantErrContain != "" && !strings.Contains(stderr, tt.wantErrContain) {
					t.Errorf("stderr %q should contain %q", stderr, tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v, stderr: %s", err, stderr)
			}
			if tt.wantOutContain != "" && !strings.Contains(stdout, tt.wantOutContain) {
				t.Errorf("stdout %q should contain %q", stdout, tt.wantOutContain)
			}
		})
	}
}

// Setup and cleanup functions

func setupTestResources(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	sc := testResources["storageclasses"]
	for _, name := range append(sc.allowed, sc.forbidden) {
		createStorageClass(t, ctx, name)
	}

	ic := testResources["ingressclasses"]
	for _, name := range append(ic.allowed, ic.forbidden) {
		createIngressClass(t, ctx, name)
	}

	pc := testResources["priorityclasses"]
	for i, name := range append(pc.allowed, pc.forbidden) {
		createPriorityClass(t, ctx, name, int64((i+1)*100))
	}

	q := testResources["quotas"]
	for _, name := range append(q.allowed, q.forbidden) {
		createQuota(t, ctx, name)
	}

	createTenant(t, ctx)
	waitForTenantReady(t, ctx)
}

func createStorageClass(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	sc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion":  "storage.k8s.io/v1",
			"kind":        "StorageClass",
			"metadata":    map[string]interface{}{"name": name},
			"provisioner": "kubernetes.io/no-provisioner",
		},
	}
	_, err := dyn.Resource(storageClassGVR).Create(ctx, sc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("failed to create storageclass %s: %v", name, err)
	}
}

func createIngressClass(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	ic := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.k8s.io/v1",
			"kind":       "IngressClass",
			"metadata":   map[string]interface{}{"name": name},
			"spec":       map[string]interface{}{"controller": "k8s.io/ingress-" + name},
		},
	}
	_, err := dyn.Resource(ingressClassGVR).Create(ctx, ic, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("failed to create ingressclass %s: %v", name, err)
	}
}

func createPriorityClass(t *testing.T, ctx context.Context, name string, value int64) {
	t.Helper()
	pc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion":    "scheduling.k8s.io/v1",
			"kind":          "PriorityClass",
			"metadata":      map[string]interface{}{"name": name},
			"value":         value,
			"globalDefault": false,
			"description":   "E2E test priority class",
		},
	}
	_, err := dyn.Resource(priorityClassGVR).Create(ctx, pc, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("failed to create priorityclass %s: %v", name, err)
	}
}

func createQuota(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	quota := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tenantoperator.stakater.com/v1beta1",
			"kind":       "Quota",
			"metadata":   map[string]interface{}{"name": name},
			"spec": map[string]interface{}{
				"resourcequota": map[string]interface{}{
					"hard": map[string]interface{}{
						"requests.cpu":    "2",
						"requests.memory": "4Gi",
						"limits.cpu":      "4",
						"limits.memory":   "8Gi",
					},
				},
			},
		},
	}
	_, err := dyn.Resource(quotaGVR).Create(ctx, quota, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("failed to create quota %s: %v", name, err)
	}
}

func createTenant(t *testing.T, ctx context.Context) {
	t.Helper()
	sc := testResources["storageclasses"]
	ic := testResources["ingressclasses"]
	pc := testResources["priorityclasses"]
	q := testResources["quotas"]

	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tenantoperator.stakater.com/v1beta3",
			"kind":       "Tenant",
			"metadata":   map[string]interface{}{"name": testTenant},
			"spec": map[string]interface{}{
				"quota": q.allowed[0],
				"namespaces": map[string]interface{}{
					"withTenantPrefix":        []interface{}{"dev", "prod"},
					"onDeletePurgeNamespaces": true,
				},
				"storageClasses":     map[string]interface{}{"allowed": toInterfaceSlice(sc.allowed)},
				"ingressClasses":     map[string]interface{}{"allowed": toInterfaceSlice(ic.allowed)},
				"podPriorityClasses": map[string]interface{}{"allowed": toInterfaceSlice(pc.allowed)},
			},
		},
	}
	_, err := dyn.Resource(tenantGVR).Create(ctx, tenant, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("failed to create tenant: %v", err)
	}
}

func toInterfaceSlice(s []string) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

func waitForTenantReady(t *testing.T, ctx context.Context) {
	t.Helper()
	timeout := time.After(2 * time.Minute)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for tenant to be ready")
		case <-tick.C:
			tenant, err := dyn.Resource(tenantGVR).Get(ctx, testTenant, metav1.GetOptions{})
			if err != nil {
				continue
			}

			// Check if namespaces are deployed
			deployedNs, found, _ := unstructured.NestedStringSlice(tenant.Object, "status", "deployedNamespaces")
			if found && len(deployedNs) >= 2 {
				t.Logf("Tenant ready with namespaces: %v", deployedNs)
				return
			}
		}
	}
}

func cleanupTestResources(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	_ = dyn.Resource(tenantGVR).Delete(ctx, testTenant, metav1.DeleteOptions{})

	sc := testResources["storageclasses"]
	for _, name := range append(sc.allowed, sc.forbidden) {
		_ = dyn.Resource(storageClassGVR).Delete(ctx, name, metav1.DeleteOptions{})
	}

	ic := testResources["ingressclasses"]
	for _, name := range append(ic.allowed, ic.forbidden) {
		_ = dyn.Resource(ingressClassGVR).Delete(ctx, name, metav1.DeleteOptions{})
	}

	pc := testResources["priorityclasses"]
	for _, name := range append(pc.allowed, pc.forbidden) {
		_ = dyn.Resource(priorityClassGVR).Delete(ctx, name, metav1.DeleteOptions{})
	}

	q := testResources["quotas"]
	for _, name := range append(q.allowed, q.forbidden) {
		_ = dyn.Resource(quotaGVR).Delete(ctx, name, metav1.DeleteOptions{})
	}
}

// TestE2E runs all e2e tests
func TestE2E(t *testing.T) {
	setupTestResources(t)
	t.Cleanup(func() { cleanupTestResources(t) })

	// Test all resources using generated tests
	for name, cfg := range testResources {
		t.Run(name, func(t *testing.T) {
			tests := generateResourceTests(cfg)
			runTestCases(t, tests)
		})
	}
}
