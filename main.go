package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/get"
)

const (
	PluginName = "kubectl-tenant"
)

type getOptions struct {
	resource               schema.GroupVersionResource
	listKind               string
	extractTenantResources func(*unstructured.Unstructured) []string
}

var ClusterResources = map[string]getOptions{
	"storageclasses": {
		resource: schema.GroupVersionResource{
			Group:    "storage.k8s.io",
			Version:  "v1",
			Resource: "storageclasses",
		},
		listKind:               "StorageClassList",
		extractTenantResources: extractStorageClassNames,
	},
	"namespaces": {
		resource: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "namespaces",
		},
		listKind:               "NamespaceList",
		extractTenantResources: extractNamespaceNames,
	},
	"ingressclasses": {
		resource: schema.GroupVersionResource{
			Group:    "networking.k8s.io",
			Version:  "v1",
			Resource: "ingressclasses",
		},
		listKind:               "IngressClassList",
		extractTenantResources: extractIngressClassNames,
	},
	"priorityclasses": {
		resource: schema.GroupVersionResource{
			Group:    "scheduling.k8s.io",
			Version:  "v1",
			Resource: "priorityclasses",
		},
		listKind:               "PriorityClassList",
		extractTenantResources: extractPodPriorityClassNames,
	},
	"quotas": {
		resource: schema.GroupVersionResource{
			Group:    "tenantoperator.stakater.com",
			Version:  "v1beta1",
			Resource: "quotas",
		},
		listKind:               "QuotaList",
		extractTenantResources: extractQuotaNames,
	},
}

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		// Match kubectl's non-verbose error reporting style
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	flags := genericclioptions.NewConfigFlags(true)
	ioStreams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	root := &cobra.Command{
		Use:   "tenant",
		Short: "Tenant-related helpers for kubectl",
		Long: `kubectl-tenant extends kubectl with tenant-scoped resource operations.

It works with Stakater's Multi Tenant Operator to provide filtered views of 
cluster-scoped resources based on tenant permissions.`,
	}

	getCmd := newGetCmd(flags, ioStreams)
	listCmd := newListCmd(flags, ioStreams)
	docsCmd := newDocsCmd(root)

	flags.AddFlags(root.PersistentFlags())
	root.AddCommand(getCmd)
	root.AddCommand(listCmd)
	root.AddCommand(docsCmd)
	return root
}

func newGetCmd(configFlags *genericclioptions.ConfigFlags, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get tenant-scoped resources",
		Long: `Get cluster-scoped Kubernetes resources filtered by tenant permissions.

This behaves like 'kubectl get <resource>', but filtered to those resources
permitted for the specified tenant according to the Tenant CR status.`,
	}

	for resourceName, opts := range ClusterResources {
		cmd.AddCommand(newGetResourceCmd(resourceName, opts, configFlags, ioStreams))
	}

	return cmd
}

func newDocsCmd(root *cobra.Command) *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate documentation for kubectl-tenant",
		Long:   `Generate Markdown documentation for all kubectl-tenant commands.`,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			if err := doc.GenMarkdownTree(root, outputDir); err != nil {
				return fmt.Errorf("failed to generate docs: %w", err)
			}

			absPath, _ := filepath.Abs(outputDir)
			fmt.Printf("Documentation generated successfully in: %s\n", absPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "./docs", "Output directory for generated documentation")

	return cmd
}

