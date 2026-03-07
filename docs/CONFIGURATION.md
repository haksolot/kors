# Manuel de Configuration KORS

KORS est configuré exclusivement via l'environnement.

## 1. Base de données (PostgreSQL)

| Variable | Rôle | Impact si mal configuré |
|---|---|---|
| `DATABASE_URL` | DSN complet (ex: `postgres://...`) | Le service panique au démarrage. |
| `DATABASE_MAX_CONNS` | Taille max du pool (défaut: 25) | Sature la DB ou ralentit l'API en pic. |

## 2. Bus d'événements (NATS JetStream)

| Variable | Rôle | Impact si mal configuré |
|---|---|---|
| `NATS_URL` | Endpoint NATS (ex: `nats://nats:4222`) | Perte de traçabilité en temps réel. |
| `NATS_STREAM_NAME` | Nom du stream (défaut: `KORS`) | Incompatibilité entre API et Events. |

## 3. Sécurité & Hardening (kors-api)

| Variable | Rôle | Description technique |
|---|---|---|
| `GRAPHQL_COMPLEXITY_LIMIT` | Défaut: `1000` | Empêche les requêtes calculatoirement lourdes. |
| `GRAPHQL_INTROSPECTION` | `true`/`false` | Permet ou non d'explorer le schéma (Désactiver en Prod). |
| `RATE_LIMIT_STANDARD` | Défaut: `100` | Nombre de requêtes HTTP max par minute par adresse IP. |
| `JWKS_ENDPOINT` | URL Keycloak | Endpoint pour récupérer les clés RS256 de validation JWT. |

## 4. Maintenance (kors-worker)

| Variable | Rôle | Description technique |
|---|---|---|
| `PERMISSIONS_CLEANUP_INTERVAL` | Ex: `1h`, `30s` | Fréquence de purge des droits `expires_at < NOW()`. |
| `EVENTS_ARCHIVE_AFTER` | Défaut: `2160h` | Seuil pour l'archivage/purge des événements (90 jours). |

## 5. Stockage Objet (MinIO)

| Variable | Rôle | Impact |
|---|---|---|
| `MINIO_URL` | Host:Port | Échec de l'upload des révisions. |
| `MINIO_ACCESS_KEY` | Login admin | — |
| `MINIO_SECRET_KEY` | Password admin | — |
| `MINIO_BUCKET` | Défaut: `kors-files` | Doit exister ou sera créé au démarrage. |
