package jobs

import (
	"context"
	"log"
	"time"

	"github.com/safran-ls/kors/kors-worker/internal/adapter/postgres"
)

type PermissionCleanupJob struct {
	Repo     *postgres.PermissionRepository
	Interval time.Duration
}

func (j *PermissionCleanupJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	log.Printf("Starting Permission Cleanup Job (Interval: %v)", j.Interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := j.Repo.CleanupExpired(ctx)
			if err != nil {
				log.Printf("Error cleaning up permissions: %v", err)
				continue
			}
			if count > 0 {
				log.Printf("Successfully removed %d expired permissions", count)
			}
		}
	}
}
