package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/haksolot/kors/kors-api/internal/domain/event"
)

type NatsPublisher struct {
	JS nats.JetStreamContext
}

func (p *NatsPublisher) Publish(ctx context.Context, e *event.Event) error {
	// 1. Serialize payload to JSON
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("failed to marshal event for NATS: %w", err)
	}

	// 2. Publish message on NATS (using event type as subject)
	// Subject format: kors.resource.created, kors.resource.state_changed, etc.
	_, err = p.JS.Publish(e.Type, data, nats.MsgId(e.ID.String()))
	if err != nil {
		return fmt.Errorf("failed to publish message on NATS: %w", err)
	}

	return nil
}
