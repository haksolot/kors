package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safran-ls/kors/kors-api/internal/domain/event"
	"github.com/safran-ls/kors/kors-api/internal/domain/resource"
	"github.com/safran-ls/kors/kors-api/internal/domain/revision"
)

type CreateRevisionInput struct {
	ResourceID  uuid.UUID
	IdentityID  uuid.UUID
	FileContent []byte
	FileName    string
}

type CreateRevisionUseCase struct {
	ResourceRepo   resource.Repository
	RevisionRepo   revision.Repository
	FileStore      revision.FileStore
	EventRepo      event.Repository
	EventPublisher event.Publisher
}

func (uc *CreateRevisionUseCase) Execute(ctx context.Context, input CreateRevisionInput) (*revision.Revision, error) {
	// 1. Get current resource state
	res, err := uc.ResourceRepo.GetByID(ctx, input.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("resource not found")
	}

	// 2. Handle File Upload if provided
	var filePath *string
	if len(input.FileContent) > 0 {
		path := fmt.Sprintf("resources/%s/%d_%s", res.ID, time.Now().Unix(), input.FileName)
		if err := uc.FileStore.Upload(ctx, path, input.FileContent); err != nil {
			return nil, fmt.Errorf("failed to upload file: %w", err)
		}
		filePath = &path
	}

	// 3. Create Revision snapshot
	rev := &revision.Revision{
		ID:         uuid.New(),
		ResourceID: res.ID,
		IdentityID: input.IdentityID,
		Snapshot:   res.Metadata, // We snapshot current metadata
		FilePath:   filePath,
		CreatedAt:  time.Now(),
	}

	// 4. Persist Revision
	if err := uc.RevisionRepo.Create(ctx, rev); err != nil {
		return nil, fmt.Errorf("failed to save revision: %w", err)
	}

	// 5. Audit & Event
	ev := &event.Event{
		ID:         uuid.New(),
		ResourceID: &res.ID,
		IdentityID: input.IdentityID,
		Type:       "kors.resource.revision_created",
		Payload: map[string]interface{}{
			"revision_id": rev.ID,
			"has_file":    filePath != nil,
		},
		CreatedAt: time.Now(),
	}
	_ = uc.EventRepo.Create(ctx, ev)
	if uc.EventPublisher != nil {
		_ = uc.EventPublisher.Publish(ctx, ev)
	}

	return rev, nil
}
