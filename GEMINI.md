# KORS — CLAUDE.md

> Ce fichier est lu par Claude en premier lors de toute intervention sur le repo.
> Il décrit le projet, son architecture, ses conventions de code et sa feuille de route.
> Ne pas modifier sans validation de l'équipe technique.

---

## 1. Contexte et raison d'être

**KORS** (_Kernel for Operations & Resource Systems_) est le noyau technique commun de Safran Landing Systems. Il remplace progressivement un ensemble d'outils industriels sous licence (MES, TMS, PLM, PDM, MEDS) par une plateforme interne souveraine, intégrée et traçable.

KORS n'est **pas** un outil métier. C'est le socle sur lequel les outils métiers se construisent. Il fournit quatre capacités fondamentales à n'importe quel module qui se branche dessus :

- **Registre d'entités typées** — toute entité métier existe dans KORS avec un type, un état et des métadonnées validées.
- **Cycle de vie contrôlé** — les transitions d'état sont déclarées par le module et appliquées par KORS. Une transition non autorisée est rejetée.
- **Traçabilité immuable** — tout événement est journalisé de façon permanente. Requis EN9100. On peut reconstituer l'état exact d'une entité à n'importe quel instant passé.
- **Bus d'événements découplé** — un module publie, les autres réagissent. Jamais d'appel direct entre modules. Le TMS ne connaît pas le MES.

### Ce que KORS ne fait pas

KORS ne connaît pas les dates de calibration, les quantités d'ordres de fabrication, les gammes d'usinage. Toute logique métier reste dans le module. KORS fournit le cadre commun pour que cette logique soit traçable, sécurisée et interopérable.

---

## 2. Architecture générale

### Services du noyau

```
kors/
  kors-api/        # Serveur GraphQL principal — gqlgen + chi
  kors-events/     # Consommateur / Producteur NATS JetStream
  kors-worker/     # Jobs asynchrones (SAP sync, tâches différées)
  kors-sap/        # Adaptateur SAP OData / RFC-BAPI
  shared/
    korsctx/       # Context helpers (identity_id, request_id)
    health/        # Handlers Kubernetes /healthz /readyz
    pagination/    # Cursor-based pagination spec Relay Connection
  infra/
    docker/        # docker-compose développement local
    k8s/           # Manifestes Kubernetes (Kustomize)
  docs/
    adr/           # Architecture Decision Records
```

### Stack technique

| Composant | Technologie | Justification |
|---|---|---|
| API | Go 1.26 + gqlgen | Performance, typage fort, génération de code depuis le schéma |
| Bus événements | NATS JetStream | Persistance disque, déduplication, rétention 90j, faible latence |
| Base de données | PostgreSQL 18.3 + Patroni | Fiabilité, JSONB, FKs, migrations SQL versionnées |
| Object storage | MinIO (S3) | Souveraineté, versioning, plans / modèles 3D / docs |
| SSO | Keycloak (OIDC + LDAP) | Intégration Active Directory Safran, JWT RS256 |
| Observabilité | OpenTelemetry + Grafana Stack | Traces distribuées, métriques, logs structurés |
| Orchestration | Kubernetes | Déploiement, scaling, health checks |

### Les six primitives du schéma `kors`

Toute la valeur de KORS repose sur six tables PostgreSQL dans le schéma `kors`. Aucun module n'écrit dans ce schéma — il est géré exclusivement par `kors-api`.

| Primitive | Rôle |
|---|---|
| `identities` | Tout acteur du système (utilisateur, service, système tiers) |
| `resource_types` | Registre des types déclarés par les modules avec leur JSON Schema et leur graphe de transitions |
| `resources` | Index universel de toutes les entités métier avec leur état courant |
| `events` | Journal immuable de tout ce qui se passe dans le système |
| `revisions` | Snapshots versionnés des entités (avec référence MinIO si fichier associé) |
| `permissions` | RBAC générique : qui peut faire quoi sur quoi, avec expiration optionnelle |

---

## 3. Modèle mental pour les développeurs

Avant d'écrire du code, garder ce modèle en tête :

