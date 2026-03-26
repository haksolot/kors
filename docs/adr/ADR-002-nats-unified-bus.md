# ADR-002 — NATS as the Unified Message Bus (Async + Sync)

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | All inter-service communication in `/services/`, `/libs/core/` |

---

## Context

KORS requires two communication paradigms simultaneously:

1. **Asynchronous** : IoT telemetry, business event notifications (OF created, NC opened), decoupled domain events between services. Emitter does not know the consumers.
2. **Synchronous** : Direct user interactions (load an OF, validate a control), transactional business validations requiring an immediate response.

Typical microservice architectures use two separate stacks: a message broker (Kafka, RabbitMQ) for async, and an RPC framework (gRPC) for sync. This duality creates double infrastructure to operate, monitor, and secure.

Additionally, KORS has a specific requirement: the **Edge/Cloud topology** — local clusters must operate autonomously and synchronize with a central hub. This topology must be native, not bolted on.

The alternatives considered were:

- **Kafka + gRPC** : industry standard, mature tooling, but requires Kafka (JVM + ZooKeeper or KRaft, minimum 512 MB), gRPC (protoc-gen-go-grpc, grpc-gateway, L7 load balancer), and a custom Edge/Cloud bridge. Two infrastructures, two monitoring stacks, two security models.
- **RabbitMQ + REST** : simpler than Kafka but still dual infrastructure. REST lacks the typing guarantees of Protobuf.
- **Redis Streams + gRPC** : fast but Redis is not designed for durable event streaming with complex consumer group semantics.
- **NATS + JetStream** : single infrastructure covering both paradigms, native Leaf Node topology for Edge/Cloud, minimal footprint (~10 MB idle), Go-first client.

## Decision

**NATS is the sole messaging infrastructure for KORS.**

- **NATS JetStream** covers asynchronous flows (persistent pub/sub, replay, At-Least-Once delivery).
- **NATS Core Request-Reply** covers synchronous inter-service calls (substitute for gRPC, see ADR-006).
- **NATS Leaf Nodes** handle the Edge/Cloud topology natively (see ADR-005).

All inter-service communication uses NATS. No direct HTTP between internal services.

## Subject Naming Convention

**This convention is mandatory. All subjects must follow it exactly. AI agents must not invent subject names.**

Format: `kors.{domain}.{entity}.{past_tense_verb}`

```
# Asynchronous events (JetStream)
kors.mes.of.created
kors.mes.of.completed
kors.mes.of.suspended
kors.mes.operation.started
kors.mes.operation.completed
kors.qms.nc.opened
kors.qms.nc.closed
kors.qms.control.saved
kors.iam.user.created

# Synchronous Request-Reply calls
kors.mes.of.get
kors.mes.of.list
kors.mes.of.create
kors.mes.operation.start
kors.mes.operation.complete
kors.mes.trs.get
kors.qms.nc.list
kors.qms.nc.get
kors.qms.control.plan.get
```

Subject name constants must be defined in the domain package, never hardcoded as strings in handlers:

```go
// CORRECT — in services/mes/domain/subjects.go
const (
    SubjectOFCreated     = "kors.mes.of.created"
    SubjectOFGet         = "kors.mes.of.get"
    SubjectOFList        = "kors.mes.of.list"
    SubjectOFCreate      = "kors.mes.of.create"
)

// WRONG — hardcoded in handler
nc.Publish("kors.mes.of.created", data) // DO NOT DO THIS
```

## Communication Modes

### Asynchronous (JetStream)

Used for: telemetry, state change notifications, domain events between services.

```go
// Publishing an event (always via Transactional Outbox — see ADR-004)
// The outbox worker calls this after the DB commit
err := core.Publish(conn, domain.SubjectOFCreated, &pb.OFCreatedEvent{...})

// Subscribing (durable consumer — survives service restarts)
js.Subscribe(domain.SubjectOFCreated, handler, nats.Durable("mes-of-created-qms"))
```

### Synchronous (Request-Reply)

Used for: direct user requests, transactional validations requiring immediate response.

```go
// Calling another service synchronously
resp, err := core.Request(conn, domain.SubjectOFGet, &pb.GetOFRequest{Id: ofID}, 5*time.Second)

// Handling a synchronous request
nc.Subscribe(domain.SubjectOFGet, func(msg *nats.Msg) {
    var req pb.GetOFRequest
    proto.Unmarshal(msg.Data, &req)
    // ... process
    msg.Respond(responseBytes)
})
```

### Queue Groups (load balancing)

Multiple instances of the same service subscribe with a Queue Group. NATS distributes requests round-robin automatically — no L7 load balancer needed.

```go
// All MES instances join the same Queue Group
nc.QueueSubscribe(domain.SubjectOFCreate, "mes-workers", handler)
```

## Edge/Cloud Topology

Each industrial site runs a NATS Leaf Node that connects to the central Cloud Hub. The local cluster operates fully autonomously when WAN connectivity is lost. Deferred events synchronize automatically on reconnect.

```
[Factory Edge]                    [Cloud Hub]
NATS Leaf Node  <-- WAN link -->  NATS Hub
  (autonomous)     (optional)     (consolidation)
```

Configuration is in `/infra/nats/leaf-node.conf` (Edge) and `/infra/nats/hub.conf` (Cloud).

## Message Serialization

All NATS messages use **Protobuf** encoding. Never use JSON for inter-service messages. See the Protobuf schema in `/proto/`. See ADR-006 for the rationale against gRPC.

## Security

Each service has its own NATS credentials (`nkey` + credentials file) scoped to its domain subjects only. A MES service cannot publish to `kors.qms.*` subjects. The NATS server configuration enforces this at the broker level.

Credentials are injected via environment variables — never hardcoded.

## Consequences

**Positive:**
- Single infrastructure for async + sync. One operator, one monitoring dashboard (NATS in Grafana).
- Edge/Cloud topology without additional development.
- Minimal footprint compatible with industrial Edge servers.
- Queue Groups = native load balancing without service mesh.
- `kors-core-lib` provides a single interface for all inter-service communication.

**Negative / constraints:**
- NATS is less known than Kafka in the community. Documentation and examples are less numerous.
- No tooling as rich as grpcurl for debugging Request-Reply calls. Mitigated by OpenTelemetry (ADR-008).
- JetStream consumers must be idempotent (At-Least-Once delivery).
- NATS does not provide Exactly-Once semantics natively — idempotency is a consumer responsibility.

## Rules for Agents

```
NEVER: direct HTTP calls between internal services
NEVER: use Kafka, RabbitMQ, or Redis Pub/Sub
NEVER: use gRPC (see ADR-006)
NEVER: hardcode subject strings in handlers — define them as constants in the domain package
NEVER: publish a NATS event outside a database transaction (see ADR-004)
ALWAYS: subjects follow the kors.{domain}.{entity}.{past_tense_verb} convention
ALWAYS: async events use JetStream, sync calls use Request-Reply
ALWAYS: multiple service instances use Queue Groups for load balancing
ALWAYS: each service uses scoped NATS credentials, never shared credentials
```

## Related ADRs

- ADR-001: Go (uses Go NATS client `nats.go`)
- ADR-003: Monorepo (NATS config centralized in `/infra/nats/`)
- ADR-004: Transactional Outbox (governs when events are published to NATS)
- ADR-005: K3s Edge (uses NATS Leaf Nodes)
- ADR-006: No gRPC (NATS Request-Reply as substitute)
