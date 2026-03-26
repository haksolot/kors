# ADR-005 — K3s for Edge Deployments

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-26 |
| **Deciders** | Architecture lead |
| **Applies to** | On-Premise deployments at client industrial sites, `/infra/k3s/` |

---

## Context

KORS On-Premise deployments at client industrial sites (aeronautics SME subcontractors) run on local physical servers with variable characteristics: typically x86_64 standard machines repurposed, with 8–32 GB RAM, without guaranteed internet connectivity, administered by small IT teams.

A core KORS architectural principle is **Edge-first**: every KORS service must be able to operate fully autonomously without internet connectivity. Production cannot depend on a cloud connection.

The deployment orchestration must:
- Run on constrained hardware (8 GB minimum usable for KORS)
- Use the same manifests as the Cloud deployment (no divergence)
- Be installable by an IT team without Kubernetes expertise
- Support rolling updates without production interruption
- Be a certified Kubernetes distribution (not a proprietary solution)

The alternatives considered were:

- **Kubernetes standard (k8s)** : CNCF standard, but requires minimum 2 vCPUs and 2 GB RAM for the control plane alone. Complex installation (kubeadm). Not acceptable on constrained industrial hardware.
- **Docker Compose** : simple, but no native rolling updates, no health checks, no horizontal scaling, not compatible with Cloud manifests. Dead-end for production.
- **MicroK8s (Canonical)** : Snap-based, heavier than K3s, Canonical-specific.
- **K3s (Rancher/SUSE)** : CNCF-certified Kubernetes distribution, < 512 MB RAM for control plane, single-binary installation, identical to standard Kubernetes for manifests and `kubectl`.

## Decision

**K3s is used for all On-Premise (Edge) deployments.** Standard Kubernetes is used for Cloud deployments.

The same Helm charts are used for both environments, with distinct values files (`values-edge.yaml` vs `values-cloud.yaml`).

## Edge Deployment Architecture

```
Industrial client server (physical, x86_64, 8–32 GB RAM)
└── K3s single-node (control plane + worker on same machine)
    │
    ├── NATS Leaf Node              → Connected to Cloud Hub via WAN
    │   └── Autonomous operation    → Production continues if WAN is down
    │
    ├── Services
    │   ├── mes              (Go binary, ~30 MB RAM)
    │   ├── qms              (Go binary, ~25 MB RAM)
    │   ├── iam (Keycloak)   (~512 MB RAM — degraded replica)
    │   └── bff + Traefik    (~50 MB RAM)
    │
    ├── Databases
    │   ├── PostgreSQL 16 + TimescaleDB  (~500 MB RAM)
    │   └── MinIO                        (~128 MB RAM)
    │
    └── Observability (optional on constrained hardware)
        ├── Prometheus       (~128 MB RAM)
        ├── Loki             (~256 MB RAM)
        └── Grafana          (~128 MB RAM)

Total estimated RAM: ~2–3 GB for KORS core stack
Recommended minimum: 8 GB (leaves 5+ GB for OS, overhead, and headroom)
```

## Installation Procedure

K3s installation on a client server:

```bash
# 1. Install K3s (single command)
curl -sfL https://get.k3s.io | sh -

# 2. Verify cluster is up
kubectl get nodes

# 3. Install Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# 4. Deploy KORS Edge stack
helm install kors ./infra/k3s/helm \
  --values ./infra/k3s/helm/values-edge.yaml \
  --set nats.leafNodeRemoteUrl=nats://hub.kors.io:7422

# 5. Verify all pods are running
kubectl get pods -n kors
```

## Helm Values Differences (Edge vs Cloud)