**Un module déclare, KORS enregistre.** Le module TMS dit à KORS "je vais gérer des entités de type `tool`, voici leurs états possibles et leurs transitions autorisées". KORS enregistre ce contrat. À partir de là, toute Resource de type `tool` est soumise à ce contrat — KORS l'applique sans que le module ait à re-vérifier.

**Une Resource KORS est distincte de l'entité métier.** Quand le TMS crée un outil, il y a deux choses : une `Resource` dans `kors.resources` (l'enveloppe KORS) et une ligne dans `tms.tools` (les détails métier). Les deux partagent le même UUID. La Resource KORS porte l'état, la traçabilité, les droits. La table `tms.tools` porte les données riches.

**Les modules ne se parlent pas directement.** Le MES ne fait jamais d'appel HTTP vers le TMS. Il écoute les événements KORS et réagit. Si le TMS publie `kors.resource.state_changed` pour un outil qui passe en maintenance, le MES peut bloquer les opérations planifiées sur cet outil — sans couplage.

---

## 4. Conventions de code

### Langue

- **Code, variables, commentaires** : anglais exclusivement.
- **Commits, PR, documentation** : français (contexte Safran Landing Systems).
- **Logs applicatifs** : anglais (Grafana, alertes, on-call).

### Structure d'un service Go

Chaque service suit une architecture en couches stricte. Les dépendances ne vont que vers l'intérieur.

```
kors-api/
  cmd/server/
    main.go              # Point d'entrée uniquement — wiring, démarrage, arrêt gracieux
  internal/
    config/
      config.go          # Chargement config depuis variables d'environnement
    domain/              # Couche domaine — zéro dépendance externe
      resource/
        resource.go      # Struct + interface Repository
      event/
        event.go
      identity/
        identity.go
      resourcetype/
        resourcetype.go
      revision/
        revision.go
      permission/
        permission.go
    usecase/             # Cas d'usage — orchestrent les domaines
      create_resource.go
      transition_resource.go
      publish_event.go
    adapter/             # Implémentations concrètes des interfaces domaine
      postgres/
        resource_repo.go
        event_repo.go
      nats/
        event_publisher.go
        subscription_manager.go
      minio/
        file_store.go
    graph/               # Couche GraphQL — gqlgen
      schema/
        schema.graphql   # Source de vérité du contrat API
      generated/         # Généré par gqlgen — ne jamais éditer à la main
      resolvers/
        resource.resolvers.go
        event.resolvers.go
      model/
        models_gen.go    # Généré par gqlgen
    middleware/
      auth.go            # Vérification JWT, injection identity dans context
      tracing.go         # OpenTelemetry middleware
      logger.go          # Logging structuré zerolog
  migrations/
    00001_init_kors_schema.sql
    00002_add_index_events_type.sql
  gqlgen.yml             # Configuration gqlgen
  go.mod
```

### Règles d'architecture (non négociables)

**Règle 1 — Le domaine est pur.** Les structs et interfaces dans `internal/domain/` n'importent aucun package externe hormis `github.com/google/uuid` et la bibliothèque standard. Pas de `pgx`, pas de `nats`, pas de `zerolog` dans le domaine.

**Règle 2 — Les interfaces vont dans le domaine, les implémentations dans les adapters.** `domain/resource/resource.go` définit l'interface `Repository`. `adapter/postgres/resource_repo.go` l'implémente. Un usecase ne connaît que l'interface.

**Règle 3 — Les usecases orchestrent, ils ne persistent pas.** Un usecase appelle les méthodes des repositories et publishers. Il ne fait jamais de SQL direct.

**Règle 4 — Les resolvers GraphQL sont minces.** Un resolver extrait les arguments, appelle un usecase, retourne le résultat. Jamais de logique métier dans un resolver.

**Règle 5 — `main.go` fait le câblage.** C'est le seul endroit où les implémentations concrètes sont instanciées et injectées dans les usecases. Pas d'initialisation globale avec `var db *pgx.Pool` au niveau du package.

**Règle 6 — Pas de `panic` hors de `main.go`.** Les erreurs remontent via `error`. Un `panic` dans un handler tué le pod Kubernetes.

