# KORS — Agent Guidelines

This file is the primary context document for AI coding agents (Claude Code, Gemini CLI).
Read this file fully before writing any code. When in doubt, re-read the relevant section.
All decisions made here are backed by ADRs in `docs/adr/`. Reference them by ID.

---

## 0. Before You Write Anything

Run these checks before starting any task:

1. Read the TASK.md file at the repo root if it exists — it defines the current task scope.
2. Read the relevant ADRs for the domain you are working in.
3. Check existing code in the target service before generating new patterns.
4. Never introduce a new dependency without explicit justification in the commit message.
5. If the task touches the MES domain, read `docs/specs/MES_REQUIREMENTS.md` before generating any entity, struct, or migration. The functional requirements defined there are binding.

---

## 1. Architecture Invariants

These are hard constraints. Any code that violates them must be rejected or documented in a new ADR.

### 1.1 Service communication

```
CORRECT:   All inter-service calls use NATS (async pub/sub or request-reply)
CORRECT:   All messages are serialized with Protobuf (schemas in /proto)
WRONG:     Direct HTTP calls between internal services
WRONG:     JSON for inter-service payloads
WRONG:     gRPC — not used in this project (see ADR-006)
```

### 1.2 Event publishing — Transactional Outbox (ADR-004)

Every business event MUST be persisted in the outbox table within the SAME database transaction as the business data. Never publish directly to NATS outside a transaction.

```go
// CORRECT
tx, err := db.Begin(ctx)
// 1. Write business data
_, err = tx.Exec("INSERT INTO manufacturing_orders ...")
// 2. Write to outbox in the SAME transaction
_, err = tx.Exec("INSERT INTO outbox (event_type, payload) VALUES ($1, $2)", "of.created", payload)
tx.Commit()

// WRONG — publishes before data is committed, risks data loss on crash
_, err = db.Exec("INSERT INTO manufacturing_orders ...")
nats.Publish("kors.mes.of.created", payload) // DO NOT DO THIS
```

### 1.3 NATS subject naming convention

Format: `kors.{domain}.{entity}.{past_tense_verb}`

```
kors.mes.of.created
kors.mes.of.completed
kors.mes.of.suspended
kors.mes.operation.started
kors.mes.operation.completed
kors.qms.nc.opened
kors.qms.nc.closed
kors.qms.control.saved
kors.iam.user.created
```

Request-reply subjects (synchronous calls):
```
kors.mes.of.get
kors.mes.of.list
kors.qms.nc.list
```

### 1.4 Always use kors-core-lib

Never implement NATS connection, JWT validation, Protobuf serialization, or logging directly in a service. Import `libs/core` and use its exported functions.

```go
// CORRECT
import "github.com/kors/kors/libs/core"

conn, err := core.NewNATSConn(cfg.NATSURL, cfg.NATSCreds)
claims, err := core.ValidateJWT(token, cfg.JWKSEndpoint)
log := core.NewLogger(traceID, "mes")
ctx, span := core.StartSpan(ctx, "CreateMO")

// WRONG — reimplementing what core already provides
nc, err := nats.Connect(natsURL) // DO NOT DO THIS
```

### 1.5 Protobuf first

Define the .proto schema BEFORE writing any service code. The schema lives in `/proto/{domain}/`. Implement only after the schema is reviewed and committed.

```
/proto/mes/manufacturing_order.proto   ← define first
/proto/mes/events.proto                ← define first
/services/mes/handler.go               ← implement after
```

### 1.6 Stateless services

Services must not hold in-memory state between requests. All state lives in PostgreSQL or NATS JetStream. This enables horizontal scaling and crash recovery.

```go
// WRONG — in-memory cache in a service
var orderCache = map[string]*Order{} // DO NOT DO THIS

// CORRECT — always read from the database
func GetOrder(ctx context.Context, id string) (*Order, error) {
    return repo.FindByID(ctx, id)
}
```

---

## 2. Go Code Conventions

### 2.1 Error handling

Always wrap errors with context. Never swallow errors silently. Never use `_` to ignore an error unless there is a comment explaining why.

```go
// CORRECT
order, err := repo.FindByID(ctx, orderID)
if err != nil {
    return fmt.Errorf("GetOrder: find by id %s: %w", orderID, err)
}

// WRONG — no context, impossible to trace
order, err := repo.FindByID(ctx, orderID)
if err != nil {
    return err
}

// WRONG — silent ignore
order, _ := repo.FindByID(ctx, orderID)
```

