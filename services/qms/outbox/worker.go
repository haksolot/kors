package outbox

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/haksolot/kors/libs/core"
	"github.com/haksolot/kors/services/qms/domain"
)

const (
	pollIntervalActive = 100 * time.Millisecond
	pollIntervalIdle   = 1 * time.Second
	batchSize          = 100
)

// Repository is the minimal outbox persistence interface needed by the Worker.
type Repository interface {
	ListUnpublishedOutbox(ctx context.Context, limit int) ([]domain.OutboxEntry, error)
	MarkOutboxPublished(ctx context.Context, ids []int64) error
}

// Worker polls the outbox table and publishes pending events to NATS JetStream.
type Worker struct {
	repo         Repository
	nc           *nats.Conn
	log          zerolog.Logger
	pendingGauge prometheus.Gauge
}

// New returns a Worker ready to run.
func New(repo Repository, nc *nats.Conn, log zerolog.Logger, reg prometheus.Registerer) *Worker {
	return &Worker{
		repo:         repo,
		nc:           nc,
		log:          log,
		pendingGauge: core.NewGauge(reg, "qms", "outbox_pending_events", "Number of QMS outbox events not yet published to NATS"),
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollIntervalActive)
	defer ticker.Stop()

	w.log.Info().Msg("QMS outbox worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("QMS outbox worker stopped")
			return
		case <-ticker.C:
			count, err := w.processOnce(ctx)
			if err != nil {
				w.log.Error().Err(err).Msg("QMS outbox worker error")
			}
			if count == 0 {
				ticker.Reset(pollIntervalIdle)
			} else {
				ticker.Reset(pollIntervalActive)
			}
		}
	}
}

func (w *Worker) processOnce(ctx context.Context) (int, error) {
	entries, err := w.repo.ListUnpublishedOutbox(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	w.pendingGauge.Set(float64(len(entries)))
	if len(entries) == 0 {
		return 0, nil
	}

	var publishedIDs []int64
	for _, e := range entries {
		if err := w.nc.Publish(e.Subject, e.Payload); err != nil {
			w.log.Error().Err(err).
				Str("subject", e.Subject).
				Str("event_type", e.EventType).
				Msg("QMS outbox publish failed")
			continue
		}
		publishedIDs = append(publishedIDs, e.ID)
	}

	if len(publishedIDs) == 0 {
		return 0, nil
	}

	if err := w.repo.MarkOutboxPublished(ctx, publishedIDs); err != nil {
		return 0, err
	}

	w.log.Debug().Int("published", len(publishedIDs)).Msg("QMS outbox batch published")
	return len(publishedIDs), nil
}