**Règle 7 — Le schéma GraphQL est la source de vérité.** On modifie `schema.graphql`, on lance `make generate`, le compilateur signale ce qui est cassé. On ne modifie jamais les fichiers dans `generated/`.

### Nommage

```go
// Bon — nom explicite, verbe d'action pour les usecases
type CreateResourceUseCase struct { ... }
func (uc *CreateResourceUseCase) Execute(ctx context.Context, input CreateResourceInput) (*resource.Resource, error)

// Mauvais — trop générique
type ResourceService struct { ... }
func (s *ResourceService) Create(input interface{}) error
```

```go
// Bon — erreur typée avec contexte
return nil, fmt.Errorf("transition resource %s: state %s → %s not allowed: %w", id, from, to, ErrInvalidTransition)

// Mauvais — erreur muette
return nil, errors.New("invalid transition")
```

### Gestion des erreurs

Définir les erreurs sentinel dans le package domaine concerné :

```go
// domain/resource/errors.go
var (
    ErrNotFound         = errors.New("resource not found")
    ErrInvalidTransition = errors.New("transition not allowed")
    ErrForbidden        = errors.New("permission denied")
)
```

Dans les resolvers GraphQL, mapper les erreurs domaine vers les codes `MutationError` :

```go
if errors.Is(err, resource.ErrInvalidTransition) {
    return &model.ResourceResult{
        Success: false,
        Error: &model.MutationError{
            Code:    "INVALID_STATE",
            Message: err.Error(),
        },
    }, nil
}
```

### Tests

Chaque couche a sa stratégie de test :

| Couche | Type de test | Outils |
|---|---|---|
| Domaine | Tests unitaires purs | `testing` stdlib, pas de mock |
| Usecases | Tests unitaires avec mocks des interfaces | `testify/mock` |
| Adapters postgres | Tests d'intégration | `testcontainers-go` + PostgreSQL réel |
| Adapters NATS | Tests d'intégration | `testcontainers-go` + NATS réel |
| Resolvers GraphQL | Tests de bout en bout | `net/http/httptest` + client GraphQL |

Règle de couverture minimale : 80 % sur les usecases, 100 % sur les fonctions de validation du domaine.

Nommage des tests :

```go
// Pattern : Test{Sujet}_{Scenario}_{ResultatAttendu}
func TestTransitionResource_InvalidTransition_ReturnsErrInvalidTransition(t *testing.T)
func TestTransitionResource_ValidTransition_UpdatesStateAndPublishesEvent(t *testing.T)
```

### Logging

Zerolog uniquement. Pas de `fmt.Println`, pas de `log.Printf` stdlib.

```go
// Bon
log.Ctx(ctx).Info().
    Str("resource_id", id.String()).
    Str("from_state", from).
    Str("to_state", to).
    Msg("resource state transitioned")

// Mauvais
fmt.Printf("resource %s transitioned to %s\n", id, to)
```

Niveaux :
- `Debug` — flux internes, requêtes SQL en dev
- `Info` — événements métier significatifs (création, transition)
- `Warn` — situations anormales non bloquantes (retry, degraded)
- `Error` — erreurs avec stack trace, toujours avec `Err(err)`
- `Fatal` — uniquement dans `main.go` au démarrage

### Commits

Format Conventional Commits :

```
feat(kors-api): add transitionResource mutation with permission check
fix(kors-events): deduplicate events on nats_message_id before insert
refactor(domain): extract transition validation to ResourceType method
test(adapter/postgres): add integration test for resource repository
docs(adr): add ADR-003 for NATS JetStream consumer strategy
chore: upgrade gqlgen to v0.17.45
```

---

## 5. Contrat API GraphQL

Le schéma GraphQL complet est dans `kors-api/internal/graph/schema/schema.graphql`. C'est la source de vérité. Toute modification du contrat passe par ce fichier.

### Règles d'évolution du schéma

- **Ajout** : libre, sans impact sur les clients existants.
- **Modification** : interdite. Ajouter un nouveau champ et déprécier l'ancien avec `@deprecated(reason: "...")`.
- **Suppression** : uniquement après 3 mois de dépréciation et confirmation que tous les modules ont migré.

### Pagination

