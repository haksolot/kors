package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/haksolot/kors/kors-worker/internal/adapter/postgres"
	"github.com/rs/zerolog/log"
)

type PermissionCleaner interface {
	CleanupExpired(ctx context.Context) (int64, error)
}

type PermissionCleanupJob struct {
	Repo     PermissionCleaner
	Interval time.Duration
}

func NewPermissionCleanupJob(repo PermissionCleaner) *PermissionCleanupJob {
	return &PermissionCleanupJob{Repo: repo, Interval: 1 * time.Hour}
}

func (j *PermissionCleanupJob) Run(ctx context.Context) error {
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	log.Info().Dur("interval", j.Interval).Msg("Starting Permission Cleanup Job")

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			count, err := j.Repo.CleanupExpired(ctx)
			if err != nil {
				if errors.Is(err, postgres.ErrLocked) {
					log.Info().Msg("Job skipped: another worker is already cleaning up.")
					continue
				}
				log.Error().Err(err).Msg("Error cleaning up permissions")
				return err
			}
			if count > 0 {
				log.Info().Int64("count", count).Msg("Successfully removed expired permissions")
			} else {
				log.Info().Msg("No expired permissions found.")
			}
		}
	}
}
