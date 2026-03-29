// Package qualification contains the background scanner that detects and publishes
// qualification expiry alerts as per AS9100D §7.2.
package qualification

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/haksolot/kors/libs/core"
	pbmes "github.com/haksolot/kors/proto/gen/mes"
	"github.com/haksolot/kors/services/mes/domain"
)

const defaultScanInterval = 1 * time.Hour

// Repository is the persistence contract needed by the Scanner.
type Repository interface {
	// ListExpiringQualifications returns qualifications expiring within warningDays from now.
	ListExpiringQualifications(ctx context.Context, warningDays int, now time.Time) ([]*domain.Qualification, error)
	// InsertOutboxDirect inserts an outbox entry outside a business transaction.
	// Used only by the scanner, which has no associated business mutation.
	InsertOutboxDirect(ctx context.Context, entry domain.OutboxEntry) error
}

// Scanner is a background goroutine that periodically scans for qualifications
// approaching expiry or already expired and publishes the corresponding events
// via the transactional outbox.
type Scanner struct {
	repo         Repository
	warningDays  int
	interval     time.Duration
	log          zerolog.Logger
	alertsTotal  prometheus.Counter
	expiredTotal prometheus.Counter
}

// New returns a Scanner ready to run.
// warningDays: how many days before expiry to start emitting alerts (e.g. 30).
// interval: how often to run the scan (defaults to 1h if zero).
func New(repo Repository, warningDays int, interval time.Duration, log zerolog.Logger, reg prometheus.Registerer) *Scanner {
	if interval <= 0 {
		interval = defaultScanInterval
	}
	alerts := core.NewCounter(reg, "mes", "qualification_expiring_alerts", "Total expiring-soon alerts emitted by the qualification scanner", []string{})
	expired := core.NewCounter(reg, "mes", "qualification_expired_events", "Total expired events emitted by the qualification scanner", []string{})
	return &Scanner{
		repo:         repo,
		warningDays:  warningDays,
		interval:     interval,
		log:          log,
		alertsTotal:  alerts.WithLabelValues(),
		expiredTotal: expired.WithLabelValues(),
	}
}

// Run starts the scanning loop and blocks until ctx is cancelled.
// Always call this in a goroutine: go scanner.Run(ctx)
func (s *Scanner) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info().
		Int("warning_days", s.warningDays).
		Dur("interval", s.interval).
		Msg("qualification scanner started")

	// Run once at startup so we don't wait for the first tick.
	s.scan(ctx)

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("qualification scanner stopped")
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

func (s *Scanner) scan(ctx context.Context) {
	now := time.Now().UTC()
	quals, err := s.repo.ListExpiringQualifications(ctx, s.warningDays, now)
	if err != nil {
		s.log.Error().Err(err).Msg("qualification scanner: list expiring failed")
		return
	}

	for _, q := range quals {
		if q.IsRevoked {
			continue
		}
		if q.ExpiresAt.Before(now) {
			s.publishExpired(ctx, q, now)
		} else {
			s.publishExpiringAlert(ctx, q, now)
		}
	}
}

func (s *Scanner) publishExpiringAlert(ctx context.Context, q *domain.Qualification, now time.Time) {
	daysRemaining := int32(q.ExpiresAt.Sub(now).Hours() / 24)
	evt := &pbmes.QualificationExpiringAlertEvent{
		EventId:         uuid.NewString(),
		QualificationId: q.ID,
		OperatorId:      q.OperatorID,
		Skill:           q.Skill,
		ExpiresAt:       timestamppb.New(q.ExpiresAt),
		DaysRemaining:   daysRemaining,
	}
	payload, err := proto.Marshal(evt)
	if err != nil {
		s.log.Error().Err(err).Str("qualification_id", q.ID).Msg("marshal expiring alert failed")
		return
	}
	entry := domain.OutboxEntry{
		EventType: "QualificationExpiringAlert",
		Subject:   domain.SubjectQualificationExpiringAlert,
		Payload:   payload,
	}
	if err := s.repo.InsertOutboxDirect(ctx, entry); err != nil {
		s.log.Error().Err(err).Str("qualification_id", q.ID).Msg("insert expiring alert outbox failed")
		return
	}
	s.alertsTotal.Inc()
	s.log.Info().
		Str("qualification_id", q.ID).
		Str("operator_id", q.OperatorID).
		Str("skill", q.Skill).
		Int32("days_remaining", daysRemaining).
		Msg("qualification expiring alert emitted")
}

func (s *Scanner) publishExpired(ctx context.Context, q *domain.Qualification, _ time.Time) {
	evt := &pbmes.QualificationExpiredEvent{
		EventId:         uuid.NewString(),
		QualificationId: q.ID,
		OperatorId:      q.OperatorID,
		Skill:           q.Skill,
		ExpiredAt:       timestamppb.New(q.ExpiresAt),
	}
	payload, err := proto.Marshal(evt)
	if err != nil {
		s.log.Error().Err(err).Str("qualification_id", q.ID).Msg("marshal expired event failed")
		return
	}
	entry := domain.OutboxEntry{
		EventType: "QualificationExpired",
		Subject:   domain.SubjectQualificationExpired,
		Payload:   payload,
	}
	if err := s.repo.InsertOutboxDirect(ctx, entry); err != nil {
		s.log.Error().Err(err).Str("qualification_id", q.ID).Msg("insert expired outbox failed")
		return
	}
	s.expiredTotal.Inc()
	s.log.Info().
		Str("qualification_id", q.ID).
		Str("operator_id", q.OperatorID).
		Str("skill", q.Skill).
		Msg("qualification expired event emitted")
}