Toute liste pouvant dépasser 100 éléments utilise la pagination cursor-based (spec Relay Connection). Arguments standard : `first: Int`, `after: String`. Réponse standard : `{ edges { cursor node } pageInfo { hasNextPage endCursor } totalCount }`.

### Erreurs

Les erreurs métier ne sont jamais des erreurs GraphQL de niveau protocole. Elles sont des `MutationError` dans le type `Result` retourné. Seules les erreurs techniques (base indisponible, timeout) remontent comme erreurs protocole avec une extension `code`.

---

## 6. Modèle de données

Le schéma `core` PostgreSQL est géré exclusivement par `kors-api` via les migrations Goose dans `kors-api/migrations/`. Aucun autre service n'écrit dans ce schéma.

Les modules ont leur propre schéma PostgreSQL dans la même instance. KORS provisionne le schéma vide et l'utilisateur dédié. Le module applique ses propres migrations au démarrage. Les règles d'interaction :

- Un module peut lire le schéma `kors` en lecture seule.
- Un module ne crée jamais de foreign key vers le schéma d'un autre module.
- Les références inter-modules passent uniquement par les UUIDs des Resources KORS.

---

## 7. Événements NATS — conventions

### Nommage des sujets

Format : `{emetteur}.{entite}.{action}`

```
kors.resource.created
kors.resource.state_changed
kors.resource.revision_created
kors.identity.created
```

Les modules utilisent leur propre préfixe pour leurs événements internes :

```
tms.tool.maintenance_started
mes.order.released
```

### Idempotence

Chaque message publié sur NATS porte un header `Nats-Msg-Id` avec un UUID unique. `kors-events` vérifie la présence de cet ID dans `kors.events.nats_message_id` avant toute insertion. Un message déjà traité est acquitté sans retraitement.

### Consumer durable

`kors-events` utilise un consumer durable (`kors-events-consumer`) pour garantir qu'aucun message n'est perdu en cas de redémarrage. La rétention JetStream est de 90 jours minimum (exigence EN9100).

---

## 8. Feuille de route

### Phase 1 — Fondations (mois 1–4)

Objectif : un développeur peut créer un ResourceType et des Resources via l'API GraphQL. Le bus événements fonctionne. L'infrastructure tourne en Kubernetes.

**Sprint 1–2 — Infrastructure et squelette**
- [ ] Cluster Kubernetes opérationnel (K3s staging)
- [ ] PostgreSQL Patroni déployé (1 primaire + 2 replicas)
- [ ] NATS JetStream cluster 3 nœuds
- [ ] MinIO avec erasure coding
- [ ] Keycloak configuré avec realm `kors` et sync LDAP Active Directory Safran
- [ ] Pipeline CI/CD GitHub Actions (lint → test → build → push image)
- [ ] Squelette `kors-api` : chi router, health checks, middleware auth JWT

**Sprint 3–4 — Domaine KORS**
- [ ] Migrations SQL schéma `kors` (6 tables)
- [ ] Implémentation repositories PostgreSQL (resource, identity, resourcetype, event, revision, permission)
- [ ] Usecases : `CreateResource`, `UpdateResourceMetadata`, `TransitionResource`
- [ ] Usecase : `RegisterResourceType` avec validation JSON Schema
- [ ] Usecase : `GrantPermission`, `RevokePermission`, `CheckPermission`
- [ ] Résolution du graphe de transitions dans `ResourceType`

**Sprint 5–6 — API GraphQL et bus événements**
- [ ] Schéma GraphQL complet dans `schema.graphql`
- [ ] Génération gqlgen, implémentation de tous les resolvers
- [ ] Pagination cursor-based sur toutes les listes
- [ ] `kors-events` : consumer durable NATS, déduplication, persistence PostgreSQL
- [ ] Subscriptions GraphQL via graphql-ws + NATS fan-out
- [ ] Usecase `PublishEvent` avec publication NATS transactionnelle

**Sprint 7–8 — Révisions, MinIO et observabilité**
- [ ] Usecase `CreateRevision` avec snapshot et référence MinIO
- [ ] Intégration MinIO : upload, download, versioning
- [ ] OpenTelemetry : traces distribuées sur toutes les requêtes GraphQL
- [ ] Grafana Stack : dashboards latence, erreurs, événements/s
- [ ] Tests d'intégration complets (testcontainers)

