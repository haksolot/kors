# ADR-004 — Transactional Outbox Pattern for All Business Events

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | All services that publish NATS events: `services/mes/`, `services/qms/` |

---

## Context

In an event-driven architecture, publishing a message to NATS after committing a database transaction creates a data loss window: if the service crashes between the commit and the publish, the event is permanently lost. Publishing before the commit can create orphan events if the transaction is rolled back.

In KORS's industrial context (EN9100 traceability, non-conformity tracking), any data loss is unacceptable. A missing `of.completed` event means a manufacturing order appears open in the system when it is actually finished. A missing `nc.opened` event means a defective lot is not blocked.

The alternatives considered were:

- **Publish directly to NATS after DB commit** : simple, but exposes the data loss window described above.
- **Two-phase commit (XA transactions)** : distributes the transaction across DB and NATS. Unsupported by NATS natively. Complex, slow, no real implementation available for Go + NATS.
- **Change Data Capture (CDC) with Debezium** : captures PostgreSQL WAL changes and publishes to a broker. Works, but requires Kafka or Kafka Connect, which violates ADR-002.
- **Transactional Outbox Pattern** : persist the event in the same DB transaction as the business data, then publish asynchronously. Simple, reliable, fully compatible with NATS and Go.

## Decision

**Every business event is persisted in an `outbox` table within the SAME PostgreSQL transaction as the business data.** An asynchronous worker (goroutine) polls the outbox table, publishes to NATS JetStream, and marks the event as delivered after receiving the NATS ACK.

## Outbox Table Schema

Each service that publishes events must have its own outbox table. The migration is mandatory before any handler is implemented.

```sql
-- +goose Up
CREATE TABLE outbox (
    id           BIGSERIAL PRIMARY KEY,
    event_type   TEXT NOT NULL,          -- human-readable type, e.g. "of.created"
    subject      TEXT NOT NULL,          -- NATS target subject, e.g. "kors.mes.of.created"
    payload      BYTEA NOT NULL,         -- Protobuf-encoded message
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ             -- NULL = not yet published
);

-- Partial index for efficient polling of unpublished events
CREATE INDEX idx_outbox_unpublished ON outbox(id)
    WHERE published_at IS NULL;

-- +goose Down
DROP INDEX idx_outbox_unpublished;
DROP TABLE outbox;
```

## Publication Flow

```
1. Handler receives a request (e.g.: create a manufacturing order)
2. BEGIN TRANSACTION (PostgreSQL)
   a. INSERT INTO manufacturing_orders (id, reference, status, ...)
   b. INSERT INTO outbox (event_type, subject, payload)
      VALUES ('of.created', 'kors.mes.of.created', <protobuf encoded>)
3. COMMIT — both insertions are atomic
   → If crash here: both are in DB, outbox worker will publish on restart
   → If NATS unavailable: outbox worker retries with backoff
4. [Outbox Worker — separate goroutine in /services/{name}/outbox/worker.go]
   a. SELECT id, subject, payload FROM outbox
      WHERE published_at IS NULL
      ORDER BY id
      LIMIT 100
   b. For each message: publish to NATS JetStream
   c. If NATS ACK received: UPDATE outbox SET published_at = NOW() WHERE id = $1
   d. If no ACK: retry with exponential backoff (1s, 2s, 4s, 8s, max 30s)
   e. Poll interval: 100ms when events pending, 1s when table empty
```

## Go Implementation Pattern

