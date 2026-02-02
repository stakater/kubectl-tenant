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

const (
	// Test fixtures - these get created in setup
	testTenant     = "e2e-tenant"
	testNamespace1 = "e2e-tenant-dev"  // withTenantPrefix: dev
	testNamespace2 = "e2e-tenant-prod" // withTenantPrefix: prod
	testSC1        = "e2e-sc-standard"
	testSC2        = "e2e-sc-fast"

	// Invalid names for error cases
	invalidTenant = "nonexistent-tenant"
	invalidNS     = "nonexistent-namespace"
	invalidSC     = "nonexistent-storageclass"
	forbiddenSC   = "e2e-sc-forbidden" // exists in cluster but NOT in tenant's allowed list
)

var (
	dyn dynamic.Interface

	tenantGVR = schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com",
		Version:  "v1beta3",
		Resource: "tenants",
	}
	storageClassGVR = schema.GroupVersionResource{
		Group:    "storage.k8s.io",
		Version:  "v1",
		Resource: "storageclasses",
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

// setupTestResources creates all resources needed for e2e tests.
// Call this once before running tests.
func setupTestResources(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Create StorageClasses
	for _, sc := range []string{testSC1, testSC2, forbiddenSC} {
		createStorageClass(t, ctx, sc)
	}

	// Create Tenant - MTO will create namespaces and populate status automatically
	createTenant(t, ctx)

	// Wait for tenant to be ready and namespaces to be created
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

func createTenant(t *testing.T, ctx context.Context) {
	t.Helper()
	tenant := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tenantoperator.stakater.com/v1beta3",
			"kind":       "Tenant",
			"metadata":   map[string]interface{}{"name": testTenant},
			"spec": map[string]interface{}{
				"quota": "small",
				"namespaces": map[string]interface{}{
					"withTenantPrefix":        []interface{}{"dev", "prod"},
					"onDeletePurgeNamespaces": true,
				},
				"storageClasses": map[string]interface{}{
					"allowed": []interface{}{testSC1, testSC2},
				},
			},
		},
	}
	_, err := dyn.Resource(tenantGVR).Create(ctx, tenant, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("failed to create tenant: %v", err)
	}
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

	// Delete tenant (MTO will cleanup namespaces if onDeletePurgeNamespaces is set)
	_ = dyn.Resource(tenantGVR).Delete(ctx, testTenant, metav1.DeleteOptions{})

	// Delete storage classes
	for _, sc := range []string{testSC1, testSC2, forbiddenSC} {
		_ = dyn.Resource(storageClassGVR).Delete(ctx, sc, metav1.DeleteOptions{})
	}
}

// TestE2E runs all e2e tests in sequence with shared setup
func TestE2E(t *testing.T) {
	setupTestResources(t)
	t.Cleanup(func() { cleanupTestResources(t) })

	t.Run("Namespaces", testGetNamespaces)
	t.Run("StorageClasses", testGetStorageClasses)
}

func testGetNamespaces(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		wantErrContain string
		wantOutContain string
	}{
		{
			name:           "list permitted namespaces",
			args:           []string{"get", "namespaces", testTenant},
			wantOutContain: testNamespace1,
		},
		{
			name:           "get specific permitted namespace",
			args:           []string{"get", "namespaces", testTenant, testNamespace1},
			wantOutContain: testNamespace1,
		},
		{
			name:           "get specific permitted namespace",
			args:           []string{"get", "namespaces", testTenant, testNamespace2},
			wantOutContain: testNamespace2,
		},
		{
			name:           "error: namespace not in tenant allowed list",
			args:           []string{"get", "namespaces", testTenant, "default"},
			wantErr:        true,
			wantErrContain: "not permitted",
		},
		{
			name:           "error: namespace doesn't exist in cluster",
			args:           []string{"get", "namespaces", testTenant, invalidNS},
			wantErr:        true,
			wantErrContain: "not permitted",
		},
		{
			name:           "error: invalid tenant name",
			args:           []string{"get", "namespaces", invalidTenant},
			wantErr:        true,
			wantErrContain: invalidTenant,
		},
		{
			name:           "output format: json",
			args:           []string{"get", "namespaces", testTenant, "-o", "json"},
			wantOutContain: `"kind"`,
		},
		{
			name:           "output format: yaml",
			args:           []string{"get", "namespaces", testTenant, "-o", "yaml"},
			wantOutContain: "kind:",
		},
	}

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

func testGetStorageClasses(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantErr        bool
		wantErrContain string
		wantOutContain string
	}{
		{
			name:           "list permitted storage classes",
			args:           []string{"get", "storageclasses", testTenant},
			wantOutContain: testSC1,
		},
		{
			name:           "get specific permitted storage class",
			args:           []string{"get", "storageclasses", testTenant, testSC1},
			wantOutContain: testSC1,
		},
		{
			name:           "error: storage class not in tenant allowed list",
			args:           []string{"get", "storageclasses", testTenant, forbiddenSC},
			wantErr:        true,
			wantErrContain: "not permitted",
		},
		{
			name:           "error: storage class doesn't exist",
			args:           []string{"get", "storageclasses", testTenant, invalidSC},
			wantErr:        true,
			wantErrContain: "not permitted",
		},
		{
			name:           "error: invalid tenant name",
			args:           []string{"get", "storageclasses", invalidTenant},
			wantErr:        true,
			wantErrContain: invalidTenant,
		},
	}

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
