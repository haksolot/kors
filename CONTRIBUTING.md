# KORS — Contributing Guidelines

This document defines the Git workflow, commit conventions, branch naming, and PR requirements for the KORS monorepo. These conventions apply to both human contributors and AI coding agents.

---

## 1. Branch Naming

Format: `{type}/{scope}-{short-description}`

### Types

| Type | Usage |
|---|---|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `refactor` | Code restructuring without behavior change |
| `test` | Adding or fixing tests only |
| `docs` | Documentation changes only |
| `chore` | Build, tooling, dependency updates |
| `infra` | Infrastructure, Kubernetes manifests, CI changes |
| `proto` | Protobuf schema changes |

### Scope

The scope is the affected service or layer: `mes`, `qms`, `iam`, `bff`, `core`, `frontend`, `infra`, `proto`, `docs`.

### Examples

```
feat/mes-of-creation
feat/qms-nc-declaration
fix/core-nats-reconnect
refactor/mes-order-repo
test/qms-control-validation
proto/mes-events-v2
infra/k3s-leaf-node-config
docs/adr-007-keycloak-edge
chore/update-go-dependencies
```

### Rules

- Lowercase only, hyphens as separators, no underscores, no slashes within the description part.
- Maximum 60 characters total.
- Branch names must be unique — include a ticket number or timestamp suffix if needed.
- Delete branches after merge. No stale branches older than 30 days.

---

## 2. Commit Conventions

