# ADR-008 — Observability: OpenTelemetry, Prometheus, Grafana, Loki

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | All services via `kors-core-lib`, `/infra/grafana/` |

---

## Context

In a microservice architecture where several Go services communicate via NATS, diagnosing a production problem without distributed observability means searching for a needle in a haystack. A user request can traverse the BFF, the MES service, the database, the outbox worker, and NATS — if a problem occurs, the entire path must be reconstructable.

Additionally, in an industrial Edge deployment context, monitoring must function locally without cloud dependency. A factory cannot depend on Datadog or New Relic to know if its MES is running.

The observability stack must cover three pillars:
1. **Traces** : distributed trace reconstruction across all services
2. **Metrics** : quantitative indicators (latency, error rate, queue depth, TRS)
3. **Logs** : structured events with trace correlation

The alternatives considered were:

- **Datadog / New Relic** : excellent SaaS tooling but cloud-only, expensive, impossible to deploy On-Premise on a factory server.
- **Jaeger + Prometheus + ELK** : open source but three separate stacks, heavy (Elasticsearch alone requires 4+ GB RAM).
- **OpenTelemetry + Prometheus + Grafana + Loki (LGTM stack)** : unified under Grafana, open source, deployable On-Premise, reasonable footprint (~1.5 GB additional on Edge), single query interface.

## Decision

**The KORS observability stack is standardized on OpenTelemetry + Prometheus + Grafana + Loki + Tempo (LGTM stack), deployed locally on K3s Edge and Cloud.**

- **OpenTelemetry** : instrumentation and trace propagation (in `kors-core-lib`)
- **Prometheus** : metric collection and storage
- **Loki** : log aggregation and storage
- **Tempo** : distributed trace storage (OTel backend)
- **Grafana** : unified visualization (logs + metrics + traces in one interface)
- **Alertmanager** : alerting based on Prometheus rules

## Implementation by Layer

### Traces — OpenTelemetry

Implemented in `kors-core-lib`. Every service imports core and automatically obtains:
- TraceID propagation in HTTP headers (incoming and outgoing)
- TraceID propagation in NATS message headers
- Automatic spans for NATS handlers and PostgreSQL queries
- Export to Tempo via OTLP protocol

```go
// In libs/core/tracing.go
func InitTracer(serviceName, otlpEndpoint string) {
    exporter, _ := otlptrace.New(ctx,
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(otlpEndpoint),
            otlptracegrpc.WithInsecure(),
        ),
    )
    tp := tracesdk.NewTracerProvider(
        tracesdk.WithBatcher(exporter),
        tracesdk.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
        )),
    )
    otel.SetTracerProvider(tp)
}

// Wrapping a NATS handler with a span
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
    return otel.Tracer("kors").Start(ctx, name)
}

// Propagating TraceID in a NATS message
func PublishWithTrace(ctx context.Context, nc *nats.Conn, subject string, data []byte) error {
    msg := nats.NewMsg(subject)
    msg.Data = data
    // Inject trace context into NATS message headers
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))
    return nc.PublishMsg(msg)
}
```

### Metrics — Prometheus

Each Go service exposes a `/metrics` endpoint in Prometheus format. Standard metrics are exposed automatically via `kors-core-lib`:

```go
// In libs/core/metrics.go — auto-registered at service startup
var (
    natsHandlerDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "kors_nats_handler_duration_seconds",
            Help:    "NATS handler duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"service", "subject"},
    )
    outboxPendingEvents = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "kors_outbox_pending_events",
            Help: "Number of events pending publication in outbox",
        },
        []string{"service"},
    )
    natsReconnectTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "kors_nats_reconnect_total",
            Help: "Total number of NATS reconnections",
        },
        []string{"service"},
    )
)
```

### Metric Naming Convention

Format: `kors_{service}_{metric_name}_{unit}`

```
# Business metrics MES
kors_mes_order_created_total
kors_mes_order_completed_total
kors_mes_order_suspended_total
kors_mes_nats_handler_duration_seconds{subject="kors.mes.of.create"}
kors_mes_trs_current_percent{of_id="..."}

# Business metrics QMS
kors_qms_nc_opened_total
kors_qms_nc_closed_total
kors_qms_control_saved_total
kors_qms_control_out_of_tolerance_total

# Infrastructure metrics (all services)
kors_outbox_pending_events{service="mes"}
kors_nats_reconnect_total{service="mes"}
kors_db_pool_acquired_connections{service="mes"}
```

### Logs — zerolog + Loki

All services use `zerolog` via `kors-core-lib`. Every log entry systematically contains:

```json
{
  "level": "info",
  "time": "2026-03-26T10:30:00.000Z",
  "service": "mes",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "of_id": "550e8400-e29b-41d4-a716-446655440000",
  "operator_id": "operator-uuid",
  "message": "Manufacturing order created"
}
```