func newGetResourceCmd(resourceName string, opts getOptions, configFlags *genericclioptions.ConfigFlags,
	ioStreams genericiooptions.IOStreams) *cobra.Command {
	printFlags := get.NewGetPrintFlags()

	cmd := &cobra.Command{
		Use:   resourceName + " <tenant> [resource-name]",
		Short: fmt.Sprintf("List %s permitted for a Tenant", resourceName),
		Long: fmt.Sprintf(`List %s permitted for a Tenant.

This behaves like 'kubectl get %s', but filtered to those listed in
the Tenant CR status (tenant.tenantoperator.stakater.com).

When a specific resource name is provided, the command validates tenant access
and passes through to kubectl for native output.`, resourceName, resourceName),
		Example: fmt.Sprintf(`  # List %s for my-tenant
  kubectl tenant get %s my-tenant

  # Get a specific %s
  kubectl tenant get %s my-tenant specific-resource`,
			resourceName, resourceName, resourceName, resourceName),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenantName := args[0]

			cfg, err := configFlags.ToRESTConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// If a specific resource name is provided, validate and get it
			if len(args) > 1 {
				resourceToGet := args[1]
				return handleSpecificResource(ctx, cfg, tenantName, resourceName, resourceToGet, opts, printFlags, ioStreams)
			}

			return listResources(ctx, cfg, tenantName, opts, printFlags, ioStreams)
		},
	}

	printFlags.AddFlags(cmd)
	return cmd
}

func handleSpecificResource(
	ctx context.Context,
	cfg *rest.Config,
	tenantName string,
	resourceType string,
	resourceName string,
	opts getOptions,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	tenantGVR := schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com",
		Version:  "v1beta3",
		Resource: "tenants",
	}

	tenant, err := dyn.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	allowedResources := opts.extractTenantResources(tenant)

	allowed := false
	for _, name := range allowedResources {
		if name == resourceName {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("%s %q is not permitted for tenant %q", resourceType, resourceName, tenantName)
	}

	obj, err := dyn.Resource(opts.resource).Get(ctx, resourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}
	return p.PrintObj(obj, ioStreams.Out)
}

func listResources(
	ctx context.Context,
	cfg *rest.Config,
	tenantName string,
	opts getOptions,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	// dynamic client to read the Tenant (unstructured)
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	tenantGVR := schema.GroupVersionResource{
		Group:    "tenantoperator.stakater.com", // CRD group
		Version:  "v1beta3",                     // adjust if your CRD version differs
		Resource: "tenants",
	}

	var tenant *unstructured.Unstructured
	tenant, err = dyn.Resource(tenantGVR).Get(ctx, tenantName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get tenant %q: %w", tenantName, err)
	}

	names := opts.extractTenantResources(tenant)
	if len(names) == 0 {
		return printResourceList(opts, []*unstructured.Unstructured{}, printFlags, ioStreams)
	}

	items := make([]*unstructured.Unstructured, 0, len(names))
	for _, name := range names {
		obj, err := dyn.Resource(opts.resource).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// If a name listed in the Tenant doesn't exist, skip it but keep going
			continue
		}
		items = append(items, obj)
	}
	// Sort by name to keep stable output (like kubectl)
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})

	return printResourceList(opts, items, printFlags, ioStreams)
}

