package client

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	QuotaGroup   = "tenantoperator.stakater.com"
	QuotaVersion = "v1beta1"
	QuotaKind    = "Quota"
	QuotaPlural  = "quotas"
)

type QuotaClient struct {
	dynClient dynamic.Interface
	gvr       schema.GroupVersionResource
	logger    *zap.Logger
}

func NewQuotaClient(config dynamic.Interface, logger *zap.Logger) *QuotaClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &QuotaClient{
		dynClient: config,
		gvr: schema.GroupVersionResource{
			Group:    QuotaGroup,
			Version:  QuotaVersion,
			Resource: QuotaPlural,
		},
		logger: logger,
	}
}

// GetQuota fetches a Quota CR by name
func (qc *QuotaClient) GetQuota(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	quota, err := qc.dynClient.Resource(qc.gvr).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get quota %q: %w", name, err)
	}
	return quota, nil
}