Use sentinel errors for expected domain cases:

```go
var ErrOrderNotFound = errors.New("manufacturing order not found")
var ErrOrderAlreadyCompleted = errors.New("manufacturing order already completed")

if errors.Is(err, ErrOrderNotFound) {
    // handle 404
}
```

### 2.2 Package naming

- Short, lowercase, no underscores, no abbreviations unless universally known.
- Package name = what it provides, not what it contains.

```
CORRECT:   mes, qms, iam, outbox, handler, repo, domain
WRONG:     mesService, manufacturing_execution, svc, mgmt
```

### 2.3 Struct and interface design

Keep interfaces small — define them where they are used, not where the implementation lives.

```go
// CORRECT — defined in the package that consumes it
type OrderRepository interface {
    FindByID(ctx context.Context, id string) (*Order, error)
    Save(ctx context.Context, order *Order) error
}

// In the handler, inject the interface, not the concrete type
type Handler struct {
    orders OrderRepository
    events core.EventPublisher
    log    *zerolog.Logger
}
```

### 2.4 Context propagation

Always pass `context.Context` as the first parameter in any function that does I/O (DB, NATS, HTTP). Never store context in a struct.

```go
// CORRECT
func (r *PostgresRepo) FindByID(ctx context.Context, id string) (*Order, error)

// WRONG
type Repo struct {
    ctx context.Context // DO NOT DO THIS
}
```

### 2.5 Configuration

All configuration comes from environment variables. No hardcoded values, no config files at runtime. Use a dedicated `Config` struct populated at startup.

```go
type Config struct {
    NATSUrl      string `env:"NATS_URL,required"`
    NATSCreds    string `env:"NATS_CREDS_PATH,required"`
    DatabaseURL  string `env:"DATABASE_URL,required"`
    JWKSEndpoint string `env:"JWKS_ENDPOINT,required"`
    ServiceName  string `env:"SERVICE_NAME,required"`
}

// WRONG — hardcoded anywhere
nats.Connect("nats://localhost:4222") // DO NOT DO THIS
```

### 2.6 Dependency injection

Inject dependencies at the top level (main or cmd). Never use global variables or singletons.

```go
// CORRECT — in main.go
cfg := loadConfig()
db := connectDB(cfg.DatabaseURL)
nc := core.NewNATSConn(cfg.NATSUrl, cfg.NATSCreds)
repo := mes.NewPostgresRepo(db)
handler := mes.NewHandler(repo, nc, core.NewLogger("mes"))
```

---

## 3. Database Conventions

### 3.1 Migrations (goose)

Every schema change must have a migration file in `services/{service}/migrations/`.
Every migration must have both `-- +goose Up` and `-- +goose Down` sections.
Migrations run automatically at service startup.

```sql
-- +goose Up
CREATE TABLE manufacturing_orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference   TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'planned',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mo_status ON manufacturing_orders(status);

-- +goose Down
DROP TABLE manufacturing_orders;
```

Naming convention for migration files: `{sequence}_{description}.sql`
```
0001_create_manufacturing_orders.sql
0002_add_operator_id_to_operations.sql
0003_create_outbox.sql
```

### 3.2 Outbox table

Every service that publishes events must have an outbox table:

```sql
-- +goose Up
CREATE TABLE outbox (
    id          BIGSERIAL PRIMARY KEY,
    event_type  TEXT NOT NULL,
    payload     BYTEA NOT NULL,          -- Protobuf encoded
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unpublished ON outbox(id) WHERE published_at IS NULL;
```

### 3.3 Naming

- Table names: snake_case, plural (`manufacturing_orders`, not `ManufacturingOrder`)
- Column names: snake_case
- Index names: `idx_{table}_{columns}`
- Foreign key names: `fk_{table}_{referenced_table}`

---

## 4. Testing

### 4.1 Test-first discipline

Write the test BEFORE or simultaneously with the implementation. Never write implementation then tests as an afterthought.

### 4.2 Table-driven tests

Always use table-driven tests for functions with multiple cases. This is non-negotiable.

