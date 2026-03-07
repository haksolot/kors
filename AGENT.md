# KORS AI Agent Context

Ce fichier fournit les instructions et le contexte technique nécessaires à un agent IA pour développer des modules sur KORS.

## 1. Principes de Développement
*   **Identité Unique** : Chaque entité métier possède un UUID généré par KORS. Ne jamais générer un ID métier local sans l'enregistrer dans KORS.
*   **Découplage NATS** : Les modules ne s'appellent jamais entre eux (pas de REST direct). Communiquez via `kors.resource.*` sur NATS.
*   **Schémas Hybrides** : Vos données métier riches vont dans un schéma PostgreSQL dédié (ex: `tms.tools`), les métadonnées de cycle de vie vont dans `kors.resources`.

## 2. Guide de Débogage pour l'Agent
Si une action échoue sur l'API KORS, vérifiez dans cet ordre :

1.  **Permissions** :
    *   Vérifier si l'identité a le droit requis via :
    ```sql
    SELECT * FROM kors.permissions WHERE identity_id = '<ID>' AND action = '<ACTION>';
    ```
2.  **Machine à États** :
    *   Vérifier si la transition est autorisée dans le type :
    ```sql
    SELECT transitions FROM kors.resource_types WHERE name = '<TYPE_NAME>';
    ```
3.  **Logs API** :
    *   Chercher les `runtime error: invalid memory address` ou les erreurs de connexion NATS dans `docker logs docker-kors-api-1`.
4.  **Traces Jaeger** :
    *   Consulter `http://localhost:16686` pour voir où la requête bloque.

## 3. Snippets de code standards

### Connexion NATS (Go)
```go
nc, _ := nats.Connect(os.Getenv("NATS_URL"))
js, _ := nc.JetStream()
// Publier un événement métier
js.Publish("tms.tool.calibrated", payload)
```

### Appel KORS (GraphQL)
Utilisez toujours le header `Authorization: Bearer <token>`. L'absence du token ou un token invalide injectera un ID anonyme provoquant un rejet RBAC.

## 4. Architecture Monorepo
*   `kors-api` : Source de vérité synchrone.
*   `kors-events` : Gestionnaire de messages asynchrones.
*   `shared` : Helpers communs (Context, Tracing, Pagination).
*   `kors-worker` : Maintenance et tâches de fond.