KORS uses [Conventional Commits](https://www.conventionalcommits.org/). This is enforced by a commit-msg hook and validated in CI.

### Format

```
{type}({scope}): {short description}

{optional body}

{optional footer}
```

### Rules for the subject line

- Type and scope are mandatory.
- Short description: imperative mood, lowercase, no period at the end.
- Maximum 72 characters for the subject line.

### Rules for the body

- Wrap at 100 characters.
- Explain **what** changed and **why**, not how (the code explains how).
- Reference GitHub issues with `Closes #123` or `Refs #123`.

### Breaking changes

Add `BREAKING CHANGE:` in the footer, or append `!` after the type:

```
feat!(mes)!: change OF status enum values

BREAKING CHANGE: Status values renamed from snake_case to SCREAMING_SNAKE_CASE.
Existing OF records require migration script 0012_migrate_of_status.sql.
```

### Examples

```
feat(mes): add manufacturing order creation endpoint

Implements the POST /orders endpoint on the BFF.
Publishes kors.mes.of.created event via Transactional Outbox.
Includes input validation for reference and quantity.

Closes #14
```

```
fix(core): handle NATS reconnection on Leaf Node disconnect

The previous implementation did not handle graceful reconnection
when the WAN link between Edge and Cloud was interrupted.
Added exponential backoff with jitter and reconnect callback.

Refs #31
```

```
proto(mes): add OperationCompletedEvent message

Adds the OperationCompletedEvent to mes/events.proto.
Bumps the proto version to v1.1.0.
```

```
chore(deps): update NATS Go client to v1.37.0

No breaking changes. Includes reconnect stability improvements
relevant to the Edge Leaf Node deployment (ADR-002, ADR-005).
Justification: fixes race condition in reconnect handler.
```

```
test(qms): add table-driven tests for control validation

Covers: valid value, out-of-tolerance value, missing characteristic ID,
zero-value tolerance boundary. All cases use the table-driven pattern
as required by AGENTS.md section 4.2.
```

### What not to do

```
// WRONG — vague
git commit -m "fix stuff"
git commit -m "wip"
git commit -m "update"
git commit -m "changes"
git commit -m "Claude Code changes"

// WRONG — no scope
git commit -m "feat: add order creation"

// WRONG — uppercase
git commit -m "feat(mes): Add Order Creation"
```

---

## 3. Git Workflow

### 3.1 Standard feature workflow

```bash
# 1. Always branch from main — no long-lived branches
git checkout main
git pull origin main
git checkout -b feat/mes-of-creation

# 2. Work in small, atomic commits
# Each commit must leave the codebase in a buildable, testable state

# 3. Keep your branch up to date with main via rebase (not merge)
git fetch origin
git rebase origin/main

# 4. Before opening a PR, ensure tests pass locally
go test ./...
go vet ./...
golangci-lint run

# 5. Push and open a PR
git push origin feat/mes-of-creation
```

### 3.2 Commit atomicity

Each commit must be a self-contained, coherent change. Ask: "Can this commit be reverted independently without breaking the codebase?" If no, split it.

```
CORRECT atomic commit sequence:
  proto(mes): add CreateOrderRequest and CreateOrderResponse messages
  feat(mes): implement CreateOrder domain function with validation
  feat(mes): add PostgreSQL repository for manufacturing orders
  feat(mes): expose CreateOrder via BFF REST endpoint
  test(mes): add integration tests for CreateOrder

WRONG — everything in one commit:
  feat(mes): add order creation
```

### 3.3 Rebase, never merge

Do not use `git merge` to integrate changes from main into your branch. Always use `git rebase origin/main`. This keeps the history linear and readable.

```bash
# CORRECT
git rebase origin/main

# WRONG
git merge origin/main // DO NOT DO THIS on feature branches
```

### 3.4 Squashing

Squash commits before merging only if the intermediate commits contain WIP messages or correction commits (e.g., "fix typo", "oops"). Keep meaningful intermediate commits. Use squash-merge for PRs from AI agents when the intermediate history is noise.

---

## 4. Pull Request Requirements

### 4.1 PR title

Must follow the same Conventional Commits format as commit messages.

```
feat(mes): implement manufacturing order creation
fix(core): handle NATS reconnect on Leaf Node disconnect
```

### 4.2 PR description — mandatory template

Every PR must include all four sections. Incomplete PRs will not be reviewed.

```markdown
## What

<!-- One paragraph describing what this PR does. -->

## Why

<!-- Why this change is needed. Link to the issue or relevant discussion. -->

## How to test

<!-- Step-by-step instructions to verify the change locally.
     Include the exact commands to run. -->

## Checklist

- [ ] Tests written and passing (`go test ./...`)
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run` passes with no new warnings
- [ ] No hardcoded secrets or credentials
- [ ] Migrations have a Down section
- [ ] New Protobuf changes are backward-compatible
- [ ] AGENTS.md conventions followed
- [ ] If adding a dependency: justification included in commit message
- [ ] If changing an architectural pattern: ADR created or updated
```

### 4.3 Size guidelines

- Preferred PR size: under 400 lines changed (excluding generated files and migrations).
- PRs over 800 lines will be asked to split unless the change is genuinely atomic (e.g., a large migration with corresponding handler changes).
- Generated files (Protobuf output, mocks): excluded from size count but must be reviewed for correctness.

### 4.4 Review process

- Every PR requires at least one review before merge.
- Architecture lead reviews any change to `/libs/core`, `/proto`, `/infra`, or `/docs/adr`.
- Self-merge is allowed only for `docs` and `chore` PRs with no code changes.
- PRs from AI agents follow the same requirements — the human who opened the PR is responsible for the review.

### 4.5 CI requirements (must all pass before merge)

```
✓ go build ./...
✓ go test ./... -race
✓ go vet ./...
✓ golangci-lint run
✓ Secret scanning (gitleaks)
✓ Dependency vulnerability check (govulncheck)
✓ Proto schema compatibility check (buf breaking)
```

---

## 5. Working with AI Agents

### 5.1 Always use TASK.md for multi-step work

Before starting any task that involves more than one commit, create a `TASK.md` at the repo root:

```markdown
# Task: Implement Manufacturing Order Creation

## Objective
Implement the full flow for creating a manufacturing order: Protobuf schema,
domain logic, PostgreSQL repository, BFF endpoint, and tests.

## Steps
- [x] Define CreateOrderRequest/Response in proto/mes/orders.proto
- [x] Implement Order domain struct and CreateOrder function
- [ ] Implement PostgresOrderRepo with Save and FindByID
- [ ] Implement BFF handler calling MES via NATS Request-Reply
- [ ] Write integration tests for the repository
- [ ] Write handler tests with mocked repo
- [ ] Update OpenAPI spec

## Constraints
- Follow ADR-004 for event publishing (Transactional Outbox)
- Follow AGENTS.md section 2.1 for error handling
- No new external dependencies

## Definition of Done
- All checklist items completed
- go test ./... passes
- Manual test with curl documented in PR description
```

Delete TASK.md in the final commit of the task. It must never be merged into main.

### 5.2 Reviewing AI-generated PRs

When reviewing a PR generated by an AI agent, pay specific attention to:

- Error handling — agents often silently ignore errors or wrap without context.
- Global variables — agents sometimes introduce package-level state.
- Test quality — agents write shallow tests; check that edge cases are covered.
- New imports — agents add dependencies; verify each one is justified.
- Migration Down sections — agents often omit them.
- Subject naming — verify NATS subjects follow the naming convention.
- Outbox pattern — verify events are published within transactions.

### 5.3 Prompting discipline for agents

When giving a task to an agent, always specify:
- The exact file(s) to create or modify.
- The ADRs that apply.
- The section of AGENTS.md to follow.
- The expected test coverage.

Example of a good task prompt:
```
Implement the PostgreSQL repository for manufacturing orders in services/mes/repo/postgres.go.
Follow AGENTS.md sections 2.1 (error handling), 3.1 (migrations), and 4.2 (table-driven tests).
Apply ADR-004 for the outbox insertion.
The repository must implement the OrderRepository interface defined in services/mes/domain/order.go.
Write integration tests using testcontainers in services/mes/repo/postgres_test.go.
Do not add any new dependencies.
```

---

## 6. Versioning and Releases

### 6.1 Semantic versioning

KORS services follow [SemVer](https://semver.org/):
- PATCH: bug fixes, no API change
- MINOR: new features, backward-compatible
- MAJOR: breaking changes (Protobuf incompatibility, API removal)

### 6.2 Protobuf versioning

Protobuf schemas follow their own versioning. Breaking changes require a new package version:
```
proto/mes/v1/orders.proto   → current
proto/mes/v2/orders.proto   → breaking change, both versions maintained during migration
```

Use `buf breaking` in CI to prevent accidental breaking changes.

### 6.3 Tagging

Tags follow the format `{service}/v{semver}`:
```
mes/v1.0.0
qms/v1.0.0
core/v1.2.1
```

---

## 7. What Belongs Where

```
/proto/{domain}/          Protobuf schemas — no logic, no imports from /services
/libs/core/               Shared library — no domain logic, no service-specific code
/services/{name}/
  domain/                 Pure Go domain logic — no I/O, no framework dependencies
  repo/                   Database access — implements domain interfaces
  handler/                NATS and HTTP handlers — orchestrates domain + repo
  migrations/             SQL migration files — goose format
  cmd/                    Main entrypoint — wires everything together
/frontend/operator/       React SPA — no direct DB or NATS access
/infra/                   K8s/K3s manifests, NATS config — no application logic
/docs/adr/                Architecture Decision Records — never delete, only deprecate
```