package korsctx

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const identityKey contextKey = "identity_id"

// FromContext extracts the identity ID from the context.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(identityKey).(uuid.UUID)
	return id, ok
}

// WithIdentity injects an identity ID into the context.
func WithIdentity(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, identityKey, id)
}
