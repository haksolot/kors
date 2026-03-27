# MES Service — Agent Guidelines

This file complements the root `AGENT.md` with MES-specific conventions.
Read `AGENT.md` first, then this file before touching any MES code.

---

## 1. Domain Overview

The MES (Manufacturing Execution System) manages the lifecycle of manufacturing
orders (OFs) and their constituent operations.

### Aggregates

| Aggregate | File | Description |
|---|---|---|
| `Order` | `domain/order.go` | Central aggregate. Tracks OF lifecycle. |
| `Operation` | `domain/operation.go` | Single production step within an OF. |

### State Machines

**Order transitions:**
```
PLANNED → IN_PROGRESS → COMPLETED
PLANNED → CANCELLED
IN_PROGRESS → SUSPENDED → IN_PROGRESS  (resume)
IN_PROGRESS → CANCELLED
```

**Operation transitions:**
```
PENDING → IN_PROGRESS → COMPLETED
PENDING → SKIPPED  (requires non-empty reason)
```

---

## 2. NATS Subjects

All subjects are defined as constants in `domain/subjects.go`. Never use raw strings.

### Async events (JetStream)

| Constant | Subject | Published when |
|---|---|---|
| `SubjectOFCreated` | `kors.mes.of.created` | New OF created |
| `SubjectOFStarted` | `kors.mes.of.started` | First operation started |
| `SubjectOFCompleted` | `kors.mes.of.completed` | All operations done |
| `SubjectOFSuspended` | `kors.mes.of.suspended` | OF suspended |
| `SubjectOFCancelled` | `kors.mes.of.cancelled` | OF cancelled |
| `SubjectOperationStarted` | `kors.mes.operation.started` | Operation started |
| `SubjectOperationCompleted` | `kors.mes.operation.completed` | Operation completed |

### Request-Reply (sync)

| Constant | Subject | Action |
|---|---|---|
| `SubjectOFCreate` | `kors.mes.of.create` | Create a new OF |
| `SubjectOFGet` | `kors.mes.of.get` | Fetch a single OF by ID |
| `SubjectOFList` | `kors.mes.of.list` | Paginated list of OFs |
| `SubjectOperationStart` | `kors.mes.operation.start` | Start an operation |
| `SubjectOperationComplete` | `kors.mes.operation.complete` | Complete an operation |

Queue group: `QueueGroupMES = "mes"` — used for all subscriptions.

---

## 3. Protobuf Schemas

Schemas: `proto/mes/` — never edit `proto/gen/` manually.

| Proto file | Key messages |
|---|---|
| `manufacturing_order.proto` | `ManufacturingOrder`, `CreateOrderRequest/Response`, `GetOrderRequest/Response`, `ListOrdersRequest/Response` |
| `operation.proto` | `Operation`, `StartOperationRequest/Response`, `CompleteOperationRequest/Response` |
| `events.proto` | `OFCreatedEvent`, `OFStartedEvent`, `OFCompletedEvent`, `OFSuspendedEvent`, `OFCancelledEvent`, `OperationStartedEvent`, `OperationCompletedEvent` |

Regenerate bindings: `cd proto && buf generate`

---

## 4. Database Schema

Migrations: `services/mes/migrations/`

| Migration | Table | Notes |
|---|---|---|
| `0001` | `manufacturing_orders` | UUID PK, reference UNIQUE, status CHECK constraint |
| `0002` | `operations` | FK → manufacturing_orders, UNIQUE(of_id, step_number) |
| `0003` | `outbox` | Transactional outbox — partial index on `published_at IS NULL` |

---

## 5. Conventions spécifiques MES

- `operator_id` vient **toujours** des claims JWT (`Claims.Subject`) — jamais du corps de la requête.
- Les transitions d'état passent par les méthodes du domaine (`order.Start()`, etc.) — jamais via mutation directe de `Status`.
- Chaque handler qui crée/modifie un OF ou une opération **doit** écrire dans `outbox` dans la même transaction (ADR-004).
- `ErrInvalidProductID` est réutilisé pour les champs FK obligatoires vides (of_id, operator_id) en attendant une refactorisation dédiée.

---

## 6. Tests

```
domain/          → unit tests, 0 I/O, 90%+ coverage obligatoire
repo/            → integration tests avec testcontainers (PostgreSQL)
handler/         → unit tests avec mock repo (testify/mock)
```

Lancer les tests domain : `go test ./services/mes/domain/... -cover`