```yaml
# values-edge.yaml (Edge)
replicaCount: 1                    # Single node, no horizontal scaling
storage:
  class: local-path                # K3s built-in local storage
  postgresql:
    size: 50Gi
  minio:
    size: 100Gi
nats:
  mode: leaf-node
  remoteUrl: ""                    # Set at install time
tls:
  mode: self-signed                # Traefik generates self-signed cert
observability:
  enabled: true
  retention:
    logs: 30d
    metrics: 15d

# values-cloud.yaml (Cloud)
replicaCount: 3                    # High availability
storage:
  class: standard                  # Cloud storage class
nats:
  mode: hub
tls:
  mode: lets-encrypt               # Traefik with Let's Encrypt
```

## Traefik Configuration

Traefik is K3s's built-in Ingress controller. It handles:
- HTTP → HTTPS redirect
- TLS termination (self-signed Edge, Let's Encrypt Cloud)
- WebSocket proxying (required for NATS WebSocket via BFF)
- Service discovery via Kubernetes IngressRoute CRDs

```yaml
# Example IngressRoute for BFF
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: kors-bff
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`kors.local`) && PathPrefix(`/api`)
      kind: Rule
      services:
        - name: kors-bff
          port: 8080
    - match: Host(`kors.local`) && PathPrefix(`/ws`)
      kind: Rule
      services:
        - name: kors-bff
          port: 8080
      middlewares:
        - name: websocket-headers
```

## Keycloak Edge Strategy (IAM)

Keycloak is deployed in Edge mode for auth continuity during WAN outages. Strategy:

- A Keycloak replica is deployed on the Edge K3s cluster
- It synchronizes realm configuration from the Cloud on reconnect
- During WAN outage: the local replica continues issuing and validating tokens
- Services cache JWKS public keys locally (TTL: 1 hour) — validation works without Keycloak if keys are cached

This is documented in ADR-002 (IAM subject `kors.iam.*`) and in the Keycloak Helm values.

## Rolling Updates Without Downtime

```bash
# Update a KORS service (zero-downtime rolling update)
helm upgrade kors ./infra/k3s/helm \
  --values ./infra/k3s/helm/values-edge.yaml \
  --set mes.image.tag=1.2.0

# K3s applies the rolling update:
# 1. Start new pod with new image
# 2. Wait for health check to pass
# 3. Terminate old pod
# Total downtime: 0 (traffic routes to new pod before old is terminated)
```

Database schema migrations run as Kubernetes Jobs before the new service pod starts:

```yaml
# Helm hook: run migration before deploying new service version
annotations:
  helm.sh/hook: pre-upgrade
  helm.sh/hook-weight: "-1"
```

## Consequences

**Positive:**
- Memory footprint compatible with standard industrial hardware.
- Installation accessible to small IT teams (one command).
- Total parity with Cloud via identical Helm charts.
- Client retains complete control over their local infrastructure.
- Rolling updates without production interruption.

**Negative / constraints:**
- K3s single-node does not provide control plane high availability. Acceptable for single-site SMEs (K3s restart takes ~30 seconds).
- Some recent Kubernetes operators may not yet be fully K3s compatible. Verify case by case.
- TLS certificates in Edge are self-signed. Document the import procedure for operator tablets.
- K3s uses `containerd` as runtime (not Docker). Build images with `docker buildx` targeting `linux/amd64`.

## Rules for Agents

```
NEVER: deploy standard Kubernetes (k8s) on Edge client hardware
NEVER: create K3s-specific manifests — use standard Kubernetes manifests
NEVER: hardcode replicaCount > 1 in values-edge.yaml
ALWAYS: maintain values-edge.yaml and values-cloud.yaml in functional sync
ALWAYS: test Edge deployment in a local VM before each release
ALWAYS: database migrations run as Helm pre-upgrade hooks, not in service startup code
ALWAYS: Keycloak is deployed on Edge K3s alongside the other services
```

## Related ADRs

- ADR-002: NATS (NATS Leaf Node deployed on K3s Edge)
- ADR-003: Monorepo (`/infra/k3s/` and `/infra/k8s/` share Helm charts)
- ADR-007: Polyglot persistence (PostgreSQL + TimescaleDB + MinIO deployed on K3s)
- ADR-008: Observability (Prometheus + Loki + Grafana deployed on K3s)
