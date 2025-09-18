package client

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type StorageClassClient struct {
	client *kubernetes.Clientset
	logger *zap.Logger
}

func NewStorageClassClient(cfg *rest.Config, logger *zap.Logger) (*StorageClassClient, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &StorageClassClient{
		client: clientset,
		logger: logger,
	}, nil
}

// GetStorageClassesByNames fetches StorageClasses by name list
func (sc *StorageClassClient) GetStorageClassesByNames(ctx context.Context, names []string) (*storagev1.StorageClassList, error) {
	if len(names) == 0 {
		return &storagev1.StorageClassList{}, nil
	}

	var items []storagev1.StorageClass

	for _, name := range names {
		storageClass, err := sc.client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			sc.logger.Warn("StorageClass not found, skipping", zap.String("name", name))
			continue // Skip missing, don't fail
		}
		items = append(items, *storageClass)
	}

	// Sort for stable output
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return &storagev1.StorageClassList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "StorageClassList",
		},
		Items: items,
	}, nil
}
