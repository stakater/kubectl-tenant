// test/unit/client/mocks/mock_dynamic.go
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type MockDynamicClient struct {
	mock.Mock
	dynamic.Interface
}

func (m *MockDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	args := m.Called(gvr)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

type MockNamespaceableResourceInterface struct {
	mock.Mock
	dynamic.NamespaceableResourceInterface
}

func (m *MockNamespaceableResourceInterface) Namespace(string) dynamic.ResourceInterface {
	args := m.Called()
	return args.Get(0).(dynamic.ResourceInterface)
}

type MockResourceInterface struct {
	mock.Mock
	dynamic.ResourceInterface
}

func (m *MockResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.UnstructuredList), args.Error(1)
}

func (m *MockResourceInterface) Get(ctx context.Context, name string, opts metav1.ListOptions) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}
