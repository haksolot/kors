package minio

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

type MinioFileStore struct {
	Client        *minio.Client
	DefaultBucket string
}

func (s *MinioFileStore) Upload(ctx context.Context, bucket, path string, content []byte) error {
	reader := bytes.NewReader(content)
	
	// Create bucket if it doesn't exist
	exists, err := s.Client.BucketExists(ctx, bucket)
	if err == nil && !exists {
		_ = s.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}

	_, err = s.Client.PutObject(ctx, bucket, path, reader, int64(len(content)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload to minio: %w", err)
	}
	return nil
}

func (s *MinioFileStore) GetDownloadURL(ctx context.Context, bucket, path string) (string, error) {
	// Generate a pre-signed URL valid for 1 hour
	reqParams := make(url.Values)
	presignedURL, err := s.Client.PresignedGetObject(ctx, bucket, path, time.Hour, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}
	return presignedURL.String(), nil
}
