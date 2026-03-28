package outbox

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/haksolot/kors/libs/core"
	"github.com/haksolot/kors/services/mes/domain"
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
// It runs as a background goroutine started in cmd/main.go.
// Adaptive polling: 100ms when events are pending, 1s when the table is empty.
type Worker struct {
	repo        Repository
	nc          *nats.Conn
	log         zerolog.Logger
	pendingGauge prometheus.Gauge
}

// New returns a Worker ready to run.
// reg is used to register the kors_outbox_pending_events gauge (ADR-008).
func New(repo Repository, nc *nats.Conn, log zerolog.Logger, reg prometheus.Registerer) *Worker {
	return &Worker{
		repo:         repo,
		nc:           nc,
		log:          log,
		pendingGauge: core.NewGauge(reg, "mes", "outbox_pending_events", "Number of outbox events not yet published to NATS"),
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
// Always call this in a goroutine: go worker.Run(ctx)
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(pollIntervalActive)
	defer ticker.Stop()

	w.log.Info().Msg("outbox worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info().Msg("outbox worker stopped")
			return
		case <-ticker.C:
			count, err := w.processOnce(ctx)
			if err != nil {
				w.log.Error().Err(err).Msg("outbox worker error")
			}
			// Back off when table is empty to reduce DB load.
			if count == 0 {
				ticker.Reset(pollIntervalIdle)
			} else {
				ticker.Reset(pollIntervalActive)
			}
		}
	}
}

// processOnce fetches a batch of unpublished entries, publishes each to NATS,
// and marks them as published. Returns the number of entries processed.
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
			// Log and continue — the entry stays unpublished for the next cycle.
			w.log.Error().Err(err).
				Str("subject", e.Subject).
				Str("event_type", e.EventType).
				Msg("outbox publish failed")
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

	w.log.Debug().
		Int("published", len(publishedIDs)).
		Msg("outbox batch published")

	return len(publishedIDs), nil
}
