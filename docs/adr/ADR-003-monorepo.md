# ADR-003 — Monorepo

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | Repository structure, CI/CD, dependency management |

---

## Context

With a two-person team using AI coding agents, managing dependencies between multiple Git repositories creates high operational friction: versioning `kors-core-lib` in each repo, risk of Protobuf contract incompatibilities between services, and fragmented context for agents that see only one repo at a time.

The alternatives considered were:

- **Multi-repo (one repo per service)** : each service has its own Git history, CI pipeline, and release cycle. Standard at large organizations with separate teams per service. For a 2-person team with heavy agent usage, this creates: `kors-core-lib` version management across all repos, Protobuf schema drift between services, AGENTS.md duplication, and agents losing global context between sessions.
- **Monorepo** : single repository for all services, shared library, schemas, infrastructure, and documentation. Coherence at the cost of a growing repository.

## Decision

**A single Git repository contains all services, the core library, Protobuf schemas, infrastructure, and documentation.**

CI/CD pipelines rebuild only modified components using path-based change detection (`git diff --name-only`).

## Repository Structure

This structure is canonical. AI agents must place files exactly here. Any deviation requires a new ADR.

```
kors/
├── AGENTS.md                    # AI agent conventions — read at every session
├── CONTRIBUTING.md              # Git workflow, branch naming, commit convention, PR template
├── go.work                      # Go workspace — links all modules
├── go.work.sum
├── .gitignore                   # Go binaries, .env, node_modules, TASK.md
│
├── proto/                       # Protobuf schemas — source of truth for all contracts
│   ├── buf.yaml                 # buf CLI configuration
│   ├── buf.gen.yaml             # Go binding generation config
│   ├── buf.lock
│   ├── mes/
│   │   ├── manufacturing_order.proto
│   │   ├── operation.proto
│   │   ├── events.proto
│   │   └── traceability.proto
│   ├── qms/
│   │   ├── control.proto
│   │   ├── non_conformity.proto
│   │   └── events.proto
│   ├── iam/
│   │   └── user.proto
│   └── gen/                     # Generated Go bindings — DO NOT EDIT MANUALLY
│       ├── mes/
│       └── qms/
│
├── libs/
│   └── core/                    # kors-core-lib — imported by all services
│       ├── go.mod
│       ├── nats.go              # NATS connection + publish/subscribe helpers
│       ├── jwt.go               # JWT validation via JWKS
│       ├── proto.go             # Protobuf encode/decode helpers
│       ├── logger.go            # zerolog structured logger
│       ├── tracing.go           # OpenTelemetry span + TraceID propagation
│       ├── config.go            # Common config struct helpers
│       └── *_test.go
│
├── services/
│   ├── mes/                     # Manufacturing Execution System
│   │   ├── go.mod
│   │   ├── cmd/
│   │   │   └── main.go          # Entrypoint — wires all dependencies
│   │   ├── domain/
│   │   │   ├── order.go         # ManufacturingOrder struct, business logic, validation
│   │   │   ├── operation.go     # Operation struct, state transitions
│   │   │   ├── genealogy.go     # ProductGenealogy, lot traceability
│   │   │   ├── subjects.go      # NATS subject constants
│   │   │   └── *_test.go        # Unit tests — no I/O
│   │   ├── repo/
│   │   │   ├── postgres.go      # Implements domain interfaces with pgx
│   │   │   └── postgres_test.go # Integration tests with testcontainers
│   │   ├── handler/
│   │   │   ├── handler.go       # NATS handlers — orchestrates domain + repo
│   │   │   └── handler_test.go  # Unit tests with mocked repo
│   │   ├── outbox/
│   │   │   └── worker.go        # Outbox polling worker goroutine
│   │   └── migrations/
│   │       ├── 0001_create_manufacturing_orders.sql
│   │       ├── 0002_create_operations.sql
│   │       └── 0003_create_outbox.sql
│   │
│   ├── qms/                     # Quality Management System
│   │   ├── go.mod
│   │   ├── cmd/main.go
│   │   ├── domain/
│   │   ├── repo/
│   │   ├── handler/
│   │   ├── outbox/
│   │   └── migrations/
│   │
│   ├── iam/                     # Identity & Access Management (Keycloak wrapper)
│   │   ├── go.mod
│   │   └── cmd/main.go
│   │
│   └── bff/                     # Backend For Frontend
│       ├── go.mod
│       ├── cmd/main.go
│       ├── handler/             # REST + WebSocket handlers
│       └── middleware/          # JWT validation, role extraction
│
├── frontend/
│   └── operator/                # React SPA — operator tablet interface
│       ├── package.json
│       ├── vite.config.ts
│       ├── src/
│       │   ├── components/
│       │   ├── pages/
│       │   ├── hooks/
│       │   └── lib/
│       └── public/
│
├── infra/
│   ├── k8s/                     # Kubernetes manifests — Cloud deployment
│   │   └── helm/
│   │       ├── values.yaml
│   │       └── values-cloud.yaml
│   ├── k3s/                     # K3s manifests — Edge deployment
│   │   └── helm/
│   │       └── values-edge.yaml
│   ├── nats/
│   │   ├── hub.conf             # Cloud NATS Hub config
│   │   └── leaf-node.conf       # Edge Leaf Node config
│   ├── grafana/
│   │   └── dashboards/
│   │       ├── infrastructure.json
│   │       ├── nats.json
│   │       ├── mes-business.json
│   │       └── qms-quality.json
│   └── docker-compose.yml       # Local dev environment
│
└── docs/
    ├── adr/                     # Architecture Decision Records — never delete
    │   ├── ADR-001-go-backend-language.md
    │   ├── ADR-002-nats-unified-bus.md
    │   ├── ADR-003-monorepo.md
    │   ├── ADR-004-transactional-outbox.md
    │   ├── ADR-005-k3s-edge-deployment.md
    │   ├── ADR-006-no-grpc.md
    │   ├── ADR-007-polyglot-persistence.md
    │   └── ADR-008-observability.md
    └── async-api.yaml           # AsyncAPI spec for all NATS subjects
```