func extractAvailableNames(u *unstructured.Unstructured, statusField string) []string {
	list, found, err := unstructured.NestedSlice(u.Object, "status", statusField, "available")
	if err != nil || !found {
		return nil
	}

	seen := map[string]struct{}{}
	var out []string

	for _, entry := range list {
		e, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		n, ok := e["name"].(string)
		if !ok || strings.TrimSpace(n) == "" {
			continue
		}
		if _, dup := seen[n]; !dup {
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func extractStorageClassNames(u *unstructured.Unstructured) []string {
	return extractAvailableNames(u, "storageClasses")
}

func extractIngressClassNames(u *unstructured.Unstructured) []string {
	return extractAvailableNames(u, "ingressClasses")
}

func extractPodPriorityClassNames(u *unstructured.Unstructured) []string {
	return extractAvailableNames(u, "podPriorityClasses")
}

func extractQuotaNames(u *unstructured.Unstructured) []string {
	return extractAvailableNames(u, "quota")
}

func extractNamespaceNames(u *unstructured.Unstructured) []string {
	seen := map[string]struct{}{}
	var out []string

	deployedNs, found, err := unstructured.NestedStringSlice(u.Object, "status", "deployedNamespaces")
	if err == nil && found {
		for _, ns := range deployedNs {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				if _, dup := seen[ns]; !dup {
					seen[ns] = struct{}{}
					out = append(out, ns)
				}
			}
		}
	}

	sandboxes, found, err := unstructured.NestedMap(u.Object, "status", "deployedSandboxes")
	if err == nil && found {
		for _, val := range sandboxes {
			ns, ok := val.(string)
			if !ok {
				continue
			}
			ns = strings.TrimSpace(ns)
			if ns != "" {
				if _, dup := seen[ns]; !dup {
					seen[ns] = struct{}{}
					out = append(out, ns)
				}
			}
		}
	}

	sort.Strings(out)
	return out
}

func newListCmd(configFlags *genericclioptions.ConfigFlags, ioStreams genericiooptions.IOStreams) *cobra.Command {
	var operatorNamespace string
	var operatorService string
	var operatorPort string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tenants for the current user",
		Long: `List all tenants that the current user has access to.

This command calls the tenant-operator API to retrieve the list of tenants
where the current user appears as an owner, editor, or viewer.`,
		Example: `  # List tenants for the current user
  kubectl tenant list

  # List tenants with custom operator namespace
  kubectl tenant list --operator-namespace my-namespace`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configFlags.ToRESTConfig()
			if err != nil {
				return err
			}
			return listUserTenants(cmd.Context(), cfg, operatorNamespace, operatorService, operatorPort, ioStreams)
		},
	}

	cmd.Flags().StringVar(&operatorNamespace, "operator-namespace", "multi-tenant-operator", "Namespace where tenant-operator is deployed")
	cmd.Flags().StringVar(&operatorService, "operator-service", "tenant-operator-api", "Name of the tenant-operator API service")
	cmd.Flags().StringVar(&operatorPort, "operator-port", "8080", "Port of the tenant-operator API service")

	return cmd
}

// extractBearerToken gets the bearer token from a REST config.
// Supports static tokens, token files, and exec/OIDC-based credentials.
func extractBearerToken(cfg *rest.Config) (string, error) {
	if cfg.BearerToken != "" {
		return cfg.BearerToken, nil
	}

	if cfg.BearerTokenFile != "" {
		data, err := os.ReadFile(cfg.BearerTokenFile)
		if err != nil {
			return "", fmt.Errorf("reading bearer token file: %w", err)
		}
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token, nil
		}
	}

	// For exec/OIDC: make a lightweight API call and capture the token from the Authorization header.
	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return "", fmt.Errorf("creating transport: %w", err)
	}

	var captured string
	capturingTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			captured = strings.TrimPrefix(auth, "Bearer ")
		}
		return transport.RoundTrip(req)
	})

	url := strings.TrimRight(cfg.Host, "/") + "/version"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Transport: capturingTransport}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("probing API server for token: %w", err)
	}
	resp.Body.Close()

	if captured == "" {
		return "", fmt.Errorf("could not extract bearer token from kubeconfig credentials")
	}
	return captured, nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func listUserTenants(
	ctx context.Context,
	cfg *rest.Config,
	namespace, service, port string,
	ioStreams genericiooptions.IOStreams,
) error {
	token, err := extractBearerToken(cfg)
	if err != nil {
		return fmt.Errorf("failed to extract bearer token: %w", err)
	}

	proxyPath := fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s:%s/proxy/api/v1/tenants",
		namespace, service, port,
	)

	transport, err := rest.TransportFor(cfg)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	bodyBytes, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := strings.TrimRight(cfg.Host, "/") + proxyPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call tenant-operator API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tenant-operator API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Tenants []struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"tenants"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Tenants) == 0 {
		fmt.Fprintln(ioStreams.Out, "No tenants found for the current user.")
		return nil
	}

	w := tabwriter.NewWriter(ioStreams.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tROLE")
	for _, t := range result.Tenants {
		fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Role)
	}
	return w.Flush()
}

func printResourceList(
	opts getOptions,
	items []*unstructured.Unstructured,
	printFlags *get.PrintFlags,
	ioStreams genericiooptions.IOStreams,
) error {
	p, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": opts.resource.GroupVersion().String(),
			"kind":       opts.listKind,
		},
	}

	for _, item := range items {
		list.Items = append(list.Items, *item)
	}

	return p.PrintObj(list, ioStreams.Out)
}
