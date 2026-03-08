package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"github.com/haksolot/kors/kors-events/internal/domain/event"
)

type EventConsumer struct {
	JS        nats.JetStreamContext
	EventRepo event.Repository
	Handlers  []event.Handler
}

func (c *EventConsumer) Start(ctx context.Context) error {
	// 1. Create durable consumer (if not exists)
	// Subjects: kors.>
	_, err := c.JS.AddConsumer("KORS", &nats.ConsumerConfig{
		Durable:       "kors-events-consumer",
		AckPolicy:     nats.AckExplicitPolicy,
		FilterSubject: "kors.>",
	})
	if err != nil {
		log.Warn().Err(err).Msg("failed to ensure durable consumer")
	}

	// 2. Subscribe and process
	// With PullSubscribe and the same Durable name, NATS automatically 
	// load balances messages across all instances sharing this durable.
	sub, err := c.JS.PullSubscribe("kors.>", "kors-events-consumer")
	if err != nil {
		return fmt.Errorf("failed to pull subscribe: %w", err)
	}

	log.Info().Msg("kors-events: Balanced consumer started, listening for events...")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msgs, err := sub.Fetch(1, nats.MaxWait(time.Second*5))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				log.Error().Err(err).Msg("Error fetching messages")
				continue
			}

			for _, msg := range msgs {
				c.handleMessage(ctx, msg)
			}
		}
	}
}

func (c *EventConsumer) handleMessage(ctx context.Context, msg *nats.Msg) {
	// 1. Extract Event ID (used as Nats-Msg-Id for idempotency)
	msgID := msg.Header.Get(nats.MsgIdHdr)
	if msgID == "" {
		log.Error().Msg("message without MsgIdHdr, skipping.")
		msg.Term()
		return
	}

	eventID, err := uuid.Parse(msgID)
	if err != nil {
		log.Error().Err(err).Str("msgID", msgID).Msg("Error parsing event ID")
		msg.Term()
		return
	}

	// 2. Check Idempotency (has this event already been processed by kors-api?)
	// In KORS, the API writes to DB AND publishes to NATS.
	// kors-events here is for REAL-TIME reactions (e.g. notifications).
	processed, err := c.EventRepo.IsProcessed(ctx, eventID)
	if err != nil {
		log.Error().Err(err).Msg("Error checking idempotency")
		return // Let NATS retry
	}

	if !processed {
		log.Warn().Str("eventID", eventID.String()).Msg("Event not found in DB, something is wrong (API should write first)")
		// We could wait or handle out-of-sync
	}

	log.Info().Str("eventID", eventID.String()).Str("subject", msg.Subject).Msg("kors-events: Processed event")

	// 3. Acknowledge message
	msg.Ack()

	for _, h := range c.Handlers {
		if err := h.Handle(ctx, msg.Subject, msg.Data); err != nil {
			log.Error().Err(err).Str("subject", msg.Subject).Msg("handler error for subject")
		}
	}
}