**Jalon Phase 1** : un développeur externe peut créer un ResourceType, créer des Resources, les faire transiter, créer des révisions et recevoir les événements en temps réel via subscription GraphQL — sans assistance de l'équipe KORS.

---

### Phase 2 — Stabilisation et production (mois 5–8)

Objectif : KORS est validé par la DSI et l'équipe qualité, prêt pour un premier module en production.

**Sprint 9–10 — Hardening**
- [ ] `kors-worker` : jobs asynchrones (nettoyage permissions expirées, archivage events anciens)
- [ ] Rate limiting par identity (100 req/min standard, 1000 req/min service)
- [ ] Depth limit et complexity limit GraphQL
- [ ] Persisted queries en production
- [ ] Rotation automatique des secrets (Kubernetes Secrets + Vault si disponible)
- [ ] Backup PostgreSQL testé et documenté (PITR 30 jours)

**Sprint 11–12 — SAP et provisionnement modules**
- [ ] `kors-sap` : adaptateur OData SAP, synchronisation bidirectionnelle
- [ ] API de provisionnement module : création schéma + utilisateur PostgreSQL
- [ ] Documentation opérationnelle (runbooks Grafana)
- [ ] Tests de charge (k6) : 500 utilisateurs simultanés, p99 < 200ms

**Sprint 13–14 — Qualification EN9100**
- [ ] Audit traçabilité : reconstitution état d'une Resource à date passée
- [ ] Vérification rétention events 90 jours
- [ ] Dossier qualification pour l'équipe qualité Safran
- [ ] ADRs complètes et revues

**Jalon Phase 2** : KORS validé DSI et qualité, prêt à accueillir le premier module TMS en production.

---

### Phase 3 — Premier module TMS (mois 9–13)

Objectif : le TMS remplace Shopvue en production. Première licence résiliée.

- [ ] Module TMS développé sur KORS (hors scope de ce repo)
- [ ] Migration données Shopvue → KORS + TMS
- [ ] Run en parallèle Shopvue / TMS pendant 1 mois
- [ ] Bascule production, résiliation licence Shopvue

**Jalon Phase 3** : première licence résiliée, ROI KORS démontré.

---

### Phase 4 — Écosystème (mois 14–18)

- [ ] REX Phase 3, ajustements KORS si nécessaire
- [ ] Roadmap modules suivants validée (MES, PLM ou MEDS selon priorités DSI)
- [ ] Benchmark économique annuel (licences résiliées vs coût équipe KORS)

---

## 9. Décisions d'architecture (ADR)

Les ADRs sont dans `docs/adr/`. Toute décision technique significative fait l'objet d'un ADR avant implémentation. Template dans `docs/adr/000-template.md`.

ADRs existantes :
- `001` — Monorepo Go Workspace
- `002` — API GraphQL avec gqlgen
- `003` — NATS JetStream comme bus d'événements _(à rédiger)_
- `004` — Modèle de données hybride schémas PostgreSQL _(à rédiger)_
- `005` — Apollo Federation pour la fédération des schémas modules _(à rédiger)_

---

## 10. Configuration et variables d'environnement

Toute la configuration de KORS passe par des variables d'environnement. Aucune valeur sensible n'est commitée dans le repo. En développement local, un fichier `.env` copié depuis `.env.example` suffit. En production, les valeurs sont injectées par Kubernetes Secrets (ou Vault si disponible).

### kors-api

