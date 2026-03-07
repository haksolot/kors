# KORS Volume 2 : Infrastructure et Opérations

## 1. Variables d'Environnement (Standard Industriel)

Toutes les variables sont injectées via le système (Kubernetes Secrets ou .env).

### Configuration Core
| Variable | Description | Défaut |
|---|---|---|
| `DATABASE_URL` | DSN Postgres (ex: `postgres://user:pass@host:port/db`) | — |
| `NATS_URL` | Endpoint du bus NATS | `nats://localhost:4222` |
| `MINIO_URL` | Endpoint S3 MinIO | `localhost:9000` |
| `MINIO_BUCKET` | Nom du bucket pour les fichiers KORS | `kors-files` |

### Hardening & Sécurité
| Variable | Description | Défaut |
|---|---|---|
| `RATE_LIMIT_STANDARD` | Requêtes max par minute par IP | `100` |
| `GRAPHQL_COMPLEXITY_LIMIT` | Score max de complexité par requête | `1000` |
| `GRAPHQL_INTROSPECTION` | Activer l'introspection du schéma | `false` |
| `MINIO_USE_SSL` | Utiliser HTTPS pour MinIO | `false` |

### Maintenance (Worker)
| Variable | Description | Défaut |
|---|---|---|
| `PERMISSIONS_CLEANUP_INTERVAL` | Fréquence de purge des droits expirés | `1h` |
| `EVENTS_ARCHIVE_AFTER` | Seuil d'archivage des événements | `2160h` |

---

## 2. Commandes Opérationnelles

### Déploiement Local (Docker Compose)
*   **Démarrage complet** : `docker-compose -f infra/docker/docker-compose.yml up -d`
*   **Scaling horizontal** : `docker-compose up -d --scale kors-api=3`
*   **Mise à jour sans coupure** : `docker-compose up -d --build --no-deps <service>`

### Migration de Base de Données
KORS utilise `goose`.
*   **Up** : `goose -dir migrations up`
*   **Status** : `goose -dir migrations status`

---

## 3. Mécanismes de Résilience

### Arrêt Gracieux (Graceful Shutdown)
`kors-api` intercepte les signaux `SIGINT` et `SIGTERM`. Il attend jusqu'à 30 secondes que les requêtes GraphQL en cours soient terminées avant de fermer les connexions DB et NATS.

### Verrouillage Distribué (Distributed Locks)
`kors-worker` utilise des **Advisory Locks PostgreSQL** (ID 12345).
*   Garantit qu'une seule instance du worker effectue le nettoyage à un instant T.
*   Les autres instances sautent le cycle si le verrou est déjà tenu.

### Atomicité DB/NATS
Chaque création/transition de ressource est soumise à une **barrière transactionnelle** : si la publication sur le bus NATS échoue, l'API renvoie une erreur et l'opération est considérée comme avortée.
