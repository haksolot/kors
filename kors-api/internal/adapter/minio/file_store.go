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
	Client *minio.Client
	Bucket string
}

func (s *MinioFileStore) Upload(ctx context.Context, path string, content []byte) error {
	reader := bytes.NewReader(content)
	_, err := s.Client.PutObject(ctx, s.Bucket, path, reader, int64(len(content)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to upload to minio: %w", err)
	}
	return nil
}

func (s *MinioFileStore) GetDownloadURL(ctx context.Context, path string) (string, error) {
	// Generate a pre-signed URL valid for 1 hour
	reqParams := make(url.Values)
	presignedURL, err := s.Client.PresignedGetObject(ctx, s.Bucket, path, time.Hour, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}
	return presignedURL.String(), nil
}
