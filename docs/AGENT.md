# KORS AI Agent Technical Context (V1)

Ce document fournit une base de connaissances exhaustive pour les agents IA chargés de développer ou de maintenir des modules sur l'écosystème KORS.

---

## 1. Architecture du Système (Invariants)

KORS suit une architecture en couches stricte. En tant qu'agent, vous DEVEZ respecter ces règles :
*   **Layer Domain Pure** : Zéro dépendance externe (pas de NATS, pas de SQL) dans `internal/domain`. Utilisation exclusive de `std lib` et `google/uuid`.
*   **Dependency Injection** : Toutes les dépendances sont injectées dans `main.go`. Jamais de variables globales pour la DB ou NATS.
*   **Identité Universelle** : L'UUID d'une ressource KORS est l'unique source de vérité. Les modules tiers DOIVENT utiliser cet UUID comme clé primaire dans leur schéma local.

---

## 2. Base de Données (Schéma `kors`)

Si vous devez effectuer des requêtes de diagnostic directes, voici le schéma :

| Table | Colonne Pivot | Rôle |
|---|---|---|
| `identities` | `external_id` | Mapping avec l'UUID Keycloak. |
| `resource_types` | `name` | Contient le `json_schema` et les `transitions`. |
| `resources` | `id` (UUID) | État courant (`state`) et `metadata` (JSONB). |
| `events` | `nats_message_id` | Log immuable. Utilisé pour l'idempotence. |
| `permissions` | `identity_id` | RBAC. Colonnes `resource_id` et `resource_type_id` optionnelles. |

**Requête de vérification de transition :**
```sql
SELECT transitions->'IDLE' FROM kors.resource_types WHERE name = 'tool';
```

---

## 3. Communication & Protocoles

### Synchrone (GraphQL)
*   **Authentification** : Requiert un `Authorization: Bearer <token>`.
*   **Middleware Auth** : Injecte l'ID dans le contexte Go. Récupération via `korsctx.FromContext(ctx)`.
*   **Erreurs** : Ne pas chercher d'erreurs dans le tableau `errors` de GraphQL pour le métier. Vérifiez toujours le champ `error { code message }` du type de retour de la mutation.

### Asynchrone (NATS JetStream)
*   **Stream** : Nommé `KORS`.
*   **Subjects** : `kors.resource.created`, `kors.resource.state_changed`, `kors.resource.revision_created`.
*   **Idempotence** : `kors-events` vérifie chaque message via `Nats-Msg-Id` (UUID de l'événement).

---

## 4. Pièges et Débogage (Guide de Survie)

### Nil Pointer Dereference
Survient souvent dans les resolvers si une nouvelle UseCase n'a pas été injectée dans le `rootResolver` de `main.go`.
*   **Check** : Vérifier l'instanciation dans `kors-api/cmd/server/main.go`.

### SQL Not Null Violation
La colonne `metadata` dans `kors.resources` est `NOT NULL`.
*   **Check** : S'assurer que le UseCase initialise une map vide `make(map[string]interface{})` si l'input est `nil`.

### Subscription Failure
Les WebSockets requièrent l'init du protocole `graphql-ws`.
*   **Check** : Utiliser le script `kors-api/cmd/ws-monitor/main.go` pour tester la connexion.

---

## 5. Procédure de Validation Standard
Avant de considérer une tâche comme terminée, l'agent doit :
1.  **Compiler** : `cd kors-api; go build ./cmd/server`.
2.  **Tester** : Lancer le service sur Docker et exécuter le `test-client` approprié.
3.  **Vérifier les Logs** : `docker logs docker-kors-api-1`.
4.  **Vérifier la DB** : Requête `psql` directe pour confirmer la persistance.
5.  **Vérifier NATS** : Utiliser `nats-monitor` ou l'API `/jsz`.

---

## 6. Variables d'Environnement Critique
*   `DATABASE_URL` : Format `postgres://user:pass@host:5432/db?sslmode=disable`.
*   `NATS_URL` : Format `nats://host:4222`.
*   `MINIO_URL` : Host et Port uniquement (pas de `http://`).
*   `JWKS_ENDPOINT` : URL complète vers les certs Keycloak.
