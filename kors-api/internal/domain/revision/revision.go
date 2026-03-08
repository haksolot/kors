package revision

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Revision is a versioned snapshot of a resource.
type Revision struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	IdentityID uuid.UUID
	Snapshot   map[string]interface{}
	FilePath   *string // Reference to a file in MinIO (optional)
	CreatedAt  time.Time
}

// Repository defines the contract for persisting revisions.
type Repository interface {
	Create(ctx context.Context, r *Revision) error
	GetByID(ctx context.Context, id uuid.UUID) (*Revision, error)
	ListByResource(ctx context.Context, resourceID uuid.UUID) ([]*Revision, error)
}

// FileStore defines the contract for storing associated files (MinIO).
type FileStore interface {
	Upload(ctx context.Context, bucket string, path string, content []byte) error
	GetDownloadURL(ctx context.Context, bucket string, path string) (string, error)
}