```go
// In libs/core/logger.go
func NewLogger(serviceName string) *zerolog.Logger {
    log := zerolog.New(os.Stdout).
        With().
        Str("service", serviceName).
        Timestamp().
        Logger()
    return &log
}

// Usage in a handler — always include relevant business IDs
func (h *Handler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) error {
    ctx, span := core.StartSpan(ctx, "CreateOrder")
    defer span.End()

    traceID := span.SpanContext().TraceID().String()
    log := h.log.With().
        Str("trace_id", traceID).
        Str("of_reference", req.Reference).
        Logger()

    log.Info().Msg("Creating manufacturing order")
    // ...
    log.Info().Str("of_id", newOrder.ID).Msg("Manufacturing order created")
    return nil
}
```

### Prohibited logging patterns

```go
// WRONG — fmt.Println in service code
fmt.Println("order created") // DO NOT DO THIS

// WRONG — logging sensitive data
log.Info().Str("token", jwtToken).Msg("user authenticated") // DO NOT DO THIS

// WRONG — unstructured log
log.Info().Msg("created order 550e8400 for operator abc123") // DO NOT DO THIS
// CORRECT — structured with separate fields
log.Info().Str("of_id", order.ID).Str("operator_id", operatorID).Msg("Manufacturing order created")
```

## Pre-configured Grafana Dashboards

Four dashboards are delivered with KORS, defined as JSON in `/infra/grafana/dashboards/`:

### 1. Infrastructure (`infrastructure.json`)
- K3s pod status (green/red for each service)
- CPU and RAM usage per service over 24h
- NATS throughput (messages/sec)
- PostgreSQL connection pool saturation
- Outbox queue depth with alert threshold line at 1000

### 2. NATS (`nats.json`)
- Messages/sec per subject (heatmap)
- JetStream queue depths per consumer
- Request-Reply latency (p50, p95, p99)
- NATS reconnection count

### 3. MES Business (`mes-business.json`)
- Live TRS (large gauge, visible from 5 meters)
- Open / in-progress / completed orders (today)
- Average cycle time per reference (bar chart)
- Operators active now (count)
- Genealogy query response time

### 4. QMS Quality (`qms-quality.json`)
- Open NCs (count, per status)
- Conformity rate per reference over last 30 days
- Out-of-tolerance controls today
- Compliance dossiers generated (count)
- NC resolution average time

## Alert Rules (Alertmanager)

```yaml
# /infra/k3s/alertmanager-rules.yaml
groups:
  - name: kors-critical
    rules:
      - alert: OutboxQueueGrowing
        expr: kors_outbox_pending_events > 1000
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Outbox queue growing — NATS connectivity issue suspected"

      - alert: ServiceDown
        expr: up{job=~"kors-.*"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "KORS service {{ $labels.job }} is down"

      - alert: HighErrorRate
        expr: rate(kors_nats_handler_duration_seconds_count{status="error"}[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Error rate > 5% on service {{ $labels.service }}"

      - alert: NATSReconnecting
        expr: increase(kors_nats_reconnect_total[5m]) > 3
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "NATS reconnecting frequently — check network stability"
```

## Consequences

**Positive:**
- Production diagnosis possible without SSH access to servers.
- Log/metrics/traces correlation from a single interface (Grafana).
- Open source stack, deployable On-Premise without licensing.
- Works in degraded mode (local logs) if Loki/Tempo are unavailable.
- Business MES/QMS dashboards are a differentiator for clients.

**Negative / constraints:**
- LGTM stack consumes ~1.5–2 GB additional RAM on the Edge server. Must be included in sizing at client installation.
- OpenTelemetry adds ~1ms latency on each instrumented operation. Negligible in industrial context.
- Loki/Prometheus retention must be configured based on available storage (default: 30 days logs, 15 days metrics).
- Grafana requires a persistent volume on K3s for dashboard persistence.

## Rules for Agents

```
NEVER: use fmt.Println in service code — use zerolog via kors-core-lib
NEVER: log sensitive data (tokens, passwords, PII)
NEVER: create a metric without the kors_{service}_ prefix
ALWAYS: propagate context.Context through all I/O functions (required for OTel spans)
ALWAYS: log entry includes trace_id and relevant business IDs (of_id, nc_id, operator_id)
ALWAYS: new service calls core.InitTracer() and core.InitMetrics() in cmd/main.go
ALWAYS: kors_outbox_pending_events gauge is updated by the outbox worker after each poll
ALWAYS: new NATS handler is wrapped with core.StartSpan() for trace propagation
ALWAYS: alert rules are defined for: outbox > 1000 events, service down, error rate > 5%
```

## Related ADRs

- ADR-001: Go (zerolog is a Go-native structured logger)
- ADR-002: NATS (TraceID propagated in NATS message headers)
- ADR-004: Transactional Outbox (outbox queue depth as critical metric)
- ADR-005: K3s Edge (LGTM stack deployed on K3s)
