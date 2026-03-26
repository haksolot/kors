# ADR-007 — Polyglot Persistence: PostgreSQL, TimescaleDB, MinIO

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | All services with persistence needs, `/services/*/repo/`, `/services/*/migrations/` |

---

## Context

KORS handles three fundamentally different categories of data with distinct access patterns and volumes:

1. **Transactional business data** (manufacturing orders, quality controls, non-conformities, users): relational, ACID-critical, moderate volume.
2. **Time-series data** (machine telemetry, cycle times, historical TRS): high write volume, time-based aggregations, partitioning required.
3. **Heavy unstructured objects** (work instructions PDFs, NC photos, CAD files): large binary objects, random access by key, no querying.

Using a single engine for all three categories creates either unacceptable performance trade-offs or unnecessary modeling complexity.

The alternatives considered were:

- **PostgreSQL only** : covers #1 well, #2 poorly (time-series queries on standard tables degrade rapidly beyond millions of rows), #3 unacceptably (BYTEA columns in PostgreSQL create bloat and performance issues above ~1 MB per object).
- **MongoDB** : flexible for #1 but sacrifices ACID guarantees required for the Transactional Outbox Pattern (ADR-004).
- **InfluxDB for time-series** : good for #2 but introduces an additional engine entirely separate from PostgreSQL, with its own query language (Flux/InfluxQL).
- **PostgreSQL + TimescaleDB + MinIO** : TimescaleDB is a PostgreSQL extension — same connection string, same driver, same migration tool. MinIO is S3-compatible. Zero additional language to learn.

## Decision

**KORS uses three storage engines, each optimized for its data category:**

- **PostgreSQL 16** for all transactional business data
- **TimescaleDB** (PostgreSQL extension) for IoT time-series and historical metrics
- **MinIO** for heavy binary objects (S3-compatible)

## PostgreSQL 16 — Transactional Data

### What goes in PostgreSQL

Everything that requires ACID transactions, relational integrity, and the Transactional Outbox Pattern:
- Manufacturing orders (`manufacturing_orders`)
- Operations (`operations`)
- Quality controls (`control_results`)
- Non-conformities (`non_conformities`)
- Users and roles (managed by Keycloak, mirrored for reference)
- Outbox tables (`outbox`) — one per service
- Work instructions metadata (filename, MinIO key, version) — NOT the file itself
- NC attachment references (MinIO bucket + key) — NOT the binary

### Driver and ORM policy

**pgx/v5 exclusively. No ORM.**

```go
// CORRECT — raw SQL with pgx/v5
import "github.com/jackc/pgx/v5/pgxpool"

rows, err := pool.Query(ctx,
    "SELECT id, reference, status FROM manufacturing_orders WHERE status = $1",
    "planned",
)

// WRONG — ORM
import "gorm.io/gorm" // DO NOT ADD THIS DEPENDENCY
db.Where("status = ?", "planned").Find(&orders)
```

Rationale: ORMs hide query complexity, generate N+1 problems, and fight against pgx's connection pool. Raw SQL is readable, predictable, and fully compatible with TimescaleDB-specific syntax.

### Schema conventions

```sql
-- Table names: snake_case, plural
-- Column names: snake_case
-- Primary keys: UUID (gen_random_uuid()), not BIGSERIAL (portability)
-- Timestamps: TIMESTAMPTZ (always UTC, never TIMESTAMP)
-- Foreign keys: explicit constraint, named fk_{table}_{referenced_table}
-- Indexes: named idx_{table}_{columns}

CREATE TABLE manufacturing_orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference   TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'planned'
                    CHECK (status IN ('planned', 'in_progress', 'completed', 'suspended')),
    operator_id UUID,
    lot_number  TEXT,
    serial_number TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_manufacturing_orders_status ON manufacturing_orders(status);
CREATE INDEX idx_manufacturing_orders_lot ON manufacturing_orders(lot_number);
```

### Migration management (goose)

All schema changes use goose migration files in `/services/{name}/migrations/`.

```
# File naming: {sequence}_{description}.sql
0001_create_manufacturing_orders.sql
0002_create_operations.sql
0003_create_outbox.sql
0004_add_serial_number_to_orders.sql
```

