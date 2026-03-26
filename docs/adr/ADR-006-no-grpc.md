# ADR-006 — No gRPC — NATS Request-Reply as Substitute

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | All inter-service synchronous communication, `/services/`, `/libs/core/` |

---

## Context

gRPC is the de facto standard for synchronous inter-service calls in modern microservice architectures. It provides strong typing via Protobuf, automatic client generation, bidirectional streaming, and a mature tooling ecosystem (grpcurl, grpc-gateway, Postman gRPC, etc.).

The question is: why not use it in KORS?

## What gRPC Would Provide

- Strong typing for synchronous calls (Protobuf) → **already covered by NATS + Protobuf**
- Automatic Go client generation (protoc-gen-go-grpc) → **not needed with NATS helpers in kors-core-lib**
- Bidirectional streaming → **covered by NATS JetStream subscriptions**
- Request tracing → **covered by OpenTelemetry (ADR-008)**
- Load balancing → **covered by NATS Queue Groups (ADR-002)**

## Decision

**gRPC is not used in KORS.** Synchronous inter-service calls use **NATS Request-Reply with Protobuf** serialization. The BFF exposes a **REST + WebSocket** API to the browser — no gRPC-web.

## Justification

### 1. Unified infrastructure (ADR-002)

Adding gRPC would create a second communication infrastructure alongside NATS:
- Separate gRPC ports on each service (`:9090` or similar)
- `grpc-gateway` or Envoy proxy for HTTP exposure
- L7 load balancer for gRPC routing (gRPC uses HTTP/2 multiplexing, incompatible with standard L4 LB)
- Double monitoring (NATS metrics + gRPC metrics)
- Double security (NATS credentials + TLS client certificates for gRPC)

With NATS: single protocol, single port, single infrastructure to operate.

### 2. Native load balancing via Queue Groups

gRPC in Kubernetes requires an L7 load balancer (Envoy, Nginx, or a service mesh like Istio) to distribute requests across multiple service instances. gRPC uses HTTP/2 persistent connections — a standard L4 load balancer sends all requests from one connection to the same pod.

NATS Queue Groups distribute requests natively in round-robin across all subscribers in the same group. No load balancer, no service mesh.

```go
// All MES instances join "mes-workers" Queue Group
// NATS automatically distributes requests across them
nc.QueueSubscribe(domain.SubjectOFCreate, "mes-workers", handler)
```

### 3. Strong typing already covered

gRPC's main advantage over JSON REST is strong typing via Protobuf. NATS Request-Reply in KORS uses the same Protobuf schemas defined in `/proto/`. The typing is identical.

```go
// gRPC style (NOT used in KORS)
client := pb.NewMESServiceClient(conn)
resp, err := client.CreateOrder(ctx, &pb.CreateOrderRequest{...})

// NATS Request-Reply style (used in KORS)
// Same Protobuf types, different transport
resp, err := core.Request(conn, domain.SubjectOFCreate, &pb.CreateOrderRequest{...}, 5*time.Second)
```

### 4. No service mesh required

gRPC in Kubernetes typically requires a service mesh (Istio, Linkerd) for:
- mTLS between services
- Distributed tracing
- Advanced traffic management

With NATS:
- mTLS equivalent: NATS credentials (nkey) scoped per service
- Distributed tracing: OpenTelemetry in kors-core-lib propagates TraceID through NATS headers
- No sidecar, no service mesh, no additional infrastructure

### 5. Bidirectional streaming covered by NATS

gRPC's bidirectional streaming (server-side push to clients) is covered in KORS by:
- **Service-to-service** : NATS JetStream subscriptions (consumers receive events as they arrive)
- **Service-to-browser** : NATS WebSocket via the BFF (operator tablets receive live events)

## BFF API Design

The BFF exposes to the browser:

```
REST API (HTTP/1.1)
  POST   /api/orders                    → Create manufacturing order
  GET    /api/orders                    → List open orders
  GET    /api/orders/:id                → Get order detail + genealogy
  POST   /api/orders/:id/operations/:op_id/start     → Start operation
  POST   /api/orders/:id/operations/:op_id/complete  → Complete operation
  POST   /api/qms/controls              → Save quality control
  POST   /api/qms/nc                    → Open non-conformity
  PATCH  /api/qms/nc/:id               → Update NC status
  GET    /api/mes/trs                   → Current TRS

WebSocket (NATS events, filtered by role)
  ws://kors.local/ws
    ← kors.mes.of.created     (all roles)
    ← kors.mes.of.completed   (all roles)
    ← kors.qms.nc.opened      (QualityManager, IndustrialDirector)
    ← kors.mes.trs.updated    (IndustrialDirector)
```

The BFF translates between HTTP/WebSocket (browser-facing) and NATS (internal). No gRPC-web, no Protobuf on the browser side (REST uses JSON).

## Debugging Without grpcurl

For debugging NATS Request-Reply without grpcurl, use the `nats` CLI:

```bash
# Install nats CLI
brew install nats-io/nats-tools/nats

# Send a request-reply (for debugging only)
nats request kors.mes.of.list '{}' --creds /path/to/service.creds

# Monitor events on a subject
nats sub 'kors.mes.of.*' --creds /path/to/creds
```

OpenTelemetry traces (ADR-008) are the primary debugging tool for Request-Reply call chains in production.

## Consequences

**Positive:**
- Unified NATS infrastructure for everything (async + sync).
- No sidecar or service mesh.
- Native load balancing via Queue Groups.
- `kors-core-lib` covers both paradigms with the same interface.
- No L7 load balancer for gRPC routing.

**Negative / constraints:**
- Loss of gRPC tooling (grpcurl, Postman gRPC). Mitigated by `nats` CLI and OpenTelemetry.
- Native gRPC bidirectional streams are not available. Mitigated by JetStream.
- Fewer online examples for NATS Request-Reply than for gRPC. Internal documentation (ADR + AGENTS.md) must compensate.
- Slightly higher latency for Request-Reply vs gRPC due to NATS broker routing (~0.1ms additional). Negligible for all KORS use cases.

## Rules for Agents

```
NEVER: generate gRPC code (protoc-gen-go-grpc, grpc-gateway, connect-go)
NEVER: add google.golang.org/grpc as a dependency
NEVER: expose gRPC ports on internal services
NEVER: use gRPC-web for the browser — the BFF exposes REST + WebSocket only
ALWAYS: use NATS Request-Reply + Protobuf for synchronous inter-service calls
ALWAYS: the BFF exposes only REST (JSON) and WebSocket to the browser
ALWAYS: use core.Request() from kors-core-lib for synchronous calls
ALWAYS: multiple service instances use Queue Groups (nc.QueueSubscribe)
```

## Related ADRs

- ADR-002: NATS (NATS Request-Reply as the synchronous call mechanism)
- ADR-001: Go (uses Go NATS client, not gRPC Go client)
- ADR-008: Observability (OpenTelemetry replaces gRPC tracing tooling)
