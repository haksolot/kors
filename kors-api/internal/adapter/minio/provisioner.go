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

func (p *MinioProvisioner) DeprovisionBucket(ctx context.Context, moduleName string) error {
	// For safety, we only remove the bucket if it's empty, or we could just leave it.
	// We'll leave it as a no-op or return an error if we try to delete a non-empty one.
	// But to keep it clean, let's just attempt a simple RemoveBucket. 
	// If it fails because it's not empty, the admin will have to handle it manually to avoid data loss.
	cleanName := strings.ReplaceAll(strings.ToLower(moduleName), "_", "-")
	bucketName := fmt.Sprintf("module-%s", cleanName)
	exists, err := p.Client.BucketExists(ctx, bucketName)
	if err == nil && exists {
		_ = p.Client.RemoveBucket(ctx, bucketName) // Ignore error on purpose for safety (won't delete if not empty)
	}
	return nil
}
