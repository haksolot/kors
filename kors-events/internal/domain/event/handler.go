package event

import "context"

type Handler interface {
    Handle(ctx context.Context, subject string, payload []byte) error
}