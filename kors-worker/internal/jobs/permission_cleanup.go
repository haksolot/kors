package jobs

import (
	"context"
	"errors"
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
				if errors.Is(err, postgres.ErrLocked) {
					log.Println("Job skipped: another worker is already cleaning up.")
					continue
				}
				log.Printf("Error cleaning up permissions: %v", err)
				continue
			}
			if count > 0 {
				log.Printf("Successfully removed %d expired permissions", count)
			} else {
				log.Println("No expired permissions found.")
			}
		}
	}
}
