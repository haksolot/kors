# KORS Infrastructure Runbook

## Deployment (K3s/Helm)

To deploy the KORS platform on a K3s cluster:

```bash
helm upgrade --install kors ./infra/helm/kors --namespace kors --create-namespace
```

## Scaling

To scale the MES service:

```bash
kubectl scale deployment kors-mes --replicas=5 -n kors
```

## Backup & Restore (Postgres)

### Backup

```bash
kubectl exec -t kors-postgres-0 -n kors -- pg_dump -U mes mes > mes_backup.sql
```

### Restore

```bash
cat mes_backup.sql | kubectl exec -i kors-postgres-0 -n kors -- psql -U mes mes
```

## Disaster Recovery

In case of NATS failure:
1. Verify NATS JetStream storage persistence.
2. Restart NATS statefulset.
3. Services will automatically reconnect and resume processing from the last acknowledged sequence (transactional outbox pattern).

## Monitoring

- Health checks: `GET /health` on BFF.
- Metrics: `GET /metrics` on all services (port 9090-9092).
- Tracing: Jaeger UI (if enabled).