| Variable | Obligatoire | Défaut | Description |
|---|---|---|---|
| `PORT` | non | `8080` | Port d'écoute HTTP |
| `SERVICE_NAME` | non | `kors-api` | Nom du service pour les traces OpenTelemetry |
| `DATABASE_URL` | **oui** | — | DSN PostgreSQL complet avec credentials |
| `DATABASE_MAX_CONNS` | non | `25` | Taille max du pool de connexions pgx |
| `DATABASE_MIN_CONNS` | non | `5` | Taille min du pool de connexions pgx |
| `NATS_URL` | **oui** | — | URL NATS JetStream (ex: `nats://nats:4222`) |
| `NATS_STREAM_NAME` | non | `KORS` | Nom du stream JetStream |
| `MINIO_URL` | **oui** | — | Endpoint MinIO sans protocole (ex: `minio:9000`) |
| `MINIO_ACCESS_KEY` | **oui** | — | Clé d'accès MinIO |
| `MINIO_SECRET_KEY` | **oui** | — | Clé secrète MinIO |
| `MINIO_BUCKET` | non | `kors-files` | Bucket MinIO principal |
| `MINIO_USE_SSL` | non | `false` | TLS vers MinIO (`true` en production) |
| `JWKS_ENDPOINT` | **oui** | — | URL JWKS Keycloak pour vérification des JWT |
| `KEYCLOAK_REALM` | non | `kors` | Nom du realm Keycloak |
| `OTLP_ENDPOINT` | non | `localhost:4317` | Endpoint OpenTelemetry Collector (gRPC) |
| `OTLP_INSECURE` | non | `true` | Désactiver TLS vers OTLP (`false` en production) |
| `LOG_LEVEL` | non | `info` | Niveau de log : `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | non | `json` | Format des logs : `json` (prod) ou `pretty` (dev) |
| `GRAPHQL_DEPTH_LIMIT` | non | `10` | Profondeur max des requêtes GraphQL |
| `GRAPHQL_COMPLEXITY_LIMIT` | non | `1000` | Score de complexité max par requête |
| `GRAPHQL_INTROSPECTION` | non | `false` | Activer l'introspection (`true` en dev uniquement) |
| `RATE_LIMIT_STANDARD` | non | `100` | Requêtes par minute pour les identités standard |
| `RATE_LIMIT_SERVICE` | non | `1000` | Requêtes par minute pour les identités service |
| `SHUTDOWN_TIMEOUT` | non | `30` | Délai d'arrêt gracieux en secondes |

### kors-events

| Variable | Obligatoire | Défaut | Description |
|---|---|---|---|
| `SERVICE_NAME` | non | `kors-events` | Nom du service |
| `DATABASE_URL` | **oui** | — | DSN PostgreSQL (lecture/écriture schéma `kors`) |
| `NATS_URL` | **oui** | — | URL NATS JetStream |
| `NATS_CONSUMER_NAME` | non | `kors-events-consumer` | Nom du consumer durable JetStream |
| `NATS_STREAM_NAME` | non | `KORS` | Nom du stream JetStream |
| `NATS_MAX_ACK_PENDING` | non | `1000` | Messages en attente d'ACK maximum |
| `NATS_ACK_WAIT` | non | `30s` | Délai avant re-livraison si pas d'ACK |
| `OTLP_ENDPOINT` | non | `localhost:4317` | Endpoint OpenTelemetry Collector |
| `LOG_LEVEL` | non | `info` | Niveau de log |

### kors-worker

| Variable | Obligatoire | Défaut | Description |
|---|---|---|---|
| `SERVICE_NAME` | non | `kors-worker` | Nom du service |
| `DATABASE_URL` | **oui** | — | DSN PostgreSQL |
| `NATS_URL` | **oui** | — | URL NATS JetStream |
| `WORKER_CONCURRENCY` | non | `10` | Nombre de goroutines worker parallèles |
| `PERMISSIONS_CLEANUP_INTERVAL` | non | `1h` | Fréquence de nettoyage des permissions expirées |
| `EVENTS_ARCHIVE_AFTER` | non | `2160h` | Archivage des events après N heures (90 jours = 2160h) |
| `OTLP_ENDPOINT` | non | `localhost:4317` | Endpoint OpenTelemetry Collector |
| `LOG_LEVEL` | non | `info` | Niveau de log |

### kors-sap

| Variable | Obligatoire | Défaut | Description |
|---|---|---|---|
| `SERVICE_NAME` | non | `kors-sap` | Nom du service |
| `DATABASE_URL` | **oui** | — | DSN PostgreSQL |
| `NATS_URL` | **oui** | — | URL NATS JetStream |
| `SAP_HOST` | **oui** | — | Hôte du serveur SAP (ex: `sap-erp.safran-ls.local`) |
| `SAP_CLIENT` | **oui** | — | Numéro de mandant SAP (ex: `100`) |
| `SAP_USERNAME` | **oui** | — | Utilisateur technique SAP dédié KORS |
| `SAP_PASSWORD` | **oui** | — | Mot de passe utilisateur SAP |
| `SAP_SYSTEM_ID` | **oui** | — | System ID SAP (SID) |
| `SAP_ODATA_BASE_URL` | non | — | URL de base OData SAP si différente du host standard |
| `SAP_SYNC_INTERVAL` | non | `5m` | Fréquence de synchronisation SAP → KORS |
| `SAP_TIMEOUT` | non | `30s` | Timeout des appels SAP |
| `OTLP_ENDPOINT` | non | `localhost:4317` | Endpoint OpenTelemetry Collector |
| `LOG_LEVEL` | non | `info` | Niveau de log |

### Fichier `.env.example` (développement local)

```bash
# ── kors-api ──────────────────────────────────────────────────────────────────
PORT=8080
SERVICE_NAME=kors-api
LOG_LEVEL=debug
LOG_FORMAT=pretty
GRAPHQL_INTROSPECTION=true

