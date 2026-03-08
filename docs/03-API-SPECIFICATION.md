# KORS Volume 3 : Spécification API et SDK

## 1. Endpoints de Connexion

| Service | Protocole | Endpoint (Dev) |
|---|---|---|
| API GraphQL | HTTP (POST) | `http://localhost:8080/query` |
| Subscriptions | WebSocket | `ws://localhost:8080/query` |
| Playground | HTTP (GET) | `http://localhost:8080/` |

**Header requis** : `Authorization: Bearer <JWT_TOKEN>`

## 2. Guide d'utilisation des SDKs

Trois SDKs professionnels sont disponibles et synchronisés avec le schéma KORS.

### SDK Go (genqlient)
*   **Init** : `sdk.NewClient(endpoint, token)`
*   **Caractéristique** : Client entièrement typé, asynchrone, supporte les transactions locales.

### SDK TypeScript (graphql-request)
*   **Init** : `new KorsClient({ endpoint, token })`
*   **Caractéristique** : Parfait pour les Dashboards React ou les modules Node.js. Exportation de tous les types `kors.graphql`.

### SDK Python (ariadne-codegen)
*   **Init** : `KorsClient(endpoint, token)`
*   **Caractéristique** : Asyncio-ready, basé sur Pydantic pour une validation stricte des inputs.

## 3. Détails des Primitives GraphQL

### ResourceType (`registerResourceType`)
*   **jsonSchema** : Doit être un objet valide JSON Schema Draft 7. Sert à valider le champ `metadata` des ressources.
*   **transitions** : Map d'états. Clé = État source, Valeur = Liste d'états cibles autorisés.

### Pagination Relay (`resources`)
KORS utilise la spécification Relay pour tous les listings massifs :
*   Arguments : `first` (Int), `after` (Base64 Cursor).
*   Réponse : `edges { node cursor }`, `pageInfo { hasNextPage endCursor }`, `totalCount`.

### Révisions (`createRevision`)
Permet de lier un snapshot de métadonnées à un binaire physique.
*   Le fichier est stocké dans MinIO.
*   Le SDK renvoie un `filePath` (chemin interne) et un `downloadUrl` (URL pré-signée temporaire).

## 4. Gouvernance des Modules

### Provisionner (`provisionModule`)
Crée les accès SQL et le schéma pour un module.
*   Paramètre : `moduleName` (String).
*   Retour : `success`, `username`, `password`, `schema`.

### Lister (`provisionedModules`)
Affiche les noms des modules actifs.
*   Retour : `[String!]`.

### Supprimer (`deprovisionModule`)
Détruit les accès et le schéma d'un module.
*   Paramètre : `moduleName` (String).
*   Retour : `Boolean`.
