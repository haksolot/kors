package domain

import "context"

// NCRepository defines read-only persistence for NonConformities.
type NCRepository interface {
	FindNCByID(ctx context.Context, id string) (*NonConformity, error)
	FindNCByOperationID(ctx context.Context, operationID string) (*NonConformity, error)
	ListNCs(ctx context.Context, filter ListNCsFilter) ([]*NonConformity, error)
}

// CAPARepository defines read-only persistence for CAPAs.
type CAPARepository interface {
	FindCAPAByID(ctx context.Context, id string) (*CAPA, error)
	ListCAPAs(ctx context.Context, filter ListCAPAsFilter) ([]*CAPA, error)
}

// TxOps defines all write operations available within a database transaction.
// Every mutation that triggers a domain event must use TxOps so the outbox entry
// is written in the same transaction as the business data (ADR-004).
type TxOps interface {
	SaveNC(ctx context.Context, nc *NonConformity) error
	UpdateNC(ctx context.Context, nc *NonConformity) error
	SaveCAPA(ctx context.Context, c *CAPA) error
	UpdateCAPA(ctx context.Context, c *CAPA) error
	InsertOutbox(ctx context.Context, entry OutboxEntry) error
}

// Transactor manages database transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(TxOps) error) error
}

// OutboxEntry holds a single unpublished event from the outbox table.
type OutboxEntry struct {
	ID        int64
	EventType string
	Subject   string
	Payload   []byte
}

// ListNCsFilter defines optional filtering for NC listings.
type ListNCsFilter struct {
	Status   *NCStatus
	PageSize int
}

// ListCAPAsFilter defines optional filtering for CAPA listings.
type ListCAPAsFilter struct {
	NCID     string
	Status   *CAPAStatus
	PageSize int
}