## go.work Configuration

```
// go.work
go 1.26.1

use (
    ./libs/core
    ./services/mes
    ./services/qms
    ./services/iam
    ./services/bff
)
```

This makes `kors-core-lib` directly importable from any service without version pinning:

```go
// In services/mes/go.mod
require github.com/kors/kors/libs/core v0.0.0

// In services/mes/domain/order.go
import "github.com/kors/kors/libs/core"
```

## CI/CD Path-Based Triggering

GitHub Actions jobs are triggered only when relevant paths change:

```yaml
# .github/workflows/mes.yml
on:
  push:
    paths:
      - 'services/mes/**'
      - 'libs/core/**'
      - 'proto/**'
```

## What Belongs Where

| Content | Location | Notes |
|---|---|---|
| Protobuf schemas | `/proto/{domain}/` | Edited by humans, never generated |
| Generated Go bindings | `/proto/gen/` | Generated by `buf generate`, never edited manually |
| Shared library code | `/libs/core/` | No domain logic, no service-specific code |
| Business domain logic | `/services/{name}/domain/` | Pure Go, no I/O, no framework imports |
| Database access | `/services/{name}/repo/` | Implements domain interfaces with pgx |
| NATS handlers | `/services/{name}/handler/` | Orchestrates domain + repo |
| SQL migrations | `/services/{name}/migrations/` | goose format, always Up + Down |
| K8s/K3s manifests | `/infra/` | No application logic here |
| ADR documents | `/docs/adr/` | Never deleted, only deprecated |
| Local dev config | `docker-compose.yml` | Never commit `.env` files |

## Consequences

**Positive:**
- Zero versioning friction between services.
- Complete context for AI agents in a single session.
- AGENTS.md at root applies to the entire project without duplication.
- Cross-service PRs are atomic.
- Protobuf contract changes are immediately visible to all services.

**Negative / constraints:**
- The monorepo grows over time. Monitor beyond ~20 services.
- CI pipelines must be configured to rebuild only what changed.
- `go.work` can create surprises with some Go tools. Document issues as they arise.

## Rules for Agents

```
NEVER: create a separate repository for a new KORS service
NEVER: edit files in /proto/gen/ manually — run buf generate
NEVER: put domain logic in /infra/ or /libs/core/
NEVER: put shared infrastructure code directly in a service
ALWAYS: new service follows the structure: cmd/ domain/ repo/ handler/ outbox/ migrations/
ALWAYS: ADR documents in /docs/adr/ are never deleted — add a Deprecated status instead
ALWAYS: TASK.md is created at the repo root during a task and deleted before merging
```

## Related ADRs

- ADR-001: Go (go.work enables kors-core-lib sharing)
- ADR-002: NATS (NATS config centralized in `/infra/nats/`)
- ADR-004: Transactional Outbox (outbox/ directory per service)
