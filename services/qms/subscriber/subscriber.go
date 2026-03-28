package subscriber

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"

	pbmes "github.com/haksolot/kors/proto/gen/mes"
	pbqms "github.com/haksolot/kors/proto/gen/qms"
	"github.com/haksolot/kors/services/qms/domain"
)

// Repository is the subset of persistence needed by the subscriber.
type Repository interface {
	FindNCByOperationID(ctx context.Context, operationID string) (*domain.NonConformity, error)
	domain.Transactor
}

// Subscriber consumes MES events and materialises them in the QMS domain.
type Subscriber struct {
	repo Repository
	nc   *nats.Conn
	log  zerolog.Logger
}

// New returns a ready Subscriber.
func New(repo Repository, nc *nats.Conn, log zerolog.Logger) *Subscriber {
	return &Subscriber{repo: repo, nc: nc, log: log}
}

// Subscribe registers NATS subscriptions and returns the subscriptions so the
// caller can drain them on shutdown.
func (s *Subscriber) Subscribe() ([]*nats.Subscription, error) {
	sub, err := s.nc.QueueSubscribe(
		domain.SubjectMESNCDeclared,
		domain.QueueGroupQMS,
		s.handleNCDeclared,
	)
	if err != nil {
		return nil, err
	}
	return []*nats.Subscription{sub}, nil
}

// handleNCDeclared creates a new NC from a kors.mes.nc.declared event (idempotent).
func (s *Subscriber) handleNCDeclared(msg *nats.Msg) {
	ctx := context.Background()

	var evt pbmes.NCDeclaredEvent
	if err := proto.Unmarshal(msg.Data, &evt); err != nil {
		s.log.Error().Err(err).Msg("QMS subscriber: unmarshal NCDeclaredEvent")
		return
	}

	// Idempotency: if we already have a NC for this operation, skip.
	existing, err := s.repo.FindNCByOperationID(ctx, evt.OperationId)
	if err != nil && !errors.Is(err, domain.ErrNCNotFound) {
		s.log.Error().Err(err).Str("operation_id", evt.OperationId).Msg("QMS subscriber: lookup NC by operation_id")
		return
	}
	if existing != nil {
		s.log.Debug().Str("operation_id", evt.OperationId).Str("nc_id", existing.ID).Msg("QMS subscriber: NC already exists, skipping")
		return
	}

	nc, err := domain.NewNC(
		evt.OperationId,
		evt.OfId,
		evt.DefectCode,
		evt.Description,
		int(evt.AffectedQuantity),
		evt.SerialNumbers,
		evt.DeclaredBy,
	)
	if err != nil {
		s.log.Error().Err(err).Str("operation_id", evt.OperationId).Msg("QMS subscriber: build NC")
		return
	}

	openEvt, err := proto.Marshal(&pbqms.NCOpenedEvent{
		EventId:    evt.EventId,
		NcId:       nc.ID,
		OperationId: evt.OperationId,
		OfId:       evt.OfId,
		DefectCode: evt.DefectCode,
		DeclaredBy: evt.DeclaredBy,
		CreatedAt:  evt.DeclaredAt,
	})
	if err != nil {
		s.log.Error().Err(err).Str("nc_id", nc.ID).Msg("QMS subscriber: marshal NCOpenedEvent")
		return
	}

	if err := s.repo.WithTx(ctx, func(tx domain.TxOps) error {
		if err := tx.SaveNC(ctx, nc); err != nil {
			return err
		}
		return tx.InsertOutbox(ctx, domain.OutboxEntry{
			EventType: "NCOpened",
			Subject:   domain.SubjectNCOpened,
			Payload:   openEvt,
		})
	}); err != nil {
		if errors.Is(err, domain.ErrNCAlreadyExists) {
			// Race: another instance saved it first — safe to ignore.
			s.log.Debug().Str("operation_id", evt.OperationId).Msg("QMS subscriber: NC already exists (race), skipping")
			return
		}
		s.log.Error().Err(err).Str("nc_id", nc.ID).Msg("QMS subscriber: save NC")
		return
	}

	s.log.Info().Str("nc_id", nc.ID).Str("operation_id", evt.OperationId).Msg("QMS NC opened")
}