# PostgreSQL
DATABASE_URL=postgres://kors:kors_dev@localhost:5432/kors?sslmode=disable
DATABASE_MAX_CONNS=10
DATABASE_MIN_CONNS=2

# NATS JetStream
NATS_URL=nats://localhost:4222
NATS_STREAM_NAME=KORS

# MinIO
MINIO_URL=localhost:9000
MINIO_ACCESS_KEY=kors_admin
MINIO_SECRET_KEY=kors_dev_secret
MINIO_BUCKET=kors-files
MINIO_USE_SSL=false

# Keycloak
JWKS_ENDPOINT=http://localhost:8180/realms/kors/protocol/openid-connect/certs
KEYCLOAK_REALM=kors

# OpenTelemetry
OTLP_ENDPOINT=localhost:4317
OTLP_INSECURE=true

# ── kors-events ───────────────────────────────────────────────────────────────
# DATABASE_URL=  (même valeur)
# NATS_URL=      (même valeur)
NATS_CONSUMER_NAME=kors-events-consumer
NATS_ACK_WAIT=30s

# ── kors-sap ──────────────────────────────────────────────────────────────────
# Renseigner uniquement si SAP accessible en dev (sinon mock)
# SAP_HOST=
# SAP_CLIENT=
# SAP_USERNAME=
# SAP_PASSWORD=
# SAP_SYSTEM_ID=
SAP_SYNC_INTERVAL=5m
SAP_TIMEOUT=30s
```

### Notes de déploiement en production

**Secrets Kubernetes.** Toutes les variables marquées **oui** dans la colonne Obligatoire doivent être injectées via `kind: Secret`. Ne jamais les mettre dans un `ConfigMap` ni dans les manifestes Kustomize commitées.

```yaml
# Exemple structure Secret Kubernetes
apiVersion: v1
kind: Secret
metadata:
  name: kors-api-secrets
  namespace: kors
type: Opaque
stringData:
  DATABASE_URL: "postgres://kors:XXXX@postgres-primary:5432/kors?sslmode=require"
  MINIO_ACCESS_KEY: "XXXX"
  MINIO_SECRET_KEY: "XXXX"
  SAP_PASSWORD: "XXXX"
```

**TLS obligatoire en production.** `MINIO_USE_SSL=true`, `OTLP_INSECURE=false`, `DATABASE_URL` avec `sslmode=require`.

**Introspection GraphQL.** `GRAPHQL_INTROSPECTION=false` impérativement en production.

**Log format.** `LOG_FORMAT=json` en production pour que Loki puisse parser les logs structurés.

---

## 11. Pour démarrer

```bash
# Cloner le repo
git clone git@github.com:safran-ls/kors.git
cd kors

# Copier les variables d'environnement locales
cp .env.example .env

# Démarrer l'environnement local (PostgreSQL, NATS, MinIO, Keycloak)
make dev

# Appliquer les migrations
make migrate

# Régénérer le code GraphQL après modification du schéma
make generate

# Lancer les tests
make test
```

---