```go
// CORRECT — in handler: both DB write and outbox in one transaction
func (h *Handler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) error {
    ctx, span := core.StartSpan(ctx, "CreateOrder")
    defer span.End()

    // Encode the event payload
    event := &pb.OFCreatedEvent{
        EventId:   uuid.NewString(),
        OfId:      req.Id,
        Reference: req.Reference,
        CreatedAt: timestamppb.Now(),
    }
    payload, err := proto.Marshal(event)
    if err != nil {
        return fmt.Errorf("CreateOrder: marshal event: %w", err)
    }

    // Single transaction: business data + outbox
    tx, err := h.db.Begin(ctx)
    if err != nil {
        return fmt.Errorf("CreateOrder: begin tx: %w", err)
    }
    defer tx.Rollback(ctx)

    _, err = tx.Exec(ctx,
        "INSERT INTO manufacturing_orders (id, reference, status) VALUES ($1, $2, $3)",
        req.Id, req.Reference, "planned",
    )
    if err != nil {
        return fmt.Errorf("CreateOrder: insert order: %w", err)
    }

    _, err = tx.Exec(ctx,
        "INSERT INTO outbox (event_type, subject, payload) VALUES ($1, $2, $3)",
        "of.created", domain.SubjectOFCreated, payload,
    )
    if err != nil {
        return fmt.Errorf("CreateOrder: insert outbox: %w", err)
    }

    return tx.Commit(ctx)
}

// WRONG — publish directly after commit, risk of data loss
func (h *Handler) CreateOrderWrong(ctx context.Context, req *pb.CreateOrderRequest) error {
    _, err = h.db.Exec(ctx, "INSERT INTO manufacturing_orders ...")
    if err != nil { return err }
    // CRASH HERE = event lost permanently
    return h.nats.Publish(domain.SubjectOFCreated, payload) // DO NOT DO THIS
}
```

## Outbox Worker Pattern

```go
// /services/{name}/outbox/worker.go
type Worker struct {
    db   *pgxpool.Pool
    nats *nats.Conn
    log  *zerolog.Logger
}

func (w *Worker) Run(ctx context.Context) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            count, err := w.processOutbox(ctx)
            if err != nil {
                w.log.Error().Err(err).Msg("outbox worker error")
            }
            // Back off when table is empty
            if count == 0 {
                ticker.Reset(1 * time.Second)
            } else {
                ticker.Reset(100 * time.Millisecond)
            }
        }
    }
}

func (w *Worker) processOutbox(ctx context.Context) (int, error) {
    rows, err := w.db.Query(ctx,
        "SELECT id, subject, payload FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT 100",
    )
    // ... publish each, update published_at on ACK
}
```

## Consumer Idempotency

Because NATS JetStream guarantees At-Least-Once (not Exactly-Once), event consumers must be idempotent. A consumer should check whether the event was already processed before applying the mutation.

```go
// Pattern: idempotent consumer
func (h *Handler) HandleOFCreated(msg *nats.Msg) {
    var event pb.OFCreatedEvent
    if err := proto.Unmarshal(msg.Data, &event); err != nil {
        msg.Nak()
        return
    }

    // Check idempotency using event_id
    already, _ := h.repo.EventAlreadyProcessed(ctx, event.EventId)
    if already {
        msg.Ack() // already processed, acknowledge to prevent redelivery
        return
    }

    if err := h.repo.ProcessEvent(ctx, &event); err != nil {
        msg.Nak() // trigger redelivery
        return
    }
    msg.Ack()
}
```

## Monitoring

The outbox queue depth is a critical metric. A growing outbox indicates NATS connectivity issues or worker failures.

```go
// Expose as Prometheus gauge in kors-core-lib
// kors_outbox_pending_events{service="mes"} 0
```

Alert if `kors_outbox_pending_events > 1000` for more than 5 minutes.

## Consequences

**Positive:**
- Zero data loss on service crash, network failure, or restart.
- Events are always consistent with the database state.
- The outbox serves as an audit log of business mutations.
- The worker can be paused for maintenance without losing events.

**Negative / constraints:**
- Each service needs its own outbox table (lightweight, ~a few thousand rows in normal operation).
- The outbox worker is an additional component to test and monitor.
- Consumers must be idempotent — design requirement to document for each consumer.
- Slightly increased latency: the event is published a few milliseconds after the commit (worker polling). Acceptable for all KORS use cases.

## Rules for Agents

```
NEVER: publish a NATS event outside a database transaction
NEVER: publish directly from the handler without going through the outbox
NEVER: omit the Down section from outbox migrations
ALWAYS: every service that publishes events has an outbox table (migration 0003_create_outbox.sql)
ALWAYS: every event consumer is idempotent — check event_id before applying mutation
ALWAYS: the outbox worker exposes kors_outbox_pending_events as a Prometheus gauge
ALWAYS: outbox worker is started in cmd/main.go as a goroutine alongside the NATS handlers
```

## Related ADRs

- ADR-002: NATS (outbox publishes to NATS JetStream)
- ADR-003: Monorepo (outbox/ directory per service)
- ADR-007: Polyglot persistence (PostgreSQL hosts the outbox table)
- ADR-008: Observability (outbox queue depth as a Prometheus metric)