Every migration has both `-- +goose Up` and `-- +goose Down` sections. A migration without a Down section will be rejected in code review.

Migrations are applied automatically at service startup:

```go
// In cmd/main.go
goose.Up(db, "migrations")
```

## TimescaleDB — Time-Series Data

### What goes in TimescaleDB

Any data with a timestamp and a volume exceeding a few thousand rows per day:
- Cycle times per operation (written on every `operation.completed` event)
- Machine telemetry (temperature, vibration, speed) — if IoT sensors are connected
- TRS historical data (computed every 15 minutes from cycle times)
- OEE aggregations per reference per day

### Hypertable configuration

TimescaleDB is a PostgreSQL extension — same connection string, same pgx driver. Tables are created as standard PostgreSQL tables, then converted to hypertables.

```sql
-- +goose Up
CREATE TABLE cycle_times (
    id           BIGSERIAL,          -- not UUID — high insert volume, BIGSERIAL is faster
    of_id        UUID NOT NULL,
    operation_id UUID NOT NULL,
    operator_id  UUID NOT NULL,
    duration_ms  INTEGER NOT NULL,
    measured_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Convert to hypertable partitioned by measured_at
-- chunk_time_interval = 1 hour for high-frequency data
SELECT create_hypertable('cycle_times', 'measured_at',
    chunk_time_interval => INTERVAL '1 hour');

-- Compression policy: compress chunks older than 7 days (~10:1 ratio)
SELECT add_compression_policy('cycle_times', INTERVAL '7 days');

-- Retention policy: drop data older than 2 years
SELECT add_retention_policy('cycle_times', INTERVAL '2 years');

-- +goose Down
DROP TABLE cycle_times;
```

### TRS query pattern

```sql
-- TRS for an active OF over the last 8 hours
-- Uses time_bucket() — TimescaleDB native function
SELECT
    time_bucket('1 hour', measured_at) AS hour,
    AVG(duration_ms)                   AS avg_cycle_ms,
    COUNT(*)                           AS operations_count,
    -- Theoretical cycle time comes from the work instruction (joined from PostgreSQL)
    AVG(duration_ms) / wi.theoretical_duration_ms AS performance_rate
FROM cycle_times ct
JOIN work_instructions wi ON wi.operation_id = ct.operation_id
WHERE ct.of_id = $1
  AND ct.measured_at > NOW() - INTERVAL '8 hours'
GROUP BY hour, wi.theoretical_duration_ms
ORDER BY hour;
```

### TimescaleDB vs standard PostgreSQL tables

```
Standard PostgreSQL table:
→ FOR: orders, users, NC, controls — anything with relational joins and moderate volume
→ AGAINST: time-series (query degradation above ~10M rows, no partitioning)

TimescaleDB hypertable:
→ FOR: anything with a timestamp and high write volume
→ AGAINST: tables requiring complex relational joins across chunks
```

## MinIO — Object Storage

### What goes in MinIO

Any binary object larger than ~1 KB:
- Work instruction PDFs (attached to operations in manufacturing orders)
- Photos attached to non-conformities (JPEG/PNG, uploaded from tablets)
- 3D CAD files (STEP, IGES) — for future PLM module
- Generated compliance PDFs (EN9100 conformity dossiers)

### Reference pattern

**Never store binaries in PostgreSQL.** Store only the MinIO reference:

```sql
-- In PostgreSQL: only the reference, not the binary
CREATE TABLE non_conformity_attachments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nc_id      UUID NOT NULL REFERENCES non_conformities(id),
    filename   TEXT NOT NULL,
    mime_type  TEXT NOT NULL,
    minio_bucket TEXT NOT NULL DEFAULT 'kors-attachments',
    minio_key    TEXT NOT NULL,   -- e.g. "nc/2026/03/photo-001.jpg"
    size_bytes   BIGINT NOT NULL,
    uploaded_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

```go
// Upload to MinIO, store reference in PostgreSQL
func (r *Repo) AttachPhoto(ctx context.Context, ncID string, file []byte, filename string) error {
    key := fmt.Sprintf("nc/%s/%s", time.Now().Format("2006/01"), filename)

    // 1. Upload to MinIO
    _, err := r.minio.PutObject(ctx, "kors-attachments", key,
        bytes.NewReader(file), int64(len(file)),
        minio.PutObjectOptions{ContentType: "image/jpeg"},
    )
    if err != nil {
        return fmt.Errorf("AttachPhoto: upload to MinIO: %w", err)
    }

    // 2. Store reference in PostgreSQL (in same transaction as NC update)
    _, err = r.db.Exec(ctx,
        "INSERT INTO non_conformity_attachments (nc_id, filename, minio_bucket, minio_key, size_bytes) VALUES ($1, $2, $3, $4, $5)",
        ncID, filename, "kors-attachments", key, len(file),
    )
    return err
}
```

### MinIO client configuration

```go
// In kors-core-lib or service config
import "github.com/minio/minio-go/v7"

client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
    Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
    Secure: cfg.MinIOSecure, // false for Edge HTTP, true for Cloud HTTPS
})
```

MinIO is S3-compatible: migrating to AWS S3 or OVH Object Storage in the future requires only changing the endpoint and credentials — no code change.

### Bucket structure

```
kors-attachments/
├── nc/                    # Non-conformity photos
│   └── 2026/03/
├── instructions/          # Work instruction PDFs
│   └── {operation_id}/
├── compliance/            # Generated EN9100 compliance dossiers
│   └── {of_id}/
└── cad/                   # CAD files (future PLM module)
```

## Data Partitioning Decision Table

| Data type | Engine | Rationale |
|---|---|---|
| Manufacturing orders | PostgreSQL | Transactional, relational, low volume |
| Operations | PostgreSQL | Transactional, FK to orders |
| Quality controls | PostgreSQL | Transactional, FK to operations |
| Non-conformities | PostgreSQL | Transactional, audit trail |
| Users, roles | PostgreSQL (Keycloak) | IAM data |
| Outbox tables | PostgreSQL | Same transaction as business data |
| Cycle times | TimescaleDB | High write volume, time-based queries |
| Machine telemetry | TimescaleDB | Very high write volume, compression |
| TRS history | TimescaleDB | Computed aggregates, time-series |
| Work instructions (PDF) | MinIO | Binary, > 1 MB |
| NC photos | MinIO | Binary, JPEG/PNG |
| CAD files | MinIO | Binary, very large |
| Compliance PDFs | MinIO | Generated binary |

## Consequences

**Positive:**
- Each data type exploits the optimal engine for its access pattern.
- TimescaleDB shares the PostgreSQL connection — no additional infrastructure.
- MinIO is S3-compatible: future migration to cloud storage requires no code change.
- Separation of concerns: degradation of one engine does not affect the others.

**Negative / constraints:**
- Three engines to deploy, monitor, and back up (mitigated by TimescaleDB being a PG extension).
- Developers must know which entity goes in which engine. Documented in this ADR.
- MinIO in single-node on Edge has no redundancy. Acceptable for single-site SMEs with a local backup policy.
- pgx/v5 is slightly more verbose than an ORM. Compensated by predictability and performance.

## Rules for Agents

```
NEVER: store a binary file directly in PostgreSQL (use MinIO)
NEVER: store time-series data in standard PostgreSQL tables (use TimescaleDB hypertables)
NEVER: use an ORM — use pgx/v5 with raw SQL
NEVER: store MinIO credentials in code — use environment variables
ALWAYS: store only the MinIO reference (bucket + key) in PostgreSQL, never the binary
ALWAYS: TimescaleDB hypertables have chunk_time_interval adapted to expected write volume
ALWAYS: PostgreSQL column names are snake_case, plural table names
ALWAYS: timestamps use TIMESTAMPTZ, never TIMESTAMP (timezone-aware)
ALWAYS: primary keys use UUID (gen_random_uuid()), not BIGSERIAL for business entities
ALWAYS: every migration has a -- +goose Down section
```

## Related ADRs

- ADR-004: Transactional Outbox (outbox table in PostgreSQL)
- ADR-005: K3s Edge (PostgreSQL + TimescaleDB + MinIO deployed on K3s)
- ADR-008: Observability (TimescaleDB used for historical TRS metrics)