```go
func TestCreateOrder(t *testing.T) {
    tests := []struct {
        name    string
        input   CreateOrderRequest
        want    *Order
        wantErr error
    }{
        {
            name:  "valid request creates order",
            input: CreateOrderRequest{Reference: "OF-001", Quantity: 10},
            want:  &Order{Reference: "OF-001", Status: "planned"},
        },
        {
            name:    "empty reference returns error",
            input:   CreateOrderRequest{Reference: ""},
            wantErr: ErrInvalidReference,
        },
        {
            name:    "zero quantity returns error",
            input:   CreateOrderRequest{Reference: "OF-001", Quantity: 0},
            wantErr: ErrInvalidQuantity,
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := CreateOrder(context.Background(), tc.input)
            if tc.wantErr != nil {
                require.ErrorIs(t, err, tc.wantErr)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tc.want.Reference, got.Reference)
            assert.Equal(t, tc.want.Status, got.Status)
        })
    }
}
```

### 4.3 Test structure

```
services/mes/
├── domain/
│   ├── order.go
│   └── order_test.go          ← unit tests, no I/O
├── repo/
│   ├── postgres.go
│   └── postgres_test.go       ← integration tests with testcontainers
└── handler/
    ├── handler.go
    └── handler_test.go        ← handler tests with mocked repo
```

### 4.4 Mocking

Use interfaces and manual mocks or `testify/mock`. Never mock the database directly — use testcontainers for integration tests.

```go
// Mock the repository interface, not the database
type MockOrderRepo struct {
    mock.Mock
}

func (m *MockOrderRepo) FindByID(ctx context.Context, id string) (*Order, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*Order), args.Error(1)
}
```

### 4.5 Coverage requirements

- Domain logic (pure functions): 90%+ coverage required
- Repository layer: integration tests with testcontainers, not mocks
- Handler layer: unit tests with mocked dependencies
- No PR is merged with failing tests

---

## 5. Security Rules

### 5.1 Secrets

Absolute prohibitions — these will cause CI to fail:

```
NEVER hardcode any value matching these patterns:
- Passwords, tokens, API keys, private keys
- Connection strings with credentials embedded
- NATS credentials inline in code
- JWT secrets
```

Use environment variables exclusively. For local development, use a `.env` file that is listed in `.gitignore`.

```go
// CORRECT
cfg.DatabaseURL = os.Getenv("DATABASE_URL")

// WRONG — will be caught by secret scanning in CI
db.Connect("postgres://admin:supersecret@localhost/kors") // DO NOT DO THIS
```

### 5.2 Input validation

Validate all inputs at the handler boundary before they reach domain logic. Use explicit validation, not silent coercion.

```go
func (h *Handler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
    if req.Reference == "" {
        return nil, status.Error(codes.InvalidArgument, "reference is required")
    }
    if req.Quantity <= 0 {
        return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
    }
    // proceed to domain logic only after validation
}
```

### 5.3 JWT and authorization

Every endpoint exposed by the BFF must validate the JWT and check the role claims. Never trust client-provided user IDs — always extract them from the validated JWT claims.

```go
// CORRECT
claims, err := core.ValidateJWT(tokenFromHeader, cfg.JWKSEndpoint)
if err != nil {
    return nil, ErrUnauthorized
}
operatorID := claims.Subject // from JWT, not from request

// WRONG — trusting the client
operatorID := req.OperatorID // DO NOT DO THIS
```

### 5.4 NATS subject authorization

NATS credentials are scoped per service. Each service can only publish and subscribe to subjects in its own domain. The NATS server configuration enforces this — do not attempt to publish to another domain's subjects.

### 5.5 Dependency management

Before adding any new Go module dependency:
1. Check if kors-core-lib already provides the functionality
2. Verify the module has an active maintainer and recent commits
3. Pin the exact version in go.mod
4. Document the justification in the commit message

Prohibited dependency categories:
- ORM libraries (use raw SQL with pgx)
- Global state packages (loggers, configs initialized as package-level vars)
- Any package that requires CGO unless absolutely unavoidable

---

## 6. What You Must Never Do

This section lists antipatterns observed in AI-generated code. Each item here exists because an agent did it at some point.

```
NEVER add a TODO comment and leave it — either implement it or create a GitHub issue
NEVER use panic() in a service — return errors instead
NEVER use init() functions — initialize explicitly in main
NEVER use global variables for state — inject dependencies
NEVER write migrations without a Down section
NEVER publish a NATS event outside a database transaction
NEVER add a new service without a corresponding AGENTS.md in that service's directory
NEVER use time.Sleep in production code — use proper retry logic with backoff
NEVER ignore context cancellation — check ctx.Err() in loops
NEVER log sensitive data (tokens, passwords, PII) — log IDs and statuses only
NEVER use fmt.Println in service code — use the structured logger from core
NEVER hardcode subject names as strings in handlers — define them as constants
```

