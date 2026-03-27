package domain

import "context"

// OrderRepository defines the persistence contract for ManufacturingOrders.
// Defined here (consumed by handler), implemented in repo/.
type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, id string) (*Order, error)
	FindByReference(ctx context.Context, reference string) (*Order, error)
	Update(ctx context.Context, order *Order) error
	List(ctx context.Context, filter ListOrdersFilter) ([]*Order, error)
}

// OperationRepository defines the persistence contract for Operations.
type OperationRepository interface {
	Save(ctx context.Context, op *Operation) error
	FindByID(ctx context.Context, id string) (*Operation, error)
	FindByOFID(ctx context.Context, ofID string) ([]*Operation, error)
	Update(ctx context.Context, op *Operation) error
}

// OutboxRepository defines the persistence contract for the transactional outbox.
// Implementations must write within the caller's transaction (pgx.Tx).
type OutboxRepository interface {
	// InsertTx writes an outbox entry within an existing transaction.
	InsertTx(ctx context.Context, tx TX, entry OutboxEntry) error
	// ListUnpublished returns up to limit unpublished entries ordered by id.
	ListUnpublished(ctx context.Context, limit int) ([]OutboxEntry, error)
	// MarkPublished sets published_at = NOW() for the given entry IDs.
	MarkPublished(ctx context.Context, ids []int64) error
}

// TX is the minimal transaction interface used by OutboxRepository.InsertTx.
// pgx.Tx satisfies this interface.
type TX interface {
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
}

// OutboxEntry holds a single unpublished event from the outbox table.
type OutboxEntry struct {
	ID        int64
	EventType string
	Subject   string
	Payload   []byte
}

// ListOrdersFilter defines optional filtering criteria for Order listings.
type ListOrdersFilter struct {
	Status    *OrderStatus // nil means all statuses
	PageSize  int          // 0 defaults to 50, max 100
	PageToken string       // opaque cursor; empty means first page
}
