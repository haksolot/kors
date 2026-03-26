# ADR-001 — Go as the Only Backend Language

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | All services in `/services/`, `/libs/core/` |

---

## Context

KORS microservices must be deployed on Edge industrial servers with constrained resources (8–16 GB RAM, no GPU, sometimes without permanent internet connectivity). The tech stack must be homogeneous to simplify maintenance by a small team and enable sharing of `kors-core-lib`. AI coding agents (Claude Code, Gemini CLI) must be able to generate consistent code across services without switching between languages and their respective idioms.

The alternatives considered were:

- **Node.js / TypeScript** : large ecosystem, but requires a Node.js runtime, significantly higher memory footprint, and event loop concurrency model less suited to I/O-intensive industrial workloads.
- **Python** : good for data science, but interpreted, requires runtime, slow startup, not suitable for Edge binaries.
- **Java / Kotlin** : JVM startup time (500ms–2s) incompatible with fast service restarts on Edge. Minimum 512 MB for JVM alone.
- **Rust** : excellent performance and safety, but steep learning curve and significantly slower development velocity, especially with AI agents.

## Decision

**Go is the sole language for all backend services.** This includes MES, QMS, IAM, and BFF microservices. No Node.js, Python, or Java service will be introduced in the main codebase.

The frontend (React/TypeScript) is explicitly excluded from this rule — it lives in `/frontend/` and runs in the browser.

## Justification

**Static binaries with no runtime dependencies.** A compiled Go binary runs directly on the Edge server without installing a JVM, Node runtime, or Python interpreter. Deployment is a single file copy. Docker images are ~10 MB versus ~200 MB for a JVM image.

**Memory footprint.** A Go service consumes 5–10x less memory than a JVM equivalent under the same load. On a shared industrial Edge server, this is critical — a full KORS Edge stack (MES + QMS + IAM + BFF + NATS + PostgreSQL) must fit comfortably within 8 GB.

**Native concurrency.** Goroutines allow handling thousands of concurrent NATS connections without OS thread overhead. Directly relevant for the BFF managing parallel WebSocket connections from operator tablets.

**AI agent consistency.** A single language, a single set of conventions in AGENTS.md. AI agents produce significantly more consistent code on a mono-language codebase. Pattern drift between services is the main risk in agentic development — Go's opinionated formatting (`gofmt`) and linting (`golangci-lint`) reduce this risk structurally.

**kors-core-lib.** The shared library is importable by all services via `go.work` without version management, interface layers, or cross-language marshalling.

**Cross-compilation.** Go compiles natively for any target architecture (`GOOS/GOARCH`). Building an ARM64 binary for an industrial embedded server from a developer's x86_64 machine is a one-liner.

## Consequences

**Positive:**
- Static binaries, minimal footprint, simplified K3s Edge deployment.
- Single linter (`golangci-lint`), single formatter (`gofmt`), single CI pipeline.
- AI agents generate consistent code matching existing patterns.
- `kors-core-lib` shareable without versioning friction.

**Negative / constraints:**
- Learning curve for developers coming from Python or JavaScript. Moderate but real.
- CGO is forbidden unless a specific ADR documents the exception — CGO breaks cross-compilation for Edge targets.
- Go's error handling (explicit `if err != nil`) is verbose. This is intentional and must not be worked around with panic or generic catch-all patterns.

## Rules for Agents

```
NEVER: introduce a Node.js, Python, Java, or Rust service in /services/
NEVER: use CGO without a dedicated validated ADR
NEVER: use panic() in service code — return errors instead
NEVER: use init() functions — initialize explicitly in main
ALWAYS: compile with target GOOS/GOARCH to validate cross-compilation in CI
ALWAYS: run gofmt and golangci-lint before any commit
```

## Related ADRs

- ADR-002: NATS as unified bus (uses Go NATS client)
- ADR-003: Monorepo (enables go.work for kors-core-lib)
