package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	korsminio "github.com/haksolot/kors/kors-api/internal/adapter/minio"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/shared/korsctx"
)

type UploadFileUseCase struct {
	FileStore    *korsminio.MinioFileStore
	IdentityRepo identity.Repository
}

type UploadFileInput struct {
	FileName    string
	FileContent string // Base64
	ContentType string
}

type UploadFileOutput struct {
	Success  bool
	URL      string
	FilePath string
	Error    string
}

const maxUploadSizeBytes = 50 * 1024 * 1024 // 50 MB

func (uc *UploadFileUseCase) Execute(ctx context.Context, input UploadFileInput) (*UploadFileOutput, error) {
	identityID, ok := korsctx.FromContext(ctx)
	if !ok || identityID == uuid.Nil {
		return &UploadFileOutput{Success: false, Error: "unauthorized"}, nil
	}

	ident, err := uc.IdentityRepo.GetByID(ctx, identityID)
	if err != nil || ident == nil {
		return &UploadFileOutput{Success: false, Error: "identity not found"}, nil
	}

	content, err := base64.StdEncoding.DecodeString(input.FileContent)
	if err != nil {
		return &UploadFileOutput{Success: false, Error: "invalid base64 content"}, nil
	}
	if len(content) > maxUploadSizeBytes {
        return &UploadFileOutput{Success: false, Error: fmt.Sprintf("file too large: max %d MB", maxUploadSizeBytes/1024/1024)}, nil
    }

	// Module bucket name: e.g., "module-<module_name>"
	// Fallback to "kors-files" if identity type is not 'service'
	bucket := "kors-files"
	if ident.Type == "service" {
		// MinIO bucket names must be lowercase, can contain hyphens, but no underscores
		cleanName := strings.ReplaceAll(strings.ToLower(ident.Name), "_", "-")
		bucket = fmt.Sprintf("module-%s", cleanName)
	}

	timestamp := time.Now().Unix()
	filePath := fmt.Sprintf("%d_%s", timestamp, input.FileName)

	err = uc.FileStore.Upload(ctx, bucket, filePath, content)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	url, err := uc.FileStore.GetDownloadURL(ctx, bucket, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get download url: %w", err)
	}

	return &UploadFileOutput{
		Success:  true,
		URL:      url,
		FilePath: filePath,
	}, nil
}
