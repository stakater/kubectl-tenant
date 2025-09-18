// test/unit/client/mocks/mock_dynamic.go
package mocks

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type MockDynamicClient struct {
	dynamic.Interface
}

func (m *MockDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return nil
}

type MockNamespaceableResourceInterface struct {
	dynamic.NamespaceableResourceInterface
}

func (m *MockNamespaceableResourceInterface) Namespace(string) dynamic.ResourceInterface {
	return nil
}

type MockResourceInterface struct {
	dynamic.ResourceInterface
}

func (m *MockResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, nil
}

func (m *MockResourceInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}