---

## 7. File and Directory Ownership

When modifying shared infrastructure, be aware of what each directory owns:

| Directory | Owner | Notes |
|---|---|---|
| `/libs/core` | Architecture lead | Changes require ADR or explicit approval |
| `/proto` | Architecture lead | Schema changes require version bump |
| `/infra` | Architecture lead | No service-specific logic here |
| `/docs/adr` | Architecture lead | New ADRs welcome, never delete existing ones |
| `/services/{name}` | Service owner | Autonomous within conventions |
| `/frontend` | Frontend lead | No direct NATS or DB access from here |

---

## 8. ADR Quick Reference

| ID | Decision | Status |
|---|---|---|
| ADR-001 | Go as the only backend language | Accepted |
| ADR-002 | NATS as unified bus (async + sync) | Accepted |
| ADR-003 | Monorepo | Accepted |
| ADR-004 | Transactional Outbox Pattern for all events | Accepted |
| ADR-005 | K3s for Edge deployments | Accepted |
| ADR-006 | No gRPC — NATS Request-Reply instead | Accepted |

Full ADR texts: `docs/adr/`

---

## 9. When You Are Unsure

If you are unsure whether a pattern is correct for this codebase:

1. Search for an existing example in the codebase before inventing a new pattern.
2. Check the relevant ADR.
3. If no precedent exists, implement the simplest version and add a comment flagging it for review.
4. Never invent a new architectural pattern without documenting it in a new ADR.

The goal is consistency over cleverness. Code that looks identical across services is a feature, not a lack of creativity.

---

## 10. Functional Specifications

Functional specs define WHAT the system must do. They are as binding as the architecture invariants.
Code that implements a domain entity or workflow without covering the corresponding spec requirement is incomplete, not wrong — but must be flagged with a GitHub issue.

### MES — Manufacturing Execution System

Source of truth: `docs/specs/MES_REQUIREMENTS.md`

This document covers 16 functional domains. Before implementing any MES entity, handler, or migration, identify which section(s) it maps to and list them in the commit message.
```
CORRECT commit:
feat(mes): add WorkOrder aggregate with status machine

Covers: MES_REQUIREMENTS §1 (OF management), §12 (RBAC)
Statuses: planned, launched, in_progress, suspended, completed, closed
Transitions enforced at domain layer, not handler layer.

WRONG commit:
feat(mes): add work order model
```

**Non-negotiable requirements from MES_REQUIREMENTS.md:**

These are the constraints most likely to be missed by an agent working without context. They are listed here as a first-line reminder — read the full spec for detail.
```
TRACEABILITY
- Every produced unit must be linked to: OF, route version, operator per operation,
  workstation per operation, consumed material lots, tools and gauges used, timestamps
- Full genealogy (ascending + descending) must be reconstructable in < 10 seconds
- Audit trail is append-only — never UPDATE or DELETE traceability records

STATUS MACHINES
- WorkOrder statuses: planned → launched → in_progress → suspended → completed → closed
- Suspension requires a mandatory reason field — null is rejected at DB level
- Status transitions are enforced in the domain layer (not the handler, not the DB)

BLOCKING RULES — these are hard blocks, not warnings
- An operator cannot start an operation if their qualification for that operation is expired
- A material lot with status 'blocked' or past its TOE/expiry date cannot be consumed
- A gauge/tool past its calibration date blocks the operation it is assigned to
- A non-conforming lot blocks all downstream operations automatically

OPERATOR INTERFACE
- All operator-facing operations must complete in < 2 seconds response time
- Every critical step must support offline mode — data queued locally, synced on reconnect
- Operator ID is always extracted from the validated JWT — never from the request payload

EVENTS — every state transition publishes an outbox event
- kors.mes.of.created
- kors.mes.of.launched
- kors.mes.of.suspended        (payload must include reason)
- kors.mes.of.completed
- kors.mes.operation.started
- kors.mes.operation.completed
- kors.mes.material.lot.consumed
- kors.mes.nonconformity.raised (triggers automatic lot block)
```

### Adding specs for other domains

When QMS, GMAO, TMS, or other modules are implemented, add their spec files under `docs/specs/` and add a corresponding block in this section following the same structure.