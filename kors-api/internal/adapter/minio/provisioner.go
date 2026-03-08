package minio

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
)

type MinioProvisioner struct {
	Client *minio.Client
}

func (p *MinioProvisioner) ProvisionBucket(ctx context.Context, moduleName string) error {
	cleanName := strings.ReplaceAll(strings.ToLower(moduleName), "_", "-")
	bucketName := fmt.Sprintf("module-%s", cleanName)
	exists, err := p.Client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		err = p.Client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
		}
	}
	return nil
}

func (p *MinioProvisioner) DeprovisionBucket(ctx context.Context, moduleName string, force bool) error {
	cleanName := strings.ReplaceAll(strings.ToLower(moduleName), "_", "-")
	bucketName := fmt.Sprintf("module-%s", cleanName)

	exists, err := p.Client.BucketExists(ctx, bucketName)
	if err != nil || !exists {
		return nil
	}

	if force {
		objectsCh := p.Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
		for object := range objectsCh {
			if object.Err != nil {
				continue
			}
			_ = p.Client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
		}
	}

	if err := p.Client.RemoveBucket(ctx, bucketName); err != nil {
		return fmt.Errorf("bucket %s not deleted (may not be empty): %w", bucketName, err)
	}
	return nil
}
